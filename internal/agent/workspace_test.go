package agent

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureWorkspaceAtRootOverlay(t *testing.T) {
	hostRoot := t.TempDir()

	gotRoot, err := ensureWorkspaceAtRoot(hostRoot, workspaceTemplateWorkerPicoclaw)
	if err != nil {
		t.Fatalf("ensureWorkspaceAtRoot() error = %v", err)
	}
	if gotRoot != hostRoot {
		t.Fatalf("ensureWorkspaceAtRoot() root = %q, want %q", gotRoot, hostRoot)
	}

	baseUserPath := filepath.Join(hostRoot, "USER.md")
	baseUserData, err := os.ReadFile(baseUserPath)
	if err != nil {
		t.Fatalf("ReadFile(USER.md) error = %v", err)
	}
	if len(baseUserData) == 0 {
		t.Fatalf("USER.md should be populated by the base workspace template")
	}

	overlayRoot := filepath.Join(t.TempDir(), "overlay")
	writeWorkspaceFileAt(t, overlayRoot, "USER.md", "custom user\n", 0o644)
	writeWorkspaceFileAt(t, overlayRoot, "skills/frontend.sh", "#!/bin/sh\necho ready\n", 0o755)

	if err := overlayWorkspaceTree(overlayRoot, hostRoot); err != nil {
		t.Fatalf("overlayWorkspaceTree() error = %v", err)
	}

	userData, err := os.ReadFile(baseUserPath)
	if err != nil {
		t.Fatalf("ReadFile(USER.md) after overlay error = %v", err)
	}
	if got, want := string(userData), "custom user\n"; got != want {
		t.Fatalf("USER.md contents = %q, want %q", got, want)
	}

	if _, err := os.Stat(filepath.Join(hostRoot, "SOUL.md")); err != nil {
		t.Fatalf("Stat(SOUL.md) error = %v", err)
	}

	skillPath := filepath.Join(hostRoot, "skills", "frontend.sh")
	skillData, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(skills/frontend.sh) error = %v", err)
	}
	if got, want := string(skillData), "#!/bin/sh\necho ready\n"; got != want {
		t.Fatalf("skills/frontend.sh contents = %q, want %q", got, want)
	}
	info, err := os.Stat(skillPath)
	if err != nil {
		t.Fatalf("Stat(skills/frontend.sh) error = %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("skills/frontend.sh mode = %o, want executable bit preserved", info.Mode().Perm())
	}
}

func TestOverlayWorkspaceTreeRejectsEmptyWorkspace(t *testing.T) {
	srcRoot := t.TempDir()

	err := overlayWorkspaceTree(srcRoot, t.TempDir())
	if !errors.Is(err, ErrWorkspaceEmpty) {
		t.Fatalf("overlayWorkspaceTree() error = %v, want ErrWorkspaceEmpty", err)
	}
}

func TestOverlayWorkspaceTreeRejectsSymlinks(t *testing.T) {
	srcRoot := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(target, []byte("outside"), 0o644); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.Symlink(target, filepath.Join(srcRoot, "outside.txt")); err != nil {
		t.Skipf("Symlink() unsupported: %v", err)
	}

	err := overlayWorkspaceTree(srcRoot, t.TempDir())
	if !errors.Is(err, ErrWorkspaceSymlinkDenied) {
		t.Fatalf("overlayWorkspaceTree() error = %v, want ErrWorkspaceSymlinkDenied", err)
	}
}

func TestValidateWorkspaceRelativePath(t *testing.T) {
	tests := []struct {
		name    string
		rel     string
		wantErr bool
	}{
		{name: "simple file", rel: "AGENTS.md"},
		{name: "nested file", rel: filepath.Join("skills", "frontend.txt")},
		{name: "empty", rel: "", wantErr: true},
		{name: "dot dot", rel: "..", wantErr: true},
		{name: "parent traversal", rel: filepath.Join("..", "escape"), wantErr: true},
		{name: "embedded traversal", rel: "skills/../escape", wantErr: true},
		{name: "absolute", rel: string(filepath.Separator) + "tmp", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorkspaceRelativePath(tt.rel)
			if tt.wantErr {
				if !errors.Is(err, ErrWorkspacePathUnsafe) {
					t.Fatalf("validateWorkspaceRelativePath(%q) error = %v, want ErrWorkspacePathUnsafe", tt.rel, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateWorkspaceRelativePath(%q) error = %v", tt.rel, err)
			}
		})
	}
}

func writeWorkspaceFileAt(t *testing.T, root, relPath, contents string, mode os.FileMode) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(fullPath), err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), mode); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", fullPath, err)
	}
}
