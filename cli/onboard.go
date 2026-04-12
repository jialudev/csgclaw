package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"csgclaw/internal/agent"
	"csgclaw/internal/bot"
	"csgclaw/internal/config"
	"csgclaw/internal/im"
)

var (
	botCreateManager       = createOnboardManagerBot
	imEnsureBootstrapState = im.EnsureBootstrapState
)

func (a *App) runOnboard(args []string, globals GlobalOptions) error {
	fs := a.newCommandFlagSet("onboard", "csgclaw onboard [flags]", "Initialize local config and bootstrap state.")
	baseURL := fs.String("base-url", "", "LLM provider base URL")
	apiKey := fs.String("api-key", "", "LLM provider API key")
	modelID := fs.String("model-id", "", "LLM model identifier")
	managerImage := fs.String("manager-image", "", "bootstrap manager image")
	forceRecreateManager := fs.Bool("force-recreate-manager", false, "remove and recreate the bootstrap manager box")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path, err := configPath(globals.Config)
	if err != nil {
		return err
	}

	cfg, hasExistingConfig, err := loadOnboardConfig(path)
	if err != nil {
		return err
	}
	if !hasExistingConfig {
		cfg = config.Config{
			Server: config.ServerConfig{
				ListenAddr:  config.DefaultListenAddr(),
				AccessToken: config.DefaultAccessToken,
			},
			Bootstrap: config.BootstrapConfig{
				ManagerImage: config.DefaultManagerImage,
			},
		}
	}
	if *baseURL != "" {
		cfg.Model.BaseURL = *baseURL
	}
	if *apiKey != "" {
		cfg.Model.APIKey = *apiKey
	}
	if *modelID != "" {
		cfg.Model.ModelID = *modelID
	}
	if *managerImage != "" {
		cfg.Bootstrap.ManagerImage = *managerImage
	}
	if err := validateModelConfig(cfg); err != nil {
		return err
	}

	if err := cfg.Save(path); err != nil {
		return err
	}

	agentsPath, err := config.DefaultAgentsPath()
	if err != nil {
		return err
	}
	imStatePath, err := config.DefaultIMStatePath()
	if err != nil {
		return err
	}
	if err := imEnsureBootstrapState(imStatePath); err != nil {
		return err
	}
	if _, err := botCreateManager(context.Background(), agentsPath, imStatePath, cfg, *forceRecreateManager); err != nil {
		return err
	}

	fmt.Fprintf(a.stdout, "initialized config at %s\n", path)
	fmt.Fprintf(a.stdout, "ensured bootstrap agent %q with image %q\n", agent.ManagerName, cfg.Bootstrap.ManagerImage)
	fmt.Fprintf(a.stdout, "ensured IM members %q and %q\n", "admin", "manager")
	fmt.Fprintln(a.stdout, "cleared IM invite draft data")
	if *forceRecreateManager {
		fmt.Fprintln(a.stdout, "manager box was force-recreated")
	}
	return nil
}

func createOnboardManagerBot(ctx context.Context, agentsPath, imStatePath string, cfg config.Config, forceRecreateManager bool) (bot.Bot, error) {
	agentSvc, err := agent.NewService(cfg.Model, cfg.Server, cfg.Bootstrap.ManagerImage, agentsPath)
	if err != nil {
		return bot.Bot{}, err
	}
	defer func() {
		_ = agentSvc.Close()
	}()

	imSvc, err := im.NewServiceFromPath(imStatePath)
	if err != nil {
		return bot.Bot{}, err
	}
	store, err := bot.NewStore(filepath.Join(filepath.Dir(imStatePath), "bots.json"))
	if err != nil {
		return bot.Bot{}, err
	}
	botSvc, err := bot.NewServiceWithDependencies(store, agentSvc, imSvc)
	if err != nil {
		return bot.Bot{}, err
	}
	return botSvc.CreateManager(ctx, bot.CreateRequest{
		Name:    agent.ManagerName,
		Role:    string(bot.RoleManager),
		Channel: string(bot.ChannelCSGClaw),
	}, forceRecreateManager)
}

func loadOnboardConfig(path string) (config.Config, bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return config.Config{}, false, nil
		}
		return config.Config{}, false, fmt.Errorf("stat config: %w", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, false, err
	}
	return cfg, true, nil
}
