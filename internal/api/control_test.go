package api

import (
	"bytes"
	"context"
	agentruntime "csgclaw/internal/runtime"
	hub "csgclaw/internal/template"
	"encoding/json"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"csgclaw/internal/agentengine"
	"csgclaw/internal/agentengine/enginetest"
	"csgclaw/internal/controlplane"
)

func newControlTestHandler(t *testing.T) (*Handler, *enginetest.MemoryClient) {
	t.Helper()
	client := enginetest.NewMemoryClient(agentengine.Agent{ID: "a", ResourceVersion: "one", Spec: agentengine.AgentSpec{Name: "Assistant", Role: agentengine.AgentRoleWorker, Runtime: agentengine.RuntimeSpec{Adapter: "codex", Credentials: map[string]string{"auth": "hidden"}}, Model: agentengine.ModelSpec{APIKey: "hidden"}}, Status: agentengine.AgentStatus{Ready: true}})
	s, err := controlplane.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	h := &Handler{agentEngine: client, serverAccessToken: "service-test"}
	h.SetControlStore(s, context.Background())
	return h, client
}
func controlTestRequest(h *Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	return rec
}
func TestControlConfigurationConflictAndFieldBoundary(t *testing.T) {
	h, client := newControlTestHandler(t)
	path := "/api/v1/agents/a/configuration"
	if got := controlTestRequest(h, "GET", path, "", nil); got.Code != 401 {
		t.Fatalf("unauthorized: %d", got.Code)
	}
	got := controlTestRequest(h, "GET", path, "service-test", nil)
	if got.Code != 200 || strings.Contains(got.Body.String(), "hidden") {
		t.Fatalf("projection: %d %s", got.Code, got.Body)
	}
	var view struct {
		Revision string `json:"desired_revision"`
	}
	json.Unmarshal(got.Body.Bytes(), &view)
	body := map[string]any{"expected_revision": view.Revision, "field_mask": []string{"instructions"}, "spec": map[string]any{"instructions": "new instructions"}}
	if got = controlTestRequest(h, "PATCH", path, "service-test", body); got.Code != 200 {
		t.Fatalf("patch: %d %s", got.Code, got.Body)
	}
	if got = controlTestRequest(h, "PATCH", path, "service-test", body); got.Code != 409 {
		t.Fatalf("stale: %d %s", got.Code, got.Body)
	}
	a, _ := client.Agents().Get(context.Background(), "a", agentengine.AgentGetOptions{})
	if a.Spec.Instructions != "new instructions" || a.Spec.Name != "Assistant" {
		t.Fatalf("mask: %+v", a.Spec)
	}
	got = controlTestRequest(h, "GET", path, "service-test", nil)
	json.Unmarshal(got.Body.Bytes(), &view)
	body["expected_revision"] = view.Revision
	body["field_mask"] = []string{"runtime.credentials"}
	if got = controlTestRequest(h, "PATCH", path, "service-test", body); got.Code != 422 {
		t.Fatalf("protected field: %d", got.Code)
	}
}
func TestControlFilesRemainAgentScoped(t *testing.T) {
	h, _ := newControlTestHandler(t)
	req := httptest.NewRequest("POST", "/api/v1/agents/a/files?name=hello.txt", strings.NewReader("hello"))
	req.Header.Set("Authorization", "Bearer service-test")
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	var file agentengine.OutputFile
	json.Unmarshal(rec.Body.Bytes(), &file)
	got := controlTestRequest(h, "GET", "/api/v1/agents/a/files/"+file.ID, "service-test", nil)
	if got.Code != 200 || got.Body.String() != "hello" {
		t.Fatalf("download: %d %s", got.Code, got.Body)
	}
	got = controlTestRequest(h, "GET", "/api/v1/agents/other/files/"+file.ID, "service-test", nil)
	if got.Code != 404 {
		t.Fatalf("cross-agent: %d", got.Code)
	}
}

