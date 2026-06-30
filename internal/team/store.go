package team

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"csgclaw/internal/localstore"
	"csgclaw/internal/taskcore"
)

type Store struct {
	statePath string
	tasks     *taskcore.Store
}

type teamSnapshot struct {
	Meta      TeamMeta
	Tasks     []TeamTask
	Approvals []TeamApproval
	Presence  []MemberPresence
	Events    []TeamEvent
}

type rootTeamsState struct {
	Items []TeamMeta `json:"items"`
}

func NewStore(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if err := validateStorePath(path); err != nil {
		return nil, err
	}
	taskStore, err := taskcore.NewStore(defaultTaskStoreRoot(path))
	if err != nil {
		return nil, err
	}
	return NewStoreWithTaskStore(path, taskStore)
}

func NewStoreWithTaskStore(path string, taskStore *taskcore.Store) (*Store, error) {
	path = strings.TrimSpace(path)
	if err := validateStorePath(path); err != nil {
		return nil, err
	}
	if taskStore == nil {
		return nil, fmt.Errorf("task store is required")
	}
	return &Store{statePath: path, tasks: taskStore}, nil
}

func (s *Store) TaskStore() *taskcore.Store {
	if s == nil {
		return nil
	}
	return s.tasks
}

func validateStorePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("team store path is required")
	}
	if !localstore.IsRootStatePath(path) {
		return fmt.Errorf("team store path must point to root state.json")
	}
	return nil
}

func defaultTaskStoreRoot(statePath string) string {
	statePath = strings.TrimSpace(statePath)
	return filepath.Join(filepath.Dir(statePath), "tasks")
}

func (s *Store) Load() ([]teamSnapshot, error) {
	if s == nil {
		return nil, fmt.Errorf("store is required")
	}
	return s.loadRootState()
}

func (s *Store) Save(snapshot teamSnapshot, newEvents []TeamEvent) error {
	if s == nil {
		return fmt.Errorf("store is required")
	}
	if strings.TrimSpace(snapshot.Meta.ID) == "" {
		return fmt.Errorf("team id is required")
	}
	return s.saveRootState(snapshot, newEvents)
}

func (s *Store) Delete(teamID string) error {
	if s == nil {
		return fmt.Errorf("store is required")
	}
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return fmt.Errorf("team id is required")
	}
	return s.deleteRootState(teamID)
}

func (s *Store) loadRootState() ([]teamSnapshot, error) {
	items, err := s.loadRootTeamItems()
	if err != nil {
		return nil, err
	}
	out := make([]teamSnapshot, 0, len(items))
	for _, meta := range items {
		meta = cloneTeamMeta(meta)
		if strings.TrimSpace(meta.ID) == "" {
			return nil, fmt.Errorf("team id is required")
		}
		taskSnapshots, err := s.tasks.LoadByAssignment(taskcore.AssignmentTypeTeam, meta.ID)
		if err != nil {
			return nil, err
		}
		tasks, approvals, presence, events := teamStateFromTaskCore(taskSnapshots)
		out = append(out, teamSnapshot{
			Meta:      meta,
			Tasks:     tasks,
			Approvals: approvals,
			Presence:  presence,
			Events:    events,
		})
	}
	return out, nil
}

func (s *Store) saveRootState(snapshot teamSnapshot, newEvents []TeamEvent) error {
	if err := s.saveTaskSnapshots(snapshot, newEvents); err != nil {
		return err
	}
	items, err := s.loadRootTeamItems()
	if err != nil {
		return err
	}
	meta := cloneTeamMeta(snapshot.Meta)
	replaced := false
	for i := range items {
		if items[i].ID == meta.ID {
			items[i] = meta
			replaced = true
			break
		}
	}
	if !replaced {
		items = append(items, meta)
	}
	sortTeamMeta(items)
	return s.writeRootTeamItems(items)
}

func (s *Store) deleteRootState(teamID string) error {
	if s.tasks != nil {
		if err := s.tasks.DeleteAssignment(taskcore.AssignmentTypeTeam, teamID); err != nil {
			return err
		}
	}
	items, err := s.loadRootTeamItems()
	if err != nil {
		return err
	}
	next := make([]TeamMeta, 0, len(items))
	for _, item := range items {
		if item.ID != teamID {
			next = append(next, item)
		}
	}
	return s.writeRootTeamItems(next)
}

func (s *Store) loadRootTeamItems() ([]TeamMeta, error) {
	var state rootTeamsState
	ok, err := localstore.ReadSection(s.statePath, "teams", &state)
	if err != nil {
		return nil, err
	}
	if !ok || len(state.Items) == 0 {
		return []TeamMeta{}, nil
	}
	items := make([]TeamMeta, 0, len(state.Items))
	for _, item := range state.Items {
		items = append(items, cloneTeamMeta(item))
	}
	sortTeamMeta(items)
	return items, nil
}

