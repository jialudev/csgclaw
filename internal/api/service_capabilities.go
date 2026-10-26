package api

import "net/http"

func (h *Handler) getServiceCapabilities(w http.ResponseWriter, r *http.Request) {
	if !h.assistantAuthorized(w, r) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"backend_id": h.control.store.ID, "control_api_version": 1, "api_only": h.apiOnly, "ready": true})
}

func (h *Handler) assistantAuthorized(w http.ResponseWriter, r *http.Request) bool {
	if !h.validateServerAccessToken(r.Header.Get("Authorization")) {
		controlError(w, 401, "unauthorized", "service credential is required")
		return false
	}
	if h.control == nil || h.control.store == nil || h.agentEngine == nil {
		controlError(w, 503, "control_unavailable", "control service is unavailable")
		return false
	}
	return true
}
