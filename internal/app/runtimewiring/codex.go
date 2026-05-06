package runtimewiring

import (
	"fmt"
	"strings"

	"csgclaw/internal/agent"
	"csgclaw/internal/channel/codexbridge"
	"csgclaw/internal/codexacp"
	agentruntime "csgclaw/internal/runtime"
	runtimecodex "csgclaw/internal/runtime/codex"
)

func WithCodexRuntime() agent.ServiceOption {
	return func(s *agent.Service) error {
		if s == nil {
			return fmt.Errorf("agent service is required")
		}

		host := s.PicoClawRuntimeHost()
		events := codexbridge.NewEventSink()
		rt := runtimecodex.New(runtimecodex.Dependencies{
			BinaryProvider: codexacp.Installer{
				Locator: codexacp.Locator{},
			},
			ResolveAgent: func(h agentruntime.Handle) (runtimecodex.AgentRef, error) {
				got, err := host.ResolveAgent(h)
				if err != nil {
					return runtimecodex.AgentRef{}, err
				}
				return runtimecodex.AgentRef{
					ID:        got.ID,
					Name:      got.Name,
					RuntimeID: strings.TrimSpace(got.RuntimeID),
					HandleID:  strings.TrimSpace(got.BoxID),
					Profile: agentruntime.Profile{
						ModelID: codexModelID(got),
						Env:     cloneEnvMap(got.AgentProfile.Env),
					},
				}, nil
			},
			AgentHome: host.AgentHome,
			EventSink: events,
		})
		return agent.WithRuntime(rt)(s)
	}
}

func codexModelID(got agent.Agent) string {
	modelID := strings.TrimSpace(got.AgentProfile.ModelID)
	if modelID != "" {
		return modelID
	}
	return strings.TrimSpace(got.ModelID)
}

func cloneEnvMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		dst[key] = value
	}
	if len(dst) == 0 {
		return nil
	}
	return dst
}
