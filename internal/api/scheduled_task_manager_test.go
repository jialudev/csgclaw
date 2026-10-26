package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"csgclaw/internal/apitypes"
	"csgclaw/internal/scheduledtask"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestManagerScheduleConfirmationAndTrigger(t *testing.T) {
	h := newAppTaskTestHandler(t)
	store, err := scheduledtask.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	due := now.Add(time.Minute)
	svc, err := scheduledtask.NewService(store, h.agentTaskSvc, scheduledtask.WithNowFunc(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	h.SetScheduledTaskService(svc)
	h.registerAppScheduledTaskTools("agent-manager")
	client := taskMCPClient(t, h, "agent-manager")
	args := map[string]any{"title": "Morning summary", "prompt": "Summarize my tasks", "agent_id": "manager", "first_run_at": due.Format(time.RFC3339), "recurrence": "daily", "time_zone": "Asia/Shanghai", "client_request_id": "manager-confirm-1", "confirmed": false}
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "scheduled_task_create", Arguments: args})
	if err != nil || !result.IsError || len(svc.List()) != 0 {
		t.Fatalf("saved unconfirmed task: %v %+v", err, result)
	}
	args["confirmed"] = true
	var created apitypes.ScheduledTask
	callTaskTool(t, client, "scheduled_task_create", args, &created)
	callTaskTool(t, client, "scheduled_task_create", args, nil)
	if len(svc.List()) != 1 || created.AgentID != "agent-manager" {
		t.Fatal("wrong executor or duplicate creation")
	}
	now = due.Add(time.Second)
	runs := svc.TriggerDue(context.Background())
	if len(runs) != 1 || runs[0].Status != scheduledtask.StatusTriggered {
		t.Fatalf("trigger failed: %+v", runs)
	}
	callTaskTool(t, client, "scheduled_task_create", args, nil)
	if len(svc.List()) != 1 {
		t.Fatal("retry after the scheduled time duplicated the plan")
	}
	tasks := h.agentTaskSvc.List()
	if len(tasks) != 1 || tasks[0].AssignmentID != "agent-manager" {
		t.Fatalf("wrong task assignment: %+v", tasks)
	}
	room, ok := h.im.Room(tasks[0].RoomID)
	if !ok || !room.IsDirect || len(room.Messages) < 2 {
		t.Fatalf("Manager did not receive direct scheduled work: %+v", room)
	}
	user, _ := h.im.User("manager")
	if user.Role != "manager" {
		t.Fatal("Manager identity was overwritten")
	}
	if runs := svc.TriggerDue(context.Background()); len(runs) != 0 {
		t.Fatal("triggered again before next scheduled time")
	}
}
func TestScheduleRoutesRestrictRuntimeWrites(t *testing.T) {
	h := newAppTaskTestHandler(t)
	for _, tc := range []struct {
		id, method, path, body string
		allowed                bool
	}{
		{"agent-dev", "POST", "/api/v1/scheduled-tasks", `{"confirmed":true}`, false},
		{"agent-manager", "POST", "/api/v1/scheduled-tasks", `{}`, false},
		{"agent-manager", "POST", "/api/v1/scheduled-tasks", `{"confirmed":true}`, true},
		{"agent-manager", "POST", "/api/v1/scheduled-tasks/parse", `{}`, true},
		{"agent-manager", "DELETE", "/api/v1/scheduled-tasks/any", `{}`, false},
	} {
		if got := h.authorizeAgentPlatformRoute(httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)), tc.id); got != tc.allowed {
			t.Fatalf("%+v: %v", tc, got)
		}
	}
	recorder := httptest.NewRecorder()
	h.Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/scheduled-tasks/parse", strings.NewReader(`{"text":"tomorrow"}`)))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("parser not configured: %d", recorder.Code)
	}
}
