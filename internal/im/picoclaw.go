package im

import (
	"encoding/json"
	"fmt"
	"sync"
)

type PicoClawBridge struct {
	mu          sync.Mutex
	subscribers map[string]map[chan PicoClawEvent]struct{}
	pending     map[string][]PicoClawEvent
}

const picoClawEventBufferSize = 64

type PicoClawEvent struct {
	MessageID string         `json:"message_id"`
	RoomID    string         `json:"room_id"`
	ChatType  string         `json:"chat_type"`
	Sender    PicoClawSender `json:"sender"`
	Text      string         `json:"text"`
	Timestamp string         `json:"timestamp"`
	Mentions  []string       `json:"mentions,omitempty"`
}

type PicoClawSender struct {
	ID          string `json:"id"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

type PicoClawSendMessageRequest struct {
	RoomID string `json:"room_id"`
	Text   string `json:"text"`
}

func NewPicoClawBridge(string) *PicoClawBridge {
	return &PicoClawBridge{
		subscribers: make(map[string]map[chan PicoClawEvent]struct{}),
		pending:     make(map[string][]PicoClawEvent),
	}
}

func (b *PicoClawBridge) Subscribe(botID string) (<-chan PicoClawEvent, func()) {
	ch := make(chan PicoClawEvent, picoClawEventBufferSize)

	b.mu.Lock()
	for _, evt := range b.pending[botID] {
		ch <- evt
	}
	delete(b.pending, botID)
	if b.subscribers[botID] == nil {
		b.subscribers[botID] = make(map[chan PicoClawEvent]struct{})
	}
	b.subscribers[botID][ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if subs, ok := b.subscribers[botID]; ok {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(b.subscribers, botID)
			}
		}
		close(ch)
		b.mu.Unlock()
	}
	return ch, cancel
}

func (b *PicoClawBridge) PublishMessageEvent(room Room, sender User, message Message) {
	b.mu.Lock()
	defer b.mu.Unlock()

	seen := make(map[string]struct{}, len(room.Members))
	for _, botID := range room.Members {
		if _, ok := seen[botID]; ok {
			continue
		}
		seen[botID] = struct{}{}
		if !shouldNotifyBot(room, message, botID) {
			continue
		}

		evt := PicoClawEvent{
			MessageID: message.ID,
			RoomID:    room.ID,
			ChatType:  chatTypeForRoom(room),
			Sender: PicoClawSender{
				ID:          sender.ID,
				Username:    sender.Handle,
				DisplayName: sender.Name,
			},
			Text:      message.Content,
			Timestamp: fmt.Sprintf("%d", message.CreatedAt.UnixMilli()),
			Mentions:  mentionsForBot(message.Mentions, botID),
		}

		subs := b.subscribers[botID]
		if len(subs) == 0 {
			b.appendPendingLocked(botID, evt)
			continue
		}

		delivered := false
		for ch := range subs {
			select {
			case ch <- evt:
				delivered = true
			default:
			}
		}
		if !delivered {
			b.appendPendingLocked(botID, evt)
		}
	}
}

func (b *PicoClawBridge) appendPendingLocked(botID string, evt PicoClawEvent) {
	events := append(b.pending[botID], evt)
	if len(events) > picoClawEventBufferSize {
		events = events[len(events)-picoClawEventBufferSize:]
	}
	b.pending[botID] = events
}

func (e PicoClawEvent) MarshalJSONLine() ([]byte, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func shouldNotifyBot(room Room, message Message, botID string) bool {
	if message.SenderID == botID {
		return false
	}
	if !containsUserIDInRoom(room, botID) {
		return false
	}
	return true
}

func mentionsForBot(mentions []Mention, botID string) []string {
	if len(mentions) == 0 {
		return nil
	}
	result := make([]string, 0, len(mentions))
	for _, mention := range mentions {
		if mention.ID == botID {
			result = append(result, mention.ID)
		}
	}
	return result
}

func chatTypeForRoom(room Room) string {
	if room.IsDirect {
		return "direct"
	}
	return "group"
}
