package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"csgclaw/internal/agentengine"
	agent "csgclaw/internal/agentengine/agents"
	"csgclaw/internal/agentengine/contract"
	"csgclaw/internal/controlplane"
)

const controlBodyLimit = 1024 * 1024

type controlService struct {
	store           *controlplane.Store
	ctx             context.Context
	mu              sync.Mutex
	configurationMu sync.Mutex
	skillRoot       func() (string, error)
}

func (h *Handler) SetControlStore(store *controlplane.Store, ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	h.control = &controlService{store: store, ctx: ctx}
}
func controlError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]any{"code": code, "message": message, "retryable": status == 503 || status == 429, "request_id": newAgentSessionAPIID("req")})
}
func writeControlError(w http.ResponseWriter, err error) {
	status, code := http.StatusBadRequest, string(agentengine.ErrorCodeOf(err))
	if code == "" {
		code = "invalid_request"
	}
	switch {
	case errors.Is(err, os.ErrNotExist):
		status = 404
		code = "not_found"
	case errors.Is(err, agent.ErrAgentResourceVersionConflict):
		status = 409
		code = "configuration_conflict"
	case agentengine.ErrorCodeOf(err) == agentengine.ErrorAgentNotFound || agentengine.ErrorCodeOf(err) == agentengine.ErrorFileNotFound || agentengine.ErrorCodeOf(err) == agentengine.ErrorInteractionNotFound:
		status = 404
	case agentengine.ErrorCodeOf(err) == agentengine.ErrorExecutionUnknown || agentengine.ErrorCodeOf(err) == agentengine.ErrorConversationBusy || agentengine.ErrorCodeOf(err) == agentengine.ErrorInteractionAlreadyResolved:
		status = 409
	case agentengine.ErrorCodeOf(err) == agentengine.ErrorRuntimeFailed || agentengine.ErrorCodeOf(err) == agentengine.ErrorFileUnavailable || agentengine.ErrorCodeOf(err) == agentengine.ErrorAgentUnavailable || agentengine.ErrorCodeOf(err) == agentengine.ErrorRuntimeAdapterUnavailable:
		status = 503
	}
	controlError(w, status, code, err.Error())
}
func decodeControl(w http.ResponseWriter, r *http.Request, out any) bool {
	return decodeControlLimit(w, r, out, controlBodyLimit)
}
func decodeControlLimit(w http.ResponseWriter, r *http.Request, out any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			controlError(w, 413, "body_too_large", "request exceeds the body limit")
			return false
		}
		controlError(w, 400, "invalid_request", "invalid request body")
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		controlError(w, 400, "invalid_request", "only one JSON value is allowed")
		return false
	}
	return true
}
func (h *Handler) controlAgent(w http.ResponseWriter, r *http.Request) (agentengine.Agent, bool) {
	if !h.validateServerAccessToken(r.Header.Get("Authorization")) {
		controlError(w, 401, "unauthorized", "service credential is required")
		return agentengine.Agent{}, false
	}
	if h.agentEngine == nil || h.control == nil || h.control.store == nil {
		controlError(w, 503, "control_unavailable", "control service is unavailable")
		return agentengine.Agent{}, false
	}
	current, err := h.agentEngine.Agents().Get(r.Context(), pathValue(r, "id"), agentengine.AgentGetOptions{})
	if err != nil {
		writeControlError(w, err)
		return current, false
	}
	w.Header().Set("Cache-Control", "no-store")
	return current, true
}
func configurationRevision(a agentengine.Agent) string {
	return contract.ConfigurationRevision(a.Spec, a.ResourceVersion)
}
func editableSpec(spec agentengine.AgentSpec) agentengine.AgentSpec {
	spec.Runtime.Credentials = nil
	spec.Runtime.InitShell = ""
	spec.Model.APIKey = ""
	spec.Model.Headers = nil
	spec.Model.Env = nil
	servers := map[string]agentengine.MCPServerConfig{}
	for name, server := range spec.MCPServers {
		view := agentengine.MCPServerConfig{"_credentials_redacted": true}
		for _, key := range []string{"command", "type", "transport", "disabled", "enabled"} {
			if value, ok := server[key]; ok {
				view[key] = value
			}
		}
		if raw, ok := server["url"].(string); ok {
			if u, err := url.Parse(raw); err == nil && u.User == nil && u.RawQuery == "" {
				view["url"] = raw
			}
		}
		servers[name] = view
	}
	spec.MCPServers = servers
	return spec
}
func configurationView(a agentengine.Agent) map[string]any {
	rev := configurationRevision(a)
	applied := ""
	state := "not_ready"
	if a.Status.State == agentengine.AgentStateCreating || a.Status.State == agentengine.AgentStateStarting || a.Status.State == agentengine.AgentStateRecreating {
		state = "applying"
	}
	if a.Status.State == agentengine.AgentStateFailed {
		state = "failed"
	}
	if a.Status.Model.EnvRestartRequired || a.Status.Model.ImageUpgradeRequired {
		state = "restart_required"
	} else if a.Status.Ready {
		state = "applied"
		applied = rev
	}
	return map[string]any{"agent_id": a.ID, "spec": editableSpec(a.Spec), "desired_revision": rev, "applied_revision": applied, "state": state}
}
func (h *Handler) getControlConfiguration(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if ok {
		h.writeConfigurationView(w, a)
	}
}

