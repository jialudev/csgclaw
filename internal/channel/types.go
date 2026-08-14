package channel

import (
	"strings"

	"csgclaw/internal/agentengine"
)

// ChannelID identifies a channel implementation, such as csgclaw or feishu.
type ChannelID string

const (
	ChannelCSGClaw ChannelID = "csgclaw"
	ChannelFeishu  ChannelID = "feishu"
)

// BindingID is a stable channel binding identity, not a Runtime or Session ID.
type BindingID string

// SourceEventID is the source-side dedupe key for a message or action.
type SourceEventID string

// ActorRef is a source sender or interaction actor.
type ActorRef struct {
	ID          string
	DisplayName string
	IsBot       bool
}

// ConversationScope is channel context used to build an Engine ConversationKey.
type ConversationScope struct {
	BindingID BindingID
	RoomID    string
	ThreadID  string
	ReplyToID string
}

// Binding connects a channel identity to one CSGClaw Agent.
type Binding struct {
	ID            BindingID
	Channel       ChannelID
	AgentID       string
	ParticipantID string
	Enabled       bool
}

// StableID returns the persistent binding identity, falling back to ParticipantID.
func (b Binding) StableID() BindingID {
	if id := BindingID(strings.TrimSpace(string(b.ID))); id != "" {
		return id
	}
	return BindingID(strings.TrimSpace(b.ParticipantID))
}

// TurnContext is channel-owned identity for rendering and source delivery.
// SourceMessageID is a dedupe key, not a TurnID.
type TurnContext struct {
	BindingID       BindingID
	ParticipantID   string
	AgentID         string
	RoomID          string
	Locale          string
	ChatType        string
	ThreadRootID    string
	SourceMessageID string
	ConversationKey agentengine.ConversationKey
	TurnID          agentengine.TurnID
}

// Outcome keeps the channel context next to the normalized Engine result.
type Outcome struct {
	Turn   TurnContext
	Result agentengine.TurnResult
}
