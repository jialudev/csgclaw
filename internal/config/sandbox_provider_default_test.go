package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultSandboxProviderUsesBundledBoxLiteWhenPresent(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "boxlite"), []byte(""), 0o755); err != nil {
		t.Fatalf("WriteFile(boxlite) error = %v", err)
	}

	restore := stubSandboxProviderExecutablePath(t, filepath.Join(binDir, "csgclaw"))
	defer restore()

	if got, want := defaultSandboxProvider(), BoxLiteCLIProvider; got != want {
		t.Fatalf("defaultSandboxProvider() = %q, want %q", got, want)
	}
}

func TestDefaultSandboxProviderFallsBackToDockerWhenBundledBoxLiteIsAbsent(t *testing.T) {
	restore := stubSandboxProviderExecutablePath(t, filepath.Join(t.TempDir(), "bin", "csgclaw"))
	defer restore()

	if got, want := defaultSandboxProvider(), DockerProvider; got != want {
		t.Fatalf("defaultSandboxProvider() = %q, want %q", got, want)
	}
}

func stubSandboxProviderExecutablePath(t *testing.T, path string) func() {
	t.Helper()
	previous := sandboxProviderExecutablePath
	sandboxProviderExecutablePath = func() (string, error) {
		return path, nil
	}
	return func() {
		sandboxProviderExecutablePath = previous
	}
}
