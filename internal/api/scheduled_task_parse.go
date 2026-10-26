package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"csgclaw/internal/agentengine"
	agent "csgclaw/internal/agentengine/agents"
	"csgclaw/internal/scheduledtask"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (h *Handler) handleParseScheduledTask(w http.ResponseWriter, r *http.Request) {
	if h.llm == nil || h.agentEngine == nil {
		http.Error(w, "Configure the Manager model, or fill in the schedule form.", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Text           string `json:"text"`
		TimeZone       string `json:"time_zone"`
		DefaultAgentID string `json:"default_agent_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 20000)).Decode(&req); err != nil {
		http.Error(w, "invalid parse request", http.StatusBadRequest)
		return
	}
	if req.TimeZone == "" {
		req.TimeZone = "UTC"
	}
	if req.DefaultAgentID == "" {
		req.DefaultAgentID = agent.ManagerUserID
	}
	items, err := h.agentEngine.Agents().List(r.Context(), agentengine.AgentListOptions{})
	if err != nil {
		http.Error(w, "executors unavailable", http.StatusServiceUnavailable)
		return
	}
	executors := make([]scheduledtask.Executor, 0, len(items))
	for _, item := range items {
		if item.Spec.Role == "manager" || item.Spec.Role == "worker" || item.Spec.Role == "agent" {
			executors = append(executors, scheduledtask.Executor{ID: item.ID, Name: item.Spec.Name})
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	draft, err := scheduledtask.Parse(ctx, h.llm, req.Text, req.TimeZone, req.DefaultAgentID, executors, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if draft.Clarification == "" {
		draft.ClientRequestID = rand.Text()
	}
	writeJSON(w, http.StatusOK, draft)
}

// Manager gets scheduling tools in direct conversations without needing a shell.
// Preview never persists; confirmation is a separate write tool with a stable retry ID.
func (h *Handler) registerAppScheduledTaskTools(agentID string) {
	if !h.agentPlatformManager(agentID) {
		return
	}
	parseFields := taskToolFields("text", "time_zone", "default_agent_id")
	h.addAppPlatformTool(agentID, "scheduled_task_parse", "Preview a natural-language schedule. Show time, time zone, recurrence, prompt and executor to the user and wait for confirmation. This does not save or run anything.", parseFields, []string{"text", "time_zone"}, true, func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (any, error) {
		return h.invokeAppPlatformHandler(ctx, agentID, http.MethodPost, nil, nil, args, h.handleParseScheduledTask)
	})
	fields := taskToolFields("title", "prompt", "agent_id", "first_run_at", "expires_at", "recurrence", "time_zone", "client_request_id")
	fields["confirmed"] = map[string]any{"type": "boolean"}
	h.addAppPlatformTool(agentID, "scheduled_task_create", "Save the edited schedule ONLY after the user confirms the preview in a later message. Set confirmed=true and reuse client_request_id from the preview on retries. Never create immediately after parsing.", fields, []string{"title", "prompt", "agent_id", "first_run_at", "recurrence", "time_zone", "client_request_id", "confirmed"}, false, func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (any, error) {
		confirmed, _ := args["confirmed"].(bool)
		if !confirmed {
			return nil, fmt.Errorf("show the schedule preview and wait for user confirmation")
		}
		if taskToolText(args, "client_request_id") == "" {
			return nil, fmt.Errorf("client_request_id is required")
		}
		return h.invokeAppPlatformHandler(ctx, agentID, http.MethodPost, nil, nil, args, h.handleCreateScheduledTask)
	})
	h.addAppPlatformTool(agentID, "scheduled_tasks_list", "List saved schedules to verify a request before retrying creation.", nil, nil, true, func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (any, error) {
		return h.invokeAppPlatformHandler(ctx, agentID, http.MethodGet, nil, nil, nil, h.handleListScheduledTasks)
	})
}
