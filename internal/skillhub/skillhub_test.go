package skillhub

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListReturnsSkillDirectoriesWithDescriptions(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "alpha", "SKILL.md"), "---\ndescription: First skill\n---\n# Alpha\n")
	mustWriteFile(t, filepath.Join(root, "beta", "SKILL.md"), "# Beta\n")
	mustWriteFile(t, filepath.Join(root, "notes.md"), "ignore")
	if err := os.MkdirAll(filepath.Join(root, "gamma"), 0o755); err != nil {
		t.Fatalf("MkdirAll(gamma) error = %v", err)
	}

	got, err := List(root)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(List()) = %d, want 2", len(got))
	}
	if got[0].Name != "alpha" || got[0].Description != "First skill" {
		t.Fatalf("first skill = %+v, want alpha with description", got[0])
	}
	if got[1].Name != "beta" || got[1].Description != "" {
		t.Fatalf("second skill = %+v, want beta without description", got[1])
	}
}

func TestListParsesBlockScalarDescription(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "alpha", "SKILL.md"), "---\ndescription: >\n  line one\n  line two\n---\n# Alpha\n")

	got, err := List(root)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(got))
	}
	if got[0].Description != "line one line two" {
		t.Fatalf("description = %q, want %q", got[0].Description, "line one line two")
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
