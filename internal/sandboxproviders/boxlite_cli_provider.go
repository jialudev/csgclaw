package sandboxproviders

import (
	"csgclaw/internal/agent"
	"csgclaw/internal/config"
	"csgclaw/internal/sandbox/boxlitecli"
)

// Non-SDK sandbox providers register unconditionally so they remain available
// in every csgclaw build.
func init() {
	Register(config.BoxLiteCLIProvider, func(cfg config.SandboxConfig) (agent.ServiceOption, error) {
		opts := []boxlitecli.ProviderOption{boxlitecli.WithPath(boxlitecli.ResolvePath(""))}
		for _, registry := range cfg.EffectiveDebianRegistries() {
			opts = append(opts, boxlitecli.WithRegistry(registry))
		}
		return agent.WithSandboxProvider(boxlitecli.NewProvider(opts...)), nil
	})
}
