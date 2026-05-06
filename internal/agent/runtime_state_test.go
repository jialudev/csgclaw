package agent

import (
	"path/filepath"
	"testing"

	"csgclaw/internal/config"
)

func TestPicoClawRuntimeHostAgentHomeUsesAgentRoot(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	svc, err := NewService(config.ModelConfig{}, config.ServerConfig{}, "", "")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	host := svc.PicoClawRuntimeHost()
	got, err := host.AgentHome("alice")
	if err != nil {
		t.Fatalf("host.AgentHome() error = %v", err)
	}

	want := filepath.Join(homeDir, config.AppDirName, managerAgentsDirName, "alice")
	if got != want {
		t.Fatalf("host.AgentHome() = %q, want %q", got, want)
	}
}
