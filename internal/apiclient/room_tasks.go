package apiclient

import (
	"context"
	"net/http"
	"net/url"

	"csgclaw/internal/apitypes"
)

func (c *Client) ReportRoomTask(ctx context.Context, roomID, taskID, outcome, summary string) (apitypes.TeamTask, error) {
	var result apitypes.TeamTask
	req := struct {
		Outcome string `json:"outcome"`
		Summary string `json:"summary"`
	}{outcome, summary}
	err := c.DoJSON(ctx, http.MethodPost, roomTasksPath(roomID)+"/"+url.PathEscape(taskID)+"/report", req, &result)
	return result, err
}
func (c *Client) RetryRoomDelivery(ctx context.Context, roomID string) ([]apitypes.TeamTask, error) {
	var result []apitypes.TeamTask
	err := c.DoJSON(ctx, http.MethodPost, roomTasksPath(roomID)+"/retry-delivery", struct{}{}, &result)
	return result, err
}

func roomTasksPath(roomID string) string { return "/api/v1/rooms/" + url.PathEscape(roomID) + "/tasks" }
func (c *Client) ListRoomTasks(ctx context.Context, roomID string) ([]apitypes.TeamTask, error) {
	var result []apitypes.TeamTask
	err := c.GetJSON(ctx, roomTasksPath(roomID), &result)
	return result, err
}
func (c *Client) CreateRoomTask(ctx context.Context, roomID string, req apitypes.CreateRoomTaskRequest) (apitypes.TeamTask, error) {
	var result apitypes.TeamTask
	err := c.DoJSON(ctx, http.MethodPost, roomTasksPath(roomID), req, &result)
	return result, err
}
func (c *Client) PlanRoomTask(ctx context.Context, roomID, taskID string) (apitypes.PlanTeamTaskResponse, error) {
	var result apitypes.PlanTeamTaskResponse
	err := c.DoJSON(ctx, http.MethodPost, roomTasksPath(roomID)+"/"+url.PathEscape(taskID)+"/plan", apitypes.PlanTeamTaskRequest{AutoStart: true}, &result)
	return result, err
}
func (c *Client) ClaimRoomTask(ctx context.Context, roomID, taskID, participantID string) (apitypes.TeamTask, error) {
	var result apitypes.TeamTask
	err := c.DoJSON(ctx, http.MethodPost, roomTasksPath(roomID)+"/"+url.PathEscape(taskID)+"/claim", apitypes.ClaimTeamTaskRequest{ParticipantID: participantID}, &result)
	return result, err
}
func (c *Client) UpdateRoomTask(ctx context.Context, roomID, taskID, actorID string, req apitypes.PatchTeamTaskRequest) (apitypes.TeamTask, error) {
	var result apitypes.TeamTask
	body := struct {
		apitypes.PatchTeamTaskRequest
		ActorID string `json:"actor_id"`
	}{req, actorID}
	err := c.DoJSON(ctx, http.MethodPatch, roomTasksPath(roomID)+"/"+url.PathEscape(taskID), body, &result)
	return result, err
}
