package runtimewiring

import (
	"fmt"
	"log/slog"
	"strings"

	"csgclaw/internal/agent"
	agentruntime "csgclaw/internal/runtime"
	runtimeopenclaw "csgclaw/internal/runtime/openclaw"
)

func WithOpenClawRuntime() agent.ServiceOption {
	return func(s *agent.Service) error {
		if s == nil {
			return fmt.Errorf("agent service is required")
		}
		host := s.OpenClawHostRuntimeHost()
		rt := runtimeopenclaw.New(runtimeopenclaw.Dependencies{
			ResolveAgent: func(h agentruntime.Handle) (runtimeopenclaw.AgentRef, error) {
				got, err := host.ResolveAgent(h)
				if err != nil {
					return runtimeopenclaw.AgentRef{}, err
				}
				profile, err := host.ResolveRuntimeProfile(h)
				if err != nil {
					return runtimeopenclaw.AgentRef{}, err
				}
				return runtimeopenclaw.AgentRef{
					ID:        got.ID,
					Name:      got.Name,
					RuntimeID: strings.TrimSpace(got.RuntimeID),
					HandleID:  strings.TrimSpace(got.BoxID),
					Profile:   profile,
				}, nil
			},
			AgentHome:   host.AgentHome,
			InstallRoot: host.InstallRoot,
			PrepareAgent: func(agentName, botID string, profile agentruntime.Profile, opts runtimeopenclaw.PrepareOptions) (runtimeopenclaw.PreparedAgent, error) {
				return host.PrepareAgent(agentName, botID, profile, opts)
			},
			Progress: func(progress runtimeopenclaw.Progress) {
				slog.Info("openclaw runtime progress",
					"agent", progress.AgentName,
					"stage", progress.Stage,
					"percent", progress.Percent,
					"message", progress.Message,
				)
			},
		})
		return agent.WithRuntime(rt)(s)
	}
}
