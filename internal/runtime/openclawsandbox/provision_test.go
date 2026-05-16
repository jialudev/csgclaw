package openclawsandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"csgclaw/internal/config"
	agentruntime "csgclaw/internal/runtime"
)

func TestProvisionPreparesGatewayAssets(t *testing.T) {
	var (
		configCalls    int
		templateCalls  int
		workspaceCalls int
		projectsCalls  int
	)
	workspaceRoot := t.TempDir()
	overlayRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(overlayRoot, "USER.md"), []byte("overlay user\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(USER.md) error = %v", err)
	}

	rt := New(Dependencies{
		ModelFallback: "fallback-model",
		Server: config.ServerConfig{
			AdvertiseBaseURL: "http://127.0.0.1:18080",
			AccessToken:      "shared-token",
		},
		ResolveBaseURL: func(server config.ServerConfig) string {
			return server.AdvertiseBaseURL
		},
		EnsureGatewayConfig: func(agentName, botID string, profile agentruntime.Profile) error {
			configCalls++
			if got, want := agentName, "alice"; got != want {
				t.Fatalf("agentName = %q, want %q", got, want)
			}
			if got, want := botID, "u-alice"; got != want {
				t.Fatalf("botID = %q, want %q", got, want)
			}
			if got, want := profile.ModelID, "fallback-model"; got != want {
				t.Fatalf("profile.ModelID = %q, want %q", got, want)
			}
			return nil
		},
		WorkspaceTemplate: func(name, botID string) (string, error) {
			templateCalls++
			if got, want := name, "alice"; got != want {
				t.Fatalf("name = %q, want %q", got, want)
			}
			if got, want := botID, "u-alice"; got != want {
				t.Fatalf("botID = %q, want %q", got, want)
			}
			return "template-root", nil
		},
		EnsureWorkspace: func(agentName, template string) (WorkspaceLayout, error) {
			workspaceCalls++
			if got, want := agentName, "alice"; got != want {
				t.Fatalf("agentName = %q, want %q", got, want)
			}
			if got, want := template, "template-root"; got != want {
				t.Fatalf("template = %q, want %q", got, want)
			}
			return WorkspaceLayout{MountHostPath: workspaceRoot, WorkspaceHostPath: workspaceRoot}, nil
		},
		EnsureProjectsRoot: func() (string, error) {
			projectsCalls++
			return t.TempDir(), nil
		},
	})

	if err := rt.Provision(context.Background(), agentruntime.ProvisionRequest{
		RuntimeID:        "rt-1",
		AgentID:          "u-alice",
		AgentName:        "alice",
		Profile:          agentruntime.Profile{},
		WorkspaceOverlay: overlayRoot,
	}); err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	if configCalls != 1 {
		t.Fatalf("EnsureGatewayConfig calls = %d, want 1", configCalls)
	}
	if templateCalls != 1 {
		t.Fatalf("WorkspaceTemplate calls = %d, want 1", templateCalls)
	}
	if workspaceCalls != 1 {
		t.Fatalf("EnsureWorkspace calls = %d, want 1", workspaceCalls)
	}
	if projectsCalls != 1 {
		t.Fatalf("EnsureProjectsRoot calls = %d, want 1", projectsCalls)
	}
	data, err := os.ReadFile(filepath.Join(workspaceRoot, "USER.md"))
	if err != nil {
		t.Fatalf("ReadFile(USER.md) error = %v", err)
	}
	if got, want := string(data), "overlay user\n"; got != want {
		t.Fatalf("USER.md = %q, want %q", got, want)
	}
}
