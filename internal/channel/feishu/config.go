package feishu

import (
	"errors"
	"strings"
	"sync"

	"csgclaw/internal/config"
)

type Config struct {
	mu         sync.RWMutex
	configPath string
}

type Update struct {
	BotID       string
	AppID       string
	AppSecret   string
	AdminOpenID string
}

type Entry struct {
	BotID       string
	Configured  bool
	AppID       string
	HasSecret   bool
	AdminOpenID string
}

type AppConfig struct {
	AppID       string
	AppSecret   string
	AdminOpenID string
}

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func IsValidationError(err error) bool {
	var validationErr ValidationError
	return errors.As(err, &validationErr)
}

func NewConfig(configPath string) *Config {
	return &Config{
		configPath: strings.TrimSpace(configPath),
	}
}

func (c *Config) SetPath(path string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configPath = strings.TrimSpace(path)
}

func (c *Config) Get(botID string) (Entry, error) {
	botID, err := normalizeConfigBotID(botID)
	if err != nil {
		return Entry{}, err
	}
	provider, err := c.provider()
	if err != nil {
		return Entry{}, err
	}
	app, ok := provider.BotConfig(botID)
	return MaskAppConfig(botID, app, ok), nil
}

func (c *Config) Update(req Update) (Entry, error) {
	provider, err := c.provider()
	if err != nil {
		return Entry{}, err
	}
	view, _, err := provider.Update(req)
	return view, err
}

func (c *Config) Load() (config.Config, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loadConfigWithChannelFiles()
}

func normalizeConfigUpdate(req Update) (string, string, string, string, error) {
	botID, err := normalizeConfigBotID(req.BotID)
	if err != nil {
		return "", "", "", "", err
	}
	appID := strings.TrimSpace(req.AppID)
	if appID == "" {
		return "", "", "", "", ValidationError{Message: "app_id is required"}
	}
	appSecret := strings.TrimSpace(req.AppSecret)
	if appSecret == "" {
		return "", "", "", "", ValidationError{Message: "app_secret is required"}
	}
	return botID, appID, appSecret, strings.TrimSpace(req.AdminOpenID), nil
}

func normalizeConfigBotID(botID string) (string, error) {
	botID = strings.TrimSpace(botID)
	if err := ValidateBotID(botID); err != nil {
		return "", ValidationError{Message: err.Error()}
	}
	return botID, nil
}

func (c *Config) loadConfigWithChannelFiles() (config.Config, error) {
	if strings.TrimSpace(c.configPath) == "" {
		return config.LoadDefault()
	}
	return config.Load(c.configPath)
}

func MaskAppConfig(botID string, app AppConfig, configured bool) Entry {
	view := Entry{
		BotID:       botID,
		Configured:  configured,
		AdminOpenID: strings.TrimSpace(app.AdminOpenID),
	}
	if configured {
		view.AppID = strings.TrimSpace(app.AppID)
		view.HasSecret = strings.TrimSpace(app.AppSecret) != ""
	}
	return view
}

func AppsFromSnapshot(snapshot Snapshot) map[string]AppConfig {
	return cloneAppConfigs(snapshot.Bots)
}

func (c *Config) provider() (*ConfigProvider, error) {
	c.mu.RLock()
	path := c.configPath
	c.mu.RUnlock()
	return NewProvider(NewFileStore(path))
}
