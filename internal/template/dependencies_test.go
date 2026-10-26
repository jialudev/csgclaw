package template

import (
	"csgclaw/internal/apitypes"
	"github.com/pelletier/go-toml/v2"
	"reflect"
	"testing"
)

func TestManifestDependenciesAreOptionalAndRoundTrip(t *testing.T) {
	original := templateManifest{Name: "assistant", RuntimeKind: "codex", Dependencies: []apitypes.RuntimeDependency{{Name: "node", MinVersion: "22.13.0"}, {Name: "python", Optional: true}}}
	raw, err := toml.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded templateManifest
	if err = toml.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Dependencies, original.Dependencies) {
		t.Fatalf("lost dependencies: %s", raw)
	}
	if err = validateManifest(decoded); err != nil {
		t.Fatal(err)
	}
	decoded.Dependencies = nil
	if err = validateManifest(decoded); err != nil {
		t.Fatal("legacy manifest rejected", err)
	}
	decoded.Dependencies = []apitypes.RuntimeDependency{{Name: "install.sh"}}
	if validateManifest(decoded) == nil {
		t.Fatal("arbitrary dependency accepted")
	}
}
