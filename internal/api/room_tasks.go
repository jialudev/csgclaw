package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"csgclaw/internal/apitypes"
	"csgclaw/internal/team"
)

func (h *Handler) handleReportRoomTask(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.requireTeamService(w)
	if !ok {
		return
	}
	var req struct {
		Outcome string `json:"outcome"`
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	task, err := svc.ReportRoomTask(team.RoomTaskReportInput{RoomID: pathValue(r, "id"), TaskID: pathValue(r, "task_id"), Outcome: req.Outcome, Summary: req.Summary})
	if err != nil {
		writeTeamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiTask(task, h.newTeamIdentityPresenter()))
}

func (h *Handler) handleRetryRoomDelivery(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.requireTeamService(w)
	if !ok {
		return
	}
	if err := svc.RetryRoomDelivery(pathValue(r, "id")); err != nil {
		writeTeamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiTasks(svc.ListTasks(team.RoomScope(pathValue(r, "id"))), h.newTeamIdentityPresenter()))
}

func (h *Handler) roomSchedulingContext(roomID string) (team.TeamMeta, bool) {
	if h.im == nil {
		return team.TeamMeta{}, false
	}
	room, ok := h.im.Room(roomID)
	if !ok || room.CollaborationMode != apitypes.CollaborationModeOnDemand || room.IsDirect {
		return team.TeamMeta{}, false
	}
	meta := team.TeamMeta{ID: team.RoomScope(room.ID), RoomID: room.ID, Title: room.Title, Status: team.TeamStatusActive}
	for _, memberID := range room.Members {
		if team.ParticipantIDsMatch(memberID, room.ManagerID) {
			user, found := h.im.User(memberID)
			if !found || !strings.EqualFold(user.Role, "manager") {
				return team.TeamMeta{}, false
			}
			meta.RoomManagerID = h.participantBridgeTargetForRoomMember(memberID).bridgeID
			meta.LeadAgentID = h.runtimeAgentIDForBridgeID(meta.RoomManagerID)
			continue
		}
		if user, found := h.im.User(memberID); found && (strings.EqualFold(user.Role, "worker") || strings.EqualFold(user.Role, "agent")) {
			if id := h.runtimeAgentIDForBridgeID(h.participantBridgeTargetForRoomMember(memberID).bridgeID); id != "" {
				meta.MemberAgentIDs = append(meta.MemberAgentIDs, id)
				meta.RoomWorkerIDs = append(meta.RoomWorkerIDs, h.participantBridgeTargetForRoomMember(memberID).bridgeID)
			}
		}
	}
	return meta, meta.LeadAgentID != ""
}

// Adapt only the internal scheduling namespace; public callers use room IDs.
func (h *Handler) withRoomTaskScope(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID := pathValue(r, "id")
		if _, ok := h.roomSchedulingContext(roomID); !ok {
			http.Error(w, "collaboration room or manager unavailable", http.StatusConflict)
			return
		}
		r.SetPathValue("team_id", team.RoomScope(roomID))
		next(w, r)
	}
}

func (h *Handler) handleCreateRoomTask(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.requireTeamService(w)
	if !ok {
		return
	}
	var req apitypes.CreateRoomTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	meta, found := h.roomSchedulingContext(pathValue(r, "id"))
	if !found {
		http.Error(w, "collaboration room or manager unavailable", http.StatusConflict)
		return
	}
	for _, existing := range svc.ListTasks(meta.ID) {
		if existing.ParentID == "" && existing.SourceMessageID == strings.TrimSpace(req.SourceMessageID) && team.ParticipantIDsMatch(existing.CreatedBy, req.CreatedBy) {
			writeJSON(w, http.StatusOK, apiTask(existing, h.newTeamIdentityPresenter()))
			return
		}
	}
	if len(meta.MemberAgentIDs) == 0 {
		http.Error(w, "no available room workers; add a suitable worker before starting", http.StatusConflict)
		return
	}
	// A stable source identifies the user's submission, not the manager's retry.
	room, _ := h.im.Room(meta.RoomID)
	sourceFound := false
	for _, message := range room.Messages {
		if message.ID == strings.TrimSpace(req.SourceMessageID) && team.ParticipantIDsMatch(message.SenderID, req.CreatedBy) {
			sourceFound = true
			break
		}
	}
	if !sourceFound {
		http.Error(w, "source_message_id must identify the requesting user's room message", http.StatusBadRequest)
		return
	}
	task, err := svc.CreateTask(team.CreateTaskInput{TeamID: meta.ID, Title: req.Title, Body: req.Body, CreatedBy: req.CreatedBy, SourceMessageID: strings.TrimSpace(req.SourceMessageID)})
	if err != nil {
		writeTeamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiTask(task, h.newTeamIdentityPresenter()))
}
