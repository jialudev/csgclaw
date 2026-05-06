package codexacp

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type BinaryProvider interface {
	Ensure(ctx context.Context) (string, error)
}

type Installer struct {
	Locator Locator
	BaseURL string
	GOOS    string
	GOARCH  string
	Client  *http.Client

	Get      func(context.Context, string) (io.ReadCloser, error)
	MkdirAll func(string, os.FileMode) error
	Rename   func(string, string) error
	Remove   func(string) error
	OpenFile func(string, int, os.FileMode) (*os.File, error)
	Chmod    func(string, os.FileMode) error
}

func (i Installer) Ensure(ctx context.Context) (string, error) {
	if path, err := i.Locator.Locate(); err == nil {
		return path, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return i.install(ctx)
}

func (i Installer) install(ctx context.Context) (string, error) {
	targetPath, err := i.Locator.CachePath()
	if err != nil {
		return "", err
	}
	targetDir := filepath.Dir(targetPath)
	if err := i.mkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", targetDir, err)
	}

	url, err := i.downloadURL()
	if err != nil {
		return "", err
	}
	body, err := i.get(ctx, url)
	if err != nil {
		return "", fmt.Errorf("download codex-acp from %s: %w", url, err)
	}
	defer body.Close()

	tmpPath := targetPath + ".tmp"
	_ = i.remove(tmpPath)
	target, err := i.downloadTarget()
	if err != nil {
		return "", err
	}
	if err := extractArchiveBinary(body, target.archiveExt, executableName(target.goos), tmpPath, i.openFile); err != nil {
		_ = i.remove(tmpPath)
		return "", err
	}
	if err := i.chmod(tmpPath, 0o755); err != nil {
		_ = i.remove(tmpPath)
		return "", fmt.Errorf("chmod %s: %w", tmpPath, err)
	}
	if err := i.rename(tmpPath, targetPath); err != nil {
		_ = i.remove(tmpPath)
		return "", fmt.Errorf("rename %s: %w", targetPath, err)
	}
	return targetPath, nil
}

func (i Installer) downloadURL() (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(i.resolvedBaseURL()), "/")
	if baseURL == "" {
		return "", fmt.Errorf("codex-acp base URL is not configured; set %s or specify Installer.BaseURL", EnvBaseURL)
	}
	version := normalizeVersion(i.Locator.resolvedVersion())
	target, err := i.downloadTarget()
	if err != nil {
		return "", err
	}
	archive := fmt.Sprintf("%s-%s-%s.%s", BinaryName, version, target.target, target.archiveExt)
	return fmt.Sprintf("%s/v%s/%s", baseURL, version, archive), nil
}

func (i Installer) downloadTarget() (downloadTarget, error) {
	goos := strings.TrimSpace(i.GOOS)
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := strings.TrimSpace(i.GOARCH)
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	switch goos + "/" + goarch {
	case "darwin/arm64":
		return downloadTarget{goos: goos, target: "aarch64-apple-darwin", archiveExt: "tar.gz"}, nil
	case "darwin/amd64":
		return downloadTarget{goos: goos, target: "x86_64-apple-darwin", archiveExt: "tar.gz"}, nil
	case "linux/amd64":
		return downloadTarget{goos: goos, target: "x86_64-unknown-linux-musl", archiveExt: "tar.gz"}, nil
	case "linux/arm64":
		return downloadTarget{goos: goos, target: "aarch64-unknown-linux-musl", archiveExt: "tar.gz"}, nil
	case "windows/amd64":
		return downloadTarget{goos: goos, target: "x86_64-pc-windows-msvc", archiveExt: "zip"}, nil
	case "windows/arm64":
		return downloadTarget{goos: goos, target: "aarch64-pc-windows-msvc", archiveExt: "zip"}, nil
	default:
		return downloadTarget{}, fmt.Errorf("unsupported codex-acp target %s/%s", goos, goarch)
	}
}

func (i Installer) resolvedBaseURL() string {
	if baseURL := strings.TrimSpace(i.BaseURL); baseURL != "" {
		return baseURL
	}
	if baseURL := strings.TrimSpace(os.Getenv(EnvBaseURL)); baseURL != "" {
		return baseURL
	}
	return DefaultBaseURL
}

func (i Installer) get(ctx context.Context, url string) (io.ReadCloser, error) {
	if i.Get != nil {
		return i.Get(ctx, url)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := i.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return resp.Body, nil
}

func (i Installer) httpClient() *http.Client {
	if i.Client != nil {
		return i.Client
	}
	return http.DefaultClient
}

func (i Installer) mkdirAll(path string, mode os.FileMode) error {
	if i.MkdirAll != nil {
		return i.MkdirAll(path, mode)
	}
	return os.MkdirAll(path, mode)
}

func (i Installer) rename(oldPath, newPath string) error {
	if i.Rename != nil {
		return i.Rename(oldPath, newPath)
	}
	return os.Rename(oldPath, newPath)
}

func (i Installer) remove(path string) error {
	if i.Remove != nil {
		return i.Remove(path)
	}
	return os.Remove(path)
}

func (i Installer) openFile(path string, flag int, mode os.FileMode) (*os.File, error) {
	if i.OpenFile != nil {
		return i.OpenFile(path, flag, mode)
	}
	return os.OpenFile(path, flag, mode)
}

func (i Installer) chmod(path string, mode os.FileMode) error {
	if i.Chmod != nil {
		return i.Chmod(path, mode)
	}
	return os.Chmod(path, mode)
}

type downloadTarget struct {
	goos       string
	target     string
	archiveExt string
}

func normalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

func extractArchiveBinary(src io.Reader, archiveExt, binaryName, targetPath string, openFile func(string, int, os.FileMode) (*os.File, error)) error {
	switch archiveExt {
	case "tar.gz":
		return extractTarGzBinary(src, binaryName, targetPath, openFile)
	case "zip":
		return extractZipBinary(src, binaryName, targetPath, openFile)
	default:
		return fmt.Errorf("unsupported codex-acp archive format %s", archiveExt)
	}
}

func extractTarGzBinary(src io.Reader, binaryName, targetPath string, openFile func(string, int, os.FileMode) (*os.File, error)) error {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("read codex-acp archive: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read codex-acp archive entry: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Base(strings.TrimSpace(hdr.Name))
		if !matchesBinaryName(name, binaryName) {
			continue
		}
		return writeBinary(tr, targetPath, openFile)
	}
	return fmt.Errorf("codex-acp binary not found in archive")
}

func extractZipBinary(src io.Reader, binaryName, targetPath string, openFile func(string, int, os.FileMode) (*os.File, error)) error {
	data, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("read codex-acp archive: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("read codex-acp archive: %w", err)
	}
	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(strings.TrimSpace(file.Name))
		if !matchesBinaryName(name, binaryName) {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open archive entry %s: %w", file.Name, err)
		}
		err = writeBinary(rc, targetPath, openFile)
		_ = rc.Close()
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("codex-acp binary not found in archive")
}

func matchesBinaryName(name, binaryName string) bool {
	return name == binaryName || strings.HasPrefix(name, binaryName+"-")
}

func writeBinary(src io.Reader, targetPath string, openFile func(string, int, os.FileMode) (*os.File, error)) error {
	dst, err := openFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("open %s: %w", targetPath, err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return fmt.Errorf("write %s: %w", targetPath, err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("close %s: %w", targetPath, err)
	}
	return nil
}
