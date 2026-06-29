package openclawsandbox

import (
	"strings"
	"testing"

	"csgclaw/internal/channel/feishu"
	"csgclaw/internal/config"
	agentruntime "csgclaw/internal/runtime"
	"csgclaw/internal/runtime/sandboxgateway"
)

func TestGatewayRunCommandWritesGatewayOutputToSingleLog(t *testing.T) {
	cmd := GatewayRunCommand()

	if !strings.Contains(cmd, "exec node /app/openclaw.mjs gateway ") {
		t.Fatalf("GatewayRunCommand() = %q, want Node to launch OpenClaw", cmd)
	}
	if !strings.Contains(cmd, "1>"+BoxGatewayLogPath) {
		t.Fatalf("GatewayRunCommand() = %q, want stdout written to gateway log", cmd)
	}
	if !strings.Contains(cmd, " 2>&1") {
		t.Fatalf("GatewayRunCommand() = %q, want stderr written to gateway log", cmd)
	}
	if !strings.Contains(cmd, "mkdir -p "+BoxStateDir) {
		t.Fatalf("GatewayRunCommand() = %q, want OpenClaw state dir prepared", cmd)
	}
	if !strings.Contains(cmd, "cp "+BoxDir+"/"+HostExecApproval+" "+BoxStateDir+"/"+HostExecApproval) {
		t.Fatalf("GatewayRunCommand() = %q, want exec approvals copied into OpenClaw state dir", cmd)
	}
	if strings.Contains(cmd, "gateway.err.pipe") || strings.Contains(cmd, "mkfifo") || strings.Contains(cmd, "tee ") {
		t.Fatalf("GatewayRunCommand() = %q, want direct logging without stderr pipe", cmd)
	}
	if strings.Contains(cmd, "gateway stop") || strings.Contains(cmd, "sleep ") {
		t.Fatalf("GatewayRunCommand() = %q, want first-start command without pre-stop delay", cmd)
	}
}

func TestGatewayCreateSpecPinsOpenClawRuntimeStateOffMountedHome(t *testing.T) {
	rt := New(Dependencies{
		BuildRuntimeEnv: func(string, string, string, string, string, string, feishu.AgentCredentialProvider) map[string]string {
			return map[string]string{}
		},
		AddProfileEnv: func(envVars map[string]string, profileEnv map[string]string) {
			for key, value := range profileEnv {
				envVars[key] = value
			}
		},
	})
	rt.RememberPreparedGatewayProvision("u-alice", sandboxgateway.PreparedGatewayProvision{
		AgentID:       "u-alice",
		ParticipantID: "u-alice",
		ModelID:       "model",
		Profile: agentruntime.Profile{
			ModelID: "model",
			Env: map[string]string{
				"OPENCLAW_HOME":        "/home/node/.openclaw",
				"OPENCLAW_STATE_DIR":   "/home/node/.openclaw",
				"OPENCLAW_CONFIG_PATH": "/bad/openclaw.json",
			},
		},
		WorkspaceLayout: WorkspaceLayout{
			MountHostPath:      "/host/agent/.openclaw",
			MountGuestPath:     BoxDir,
			WorkspaceHostPath:  "/host/agent/.openclaw/workspace",
			WorkspaceGuestPath: BoxWorkspaceDir,
		},
		ProjectsRoot:   "/host/projects",
		ManagerBaseURL: "http://127.0.0.1:18080",
		Server:         config.ServerConfig{AccessToken: "token"},
	})

	spec, err := rt.GatewayCreateSpec("openclaw:test", "alice", "u-alice", agentruntime.Profile{})
	if err != nil {
		t.Fatalf("GatewayCreateSpec() error = %v", err)
	}
	if got, want := spec.Env["HOME"], BoxUserHome; got != want {
		t.Fatalf("GatewayCreateSpec() HOME = %q, want %q", got, want)
	}
	if got, want := spec.Env["OPENCLAW_HOME"], BoxRuntimeHome; got != want {
		t.Fatalf("GatewayCreateSpec() OPENCLAW_HOME = %q, want %q", got, want)
	}
	if got, want := spec.Env["OPENCLAW_STATE_DIR"], BoxStateDir; got != want {
		t.Fatalf("GatewayCreateSpec() OPENCLAW_STATE_DIR = %q, want %q", got, want)
	}
	if got, want := spec.Env["OPENCLAW_CONFIG_PATH"], BoxConfigPath; got != want {
		t.Fatalf("GatewayCreateSpec() OPENCLAW_CONFIG_PATH = %q, want %q", got, want)
	}
}
