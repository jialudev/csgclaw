package codexbridge

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	runtimecodex "csgclaw/internal/runtime/codex"

	acp "github.com/coder/acp-go-sdk"
)

type streamResult struct {
	events <-chan BotEvent
	errs   <-chan error
}

type fakeBotClient struct {
	mu          sync.Mutex
	streams     map[string][]streamResult
	sendRecords []SendMessageRequest
}

func (c *fakeBotClient) StreamEvents(_ context.Context, botID, _ string) (<-chan BotEvent, <-chan error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	results := c.streams[botID]
	if len(results) == 0 {
		events := make(chan BotEvent)
		close(events)
		errs := make(chan error)
		close(errs)
		return events, errs
	}
	next := results[0]
	c.streams[botID] = results[1:]
	return next.events, next.errs
}

func (c *fakeBotClient) SendMessage(_ context.Context, _ string, req SendMessageRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sendRecords = append(c.sendRecords, req)
	return nil
}

func (c *fakeBotClient) sentTexts() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.sendRecords))
	for _, req := range c.sendRecords {
		out = append(out, req.Text)
	}
	return out
}

type promptCall struct {
	runtimeID string
	sessionID string
	text      string
}

type fakePrompter struct {
	mu     sync.Mutex
	calls  []promptCall
	prompt func(context.Context, runtimecodex.SessionHandle, acp.PromptRequest) error
}

func (p *fakePrompter) Prompt(ctx context.Context, handle runtimecodex.SessionHandle, req acp.PromptRequest) (acp.PromptResponse, error) {
	call := promptCall{runtimeID: handle.RuntimeID, sessionID: string(req.SessionId)}
	if len(req.Prompt) > 0 && req.Prompt[0].Text != nil {
		call.text = req.Prompt[0].Text.Text
	}
	p.mu.Lock()
	p.calls = append(p.calls, call)
	p.mu.Unlock()

	if p.prompt != nil {
		if err := p.prompt(ctx, handle, req); err != nil {
			return acp.PromptResponse{}, err
		}
	}
	return acp.PromptResponse{}, nil
}

func (p *fakePrompter) texts() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, 0, len(p.calls))
	for _, call := range p.calls {
		out = append(out, call.text)
	}
	return out
}

func TestServiceRoundTrip(t *testing.T) {
	t.Parallel()

	stream := make(chan BotEvent, 1)
	errs := make(chan error)
	close(errs)
	stream <- BotEvent{MessageID: "m-1", RoomID: "room-1", Text: "hello"}

	sink := NewEventSink()
	client := &fakeBotClient{
		streams: map[string][]streamResult{
			"u-codex": {{events: stream, errs: errs}},
		},
	}
	prompter := &fakePrompter{
		prompt: func(_ context.Context, handle runtimecodex.SessionHandle, req acp.PromptRequest) error {
			sink.Publish(runtimecodex.SessionEvent{
				RuntimeID: handle.RuntimeID,
				SessionID: string(req.SessionId),
				Kind:      runtimecodex.SessionEventTextDelta,
				Text:      "Hello back",
			})
			sink.Publish(runtimecodex.SessionEvent{
				RuntimeID: handle.RuntimeID,
				SessionID: string(req.SessionId),
				Kind:      runtimecodex.SessionEventPromptCompleted,
			})
			return nil
		},
	}

	svc := NewService(client, prompter, sink)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := svc.StartBot(ctx, Binding{BotID: "u-codex", RuntimeID: "rt-1", SessionID: "sess-1"}); err != nil {
		t.Fatalf("StartBot() error = %v", err)
	}
	defer svc.Close()

	waitFor(t, func() bool {
		return slices.Equal(prompter.texts(), []string{"hello"}) && slices.Equal(client.sentTexts(), []string{"Hello back"})
	})
}

