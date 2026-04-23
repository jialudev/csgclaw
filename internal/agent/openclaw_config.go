package agent

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"csgclaw/internal/config"
)

//go:embed defaults/openclaw-gateway.json
var defaultOpenClawGatewayConfig []byte

const (
	hostOpenClawDir     = ".openclaw"
	hostOpenClawConfig  = "openclaw.json"
	hostOpenClawLogs    = "logs"
	boxOpenClawUserHome = "/home/node"
	boxOpenClawDir      = "/home/node/.openclaw"
	boxOpenClawWorkspaceDir = boxOpenClawDir + "/workspace"
	boxOpenClawProjectsDir  = boxOpenClawDir + "/workspace/projects"
	openClawGatewayLog      = boxOpenClawDir + "/gateway.log"
)

func agentOpenClawRoot(agentName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve host home dir: %w", err)
	}
	return filepath.Join(homeDir, config.AppDirName, managerAgentsDirName, agentName, hostOpenClawDir), nil
}

func ensureAgentOpenClawConfig(agentName, botID string, server config.ServerConfig, model config.ModelConfig, chCfg config.ChannelsConfig) (string, error) {
	hostRoot, err := agentOpenClawRoot(agentName)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(hostRoot, hostOpenClawLogs), 0o755); err != nil {
		return "", fmt.Errorf("create openclaw logs dir: %w", err)
	}
	data, err := renderAgentOpenClawConfig(botID, server, model, chCfg)
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(hostRoot, hostOpenClawConfig)
	if err := os.WriteFile(configPath, append(data, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("write openclaw config: %w", err)
	}
	return hostRoot, nil
}

func renderAgentOpenClawConfig(botID string, server config.ServerConfig, model config.ModelConfig, chCfg config.ChannelsConfig) ([]byte, error) {
	var cfg map[string]any
	if err := json.Unmarshal(defaultOpenClawGatewayConfig, &cfg); err != nil {
		return nil, fmt.Errorf("decode embedded openclaw config: %w", err)
	}
	if err := updateOpenClawModelProvider(cfg, botID, server, model); err != nil {
		return nil, err
	}
	if err := updateOpenClawCsgclawChannel(cfg, botID, server); err != nil {
		return nil, err
	}
	if err := updateOpenClawGatewayAuth(cfg, server); err != nil {
		return nil, err
	}
	updateOpenClawFeishuChannel(cfg, botID, chCfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode openclaw config: %w", err)
	}
	return data, nil
}

func updateOpenClawModelProvider(cfg map[string]any, botID string, server config.ServerConfig, modelCfg config.ModelConfig) error {
	modelCfg = modelCfg.Resolved()
	modelsRoot, ok := cfg["models"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded openclaw config is missing models")
	}
	providers, ok := modelsRoot["providers"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded openclaw config is missing models.providers")
	}
	llm, ok := providers["csgclaw-llm"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded openclaw config is missing models.providers.csgclaw-llm")
	}
	managerBaseURL := resolveManagerBaseURL(server)
	if base := strings.TrimSpace(modelCfg.BaseURL); base != "" {
		llm["baseUrl"] = strings.TrimRight(base, "/")
	} else {
		llm["baseUrl"] = llmBridgeBaseURL(managerBaseURL, botID)
	}
	apiKey := strings.TrimSpace(modelCfg.APIKey)
	if apiKey == "" {
		apiKey = server.AccessToken
	}
	if apiKey != "" {
		llm["apiKey"] = apiKey
	}
	modelID := strings.TrimSpace(modelCfg.ModelID)
	if modelID == "" {
		return fmt.Errorf("openclaw config: model id is required")
	}
	modelList, ok := llm["models"].([]any)
	if !ok || len(modelList) == 0 {
		return fmt.Errorf("embedded openclaw config is missing models.providers.csgclaw-llm.models[0]")
	}
	entry, ok := modelList[0].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded openclaw config has invalid csgclaw-llm.models[0]")
	}
	entry["id"] = modelID
	entry["name"] = modelID
	agents, ok := cfg["agents"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded openclaw config is missing agents")
	}
	defaults, ok := agents["defaults"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded openclaw config is missing agents.defaults")
	}
	modelBlock, ok := defaults["model"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded openclaw config is missing agents.defaults.model")
	}
	modelBlock["primary"] = "csgclaw-llm/" + modelID
	return nil
}

func updateOpenClawCsgclawChannel(cfg map[string]any, botID string, server config.ServerConfig) error {
	channels, ok := cfg["channels"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded openclaw config is missing channels")
	}
	ch, ok := channels["csgclaw"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded openclaw config is missing channels.csgclaw")
	}
	if baseURL := resolveManagerBaseURL(server); baseURL != "" {
		ch["baseUrl"] = baseURL
	}
	if server.AccessToken != "" {
		ch["accessToken"] = server.AccessToken
	}
	ch["botId"] = botID
	ch["enabled"] = true
	return nil
}

func updateOpenClawGatewayAuth(cfg map[string]any, server config.ServerConfig) error {
	gw, ok := cfg["gateway"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded openclaw config is missing gateway")
	}
	auth, ok := gw["auth"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded openclaw config is missing gateway.auth")
	}
	if server.AccessToken != "" {
		auth["token"] = server.AccessToken
	}
	return nil
}

func updateOpenClawFeishuChannel(cfg map[string]any, botID string, chCfg config.ChannelsConfig) {
	botID = strings.TrimSpace(botID)
	if botID == "" {
		return
	}
	feishu, ok := chCfg.Feishu[botID]
	if !ok {
		return
	}
	channelsRoot, ok := cfg["channels"].(map[string]any)
	if !ok {
		return
	}
	channelsRoot["feishu"] = map[string]any{
		"enabled":         true,
		"connectionMode":  "websocket",
		"domain":          "feishu",
		"appId":           feishu.AppID,
		"appSecret":       feishu.AppSecret,
		"dmPolicy":        "open",
		"allowFrom":       []any{"*"},
		"groupPolicy":     "open",
		"requireMention":  true,
		"typingIndicator": true,
	}
	plugins, ok := cfg["plugins"].(map[string]any)
	if !ok {
		return
	}
	entries, ok := plugins["entries"].(map[string]any)
	if !ok {
		entries = map[string]any{}
		plugins["entries"] = entries
	}
	entries["feishu"] = map[string]any{"enabled": true}
}
