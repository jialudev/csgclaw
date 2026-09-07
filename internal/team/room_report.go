package team

import (
	"fmt"
	"strings"
)

const EventTaskReported = "task.reported"

type RoomTaskReportInput struct {
	RoomID  string
	TaskID  string
	Outcome string
	Summary string
}

// ReportRoomTask persists the manager's assessment separately from execution.
// Delivery uses the same durable event ID on every retry.
func (s *Service) ReportRoomTask(input RoomTaskReportInput) (TeamTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope := RoomScope(input.RoomID)
	task, meta, err := s.requireTaskLocked(scope, input.TaskID)
	if err != nil {
		return TeamTask{}, err
	}
	if task.ParentID != "" || !roomTaskTerminal(task.Status) {
		return TeamTask{}, fmt.Errorf("only an ended root task can be summarized")
	}
	if task.Report != "" {
		for i, event := range s.events[scope] {
			if event.Type == EventTaskReported && event.TaskID == task.ID {
				if err := s.projectEventsLocked(scope, i); err != nil {
					return TeamTask{}, err
				}
				break
			}
		}
		return cloneTask(*task), nil
	}
	switch input.Outcome {
	case "succeeded", "issues", "failed", "stopped":
	default:
		return TeamTask{}, fmt.Errorf("outcome must be succeeded, issues, failed, or stopped")
	}
	if strings.TrimSpace(input.Summary) == "" {
		return TeamTask{}, fmt.Errorf("summary is required")
	}
	if input.Outcome == "succeeded" && task.Status != TaskStatusCompleted {
		return TeamTask{}, fmt.Errorf("failed or stopped execution cannot be reported as succeeded")
	}
	before := s.captureTeamStateLocked(scope)
	start := len(s.events[scope])
	task.GoalOutcome = input.Outcome
	task.Report = strings.TrimSpace(input.Summary)
	task.ReportStatus = "pending"
	task.UpdatedAt = s.now()
	s.appendEventLocked(scope, TeamEvent{Type: EventTaskReported, TaskID: task.ID, ActorID: managerParticipantID(meta), Summary: task.Report, CreatedAt: s.now()})
	if err := s.persistMutationLocked(scope, before, start); err != nil {
		return TeamTask{}, err
	}
	return cloneTask(*task), nil
}

func roomTaskTerminal(status string) bool {
	return status == TaskStatusCompleted || status == TaskStatusFailed || status == TaskStatusCancelled
}

// RetryRoomDelivery replays durable projections without re-running any work.
func (s *Service) RetryRoomDelivery(roomID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.projectEventsLocked(RoomScope(roomID), 0)
}
