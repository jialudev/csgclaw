package api

import (
	"context"
	"sync"
	"sync/atomic"

	"csgclaw/internal/activity"
	"csgclaw/internal/agentengine"
)

type uiFixtureRuntime struct {
	fakeCompatRuntime
	approved    chan struct{}
	once        sync.Once
	resolutions atomic.Int32
}

func (r *uiFixtureRuntime) Conversation(string) agentengine.RuntimeConversation { return r }
func (r *uiFixtureRuntime) Reset(context.Context, agentengine.ConversationKey) *agentengine.TurnError {
	return nil
}
func (r *uiFixtureRuntime) Resolve(_ context.Context, _ agentengine.InteractionRequest, resolution agentengine.InteractionResolution) *agentengine.TurnError {
	if resolution.OptionID != "allow" {
		return &agentengine.TurnError{Code: agentengine.ErrorInvalidRequest, Message: "Unexpected option"}
	}
	r.resolutions.Add(1)
	r.once.Do(func() { close(r.approved) })
	return nil
}
func (r *uiFixtureRuntime) Run(ctx context.Context, request agentengine.TurnRequest, sink agentengine.EventSink) agentengine.TurnResult {
	if request.ID == "approval" {
		snapshot := activity.ActivitySnapshot{ID: "e2e-permission", Title: "允许执行验证命令？", Status: activity.ActionStatusPending, Options: []activity.ActionOptionSnapshot{{ID: "allow", Label: "允许一次", Kind: "allow_once"}, {ID: "deny", Label: "拒绝", Kind: "reject"}}}
		if err := sink.Emit(ctx, agentengine.TurnEvent{Kind: agentengine.TurnEventInteractionRequest, Interaction: &agentengine.InteractionRequest{ID: snapshot.ID, Kind: agentengine.InteractionPermission, Payload: snapshot}}); err != nil {
			return agentengine.TurnResult{Status: agentengine.TurnFailed}
		}
		select {
		case <-r.approved:
		case <-ctx.Done():
			return agentengine.TurnResult{Status: agentengine.TurnCanceled, Dispatched: true}
		}
		snapshot.Status = activity.ActionStatusAllowed
		_ = sink.Emit(ctx, agentengine.TurnEvent{Kind: agentengine.TurnEventActivityUpdate, Activity: &agentengine.ActivityUpdate{ID: snapshot.ID, Kind: string(activity.RuntimeEventActionDecision), Payload: snapshot}})
		return agentengine.TurnResult{Status: agentengine.TurnSucceeded, Dispatched: true, Output: "权限确认已通过 Agent Engine。"}
	}
	args := activity.RequestUserInputArgs{Questions: []activity.RequestUserInputQuestion{{ID: "verification", Header: "验收", Question: "确认最终架构的交互回复可用？", Options: []activity.RequestUserInputOption{{Label: "验证通过", Description: "通过 Engine 回复并保存记录。"}}}}}
	if err := sink.Emit(ctx, agentengine.TurnEvent{Kind: agentengine.TurnEventOutputItem, Output: &agentengine.OutputItem{Kind: agentengine.OutputItemRequestUserInput, Payload: args}}); err != nil {
		return agentengine.TurnResult{Status: agentengine.TurnFailed}
	}
	return agentengine.TurnResult{Status: agentengine.TurnSucceeded, Dispatched: true, Output: "Agent Engine 端到端验证。"}
}
