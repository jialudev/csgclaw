package agentengine

import (
	"context"
	"fmt"
)

type protectedAgentClient struct {
	AgentInterface
	engine *Engine
}

// ProtectAgent prevents explicit deletion while preserving normal editing and
// Runtime recreation. The caller persists the binding before protecting it.
func (e *Engine) ProtectAgent(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.protected == nil {
		e.protected = map[string]bool{}
	}
	e.protected[id] = true
}
func (a protectedAgentClient) Delete(ctx context.Context, id string) error {
	current, err := a.AgentInterface.Get(ctx, id, AgentGetOptions{})
	if err != nil {
		return err
	}
	a.engine.mu.Lock()
	protected := a.engine.protected[current.ID]
	a.engine.mu.Unlock()
	if protected {
		return &TurnError{Code: ErrorInvalidRequest, Message: fmt.Sprintf("default assistant %q cannot be deleted", current.ID)}
	}
	return a.AgentInterface.Delete(ctx, id)
}