var controlEditableFields = map[string]bool{"name": true, "description": true, "instructions": true, "skills": true, "skill_states": true, "mcp_servers": true, "model.selector": true, "model.model_id": true, "model.reasoning_effort": true, "model.fast_mode": true, "runtime.adapter": true, "runtime.sandboxed": true, "runtime.options": true}

func (h *Handler) patchControlConfiguration(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision string                `json:"expected_revision"`
		FieldMask        []string              `json:"field_mask"`
		Spec             agentengine.AgentSpec `json:"spec"`
	}
	if !decodeControl(w, r, &req) {
		return
	}
	if req.ExpectedRevision == "" || len(req.FieldMask) == 0 {
		controlError(w, 400, "invalid_request", "expected_revision and field_mask are required")
		return
	}
	for _, field := range req.FieldMask {
		if !controlEditableFields[field] {
			controlError(w, 422, "unsupported_field", "field is not editable: "+field)
			return
		}
	}
	// Never pass write-only model, environment or provisioning values through the generic editor.
	req.Spec.Runtime.Credentials = nil
	req.Spec.Runtime.InitShell = ""
	req.Spec.Runtime.Image = ""
	req.Spec.Model.APIKey = ""
	req.Spec.Model.Headers = nil
	req.Spec.Model.Env = nil
	if req.ExpectedRevision != configurationRevision(a) {
		controlError(w, 409, "configuration_conflict", "configuration changed; reload before saving")
		return
	}
	for name, server := range req.Spec.MCPServers {
		if redacted, ok := server["_credentials_redacted"].(bool); ok && redacted {
			previous, exists := a.Spec.MCPServers[name]
			if !exists {
				controlError(w, 409, "configuration_conflict", "MCP server no longer exists")
				return
			}
			merged := agentengine.MCPServerConfig{}
			for key, value := range previous {
				merged[key] = value
			}
			for key, value := range server {
				if key != "_credentials_redacted" {
					merged[key] = value
				}
			}
			req.Spec.MCPServers[name] = merged
		}
	}
	updated, err := h.agentEngine.Agents().Update(r.Context(), a.ID, agentengine.AgentUpdateRequest{Spec: req.Spec, FieldMask: req.FieldMask, ResourceVersion: a.ResourceVersion, ConfigurationRevision: req.ExpectedRevision})
	if err != nil {
		writeControlError(w, err)
		return
	}
	h.publishUpdatedAgentUser(serviceAgentFromEngine(updated))
	h.writeConfigurationView(w, updated)
}
func (h *Handler) applyControlConfiguration(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision string `json:"expected_revision"`
	}
	if !decodeControl(w, r, &req) {
		return
	}
	if req.ExpectedRevision == "" || req.ExpectedRevision != configurationRevision(a) {
		controlError(w, 409, "configuration_conflict", "configuration changed; reload before applying")
		return
	}
	desired := a.Spec
	desired.DesiredState = agentengine.AgentDesiredStateRunning
	updated, err := h.agentEngine.Agents().Update(r.Context(), a.ID, agentengine.AgentUpdateRequest{Spec: desired, FieldMask: []string{"desired_state"}, ResourceVersion: a.ResourceVersion, ConfigurationRevision: req.ExpectedRevision})
	if err != nil {
		writeControlError(w, err)
		return
	}
	h.publishUpdatedAgentUser(serviceAgentFromEngine(updated))
	h.writeConfigurationView(w, updated)
}
func (h *Handler) getControlCapabilities(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	_, history := h.agentEngine.Conversations(a.ID).(agentengine.ConversationHistory)
	writeJSON(w, 200, map[string]any{"backend_id": h.control.store.ID, "agent_id": a.ID, "control_api_version": 1, "ready": a.Status.Ready, "api_only": h.apiOnly, "runtime": a.Spec.Runtime.Adapter, "features": map[string]bool{"configuration": true, "files": true, "turns": history, "events": true}, "editable_fields": controlEditableFields})
}
func (h *Handler) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if !h.validateServerAccessToken(r.Header.Get("Authorization")) {
		controlError(w, 401, "unauthorized", "service credential is required")
		return
	}
	if h.control == nil || h.control.store == nil || h.agentEngine == nil {
		controlError(w, 503, "not_ready", "configuration or storage is unavailable")
		return
	}
	// This is service readiness; Agent capabilities report execution readiness.
	writeJSON(w, 200, map[string]any{"ready": true, "backend_id": h.control.store.ID})
}
func controlPathID(r *http.Request, name string) string { return strings.TrimSpace(pathValue(r, name)) }

// Remember the last version observed as applied instead of reporting an empty
// applied version whenever a later configuration is pending or fails.
func (h *Handler) writeConfigurationView(w http.ResponseWriter, a agentengine.Agent) {
	h.control.configurationMu.Lock()
	defer h.control.configurationMu.Unlock()
	key := "configuration_state:" + a.ID
	var state struct {
		Desired string `json:"desired_revision"`
		Applied string `json:"applied_revision"`
	}
	err := h.control.store.Read(key, &state)
	if err != nil && !os.IsNotExist(err) {
		controlError(w, 503, "storage_unavailable", "configuration status could not be read")
		return
	}
	previous := state
	view := configurationView(a)
	state.Desired = configurationRevision(a)
	if applied, _ := view["applied_revision"].(string); applied != "" {
		state.Applied = applied
	}
	if state != previous {
		if err = h.control.store.Write(key, state); err != nil {
			controlError(w, 503, "storage_unavailable", "configuration status could not be saved")
			return
		}
	}
	view["applied_revision"] = state.Applied
	writeJSON(w, 200, view)
}
