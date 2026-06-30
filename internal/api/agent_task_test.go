package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"csgclaw/internal/agenttask"
	"csgclaw/internal/apitypes"
	"csgclaw/internal/im"
	"csgclaw/internal/taskcore"
)

func TestAgentTaskAPI(t *testing.T) {
	core := taskcore.NewService()
	imSvc := im.NewService()
	h := &Handler{
		im:           imSvc,
		agentTaskSvc: agenttask.NewService(core, imSvc, nil, nil),
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/agent-tasks", strings.NewReader(`{"agent_id":"agent-dev","title":"Fix flaky test","body":"Investigate it."}`))
	createRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d: %s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}
	var created apitypes.TeamTask
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.AssignmentType != taskcore.AssignmentTypeAgent || created.AssignmentID != "agent-dev" || created.RoomID == "" {
		t.Fatalf("created task = %+v, want agent assignment with room", created)
	}

	claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/agent-tasks/"+created.ID+"/claim", strings.NewReader(`{"participant_id":"pt-dev"}`))
	claimRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(claimRec, claimReq)
	if claimRec.Code != http.StatusOK {
		t.Fatalf("claim status = %d, want %d: %s", claimRec.Code, http.StatusOK, claimRec.Body.String())
	}

	completeReq := httptest.NewRequest(http.MethodPatch, "/api/v1/agent-tasks/"+created.ID, strings.NewReader(`{"actor_id":"pt-dev","status":"completed","result":"done"}`))
	completeRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(completeRec, completeReq)
	if completeRec.Code != http.StatusOK {
		t.Fatalf("complete status = %d, want %d: %s", completeRec.Code, http.StatusOK, completeRec.Body.String())
	}
	var completed apitypes.TeamTask
	if err := json.NewDecoder(completeRec.Body).Decode(&completed); err != nil {
		t.Fatalf("decode complete response: %v", err)
	}
	if completed.Status != taskcore.StatusCompleted || completed.Result != "done" {
		t.Fatalf("completed task = %+v, want completed result", completed)
	}

	globalReq := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	globalRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(globalRec, globalReq)
	if globalRec.Code != http.StatusOK {
		t.Fatalf("global status = %d, want %d: %s", globalRec.Code, http.StatusOK, globalRec.Body.String())
	}
	var global []apitypes.GlobalTask
	if err := json.NewDecoder(globalRec.Body).Decode(&global); err != nil {
		t.Fatalf("decode global response: %v", err)
	}
	if len(global) != 1 || global[0].AssignmentType != taskcore.AssignmentTypeAgent {
		t.Fatalf("global tasks = %+v, want one agent task", global)
	}
}
