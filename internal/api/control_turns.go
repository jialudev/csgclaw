package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"csgclaw/internal/agentengine"
	"csgclaw/internal/agentsession"
	"csgclaw/internal/legacyimport"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) registerControlRoutes(r chi.Router) {
	r.Get("/capabilities", h.getControlCapabilities)
	r.Get("/readiness", h.preflightAgent)
	r.Post("/preflight", h.preflightAgent)
	r.Get("/configuration", h.getControlConfiguration)
	r.Get("/imports", h.listLegacyImports)
	r.Post("/imports", h.createLegacyImport)
	r.Get("/imports/{import_id}", h.getLegacyImport)
	r.Patch("/configuration", h.patchControlConfiguration)
	r.Post("/configuration:apply", h.applyControlConfiguration)
	r.Post("/files", h.createControlFile)
	r.Get("/files/{file_id}", h.getControlFile)
	r.Delete("/files/{file_id}", h.deleteControlFile)
	r.Put("/workspace/file", h.mutateControlWorkspace)
	r.Delete("/workspace/file", h.mutateControlWorkspace)
	r.Get("/sessions", h.listControlSessions)
	r.Get("/sessions/{session_id}", h.getControlSession)
	r.Get("/sessions/{session_id}/history", h.getControlSession)
	r.Post("/sessions/{session_id}/turns", h.startControlTurn)
	r.Get("/sessions/{session_id}/turns/{turn_id}", h.getControlTurn)
	r.Post("/sessions/{session_id}/turns/{turn_id}/cancel", h.cancelControlTurn)
	r.Get("/sessions/{session_id}/turns/{turn_id}/events", h.streamControlTurn)
	r.Get("/sessions/{session_id}/interactions/{interaction_id}", h.controlInteraction)
	r.Post("/sessions/{session_id}/interactions/{interaction_id}/resolve", h.controlInteraction)
	r.Get("/events", h.streamControlChanges)
}
func (h *Handler) controlBinding(w http.ResponseWriter, r *http.Request, agentID string, create bool) (agentsession.Binding, bool) {
	id := controlPathID(r, "session_id")
	if !agentSessionIDPattern.MatchString(id) {
		controlError(w, 400, "invalid_session_id", "session ID must contain 1-128 path-safe characters")
		return agentsession.Binding{}, false
	}
	if h.sessionBindings == nil {
		controlError(w, 503, "session_unavailable", "session storage is unavailable")
		return agentsession.Binding{}, false
	}
	for _, binding := range h.sessionBindings.Bindings() {
		if binding.AgentID == agentID && binding.ExternalSession == id {
			return binding, true
		}
	}
	if !create {
		controlError(w, 404, "session_not_found", "session does not exist")
		return agentsession.Binding{}, false
	}
	if strings.HasPrefix(id, "observed.") || strings.HasPrefix(id, "legacy.") {
		controlError(w, 400, "invalid_session_id", "observed sessions are created by channel adapters")
		return agentsession.Binding{}, false
	}
	binding, err := h.sessionBindings.GetOrCreate(agentID, id)
	if err != nil {
		writeControlError(w, err)
		return binding, false
	}
	return binding, true
}
func (h *Handler) controlHistory(w http.ResponseWriter, agentID string) (agentengine.ConversationHistory, bool) {
	history, ok := h.agentEngine.Conversations(agentID).(agentengine.ConversationHistory)
	if !ok {
		controlError(w, 422, "history_unsupported", "execution history is not supported by this Engine")
		return nil, false
	}
	return history, true
}
func (h *Handler) listControlSessions(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	history, ok := h.controlHistory(w, a.ID)
	if !ok {
		return
	}
	records, err := history.History(r.Context(), "", 100)
	if sessions, ok := history.(interface {
		Sessions(context.Context) ([]agentengine.ConversationKey, error)
	}); ok {
		var keys []agentengine.ConversationKey
		keys, err = sessions.Sessions(r.Context())
		records = make([]agentengine.TurnRecord, 0, len(keys))
		for _, key := range keys {
			records = append(records, agentengine.TurnRecord{Request: agentengine.TurnRequest{ConversationKey: key}})
		}
	}
	if err != nil {
		writeControlError(w, err)
		return
	}
	if h.sessionBindings == nil {
		controlError(w, 503, "session_unavailable", "session storage is unavailable")
		return
	}
	bindings := map[string]agentsession.Binding{}
	for _, b := range h.sessionBindings.Bindings() {
		if b.AgentID == a.ID {
			bindings[b.ConversationKey] = b
		}
	}
	// IM room/task executions are indexed by the same Engine. Stable aliases let
	// an external UI observe them without creating a second native conversation.
	for _, record := range records {
		key := string(record.Request.ConversationKey)
		if _, exists := bindings[key]; exists {
			continue
		}
		sum := sha256.Sum256([]byte(key))
		binding := agentsession.Binding{AgentID: a.ID, ExternalSession: "observed." + hex.EncodeToString(sum[:16]), ConversationKey: key, SourceBackend: "channel", ReadOnly: true}
		if err = h.sessionBindings.BindExisting(binding); err != nil {
			writeControlError(w, err)
			return
		}
		bindings[key] = binding
	}
	out := make([]agentsession.Binding, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExternalSession < out[j].ExternalSession })
	writeJSON(w, 200, map[string]any{"items": out})
}
func (h *Handler) getControlSession(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	b, ok := h.controlBinding(w, r, a.ID, false)
	if !ok {
		return
	}
	if b.ReadOnly && strings.HasPrefix(b.ConversationKey, "import:") {
		var record legacyimport.Record
		if err := h.control.store.Read(legacyimport.Key(a.ID, strings.TrimPrefix(b.ConversationKey, "import:")), &record); err != nil {
			writeControlError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"session_id": b.ExternalSession, "agent_id": a.ID, "read_only": true, "source_backend": b.SourceBackend, "state": record.State, "legacy_messages": record.History, "files": record.Files, "items": []agentengine.TurnRecord{}})
		return
	}
	history, ok := h.controlHistory(w, a.ID)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	var records []agentengine.TurnRecord
	var err error
	if paged, ok := history.(interface {
		HistoryBefore(context.Context, agentengine.ConversationKey, int, string) ([]agentengine.TurnRecord, error)
	}); ok {
		records, err = paged.HistoryBefore(r.Context(), agentengine.ConversationKey(b.ConversationKey), limit, r.URL.Query().Get("before"))
	} else {
		records, err = history.History(r.Context(), agentengine.ConversationKey(b.ConversationKey), limit)
	}
	if err != nil {
		writeControlError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"session_id": b.ExternalSession, "agent_id": a.ID, "items": records, "next_before": oldestControlTurn(records)})
}
func (h *Handler) startControlTurn(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	var req struct {
		RequestID string                  `json:"request_id"`
		Input     []agentengine.InputPart `json:"input"`
	}
	if !decodeControl(w, r, &req) {
		return
	}
	if !agentSessionIDPattern.MatchString(req.RequestID) || len(req.Input) == 0 {
		controlError(w, 400, "invalid_request", "a path-safe request_id and input are required")
		return
	}
	b, ok := h.controlBinding(w, r, a.ID, true)
	if !ok {
		return
	}
	if b.ReadOnly {
		controlError(w, 409, "session_read_only", "continue this read-only session through its original channel")
		return
	}
	conversations := h.agentEngine.Conversations(a.ID)
	starter, ok := conversations.(agentengine.ConversationStarter)
	if !ok {
		controlError(w, 422, "turns_unsupported", "asynchronous execution is not supported")
		return
	}
	ctx, cancel := context.WithTimeout(h.control.ctx, 30*time.Minute)
	request := agentengine.TurnRequest{ID: agentengine.TurnID(req.RequestID), ConversationKey: agentengine.ConversationKey(b.ConversationKey), Input: req.Input, Admission: agentengine.AdmissionRejectIfBusy, Interaction: agentengine.InteractionResolve}
	if err := starter.Start(ctx, request); err != nil {
		cancel()
		writeControlError(w, err)
		return
	}
	// Release the timeout as soon as execution completes; losing the HTTP client
	// does not cancel the admitted task.
	go func() {
		defer cancel()
		if history, ok := conversations.(agentengine.ConversationHistory); ok {
			_, _ = history.Subscribe(ctx, request.ConversationKey, request.ID, ^uint64(0), nil)
		}
	}()
	writeJSON(w, 202, map[string]any{"agent_id": a.ID, "session_id": b.ExternalSession, "turn_id": req.RequestID})
}
func (h *Handler) getControlTurn(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	b, ok := h.controlBinding(w, r, a.ID, false)
	if !ok {
		return
	}
	history, ok := h.controlHistory(w, a.ID)
	if !ok {
		return
	}
	record, err := history.Turn(r.Context(), agentengine.ConversationKey(b.ConversationKey), agentengine.TurnID(controlPathID(r, "turn_id")))
	if err != nil {
		writeControlError(w, err)
		return
	}
	writeJSON(w, 200, record)
}
func (h *Handler) cancelControlTurn(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	b, ok := h.controlBinding(w, r, a.ID, false)
	if !ok {
		return
	}
	err := h.agentEngine.Conversations(a.ID).Cancel(r.Context(), agentengine.ConversationKey(b.ConversationKey), agentengine.TurnID(controlPathID(r, "turn_id")))
	if err != nil {
		writeControlError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) controlInteraction(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	b, ok := h.controlBinding(w, r, a.ID, false)
	if !ok {
		return
	}
	if b.ReadOnly && r.Method == http.MethodPost {
		controlError(w, 409, "session_read_only", "answer this interaction through its original channel")
		return
	}
	c := h.agentEngine.Conversations(a.ID)
	key := agentengine.ConversationKey(b.ConversationKey)
	id := controlPathID(r, "interaction_id")
	if r.Method == http.MethodGet {
		item, err := c.GetInteraction(r.Context(), key, id)
		if err != nil {
			writeControlError(w, err)
			return
		}
		writeJSON(w, 200, item)
		return
	}
	var req struct {
		OptionID string                                   `json:"option_id"`
		Answers  map[string]agentengine.InteractionAnswer `json:"answers"`
	}
	if !decodeControl(w, r, &req) {
		return
	}
	err := c.Resolve(r.Context(), agentengine.InteractionResolution{ConversationKey: key, InteractionID: id, OptionID: req.OptionID, Answers: req.Answers})
	if err != nil {
		writeControlError(w, err)
		return
	}
	w.WriteHeader(204)
}
func controlSSE(w http.ResponseWriter, event string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body); err != nil {
		return err
	}
	return http.NewResponseController(w).Flush()
}
func beginControlSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
}
func (h *Handler) streamControlTurn(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	b, ok := h.controlBinding(w, r, a.ID, false)
	if !ok {
		return
	}
	history, ok := h.controlHistory(w, a.ID)
	if !ok {
		return
	}
	key, id := agentengine.ConversationKey(b.ConversationKey), agentengine.TurnID(controlPathID(r, "turn_id"))
	if _, err := history.Turn(r.Context(), key, id); err != nil {
		writeControlError(w, err)
		return
	}
	after, _ := strconv.ParseUint(r.URL.Query().Get("after"), 10, 64)
	beginControlSSE(w)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	events := make(chan agentengine.TurnEvent, 64)
	type completion struct {
		record agentengine.TurnRecord
		err    error
	}
	done := make(chan completion, 1)
	go func() {
		record, err := history.Subscribe(ctx, key, id, after, agentengine.EventSinkFunc(func(ctx context.Context, event agentengine.TurnEvent) error {
			select {
			case events <- event:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
		done <- completion{record, err}
		close(events)
	}()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				result := <-done
				if result.err == nil {
					_ = controlSSE(w, "turn_completed", result.record)
				}
				return
			}
			if controlSSE(w, "turn_event", event) != nil {
				return
			}
		case <-ticker.C:
			if controlSSE(w, "heartbeat", map[string]bool{"alive": true}) != nil {
				return
			}
		case <-r.Context().Done():
			return
		case <-h.eventStreamShutdown:
			return
		}
	}
}
func (h *Handler) streamControlChanges(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	beginControlSSE(w)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	last := ""
	heartbeat := 0
	for {
		current, err := h.agentEngine.Agents().Get(r.Context(), a.ID, agentengine.AgentGetOptions{})
		if err != nil {
			return
		}
		// Poll the authoritative projection so legacy write paths are also observed.
		records := []agentengine.TurnRecord{}
		if latest, ok := h.agentEngine.Conversations(a.ID).(interface {
			LatestTurn(context.Context) (agentengine.TurnRecord, bool, error)
		}); ok {
			var record agentengine.TurnRecord
			var found bool
			record, found, err = latest.LatestTurn(r.Context())
			if found {
				records = append(records, record)
			}
			if err != nil {
				return
			}
		} else if history, ok := h.agentEngine.Conversations(a.ID).(agentengine.ConversationHistory); ok {
			records, err = history.History(r.Context(), "", 1)
			if err != nil {
				return
			}
		}
		state := map[string]any{"configuration_revision": configurationRevision(current), "ready": current.Status.Ready, "state": current.Status.State}
		if len(records) > 0 {
			state["latest_turn_id"] = records[0].Request.ID
			state["turn_status"] = records[0].Status
			state["updated_at"] = records[0].UpdatedAt
		}
		raw, _ := json.Marshal(state)
		if string(raw) != last {
			if controlSSE(w, "state_changed", state) != nil {
				return
			}
			last = string(raw)
		}
		heartbeat++
		if heartbeat%8 == 0 {
			if controlSSE(w, "heartbeat", map[string]bool{"alive": true}) != nil {
				return
			}
		}
		select {
		case <-r.Context().Done():
			return
		case <-h.eventStreamShutdown:
			return
		case <-ticker.C:
		}
	}
}

func oldestControlTurn(records []agentengine.TurnRecord) string {
	if len(records) == 0 {
		return ""
	}
	return string(records[len(records)-1].Request.ID)
}
