package codexacp

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocatorLocateUsesExplicitPathFirst(t *testing.T) {
	dir := t.TempDir()
	explicit := writeExecutable(t, filepath.Join(dir, "custom-codex-acp"), "#!/bin/sh\n")
	cached := writeExecutable(t, filepath.Join(dir, "cache", DefaultVersion, BinaryName), "#!/bin/sh\n")

	locator := Locator{
		ExplicitPath: explicit,
		CacheRoot:    filepath.Join(dir, "cache"),
		Version:      DefaultVersion,
		LookPath: func(string) (string, error) {
			t.Fatal("LookPath should not be called when explicit path exists")
			return "", nil
		},
	}

	got, err := locator.Locate()
	if err != nil {
		t.Fatalf("Locate() error = %v", err)
	}
	if got != explicit {
		t.Fatalf("Locate() = %q, want %q; cached=%q", got, explicit, cached)
	}
}

func TestInstallerEnsureInstallsWhenCacheMissing(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := filepath.Join(dir, "cache")
	var requests int
	installer := Installer{
		Locator: Locator{
			CacheRoot: cacheRoot,
			Version:   DefaultVersion,
			LookPath: func(string) (string, error) {
				return "", os.ErrNotExist
			},
		},
		BaseURL: "https://example.invalid/codex-acp",
		GOOS:    "darwin",
		GOARCH:  "arm64",
		Get: func(_ context.Context, url string) (io.ReadCloser, error) {
			requests++
			wantURL := "https://example.invalid/codex-acp/v" + DefaultVersion + "/codex-acp-" + DefaultVersion + "-aarch64-apple-darwin.tar.gz"
			if url != wantURL {
				t.Fatalf("download url = %q, want %q", url, wantURL)
			}
			return io.NopCloser(bytes.NewReader(tarballBytes(t, "codex-acp-aarch64-apple-darwin", "binary-data"))), nil
		},
	}

	path, err := installer.Ensure(context.Background())
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("download requests = %d, want 1", requests)
	}
	if got, want := path, filepath.Join(cacheRoot, DefaultVersion, BinaryName); got != want {
		t.Fatalf("Ensure() path = %q, want %q", got, want)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(raw) != "binary-data" {
		t.Fatalf("binary contents = %q, want %q", string(raw), "binary-data")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("mode = %v, want executable", info.Mode())
	}

	second, err := installer.Ensure(context.Background())
	if err != nil {
		t.Fatalf("second Ensure() error = %v", err)
	}
	if second != path {
		t.Fatalf("second Ensure() = %q, want %q", second, path)
	}
	if requests != 1 {
		t.Fatalf("download requests after second Ensure = %d, want still 1", requests)
	}
}

func TestInstallerEnsureUsesPathBinaryWithoutDownload(t *testing.T) {
	dir := t.TempDir()
	pathBinary := writeExecutable(t, filepath.Join(dir, "bin", BinaryName), "#!/bin/sh\n")
	var requests int

	installer := Installer{
		Locator: Locator{
			CacheRoot: filepath.Join(dir, "cache"),
			Version:   DefaultVersion,
			LookPath: func(string) (string, error) {
				return pathBinary, nil
			},
		},
		Get: func(context.Context, string) (io.ReadCloser, error) {
			requests++
			return nil, nil
		},
	}

	got, err := installer.Ensure(context.Background())
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if got != pathBinary {
		t.Fatalf("Ensure() = %q, want %q", got, pathBinary)
	}
	if requests != 0 {
		t.Fatalf("download requests = %d, want 0", requests)
	}
}

func TestInstallerTargetSuffixRejectsUnsupportedPlatform(t *testing.T) {
	installer := Installer{
		Locator: Locator{Version: DefaultVersion},
		GOOS:    "windows",
		GOARCH:  "386",
	}
	if _, err := installer.downloadURL(); err == nil {
		t.Fatal("downloadURL() error = nil, want unsupported target error")
	}
}

func TestInstallerDownloadURLUsesWindowsZipArtifact(t *testing.T) {
	installer := Installer{
		Locator: Locator{Version: DefaultVersion},
		BaseURL: "https://example.invalid/codex-acp",
		GOOS:    "windows",
		GOARCH:  "amd64",
	}

	got, err := installer.downloadURL()
	if err != nil {
		t.Fatalf("downloadURL() error = %v", err)
	}

	want := "https://example.invalid/codex-acp/v" + DefaultVersion + "/codex-acp-" + DefaultVersion + "-x86_64-pc-windows-msvc.zip"
	if got != want {
		t.Fatalf("downloadURL() = %q, want %q", got, want)
	}
}

func TestInstallerEnsureInstallsWindowsZipWhenCacheMissing(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := filepath.Join(dir, "cache")
	installer := Installer{
		Locator: Locator{
			CacheRoot: cacheRoot,
			Version:   DefaultVersion,
			GOOS:      "windows",
			LookPath: func(string) (string, error) {
				return "", os.ErrNotExist
			},
		},
		BaseURL: "https://example.invalid/codex-acp",
		GOOS:    "windows",
		GOARCH:  "amd64",
		Get: func(_ context.Context, url string) (io.ReadCloser, error) {
			wantURL := "https://example.invalid/codex-acp/v" + DefaultVersion + "/codex-acp-" + DefaultVersion + "-x86_64-pc-windows-msvc.zip"
			if url != wantURL {
				t.Fatalf("download url = %q, want %q", url, wantURL)
			}
			return io.NopCloser(bytes.NewReader(zipBytes(t, "codex-acp.exe", "windows-binary"))), nil
		},
	}

	path, err := installer.Ensure(context.Background())
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}

	wantPath := filepath.Join(cacheRoot, DefaultVersion, "codex-acp.exe")
	if path != wantPath {
		t.Fatalf("Ensure() path = %q, want %q", path, wantPath)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(raw) != "windows-binary" {
		t.Fatalf("binary contents = %q, want %q", string(raw), "windows-binary")
	}
}

func writeExecutable(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}

func writeTarball(t *testing.T, w io.Writer, name, content string) {
	t.Helper()
	gzw := gzip.NewWriter(w)
	tw := tar.NewWriter(gzw)
	hdr := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if _, err := io.WriteString(tw, content); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close error = %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("gzip close error = %v", err)
	}
}

func tarballBytes(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writeTarball(t, &buf, name, content)
	return buf.Bytes()
}

func zipBytes(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("Create(%q) error = %v", name, err)
	}
	if _, err := io.WriteString(w, content); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close error = %v", err)
	}
	return buf.Bytes()
}
