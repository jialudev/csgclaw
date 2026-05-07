package agent

import (
	"context"
	"strings"

	agentruntime "csgclaw/internal/runtime"
)

type LifecycleObserver interface {
	EnsureAgent(context.Context, Agent) error
	StopAgent(string)
}

func (s *Service) lifecycleObserver() LifecycleObserver {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lifecycle
}

func (s *Service) syncLifecycleForAgent(ctx context.Context, a Agent) error {
	observer := s.lifecycleObserver()
	if observer == nil {
		return nil
	}
	if shouldEnsureLifecycle(a) {
		return observer.EnsureAgent(ctx, a)
	}
	observer.StopAgent(a.ID)
	return nil
}

func (s *Service) stopLifecycleAgent(agentID string) {
	observer := s.lifecycleObserver()
	if observer == nil {
		return
	}
	observer.StopAgent(strings.TrimSpace(agentID))
}

func shouldEnsureLifecycle(a Agent) bool {
	return isAgentProfileComplete(a) &&
		strings.EqualFold(strings.TrimSpace(a.Status), string(agentruntime.StateRunning))
}
