package agentengine

import (
	"context"
	agent "csgclaw/internal/agentengine/agents"
	"csgclaw/internal/controlplane"
	"csgclaw/internal/runtime"
	"testing"
)

func TestDefaultAssistantProtectionSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := controlplane.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Write(controlplane.DefaultAssistantKey, controlplane.AssistantBinding{AgentID: "agent-default"}); err != nil {
		t.Fatal(err)
	}
	store.Close()
	adapter := &fakeConversationRuntime{}
	e := New(newTestAgentService(t, []agent.Agent{{ID: "agent-default", Name: "Default", Role: agent.RoleWorker, RuntimeKind: agent.RuntimeKindCodex, RuntimeID: "runtime-default", Status: string(runtime.StateRunning)}}, adapter))
	if err = e.EnablePersistence(dir); err != nil {
		t.Fatal(err)
	}
	if err = e.Agents().Delete(context.Background(), "agent-default"); err == nil {
		t.Fatal("default assistant was deleted")
	}
	if adapter.deleteCalls != 0 {
		t.Fatal("Runtime deletion was attempted")
	}
}
