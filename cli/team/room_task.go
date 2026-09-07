package team

import (
	"context"
	"flag"
	"fmt"

	"csgclaw/cli/command"
	"csgclaw/internal/apitypes"
)

type roomTaskCmd struct{}

func NewRoomTaskCmd() command.Command { return roomTaskCmd{} }
func (roomTaskCmd) Name() string      { return "room-task" }
func (roomTaskCmd) Summary() string {
	return "Coordinate tracked work in an existing collaboration room."
}
func (c roomTaskCmd) Run(ctx context.Context, run *command.Context, args []string, globals command.GlobalOptions) error {
	if len(args) == 0 || command.IsHelpArg(args[0]) {
		run.UsageCommandGroup(c, run.Program+" room-task <submit|list|plan|claim|update|report|retry-delivery> --room <id> [flags]", []string{
			"submit --source-message <id> --actor-id <requester> --title <title> [--body <body>]  Create the room root task",
			"plan --task <id>  Plan and start the root task",
			"list  Inspect room tasks, progress and results",
			"claim --task <id> --actor-id <worker>  Claim a dispatched subtask",
			"update --task <id> --actor-id <worker> --status <completed|failed|blocked> [--result|--error|--reason <text>]",
			"report --task <id> --outcome <succeeded|issues|failed|stopped> --result <summary>  Publish the manager's assessment",
			"retry-delivery  Retry recorded room messages without re-running work",
		})
		return flag.ErrHelp
	}
	action := args[0]
	switch action {
	case "submit", "list", "plan", "claim", "update", "report", "retry-delivery":
	default:
		return fmt.Errorf("unknown room-task action %q", action)
	}
	fs := run.NewFlagSet("room-task "+action, run.Program+" room-task "+action+" --room <id> [flags]", c.Summary())
	room := fs.String("room", "", "room id")
	task := fs.String("task", "", "task id")
	actor := fs.String("actor-id", "", "requester or worker participant id")
	source := fs.String("source-message", "", "original user message id; reuse on retries")
	title := fs.String("title", "", "task title")
	body := fs.String("body", "", "task requirements and deliverables")
	status := fs.String("status", "", "completed, failed, or blocked")
	outcome := fs.String("outcome", "", "manager assessment: succeeded, issues, failed, stopped")
	result := fs.String("result", "", "deliverables, test outcomes and unresolved issues")
	failure := fs.String("error", "", "failure reason")
	reason := fs.String("reason", "", "blocking reason")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if len(fs.Args()) != 0 || *room == "" {
		return fmt.Errorf("room is required; positional arguments are not accepted")
	}
	client := run.APIClient(globals)
	var items []apitypes.TeamTask
	switch action {
	case "retry-delivery":
		got, err := client.RetryRoomDelivery(ctx, *room)
		if err != nil {
			return err
		}
		items = got
	case "report":
		if *task == "" || *outcome == "" || *result == "" {
			return fmt.Errorf("task, outcome and result are required")
		}
		got, err := client.ReportRoomTask(ctx, *room, *task, *outcome, *result)
		if err != nil {
			return err
		}
		items = []apitypes.TeamTask{got}
	case "list":
		got, err := client.ListRoomTasks(ctx, *room)
		if err != nil {
			return err
		}
		items = got
	case "submit":
		if *source == "" || *actor == "" || *title == "" {
			return fmt.Errorf("source-message, actor-id and title are required")
		}
		got, err := client.CreateRoomTask(ctx, *room, apitypes.CreateRoomTaskRequest{Title: *title, Body: *body, CreatedBy: *actor, SourceMessageID: *source})
		if err != nil {
			return err
		}
		items = []apitypes.TeamTask{got}
	case "plan":
		if *task == "" {
			return fmt.Errorf("task is required")
		}
		got, err := client.PlanRoomTask(ctx, *room, *task)
		if err != nil {
			return err
		}
		items = append([]apitypes.TeamTask{got.Task}, got.CreatedTasks...)
	case "claim":
		if *task == "" || *actor == "" {
			return fmt.Errorf("task and actor-id are required")
		}
		got, err := client.ClaimRoomTask(ctx, *room, *task, *actor)
		if err != nil {
			return err
		}
		items = []apitypes.TeamTask{got}
	case "update":
		if *task == "" || *actor == "" || *status == "" {
			return fmt.Errorf("task, actor-id and status are required")
		}
		got, err := client.UpdateRoomTask(ctx, *room, *task, *actor, apitypes.PatchTeamTaskRequest{Status: *status, Result: *result, Error: *failure, Reason: *reason})
		if err != nil {
			return err
		}
		items = []apitypes.TeamTask{got}
	}
	return command.RenderTeamTasks(globals.Output, run.Stdout, items)
}
