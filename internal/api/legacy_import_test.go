package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"csgclaw/internal/agentengine"
	"csgclaw/internal/agentsession"
	"csgclaw/internal/legacyimport"
	agentruntime "csgclaw/internal/runtime"
	hub "csgclaw/internal/template"
)

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
func importTestFile(path, content string) legacyimport.File {
	sum := sha256.Sum256([]byte(content))
	return legacyimport.File{Path: path, MediaType: "text/plain", SHA256: hex.EncodeToString(sum[:]), Content: base64.StdEncoding.EncodeToString([]byte(content))}
}
func TestLegacyImportIsRepeatableAndProtectsLaterEdits(t *testing.T) {
	h, client := newControlTestHandler(t)
	root := t.TempDir()
	h.workspace = importTestWorkspace{root: root}
	var err error
	h.sessionBindings, err = agentsession.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := client.Agents().Get(context.Background(), "a", agentengine.AgentGetOptions{})
	request := legacyimport.Request{SourceID: "csgbot", SourceSession: "session1", ExpectedRevision: configurationRevision(a), Configuration: &legacyimport.Configuration{FieldMask: []string{"instructions"}, Spec: agentengine.AgentSpec{Instructions: "legacy instructions"}}, Files: []legacyimport.File{importTestFile("reports/a.txt", "legacy file")}, History: []legacyimport.Message{{ID: "msg1", Role: "assistant", Text: "old history"}}}
	got := controlTestRequest(h, "POST", "/api/v1/agents/a/imports", "service-test", request)
	if got.Code != 201 {
		t.Fatalf("import: %d %s", got.Code, got.Body)
	}
	var record legacyimport.Record
	json.Unmarshal(got.Body.Bytes(), &record)
	fileID := record.Files["reports/a.txt"].FileID
	body, err := os.ReadFile(filepath.Join(root, record.Files["reports/a.txt"].WorkspacePath))
	if err != nil || string(body) != "legacy file" {
		t.Fatalf("workspace import: %q %v", body, err)
	}
	a, _ = client.Agents().Get(context.Background(), "a", agentengine.AgentGetOptions{})
	a.Spec.Instructions = "user changed it"
	if _, err = client.Agents().Update(context.Background(), "a", agentengine.AgentUpdateRequest{Spec: a.Spec, FieldMask: []string{"instructions"}, ResourceVersion: a.ResourceVersion}); err != nil {
		t.Fatal(err)
	}
	got = controlTestRequest(h, "POST", "/api/v1/agents/a/imports", "service-test", request)
	if got.Code != 200 {
		t.Fatalf("retry: %d %s", got.Code, got.Body)
	}
	var repeated legacyimport.Record
	json.Unmarshal(got.Body.Bytes(), &repeated)
	if repeated.Files["reports/a.txt"].FileID != fileID {
		t.Fatal("retry duplicated a file")
	}
	a, _ = client.Agents().Get(context.Background(), "a", agentengine.AgentGetOptions{})
	if a.Spec.Instructions != "user changed it" {
		t.Fatal("reimport overwrote a later edit")
	}
	got = controlTestRequest(h, "GET", "/api/v1/agents/a/sessions/legacy."+record.ID+"/history", "service-test", nil)
	if got.Code != 200 || !json.Valid(got.Body.Bytes()) {
		t.Fatalf("legacy history: %d %s", got.Code, got.Body)
	}
	got = controlTestRequest(h, "POST", "/api/v1/agents/a/sessions/legacy."+record.ID+"/turns", "service-test", map[string]any{"request_id": "new", "input": []agentengine.InputPart{{Kind: agentengine.InputPartText, Text: "hi"}}})
	if got.Code != 409 {
		t.Fatalf("legacy execution allowed: %d %s", got.Code, got.Body)
	}
	got = controlTestRequest(h, "DELETE", "/api/v1/agents/a/files/"+fileID, "service-test", nil)
	if got.Code != 409 {
		t.Fatalf("import reference deleted: %d", got.Code)
	}
	request.Files[0] = importTestFile("reports/a.txt", "changed source")
	got = controlTestRequest(h, "POST", "/api/v1/agents/a/imports", "service-test", request)
	if got.Code != 409 {
		t.Fatalf("source mutated: %d", got.Code)
	}
}
func TestLegacyImportRequiresReviewAfterUncertainApply(t *testing.T) {
	h, client := newControlTestHandler(t)
	a, _ := client.Agents().Get(context.Background(), "a", agentengine.AgentGetOptions{})
	request := legacyimport.Request{SourceID: "csgbot", SourceSession: "s1", ExpectedRevision: configurationRevision(a), Configuration: &legacyimport.Configuration{FieldMask: []string{"instructions"}, Spec: agentengine.AgentSpec{Instructions: "imported"}}}
	id := legacyimport.RecordID(request)
	record := legacyimport.Record{Kind: "legacy_import", ID: id, AgentID: "a", RequestHash: legacyimport.Hash(request), State: "applying", ExpectedRevision: request.ExpectedRevision}
	if err := h.control.store.Write(legacyimport.Key("a", id), record); err != nil {
		t.Fatal(err)
	}
	a.Spec.Instructions = "new edit"
	client.Agents().Update(context.Background(), "a", agentengine.AgentUpdateRequest{Spec: a.Spec, FieldMask: []string{"instructions"}})
	got := controlTestRequest(h, "POST", "/api/v1/agents/a/imports", "service-test", request)
	if got.Code != 409 {
		t.Fatalf("uncertain apply retried: %d %s", got.Code, got.Body)
	}
	current, _ := client.Agents().Get(context.Background(), "a", agentengine.AgentGetOptions{})
	if current.Spec.Instructions != "new edit" {
		t.Fatal("overwrote new configuration")
	}
}
