package server

import (
	"testing"

	"csgclaw/internal/im"
	"csgclaw/internal/agent"
)

func TestDeriveAgentHandle(t *testing.T) {
	tests := []struct {
		name  string
		agent agent.Agent
		want  string
	}{
		{
			name:  "plain name",
			agent: agent.Agent{Name: "Alice Smith", ID: "u-alice", Role: agent.RoleWorker},
			want:  "alice-smith",
		},
		{
			name:  "fallback to id",
			agent: agent.Agent{Name: "中文 名字", ID: "u-worker_01", Role: agent.RoleWorker},
			want:  "worker_01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveAgentHandle(tt.agent); got != tt.want {
				t.Fatalf("deriveAgentHandle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnsureWorkerIMStatePublishesBootstrapConversation(t *testing.T) {
	bus := im.NewBus()
	events, cancel := bus.Subscribe()
	defer cancel()

	srv := &HTTPServer{
		im:    im.NewService(),
		imBus: bus,
	}

	created := agent.Agent{
		ID:   "u-alice",
		Name: "Alice",
		Role: agent.RoleWorker,
	}
	if err := srv.ensureWorkerIMState(created); err != nil {
		t.Fatalf("ensureWorkerIMState() error = %v", err)
	}

	first := mustReceiveEvent(t, events)
	if first.Type != im.EventTypeUserCreated {
		t.Fatalf("first event.Type = %q, want %q", first.Type, im.EventTypeUserCreated)
	}
	if first.User == nil || first.User.ID != "u-alice" {
		t.Fatalf("first event.User = %+v, want u-alice", first.User)
	}

	second := mustReceiveEvent(t, events)
	if second.Type != im.EventTypeConversationCreated {
		t.Fatalf("second event.Type = %q, want %q", second.Type, im.EventTypeConversationCreated)
	}
	if second.Conversation == nil {
		t.Fatal("second event.Conversation = nil, want bootstrap conversation")
	}
	if second.Conversation.Title != "Alice" {
		t.Fatalf("second event.Conversation.Title = %q, want %q", second.Conversation.Title, "Alice")
	}
	if !containsParticipant(second.Conversation.Participants, "u-admin") || !containsParticipant(second.Conversation.Participants, "u-alice") {
		t.Fatalf("second event.Conversation.Participants = %+v, want admin and worker", second.Conversation.Participants)
	}
}

func containsParticipant(participants []string, want string) bool {
	for _, participant := range participants {
		if participant == want {
			return true
		}
	}
	return false
}

func mustReceiveEvent(t *testing.T, events <-chan im.Event) im.Event {
	t.Helper()

	select {
	case evt := <-events:
		return evt
	default:
		t.Fatal("expected event")
		return im.Event{}
	}
}
