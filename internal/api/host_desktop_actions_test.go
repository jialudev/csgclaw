package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDisabledHostDesktopActionsRejectLoopbackManagementRequests(t *testing.T) {
	h := &Handler{serverAccessToken: "test-token"}
	h.SetHostDesktopActionsDisabled(true)
	req := httptest.NewRequest(http.MethodPost, "http://localhost/workspace/open", strings.NewReader(`{"path":".","target":"system"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	if targets := h.workspaceOpenTargets(req, ".", true); len(targets) != 0 {
		t.Fatalf("host actions advertised: %v", targets)
	}
	rec := httptest.NewRecorder()
	h.handleWorkspaceOpen(rec, req, func(string) (workspaceLocation, error) {
		t.Fatal("disabled host action reached filesystem resolution")
		return workspaceLocation{}, nil
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want forbidden", rec.Code)
	}
}
