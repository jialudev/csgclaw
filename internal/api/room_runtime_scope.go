package api

import (
	"fmt"
	"net/http"
	"strings"

	"csgclaw/internal/apitypes"
	"csgclaw/internal/team"
)

type activeRoomReader interface {
	ActiveRooms(participantID string) []string
}

// Runtime CLI calls inherit their caller identity. Scope is resolved from the
// server's active work lease, not a model-selected room or a process-wide room
// variable. Browser/human CLI calls have no runtime identity and remain global.
// This is workflow isolation; an unsandboxed process with the server credential
// is still privileged and must not be considered a hostile-code security boundary.
func (h *Handler) roomRuntimeScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := strings.TrimSpace(r.Header.Get("X-CSGClaw-Caller-Agent"))
		if caller == "" {
			next.ServeHTTP(w, r)
			return
		}
		reader, ok := h.participantWork.(activeRoomReader)
		if !ok || h.im == nil {
			// No built-in IM execution context (for example Feishu or a detached
			// maintenance command): do not change those existing workflows.
			next.ServeHTTP(w, r)
			return
		}
		participantID := team.NewCSGClawTeamDirectory(h.im, h.agentEngine, h.participant).ParticipantIDForAgentID(caller)
		rooms := reader.ActiveRooms(participantID)
		if len(rooms) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		selectedRoom := ""
		// Independent task sessions can be active in different rooms. An explicit
		// room-task path may narrow to one active lease, never to an inactive room.
		for _, id := range rooms {
			base := "/api/v1/rooms/" + id
			if r.URL.Path == base+"/task-context" || r.URL.Path == base+"/tasks" || strings.HasPrefix(r.URL.Path, base+"/tasks/") {
				selectedRoom = id
			}
		}
		if selectedRoom == "" && len(rooms) == 1 {
			selectedRoom = rooms[0]
		}
		if selectedRoom == "" {
			http.Error(w, "runtime execution scope missing or ambiguous; retry in the active task turn", http.StatusConflict)
			return
		}
		room, found := h.im.Room(selectedRoom)
		if !found {
			http.Error(w, "active room unavailable", http.StatusConflict)
			return
		}
		if room.CollaborationMode != apitypes.CollaborationModeOnDemand {
			next.ServeHTTP(w, r)
			return
		}
		meta, valid := h.roomSchedulingContext(room.ID)
		if !valid {
			http.Error(w, "collaboration roster unavailable", http.StatusConflict)
			return
		}
		manager := team.ParticipantIDsMatch(participantID, meta.RoomManagerID)
		member := manager
		for _, id := range meta.RoomWorkerIDs {
			member = member || team.ParticipantIDsMatch(participantID, id)
		}
		if !member {
			http.Error(w, "caller is no longer a room member", http.StatusForbidden)
			return
		}
		path := strings.TrimRight(r.URL.Path, "/")
		base := "/api/v1/rooms/" + room.ID
		allowed := r.Method == http.MethodGet && (path == base+"/task-context" || path == base+"/tasks")
		if manager && (path == base+"/tasks" || strings.HasPrefix(path, base+"/tasks/")) {
			allowed = true
		}
		if !manager && strings.HasPrefix(path, base+"/tasks/") && h.teamSvc != nil {
			parts := strings.Split(strings.TrimPrefix(path, base+"/tasks/"), "/")
			task, found := h.teamSvc.GetTask(team.RoomScope(room.ID), parts[0])
			if found && task.ParentID != "" && team.ParticipantIDsMatch(task.AssignedTo, participantID) {
				allowed = (r.Method == http.MethodPatch && len(parts) == 1) || (r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "claim")
			}
		}
		if r.Method == http.MethodGet {
			allowed = allowed || path == "/api/v1/channels/csgclaw/rooms/"+room.ID+"/members"
			allowed = allowed || (path == "/api/v1/channels/csgclaw/messages" && r.URL.Query().Get("room_id") == room.ID)
		}
		if !allowed {
			http.Error(w, fmt.Sprintf("on-demand room %s is the execution boundary: use room-task context/list/submit/plan in this room; global Agent/Team/direct-task APIs and other rooms are unavailable", room.ID), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) handleRoomTaskContext(w http.ResponseWriter, r *http.Request) {
	meta, ok := h.roomSchedulingContext(pathValue(r, "id"))
	if !ok {
		http.Error(w, "collaboration room unavailable", http.StatusConflict)
		return
	}
	room, _ := h.im.Room(meta.RoomID)
	type member struct {
		ID          string `json:"participant_id"`
		Name        string `json:"name"`
		Role        string `json:"role"`
		Description string `json:"description,omitempty"`
	}
	members := make([]member, 0, len(room.Members))
	directory := h.teamDirectory()
	for _, id := range room.Members {
		user, found := h.im.User(id)
		if !found {
			continue
		}
		participantID := h.participantBridgeTargetForRoomMember(id).bridgeID
		item := member{ID: participantID, Name: user.Name, Role: user.Role}
		if profile, found := directory.AgentProfile(h.runtimeAgentIDForBridgeID(participantID)); found {
			item.Description = profile.Description
		}
		members = append(members, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"room_id": room.ID, "manager_id": meta.RoomManagerID, "members": members, "assignable_worker_ids": meta.RoomWorkerIDs, "task_policy": "one root; always plan concrete worker subtasks; serial execution; same room only"})
}
