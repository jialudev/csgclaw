package onboard

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"csgclaw/cli/command"
	"csgclaw/internal/bot"
	"csgclaw/internal/config"
)

func TestRunRequiresLLMFlagsForFirstTimeSetup(t *testing.T) {
	origCreateManager := CreateManagerBot
	origEnsureIMBootstrapState := EnsureIMBootstrapState
	t.Cleanup(func() {
		CreateManagerBot = origCreateManager
		EnsureIMBootstrapState = origEnsureIMBootstrapState
	})

	CreateManagerBot = func(context.Context, string, string, config.Config, bool) (bot.Bot, error) {
		t.Fatal("bot manager create should not run when config is incomplete")
		return bot.Bot{}, nil
	}
	EnsureIMBootstrapState = func(string) error {
		t.Fatal("IM bootstrap should not run when config is incomplete")
		return nil
	}

	run := testContext()
	err := NewCmd().Run(context.Background(), run, nil, command.GlobalOptions{Config: filepath.Join(t.TempDir(), "config.toml")})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "--base-url") || !strings.Contains(err.Error(), "--api-key") || !strings.Contains(err.Error(), "--model-id") {
		t.Fatalf("Run() error = %q, want all required LLM flags", err)
	}
}

func TestRunReusesExistingLLMConfig(t *testing.T) {
	origCreateManager := CreateManagerBot
	origEnsureIMBootstrapState := EnsureIMBootstrapState
	t.Cleanup(func() {
		CreateManagerBot = origCreateManager
		EnsureIMBootstrapState = origEnsureIMBootstrapState
	})

	callCount := 0
	CreateManagerBot = func(_ context.Context, _, _ string, cfg config.Config, _ bool) (bot.Bot, error) {
		callCount++
		if cfg.Model.BaseURL != "http://llm.test" || cfg.Model.APIKey != "secret" || cfg.Model.ModelID != "gpt-test" {
			t.Fatalf("model config = %#v, want preserved values", cfg.Model)
		}
		return bot.Bot{}, nil
	}
	EnsureIMBootstrapState = func(string) error { return nil }

	configPath := filepath.Join(t.TempDir(), "config.toml")
	run := testContext()
	cmd := NewCmd()

	args := []string{"--base-url", "http://llm.test", "--api-key", "secret", "--model-id", "gpt-test"}
	if err := cmd.Run(context.Background(), run, args, command.GlobalOptions{Config: configPath}); err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}

	if err := cmd.Run(context.Background(), run, nil, command.GlobalOptions{Config: configPath}); err != nil {
		t.Fatalf("second Run() error = %v", err)
	}

	if callCount != 2 {
		t.Fatalf("agent bootstrap call count = %d, want 2", callCount)
	}
}

func testContext() *command.Context {
	return &command.Context{
		Program: "csgclaw",
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
	}
}
