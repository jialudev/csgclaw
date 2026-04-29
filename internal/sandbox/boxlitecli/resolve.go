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

	for _, exePath := range executablePathCandidates(exe) {
		candidate := filepath.Join(filepath.Dir(exePath), defaultCLIPath)
		if isExecutableFile(candidate) {
			return candidate
		}
	}
	return ""
}

func executablePathCandidates(exe string) []string {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return nil
	}

	candidates := []string{exe}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" && resolved != exe {
		candidates = append(candidates, resolved)
	}
	return candidates
}

func isExecutableFile(candidate string) bool {
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}
