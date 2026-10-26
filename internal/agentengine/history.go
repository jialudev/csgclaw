package agentengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"csgclaw/internal/agentengine/contract"
	"csgclaw/internal/controlplane"
)

// TurnRecord is an immutable snapshot. Running records found after a crash are
// marked execution_unknown and are never automatically dispatched again.
type TurnRecord struct {
	RequestDigest string      `json:"request_digest"`
	AgentID       string      `json:"agent_id"`
	Request       TurnRequest `json:"request"`
	Status        string      `json:"status"`
	Events        []TurnEvent `json:"events,omitempty"`
	Result        *TurnResult `json:"result,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// ConversationHistory is optional so existing Engine clients remain compatible.
type ConversationHistory interface {
	History(context.Context, ConversationKey, int) ([]TurnRecord, error)
	Turn(context.Context, ConversationKey, TurnID) (TurnRecord, error)
	Subscribe(context.Context, ConversationKey, TurnID, uint64, EventSink) (TurnRecord, error)
}

type historySummary struct {
	Key                  turnIdentity
	CreatedAt, UpdatedAt time.Time
	Status               string
	Refs                 map[string]bool
}

func (e *Engine) indexRecord(record TurnRecord) {
	if e.historyIndex == nil {
		e.historyIndex = map[turnIdentity]historySummary{}
	}
	if e.historyRefs == nil {
		e.historyRefs = map[string]map[string]int{}
	}
	key := turnIdentity{conversationIdentity: conversationIdentity{agentID: record.AgentID, key: record.Request.ConversationKey}, id: record.Request.ID}
	if e.historyRefs[record.AgentID] == nil {
		e.historyRefs[record.AgentID] = map[string]int{}
	}
	for id := range e.historyIndex[key].Refs {
		e.historyRefs[record.AgentID][id]--
	}
	refs := map[string]bool{}
	for _, part := range record.Request.Input {
		if part.File != nil {
			refs[part.File.ID] = true
		}
	}
	if record.Result != nil {
		for _, file := range record.Result.Files {
			refs[file.ID] = true
		}
	}
	for id := range refs {
		e.historyRefs[record.AgentID][id]++
	}
	e.historyIndex[key] = historySummary{Key: key, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, Refs: refs, Status: record.Status}
}

// EnablePersistence must run during composition, before accepting any requests.
func (e *Engine) EnablePersistence(dir string) error {
	s, err := controlplane.Open(filepath.Join(dir, "turns"))
	if err != nil {
		return err
	}
	if err = e.files.EnablePersistence(filepath.Join(dir, "files")); err != nil {
		s.Close()
		return err
	}
	err = s.Walk(func(raw json.RawMessage) error {
		var record TurnRecord
		if json.Unmarshal(raw, &record) != nil || record.AgentID == "" || record.Request.ID == "" || record.Request.ConversationKey == "" {
			return fmt.Errorf("invalid execution record")
		}
		if record.Status == "running" {
			record.Status = "execution_unknown"
			record.UpdatedAt = time.Now().UTC()
			if err := s.Write(historyKey(record.AgentID, record.Request.ConversationKey, record.Request.ID), record); err != nil {
				return err
			}
		}
		e.indexRecord(record)
		return nil
	})
	if err != nil {
		s.Close()
		return err
	}
	e.history = s
	return nil
}
func historyKey(agent string, key ConversationKey, id TurnID) string {
	return agent + "\x00" + string(key) + "\x00" + string(id)
}
func (e *Engine) savedTurn(agent string, key ConversationKey, id TurnID) (TurnRecord, error) {
	if e.history == nil {
		return TurnRecord{}, os.ErrNotExist
	}
	var r TurnRecord
	err := e.history.Read(historyKey(agent, key, id), &r)
	if err == nil && (r.AgentID != agent || r.Request.ConversationKey != key || r.Request.ID != id) {
		return TurnRecord{}, fmt.Errorf("execution record identity does not match")
	}
	return r, err
}
func (e *Engine) saveTurn(turn *activeTurn, status string, result *TurnResult) error {
	if e.history == nil {
		return nil
	}
	r := TurnRecord{RequestDigest: requestDigest(turn.request), AgentID: turn.agentID, Request: cloneTurnRequest(turn.request), Status: status, Events: cloneTurnEvents(turn.events), Result: result, CreatedAt: turn.createdAt, UpdatedAt: time.Now().UTC()}
	if err := e.history.Write(historyKey(turn.agentID, turn.request.ConversationKey, turn.request.ID), r); err != nil {
		return err
	}
	e.indexRecord(r)
	return nil
}
func (c *conversations) Turn(ctx context.Context, key ConversationKey, id TurnID) (TurnRecord, error) {
	if err := ctx.Err(); err != nil {
		return TurnRecord{}, err
	}
	e := c.engine
	e.mu.Lock()
	defer e.mu.Unlock()
	if turn := e.active[conversationIdentity{agentID: c.agentID, key: key}]; turn != nil && turn.request.ID == id {
		return TurnRecord{AgentID: c.agentID, Request: cloneTurnRequest(turn.request), Status: "running", Events: cloneTurnEvents(turn.events), CreatedAt: turn.createdAt, UpdatedAt: time.Now().UTC()}, nil
	}
	if record, ok := e.completed[turnIdentity{conversationIdentity: conversationIdentity{agentID: c.agentID, key: key}, id: id}]; ok {
		if e.history != nil && (record.result.Error == nil || record.result.Error.Code != ErrorExecutionUnknown) {
			if saved, err := e.savedTurn(c.agentID, key, id); err == nil {
				return saved, nil
			}
		}
		result := cloneTurnResult(record.result)
		state := string(result.Status)
		if result.Error != nil && result.Error.Code == ErrorExecutionUnknown {
			state = "execution_unknown"
		}
		return TurnRecord{AgentID: c.agentID, Request: cloneTurnRequest(record.request), Events: cloneTurnEvents(record.events), Result: &result, Status: state}, nil
	}
	if record, err := e.savedTurn(c.agentID, key, id); err == nil || !errors.Is(err, os.ErrNotExist) {
		if err == nil && record.Status == "running" {
			record.Status = "execution_unknown"
		}
		return record, err
	}
	return TurnRecord{}, os.ErrNotExist
}
func (c *conversations) History(ctx context.Context, key ConversationKey, limit int) ([]TurnRecord, error) {
	return c.HistoryBefore(ctx, key, limit, "")
}

// HistoryBefore accepts the oldest returned record ID as an exclusive cursor.
func (c *conversations) HistoryBefore(ctx context.Context, key ConversationKey, limit int, before string) ([]TurnRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	out := []TurnRecord{}
	e := c.engine
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.history != nil {
		entries := []historySummary{}
		for _, entry := range e.historyIndex {
			if entry.Key.agentID == c.agentID && (key == "" || entry.Key.key == key) {
				entries = append(entries, entry)
			}
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].UpdatedAt.Equal(entries[j].UpdatedAt) {
				return entries[i].Key.id > entries[j].Key.id
			}
			return entries[i].UpdatedAt.After(entries[j].UpdatedAt)
		})
		if before != "" {
			found := false
			for i, entry := range entries {
				if string(entry.Key.id) == before {
					entries = entries[i+1:]
					found = true
					break
				}
			}
			if !found {
				return nil, &TurnError{Code: ErrorInvalidRequest, Message: "history cursor was not found"}
			}
		}
		if len(entries) > limit {
			entries = entries[:limit]
		}
		for _, entry := range entries {
			record, err := e.savedTurn(c.agentID, entry.Key.key, entry.Key.id)
			if err != nil {
				return nil, err
			}
			record.Events = nil
			if record.Status == "running" && (e.active[entry.Key.conversationIdentity] == nil || e.active[entry.Key.conversationIdentity].request.ID != entry.Key.id) {
				record.Status = "execution_unknown"
			}
			out = append(out, record)
		}
		return out, nil
	} else {
		for k, r := range e.completed {
			if k.agentID != c.agentID || (key != "" && k.key != key) {
				continue
			}
			result := cloneTurnResult(r.result)
			out = append(out, TurnRecord{AgentID: c.agentID, Request: cloneTurnRequest(r.request), Result: &result, Status: string(result.Status)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if before != "" {
		found := false
		for i, r := range out {
			if string(r.Request.ID) == before {
				out = out[i+1:]
				found = true
				break
			}
		}
		if !found {
			return nil, &TurnError{Code: ErrorInvalidRequest, Message: "history cursor was not found"}
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Subscribe reads an admitted execution; it never invokes Run or an Adapter.
func (c *conversations) Subscribe(ctx context.Context, key ConversationKey, id TurnID, after uint64, sink EventSink) (TurnRecord, error) {
	for {
		e := c.engine
		e.mu.Lock()
		var changed <-chan struct{}
		if t := e.active[conversationIdentity{agentID: c.agentID, key: key}]; t != nil && t.request.ID == id {
			changed = t.changed
		}
		e.mu.Unlock()
		r, err := c.Turn(ctx, key, id)
		if err != nil {
			return TurnRecord{}, err
		}
		for _, event := range r.Events {
			if event.Sequence <= after {
				continue
			}
			if sink != nil {
				if err = sink.Emit(ctx, event); err != nil {
					return r, err
				}
			}
			after = event.Sequence
		}
		if r.Status != "running" {
			return r, nil
		}
		if changed == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return r, ctx.Err()
		case <-changed:
		}
	}
}

// ConversationStarter admits a turn before acknowledging it to the caller.
type ConversationStarter interface {
	Start(context.Context, TurnRequest) error
}
type startNotificationKey struct{}

func (c *conversations) Start(ctx context.Context, request TurnRequest) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ready := make(chan error, 1)
	var once sync.Once
	runCtx := context.WithValue(ctx, startNotificationKey{}, func(err error) { once.Do(func() { ready <- err }) })
	go c.Run(runCtx, request, nil)
	select {
	case err := <-ready:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// FileReferenced protects both input and generated files retained by history.
func (e *Engine) FileReferenced(agentID, fileID string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	check := func(request TurnRequest, result *TurnResult) bool {
		for _, p := range request.Input {
			if p.File != nil && p.File.ID == fileID {
				return true
			}
		}
		if result != nil {
			for _, f := range result.Files {
				if f.ID == fileID {
					return true
				}
			}
		}
		return false
	}
	for k, t := range e.active {
		if k.agentID == agentID && check(t.request, nil) {
			return true, nil
		}
	}
	if e.historyRefs[agentID][fileID] > 0 {
		return true, nil
	}
	return false, nil
}

func requestDigest(request TurnRequest) string {
	raw, _ := json.Marshal(struct {
		Request TurnRequest
		Room    *contract.RoomConfiguration
	}{request, request.RoomConfiguration})
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
func (r TurnRecord) matches(request TurnRequest) bool {
	if r.RequestDigest != "" {
		return r.RequestDigest == requestDigest(request)
	}
	return sameTurnRequest(r.Request, request)
}

// ClosePersistence is called by the host after HTTP and Engine work stop.
func (e *Engine) ClosePersistence() error {
	if e == nil {
		return nil
	}
	fileErr := e.files.ClosePersistence()
	if e.history != nil {
		if err := e.history.Close(); err != nil {
			return err
		}
	}
	return fileErr
}

// Sessions lists identities without scanning or loading historical event bodies.
func (c *conversations) Sessions(ctx context.Context) ([]ConversationKey, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.engine.mu.Lock()
	defer c.engine.mu.Unlock()
	keys := map[ConversationKey]bool{}
	for key := range c.engine.historyIndex {
		if key.agentID == c.agentID {
			keys[key.key] = true
		}
	}
	for key := range c.engine.completed {
		if key.agentID == c.agentID {
			keys[key.key] = true
		}
	}
	out := make([]ConversationKey, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// Shutdown rejects new executions and waits for already admitted work to record cancellation.
func (e *Engine) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	e.closed = true
	done := []<-chan struct{}{}
	for _, turn := range e.active {
		turn.cancel()
		done = append(done, turn.done)
	}
	e.mu.Unlock()
	for _, ch := range done {
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// LatestTurn returns only state metadata for change notifications, without
// reading historical event or result bodies from disk on every poll.
func (c *conversations) LatestTurn(ctx context.Context) (TurnRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return TurnRecord{}, false, err
	}
	c.engine.mu.Lock()
	defer c.engine.mu.Unlock()
	var latest historySummary
	found := false
	for _, entry := range c.engine.historyIndex {
		if entry.Key.agentID == c.agentID && (!found || entry.UpdatedAt.After(latest.UpdatedAt)) {
			latest = entry
			found = true
		}
	}
	if !found {
		return TurnRecord{}, false, nil
	}
	state := latest.Status
	if state == "running" && (c.engine.active[latest.Key.conversationIdentity] == nil || c.engine.active[latest.Key.conversationIdentity].request.ID != latest.Key.id) {
		state = "execution_unknown"
	}
	return TurnRecord{AgentID: c.agentID, Request: TurnRequest{ID: latest.Key.id, ConversationKey: latest.Key.key}, Status: state, CreatedAt: latest.CreatedAt, UpdatedAt: latest.UpdatedAt}, true, nil
}
