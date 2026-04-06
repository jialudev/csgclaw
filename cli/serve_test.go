package cli

import (
	"bytes"
	"context"
	"testing"

	"csgclaw/internal/agent"
	"csgclaw/internal/config"
	"csgclaw/internal/im"
	"csgclaw/internal/server"
)

func TestServeForegroundPassesContextToServer(t *testing.T) {
	origRunServer := runServer
	origNewAgentService := newAgentServiceFn
	origNewIMService := newIMServiceFn
	t.Cleanup(func() {
		runServer = origRunServer
		newAgentServiceFn = origNewAgentService
		newIMServiceFn = origNewIMService
	})

	ctx := context.WithValue(context.Background(), struct{}{}, "serve-context")

	newAgentServiceFn = func(config.Config) (*agent.Service, error) {
		return nil, nil
	}
	newIMServiceFn = func() (*im.Service, error) {
		return nil, nil
	}

	called := false
	runServer = func(opts server.Options) error {
		called = true
		if opts.Context != ctx {
			t.Fatalf("Context = %v, want %v", opts.Context, ctx)
		}
		return nil
	}

	app := &App{
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
	cfg := config.Config{
		Server: config.ServerConfig{
			APIBaseURL: "http://example.test",
		},
	}

	if err := app.serveForeground(ctx, cfg); err != nil {
		t.Fatalf("serveForeground() error = %v", err)
	}
	if !called {
		t.Fatal("runServer was not called")
	}
}
