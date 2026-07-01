package task

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"csgclaw/cli/command"
	"csgclaw/internal/apitypes"
	"csgclaw/internal/taskcore"
)

type cmd struct{}

func NewCmd() command.Command {
	return cmd{}
}

func (cmd) Name() string {
	return "task"
}

func (cmd) Summary() string {
	return "Manage agent tasks."
}

func (c cmd) Run(ctx context.Context, run *command.Context, args []string, globals command.GlobalOptions) error {
	if len(args) == 0 {
		c.usage(run)
		return flag.ErrHelp
	}
	if command.IsHelpArg(args[0]) {
		c.usage(run)
		return flag.ErrHelp
	}

	switch args[0] {
	case "list":
		return c.runList(ctx, run, args[1:], globals)
	case "create":
		return c.runCreate(ctx, run, args[1:], globals)
	case "claim":
		return c.runClaim(ctx, run, args[1:], globals)
	case "update":
		return c.runUpdate(ctx, run, args[1:], globals)
	default:
		c.usage(run)
		return fmt.Errorf("unknown task subcommand %q", args[0])
	}
}

func (c cmd) usage(run *command.Context) {
	run.UsageCommandGroup(c, run.Program+" task <subcommand> [flags]", []string{
		"list                        List global tasks",
		"create                      Create an agent task",
		"claim                       Claim an agent task",
		"update                      Update an agent task status",
	})
}

func (c cmd) runList(ctx context.Context, run *command.Context, args []string, globals command.GlobalOptions) error {
	fs := run.NewFlagSet("task list", run.Program+" task list", "List global tasks.")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("task list does not accept positional arguments")
	}
	items, err := run.APIClient(globals).ListGlobalTasks(ctx)
	if err != nil {
		return err
	}
	return command.RenderGlobalTasks(globals.Output, run.Stdout, items)
}

func (c cmd) runCreate(ctx context.Context, run *command.Context, args []string, globals command.GlobalOptions) error {
	fs := run.NewFlagSet("task create", run.Program+" task create --agent-id <agent> --title <title>", "Create an agent task.")
	agentID := fs.String("agent-id", "", "agent id")
	title := fs.String("title", "", "task title")
	body := fs.String("body", "", "task body")
	createdBy := fs.String("created-by", "manager", "creator participant id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("task create does not accept positional arguments")
	}
	if strings.TrimSpace(*agentID) == "" || strings.TrimSpace(*title) == "" {
		return fmt.Errorf("agent_id and title are required")
	}
	item, err := run.APIClient(globals).CreateAgentTask(ctx, apitypes.CreateAgentTaskRequest{
		AgentID:   strings.TrimSpace(*agentID),
		Title:     strings.TrimSpace(*title),
		Body:      strings.TrimSpace(*body),
		CreatedBy: strings.TrimSpace(*createdBy),
	})
	if err != nil {
		return err
	}
	return command.RenderTasks(globals.Output, run.Stdout, []apitypes.TeamTask{item})
}

func (c cmd) runClaim(ctx context.Context, run *command.Context, args []string, globals command.GlobalOptions) error {
	fs := run.NewFlagSet("task claim", run.Program+" task claim --task <id> --participant-id <participant>", "Claim an agent task.")
	taskID := fs.String("task", "", "task id")
	participantID := fs.String("participant-id", "", "worker participant id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("task claim does not accept positional arguments")
	}
	if strings.TrimSpace(*taskID) == "" || strings.TrimSpace(*participantID) == "" {
		return fmt.Errorf("task and participant_id are required")
	}
	item, err := run.APIClient(globals).ClaimAgentTask(ctx, *taskID, *participantID)
	if err != nil {
		return err
	}
	return command.RenderTasks(globals.Output, run.Stdout, []apitypes.TeamTask{item})
}

func (c cmd) runUpdate(ctx context.Context, run *command.Context, args []string, globals command.GlobalOptions) error {
	fs := run.NewFlagSet("task update", run.Program+" task update --task <id> --actor-id <participant> --status <status>", "Update an agent task status.")
	taskID := fs.String("task", "", "task id")
	actorID := fs.String("actor-id", "", "actor participant id")
	status := fs.String("status", "", "new status: blocked, completed, or failed")
	result := fs.String("result", "", "task result text")
	errorText := fs.String("error", "", "task error text")
	reason := fs.String("reason", "", "blocking reason")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("task update does not accept positional arguments")
	}
	if strings.TrimSpace(*taskID) == "" || strings.TrimSpace(*actorID) == "" || strings.TrimSpace(*status) == "" {
		return fmt.Errorf("task, actor_id, and status are required")
	}
	if !isSupportedStatus(*status) {
		return fmt.Errorf("status must be one of: blocked, completed, failed")
	}
	item, err := run.APIClient(globals).UpdateAgentTask(ctx, *taskID, apitypes.PatchAgentTaskRequest{
		ActorID: strings.TrimSpace(*actorID),
		Status:  strings.TrimSpace(*status),
		Result:  strings.TrimSpace(*result),
		Error:   strings.TrimSpace(*errorText),
		Reason:  strings.TrimSpace(*reason),
	})
	if err != nil {
		return err
	}
	return command.RenderTasks(globals.Output, run.Stdout, []apitypes.TeamTask{item})
}

func isSupportedStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case taskcore.StatusBlocked, taskcore.StatusCompleted, taskcore.StatusFailed:
		return true
	default:
		return false
	}
}
