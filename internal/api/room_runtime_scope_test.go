package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"csgclaw/internal/apitypes"
	"csgclaw/internal/im"
	"csgclaw/internal/team"
	"csgclaw/internal/worklease"
)

type scopeLeaseReader struct{ rooms []string }

func (s *scopeLeaseReader) ActiveRooms(string) []string { return s.rooms }
func (*scopeLeaseReader) StartOrRenew(context.Context, worklease.ParticipantWorkLease) (apitypes.ParticipantWorkUpdate, error) {
	return apitypes.ParticipantWorkUpdate{}, nil
}
func (*scopeLeaseReader) Stop(context.Context, string, string) error { return nil }

func TestRoomRuntimeRejectsLegacyAndExternalRoutes(t *testing.T) {
	messages := im.NewService()
	for _, id := range []string{"dev", "outsider"} {
		if _, _, err := messages.EnsureAgentUser(im.EnsureAgentUserRequest{ID: id, Name: id, Role: "worker"}); err != nil {
			t.Fatal(err)
		}
	}
	room, err := messages.CreateRoom(im.CreateRoomRequest{Title: "Work", CreatorID: "admin", MemberIDs: []string{"dev"}, CollaborationMode: "on_demand"})
	if err != nil {
		t.Fatal(err)
	}
	leases := &scopeLeaseReader{rooms: []string{room.ID}}
	h := &Handler{im: messages, participantWork: leases}
	h.SetTeamService(team.NewService())
	routes := h.Routes()
	request := func(method, path, caller string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set("X-CSGClaw-Caller-Agent", caller)
		out := httptest.NewRecorder()
		routes.ServeHTTP(out, req)
		return out
	}
	for _, path := range []string{"/api/v1/agent-tasks", "/api/v1/teams", "/api/v1/rooms/elsewhere/tasks", "/api/v1/channels/csgclaw/rooms"} {
		if got := request(http.MethodPost, path, "agent-manager"); got.Code != http.StatusForbidden {
			t.Fatalf("legacy/external route %s: %d %s", path, got.Code, got.Body.String())
		}
	}
	for _, path := range []string{"/api/v1/agents", "/api/v1/channels/csgclaw/participants", "/api/v1/tasks", "/api/v1/channels/csgclaw/messages?room_id=elsewhere"} {
		if got := request(http.MethodGet, path, "agent-manager"); got.Code != http.StatusForbidden {
			t.Fatalf("global discovery %s: %d", path, got.Code)
		}
	}
	got := request(http.MethodGet, "/api/v1/rooms/"+room.ID+"/task-context", "agent-manager")
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "dev") || strings.Contains(got.Body.String(), "outsider") {
		t.Fatalf("room roster: %d %s", got.Code, got.Body.String())
	}
	got = request(http.MethodPost, "/api/v1/rooms/"+room.ID+"/tasks", "agent-dev")
	if got.Code != http.StatusForbidden {
		t.Fatalf("worker created root: %d", got.Code)
	}
	leases.rooms = []string{room.ID, "another-active-room"}
	if got := request(http.MethodGet, "/api/v1/rooms/"+room.ID+"/tasks", "agent-manager"); got.Code != http.StatusOK {
		t.Fatalf("explicit active task scope: %d %s", got.Code, got.Body.String())
	}
	if got := request(http.MethodGet, "/api/v1/agents", "agent-manager"); got.Code != http.StatusConflict {
		t.Fatalf("ambiguous scope not rejected: %d", got.Code)
	}
	leases.rooms = nil
	if got := request(http.MethodGet, "/api/v1/rooms/"+room.ID+"/tasks", "agent-manager"); got.Code != http.StatusOK {
		t.Fatalf("non-IM runtime workflow unexpectedly blocked: %d", got.Code)
	}
	// The browser and a human CLI do not inherit runtime restrictions.
	if got := request(http.MethodGet, "/api/v1/rooms/"+room.ID+"/tasks", ""); got.Code != http.StatusOK {
		t.Fatalf("human request blocked: %d", got.Code)
	}
}
