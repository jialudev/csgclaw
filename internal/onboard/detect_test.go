package onboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"csgclaw/internal/agent"
	"csgclaw/internal/bot"
	"csgclaw/internal/config"
	"csgclaw/internal/im"
)

func TestDetectStateFreshHomeReportsIncompleteBootstrap(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	result, err := DetectState(DetectStateOptions{})
	if err != nil {
		t.Fatalf("DetectState() error = %v", err)
	}

	if result.ConfigExists {
		t.Fatal("ConfigExists = true, want false")
	}
	if result.ConfigComplete {
		t.Fatal("ConfigComplete = true, want false")
	}
	if result.IMBootstrapComplete {
		t.Fatal("IMBootstrapComplete = true, want false")
	}
	if result.ManagerAgentComplete {
		t.Fatal("ManagerAgentComplete = true, want false")
	}
	if result.ManagerBotComplete {
		t.Fatal("ManagerBotComplete = true, want false")
	}
	if result.Complete() {
		t.Fatal("Complete() = true, want false")
	}
}

func TestDetectStateCompleteBootstrapReportsComplete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	configPath, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if err := defaultConfig().Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	imStatePath, err := config.DefaultIMStatePath()
	if err != nil {
		t.Fatalf("DefaultIMStatePath() error = %v", err)
	}
	if err := im.SaveBootstrap(imStatePath, im.Bootstrap{
		CurrentUserID: "u-admin",
		Users: []im.User{
			{ID: "u-admin", Name: "admin", Handle: "admin", Role: "admin"},
			{ID: "u-manager", Name: "manager", Handle: "manager", Role: "manager"},
		},
		Rooms: []im.Room{
			{
				ID:       "room-bootstrap",
				Title:    "admin & manager",
				IsDirect: true,
				Members:  []string{"u-admin", "u-manager"},
			},
		},
	}); err != nil {
		t.Fatalf("SaveBootstrap() error = %v", err)
	}

	if err := writeManagerAgentState(t); err != nil {
		t.Fatalf("writeManagerAgentState() error = %v", err)
	}
	if err := writeManagerBotState(t, bot.Bot{
		ID:        agent.ManagerUserID,
		Name:      "manager",
		Role:      string(bot.RoleManager),
		Channel:   string(bot.ChannelCSGClaw),
		AgentID:   agent.ManagerUserID,
		UserID:    agent.ManagerUserID,
		Available: true,
		CreatedAt: time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("writeManagerBotState() error = %v", err)
	}

	result, err := DetectState(DetectStateOptions{})
	if err != nil {
		t.Fatalf("DetectState() error = %v", err)
	}

	if !result.ConfigExists || !result.ConfigComplete || !result.IMBootstrapComplete || !result.ManagerAgentComplete || !result.ManagerBotComplete {
		t.Fatalf("DetectState() completeness = %+v, want all true", result)
	}
	if !result.Complete() {
		t.Fatal("Complete() = false, want true")
	}
}

func TestDetectStateFlagsMissingManagerBotWhenOtherBootstrapStateExists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	configPath, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if err := defaultConfig().Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	imStatePath, err := config.DefaultIMStatePath()
	if err != nil {
		t.Fatalf("DefaultIMStatePath() error = %v", err)
	}
	if err := im.EnsureBootstrapState(imStatePath); err != nil {
		t.Fatalf("EnsureBootstrapState() error = %v", err)
	}
	if err := writeManagerAgentState(t); err != nil {
		t.Fatalf("writeManagerAgentState() error = %v", err)
	}

	result, err := DetectState(DetectStateOptions{})
	if err != nil {
		t.Fatalf("DetectState() error = %v", err)
	}

	if !result.ConfigExists || !result.ConfigComplete || !result.IMBootstrapComplete || !result.ManagerAgentComplete {
		t.Fatalf("DetectState() = %+v, want config/im/agent complete", result)
	}
	if result.ManagerBotComplete {
		t.Fatal("ManagerBotComplete = true, want false")
	}
	if result.Complete() {
		t.Fatal("Complete() = true, want false")
	}
}

func writeManagerAgentState(t *testing.T) error {
	t.Helper()

	agentsPath, err := config.DefaultAgentsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(agentsPath), 0o755); err != nil {
		return err
	}

	state := map[string]any{
		"agents": []map[string]any{
			{
				"id":         agent.ManagerUserID,
				"name":       agent.ManagerName,
				"role":       agent.RoleManager,
				"status":     "running",
				"created_at": time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
			},
		},
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(agentsPath, append(data, '\n'), 0o600)
}

func writeManagerBotState(t *testing.T, manager bot.Bot) error {
	t.Helper()

	imStatePath, err := config.DefaultIMStatePath()
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(imStatePath), "bots.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(map[string]any{
		"bots": []bot.Bot{manager},
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
