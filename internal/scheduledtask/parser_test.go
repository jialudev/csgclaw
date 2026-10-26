package scheduledtask

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type parserStub struct {
	content string
	request []byte
}

func (s *parserStub) ChatCompletions(_ context.Context, id string, body []byte) ([]byte, int, string, error) {
	s.request = body
	raw, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": s.content}}}})
	return raw, 200, "application/json", nil
}
func TestSchedulePreviewValidatesModelOutput(t *testing.T) {
	now := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	valid := `{"title":"待办","prompt":"整理当天待办","agent_id":"manager","recurrence":"daily","first_run_at":"2026-10-27T09:00:00+08:00","time_zone":"Asia/Shanghai"}`
	executors := []Executor{{ID: "agent-manager", Name: "Manager"}}
	stub := &parserStub{content: valid}
	draft, err := Parse(context.Background(), stub, "每天上午9点让Manager整理待办", "Asia/Shanghai", "manager", executors, now)
	if err != nil || draft.AgentID != "agent-manager" || draft.FirstRunAt.Hour() != 9 {
		t.Fatalf("%+v %v", draft, err)
	}
	if !strings.Contains(string(stub.request), "2026-10-26T18:00:00+08:00") || strings.Contains(string(stub.request), `"tools"`) {
		t.Fatal("wrong clock or executable tools in parser")
	}
	for name, content := range map[string]string{
		"unknown agent":          strings.Replace(valid, `"manager"`, `"missing"`, 1),
		"past":                   strings.Replace(valid, "2026-10-27", "2026-10-25", 1),
		"unsupported recurrence": strings.Replace(valid, `"daily"`, `"weekdays"`, 1),
		"wrong zone":             strings.Replace(valid, "Asia/Shanghai", "UTC", 1),
		"expired before start":   strings.Replace(valid, `"title":`, `"expires_at":"2026-10-26T09:00:00+08:00","title":`, 1),
		"invalid JSON":           "not JSON",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(context.Background(), &parserStub{content: content}, "schedule", "Asia/Shanghai", "manager", executors, now); err == nil {
				t.Fatal("accepted invalid preview")
			}
		})
	}
	clarification, err := Parse(context.Background(), &parserStub{content: `{"clarification":"请指定执行时间"}`}, "提醒我", "Asia/Shanghai", "manager", executors, now)
	if err != nil || clarification.Clarification == "" || clarification.AgentID != "" {
		t.Fatalf("%+v %v", clarification, err)
	}
	monthEnd := strings.Replace(strings.Replace(valid, `"daily"`, `"monthly"`, 1), "2026-10-27", "2026-10-31", 1)
	draft, err = Parse(context.Background(), &parserStub{content: monthEnd}, "每月31日整理待办", "Asia/Shanghai", "manager", executors, now)
	if err != nil || draft.Clarification == "" {
		t.Fatalf("accepted an ambiguous month-end schedule: %+v %v", draft, err)
	}
}
func TestScheduleTimezoneSurvivesRestartAndDST(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 3, 7, 9, 0, 0, 0, zone)
	item := Task{NextRunAt: at, Recurrence: RecurrenceDaily, TimeZone: zone.String()}
	raw, _ := json.Marshal(item)
	var restored Task
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	next, _ := nextRunAt(restored, at)
	if next.In(zone).Hour() != 9 || next.Sub(at) != 23*time.Hour {
		t.Fatalf("shifted wall clock after DST: %s", next)
	}
}
func TestScheduledCreateRetryAfterTriggerDoesNotDuplicate(t *testing.T) {
	now := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	runner := &fakeAgentTaskRunner{}
	svc := newTestServiceWithRunner(t, now, runner)
	svc.now = func() time.Time { return now }
	input := CreateInput{Title: "Manager daily work", AgentID: "manager", Prompt: "Summarize tasks", Recurrence: RecurrenceDaily, TimeZone: "Asia/Shanghai", ClientRequestID: "confirm-1", FirstRunAt: now.Add(time.Minute), Enabled: true}
	original, err := svc.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	runs := svc.TriggerDue(context.Background())
	if len(runs) != 1 || runs[0].Status != StatusTriggered || len(runner.tasks) != 1 {
		t.Fatalf("did not trigger Manager task: %+v", runs)
	}
	retry, err := svc.Create(input)
	if err != nil || retry.ID != original.ID || len(svc.List()) != 1 {
		t.Fatalf("duplicate after trigger: %+v %v", retry, err)
	}
	input.Prompt = "different work"
	if _, err := svc.Create(input); err == nil {
		t.Fatal("reused retry ID with different work")
	}
	input.ClientRequestID = "new-but-past"
	if _, err := svc.Create(input); err == nil {
		t.Fatal("created a new schedule in the past")
	}
}
