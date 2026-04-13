package channel

import (
	"sync"

	"csgclaw/internal/im"
)

const FeishuMessageEventTypeMessageCreated = "message.created"

type FeishuMessageEvent struct {
	Type    string      `json:"type"`
	RoomID  string      `json:"room_id,omitempty"`
	Message *im.Message `json:"message,omitempty"`
}

type FeishuMessageBus struct {
	mu          sync.Mutex
	nextID      int
	subscribers map[int]chan FeishuMessageEvent
}

func NewFeishuMessageBus() *FeishuMessageBus {
	return &FeishuMessageBus{
		subscribers: make(map[int]chan FeishuMessageEvent),
	}
}

func (b *FeishuMessageBus) Subscribe() (<-chan FeishuMessageEvent, func()) {
	ch := make(chan FeishuMessageEvent, 16)

	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.subscribers[id] = ch
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if existing, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(existing)
		}
		b.mu.Unlock()
	}

	return ch, cancel
}

func (b *FeishuMessageBus) Publish(evt FeishuMessageEvent) {
	if b == nil {
		return
	}

	b.mu.Lock()
	targets := make([]chan FeishuMessageEvent, 0, len(b.subscribers))
	for _, ch := range b.subscribers {
		targets = append(targets, ch)
	}
	b.mu.Unlock()

	for _, ch := range targets {
		select {
		case ch <- evt:
		default:
		}
	}
}
