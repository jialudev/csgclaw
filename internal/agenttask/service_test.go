package agenttask

import (
	"context"
	"strings"
	"testing"

	"csgclaw/internal/im"
	"csgclaw/internal/taskcore"
)

func TestCreateAgentTaskBindsDirectRoomAndSendsInitialMessage(t *testing.T) {
	core := taskcore.NewService()
	imSvc := im.NewService()
	svc := NewService(core, imSvc, nil, nil)

	task, err := svc.CreateAgentTask(context.Background(), CreateInput{
		AgentID:   "agent-dev",
		Title:     "Fix flaky test",
		Body:      "Investigate the failure.",
		CreatedBy: im.AdminUserID,
	})
	if err != nil {
		t.Fatalf("CreateAgentTask() error = %v", err)
	}
	if task.AssignmentType != taskcore.AssignmentTypeAgent || task.AssignmentID != "agent-dev" {
		t.Fatalf("task assignment = %s/%s, want agent/agent-dev", task.AssignmentType, task.AssignmentID)
	}
	if task.RoomID == "" {
		t.Fatal("task.RoomID = empty, want direct room")
	}
	room, ok := imSvc.Room(task.RoomID)
	if !ok {
		t.Fatalf("Room(%s) found = false", task.RoomID)
	}
	if !room.IsDirect {
		t.Fatalf("room.IsDirect = false, want true")
	}
	if len(room.Messages) < 2 {
		t.Fatalf("room messages len = %d, want bootstrap and task message", len(room.Messages))
	}
	last := room.Messages[len(room.Messages)-1]
	if last.Kind != im.MessageKindEvent || last.Event == nil || last.Event.Key != "task_assigned" {
		t.Fatalf("initial message event = kind %q payload %+v, want task_assigned event", last.Kind, last.Event)
	}
	if last.Event.Title != "task-1 [Fix f...]" || len(last.Event.TargetIDs) != 1 || last.Event.TargetIDs[0] != "user-dev" {
		t.Fatalf("initial message event = %+v, want compact task title and user-dev target", last.Event)
	}
	if !strings.Contains(last.Content, task.ID) || !strings.Contains(last.Content, "Fix flaky test") {
		t.Fatalf("initial message content = %q, want task id and title", last.Content)
	}
	for _, want := range []string{
		"csgclaw-cli task claim --task " + task.ID,
		"csgclaw-cli task update --task " + task.ID,
	} {
		if !strings.Contains(last.Content, want) {
			t.Fatalf("initial message content = %q, want %q", last.Content, want)
		}
	}
	if strings.Contains(last.Content, "POST /api/v1/agent-tasks") || strings.Contains(last.Content, "PATCH /api/v1/agent-tasks") {
		t.Fatalf("initial message still contains raw HTTP guidance: %q", last.Content)
	}
	if len(last.Mentions) != 1 || last.Mentions[0].ID != "user-dev" {
		t.Fatalf("mentions = %+v, want user-dev", last.Mentions)
	}
}

func TestClaimAndCompleteAgentTask(t *testing.T) {
	core := taskcore.NewService()
	imSvc := im.NewService()
	svc := NewService(core, imSvc, nil, nil)
	task, err := svc.CreateAgentTask(context.Background(), CreateInput{
		AgentID:   "agent-dev",
		Title:     "Fix flaky test",
		CreatedBy: im.AdminUserID,
	})
	if err != nil {
		t.Fatalf("CreateAgentTask() error = %v", err)
	}
	claimed, err := svc.Claim(ClaimInput{TaskID: task.ID, ParticipantID: "pt-dev"})
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if claimed.Status != taskcore.StatusInProgress || claimed.ClaimedBy != "pt-dev" {
		t.Fatalf("Claim() = %+v, want in_progress by pt-dev", claimed)
	}
	completed, err := svc.Update(UpdateInput{
		TaskID:  task.ID,
		ActorID: "pt-dev",
		Status:  taskcore.StatusCompleted,
		Result:  "done",
	})
	if err != nil {
		t.Fatalf("Update(completed) error = %v", err)
	}
	if completed.Status != taskcore.StatusCompleted || completed.Result != "done" {
		t.Fatalf("Update(completed) = %+v, want completed", completed)
	}
}
