package csghubauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreSaveLoadStatusAndDelete(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "auth", "csghub.json"))
	loggedInAt := time.Date(2026, 6, 22, 8, 30, 0, 0, time.UTC)

	err := store.Save(Record{
		AIGatewayBuiltinAPIKey: " gk_api-key ",
		AccessToken:            " access-token ",
		UserID:                 " alice ",
		UserUUID:               " user-uuid ",
		Avatar:                 " https://example.test/avatar.png ",
		CSGHubBaseURL:          " https://hub.example.test/ ",
		PortalURL:              " https://hub.example.test/welcome ",
		LoggedInAt:             loggedInAt,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	path, err := store.Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat auth store: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Fatalf("auth store mode = %v, want %v", got, want)
	}

	record, ok, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !ok {
		t.Fatal("Load() ok = false, want true")
	}
	if record.AIGatewayBuiltinAPIKey != "gk_api-key" || record.AccessToken != "access-token" {
		t.Fatalf("record secrets not normalized: %+v", record)
	}
	rawStore, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read auth store: %v", err)
	}
	if !strings.Contains(string(rawStore), `"ai_gateway_builtin_api_key"`) {
		t.Fatalf("auth store does not use ai_gateway_builtin_api_key: %s", rawStore)
	}
	if strings.Contains(string(rawStore), `"api_key"`) {
		t.Fatalf("auth store still writes api_key: %s", rawStore)
	}
	if record.CSGHubBaseURL != "https://hub.example.test" {
		t.Fatalf("CSGHubBaseURL = %q", record.CSGHubBaseURL)
	}

	status, err := store.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Authenticated || status.UserID != "alice" || status.UserUUID != "user-uuid" {
		t.Fatalf("status = %+v, want authenticated alice", status)
	}
	raw, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if strings.Contains(string(raw), "access-token") || strings.Contains(string(raw), "gk_api-key") {
		t.Fatalf("status leaks secrets: %s", raw)
	}

	baseURL, token, ok, err := store.Credentials()
	if err != nil {
		t.Fatalf("Credentials() error = %v", err)
	}
	if !ok || baseURL != "https://hub.example.test" || token != "access-token" {
		t.Fatalf("Credentials() = %q, %q, %v", baseURL, token, ok)
	}

	aiBaseURL, aiKey, ok, err := store.AIGatewayCredentials()
	if err != nil {
		t.Fatalf("AIGatewayCredentials() error = %v", err)
	}
	if !ok || aiBaseURL != DefaultAIGatewayBaseURL || aiKey != "gk_api-key" {
		t.Fatalf("AIGatewayCredentials() = %q, %q, %v", aiBaseURL, aiKey, ok)
	}

	if err := store.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	status, err = store.Status()
	if err != nil {
		t.Fatalf("Status() after delete error = %v", err)
	}
	if status.Authenticated {
		t.Fatalf("status after delete = %+v, want unauthenticated", status)
	}
}

func TestStoreSaveRequiresRuntimeCredentials(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "csghub.json"))
	if err := store.Save(Record{CSGHubBaseURL: "https://hub.example.test"}); err == nil {
		t.Fatal("Save() error = nil, want access token error")
	}
	if err := store.Save(Record{AccessToken: "token"}); err == nil {
		t.Fatal("Save() error = nil, want base url error")
	}
}

func TestAIGatewayCredentialsRequiresStoredAPIKey(t *testing.T) {
	t.Setenv("CSGHUB_AIGATEWAY_BASE_URL", "https://gateway.example.test")
	t.Setenv("CSGHUB_AIGATEWAY_URL", "")

	store := NewStore(filepath.Join(t.TempDir(), "csghub.json"))
	if err := store.Save(Record{
		AccessToken:   "access-token",
		CSGHubBaseURL: "https://hub.example.test",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	baseURL, apiKey, ok, err := store.AIGatewayCredentials()
	if err != nil {
		t.Fatalf("AIGatewayCredentials() error = %v", err)
	}
	if ok || baseURL != "https://gateway.example.test/v1" || apiKey != "" {
		t.Fatalf("AIGatewayCredentials() = %q, %q, %v", baseURL, apiKey, ok)
	}
}

func TestEnsureAIGatewayCredentialsFetchesAndCachesBuiltinAPIKey(t *testing.T) {
	t.Setenv("CSGHUB_AIGATEWAY_BASE_URL", "https://gateway.example.test")
	t.Setenv("CSGHUB_AIGATEWAY_URL", "")

	var sawBuiltin bool
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/user-1/apikeys/builtin" {
			t.Fatalf("path = %q, want builtin api key path", r.URL.Path)
		}
		if got := r.URL.Query().Get("current_user"); got != "alice" {
			t.Fatalf("current_user = %q, want alice", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("Authorization = %q, want access token", got)
		}
		sawBuiltin = true
		writeJSON(t, w, map[string]any{
			"msg": "OK",
			"data": map[string]any{
				"token": "gk_builtin",
			},
		})
	}))
	defer hub.Close()

	store := NewStore(filepath.Join(t.TempDir(), "csghub.json"))
	if err := store.Save(Record{
		AIGatewayBuiltinAPIKey: "non-builtin-key",
		AccessToken:            "access-token",
		UserID:                 "alice",
		UserUUID:               "user-1",
		CSGHubBaseURL:          hub.URL,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	baseURL, apiKey, ok, err := store.EnsureAIGatewayCredentials(context.Background(), hub.Client())
	if err != nil {
		t.Fatalf("EnsureAIGatewayCredentials() error = %v", err)
	}
	if !ok || baseURL != "https://gateway.example.test/v1" || apiKey != "gk_builtin" {
		t.Fatalf("EnsureAIGatewayCredentials() = %q, %q, %v", baseURL, apiKey, ok)
	}
	if !sawBuiltin {
		t.Fatal("builtin api key endpoint was not called")
	}
	record, ok, err := store.Load()
	if err != nil || !ok {
		t.Fatalf("Load() = %+v, %v, %v", record, ok, err)
	}
	if record.AIGatewayBuiltinAPIKey != "gk_builtin" {
		t.Fatalf("stored AIGatewayBuiltinAPIKey = %q, want builtin key", record.AIGatewayBuiltinAPIKey)
	}
}
