package cli

import (
	"context"
	"flag"
	"fmt"

	"csgclaw/internal/bot"
)

func (a *App) runBot(ctx context.Context, args []string, globals GlobalOptions) error {
	if len(args) == 0 {
		a.usageCommandGroup("bot", "Manage bots.", "csgclaw bot <subcommand> [flags]", []string{
			"list               List bots",
			"create             Create a bot",
		})
		return flag.ErrHelp
	}
	if isHelpArg(args[0]) {
		a.usageCommandGroup("bot", "Manage bots.", "csgclaw bot <subcommand> [flags]", []string{
			"list               List bots",
			"create             Create a bot",
		})
		return flag.ErrHelp
	}

	switch args[0] {
	case "list":
		return a.runBotList(ctx, args[1:], globals)
	case "create":
		return a.runBotCreate(ctx, args[1:], globals)
	default:
		a.usageCommandGroup("bot", "Manage bots.", "csgclaw bot <subcommand> [flags]", []string{
			"list               List bots",
			"create             Create a bot",
		})
		return fmt.Errorf("unknown bot subcommand %q", args[0])
	}
}

func (a *App) runBotList(ctx context.Context, args []string, globals GlobalOptions) error {
	fs := a.newCommandFlagSet("bot list", "csgclaw bot list [flags]", "List bots.")
	channelName := fs.String("channel", "csgclaw", "channel name: csgclaw or feishu")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("bot list does not accept positional arguments")
	}

	client := NewAPIClient(globals.Endpoint, globals.Token, a.httpClient)
	bots, err := client.ListBots(ctx, *channelName)
	if err != nil {
		return err
	}
	return a.renderBots(globals.Output, bots)
}

func (a *App) runBotCreate(ctx context.Context, args []string, globals GlobalOptions) error {
	fs := a.newCommandFlagSet("bot create", "csgclaw bot create [flags]", "Create a bot.")
	id := fs.String("id", "", "bot id")
	name := fs.String("name", "", "bot name")
	description := fs.String("description", "", "bot description")
	role := fs.String("role", "", "bot role: manager or worker")
	channelName := fs.String("channel", "csgclaw", "channel name: csgclaw or feishu")
	modelID := fs.String("model-id", "", "agent model identifier")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("bot create does not accept positional arguments")
	}
	if *name == "" {
		return fmt.Errorf("bot create requires --name")
	}
	if *role == "" {
		return fmt.Errorf("bot create requires --role")
	}

	client := NewAPIClient(globals.Endpoint, globals.Token, a.httpClient)
	created, err := client.CreateBot(ctx, bot.CreateRequest{
		ID:          *id,
		Name:        *name,
		Description: *description,
		Role:        *role,
		Channel:     *channelName,
		ModelID:     *modelID,
	})
	if err != nil {
		return err
	}
	return a.renderBots(globals.Output, []bot.Bot{created})
}

func (a *App) renderBots(output string, bots []bot.Bot) error {
	switch output {
	case "", "table":
		return renderBotsTable(a.stdout, bots)
	case "json":
		return writeJSON(a.stdout, bots)
	default:
		return fmt.Errorf("unsupported output format %q", output)
	}
}