func (s *Store) writeRootTeamItems(items []TeamMeta) error {
	if items == nil {
		items = []TeamMeta{}
	}
	for i := range items {
		items[i] = cloneTeamMeta(items[i])
	}
	sortTeamMeta(items)
	return localstore.WriteSection(s.statePath, "teams", rootTeamsState{Items: items})
}

func sortTeamMeta(items []TeamMeta) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}

func (s *Store) saveTaskSnapshots(snapshot teamSnapshot, newEvents []TeamEvent) error {
	if s.tasks == nil {
		return nil
	}
	taskSnapshots, rootByTaskID, rootByApprovalID := taskCoreSnapshotsFromTeamState(snapshot)
	eventsByRoot := make(map[string][]taskcore.TaskEvent)
	for _, event := range newEvents {
		rootID := ""
		if strings.TrimSpace(event.TaskID) != "" {
			rootID = rootByTaskID[strings.TrimSpace(event.TaskID)]
		}
		if rootID == "" && strings.TrimSpace(event.TargetID) != "" {
			rootID = rootByApprovalID[strings.TrimSpace(event.TargetID)]
		}
		if rootID == "" {
			continue
		}
		eventsByRoot[rootID] = append(eventsByRoot[rootID], taskCoreEventFromTeamEvent(event))
	}
	for _, taskSnapshot := range taskSnapshots {
		if err := s.tasks.SaveSnapshot(taskSnapshot, eventsByRoot[taskSnapshot.Root.ID]); err != nil {
			return err
		}
	}
	return nil
}

func taskCoreSnapshotsFromTeamState(snapshot teamSnapshot) ([]taskcore.Snapshot, map[string]string, map[string]string) {
	tasksByID := make(map[string]TeamTask, len(snapshot.Tasks))
	for _, task := range snapshot.Tasks {
		tasksByID[task.ID] = task
	}
	rootByTaskID := make(map[string]string, len(snapshot.Tasks))
	var resolveRoot func(string) string
	resolveRoot = func(taskID string) string {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			return ""
		}
		if rootID, ok := rootByTaskID[taskID]; ok {
			return rootID
		}
		task, ok := tasksByID[taskID]
		if !ok {
			return ""
		}
		if strings.TrimSpace(task.ParentID) == "" {
			rootByTaskID[taskID] = task.ID
			return task.ID
		}
		rootID := resolveRoot(task.ParentID)
		rootByTaskID[taskID] = rootID
		return rootID
	}
	for _, task := range snapshot.Tasks {
		resolveRoot(task.ID)
	}

	snapshotsByRoot := make(map[string]*taskcore.Snapshot)
	for _, task := range snapshot.Tasks {
		rootID := rootByTaskID[task.ID]
		if rootID == "" {
			continue
		}
		coreTask := taskCoreTaskFromTeamTask(task)
		if task.ID == rootID {
			coreTask.ParentID = ""
			coreSnapshot := snapshotsByRoot[rootID]
			if coreSnapshot == nil {
				coreSnapshot = &taskcore.Snapshot{}
				snapshotsByRoot[rootID] = coreSnapshot
			}
			coreSnapshot.Root = coreTask
			continue
		}
		coreSnapshot := snapshotsByRoot[rootID]
		if coreSnapshot == nil {
			coreSnapshot = &taskcore.Snapshot{Root: taskCoreTaskFromTeamTask(tasksByID[rootID])}
			coreSnapshot.Root.ParentID = ""
			snapshotsByRoot[rootID] = coreSnapshot
		}
		coreSnapshot.Children = append(coreSnapshot.Children, coreTask)
	}

	rootByApprovalID := make(map[string]string, len(snapshot.Approvals))
	for _, approval := range snapshot.Approvals {
		rootID := rootByTaskID[strings.TrimSpace(approval.TaskID)]
		if rootID == "" {
			continue
		}
		coreSnapshot := snapshotsByRoot[rootID]
		if coreSnapshot == nil {
			continue
		}
		coreSnapshot.Approvals = append(coreSnapshot.Approvals, taskCoreApprovalFromTeamApproval(approval))
		rootByApprovalID[approval.ID] = rootID
	}
	for _, p := range snapshot.Presence {
		rootID := rootByTaskID[strings.TrimSpace(p.CurrentTaskID)]
		if rootID == "" {
			continue
		}
		coreSnapshot := snapshotsByRoot[rootID]
		if coreSnapshot == nil {
			continue
		}
		coreSnapshot.Presence = append(coreSnapshot.Presence, taskCorePresenceFromTeamPresence(p))
	}
	for _, event := range snapshot.Events {
		rootID := ""
		if strings.TrimSpace(event.TaskID) != "" {
			rootID = rootByTaskID[strings.TrimSpace(event.TaskID)]
		}
		if rootID == "" && strings.TrimSpace(event.TargetID) != "" {
			rootID = rootByApprovalID[strings.TrimSpace(event.TargetID)]
		}
		if rootID == "" {
			continue
		}
		coreSnapshot := snapshotsByRoot[rootID]
		if coreSnapshot == nil {
			continue
		}
		coreSnapshot.Events = append(coreSnapshot.Events, taskCoreEventFromTeamEvent(event))
	}

	out := make([]taskcore.Snapshot, 0, len(snapshotsByRoot))
	for _, snapshot := range snapshotsByRoot {
		sort.Slice(snapshot.Children, func(i, j int) bool { return snapshot.Children[i].ID < snapshot.Children[j].ID })
		sort.Slice(snapshot.Approvals, func(i, j int) bool { return snapshot.Approvals[i].ID < snapshot.Approvals[j].ID })
		sort.Slice(snapshot.Presence, func(i, j int) bool { return snapshot.Presence[i].ParticipantID < snapshot.Presence[j].ParticipantID })
		out = append(out, *snapshot)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Root.ID < out[j].Root.ID })
	return out, rootByTaskID, rootByApprovalID
}

