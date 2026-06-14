package codex

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"csgclaw/internal/modelprovider"
	agentruntime "csgclaw/internal/runtime"
)

var checkResponsesAPIForProvider = modelprovider.CheckResponsesAPI

type responsesProbeCache struct {
	mu      sync.Mutex
	results map[string]struct{}
}

var codexResponsesProbeCache = responsesProbeCache{results: make(map[string]struct{})}

func TestOnlySetResponsesAPIProbe(probe func(context.Context, string, string, string, map[string]string) error) func() {
	previous := checkResponsesAPIForProvider
	checkResponsesAPIForProvider = probe
	codexResponsesProbeCache.clear()
	return func() {
		checkResponsesAPIForProvider = previous
		codexResponsesProbeCache.clear()
	}
}

func (r *Runtime) ValidateConfig(ctx context.Context, current agentruntime.RuntimeConfigSnapshot) error {
	target, ok, err := responsesProbeTarget(current.Profile)
	if err != nil {
		return fmt.Errorf("validate Codex provider Responses API: %w", err)
	}
	if !ok {
		return nil
	}
	if err := codexResponsesProbeCache.validate(ctx, target); err != nil {
		return fmt.Errorf("validate Codex provider Responses API at %s for model %s: %w", target.baseURL, target.modelID, err)
	}
	return nil
}

func (r *Runtime) RestartRequired(change agentruntime.RuntimeConfigChange) (bool, error) {
	return codexWorkspaceOptionChanged(change.Previous.Options, change.Current.Options), nil
}

func (r *Runtime) ReconcileConfig(ctx context.Context, h agentruntime.Handle, change agentruntime.RuntimeConfigChange) error {
	return r.SyncWorkspaceAgentsFile(ctx, h, change.Previous.Options)
}

type responsesProbeTargetConfig struct {
	provider string
	baseURL  string
	apiKey   string
	modelID  string
	headers  map[string]string
}

func responsesProbeTarget(profile agentruntime.RuntimeProfileConfig) (responsesProbeTargetConfig, bool, error) {
	provider := strings.ToLower(strings.TrimSpace(profile.Provider))
	switch provider {
	case "codex", "claude_code", "claude-code":
		return responsesProbeTargetConfig{}, false, nil
	}
	baseURL := strings.TrimRight(strings.TrimSpace(profile.BaseURL), "/")
	if baseURL == "" {
		return responsesProbeTargetConfig{}, false, fmt.Errorf("provider base_url is required")
	}
	return responsesProbeTargetConfig{
		provider: provider,
		baseURL:  baseURL,
		apiKey:   strings.TrimSpace(profile.APIKey),
		modelID:  strings.TrimSpace(profile.ModelID),
		headers:  normalizeHeaders(profile.Headers),
	}, true, nil
}

func (c *responsesProbeCache) validate(ctx context.Context, target responsesProbeTargetConfig) error {
	key := responsesProbeCacheKey(target)
	c.mu.Lock()
	if c.results == nil {
		c.results = make(map[string]struct{})
	}
	if _, ok := c.results[key]; ok {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	err := checkResponsesAPIForProvider(ctx, target.baseURL, target.apiKey, target.modelID, target.headers)
	if errors.Is(err, modelprovider.ErrResponsesAPIUnsupported) {
		err = nil
	}

	if err == nil {
		c.mu.Lock()
		c.results[key] = struct{}{}
		c.mu.Unlock()
	}
	return err
}

func (c *responsesProbeCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results = make(map[string]struct{})
}

func responsesProbeCacheKey(target responsesProbeTargetConfig) string {
	return strings.TrimSpace(target.provider) + "\x00" + strings.TrimRight(strings.TrimSpace(target.baseURL), "/") + "\x00" + strings.TrimSpace(target.modelID)
}

func normalizeHeaders(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func codexWorkspaceOptionChanged(previous, current map[string]any) bool {
	previousOpts, err := DecodeRuntimeOptions(previous)
	if err != nil {
		return true
	}
	currentOpts, err := DecodeRuntimeOptions(current)
	if err != nil {
		return true
	}
	return previousOpts.LocalWorkspaceDir != currentOpts.LocalWorkspaceDir
}