func TestLegacyEditorIfMatchProtectsSharedConfiguration(t *testing.T) {
	h, client := newControlTestHandler(t)
	a, _ := client.Agents().Get(context.Background(), "a", agentengine.AgentGetOptions{})
	original := configurationRevision(a)
	send := func(revision, text string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(map[string]any{"instructions": text, "field_mask": []string{"instructions"}})
		req := httptest.NewRequest("PATCH", "/api/v1/agents/a", bytes.NewReader(raw))
		req.Header.Set("If-Match", revision)
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		return rec
	}
	if got := send(original, "first UI"); got.Code != 200 {
		t.Fatalf("initial save: %d %s", got.Code, got.Body)
	}
	if got := send(original, "stale UI"); got.Code != 409 {
		t.Fatalf("stale save: %d %s", got.Code, got.Body)
	}
	current, _ := client.Agents().Get(context.Background(), "a", agentengine.AgentGetOptions{})
	if current.Spec.Instructions != "first UI" {
		t.Fatal("stale editor overwrote the first UI")
	}
}
func TestWorkspaceWritesUseContentPreconditions(t *testing.T) {
	h, _ := newControlTestHandler(t)
	root := t.TempDir()
	h.workspace = importTestWorkspace{root: root}
	send := func(revision, content string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PUT", "/api/v1/agents/a/workspace/file?path=report.txt", strings.NewReader(content))
		req.Header.Set("Authorization", "Bearer service-test")
		req.Header.Set("If-Match", revision)
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		return rec
	}
	first := send("*", "first")
	if first.Code != 200 || first.Header().Get("ETag") == "" {
		t.Fatalf("create: %d %s", first.Code, first.Body)
	}
	second := send(first.Header().Get("ETag"), "second")
	if second.Code != 200 {
		t.Fatalf("update: %d %s", second.Code, second.Body)
	}
	stale := send(first.Header().Get("ETag"), "stale")
	if stale.Code != 409 {
		t.Fatalf("stale: %d %s", stale.Code, stale.Body)
	}
}

func TestConfigurationKeepsLastAppliedRevisionWhilePending(t *testing.T) {
	h, _ := newControlTestHandler(t)
	first := controlTestRequest(h, "GET", "/api/v1/agents/a/configuration", "service-test", nil)
	var before struct {
		Applied string `json:"applied_revision"`
	}
	json.Unmarshal(first.Body.Bytes(), &before)
	if before.Applied == "" {
		t.Fatal("initial applied version is missing")
	}
	h.agentEngine = enginetest.NewMemoryClient(agentengine.Agent{ID: "a", ResourceVersion: "changed", Spec: agentengine.AgentSpec{Name: "Assistant", Role: agentengine.AgentRoleWorker, Runtime: agentengine.RuntimeSpec{Adapter: "codex"}, Instructions: "changed"}, Status: agentengine.AgentStatus{State: agentengine.AgentStateStarting}})
	pending := controlTestRequest(h, "GET", "/api/v1/agents/a/configuration", "service-test", nil)
	var after struct {
		Applied string `json:"applied_revision"`
		Desired string `json:"desired_revision"`
		State   string `json:"state"`
	}
	json.Unmarshal(pending.Body.Bytes(), &after)
	if after.Applied != before.Applied || after.Desired == after.Applied || after.State != "applying" {
		t.Fatalf("pending state: %s", pending.Body)
	}
}

type importTestWorkspace struct{ root string }

func (w importTestWorkspace) AgentLayout(string) (agentruntime.Layout, error) {
	return agentruntime.Layout{WorkspaceRoot: w.root}, nil
}
func (w importTestWorkspace) SkillsRoot(string) (string, error) {
	return filepath.Join(w.root, "skills"), nil
}
func (w importTestWorkspace) WorkspaceRoot(string) (string, error)     { return w.root, nil }
func (w importTestWorkspace) WorkspaceRootByID(string) (string, error) { return w.root, nil }
func (w importTestWorkspace) HubPublishSpec(string, bool) (hub.PublishSpec, error) {
	return hub.PublishSpec{}, nil
}
func (w importTestWorkspace) StreamLogs(context.Context, string, bool, int, io.Writer) error {
	return nil
}
