package localstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteJSONFileCreatesParentDirAndWritesIndentedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	if err := WriteJSONFile(path, map[string]any{"items": []string{"agent-manager"}}); err != nil {
		t.Fatalf("WriteJSONFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "\n  \"items\": [\n") {
		t.Fatalf("WriteJSONFile() did not write indented JSON: %q", text)
	}
	if !strings.HasSuffix(text, "\n") {
		t.Fatalf("WriteJSONFile() did not append trailing newline: %q", text)
	}
}

func TestReadJSONFileAllowsBlankFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("  \n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	target := map[string]any{"existing": true}
	if err := ReadJSONFile(path, &target); err != nil {
		t.Fatalf("ReadJSONFile() error = %v", err)
	}
	if target["existing"] != true {
		t.Fatalf("ReadJSONFile() changed target for blank file: %#v", target)
	}
}