func TestServiceDedupesReplayAcrossReconnect(t *testing.T) {
	t.Parallel()

	first := make(chan BotEvent, 1)
	firstErrs := make(chan error)
	second := make(chan BotEvent, 1)
	secondErrs := make(chan error)
	first <- BotEvent{MessageID: "m-1", RoomID: "room-1", Text: "hello"}
	second <- BotEvent{MessageID: "m-1", RoomID: "room-1", Text: "hello"}
	close(first)
	close(second)
	close(firstErrs)
	close(secondErrs)

	sink := NewEventSink()
	client := &fakeBotClient{
		streams: map[string][]streamResult{
			"u-codex": {
				{events: first, errs: firstErrs},
				{events: second, errs: secondErrs},
			},
		},
	}
	prompter := &fakePrompter{
		prompt: func(_ context.Context, handle runtimecodex.SessionHandle, req acp.PromptRequest) error {
			sink.Publish(runtimecodex.SessionEvent{
				RuntimeID: handle.RuntimeID,
				SessionID: string(req.SessionId),
				Kind:      runtimecodex.SessionEventTextDelta,
				Text:      "once",
			})
			sink.Publish(runtimecodex.SessionEvent{
				RuntimeID: handle.RuntimeID,
				SessionID: string(req.SessionId),
				Kind:      runtimecodex.SessionEventPromptCompleted,
			})
			return nil
		},
	}

	svc := NewService(client, prompter, sink)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := svc.StartBot(ctx, Binding{BotID: "u-codex", RuntimeID: "rt-1", SessionID: "sess-1"}); err != nil {
		t.Fatalf("StartBot() error = %v", err)
	}
	defer svc.Close()

	waitFor(t, func() bool {
		return slices.Equal(prompter.texts(), []string{"hello"}) && slices.Equal(client.sentTexts(), []string{"once"})
	})
}

