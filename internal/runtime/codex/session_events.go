package codex

import (
	"fmt"
	"strings"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

func eventFromSessionUpdate(runtimeID string, note acp.SessionNotification) SessionEvent {
	base := SessionEvent{
		RuntimeID:  strings.TrimSpace(runtimeID),
		SessionID:  strings.TrimSpace(string(note.SessionId)),
		ReceivedAt: time.Now().UTC(),
		Payload:    note.Update,
	}

	switch update := note.Update; {
	case update.AgentMessageChunk != nil:
		base.Kind = SessionEventTextDelta
		base.MessageID = stringValue(update.AgentMessageChunk.MessageId)
		base.Text = textFromContentBlock(update.AgentMessageChunk.Content)
	case update.AgentThoughtChunk != nil:
		base.Kind = SessionEventThoughtDelta
		base.MessageID = stringValue(update.AgentThoughtChunk.MessageId)
		base.Text = textFromContentBlock(update.AgentThoughtChunk.Content)
	case update.UserMessageChunk != nil:
		base.Kind = SessionEventUserMessageDelta
		base.MessageID = stringValue(update.UserMessageChunk.MessageId)
		base.Text = textFromContentBlock(update.UserMessageChunk.Content)
	case update.ToolCall != nil:
		base.Kind = SessionEventToolCallStart
		base.ToolCallID = strings.TrimSpace(string(update.ToolCall.ToolCallId))
		base.ToolTitle = strings.TrimSpace(update.ToolCall.Title)
		base.ToolStatus = strings.TrimSpace(string(update.ToolCall.Status))
		base.Payload = update.ToolCall
	case update.ToolCallUpdate != nil:
		base.Kind = SessionEventToolCallUpdate
		base.ToolCallID = strings.TrimSpace(string(update.ToolCallUpdate.ToolCallId))
		base.ToolTitle = stringValue(update.ToolCallUpdate.Title)
		if update.ToolCallUpdate.Status != nil {
			base.ToolStatus = strings.TrimSpace(string(*update.ToolCallUpdate.Status))
		}
		base.Payload = update.ToolCallUpdate
	case update.Plan != nil:
		base.Kind = SessionEventPlanUpdate
		base.Payload = update.Plan
	default:
		base.Kind = SessionEventKind("session_update")
	}

	return base
}

func permissionRequestEvent(runtimeID string, params acp.RequestPermissionRequest) SessionEvent {
	return SessionEvent{
		RuntimeID:  strings.TrimSpace(runtimeID),
		SessionID:  strings.TrimSpace(string(params.SessionId)),
		Kind:       SessionEventPermissionRequest,
		ReceivedAt: time.Now().UTC(),
		ToolCallID: strings.TrimSpace(string(params.ToolCall.ToolCallId)),
		ToolTitle:  stringValue(params.ToolCall.Title),
		Payload:    params,
	}
}

func permissionDecisionEvent(runtimeID string, params acp.RequestPermissionRequest, option *acp.PermissionOption) SessionEvent {
	event := SessionEvent{
		RuntimeID:  strings.TrimSpace(runtimeID),
		SessionID:  strings.TrimSpace(string(params.SessionId)),
		Kind:       SessionEventPermissionDecision,
		ReceivedAt: time.Now().UTC(),
		ToolCallID: strings.TrimSpace(string(params.ToolCall.ToolCallId)),
		ToolTitle:  stringValue(params.ToolCall.Title),
		Payload:    params,
	}
	if option != nil {
		event.PermissionOptionID = strings.TrimSpace(string(option.OptionId))
		event.PermissionOptionKind = strings.TrimSpace(string(option.Kind))
	}
	return event
}

func promptCompletedEvent(runtimeID string, sessionID string, resp acp.PromptResponse) SessionEvent {
	return SessionEvent{
		RuntimeID:  strings.TrimSpace(runtimeID),
		SessionID:  strings.TrimSpace(sessionID),
		Kind:       SessionEventPromptCompleted,
		ReceivedAt: time.Now().UTC(),
		MessageID:  stringValue(resp.UserMessageId),
		StopReason: strings.TrimSpace(string(resp.StopReason)),
		Payload:    resp,
	}
}

func promptFailedEvent(runtimeID string, sessionID string, err error) SessionEvent {
	return SessionEvent{
		RuntimeID:  strings.TrimSpace(runtimeID),
		SessionID:  strings.TrimSpace(sessionID),
		Kind:       SessionEventPromptFailed,
		ReceivedAt: time.Now().UTC(),
		Error:      errorString(err),
		Payload:    err,
	}
}

func textFromContentBlock(block acp.ContentBlock) string {
	if block.Text != nil {
		return block.Text.Text
	}
	if block.ResourceLink != nil {
		return strings.TrimSpace(block.ResourceLink.Name) + " " + strings.TrimSpace(block.ResourceLink.Uri)
	}
	if block.Resource != nil {
		if block.Resource.Resource.TextResourceContents != nil {
			return block.Resource.Resource.TextResourceContents.Text
		}
	}
	return ""
}

func choosePermissionOption(options []acp.PermissionOption) *acp.PermissionOption {
	for i := range options {
		if options[i].Kind == acp.PermissionOptionKindAllowOnce {
			return &options[i]
		}
	}
	for i := range options {
		if options[i].Kind == acp.PermissionOptionKindAllowAlways {
			return &options[i]
		}
	}
	return nil
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", err))
}
