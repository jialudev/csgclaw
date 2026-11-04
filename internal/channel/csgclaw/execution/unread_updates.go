package execution

import (
	"context"
	"fmt"

	"csgclaw/internal/agentengine/contract"
	"csgclaw/internal/apitypes"
	"csgclaw/internal/channel"
)

type unreadUpdateReader func(roomID, participantID, threadRootID string, after uint64) (apitypes.UnreadNotification, bool)

func WithUnreadUpdates(reader unreadUpdateReader) Option {
	return func(a *Adapter) { a.unreadUpdates = reader }
}

// WithUnreadSignal binds the pending unread check to the same scope as the
// current turn. The queued notification remains available for normal admission.
func (a *Adapter) WithUnreadSignal(ctx context.Context, binding channel.Binding, event channel.Event, ready <-chan struct{}) context.Context {
	if !event.NotificationOnly || ready == nil || a.unreadUpdates == nil {
		return ctx
	}
	return contract.WithTurnInputUpdates(ctx, contract.TurnInputUpdates{
		Ready: ready,
		Latest: func(after uint64) (uint64, string) {
			notice, ok := a.unreadUpdates(event.RoomID, binding.ParticipantID, event.ThreadRootID, after)
			if !ok {
				return after, ""
			}
			return notice.Through, fmt.Sprintf(
				"[CSGClaw unread notice] %d unread. Use messages_read for your current room and thread, then messages_ack. Reassess your work before continuing.",
				notice.Count,
			)
		},
	})
}
