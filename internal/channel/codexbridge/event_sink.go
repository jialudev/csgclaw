package codexbridge

import (
	"strings"
	"sync"

	runtimecodex "csgclaw/internal/runtime/codex"
)

const defaultEventBuffer = 64

// EventSink fans out normalized Codex session events to bridge workers.
type EventSink struct {
	mu          sync.Mutex
	nextID      int
	subscribers map[int]subscription
}

type subscription struct {
	runtimeID string
	ch        chan runtimecodex.SessionEvent
}

func NewEventSink() *EventSink {
	return &EventSink{
		subscribers: make(map[int]subscription),
	}
}

func (s *EventSink) Publish(event runtimecodex.SessionEvent) {
	if s == nil {
		return
	}

	runtimeID := strings.TrimSpace(event.RuntimeID)

	s.mu.Lock()
	targets := make([]chan runtimecodex.SessionEvent, 0, len(s.subscribers))
	for _, sub := range s.subscribers {
		if sub.runtimeID != "" && sub.runtimeID != runtimeID {
			continue
		}
		targets = append(targets, sub.ch)
	}
	s.mu.Unlock()

	for _, ch := range targets {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *EventSink) Subscribe(runtimeID string) (<-chan runtimecodex.SessionEvent, func()) {
	ch := make(chan runtimecodex.SessionEvent, defaultEventBuffer)
	if s == nil {
		close(ch)
		return ch, func() {}
	}

	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.subscribers[id] = subscription{
		runtimeID: strings.TrimSpace(runtimeID),
		ch:        ch,
	}
	s.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			s.mu.Lock()
			if sub, ok := s.subscribers[id]; ok {
				delete(s.subscribers, id)
				close(sub.ch)
			}
			s.mu.Unlock()
		})
	}
	return ch, cancel
}
