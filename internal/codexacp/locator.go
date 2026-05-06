package codexacp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"csgclaw/internal/config"
)

const (
	BinaryName = "codex-acp"

	EnvBinaryPath = "CSGCLAW_CODEX_ACP_PATH"
	EnvVersion    = "CSGCLAW_CODEX_ACP_VERSION"
	EnvBaseURL    = "CSGCLAW_CODEX_ACP_BASE_URL"

	DefaultVersion = "0.13.0"
	DefaultBaseURL = "https://github.com/zed-industries/codex-acp/releases/download"
)

type Locator struct {
	ExplicitPath string
	CacheRoot    string
	Version      string
	GOOS         string

	LookPath func(string) (string, error)
	Stat     func(string) (os.FileInfo, error)
}

func (l Locator) Locate() (string, error) {
	explicit := strings.TrimSpace(l.resolvedExplicitPath())
	if explicit != "" {
		path, ok, err := l.executablePath(explicit)
		if err != nil {
			return "", err
		}
		if ok {
			return path, nil
		}
	}
	candidates, err := l.cacheCandidates()
	if err != nil {
		return "", err
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		path, ok, err := l.executablePath(candidate)
		if err != nil {
			return "", err
		}
		if ok {
			return path, nil
		}
	}
	if lookPath := l.lookPath(); lookPath != nil {
		for _, name := range l.executableNames() {
			if path, err := lookPath(name); err == nil {
				resolved, ok, statErr := l.executablePath(path)
				if statErr != nil {
					return "", statErr
				}
				if ok {
					return resolved, nil
				}
			}
		}
	}
	return "", os.ErrNotExist
}

func (l Locator) CachePath() (string, error) {
	root, err := l.cacheRoot()
	if err != nil {
		return "", err
	}
	name := l.binaryName()
	version := l.resolvedVersion()
	if version == "" {
		return filepath.Join(root, name), nil
	}
	return filepath.Join(root, version, name), nil
}

func (l Locator) cacheCandidates() ([]string, error) {
	cachePath, err := l.CachePath()
	if err != nil {
		return nil, err
	}
	root, err := l.cacheRoot()
	if err != nil {
		return nil, err
	}
	return []string{cachePath, filepath.Join(root, l.binaryName()), filepath.Join(root, BinaryName)}, nil
}

func (l Locator) executablePath(path string) (string, bool, error) {
	info, err := l.stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return "", false, nil
	}
	if info.Mode()&0o111 == 0 {
		return "", false, nil
	}
	return path, true, nil
}

func (l Locator) resolvedExplicitPath() string {
	if path := strings.TrimSpace(l.ExplicitPath); path != "" {
		return path
	}
	return strings.TrimSpace(os.Getenv(EnvBinaryPath))
}

func (l Locator) resolvedVersion() string {
	if version := strings.TrimSpace(l.Version); version != "" {
		return version
	}
	if version := strings.TrimSpace(os.Getenv(EnvVersion)); version != "" {
		return version
	}
	return DefaultVersion
}

func (l Locator) cacheRoot() (string, error) {
	if root := strings.TrimSpace(l.CacheRoot); root != "" {
		return root, nil
	}
	dir, err := config.DefaultDir()
	if err != nil {
		return "", fmt.Errorf("resolve csgclaw home: %w", err)
	}
	return filepath.Join(dir, "bin", BinaryName), nil
}

func (l Locator) lookPath() func(string) (string, error) {
	if l.LookPath != nil {
		return l.LookPath
	}
	return exec.LookPath
}

func (l Locator) stat(path string) (os.FileInfo, error) {
	if l.Stat != nil {
		return l.Stat(path)
	}
	return os.Stat(path)
}

func (l Locator) binaryName() string {
	return executableName(l.resolvedGOOS())
}

func (l Locator) executableNames() []string {
	name := l.binaryName()
	if name == BinaryName {
		return []string{name}
	}
	return []string{name, BinaryName}
}

func (l Locator) resolvedGOOS() string {
	if goos := strings.TrimSpace(l.GOOS); goos != "" {
		return goos
	}
	return runtime.GOOS
}

func executableName(goos string) string {
	if strings.TrimSpace(goos) == "windows" {
		return BinaryName + ".exe"
	}
	return BinaryName
}
