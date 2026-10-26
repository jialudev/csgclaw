package agentengine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type historyResolver struct{ adapter *historyAdapter }

func (r historyResolver) conversationRuntime(context.Context, string) (conversationRuntimeAdapter, func(), *TurnError) {
	return r.adapter, func() {}, nil
}

type historyAdapter struct {
	calls   atomic.Int32
	release <-chan struct{}
}

func (r *historyAdapter) Run(ctx context.Context, request TurnRequest, sink EventSink) TurnResult {
	r.calls.Add(1)
	if sink != nil {
		sink.Emit(ctx, TurnEvent{Kind: TurnEventTextDelta, Text: "hello"})
	}
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			return TurnResult{Status: TurnCanceled, Dispatched: true}
		}
	}
	return TurnResult{Status: TurnSucceeded, Output: "hello", Dispatched: true}
}
func (*historyAdapter) Reset(context.Context, ConversationKey) *TurnError { return nil }
func (*historyAdapter) Resolve(context.Context, InteractionRequest, InteractionResolution) *TurnError {
	return nil
}
func historyEngine(adapter *historyAdapter) *Engine {
	return &Engine{files: NewFileStore(), runtimes: historyResolver{adapter}}
}
func TestPersistentTurnReplayAndUnknownExecution(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	adapter := &historyAdapter{}
	e := historyEngine(adapter)
	if err := e.EnablePersistence(dir); err != nil {
		t.Fatal(err)
	}
	req := TurnRequest{ID: "t", ConversationKey: "c", Input: []InputPart{{Kind: InputPartText, Text: "hello"}}}
	if r := e.Conversations("a").Run(ctx, req, nil); r.Status != TurnSucceeded {
		t.Fatalf("run: %+v", r)
	}
	e2 := historyEngine(adapter)
	if err := e2.EnablePersistence(dir); err != nil {
		t.Fatal(err)
	}
	if r := e2.Conversations("a").Run(ctx, req, nil); r.Output != "hello" || adapter.calls.Load() != 1 {
		t.Fatalf("duplicate dispatch: %+v %d", r, adapter.calls.Load())
	}
	turn := newActiveTurn(ctx, TurnRequest{ID: "unknown", ConversationKey: "c", Input: req.Input})
	turn.agentID = "a"
	if err := e2.saveTurn(turn, "running", nil); err != nil {
		t.Fatal(err)
	}
	turn.cancel()
	e3 := historyEngine(adapter)
	if err := e3.EnablePersistence(dir); err != nil {
		t.Fatal(err)
	}
	record, err := e3.Conversations("a").(ConversationHistory).Turn(ctx, "c", "unknown")
	if err != nil || record.Status != "execution_unknown" {
		t.Fatalf("recovery: %+v %v", record, err)
	}
	req.ID = "unknown"
	if r := e3.Conversations("a").Run(ctx, req, nil); r.Error == nil || adapter.calls.Load() != 1 {
		t.Fatal("unknown execution was dispatched")
	}
}
func TestLiveSubscriptionNeverRunsASecondTurn(t *testing.T) {
	release := make(chan struct{})
	adapter := &historyAdapter{release: release}
	e := historyEngine(adapter)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c := e.Conversations("a")
	req := TurnRequest{ID: "t", ConversationKey: "c", Input: []InputPart{{Kind: InputPartText, Text: "hello"}}}
	if err := c.(ConversationStarter).Start(ctx, req); err != nil {
		t.Fatal(err)
	}
	seen := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		_, err := c.(ConversationHistory).Subscribe(ctx, "c", "t", 0, EventSinkFunc(func(context.Context, TurnEvent) error { seen <- struct{}{}; return nil }))
		done <- err
	}()
	select {
	case <-seen:
	case <-ctx.Done():
		t.Fatal("live event was not delivered")
	}
	close(release)
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if adapter.calls.Load() != 1 {
		t.Fatal("subscriber dispatched a task")
	}
}

func TestEngineShutdownCancelsAndPersistsAcceptedWork(t *testing.T) {
	release := make(chan struct{})
	adapter := &historyAdapter{release: release}
	e := historyEngine(adapter)
	if err := e.EnablePersistence(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c := e.Conversations("a")
	request := TurnRequest{ID: "shutdown", ConversationKey: "c", Input: []InputPart{{Kind: InputPartText, Text: "work"}}}
	if err := c.(ConversationStarter).Start(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := e.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	record, err := c.(ConversationHistory).Turn(ctx, "c", "shutdown")
	if err != nil || record.Status != "canceled" {
		t.Fatalf("shutdown record: %+v %v", record, err)
	}
	request.ID = "late"
	if result := c.Run(ctx, request, nil); result.Error == nil {
		t.Fatal("Engine accepted work after shutdown")
	}
}
func TestPersistentHistoryPaginationUsesAllIndexedSessions(t *testing.T) {
	e := historyEngine(&historyAdapter{})
	if err := e.EnablePersistence(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c := e.Conversations("a")
	for _, id := range []TurnID{"one", "two", "three"} {
		result := c.Run(ctx, TurnRequest{ID: id, ConversationKey: "c", Input: []InputPart{{Kind: InputPartText, Text: string(id)}}}, nil)
		if result.Error != nil {
			t.Fatal(result.Error)
		}
	}
	first, err := c.(*conversations).HistoryBefore(ctx, "c", 2, "")
	if err != nil || len(first) != 2 {
		t.Fatalf("first page: %v %v", first, err)
	}
	second, err := c.(*conversations).HistoryBefore(ctx, "c", 2, string(first[1].Request.ID))
	if err != nil || len(second) != 1 || second[0].Request.ID != "one" {
		t.Fatalf("second page: %v %v", second, err)
	}
	latest, ok, err := c.(*conversations).LatestTurn(ctx)
	if err != nil || !ok || latest.Request.ID != "three" || len(latest.Events) != 0 {
		t.Fatalf("latest metadata: %+v %v", latest, err)
	}
}
