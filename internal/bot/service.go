package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"csgclaw/internal/agent"
	"csgclaw/internal/channel"
	"csgclaw/internal/im"
)

type Service struct {
	store  *Store
	agents *agent.Service
	im     *im.Service
	feishu *channel.FeishuService
}

func NewService(store *Store) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("bot store is required")
	}
	return &Service{store: store}, nil
}

func NewServiceWithDependencies(store *Store, agentSvc *agent.Service, imSvc *im.Service, feishuSvc ...*channel.FeishuService) (*Service, error) {
	s, err := NewService(store)
	if err != nil {
		return nil, err
	}
	s.SetDependencies(agentSvc, imSvc, feishuSvc...)
	return s, nil
}

func (s *Service) SetDependencies(agentSvc *agent.Service, imSvc *im.Service, feishuSvc ...*channel.FeishuService) {
	if s == nil {
		return
	}
	s.agents = agentSvc
	s.im = imSvc
	if len(feishuSvc) > 0 {
		s.feishu = feishuSvc[0]
	}
}

func (s *Service) List(channel string) ([]Bot, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("bot store is required")
	}
	if channel == "" {
		return s.store.List(), nil
	}

	normalized, err := NormalizeChannel(channel)
	if err != nil {
		return nil, err
	}

	all := s.store.List()
	filtered := make([]Bot, 0, len(all))
	for _, b := range all {
		if b.Channel == string(normalized) {
			filtered = append(filtered, b)
		}
	}
	return filtered, nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (Bot, error) {
	if s == nil || s.store == nil {
		return Bot{}, fmt.Errorf("bot store is required")
	}

	normalized, err := NormalizeCreateRequest(req)
	if err != nil {
		return Bot{}, err
	}
	if normalized.Role != string(RoleWorker) {
		return Bot{}, fmt.Errorf("bot create supports role %q only", RoleWorker)
	}
	if s.agents == nil {
		return Bot{}, fmt.Errorf("agent service is required")
	}
	if normalized.ID != "" {
		if _, ok := s.store.Get(normalized.ID); ok {
			return Bot{}, fmt.Errorf("bot id %q already exists", normalized.ID)
		}
	}

	created, err := s.agents.CreateWorker(ctx, agent.CreateRequest{
		ID:          normalized.ID,
		Name:        normalized.Name,
		Description: normalized.Description,
		Role:        agent.RoleWorker,
		ModelID:     normalized.ModelID,
	})
	if err != nil {
		return Bot{}, err
	}

	userID, err := s.ensureChannelUser(normalized.Channel, created)
	if err != nil {
		// TODO: compensate by deleting the agent/box created above once agent deletion
		// semantics are safe to call from bot creation.
		return Bot{}, err
	}

	createdAt := created.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	b := Bot{
		ID:        created.ID,
		Name:      created.Name,
		Role:      string(RoleWorker),
		Channel:   normalized.Channel,
		AgentID:   created.ID,
		UserID:    userID,
		CreatedAt: createdAt,
	}
	if _, ok := s.store.Get(b.ID); ok {
		return Bot{}, fmt.Errorf("bot id %q already exists", b.ID)
	}
	if err := s.store.Save(b); err != nil {
		return Bot{}, err
	}
	return b, nil
}

func (s *Service) ensureChannelUser(channelName string, created agent.Agent) (string, error) {
	switch channelName {
	case string(ChannelCSGClaw):
		if s.im == nil {
			return "", fmt.Errorf("im service is required")
		}
		user, _, err := s.im.EnsureAgentUser(im.EnsureAgentUserRequest{
			ID:     created.ID,
			Name:   created.Name,
			Handle: deriveAgentHandle(created),
			Role:   displayRole(created.Role),
		})
		if err != nil {
			return "", fmt.Errorf("agent created but failed to ensure im user: %w", err)
		}
		return user.ID, nil
	case string(ChannelFeishu):
		if s.feishu == nil {
			return "", fmt.Errorf("feishu service is required")
		}
		// Keep Feishu mock users keyed by the agent ID for now. Future real
		// open_id/app_id mappings belong in bot/channel state, not agent state.
		user, err := s.feishu.CreateUser(channel.FeishuCreateUserRequest{
			ID:     created.ID,
			Name:   created.Name,
			Handle: deriveAgentHandle(created),
			Role:   displayRole(created.Role),
		})
		if err != nil {
			return "", fmt.Errorf("agent created but failed to create feishu user: %w", err)
		}
		return user.ID, nil
	default:
		return "", fmt.Errorf("channel must be one of %q or %q", ChannelCSGClaw, ChannelFeishu)
	}
}

func deriveAgentHandle(a agent.Agent) string {
	if handle, ok := sanitizeHandle(strings.ToLower(strings.ReplaceAll(strings.TrimSpace(a.Name), " ", "-"))); ok {
		return handle
	}
	if handle, ok := sanitizeHandle(strings.ToLower(strings.TrimPrefix(strings.TrimSpace(a.ID), "u-"))); ok {
		return handle
	}
	switch strings.ToLower(strings.TrimSpace(a.Role)) {
	case agent.RoleManager:
		return "manager"
	case agent.RoleWorker:
		return "worker"
	default:
		return "agent"
	}
}

func displayRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case agent.RoleManager:
		return "manager"
	case agent.RoleWorker:
		return "Worker"
	default:
		return "Agent"
	}
}

func sanitizeHandle(input string) (string, bool) {
	var b strings.Builder
	hasAlphaNum := false
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			hasAlphaNum = true
			b.WriteRune(r)
			continue
		}
		if r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 || !hasAlphaNum {
		return "", false
	}
	return b.String(), true
}
