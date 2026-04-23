package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"csgclaw/internal/config"
)

func TestRenderAgentOpenClawConfigUsesDirectMinimaxWhenBaseURLSet(t *testing.T) {
	data, err := renderAgentOpenClawConfig("u-manager", config.ServerConfig{
		ListenAddr:       "127.0.0.1:18080",
		AdvertiseBaseURL: "http://127.0.0.1:18080",
		AccessToken:      "gateway-shared-token",
	}, config.ModelConfig{
		BaseURL: "https://api.minimaxi.com/v1",
		APIKey:  "sk-minimax-test",
		ModelID: "MiniMax-M2.7",
	}, config.ChannelsConfig{})
	if err != nil {
		t.Fatalf("renderAgentOpenClawConfig() error = %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	models := cfg["models"].(map[string]any)
	providers := models["providers"].(map[string]any)
	llm := providers["csgclaw-llm"].(map[string]any)
	if got, want := llm["baseUrl"], "https://api.minimaxi.com/v1"; got != want {
		t.Fatalf("llm baseUrl = %v, want %v", got, want)
	}
	if got, want := llm["apiKey"], "sk-minimax-test"; got != want {
		t.Fatalf("llm apiKey = %v, want %v", got, want)
	}
	agents := cfg["agents"].(map[string]any)
	defaults := agents["defaults"].(map[string]any)
	model := defaults["model"].(map[string]any)
	if got, want := model["primary"], "csgclaw-llm/MiniMax-M2.7"; got != want {
		t.Fatalf("primary model = %v, want %v", got, want)
	}
}

func TestRenderAgentOpenClawConfigUsesBridgeWhenBaseURLEmpty(t *testing.T) {
	data, err := renderAgentOpenClawConfig("u-manager", config.ServerConfig{
		ListenAddr:       "127.0.0.1:18080",
		AdvertiseBaseURL: "http://127.0.0.1:18080",
		AccessToken:      "shared-token",
	}, config.ModelConfig{
		ModelID: "MiniMax-M2.7",
	}, config.ChannelsConfig{})
	if err != nil {
		t.Fatalf("renderAgentOpenClawConfig() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `http://127.0.0.1:18080/api/bots/u-manager/llm`) {
		t.Fatalf("expected CSGClaw LLM bridge URL in config:\n%s", text)
	}
}