func TestServiceQueuesWhileBusy(t *testing.T) {
	t.Parallel()

	stream := make(chan BotEvent, 2)
	errs := make(chan error)
	stream <- BotEvent{MessageID: "m-1", RoomID: "room-1", Text: "first"}
	stream <- BotEvent{MessageID: "m-2", RoomID: "room-1", Text: "second"}
	close(errs)

	sink := NewEventSink()
	firstRelease := make(chan struct{})
	firstStarted := make(chan struct{})
	client := &fakeBotClient{
		streams: map[string][]streamResult{
			"u-codex": {{events: stream, errs: errs}},
		},
	}
	prompter := &fakePrompter{
		prompt: func(ctx context.Context, handle runtimecodex.SessionHandle, req acp.PromptRequest) error {
			text := req.Prompt[0].Text.Text
			if text == "first" {
				close(firstStarted)
				select {
				case <-firstRelease:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			sink.Publish(runtimecodex.SessionEvent{
				RuntimeID: handle.RuntimeID,
				SessionID: string(req.SessionId),
				Kind:      runtimecodex.SessionEventTextDelta,
				Text:      "reply:" + text,
			})
			sink.Publish(runtimecodex.SessionEvent{
				RuntimeID: handle.RuntimeID,
				SessionID: string(req.SessionId),
				Kind:      runtimecodex.SessionEventPromptCompleted,
			})
			return nil
		},
	}

	svc := NewService(client, prompter, sink)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := svc.StartBot(ctx, Binding{BotID: "u-codex", RuntimeID: "rt-1", SessionID: "sess-1"}); err != nil {
		t.Fatalf("StartBot() error = %v", err)
	}
	defer svc.Close()

	select {
	case <-firstStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("first prompt did not start")
	}

	time.Sleep(150 * time.Millisecond)
	if got := prompter.texts(); !slices.Equal(got, []string{"first"}) {
		t.Fatalf("prompt order before release = %v, want [first]", got)
	}

	close(firstRelease)
	waitFor(t, func() bool {
		return slices.Equal(prompter.texts(), []string{"first", "second"}) &&
			slices.Equal(client.sentTexts(), []string{"reply:first", "reply:second"})
	})
}

func TestServiceFlushesAfterPromptSettlesWithoutTerminalEvent(t *testing.T) {
	t.Parallel()

	stream := make(chan BotEvent, 1)
	errs := make(chan error)
	close(errs)
	stream <- BotEvent{MessageID: "m-1", RoomID: "room-1", Text: "hello"}

	sink := NewEventSink()
	client := &fakeBotClient{
		streams: map[string][]streamResult{
			"u-codex": {{events: stream, errs: errs}},
		},
	}
	prompter := &fakePrompter{
		prompt: func(_ context.Context, handle runtimecodex.SessionHandle, req acp.PromptRequest) error {
			sink.Publish(runtimecodex.SessionEvent{
				RuntimeID: handle.RuntimeID,
				SessionID: string(req.SessionId),
				Kind:      runtimecodex.SessionEventTextDelta,
				Text:      "settled reply",
			})
			return nil
		},
	}

	svc := NewService(client, prompter, sink)
	svc.promptSettle = 25 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := svc.StartBot(ctx, Binding{BotID: "u-codex", RuntimeID: "rt-1", SessionID: "sess-1"}); err != nil {
		t.Fatalf("StartBot() error = %v", err)
	}
	defer svc.Close()

	waitFor(t, func() bool {
		return slices.Equal(client.sentTexts(), []string{"settled reply"})
	})
}

func TestServiceIgnoresEventsFromOtherBindings(t *testing.T) {
	t.Parallel()

	stream := make(chan BotEvent, 1)
	errs := make(chan error)
	close(errs)
	stream <- BotEvent{MessageID: "m-1", RoomID: "room-1", Text: "hello"}

	sink := NewEventSink()
	client := &fakeBotClient{
		streams: map[string][]streamResult{
			"u-codex": {{events: stream, errs: errs}},
		},
	}
	prompter := &fakePrompter{
		prompt: func(_ context.Context, handle runtimecodex.SessionHandle, req acp.PromptRequest) error {
			sink.Publish(runtimecodex.SessionEvent{
				RuntimeID: "rt-other",
				SessionID: string(req.SessionId),
				Kind:      runtimecodex.SessionEventTextDelta,
				Text:      "wrong runtime",
			})
			sink.Publish(runtimecodex.SessionEvent{
				RuntimeID: handle.RuntimeID,
				SessionID: "sess-other",
				Kind:      runtimecodex.SessionEventTextDelta,
				Text:      "wrong session",
			})
			sink.Publish(runtimecodex.SessionEvent{
				RuntimeID: handle.RuntimeID,
				SessionID: string(req.SessionId),
				Kind:      runtimecodex.SessionEventTextDelta,
				Text:      "matched",
			})
			sink.Publish(runtimecodex.SessionEvent{
				RuntimeID: handle.RuntimeID,
				SessionID: string(req.SessionId),
				Kind:      runtimecodex.SessionEventPromptCompleted,
			})
			return nil
		},
	}

	svc := NewService(client, prompter, sink)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := svc.StartBot(ctx, Binding{BotID: "u-codex", RuntimeID: "rt-1", SessionID: "sess-1"}); err != nil {
		t.Fatalf("StartBot() error = %v", err)
	}
	defer svc.Close()

	waitFor(t, func() bool {
		return slices.Equal(client.sentTexts(), []string{"matched"})
	})
}

func TestHTTPClientDecodeSSE(t *testing.T) {
	t.Parallel()

	payload := ": connected\n\n" +
		"event: message\n" +
		"data: {\"message_id\":\"m-1\",\"room_id\":\"room-1\",\"text\":\"hello\"}\n\n"

	events := make(chan BotEvent, 1)
	if err := decodeSSE(context.Background(), strings.NewReader(payload), events); err != nil {
		t.Fatalf("decodeSSE() error = %v", err)
	}
	close(events)

	got, ok := <-events
	if !ok {
		t.Fatal("decodeSSE() produced no events")
	}
	if got.MessageID != "m-1" || got.RoomID != "room-1" || got.Text != "hello" {
		t.Fatalf("decoded event = %+v", got)
	}
}

func waitFor(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition not satisfied before timeout")
}

var _ BotClient = (*fakeBotClient)(nil)
var _ SessionPrompter = (*fakePrompter)(nil)

func TestWorkerReturnsPromptError(t *testing.T) {
	t.Parallel()

	stream := make(chan BotEvent, 1)
	errs := make(chan error)
	close(errs)
	stream <- BotEvent{MessageID: "m-1", RoomID: "room-1", Text: "hello"}

	sink := NewEventSink()
	client := &fakeBotClient{
		streams: map[string][]streamResult{
			"u-codex": {{events: stream, errs: errs}},
		},
	}
	prompter := &fakePrompter{
		prompt: func(context.Context, runtimecodex.SessionHandle, acp.PromptRequest) error {
			return errors.New("boom")
		},
	}

	svc := NewService(client, prompter, sink)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := svc.StartBot(ctx, Binding{BotID: "u-codex", RuntimeID: "rt-1", SessionID: "sess-1"}); err != nil {
		t.Fatalf("StartBot() error = %v", err)
	}
	defer svc.Close()

	waitFor(t, func() bool {
		return slices.Equal(client.sentTexts(), []string{"Codex runtime error: boom"})
	})
}
