package boxlitecli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathUsesCustomOverride(t *testing.T) {
	restore := stubExecutablePath(t, filepath.Join(t.TempDir(), "bin", "csgclaw"))
	defer restore()

	if got, want := ResolvePath("/opt/boxlite/bin/boxlite"), "/opt/boxlite/bin/boxlite"; got != want {
		t.Fatalf("ResolvePath(custom) = %q, want %q", got, want)
	}
}

func TestResolvePathUsesBundledBinaryForDefaultValue(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	bundled := filepath.Join(binDir, defaultCLIPath)
	if err := os.WriteFile(bundled, []byte(""), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	restore := stubExecutablePath(t, filepath.Join(binDir, "csgclaw"))
	defer restore()

	if got, want := ResolvePath(defaultCLIPath), bundled; got != want {
		t.Fatalf("ResolvePath(default) = %q, want %q", got, want)
	}
}

func TestResolvePathUsesBundledBinaryWhenUnset(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	bundled := filepath.Join(binDir, defaultCLIPath)
	if err := os.WriteFile(bundled, []byte(""), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	restore := stubExecutablePath(t, filepath.Join(binDir, "csgclaw"))
	defer restore()

	if got, want := ResolvePath(""), bundled; got != want {
		t.Fatalf("ResolvePath(\"\") = %q, want %q", got, want)
	}
}

func TestResolvePathFallsBackToPATHValue(t *testing.T) {
	restore := stubExecutablePath(t, filepath.Join(t.TempDir(), "bin", "csgclaw"))
	defer restore()

	if got, want := ResolvePath(defaultCLIPath), defaultCLIPath; got != want {
		t.Fatalf("ResolvePath(default) = %q, want %q", got, want)
	}
	if got, want := ResolvePath(""), defaultCLIPath; got != want {
		t.Fatalf("ResolvePath(\"\") = %q, want %q", got, want)
	}
}

func stubExecutablePath(t *testing.T, path string) func() {
	t.Helper()
	previous := executablePath
	executablePath = func() (string, error) {
		return path, nil
	}
	return func() {
		executablePath = previous
	}
}
