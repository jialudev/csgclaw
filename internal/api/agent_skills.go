package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"csgclaw/internal/agent"
)

type batchAddAgentSkillsRequest struct {
	Names []string `json:"names"`
}

func (h *Handler) handleAgentSkillsBatchAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.svc == nil {
		http.Error(w, "agent service is not configured", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(pathValue(r, "id"))
	if id == "" {
		http.NotFound(w, r)
		return
	}

	var req batchAddAgentSkillsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "decode request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.svc.BatchAddSkills(id, req.Names); err != nil {
		writeAgentSkillsMutationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleAgentSkillDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.svc == nil {
		http.Error(w, "agent service is not configured", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(pathValue(r, "id"))
	if id == "" {
		http.NotFound(w, r)
		return
	}
	if err := h.svc.DeleteSkill(id, pathValue(r, "name")); err != nil {
		writeAgentSkillsMutationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeAgentSkillsMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist), errors.Is(err, errAgentWorkspaceNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, agent.ErrAgentSkillAlreadyExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, agent.ErrAgentSkillInvalid):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}
