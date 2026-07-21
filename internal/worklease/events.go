package worklease

import (
	"sync"

	"csgclaw/internal/apitypes"
)

const EventTypeParticipantWorkUpdated = "participant.work.updated"

type Event struct {
	Type   string                         `json:"type"`
	RoomID string                         `json:"room_id"`
	Work   apitypes.ParticipantWorkUpdate `json:"work"`
}

type Bus struct {
	mu          sync.Mutex
	nextID      int
	subscribers map[int]chan Event
}

func NewBus() *Bus {
	return &Bus{subscribers: make(map[int]chan Event)}
}

func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 32)
	if b == nil {
		close(ch)
		return ch, func() {}
	}

	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.subscribers[id] = ch
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		delete(b.subscribers, id)
		b.mu.Unlock()
	}
}

func (b *Bus) Publish(event Event) {
	if b == nil {
		return
	}
	b.mu.Lock()
	targets := make([]chan Event, 0, len(b.subscribers))
	for _, subscriber := range b.subscribers {
		targets = append(targets, subscriber)
	}
	b.mu.Unlock()

	for _, subscriber := range targets {
		select {
		case subscriber <- event:
		default:
		}
	}
}
