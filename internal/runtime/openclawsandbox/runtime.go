package openclawsandbox

import (
	"context"

	agentruntime "csgclaw/internal/runtime"
	"csgclaw/internal/runtime/sandboxgateway"
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
	deps.RuntimeKind = agentruntime.KindOpenClawSandbox
	deps.HomeEnv = BoxUserHome
	deps.MountGuestPath = BoxDir
	deps.WorkspaceGuestPath = BoxWorkspaceDir
	deps.ProjectsGuestPath = BoxProjectsDir
	deps.GatewayLogPath = BoxGatewayLogPath
	if deps.WorkspaceTemplate == nil {
		deps.WorkspaceTemplate = func(_, _ string) (string, error) { return WorkspaceTemplateWorker, nil }
	}
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
	return "node /app/openclaw.mjs gateway stop 2>/dev/null; sleep 2; exec node /app/openclaw.mjs gateway --allow-unconfigured --bind lan --port 18789 1>>" + BoxGatewayLogPath + " 2>&1"
}
