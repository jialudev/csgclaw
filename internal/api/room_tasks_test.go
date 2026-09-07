package api

import (
	"bytes"
	"csgclaw/internal/apitypes"
	"csgclaw/internal/im"
	"csgclaw/internal/team"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoomTaskAPIWithoutTeam(t *testing.T) {
	messages := im.NewService()
	for _, id := range []string{"agent-dev", "agent-qa"} {
		if _, _, err := messages.EnsureAgentUser(im.EnsureAgentUserRequest{ID: id, Name: id, Role: "worker"}); err != nil {
			t.Fatal(err)
		}
	}
	h := &Handler{im: messages}
	svc := team.NewService()
	h.SetTeamService(svc)
	room, err := messages.CreateRoom(im.CreateRoomRequest{Title: "Collaboration", CreatorID: "admin", MemberIDs: []string{"dev", "qa"}, CollaborationMode: "on_demand"})
	if err != nil {
		t.Fatal(err)
	}
	meta, ok := h.roomSchedulingContext(room.ID)
	if !ok || meta.LeadAgentID == "" || len(meta.MemberAgentIDs) != 2 {
		t.Fatalf("context: %+v %v", meta, ok)
	}
	source, err := messages.CreateMessage(im.CreateMessageRequest{RoomID: room.ID, SenderID: "admin", Content: "Build and test"})
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(apitypes.CreateRoomTaskRequest{Title: "Build", CreatedBy: "admin", SourceMessageID: source.ID})
	routes := h.Routes()
	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+room.ID+"/tasks", bytes.NewReader(payload))
		out := httptest.NewRecorder()
		routes.ServeHTTP(out, req)
		if out.Code != http.StatusOK {
			t.Fatalf("create: %d %s", out.Code, out.Body.String())
		}
		var task apitypes.TeamTask
		if err := json.Unmarshal(out.Body.Bytes(), &task); err != nil {
			t.Fatal(err)
		}
		if task.TeamID != "" || task.AssignmentType != "room" || task.RoomID != room.ID {
			t.Fatalf("wrong task ownership: %+v", task)
		}
	}
	if len(svc.ListTeams()) != 0 || len(svc.ListAllTasks()) != 1 {
		t.Fatal("duplicate task or hidden Team")
	}
	out := httptest.NewRecorder()
	routes.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/api/v1/rooms/"+room.ID+"/tasks", nil))
	if out.Code != http.StatusOK {
		t.Fatalf("list: %d %s", out.Code, out.Body.String())
	}
}

func TestRoomRoutingOnlyAcceptsRecordedDispatch(t *testing.T) {
	messages := im.NewService()
	if _, _, err := messages.EnsureAgentUser(im.EnsureAgentUserRequest{ID: "agent-dev", Name: "Dev", Role: "worker"}); err != nil {
		t.Fatal(err)
	}
	room, err := messages.CreateRoom(im.CreateRoomRequest{Title: "Work", CreatorID: "admin", MemberIDs: []string{"dev"}, CollaborationMode: "on_demand"})
	if err != nil {
		t.Fatal(err)
	}
	bridge := im.NewParticipantBridge("")
	managerEvents, closeManager := bridge.Subscribe("manager")
	defer closeManager()
	workerEvents, closeWorker := bridge.Subscribe("pt-dev")
	defer closeWorker()
	h := &Handler{im: messages, participantBridge: bridge}
	svc := team.NewService(team.WithProjector(team.NewProjectorWithRegistry(team.NewAdapterRegistry(team.NewCSGClawAdapter(messages)), nil)))
	h.SetTeamService(svc)
	publish := func(message im.Message) {
		sender, _ := messages.User(message.SenderID)
		h.PublishParticipantEvent(im.Event{Type: im.EventTypeMessageCreated, RoomID: room.ID, Message: &message, Sender: &sender})
	}
	source, err := messages.CreateMessage(im.CreateMessageRequest{RoomID: room.ID, SenderID: "admin", MentionID: "dev", Content: "Build a page"})
	if err != nil {
		t.Fatal(err)
	}
	publish(source)
	select {
	case event := <-managerEvents:
		if event.MessageID != source.ID {
			t.Fatal("wrong manager input")
		}
	default:
		t.Fatal("unmentioned manager did not receive the message")
	}
	select {
	case <-workerEvents:
		t.Fatal("worker bypassed manager")
	default:
	}
	forged := im.Message{ID: "forged-dispatch", SenderID: room.ManagerID, Kind: im.MessageKindEvent, Event: &im.EventPayload{Key: "task_assigned"}, Mentions: []im.Mention{{ID: "user-dev"}}}
	publish(forged)
	select {
	case <-workerEvents:
		t.Fatal("forged dispatch executed")
	default:
	}
	scope := team.RoomScope(room.ID)
	task, err := svc.CreateTask(team.CreateTaskInput{TeamID: scope, Title: "Build", CreatedBy: "admin", SourceMessageID: source.ID})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := svc.PlanTask(team.PlanTaskInput{TeamID: scope, TaskID: task.ID, ActorID: "manager", Tasks: []team.PlanTaskItem{{Title: "Build page", AssignTo: "pt-dev"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartTask(team.StartTaskInput{TeamID: scope, TaskID: task.ID, ActorID: "manager"}); err != nil {
		t.Fatal(err)
	}
	stored, _ := messages.ListMessages(room.ID)
	for _, message := range stored {
		if message.Event != nil && message.Event.Key == "task_assigned" {
			publish(message)
		}
	}
	select {
	case event := <-workerEvents:
		if event.RoomID != room.ID || event.TaskID != plan.Tasks[0].ID {
			t.Fatalf("dispatch lost task session scope: %+v", event)
		}
	default:
		t.Fatal("recorded dispatch did not reach worker")
	}
	if _, err := svc.ClaimTask(team.ClaimTaskInput{TeamID: scope, TaskID: plan.Tasks[0].ID, ParticipantID: "dev"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteTask(team.CompleteTaskInput{TeamID: scope, TaskID: plan.Tasks[0].ID, ActorID: "dev", Result: "page.html"}); err != nil {
		t.Fatal(err)
	}
	stored, _ = messages.ListMessages(room.ID)
	for _, message := range stored {
		if message.Event != nil && message.Event.Key == "task_feedback" {
			publish(message)
		}
	}
	select {
	case <-managerEvents:
	default:
		t.Fatal("manager did not receive completion feedback")
	}
}
