package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"csgclaw/internal/agenttask"
	"csgclaw/internal/apitypes"
	"csgclaw/internal/im"
	"csgclaw/internal/participant"
	"csgclaw/internal/taskcore"
	"csgclaw/internal/worklease"
)

func createAutonomousTaskFixture(t *testing.T, h *Handler) (im.Room, im.Message) {
	t.Helper()
	room, err := h.im.CreateRoom(im.CreateRoomRequest{
		Title: "Gomoku", CreatorID: "admin", MemberIDs: []string{"dev", "qa"}, Type: apitypes.RoomTypeAutonomous,
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := h.im.CreateMessage(im.CreateMessageRequest{RoomID: room.ID, SenderID: "admin", Content: "Build a Gomoku game"})
	if err != nil {
		t.Fatal(err)
	}
	return room, source
}

func TestAutonomousTaskWholeClaimRacesWithThreadSplit(t *testing.T) {
	h := newAppTaskTestHandler(t)
	store, err := taskcore.NewStore(filepath.Join(t.TempDir(), "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	core := taskcore.NewService(taskcore.WithStore(store))
	h.SetRoomTaskCore(core)
	room, source := createAutonomousTaskFixture(t, h)
	type outcome struct {
		result autonomousTaskResponse
		err    error
	}
	start := make(chan struct{})
	results := make(chan outcome, 2)
	run := func(actor, tool string, args map[string]any) {
		<-start
		value, err := h.runAutonomousTaskTool(actor, tool, args)
		results <- outcome{value.(autonomousTaskResponse), err}
	}
	go run("agent-dev", "autonomous_task_claim", map[string]any{"room_id": room.ID, "source_message_id": source.ID})
	go run("agent-qa", "autonomous_task_split", map[string]any{"room_id": room.ID, "thread_root_id": source.ID, "source_message_id": source.ID,
		"parts": []map[string]string{{"title": "Board", "scope": "board.ts"}, {"title": "Strategy", "scope": "strategy.ts"}}})
	close(start)
	wins := 0
	for range 2 {
		got := <-results
		if got.err != nil || len(got.result.Tasks) == 0 || got.result.NotificationPending {
			t.Fatalf("unexpected outcome: %+v", got)
		}
		if got.result.Accepted {
			wins++
		} else if got.result.Reason == "" {
			t.Fatal("conflict did not explain the current decision")
		}
	}
	reopened := taskcore.NewService(taskcore.WithStore(store))
	tasks := reopened.ListByAssignment(taskcore.AssignmentTypeRoom, room.ID)
	var roots int
	for _, task := range tasks {
		if task.ParentID == "" {
			roots++
			if task.SourceMessageID != source.ID || task.AssignedTo == "" {
				t.Fatalf("invalid persisted owner: %+v", task)
			}
		}
	}
	if wins != 1 || roots != 1 || (len(tasks) != 1 && len(tasks) != 3) || len(h.roomTaskSvc.List("")) != 0 {
		t.Fatalf("wins=%d roots=%d tasks=%+v", wins, roots, tasks)
	}
}

func TestAutonomousTaskToolsRecordIndependentPartsAndStayInScope(t *testing.T) {
	h := newAppTaskTestHandler(t)
	h.serverNoAuth = true
	room, source := createAutonomousTaskFixture(t, h)
	other, _ := createAutonomousTaskFixture(t, h)
	h.participant = participant.NewService(participant.NewMemoryStore([]apitypes.Participant{
		{ID: "pt-dev", Channel: participant.ChannelCSGClaw, Type: participant.TypeAgent, ChannelUserRef: "user-dev", LifecycleStatus: participant.LifecycleStatusActive},
		{ID: "pt-qa", Channel: participant.ChannelCSGClaw, Type: participant.TypeAgent, ChannelUserRef: "user-qa", LifecycleStatus: participant.LifecycleStatusActive},
	}))
	registry := worklease.NewRegistry(h.participant, h.im, worklease.NewBus())
	h.participantWork = registry
	for _, actor := range []string{"pt-dev", "pt-qa"} {
		if _, err := registry.StartOrRenew(context.Background(), worklease.ParticipantWorkLease{
			ParticipantID: actor, LeaseID: worklease.NewID(), RoomID: room.ID, ThreadRootID: source.ID, Kind: apitypes.ParticipantWorkKindAgentTurn,
		}); err != nil {
			t.Fatal(err)
		}
	}
	dev, qa := taskMCPClient(t, h, "agent-dev"), taskMCPClient(t, h, "agent-qa")
	var split autonomousTaskResponse
	callTaskTool(t, qa, "autonomous_task_split", map[string]any{"source_message_id": source.ID,
		"parts": []map[string]string{{"title": "Board", "scope": "board.ts"}, {"title": "Strategy", "scope": "strategy.ts"}}}, &split)
	if !split.Accepted || split.NotificationPending || len(split.Tasks) != 3 {
		t.Fatalf("split: %+v", split)
	}
	ids := make(map[string]string)
	for _, task := range split.Tasks {
		ids[task.Title] = task.ID
	}
	denyTaskTool(t, dev, "autonomous_task_claim", map[string]any{"task_id": ids["Board"], "room_id": other.ID})
	denyTaskTool(t, dev, "autonomous_tasks_list", map[string]any{"thread_root_id": "other-thread"})
	var claimed autonomousTaskResponse
	callTaskTool(t, dev, "autonomous_task_claim", map[string]any{"task_id": ids["Board"]}, &claimed)
	if !claimed.Accepted || claimed.NotificationPending {
		t.Fatalf("claim: %+v", claimed)
	}
	var conflict autonomousTaskResponse
	callTaskTool(t, qa, "autonomous_task_claim", map[string]any{"task_id": ids["Board"]}, &conflict)
	if conflict.Accepted || len(conflict.Tasks) != 3 {
		t.Fatalf("expected conflict with the existing assignment: %+v", conflict)
	}
	if _, err := h.agentTaskSvc.Claim(agenttask.ClaimInput{TaskID: ids["Strategy"], ParticipantID: "pt-dev"}); err == nil {
		t.Fatal("a personal task operation accepted a room task")
	}
	if _, err := h.agentTaskSvc.Update(agenttask.UpdateInput{TaskID: ids["Board"], ActorID: "pt-dev", Status: taskcore.StatusCompleted, Result: "wrong route"}); err == nil {
		t.Fatal("a personal task update accepted a room task")
	}
	var second autonomousTaskResponse
	callTaskTool(t, qa, "autonomous_task_claim", map[string]any{"task_id": ids["Strategy"]}, &second)
	if !second.Accepted {
		t.Fatalf("independent claim: %+v", second)
	}
	var handoff autonomousTaskResponse
	callTaskTool(t, dev, "autonomous_task_update", map[string]any{"task_id": ids["Board"], "status": "handed_off", "next_owner": "pt-qa", "summary": "Board is ready for integration"}, &handoff)
	if !handoff.Accepted || handoff.NotificationPending {
		t.Fatalf("handoff: %+v", handoff)
	}
	var takeover autonomousTaskResponse
	callTaskTool(t, qa, "autonomous_task_claim", map[string]any{"task_id": ids["Board"]}, &takeover)
	if !takeover.Accepted {
		t.Fatalf("takeover: %+v", takeover)
	}
	var premature autonomousTaskResponse
	callTaskTool(t, qa, "autonomous_task_update", map[string]any{"task_id": ids[source.Content], "status": "completed", "summary": "Integrated"}, &premature)
	if premature.Accepted {
		t.Fatal("parent completed before its parts")
	}
	for _, title := range []string{"Board", "Strategy", source.Content} {
		var completed autonomousTaskResponse
		callTaskTool(t, qa, "autonomous_task_update", map[string]any{"task_id": ids[title], "status": "completed", "summary": title + " verified"}, &completed)
		if !completed.Accepted || completed.NotificationPending {
			t.Fatalf("completion of %q: %+v", title, completed)
		}
	}
	view, err := h.im.GetThread(room.ID, source.ID)
	if err != nil || len(view.Replies) != 8 {
		t.Fatalf("want eight successful updates in original thread, got %d: %v", len(view.Replies), err)
	}
	unread, err := h.im.PrepareUnread(room.ID, "pt-dev", source.ID, 20, true)
	if err != nil || len(unread.Messages) != 6 {
		t.Fatalf("peer cannot read the task updates in the original thread: %+v, %v", unread, err)
	}
	out := httptest.NewRecorder()
	h.Routes().ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/api/v1/rooms/"+room.ID+"/tasks", nil))
	if out.Code != http.StatusOK {
		t.Fatalf("room view: %d %s", out.Code, out.Body.String())
	}
}
