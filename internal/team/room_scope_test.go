package team

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"csgclaw/internal/im"
	"csgclaw/internal/taskcore"
)

func TestRoomCompletionSummaryDeliveryAndRetry(t *testing.T) {
	messages := im.NewService()
	for _, id := range []string{"u-dev", "u-qa"} {
		if _, _, err := messages.EnsureAgentUser(im.EnsureAgentUserRequest{ID: id, Name: id, Role: "worker"}); err != nil {
			t.Fatal(err)
		}
	}
	room, err := messages.CreateRoom(im.CreateRoomRequest{Title: "Build", CreatorID: "admin", MemberIDs: []string{"dev", "qa"}, CollaborationMode: "on_demand"})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(WithStore(store))
	svc.SetRoomResolver(func(id string) (TeamMeta, bool) {
		return TeamMeta{ID: RoomScope(id), RoomID: id, LeadAgentID: "u-manager", MemberAgentIDs: []string{"u-dev", "u-qa"}, Status: TeamStatusActive}, id == room.ID
	})
	scope := RoomScope(room.ID)
	root, err := svc.CreateTask(CreateTaskInput{TeamID: scope, Title: "Build", CreatedBy: "admin", SourceMessageID: "source"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := svc.PlanTask(PlanTaskInput{TeamID: scope, TaskID: root.ID, ActorID: "manager", Tasks: []PlanTaskItem{{IDRef: "dev", Title: "Develop", AssignTo: "dev"}, {IDRef: "qa", Title: "Test", AssignTo: "qa", DependsOnRefs: []string{"dev"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartTask(StartTaskInput{TeamID: scope, TaskID: root.ID, ActorID: "manager"}); err != nil {
		t.Fatal(err)
	}
	for i, worker := range []string{"dev", "qa"} {
		if _, err := svc.ClaimTask(ClaimTaskInput{TeamID: scope, TaskID: plan.Tasks[i].ID, ParticipantID: worker}); err != nil {
			t.Fatal(err)
		}
		result := "page.html"
		if i == 1 {
			result = "Testing complete; accessibility defects remain"
		}
		if _, err := svc.CompleteTask(CompleteTaskInput{TeamID: scope, TaskID: plan.Tasks[i].ID, ActorID: worker, Result: result}); err != nil {
			t.Fatal(err)
		}
	}
	input := RoomTaskReportInput{RoomID: room.ID, TaskID: root.ID, Outcome: "issues", Summary: "page.html delivered. Tests found accessibility defects; manual follow-up required."}
	report, err := svc.ReportRoomTask(input)
	if err != nil || report.Status != TaskStatusCompleted || report.GoalOutcome != "issues" || report.ReportStatus != "pending" {
		t.Fatalf("report state %+v %v", report, err)
	}
	svc.projector = NewProjectorWithRegistry(NewAdapterRegistry(NewCSGClawAdapter(messages)), nil)
	if err := svc.RetryRoomDelivery(room.ID); err != nil {
		t.Fatal(err)
	}
	report, _ = svc.GetTask(scope, root.ID)
	if report.ReportStatus != "delivered" {
		t.Fatalf("report not delivered: %+v", report)
	}
	if _, err := svc.ReportRoomTask(input); err != nil {
		t.Fatal(err)
	}
	delivered, _ := messages.ListMessages(room.ID)
	count := 0
	for _, message := range delivered {
		if message.Event != nil && message.Event.Key == "task_reported" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("summary duplicated: %d", count)
	}
	reloaded := NewService(WithStore(store))
	reloaded.SetRoomResolver(svc.roomResolver)
	restored, _ := reloaded.GetTask(scope, root.ID)
	if restored.Report != input.Summary || restored.ReportStatus != "delivered" || restored.GoalOutcome != "issues" {
		t.Fatalf("report lost on restart: %+v", restored)
	}
	input2 := CreateTaskInput{TeamID: scope, Title: "Another task", CreatedBy: "admin", SourceMessageID: "another"}
	if _, err := reloaded.CreateTask(input2); !errors.Is(err, ErrRoomTaskExists) {
		t.Fatalf("second task allowed after completion: %v", err)
	}
}

func TestRoomFailureClosesRemainingWork(t *testing.T) {
	svc := NewService()
	svc.SetRoomResolver(roomTestResolver([]string{"u-dev", "u-qa"}))
	scope := RoomScope("room-test")
	root, _ := svc.CreateTask(CreateTaskInput{TeamID: scope, Title: "Build", CreatedBy: "admin", SourceMessageID: "source"})
	plan, err := svc.PlanTask(PlanTaskInput{TeamID: scope, TaskID: root.ID, ActorID: "manager", Tasks: []PlanTaskItem{{Title: "Build", AssignTo: "dev"}, {Title: "Test", AssignTo: "qa"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartTask(StartTaskInput{TeamID: scope, TaskID: root.ID, ActorID: "manager"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ClaimTask(ClaimTaskInput{TeamID: scope, TaskID: plan.Tasks[0].ID, ParticipantID: "dev"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FailTask(FailTaskInput{TeamID: scope, TaskID: plan.Tasks[0].ID, ActorID: "dev", Error: "build failed"}); err != nil {
		t.Fatal(err)
	}
	parent, _ := svc.GetTask(scope, root.ID)
	qa, _ := svc.GetTask(scope, plan.Tasks[1].ID)
	if parent.Status != TaskStatusFailed || qa.Status != TaskStatusCancelled || qa.DispatchedAt != nil {
		t.Fatalf("incomplete failure closure: %+v %+v", parent, qa)
	}
	if _, err := svc.ReportRoomTask(RoomTaskReportInput{RoomID: "room-test", TaskID: root.ID, Outcome: "succeeded", Summary: "Done"}); err == nil {
		t.Fatal("failure reported as success")
	}
}

func roomTestResolver(members []string) func(string) (TeamMeta, bool) {
	return func(id string) (TeamMeta, bool) {
		return TeamMeta{ID: RoomScope(id), RoomID: id, LeadAgentID: "u-manager", MemberAgentIDs: members, Status: TeamStatusActive}, id == "room-test"
	}
}

func TestRoomTaskPersistenceAndConcurrentSubmission(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(WithStore(store))
	svc.SetRoomResolver(roomTestResolver([]string{"u-dev", "u-qa"}))
	input := CreateTaskInput{TeamID: RoomScope("room-test"), Title: "Build and test", CreatedBy: "admin", SourceMessageID: "message-1"}
	var wg sync.WaitGroup
	ids := make(chan string, 10)
	for range 10 {
		wg.Go(func() {
			task, err := svc.CreateTask(input)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- task.ID
		})
	}
	wg.Wait()
	close(ids)
	id := ""
	for got := range ids {
		if id != "" && got != id {
			t.Fatalf("duplicate roots %s and %s", id, got)
		}
		id = got
	}
	if len(svc.ListTeams()) != 0 {
		t.Fatal("room task must not create a Team")
	}
	task, _ := svc.GetTask(input.TeamID, id)
	if task.RoomID != "room-test" || task.AssignedTo != "pt-manager" {
		t.Fatalf("incorrect binding: %+v", task)
	}
	reloaded := NewService(WithStore(store))
	reloaded.SetRoomResolver(roomTestResolver([]string{"u-dev"}))
	retry, err := reloaded.CreateTask(input)
	if err != nil || retry.ID != id {
		t.Fatalf("retry after restart: %+v %v", retry, err)
	}
	input.SourceMessageID = "message-2"
	if _, err = reloaded.CreateTask(input); !errors.Is(err, ErrRoomTaskExists) {
		t.Fatalf("second root: %v", err)
	}
	snapshots, err := store.tasks.LoadByAssignment(taskcore.AssignmentTypeRoom, "room-test")
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("room assignment not persisted: %v %+v", err, snapshots)
	}
	teams, err := store.loadRootTeamItems()
	if err != nil || len(teams) != 0 {
		t.Fatalf("unexpected persisted teams: %+v %v", teams, err)
	}
	if len(reloaded.ListAllTasks()) != 1 {
		t.Fatal("room tasks missing from global board")
	}
}

func TestRoomPlanSerialExecutionAndMemberRemoval(t *testing.T) {
	svc := NewService()
	svc.SetRoomResolver(roomTestResolver([]string{"u-dev", "u-qa"}))
	scope := RoomScope("room-test")
	root, err := svc.CreateTask(CreateTaskInput{TeamID: scope, Title: "Build", CreatedBy: "admin", SourceMessageID: "message-1"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := svc.PlanTask(PlanTaskInput{TeamID: scope, TaskID: root.ID, ActorID: "manager", Tasks: []PlanTaskItem{
		{IDRef: "dev", Title: "Build page", Body: "Deliver HTML", AssignTo: "dev"},
		{IDRef: "qa", Title: "Test page", Body: "Deliver test findings", AssignTo: "qa", DependsOnRefs: []string{"dev"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	started, err := svc.StartTask(StartTaskInput{TeamID: scope, TaskID: root.ID, ActorID: "manager"})
	if err != nil || started.ScheduledCount != 1 {
		t.Fatalf("start: %+v %v", started, err)
	}
	if _, err = svc.ClaimTask(ClaimTaskInput{TeamID: scope, TaskID: plan.Tasks[1].ID, ParticipantID: "qa"}); err == nil {
		t.Fatal("QA ran before dispatch")
	}
	if _, err = svc.ClaimTask(ClaimTaskInput{TeamID: scope, TaskID: plan.Tasks[0].ID, ParticipantID: "dev"}); err != nil {
		t.Fatal(err)
	}
	svc.SetRoomResolver(roomTestResolver([]string{"u-dev"}))
	if _, err = svc.CompleteTask(CompleteTaskInput{TeamID: scope, TaskID: plan.Tasks[0].ID, ActorID: "dev", Result: "page.html"}); err != nil {
		t.Fatal(err)
	}
	qa, _ := svc.GetTask(scope, plan.Tasks[1].ID)
	if qa.Status != TaskStatusBlocked || qa.DispatchedAt != nil || qa.Error == "" {
		t.Fatalf("removed QA incorrectly dispatched: %+v", qa)
	}
	if len(svc.ListTeams()) != 0 {
		t.Fatal("Team created during planning")
	}
}

func TestRoomPlanRejectsOutsideWorkers(t *testing.T) {
	svc := NewService()
	svc.SetRoomResolver(roomTestResolver([]string{"u-dev"}))
	scope := RoomScope("room-test")
	root, err := svc.CreateTask(CreateTaskInput{TeamID: scope, Title: "Build", CreatedBy: "admin", SourceMessageID: "message-1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.PlanTask(PlanTaskInput{TeamID: scope, TaskID: root.ID, ActorID: "manager", Tasks: []PlanTaskItem{{Title: "A", AssignTo: "dev"}, {Title: "B", AssignTo: "outsider"}}})
	if err == nil || len(svc.ListTasks(scope)) != 1 {
		t.Fatalf("invalid plan was partially applied: %v", err)
	}
}
