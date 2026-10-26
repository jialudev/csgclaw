package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIOnlyRemovesDesktopActions(t *testing.T) {
	h := &Handler{apiOnly: true}
	router := h.Routes()
	for _, path := range []string{"/api/v1/local/directory-picker", "/api/v1/rooms/r/attachments/f/open", "/api/v1/agents/test/workspace/open", "/api/v1/agents/test/workspace/location"} {
		method := http.MethodPost
		if path == "/api/v1/agents/test/workspace/location" {
			method = http.MethodGet
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", path, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health: %d", rec.Code)
	}
}
