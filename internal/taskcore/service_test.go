package taskcore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServicePersistsRootChildAndEvents(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	svc := NewService(WithStore(store), WithNowFunc(func() time.Time { return now }))

	root, err := svc.CreateRoot(CreateRootInput{
		AssignmentType: AssignmentTypeTeam,
		AssignmentID:   "team-1",
		Title:          "Ship release",
		Body:           "Prepare launch",
		CreatedBy:      "pt-manager",
	})
	if err != nil {
		t.Fatalf("CreateRoot() error = %v", err)
	}
	child, err := svc.CreateChild(CreateChildInput{
		ParentID:   root.ID,
		Title:      "Draft note",
		CreatedBy:  "pt-manager",
		AssignedTo: "pt-writer",
	})
	if err != nil {
		t.Fatalf("CreateChild() error = %v", err)
	}
	if child.ParentID != root.ID || child.AssignmentType != AssignmentTypeTeam || child.AssignmentID != "team-1" {
		t.Fatalf("child assignment = %+v, want root assignment", child)
	}

	reloaded := NewService(WithStore(store))
	tasks := reloaded.ListByAssignment(AssignmentTypeTeam, "team-1")
	if len(tasks) != 2 {
		t.Fatalf("ListByAssignment() len = %d, want 2", len(tasks))
	}
	if events := reloaded.Events(root.ID); len(events) != 2 {
		t.Fatalf("Events() len = %d, want 2", len(events))
	}
}

func TestServiceClaimCompleteFailBlockApproval(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	svc := NewService(WithStore(store))
	task, err := svc.CreateRoot(CreateRootInput{
		AssignmentType: AssignmentTypeAgent,
		AssignmentID:   "agent-dev",
		Title:          "Fix bug",
		CreatedBy:      "user-admin",
		AssignedTo:     "pt-dev",
	})
	if err != nil {
		t.Fatalf("CreateRoot() error = %v", err)
	}

	claimed, err := svc.Claim(ClaimInput{TaskID: task.ID, ParticipantID: "pt-dev"})
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if claimed.Status != StatusInProgress || claimed.ClaimedBy != "pt-dev" {
		t.Fatalf("Claim() = %+v, want in_progress claimed by pt-dev", claimed)
	}
	approval, err := svc.RequestApproval(RequestApprovalInput{
		TaskID:      task.ID,
		RequestedBy: "pt-dev",
		ApproverID:  "pt-manager",
		Kind:        "command",
		Summary:     "run tests",
	})
	if err != nil {
		t.Fatalf("RequestApproval() error = %v", err)
	}
	resolved, err := svc.ResolveApproval(ResolveApprovalInput{
		ApprovalID: approval.ID,
		ApproverID: "pt-manager",
		Status:     ApprovalStatusApproved,
		Resolution: "ok",
	})
	if err != nil {
		t.Fatalf("ResolveApproval() error = %v", err)
	}
	if resolved.Status != ApprovalStatusApproved || resolved.ResolvedAt == nil {
		t.Fatalf("ResolveApproval() = %+v, want approved with resolved_at", resolved)
	}
	completed, err := svc.Complete(CompleteInput{TaskID: task.ID, ActorID: "pt-dev", Result: "done"})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if completed.Status != StatusCompleted || completed.Result != "done" || completed.CompletedAt == nil {
		t.Fatalf("Complete() = %+v, want completed result", completed)
	}

	failing, err := svc.CreateRoot(CreateRootInput{
		AssignmentType: AssignmentTypeAgent,
		AssignmentID:   "agent-dev",
		Title:          "Failing task",
		CreatedBy:      "user-admin",
	})
	if err != nil {
		t.Fatalf("CreateRoot(failing) error = %v", err)
	}
	if _, err := svc.Claim(ClaimInput{TaskID: failing.ID, ParticipantID: "pt-dev"}); err != nil {
		t.Fatalf("Claim(failing) error = %v", err)
	}
	blocked, err := svc.Block(BlockInput{TaskID: failing.ID, ActorID: "pt-dev", Reason: "need input"})
	if err != nil {
		t.Fatalf("Block() error = %v", err)
	}
	if blocked.Status != StatusBlocked || blocked.Error != "need input" {
		t.Fatalf("Block() = %+v, want blocked", blocked)
	}

	failedTask, err := svc.CreateRoot(CreateRootInput{
		AssignmentType: AssignmentTypeAgent,
		AssignmentID:   "agent-dev",
		Title:          "Another task",
		CreatedBy:      "user-admin",
	})
	if err != nil {
		t.Fatalf("CreateRoot(failedTask) error = %v", err)
	}
	if _, err := svc.Claim(ClaimInput{TaskID: failedTask.ID, ParticipantID: "pt-dev"}); err != nil {
		t.Fatalf("Claim(failedTask) error = %v", err)
	}
	failed, err := svc.Fail(FailInput{TaskID: failedTask.ID, ActorID: "pt-dev", Error: "boom"})
	if err != nil {
		t.Fatalf("Fail() error = %v", err)
	}
	if failed.Status != StatusFailed || failed.Error != "boom" {
		t.Fatalf("Fail() = %+v, want failed", failed)
	}
}

func TestStoreTrimsPartialEventLine(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	svc := NewService(WithStore(store))
	task, err := svc.CreateRoot(CreateRootInput{
		AssignmentType: AssignmentTypeTeam,
		AssignmentID:   "team-1",
		Title:          "Task",
		CreatedBy:      "pt-manager",
	})
	if err != nil {
		t.Fatalf("CreateRoot() error = %v", err)
	}
	eventsPath := filepath.Join(root, task.ID, eventsFileName)
	file, err := os.OpenFile(eventsPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("open events for append: %v", err)
	}
	if _, err := file.Write([]byte(`{"seq":99`)); err != nil {
		file.Close()
		t.Fatalf("append partial event: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close events: %v", err)
	}
	reloaded := NewService(WithStore(store))
	if events := reloaded.Events(task.ID); len(events) != 1 {
		t.Fatalf("Events() len = %d, want valid event only", len(events))
	}
}

func TestServiceRejectsInvalidTransitions(t *testing.T) {
	svc := NewService()
	task, err := svc.CreateRoot(CreateRootInput{
		AssignmentType: AssignmentTypeAgent,
		AssignmentID:   "agent-dev",
		Title:          "Task",
		CreatedBy:      "user-admin",
	})
	if err != nil {
		t.Fatalf("CreateRoot() error = %v", err)
	}
	_, err = svc.Complete(CompleteInput{TaskID: task.ID, ActorID: "pt-dev", Result: "done"})
	if !errors.Is(err, ErrTransitionInvalid) {
		t.Fatalf("Complete() error = %v, want ErrTransitionInvalid", err)
	}
}
