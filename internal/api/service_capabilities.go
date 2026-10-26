package api

import "net/http"

func (h *Handler) getServiceCapabilities(w http.ResponseWriter, r *http.Request) {
	if !h.assistantAuthorized(w, r) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"backend_id": h.control.store.ID, "control_api_version": 1, "api_only": h.apiOnly, "ready": true})
}
