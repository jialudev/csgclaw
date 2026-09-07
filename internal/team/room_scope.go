package team

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ValidRoomProjection prevents model-authored mentions or forged event kinds
// from becoming a dispatch. The durable scheduling event must exist.
func (s *Service) ValidRoomProjection(roomID, messageID, kind, target string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope := RoomScope(roomID)
	meta, err := s.requireTeamLocked(scope)
	if err != nil {
		return false
	}
	for _, event := range s.events[scope] {
		if messageID != fmt.Sprintf("team:%s:event:%d", scope, event.Seq) {
			continue
		}
		if kind == "task_assigned" && event.Type == EventTaskDispatched {
			task := s.tasks[scope][event.TaskID]
			return task != nil && task.Status == TaskStatusAssigned && task.DispatchedAt != nil && roomWorker(meta, target) && ParticipantIDsMatch(event.TargetID, target)
		}
		if kind == "task_feedback" && ParticipantIDsMatch(managerParticipantID(meta), target) {
			if task := s.tasks[scope][event.TaskID]; task != nil {
				if task.ReportStatus == "delivered" {
					return false
				}
				if task.ParentID != "" {
					if parent := s.tasks[scope][task.ParentID]; parent != nil && roomTaskTerminal(parent.Status) {
						return false
					}
				}
			}
			switch event.Type {
			case EventTaskCompleted, EventTaskFailed, EventTaskBlocked, EventTaskCancelled:
				return true
			}
		}
	}
	return false
}

const roomScopePrefix = "room:"

// Serial room execution stops at an execution failure/cancellation. Unstarted
// work is explicitly cancelled; completed results remain available to report.
func (s *Service) finishRoomFailureLocked(meta TeamMeta, task *TeamTask, actorID string, now time.Time) {
	if meta.RoomID == "" {
		return
	}
	root := task
	if task.ParentID != "" {
		root = s.tasks[meta.ID][task.ParentID]
	}
	if root == nil {
		return
	}
	for _, child := range s.taskChildrenLocked(meta.ID, root.ID) {
		if roomTaskTerminal(child.Status) {
			continue
		}
		child.Status = TaskStatusCancelled
		child.Error = "execution stopped after a failed or cancelled step"
		child.UpdatedAt = now
		child.CompletedAt = timePtr(now)
		s.appendEventLocked(meta.ID, TeamEvent{Type: EventTaskCancelled, TaskID: child.ID, ActorID: actorID, Summary: child.Error, CreatedAt: now})
	}
	if root.ID != task.ID {
		root.Status = task.Status
		root.Error = task.Error
		root.UpdatedAt = now
		root.CompletedAt = timePtr(now)
		kind := EventTaskFailed
		if task.Status == TaskStatusCancelled {
			kind = EventTaskCancelled
		}
		s.appendEventLocked(meta.ID, TeamEvent{Type: kind, TaskID: root.ID, ActorID: actorID, Summary: root.Error, CreatedAt: now})
	}
}

func (s *Service) RoomWorkerStopped(roomID, participantID string) error {
	scope := RoomScope(roomID)
	for _, task := range s.ListTasks(scope) {
		if task.ParentID != "" && task.Status == TaskStatusInProgress && ParticipantIDsMatch(task.AssignedTo, participantID) {
			_, err := s.CancelTask(CancelTaskInput{TeamID: scope, TaskID: task.ID, ActorID: participantID, Reason: "stopped by user", ExecutionStopped: true})
			return err
		}
	}
	return nil
}

var ErrRoomTaskExists = errors.New("this room already has a root task; new tasks and replanning are not supported in this version")

// RoomScope is an internal task namespace, not a Team identity.
func RoomScope(roomID string) string { return roomScopePrefix + strings.TrimSpace(roomID) }
func IsRoomScope(scope string) bool  { return strings.HasPrefix(scope, roomScopePrefix) }

func validateRoomPlan(items []PlanTaskItem) error {
	byRef := map[string]PlanTaskItem{}
	for i, item := range items {
		ref := strings.TrimSpace(item.IDRef)
		if ref == "" {
			ref = fmt.Sprintf("task_%d", i+1)
		}
		byRef[ref] = item
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(ref string) error {
		if visiting[ref] {
			return fmt.Errorf("task dependency cycle at %s", ref)
		}
		if done[ref] {
			return nil
		}
		item, ok := byRef[ref]
		if !ok {
			return fmt.Errorf("unknown task dependency %s", ref)
		}
		visiting[ref] = true
		for _, dep := range item.DependsOnRefs {
			if strings.TrimSpace(dep) != "" {
				if err := visit(strings.TrimSpace(dep)); err != nil {
					return err
				}
			}
		}
		visiting[ref] = false
		done[ref] = true
		return nil
	}
	for ref := range byRef {
		if err := visit(ref); err != nil {
			return err
		}
	}
	return nil
}

// Membership updates affect eligibility, never the recorded plan or assignees.
func (s *Service) RoomMembersChanged(roomID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope := RoomScope(roomID)
	meta, err := s.requireTeamLocked(scope)
	if err != nil {
		return err
	}
	before := s.captureTeamStateLocked(scope)
	start := len(s.events[scope])
	for _, task := range s.tasks[scope] {
		if task.ParentID == "" || (task.Status != TaskStatusPending && task.Status != TaskStatusAssigned) || roomWorker(meta, task.AssignedTo) {
			continue
		}
		task.Status = TaskStatusBlocked
		task.Error = "assigned worker is no longer a room member"
		task.UpdatedAt = s.now()
		s.appendEventLocked(scope, TeamEvent{Type: EventTaskBlocked, TaskID: task.ID, Summary: task.Error, CreatedAt: s.now()})
	}
	if len(s.events[scope]) == start {
		return nil
	}
	return s.persistMutationLocked(scope, before, start)
}

// SetRoomResolver supplies a fresh scheduling context from the authoritative
// room roster. The returned context is never registered or persisted as a Team.
func (s *Service) SetRoomResolver(resolve func(string) (TeamMeta, bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roomResolver = resolve
}

func managerParticipantID(meta TeamMeta) string {
	if meta.RoomID != "" && meta.RoomManagerID != "" {
		return cleanParticipantID(meta.RoomManagerID)
	}
	return defaultParticipantIDForAgentID(meta.LeadAgentID)
}

func roomWorker(meta TeamMeta, id string) bool {
	if len(meta.RoomWorkerIDs) > 0 {
		for _, member := range meta.RoomWorkerIDs {
			if ParticipantIDsMatch(member, id) {
				return true
			}
		}
		return false
	}
	if ParticipantIDsMatch(managerParticipantID(meta), id) {
		return false
	}
	for _, agentID := range meta.MemberAgentIDs {
		if ParticipantIDsMatch(defaultParticipantIDForAgentID(agentID), id) {
			return true
		}
	}
	return false
}
