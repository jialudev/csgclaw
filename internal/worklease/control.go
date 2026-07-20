package worklease

import (
	"strings"
	"sync"
	"time"
)

const ControlEventTypeParticipantWorkStopRequested = "participant.work.stop_requested"

type ControlEvent struct {
	RegistryEpoch string    `json:"registry_epoch"`
	ParticipantID string    `json:"participant_id"`
	RoomID        string    `json:"room_id"`
	LeaseID       string    `json:"lease_id"`
	RequestID     string    `json:"request_id"`
	RequestedAt   time.Time `json:"requested_at"`
}

// ControlBus is a bounded, best-effort path for participant-directed runtime
// controls. The Registry remains the source of truth when a live event is lost.
type ControlBus struct {
	mu          sync.Mutex
	nextID      int
	subscribers map[string]map[int]chan ControlEvent
}

func NewControlBus() *ControlBus {
	return &ControlBus{subscribers: make(map[string]map[int]chan ControlEvent)}
}

func (b *ControlBus) Subscribe(participantID string) (<-chan ControlEvent, func()) {
	ch := make(chan ControlEvent, 16)
	participantID = strings.TrimSpace(participantID)
	if b == nil || participantID == "" {
		close(ch)
		return ch, func() {}
	}

	b.mu.Lock()
	id := b.nextID
	b.nextID++
	byID := b.subscribers[participantID]
	if byID == nil {
		byID = make(map[int]chan ControlEvent)
		b.subscribers[participantID] = byID
	}
	byID[id] = ch
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		if byID := b.subscribers[participantID]; byID != nil {
			delete(byID, id)
			if len(byID) == 0 {
				delete(b.subscribers, participantID)
			}
		}
		b.mu.Unlock()
	}
}

func (b *ControlBus) Publish(event ControlEvent) {
	if b == nil {
		return
	}
	b.mu.Lock()
	targets := make([]chan ControlEvent, 0, len(b.subscribers[event.ParticipantID]))
	for _, subscriber := range b.subscribers[event.ParticipantID] {
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
