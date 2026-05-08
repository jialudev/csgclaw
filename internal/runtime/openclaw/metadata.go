package openclaw

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	agentruntime "csgclaw/internal/runtime"
)

type runtimeMetadata struct {
	RuntimeID    string             `json:"runtime_id"`
	AgentID      string             `json:"agent_id"`
	AgentName    string             `json:"agent_name"`
	ProcessID    int                `json:"process_id,omitempty"`
	GatewayPort  int                `json:"gateway_port,omitempty"`
	InstallRoot  string             `json:"install_root,omitempty"`
	OpenClawRoot string             `json:"openclaw_root,omitempty"`
	WorkspaceDir string             `json:"workspace_dir,omitempty"`
	LogPath      string             `json:"log_path,omitempty"`
	State        agentruntime.State `json:"state,omitempty"`
	CreatedAt    time.Time          `json:"created_at,omitempty"`
	StoppedAt    time.Time          `json:"stopped_at,omitempty"`
}

func normalizeRuntimeMetadata(meta runtimeMetadata) runtimeMetadata {
	meta.RuntimeID = strings.TrimSpace(meta.RuntimeID)
	meta.AgentID = strings.TrimSpace(meta.AgentID)
	meta.AgentName = strings.TrimSpace(meta.AgentName)
	meta.InstallRoot = strings.TrimSpace(meta.InstallRoot)
	meta.OpenClawRoot = strings.TrimSpace(meta.OpenClawRoot)
	meta.WorkspaceDir = strings.TrimSpace(meta.WorkspaceDir)
	meta.LogPath = strings.TrimSpace(meta.LogPath)
	meta.State = agentruntime.State(strings.TrimSpace(string(meta.State)))
	if meta.State == "" {
		meta.State = agentruntime.StateUnknown
	}
	if !meta.CreatedAt.IsZero() {
		meta.CreatedAt = meta.CreatedAt.UTC()
	}
	if !meta.StoppedAt.IsZero() {
		meta.StoppedAt = meta.StoppedAt.UTC()
	}
	return meta
}

func readJSONFile(read func(string) ([]byte, error), path string, v any) error {
	data, err := read(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func writeJSONFile(write func(string, []byte, os.FileMode) error, path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := write(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func terminateProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return err
		}
	}
	return nil
}

func nodeVersionAtLeast(version string, major, minor int) bool {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) < 2 {
		return false
	}
	gotMajor, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	gotMinor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	if gotMajor != major {
		return gotMajor > major
	}
	return gotMinor >= minor
}
