package cli

import (
	"context"
	"flag"
	"fmt"

	"csgclaw/internal/im"
)

func (a *App) runMember(ctx context.Context, args []string, globals GlobalOptions) error {
	if len(args) == 0 {
		a.usageMember()
		return flag.ErrHelp
	}
	if isHelpArg(args[0]) {
		a.usageMember()
		return flag.ErrHelp
	}

	switch args[0] {
	case "list":
		return a.runMemberList(ctx, args[1:], globals)
	case "create":
		return a.runMemberCreate(ctx, args[1:], globals)
	default:
		a.usageMember()
		return fmt.Errorf("unknown member subcommand %q", args[0])
	}
}

func (a *App) usageMember() {
	a.usageCommandGroup("member", "Manage IM room members.", "csgclaw member <subcommand> [flags]", []string{
		"list               List room members",
		"create             Add a member to a room",
	})
}

func (a *App) runMemberList(ctx context.Context, args []string, globals GlobalOptions) error {
	fs := a.newCommandFlagSet("member list", "csgclaw member list [flags]", "List room members.")
	channelName := fs.String("channel", "feishu", "channel name: feishu")
	roomID := fs.String("room-id", "", "target room id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("member list does not accept positional arguments")
	}

	client := NewAPIClient(globals.Endpoint, globals.Token, a.httpClient)
	users, err := client.ListRoomMembersByChannel(ctx, *channelName, *roomID)
	if err != nil {
		return err
	}
	return a.renderUsers(globals.Output, users)
}

func (a *App) runMemberCreate(ctx context.Context, args []string, globals GlobalOptions) error {
	fs := a.newCommandFlagSet("member create", "csgclaw member create [flags]", "Add a member to a room.")
	channelName := fs.String("channel", "feishu", "channel name: feishu")
	roomID := fs.String("room-id", "", "target room id")
	userID := fs.String("user-id", "", "user id to add")
	inviterID := fs.String("inviter-id", "", "inviter user id")
	locale := fs.String("locale", "", "room locale")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("member create does not accept positional arguments")
	}
	if *userID == "" {
		return fmt.Errorf("user_id is required")
	}

	client := NewAPIClient(globals.Endpoint, globals.Token, a.httpClient)
	room, err := client.AddRoomMemberByChannel(ctx, *channelName, im.AddRoomMembersRequest{
		RoomID:    *roomID,
		InviterID: *inviterID,
		UserIDs:   []string{*userID},
		Locale:    *locale,
	})
	if err != nil {
		return err
	}
	return a.renderRooms(globals.Output, []im.Room{room})
}
