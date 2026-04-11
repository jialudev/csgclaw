package cli

import (
	"context"
	"flag"
	"fmt"

	"csgclaw/internal/channel"
	"csgclaw/internal/im"
)

func (a *App) runUser(ctx context.Context, args []string, globals GlobalOptions) error {
	if len(args) == 0 {
		a.usageCommandGroup("user", "Manage IM users.", "csgclaw user <subcommand> [flags]", []string{
			"list               List users",
			"create             Create a user",
			"kick <id>          Remove a user",
		})
		return flag.ErrHelp
	}
	if isHelpArg(args[0]) {
		a.usageCommandGroup("user", "Manage IM users.", "csgclaw user <subcommand> [flags]", []string{
			"list               List users",
			"create             Create a user",
			"kick <id>          Remove a user",
		})
		return flag.ErrHelp
	}

	switch args[0] {
	case "list":
		return a.runUserList(ctx, args[1:], globals)
	case "create":
		return a.runUserCreate(ctx, args[1:], globals)
	case "kick":
		return a.runUserKick(ctx, args[1:], globals)
	default:
		a.usageCommandGroup("user", "Manage IM users.", "csgclaw user <subcommand> [flags]", []string{
			"list               List users",
			"create             Create a user",
			"kick <id>          Remove a user",
		})
		return fmt.Errorf("unknown user subcommand %q", args[0])
	}
}

func (a *App) runUserList(ctx context.Context, args []string, globals GlobalOptions) error {
	fs := a.newCommandFlagSet("user list", "csgclaw user list [flags]", "List users.")
	channelName := fs.String("channel", "csgclaw", "channel name: csgclaw or feishu")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("user list does not accept positional arguments")
	}

	client := NewAPIClient(globals.Endpoint, globals.Token, a.httpClient)
	users, err := client.ListUsersByChannel(ctx, *channelName)
	if err != nil {
		return err
	}
	return a.renderUsers(globals.Output, users)
}

func (a *App) runUserCreate(ctx context.Context, args []string, globals GlobalOptions) error {
	fs := a.newCommandFlagSet("user create", "csgclaw user create [flags]", "Create a user.")
	channelName := fs.String("channel", "csgclaw", "channel name: csgclaw or feishu")
	id := fs.String("id", "", "user id")
	name := fs.String("name", "", "user name")
	handle := fs.String("handle", "", "user handle")
	role := fs.String("role", "", "user role")
	avatar := fs.String("avatar", "", "user avatar initials")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("user create does not accept positional arguments")
	}
	if *channelName != "feishu" {
		return fmt.Errorf("user create currently supports --channel feishu")
	}

	client := NewAPIClient(globals.Endpoint, globals.Token, a.httpClient)
	user, err := client.CreateFeishuUser(ctx, channel.FeishuCreateUserRequest{
		ID:     *id,
		Name:   *name,
		Handle: *handle,
		Role:   *role,
		Avatar: *avatar,
	})
	if err != nil {
		return err
	}
	return a.renderUsers(globals.Output, []im.User{user})
}

func (a *App) runUserKick(ctx context.Context, args []string, globals GlobalOptions) error {
	fs := a.newCommandFlagSet("user kick", "csgclaw user kick <id> [flags]", "Remove a user.")
	if err := fs.Parse(args); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) != 1 {
		return fmt.Errorf("user kick requires exactly one id")
	}

	client := NewAPIClient(globals.Endpoint, globals.Token, a.httpClient)
	return client.KickUser(ctx, rest[0])
}

func (a *App) renderUsers(output string, users []im.User) error {
	switch output {
	case "", "table":
		return renderUsersTable(a.stdout, users)
	case "json":
		return writeJSON(a.stdout, users)
	default:
		return fmt.Errorf("unsupported output format %q", output)
	}
}
