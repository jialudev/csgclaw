package picoclawsandbox

import (
	"context"
	"path"

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
type WorkspaceLayout = sandboxgateway.WorkspaceLayout

type Runtime struct {
	*sandboxgateway.Runtime
	deps Dependencies
}

var _ agentruntime.Provisioner = (*Runtime)(nil)

func New(deps Dependencies) *Runtime {
	deps.RuntimeKind = agentruntime.KindPicoClawSandbox
	deps.HomeEnv = "/home/picoclaw"
	deps.MountGuestPath = BoxWorkspaceDir
	deps.WorkspaceGuestPath = BoxWorkspaceDir
	deps.ProjectsGuestPath = BoxProjectsDir
	deps.GatewayLogPath = BoxGatewayLogPath
	if deps.GatewayCommand == nil {
		deps.GatewayCommand = GatewayRunCommand
	}
	return &Runtime{
		Runtime: sandboxgateway.New(deps),
		deps:    deps,
	}
}

func (r *Runtime) Provision(_ context.Context, req agentruntime.ProvisionRequest) error {
	if r == nil {
		return nil
	}
	_, err := sandboxgateway.PrepareGatewayProvision(r.deps, req)
	return err
}

func GatewayRunCommand() string {
	// Use path (not filepath): this string runs inside the Linux container's /bin/sh.
	// On Windows hosts filepath.Join emits '\' which breaks the shell and mangles cp paths.
	configPath := boxWorkspaceConfigPath(HostPicoClawConfig)
	securityPath := boxWorkspaceConfigPath(HostPicoClawSecurity)
	return "mkdir -p " + BoxPicoClawDir +
		" && cp " + configPath + " " + path.Join(BoxPicoClawDir, HostPicoClawConfig) +
		" && cp " + securityPath + " " + path.Join(BoxPicoClawDir, HostPicoClawSecurity) +
		" && /usr/local/bin/picoclaw gateway -d 1>" + BoxGatewayLogPath + " 2>/dev/null"
}

func boxWorkspaceConfigPath(name string) string {
	return path.Join(BoxWorkspaceDir, HostPicoClawStateDir, name)
}
