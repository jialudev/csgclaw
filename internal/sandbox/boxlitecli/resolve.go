package boxlitecli

import (
	"os"
	"path/filepath"
	"strings"
)

const defaultCLIPath = "boxlite"

var executablePath = os.Executable

// ResolvePath chooses the effective boxlite CLI path. Custom paths override
// the bundled binary. The default "boxlite" value falls back to a sibling
// bundled binary first, then PATH resolution.
func ResolvePath(path string) string {
	path = strings.TrimSpace(path)
	if path != "" && path != defaultCLIPath {
		return path
	}
	if bundled := bundledPath(); bundled != "" {
		return bundled
	}
	if path != "" {
		return path
	}
	return defaultCLIPath
}

func bundledPath() string {
	exe, err := executablePath()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(filepath.Dir(exe), defaultCLIPath)
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return ""
	}
	return candidate
}
