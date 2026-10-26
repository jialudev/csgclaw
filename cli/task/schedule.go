package task

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"csgclaw/cli/command"
)

func (c cmd) runSchedule(ctx context.Context, run *command.Context, args []string, globals command.GlobalOptions) error {
	if len(args) == 0 {
		return fmt.Errorf("use task schedule parse|create|list")
	}
	action := args[0]
	fs := run.NewFlagSet("task schedule "+action, run.Program+" task schedule "+action+" [flags]", "Preview a schedule, then save only after the user confirms.")
	text := fs.String("text", "", "task and time in natural language")
	zone := fs.String("timezone", "UTC", "IANA time zone, e.g. Asia/Shanghai")
	executor := fs.String("agent", "manager", "default executor when not named in the request")
	file := fs.String("file", "", "edited preview JSON file")
	confirmed := fs.Bool("confirmed", false, "the user has confirmed this preview in a later message")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	client := run.APIClient(globals)
	var result any
	switch action {
	case "parse":
		if strings.TrimSpace(*text) == "" {
			return fmt.Errorf("text is required")
		}
		err := client.DoJSON(ctx, http.MethodPost, "/api/v1/scheduled-tasks/parse", map[string]string{"text": *text, "time_zone": *zone, "default_agent_id": *executor}, &result)
		if err != nil {
			return err
		}
	case "create":
		if !*confirmed || *file == "" {
			return fmt.Errorf("show the preview first; after user confirmation use --file <edited-preview.json> --confirmed")
		}
		raw, err := os.ReadFile(*file)
		if err != nil {
			return err
		}
		var payload map[string]any
		if json.Unmarshal(raw, &payload) != nil || payload == nil {
			return fmt.Errorf("invalid preview JSON")
		}
		if value, _ := payload["clarification"].(string); value != "" {
			return fmt.Errorf("resolve the clarification before creating")
		}
		if id, _ := payload["client_request_id"].(string); strings.TrimSpace(id) == "" {
			return fmt.Errorf("client_request_id from the preview is required")
		}
		payload["confirmed"] = true
		if err := client.DoJSON(ctx, http.MethodPost, "/api/v1/scheduled-tasks", payload, &result); err != nil {
			return err
		}
	case "list":
		if err := client.GetJSON(ctx, "/api/v1/scheduled-tasks", &result); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown schedule action %q", action)
	}
	return command.WriteJSON(run.Stdout, result)
}
