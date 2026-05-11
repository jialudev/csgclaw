package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultSandboxProvider remains the canonical bundled-BoxLite provider name
// for explicit configs. Unset providers are resolved dynamically at runtime.
const DefaultSandboxProvider = BoxLiteProvider

var sandboxProviderExecutablePath = os.Executable

func defaultSandboxProvider() string {
	if hasBundledBoxLite() {
		return BoxLiteProvider
	}
	return DockerProvider
}

func hasBundledBoxLite() bool {
	exe, err := sandboxProviderExecutablePath()
	if err != nil {
		return false
	}
	for _, exePath := range executablePathCandidates(exe) {
		dir := filepath.Dir(exePath)
		for _, name := range bundledBoxLiteCandidateNames() {
			candidate := filepath.Join(dir, name)
			if isExecutableFile(candidate) {
				return true
			}
		}
	}
	return false
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

func bundledBoxLiteCandidateNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"boxlite.exe", "boxlite"}
	}
	return []string{"boxlite"}
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}
