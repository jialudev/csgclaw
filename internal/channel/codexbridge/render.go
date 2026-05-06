package codexbridge

import (
	"fmt"
	"strings"

	runtimecodex "csgclaw/internal/runtime/codex"
)

type turnRenderer struct {
	text        strings.Builder
	toolTitles  map[string]string
	toolStates  map[string]string
	promptError string
}

func newTurnRenderer() *turnRenderer {
	return &turnRenderer{
		toolTitles: make(map[string]string),
		toolStates: make(map[string]string),
	}
}

func (r *turnRenderer) Apply(event runtimecodex.SessionEvent) []string {
	if r == nil {
		return nil
	}

	switch event.Kind {
	case runtimecodex.SessionEventTextDelta:
		if event.Text != "" {
			_, _ = r.text.WriteString(event.Text)
		}
	case runtimecodex.SessionEventToolCallStart:
		title := displayToolTitle(event)
		r.toolTitles[event.ToolCallID] = title
		r.toolStates[event.ToolCallID] = normalizedToolStatus(event.ToolStatus)
		return []string{fmt.Sprintf("Running tool: %s", title)}
	case runtimecodex.SessionEventToolCallUpdate:
		title := displayToolTitle(event)
		if title != "" && event.ToolCallID != "" {
			r.toolTitles[event.ToolCallID] = title
		}
		status := normalizedToolStatus(event.ToolStatus)
		if event.ToolCallID != "" && r.toolStates[event.ToolCallID] == status {
			return nil
		}
		if event.ToolCallID != "" {
			r.toolStates[event.ToolCallID] = status
		}
		switch status {
		case "completed":
			return []string{fmt.Sprintf("Tool completed: %s", title)}
		case "failed":
			return []string{fmt.Sprintf("Tool failed: %s", title)}
		}
	case runtimecodex.SessionEventPromptFailed:
		r.promptError = strings.TrimSpace(event.Error)
	}
	return nil
}

func (r *turnRenderer) FinalMessages() []string {
	if r == nil {
		return nil
	}
	var messages []string
	if text := strings.TrimSpace(r.text.String()); text != "" {
		messages = append(messages, text)
	}
	if r.promptError != "" {
		messages = append(messages, fmt.Sprintf("Codex runtime error: %s", r.promptError))
	}
	return messages
}

func displayToolTitle(event runtimecodex.SessionEvent) string {
	title := strings.TrimSpace(event.ToolTitle)
	if title == "" {
		title = "tool"
	}
	return title
}

func normalizedToolStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}
