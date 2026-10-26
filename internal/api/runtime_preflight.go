package api

import (
	"context"
	"net/http"
	"os"
	"time"

	"csgclaw/internal/agentengine"
	"csgclaw/internal/apitypes"
	agentruntime "csgclaw/internal/runtime"
	"csgclaw/internal/runtimeenv"
)

func commonDependencies(declared []apitypes.RuntimeDependency) []apitypes.RuntimeDependency {
	deps := append([]apitypes.RuntimeDependency(nil), declared...)
	for _, name := range []string{"node", "python"} {
		found := false
		for _, d := range deps {
			if d.Name == name {
				found = true
			}
		}
		if !found {
			deps = append(deps, apitypes.RuntimeDependency{Name: name, Optional: true})
		}
	}
	return deps
}
func (h *Handler) inspectRuntimeEnvironment(ctx context.Context, runtimeName string, sandboxed bool, deps []apitypes.RuntimeDependency) runtimeenv.Report {
	if sandboxed {
		return runtimeenv.Report{Ready: false, Scope: "sandbox_unverified", CheckedAt: time.Now().UTC(), Checks: []runtimeenv.Check{{Name: "runtime_environment", Required: true, State: "sandbox_probe_required"}}}
	}
	report := runtimeenv.Inspect(ctx, commonDependencies(deps), nil)
	if runtimeName == "codex" || runtimeName == "dsh" {
		check := runtimeenv.Check{Name: "agent_runtime", Required: true, State: "unavailable"}
		if h.agentRuntimes != nil {
			for _, runtime := range h.agentRuntimes.List() {
				if runtime.Name == runtimeName && runtime.Installed {
					check.State = "ready"
					check.Version = runtime.Version
				}
			}
		}
		if check.State != "ready" {
			report.Ready = false
		}
		report.Checks = append(report.Checks, check)
	}
	return report
}
func (h *Handler) preflightTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.validateServerAccessToken(r.Header.Get("Authorization")) {
		controlError(w, 401, "unauthorized", "service credential is required")
		return
	}
	if h.hub == nil {
		controlError(w, 503, "templates_unavailable", "template service is unavailable")
		return
	}
	item, err := h.hub.Get(r.Context(), pathValue(r, "id"))
	if err != nil {
		writeControlError(w, err)
		return
	}
	selected := agentruntime.RuntimeConfigForKind(item.RuntimeKind)
	report := h.inspectRuntimeEnvironment(r.Context(), selected.Name, selected.Sandboxed, item.Dependencies)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"template_id": item.ID, "template_version": item.Version, "dependencies": item.Dependencies, "environment": report, "ready": report.Ready, "model_probe": "not_performed", "mcp_probe": "not_performed"})
}
func (h *Handler) preflightAgent(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	observed, err := h.agentEngine.Agents().Get(ctx, a.ID, agentengine.AgentGetOptions{ProbeRuntime: true})
	if err != nil {
		writeControlError(w, err)
		return
	}
	report := h.inspectRuntimeEnvironment(ctx, a.Spec.Runtime.Adapter, a.Spec.Runtime.Sandboxed, a.Spec.Dependencies)
	report.Checks = append(report.Checks, runtimeenv.Check{Name: "agent", Required: true, State: string(observed.Status.State)})
	if !observed.Status.Ready {
		report.Ready = false
	}
	modelState := "configured"
	if !observed.Status.Model.ProfileComplete {
		modelState = "missing_configuration"
		report.Ready = false
	}
	report.Checks = append(report.Checks, runtimeenv.Check{Name: "model_configuration", Required: true, State: modelState})
	writable := h.checkWorkspaceWritable(a)
	if writable.State != "ready" {
		report.Ready = false
	}
	report.Checks = append(report.Checks, writable)
	writeJSON(w, 200, map[string]any{"agent_id": a.ID, "configuration_revision": configurationRevision(a), "ready": report.Ready, "environment": report, "model_probe": "not_performed", "mcp_probe": "not_performed", "message": observed.Status.Message})
}

func (h *Handler) checkWorkspaceWritable(a agentengine.Agent) runtimeenv.Check {
	writable := runtimeenv.Check{Name: "workspace", Required: true, State: "not_writable"}
	if h.workspace != nil && !a.Spec.Runtime.Sandboxed {
		if root, err := h.workspace.WorkspaceRootByID(a.ID); err == nil {
			if f, err := os.CreateTemp(root, ".preflight-"); err == nil {
				name := f.Name()
				f.Close()
				os.Remove(name)
				writable.State = "ready"
			}
		}
	}

	return writable
}