func teamStateFromTaskCore(snapshots []taskcore.Snapshot) ([]TeamTask, []TeamApproval, []MemberPresence, []TeamEvent) {
	tasks := make([]TeamTask, 0)
	approvals := make([]TeamApproval, 0)
	presenceByParticipant := make(map[string]MemberPresence)
	events := make([]TeamEvent, 0)
	for _, snapshot := range snapshots {
		tasks = append(tasks, teamTaskFromTaskCore(snapshot.Root))
		for _, child := range snapshot.Children {
			tasks = append(tasks, teamTaskFromTaskCore(child))
		}
		for _, approval := range snapshot.Approvals {
			approvals = append(approvals, teamApprovalFromTaskCore(approval))
		}
		for _, p := range snapshot.Presence {
			presenceByParticipant[p.ParticipantID] = teamPresenceFromTaskCore(p)
		}
		for _, event := range snapshot.Events {
			events = append(events, teamEventFromTaskCore(event))
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	sort.Slice(approvals, func(i, j int) bool { return approvals[i].ID < approvals[j].ID })
	presence := make([]MemberPresence, 0, len(presenceByParticipant))
	for _, p := range presenceByParticipant {
		presence = append(presence, p)
	}
	sort.Slice(presence, func(i, j int) bool { return presence[i].ParticipantID < presence[j].ParticipantID })
	sort.Slice(events, func(i, j int) bool {
		if events[i].Seq == events[j].Seq {
			return events[i].CreatedAt.Before(events[j].CreatedAt)
		}
		return events[i].Seq < events[j].Seq
	})
	return tasks, approvals, presence, events
}

func taskCoreTaskFromTeamTask(task TeamTask) taskcore.Task {
	return taskcore.Task{
		ID:               task.ID,
		ParentID:         task.ParentID,
		AssignmentType:   taskcore.AssignmentTypeTeam,
		AssignmentID:     task.TeamID,
		Title:            task.Title,
		Body:             task.Body,
		Status:           task.Status,
		CreatedBy:        task.CreatedBy,
		AssignedTo:       task.AssignedTo,
		ClaimedBy:        task.ClaimedBy,
		ExecutionChannel: task.ExecutionChannel,
		RoomID:           task.RoomID,
		DependsOn:        cloneStrings(task.DependsOn),
		Priority:         task.Priority,
		PlanSummary:      task.PlanSummary,
		DispatchedAt:     cloneTimePtr(task.DispatchedAt),
		DeadlineAt:       cloneTimePtr(task.DeadlineAt),
		TimeoutAt:        cloneTimePtr(task.TimeoutAt),
		Result:           task.Result,
		Error:            task.Error,
		CreatedAt:        task.CreatedAt,
		UpdatedAt:        task.UpdatedAt,
		CompletedAt:      cloneTimePtr(task.CompletedAt),
	}
}

func teamTaskFromTaskCore(task taskcore.Task) TeamTask {
	return TeamTask{
		ID:               task.ID,
		TeamID:           task.AssignmentID,
		ExecutionChannel: task.ExecutionChannel,
		RoomID:           task.RoomID,
		ParentID:         task.ParentID,
		Title:            task.Title,
		Body:             task.Body,
		Status:           task.Status,
		CreatedBy:        task.CreatedBy,
		AssignedTo:       task.AssignedTo,
		ClaimedBy:        task.ClaimedBy,
		DependsOn:        cloneStrings(task.DependsOn),
		Priority:         task.Priority,
		PlanSummary:      task.PlanSummary,
		DispatchedAt:     cloneTimePtr(task.DispatchedAt),
		DeadlineAt:       cloneTimePtr(task.DeadlineAt),
		TimeoutAt:        cloneTimePtr(task.TimeoutAt),
		Result:           task.Result,
		Error:            task.Error,
		CreatedAt:        task.CreatedAt,
		UpdatedAt:        task.UpdatedAt,
		CompletedAt:      cloneTimePtr(task.CompletedAt),
	}
}

func taskCoreApprovalFromTeamApproval(approval TeamApproval) taskcore.TaskApproval {
	return taskcore.TaskApproval{
		ID:             approval.ID,
		AssignmentType: taskcore.AssignmentTypeTeam,
		AssignmentID:   approval.TeamID,
		RoomID:         approval.RoomID,
		TaskID:         approval.TaskID,
		RequestedBy:    approval.RequestedBy,
		ApproverID:     approval.ApproverID,
		Kind:           approval.Kind,
		Summary:        approval.Summary,
		Payload:        approval.Payload,
		Status:         approval.Status,
		Resolution:     approval.Resolution,
		CreatedAt:      approval.CreatedAt,
		ResolvedAt:     cloneTimePtr(approval.ResolvedAt),
	}
}

func teamApprovalFromTaskCore(approval taskcore.TaskApproval) TeamApproval {
	return TeamApproval{
		ID:          approval.ID,
		TeamID:      approval.AssignmentID,
		RoomID:      approval.RoomID,
		TaskID:      approval.TaskID,
		RequestedBy: approval.RequestedBy,
		ApproverID:  approval.ApproverID,
		Kind:        approval.Kind,
		Summary:     approval.Summary,
		Payload:     approval.Payload,
		Status:      approval.Status,
		Resolution:  approval.Resolution,
		CreatedAt:   approval.CreatedAt,
		ResolvedAt:  cloneTimePtr(approval.ResolvedAt),
	}
}

func taskCorePresenceFromTeamPresence(p MemberPresence) taskcore.TaskPresence {
	return taskcore.TaskPresence{
		AssignmentType:  taskcore.AssignmentTypeTeam,
		AssignmentID:    p.TeamID,
		ParticipantID:   p.ParticipantID,
		UserID:          p.UserID,
		AgentID:         p.AgentID,
		Role:            p.Role,
		State:           p.State,
		CurrentTaskID:   p.CurrentTaskID,
		Summary:         p.Summary,
		LastHeartbeatAt: p.LastHeartbeatAt,
		UpdatedAt:       p.UpdatedAt,
	}
}

func teamPresenceFromTaskCore(p taskcore.TaskPresence) MemberPresence {
	return MemberPresence{
		TeamID:          p.AssignmentID,
		ParticipantID:   p.ParticipantID,
		UserID:          p.UserID,
		AgentID:         p.AgentID,
		Role:            p.Role,
		State:           p.State,
		CurrentTaskID:   p.CurrentTaskID,
		Summary:         p.Summary,
		LastHeartbeatAt: p.LastHeartbeatAt,
		UpdatedAt:       p.UpdatedAt,
	}
}

func taskCoreEventFromTeamEvent(event TeamEvent) taskcore.TaskEvent {
	return taskcore.TaskEvent{
		Seq:            event.Seq,
		AssignmentType: taskcore.AssignmentTypeTeam,
		AssignmentID:   event.TeamID,
		Channel:        event.Channel,
		RoomID:         event.RoomID,
		Type:           event.Type,
		ActorID:        event.ActorID,
		TaskID:         event.TaskID,
		TargetID:       event.TargetID,
		Summary:        event.Summary,
		CreatedAt:      event.CreatedAt,
	}
}

func teamEventFromTaskCore(event taskcore.TaskEvent) TeamEvent {
	return TeamEvent{
		Seq:       event.Seq,
		TeamID:    event.AssignmentID,
		Channel:   event.Channel,
		RoomID:    event.RoomID,
		Type:      event.Type,
		ActorID:   event.ActorID,
		TaskID:    event.TaskID,
		TargetID:  event.TargetID,
		Summary:   event.Summary,
		CreatedAt: event.CreatedAt,
	}
}
