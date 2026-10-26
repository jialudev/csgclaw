package agents

import (
	"csgclaw/internal/apitypes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestAgentDependenciesSurviveStoreRoundTrip(t *testing.T) {
	original := Agent{ID: "agent-a", Name: "A", Role: RoleWorker, RuntimeName: RuntimeNameCodex, RuntimeKind: RuntimeKindCodex, Dependencies: []apitypes.RuntimeDependency{{Name: "python", MinVersion: "3.10"}}}
	raw, err := json.Marshal(newPersistedAgent(original))
	if err != nil {
		t.Fatal(err)
	}
	var stored persistedAgent
	if err = json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	loaded := stored.toAgent()
	if !reflect.DeepEqual(loaded.Dependencies, original.Dependencies) {
		t.Fatalf("dependencies lost: %s", raw)
	}
	clone := cloneAgent(&loaded)
	clone.Dependencies[0].Name = "node"
	if loaded.Dependencies[0].Name != "python" {
		t.Fatal("shared dependency slice")
	}
}
