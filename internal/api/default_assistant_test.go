package api

import (
	"context"
	"csgclaw/internal/agentengine"
	"csgclaw/internal/controlplane"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestDefaultAssistantQueryIsReadOnlyAndEnsureIsUnique(t *testing.T) {
	h, client := newControlTestHandler(t)
	if got := controlTestRequest(h, "GET", "/api/v1/default-assistant", "service-test", nil); got.Code != 404 {
		t.Fatalf("query: %d %s", got.Code, got.Body)
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := controlTestRequest(h, "POST", "/api/v1/default-assistant:ensure", "service-test", map[string]any{})
			if got.Code != 200 && got.Code != 202 {
				t.Errorf("ensure: %d %s", got.Code, got.Body)
			}
		}()
	}
	wg.Wait()
	items, err := client.Agents().List(context.Background(), agentengine.AgentListOptions{})
	if err != nil || len(items) != 2 {
		t.Fatalf("expected seed + one assistant: %d %v", len(items), err)
	}
	var binding controlplane.AssistantBinding
	if err = h.control.store.Read(controlplane.DefaultAssistantKey, &binding); err != nil {
		t.Fatal(err)
	}
	// A new handler after a process restart must reuse the persisted target.
	h2 := &Handler{agentEngine: client, serverAccessToken: "service-test"}
	h2.SetControlStore(h.control.store, context.Background())
	got := controlTestRequest(h2, "POST", "/api/v1/default-assistant:ensure", "service-test", map[string]any{})
	var response struct {
		Binding controlplane.AssistantBinding `json:"binding"`
	}
	json.Unmarshal(got.Body.Bytes(), &response)
	if response.Binding.AgentID != binding.AgentID {
		t.Fatal("restart created another target")
	}
	if got = controlTestRequest(h2, "POST", "/api/v1/default-assistant:ensure", "service-test", map[string]any{"runtime": "dsh"}); got.Code != 409 {
		t.Fatalf("binding drift: %d", got.Code)
	}
}
func TestDefaultAssistantRetryRecoversReservedIdentity(t *testing.T) {
	h, _ := newControlTestHandler(t)
	binding := controlplane.AssistantBinding{AgentID: "agent-reserved", OperationID: "old", Runtime: "codex", State: "failed", CreatedAt: time.Now()}
	if err := h.control.store.Write(controlplane.DefaultAssistantKey, binding); err != nil {
		t.Fatal(err)
	}
	got := controlTestRequest(h, "POST", "/api/v1/default-assistant:retry", "service-test", map[string]any{"operation_id": "wrong"})
	if got.Code != 409 {
		t.Fatalf("stale operation: %d", got.Code)
	}
	got = controlTestRequest(h, "POST", "/api/v1/default-assistant:retry", "service-test", map[string]any{"operation_id": "old"})
	if got.Code != 200 && got.Code != 202 {
		t.Fatalf("retry: %d %s", got.Code, got.Body)
	}
	if err := h.control.store.Read(controlplane.DefaultAssistantKey, &binding); err != nil {
		t.Fatal(err)
	}
	if binding.AgentID != "agent-reserved" || binding.OperationID == "old" {
		t.Fatal("retry did not preserve identity")
	}
}
