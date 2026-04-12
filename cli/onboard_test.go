package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"csgclaw/internal/bot"
	"csgclaw/internal/config"
)

func TestRunOnboardRequiresLLMFlagsForFirstTimeSetup(t *testing.T) {
	origCreateManager := botCreateManager
	origEnsureIMBootstrapState := imEnsureBootstrapState
	t.Cleanup(func() {
		botCreateManager = origCreateManager
		imEnsureBootstrapState = origEnsureIMBootstrapState
	})

	botCreateManager = func(context.Context, string, string, config.Config, bool) (bot.Bot, error) {
		t.Fatal("bot manager create should not run when config is incomplete")
		return bot.Bot{}, nil
	}
	imEnsureBootstrapState = func(string) error {
		t.Fatal("IM bootstrap should not run when config is incomplete")
		return nil
	}

	app := &App{
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}

	err := app.runOnboard(nil, GlobalOptions{Config: filepath.Join(t.TempDir(), "config.toml")})
	if err == nil {
		t.Fatal("runOnboard() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "--base-url") || !strings.Contains(err.Error(), "--api-key") || !strings.Contains(err.Error(), "--model-id") {
		t.Fatalf("runOnboard() error = %q, want all required LLM flags", err)
	}
}

func TestRunOnboardReusesExistingLLMConfig(t *testing.T) {
	origCreateManager := botCreateManager
	origEnsureIMBootstrapState := imEnsureBootstrapState
	t.Cleanup(func() {
		botCreateManager = origCreateManager
		imEnsureBootstrapState = origEnsureIMBootstrapState
	})

	callCount := 0
	botCreateManager = func(_ context.Context, _, _ string, cfg config.Config, _ bool) (bot.Bot, error) {
		callCount++
		if cfg.Model.BaseURL != "http://llm.test" || cfg.Model.APIKey != "secret" || cfg.Model.ModelID != "gpt-test" {
			t.Fatalf("model config = %#v, want preserved values", cfg.Model)
		}
		return bot.Bot{}, nil
	}
	imEnsureBootstrapState = func(string) error { return nil }

	configPath := filepath.Join(t.TempDir(), "config.toml")
	app := &App{
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}

	if err := app.runOnboard([]string{"--base-url", "http://llm.test", "--api-key", "secret", "--model-id", "gpt-test"}, GlobalOptions{Config: configPath}); err != nil {
		t.Fatalf("initial runOnboard() error = %v", err)
	}

	if err := app.runOnboard(nil, GlobalOptions{Config: configPath}); err != nil {
		t.Fatalf("second runOnboard() error = %v", err)
	}

	if callCount != 2 {
		t.Fatalf("agent bootstrap call count = %d, want 2", callCount)
	}
}
