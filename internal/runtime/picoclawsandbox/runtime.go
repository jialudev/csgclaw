package picoclawsandbox

import (
	"path/filepath"

	agentruntime "csgclaw/internal/runtime"
	"csgclaw/internal/runtime/sandboxgateway"
)

const (
	HostPicoClawDir      = ".picoclaw"
	HostPicoClawConfig   = "config.json"
	HostPicoClawSecurity = ".security.yml"
	HostPicoClawStateDir = ".csgclaw/picoclaw"
	BoxPicoClawDir       = "/home/picoclaw/.picoclaw"
	BoxWorkspaceDir      = BoxPicoClawDir + "/workspace"
	BoxProjectsDir       = "/home/picoclaw/.picoclaw/workspace/projects"
	BoxGatewayLogPath    = BoxWorkspaceDir + "/gateway.log"
)

type AgentRef = sandboxgateway.AgentRef
type Dependencies = sandboxgateway.Dependencies
type Runtime = sandboxgateway.Runtime

func New(deps Dependencies) *Runtime {
	deps.RuntimeKind = agentruntime.KindPicoClawSandbox
	deps.HomeEnv = "/home/picoclaw"
	deps.WorkspaceGuestPath = BoxWorkspaceDir
	deps.ProjectsGuestPath = BoxProjectsDir
	deps.GatewayLogPath = BoxGatewayLogPath
	if deps.GatewayCommand == nil {
		deps.GatewayCommand = GatewayRunCommand
	}
	return sandboxgateway.New(deps)
}

func GatewayRunCommand() string {
	configPath := boxWorkspaceConfigPath(HostPicoClawConfig)
	securityPath := boxWorkspaceConfigPath(HostPicoClawSecurity)
	return "mkdir -p " + BoxPicoClawDir +
		" && cp " + configPath + " " + filepath.Join(BoxPicoClawDir, HostPicoClawConfig) +
		" && cp " + securityPath + " " + filepath.Join(BoxPicoClawDir, HostPicoClawSecurity) +
		" && /usr/local/bin/picoclaw gateway -d 1>" + BoxGatewayLogPath + " 2>/dev/null"
}

func boxWorkspaceConfigPath(name string) string {
	return filepath.Join(BoxWorkspaceDir, filepath.FromSlash(HostPicoClawStateDir), name)
}
