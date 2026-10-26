package api

import (
	"crypto/sha256"
	"csgclaw/internal/runtimeenv"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"time"

	"csgclaw/internal/agentengine"
	"csgclaw/internal/controlplane"
)

type ensureAssistantRequest struct {
	Template      string `json:"template,omitempty"`
	Runtime       string `json:"runtime,omitempty"`
	ModelSelector string `json:"model_selector,omitempty"`
	OperationID   string `json:"operation_id,omitempty"`
}

func (h *Handler) assistantAuthorized(w http.ResponseWriter, r *http.Request) bool {
	if !h.validateServerAccessToken(r.Header.Get("Authorization")) {
		controlError(w, 401, "unauthorized", "service credential is required")
		return false
	}
	if h.control == nil || h.control.store == nil || h.agentEngine == nil {
		controlError(w, 503, "control_unavailable", "control service is unavailable")
		return false
	}
	return true
}
func (h *Handler) defaultAssistant(w http.ResponseWriter, r *http.Request) {
	if !h.assistantAuthorized(w, r) {
		return
	}
	h.control.mu.Lock()
	defer h.control.mu.Unlock()
	var binding controlplane.AssistantBinding
	if err := h.control.store.Read(controlplane.DefaultAssistantKey, &binding); err != nil {
		writeControlError(w, err)
		return
	}
	h.writeAssistant(w, r, binding)
}
func (h *Handler) ensureDefaultAssistant(w http.ResponseWriter, r *http.Request) {
	h.prepareDefaultAssistant(w, r, false)
}
func (h *Handler) retryDefaultAssistant(w http.ResponseWriter, r *http.Request) {
	h.prepareDefaultAssistant(w, r, true)
}
func (h *Handler) prepareDefaultAssistant(w http.ResponseWriter, r *http.Request, retry bool) {
	if !h.assistantAuthorized(w, r) {
		return
	}
	var request ensureAssistantRequest
	if r.ContentLength != 0 && !decodeControl(w, r, &request) {
		return
	}
	if request.Runtime != "" && request.Runtime != "codex" && request.Runtime != "dsh" {
		controlError(w, 422, "unsupported_runtime", "default assistant supports explicit Codex or DSH selection")
		return
	}
	h.control.mu.Lock()
	defer h.control.mu.Unlock()
	var binding controlplane.AssistantBinding
	err := h.control.store.Read(controlplane.DefaultAssistantKey, &binding)
	if err != nil && !os.IsNotExist(err) {
		writeControlError(w, err)
		return
	}
	if os.IsNotExist(err) {
		if retry {
			controlError(w, 404, "assistant_not_found", "no default assistant has been reserved")
			return
		}
		runtime := request.Runtime
		if runtime == "" {
			runtime = "codex"
		}
		sum := sha256.Sum256([]byte(h.control.store.ID + ":assistant"))
		now := time.Now().UTC()
		binding = controlplane.AssistantBinding{AgentID: "agent-assistant-" + hex.EncodeToString(sum[:10]), OperationID: newAgentSessionAPIID("prepare"), Template: request.Template, Runtime: runtime, ModelSelector: request.ModelSelector, State: "preparing", CreatedAt: now, UpdatedAt: now}
		if err = h.control.store.Write(controlplane.DefaultAssistantKey, binding); err != nil {
			writeControlError(w, err)
			return
		}
	} else {
		if (request.Template != "" && request.Template != binding.Template) || (request.Runtime != "" && request.Runtime != binding.Runtime) || (request.ModelSelector != "" && request.ModelSelector != binding.ModelSelector) {
			controlError(w, 409, "assistant_binding_conflict", "assistant is already bound; edit its configuration instead")
			return
		}
		if retry {
			if request.OperationID == "" || request.OperationID != binding.OperationID {
				controlError(w, 409, "operation_conflict", "operation changed; reload the default assistant before retrying")
				return
			}
			if binding.State != "failed" {
				h.writeAssistant(w, r, binding)
				return
			}
			binding.OperationID = newAgentSessionAPIID("prepare")
			binding.State = "preparing"
			binding.ErrorCode = ""
			binding.UpdatedAt = time.Now().UTC()
			if err = h.control.store.Write(controlplane.DefaultAssistantKey, binding); err != nil {
				writeControlError(w, err)
				return
			}
		} else if binding.State == "failed" {
			h.writeAssistant(w, r, binding)
			return
		}
	}
	if owner, ok := h.agentEngine.(interface{ ProtectAgent(string) }); ok {
		owner.ProtectAgent(binding.AgentID)
	}
	current, err := h.agentEngine.Agents().Get(r.Context(), binding.AgentID, agentengine.AgentGetOptions{})
	if err != nil {
		if agentengine.ErrorCodeOf(err) != agentengine.ErrorAgentNotFound {
			writeControlError(w, err)
			return
		}
		current, err = h.agentEngine.Agents().Create(r.Context(), agentengine.AgentCreateRequest{ID: binding.AgentID, FromTemplate: binding.Template, Spec: agentengine.AgentSpec{Name: "assistant-" + h.control.store.ID[:8], Role: agentengine.AgentRoleWorker, Runtime: agentengine.RuntimeSpec{Adapter: binding.Runtime}, Model: agentengine.ModelSpec{Selector: binding.ModelSelector}, DesiredState: agentengine.AgentDesiredStateRunning}})
	} else if current.Spec.DesiredState != agentengine.AgentDesiredStateRunning || current.Status.State == agentengine.AgentStateStopped || current.Status.State == agentengine.AgentStateFailed {
		current.Spec.DesiredState = agentengine.AgentDesiredStateRunning
		current, err = h.agentEngine.Agents().Update(r.Context(), current.ID, agentengine.AgentUpdateRequest{Spec: current.Spec, FieldMask: []string{"desired_state"}, ResourceVersion: current.ResourceVersion})
	}
	if err != nil {
		binding.State = "failed"
		binding.ErrorCode = string(agentengine.ErrorCodeOf(err))
		if binding.ErrorCode == "" {
			binding.ErrorCode = "prepare_failed"
		}
		binding.UpdatedAt = time.Now().UTC()
		if saveErr := h.control.store.Write(controlplane.DefaultAssistantKey, binding); saveErr != nil {
			writeControlError(w, saveErr)
			return
		}
		h.writeAssistant(w, r, binding)
		return
	}
	binding.State = "preparing"
	binding.ErrorCode = ""
	binding.UpdatedAt = time.Now().UTC()
	if err = h.control.store.Write(controlplane.DefaultAssistantKey, binding); err != nil {
		writeControlError(w, err)
		return
	}
	h.writeAssistant(w, r, binding)
}
func (h *Handler) writeAssistant(w http.ResponseWriter, r *http.Request, binding controlplane.AssistantBinding) {
	ready := false
	var environment *runtimeenv.Report
	if binding.State != "failed" {
		current, err := h.agentEngine.Agents().Get(r.Context(), binding.AgentID, agentengine.AgentGetOptions{ProbeRuntime: true})
		if err == nil {
			report := h.inspectRuntimeEnvironment(r.Context(), current.Spec.Runtime.Adapter, current.Spec.Runtime.Sandboxed, current.Spec.Dependencies)
			workspace := h.checkWorkspaceWritable(current)
			report.Checks = append(report.Checks, workspace)
			if workspace.State != "ready" {
				report.Ready = false
			}
			environment = &report
			ready = current.Status.Ready && current.Status.Model.ProfileComplete && report.Ready
			if !report.Ready {
				binding.ErrorCode = "environment_not_ready"
			} else if !current.Status.Model.ProfileComplete {
				binding.ErrorCode = "model_configuration_missing"
			} else if !current.Status.Ready {
				binding.ErrorCode = "agent_not_ready"
			} else {
				binding.ErrorCode = ""
			}
			if ready {
				binding.State = "ready"
			} else {
				binding.State = "preparing"
			}
		} else if !errors.Is(err, r.Context().Err()) {
			binding.ErrorCode = "agent_unavailable"
		}
	}
	status := http.StatusAccepted
	if ready {
		status = http.StatusOK
	} else if binding.State == "failed" {
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]any{"binding": binding, "ready": ready, "environment": environment, "permissions": map[string]bool{"editable": true, "deletable": false}, "control_base": "api/v1/agents/" + binding.AgentID})
}
