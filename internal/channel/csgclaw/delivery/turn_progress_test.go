package delivery

import (
	"context"
	"csgclaw/internal/activity"
	"csgclaw/internal/agentengine"
	"csgclaw/internal/apitypes"
	"csgclaw/internal/channel"
	"csgclaw/internal/im"
	"csgclaw/internal/modelusage"
	"csgclaw/internal/roomtask"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestLargeProgressPersistsWithoutBlockingOtherRooms(t *testing.T) {
	for _, kind := range []string{"single tool", "multiple tools", "reasoning"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			bootstrap := im.Bootstrap{
				CurrentUserID: "user",
				Users:         []im.User{{ID: "user", Name: "User"}, {ID: "agent", Name: "Agent"}},
				Rooms: []im.Room{
					{ID: "room", Title: "Room", Members: []string{"user", "agent"}},
					{ID: "other", Title: "Other room", Members: []string{"user", "agent"}},
				},
			}
			if err := im.SaveBootstrap(path, bootstrap); err != nil {
				t.Fatal(err)
			}
			service, err := im.NewServiceFromPath(path)
			if err != nil {
				t.Fatal(err)
			}
			store, err := NewIMTranscriptStore(service, fixedParticipantResolver{item: apitypes.Participant{ID: "participant", ChannelUserRef: "agent"}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			turn := channel.TurnContext{AgentID: "agent", RoomID: "room", ParticipantID: "participant", TurnID: "turn-csgclaw-large", ConversationKey: "room"}
			progress := activity.TurnProgress{ID: string(turn.TurnID), Status: "running", Items: []activity.ProgressItem{}}
			progress.Usage = &modelusage.Usage{Known: true, ModelID: "auto", Input: 1000, Output: 220, Total: 1220}
			if err := store.DeliverTurnProgress(context.Background(), turn, progress, "", true, nil); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "reasoning":
				progress.Items = append(progress.Items, activity.ProgressItem{ID: "thought", Kind: "reasoning", Text: strings.Repeat("R", 70*1024)})
			default:
				count, size := 1, 64*1024
				if kind == "multiple tools" {
					count, size = 8, 8*1024
				}
				for i := 0; i < count; i++ {
					tool := progressTool(agentengine.ToolActivity{Kind: "exec_command", Status: "completed", OutputSummary: strings.Repeat("X", size)}, nil)
					progress.Items = append(progress.Items, activity.ProgressItem{ID: fmt.Sprintf("tool-%d", i), Kind: "tool", Tool: tool})
				}
			}
			progress.Status = "succeeded"
			if err := store.DeliverTurnProgress(context.Background(), turn, progress, "Short answer", true, nil); err != nil {
				t.Fatalf("large progress delivery failed: %v", err)
			}
			if _, err := service.CreateMessage(im.CreateMessageRequest{RoomID: "other", SenderID: "user", Content: "你好"}); err != nil {
				t.Fatalf("unrelated room blocked: %v", err)
			}
			if _, err := service.CreateRoom(im.CreateRoomRequest{Type: apitypes.RoomTypeOnDemand, Title: "New on-demand room", CreatorID: "user", ManagerID: "agent"}); err != nil {
				t.Fatalf("room creation blocked: %v", err)
			}
			restarted, err := im.NewServiceFromPath(path)
			if err != nil {
				t.Fatal(err)
			}
			message, loaded := progressMessage(t, restarted)
			want, _ := json.Marshal(progress)
			got, _ := json.Marshal(loaded)
			if message.Content != "Short answer" || string(got) != string(want) {
				t.Fatal("restart lost answer or progress details")
			}
		})
	}
}

func progressFixture(t *testing.T) (*TranscriptRenderer, *im.Service, channel.TurnContext) {
	t.Helper()
	service := im.NewServiceFromBootstrap(im.Bootstrap{CurrentUserID: "user", Users: []im.User{{ID: "user", Name: "User"}, {ID: "agent", Name: "Agent"}}, Rooms: []im.Room{{ID: "room", Title: "Room", Members: []string{"user", "agent"}}}})
	store, err := NewIMTranscriptStore(service, fixedParticipantResolver{item: apitypes.Participant{ID: "participant", ChannelUserRef: "agent"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewTranscriptRenderer(store), service, channel.TurnContext{AgentID: "agent", RoomID: "room", ParticipantID: "participant", TurnID: "turn", ConversationKey: "room", Locale: "en"}
}

func TestProgressUsageReplacesSnapshotsAndRetainsPartialFailure(t *testing.T) {
	for _, status := range []agentengine.TurnStatus{agentengine.TurnSucceeded, agentengine.TurnCanceled} {
		t.Run(string(status), func(t *testing.T) {
			renderer, service, turn := progressFixture(t)
			ctx := context.Background()
			for _, input := range []int64{100, 300, 300} {
				u := modelusage.Usage{Known: true, ModelID: "selected-model", Input: input, Output: 20, Total: input + 20}
				// The HTTP client decodes arbitrary activity payloads into maps.
				encoded, _ := json.Marshal(u)
				var payload map[string]any
				_ = json.Unmarshal(encoded, &payload)
				if err := renderer.Emit(ctx, turn, agentengine.TurnEvent{Kind: agentengine.TurnEventActivityUpdate, Activity: &agentengine.ActivityUpdate{Kind: modelusage.Kind, Payload: payload}}); err != nil {
					t.Fatal(err)
				}
			}
			if err := renderer.Complete(ctx, turn, agentengine.TurnResult{Status: status, Output: "done"}); err != nil {
				t.Fatal(err)
			}
			_, p := progressMessage(t, service)
			if p.Usage == nil || p.Usage.Total != 320 || p.Usage.ModelID != "selected-model" || p.Usage.Partial != (status != agentengine.TurnSucceeded) {
				t.Fatalf("usage=%+v", p.Usage)
			}
		})
	}
}
func progressMessage(t *testing.T, service *im.Service) (im.Message, activity.TurnProgress) {
	t.Helper()
	room, ok := service.Room("room")
	if !ok || len(room.Messages) != 1 {
		t.Fatalf("room=%+v", room)
	}
	msg := room.Messages[0]
	meta := msg.Metadata["csgclaw"].(map[string]any)
	data, _ := json.Marshal(meta["turn_progress"])
	var p activity.TurnProgress
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	return msg, p
}
func TestProgressStreamsThroughIMAndCommitsOneFinal(t *testing.T) {
	renderer, service, turn := progressFixture(t)
	ctx := context.Background()
	for _, event := range []agentengine.TurnEvent{
		{Sequence: 1, Kind: agentengine.TurnEventTextDelta, ItemID: "intro", Phase: "commentary", Text: "Checking files."},
		{Sequence: 2, Kind: agentengine.TurnEventToolCallStart, Tool: &agentengine.ToolActivity{ID: "read", Kind: "exec_command", Status: "started", Payload: map[string]any{"command": "cat README.md", "commandActions": []any{map[string]any{"type": "read"}}}}},
		{Sequence: 3, Kind: agentengine.TurnEventToolCallUpdate, Tool: &agentengine.ToolActivity{ID: "read", Kind: "exec_command", Status: "completed", Payload: map[string]any{"output": "contents", "exitCode": float64(0)}}},
		{Sequence: 4, Kind: agentengine.TurnEventTextDelta, ItemID: "answer", Phase: "final_answer", Text: "The answer"},
	} {
		if err := renderer.Emit(ctx, turn, event); err != nil {
			t.Fatal(err)
		}
	}
	msg, p := progressMessage(t, service)
	if msg.Content != "The answer" || !im.IsAgentActivityMessage(msg) || p.Status != "running" || len(p.Items) != 2 || p.Items[1].Tool.Actions[0] != "read" {
		t.Fatalf("stream=%+v progress=%+v", msg, p)
	}
	// Queue a trailing chunk, then finish before the timer. Completion must win.
	if err := renderer.Emit(ctx, turn, agentengine.TurnEvent{Sequence: 5, Kind: agentengine.TurnEventTextDelta, ItemID: "answer", Phase: "final_answer", Text: " is ready."}); err != nil {
		t.Fatal(err)
	}
	if err := renderer.Complete(ctx, turn, agentengine.TurnResult{Status: agentengine.TurnSucceeded}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(70 * time.Millisecond)
	msg, p = progressMessage(t, service)
	if msg.Content != "The answer is ready." || im.IsAgentActivityMessage(msg) || p.Status != "succeeded" || p.EndedAt == "" {
		t.Fatalf("final=%+v progress=%+v", msg, p)
	}
	if err := renderer.Emit(ctx, turn, agentengine.TurnEvent{Sequence: 6, Kind: agentengine.TurnEventTextDelta, Text: "late"}); err != nil {
		t.Fatal(err)
	}
	again, _ := progressMessage(t, service)
	if again.Content != msg.Content {
		t.Fatal("late update overwrote final")
	}
}
func TestProgressUnphasedSegmentationAndCancellation(t *testing.T) {
	for _, status := range []agentengine.TurnStatus{agentengine.TurnSucceeded, agentengine.TurnCanceled, agentengine.TurnFailed} {
		t.Run(string(status), func(t *testing.T) {
			renderer, service, turn := progressFixture(t)
			ctx := context.Background()
			events := []agentengine.TurnEvent{{Sequence: 1, Kind: agentengine.TurnEventTextDelta, Text: "Working."}, {Sequence: 2, Kind: agentengine.TurnEventToolCallStart, Tool: &agentengine.ToolActivity{ID: "x", Kind: "custom_tool", Status: "started"}}, {Sequence: 3, Kind: agentengine.TurnEventTextDelta, Text: "Final result."}}
			for _, event := range events {
				if err := renderer.Emit(ctx, turn, event); err != nil {
					t.Fatal(err)
				}
			}
			result := agentengine.TurnResult{Status: status}
			if status == agentengine.TurnFailed {
				result.Error = &agentengine.TurnError{Message: "Test failure"}
			}
			if err := renderer.Complete(ctx, turn, result); err != nil {
				t.Fatal(err)
			}
			msg, p := progressMessage(t, service)
			if msg.Content != "Final result." || len(p.Items) != 2 || p.Items[0].Text != "Working." || p.Items[1].Tool.Actions[0] != "tool" || p.Status != string(status) {
				t.Fatalf("message=%+v p=%+v", msg, p)
			}
			if status != agentengine.TurnSucceeded && !im.IsAgentActivityMessage(msg) {
				t.Fatal("incomplete answer wakes agents")
			}
		})
	}
}
func TestCanceledProgressDoesNotPresentRuntimeFailure(t *testing.T) {
	renderer, service, turn := progressFixture(t)
	ctx := context.Background()
	if err := renderer.Emit(ctx, turn, agentengine.TurnEvent{Sequence: 1, Kind: agentengine.TurnEventToolCallStart, Tool: &agentengine.ToolActivity{ID: "cancel-tool", Kind: "custom_tool", Status: "started"}}); err != nil {
		t.Fatal(err)
	}
	if err := renderer.Complete(ctx, turn, agentengine.TurnResult{
		Status: agentengine.TurnCanceled,
		Error:  &agentengine.TurnError{Code: agentengine.ErrorRuntimeFailed, Message: context.Canceled.Error()},
	}); err != nil {
		t.Fatal(err)
	}
	msg, progress := progressMessage(t, service)
	metadata := msg.Metadata["csgclaw"].(map[string]any)
	if progress.Status != "canceled" || progress.Error != "" || progress.ErrorCode != "" || metadata["runtime_error"] != nil || metadata["error_detail"] != nil {
		t.Fatalf("cancellation presented as failure: progress=%+v metadata=%+v", progress, metadata)
	}
	if len(progress.Items) != 1 || progress.Items[0].Tool.Status != "interrupted" || !im.IsAgentActivityMessage(msg) {
		t.Fatalf("cancellation lost interrupted activity: message=%+v progress=%+v", msg, progress)
	}
}

func TestProgressCoalescesReasoningAndFlushesTail(t *testing.T) {
	renderer, service, turn := progressFixture(t)
	ctx := context.Background()
	for i := 1; i <= 10; i++ {
		if err := renderer.Emit(ctx, turn, agentengine.TurnEvent{Sequence: uint64(i), Kind: agentengine.TurnEventThoughtDelta, Thought: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(time.Second)
	for {
		_, p := progressMessage(t, service)
		if len(p.Items) == 1 && p.Items[0].Text == strings.Repeat("x", 10) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("tail not flushed: %+v", p)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := renderer.Complete(ctx, turn, agentengine.TurnResult{Status: agentengine.TurnCanceled}); err != nil {
		t.Fatal(err)
	}
}

func TestProgressKeepsAllToolsWithoutPublishingEveryUpdate(t *testing.T) {
	renderer, service, turn := progressFixture(t)
	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("tool-%d", i)
		for j, status := range []string{"started", "completed"} {
			kind := agentengine.TurnEventToolCallStart
			if j == 1 {
				kind = agentengine.TurnEventToolCallUpdate
			}
			if err := renderer.Emit(ctx, turn, agentengine.TurnEvent{Sequence: uint64(i*2 + j + 1), Kind: kind, Tool: &agentengine.ToolActivity{ID: id, Kind: "exec_command", Status: status}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := renderer.Complete(ctx, turn, agentengine.TurnResult{Status: agentengine.TurnSucceeded, Output: "Done"}); err != nil {
		t.Fatal(err)
	}
	_, p := progressMessage(t, service)
	if len(p.Items) != 1000 || p.Revision >= 100 {
		t.Fatalf("items=%d revisions=%d", len(p.Items), p.Revision)
	}
}

func TestProgressKeepsLastUnphasedTextWhenTurnEndsWithTool(t *testing.T) {
	renderer, service, turn := progressFixture(t)
	ctx := context.Background()
	_ = renderer.Emit(ctx, turn, agentengine.TurnEvent{Sequence: 1, Kind: agentengine.TurnEventTextDelta, Text: "The useful result."})
	_ = renderer.Emit(ctx, turn, agentengine.TurnEvent{Sequence: 2, Kind: agentengine.TurnEventToolCallStart, Tool: &agentengine.ToolActivity{ID: "x", Kind: "exec_command", Status: "started"}})
	if err := renderer.Complete(ctx, turn, agentengine.TurnResult{Status: agentengine.TurnSucceeded}); err != nil {
		t.Fatal(err)
	}
	msg, p := progressMessage(t, service)
	if msg.Content != "The useful result." || len(p.Items) != 1 {
		t.Fatalf("msg=%+v p=%+v", msg, p)
	}
}

func TestProgressAppliesAuthoritativeMessageSnapshot(t *testing.T) {
	renderer, service, turn := progressFixture(t)
	ctx := context.Background()
	_ = renderer.Emit(ctx, turn, agentengine.TurnEvent{Sequence: 1, Kind: agentengine.TurnEventTextDelta, ItemID: "answer", Phase: "final_answer", Text: "Draft"})
	_ = renderer.Emit(ctx, turn, agentengine.TurnEvent{Sequence: 2, Kind: agentengine.TurnEventTextDelta, ItemID: "answer", Phase: "final_answer", Text: "Corrected", TextSnapshot: true})
	if err := renderer.Complete(ctx, turn, agentengine.TurnResult{Status: agentengine.TurnSucceeded, Output: "Corrected"}); err != nil {
		t.Fatal(err)
	}
	message, _ := progressMessage(t, service)
	if message.Content != "Corrected" {
		t.Fatal(message.Content)
	}
}

func TestProgressOnlyTurnDoesNotWakeAnotherAgent(t *testing.T) {
	renderer, service, turn := progressFixture(t)
	ctx := context.Background()
	if err := renderer.Start(ctx, turn); err != nil {
		t.Fatal(err)
	}
	if err := renderer.Complete(ctx, turn, agentengine.TurnResult{Status: agentengine.TurnSucceeded}); err != nil {
		t.Fatal(err)
	}
	message, _ := progressMessage(t, service)
	if !im.IsAgentActivityMessage(message) {
		t.Fatal("empty progress completion became conversational input")
	}
}

func TestProgressFailureRetainsDiagnosticMetadata(t *testing.T) {
	renderer, service, turn := progressFixture(t)
	ctx := context.Background()
	if err := renderer.Start(ctx, turn); err != nil {
		t.Fatal(err)
	}
	if err := renderer.Complete(ctx, turn, agentengine.TurnResult{Status: agentengine.TurnFailed, Error: &agentengine.TurnError{Message: "specific runtime failure"}}); err != nil {
		t.Fatal(err)
	}
	message, _ := progressMessage(t, service)
	metadata := message.Metadata["csgclaw"].(map[string]any)
	if metadata["error_detail"] != "specific runtime failure" || metadata["runtime_error"] != true {
		t.Fatalf("missing diagnostics: %+v", metadata)
	}
}

func TestProgressPreservesStructuredContextFailures(t *testing.T) {
	for _, tc := range []struct {
		code   agentengine.ErrorCode
		zh, en string
	}{
		{"context_length_exceeded", "本次内容超过模型容量，请拆分输入、检查模型容量设置或选择容量更大的模型。已有对话已保留。", "This request exceeds the model context capacity."},
		{"context_compaction_failed", "暂时无法整理对话，已有记录已保留。请重试或选择容量更大的模型。", "The conversation could not be compacted."},
	} {
		for _, locale := range []string{"zh-CN", "en"} {
			for _, raw := range []string{"unexpected status 400: provider diagnostic", ""} {
				t.Run(string(tc.code)+"/"+locale+"/"+raw, func(t *testing.T) {
					renderer, service, turn := progressFixture(t)
					turn.Locale = locale
					ctx := context.Background()
					if err := renderer.Emit(ctx, turn, agentengine.TurnEvent{Kind: agentengine.TurnEventTextDelta, Phase: "commentary", Text: "Checking context."}); err != nil {
						t.Fatal(err)
					}
					if err := renderer.Complete(ctx, turn, agentengine.TurnResult{Status: agentengine.TurnFailed, Error: &agentengine.TurnError{Code: tc.code, Message: raw}}); err != nil {
						t.Fatal(err)
					}
					msg, progress := progressMessage(t, service)
					want := tc.en
					if locale == "zh-CN" {
						want = tc.zh
					}
					meta := msg.Metadata["csgclaw"].(map[string]any)
					if progress.Status != "failed" || !strings.HasPrefix(progress.Error, want) || meta["error_code"] != string(tc.code) {
						t.Fatalf("lost structured failure: progress=%+v metadata=%+v", progress, meta)
					}
					if raw != "" && (meta["error_detail"] != raw || strings.Contains(progress.Error, raw) || strings.Contains(msg.Content, raw)) {
						t.Fatal("raw diagnostics must remain in activity metadata only")
					}
				})
			}
		}
	}
}

func TestProgressSilenceIsScopedToCollaboration(t *testing.T) {
	for _, tc := range []struct {
		name, chatType, taskID string
		autonomous, private    bool
	}{
		{name: "autonomous", chatType: "group", autonomous: true, private: true},
		{name: "on-demand task", chatType: "group", taskID: "task-1", private: true},
		{name: "direct", chatType: "direct"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			renderer, service, turn := progressFixture(t)
			turn.Autonomous = tc.autonomous
			turn.ChatType = tc.chatType
			turn.TaskID = tc.taskID
			ctx := context.Background()
			if err := renderer.Emit(ctx, turn, agentengine.TurnEvent{Kind: agentengine.TurnEventTextDelta, Sequence: 1, Text: roomtask.AutonomousSilence}); err != nil {
				t.Fatal(err)
			}
			if err := renderer.Complete(ctx, turn, agentengine.TurnResult{Status: agentengine.TurnSucceeded, Output: roomtask.AutonomousSilence}); err != nil {
				t.Fatal(err)
			}
			message, progress := progressMessage(t, service)
			if progress.Status != "succeeded" || im.IsAgentActivityMessage(message) != tc.private ||
				strings.Contains(message.Content, roomtask.AutonomousSilence) == tc.private {
				t.Fatalf("unexpected delivery (want activity=%v): %+v", tc.private, message)
			}
		})
	}
}

func TestAutonomousFinalReplyReturnsToConversation(t *testing.T) {
	for _, tc := range []struct {
		name, sent, answer string
		thread, private    bool
	}{
		{name: "peer without mention", answer: "🔧 Looks good"},
		{name: "status after claim", sent: "I will check", answer: "Finished checking"},
		{name: "already published", sent: "Done", answer: "Done", private: true},
		{name: "explicit silence", answer: roomtask.AutonomousSilence, private: true},
		{name: "thread reply without mention", answer: "In thread", thread: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			renderer, service, turn := progressFixture(t)
			peer, _, err := service.EnsureAgentUser(im.EnsureAgentUserRequest{ID: "peer", Name: "Peer", Role: "worker"})
			if err != nil {
				t.Fatal(err)
			}
			room, err := service.CreateRoom(im.CreateRoomRequest{Type: apitypes.RoomTypeAutonomous, Title: "Collaboration", CreatorID: "user", MemberIDs: []string{"agent", peer.ID}})
			if err != nil {
				t.Fatal(err)
			}
			turn.RoomID = room.ID
			turn.Autonomous = true
			request := im.CreateMessageRequest{RoomID: room.ID, SenderID: peer.ID, Content: "Could you review this?"}
			if tc.thread {
				root, err := service.CreateMessage(im.CreateMessageRequest{RoomID: room.ID, SenderID: "user", Content: "Review"})
				if err != nil {
					t.Fatal(err)
				}
				turn.ThreadRootID = root.ID
				request.RelatesTo = &im.MessageRelation{RelType: "m.thread", EventID: root.ID}
			}
			source, err := service.CreateMessage(request)
			if err != nil {
				t.Fatal(err)
			}
			turn.SourceMessageID = source.ID
			if err := renderer.Start(ctx, turn); err != nil {
				t.Fatal(err)
			}
			if tc.sent != "" {
				if _, err := service.CreateMessage(im.CreateMessageRequest{RoomID: room.ID, SenderID: "agent", Content: tc.sent, RelatesTo: request.RelatesTo}); err != nil {
					t.Fatal(err)
				}
			}
			if err := renderer.Emit(ctx, turn, agentengine.TurnEvent{Sequence: 1, Kind: agentengine.TurnEventTextDelta, Phase: "commentary", Text: "Checking"}); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := renderer.Complete(ctx, turn, agentengine.TurnResult{Status: agentengine.TurnSucceeded, Output: tc.answer}); err != nil {
					t.Fatal(err)
				}
			}
			page, err := service.PrepareUnread(room.ID, peer.ID, turn.ThreadRootID, 50, true)
			if err != nil {
				t.Fatal(err)
			}
			var delivered int
			for _, message := range page.Messages {
				if message.ID == finalMessageID(turn) {
					delivered++
					if message.Content != tc.answer {
						t.Fatalf("wrong reply: %q", message.Content)
					}
				}
			}
			want := 1
			if tc.private {
				want = 0
			}
			if delivered != want {
				t.Fatalf("final reply appears %d times in the peer inbox, want %d", delivered, want)
			}
		})
	}
}

func TestCollaborationToolDetailsKeepNativeArgumentsAndResults(t *testing.T) {
	for _, schema := range []string{"codex", "dsh"} {
		t.Run(schema, func(t *testing.T) {
			input := map[string]any{"query": strings.Repeat("I", 500), "ack_token": "[redacted]"}
			output := map[string]any{"text": strings.Repeat("O", 2000), "tail": "END"}
			payload := map[string]any{"arguments": input, "result": output}
			if schema == "dsh" {
				payload = map[string]any{"rawInput": input, "content": output}
			}
			tool := agentengine.ToolActivity{Kind: "mcp_tool_call", Title: "messages_read", Status: "completed", InputSummary: "short input", OutputSummary: "short output", Payload: payload}
			full := progressToolDetails(tool, nil, true)
			if !strings.Contains(full.Input, strings.Repeat("I", 500)) || !strings.Contains(full.Output, "END") || full.Truncated || full.DetailsVersion != 1 {
				t.Fatalf("lost full details: %+v", full)
			}
			direct := progressTool(tool, nil)
			if direct.Input != "short input" || direct.Output != "short output" || direct.DetailsVersion != 0 {
				t.Fatalf("single agent presentation changed: %+v", direct)
			}
			payload["result"] = map[string]any{"text": strings.Repeat("界", 30000)}
			delete(payload, "content")
			truncated := progressToolDetails(tool, nil, true)
			if !truncated.Truncated || len(truncated.Output) > 64<<10 || !utf8.ValidString(truncated.Output) {
				t.Fatal("missing safe truncation")
			}
		})
	}
}

func TestCollaborationDetailProtectsEmbeddedCredentialsWithoutLosingLongResults(t *testing.T) {
	text := `{"record_id":9007199254740993,"ack_token":"fixture-ack-secret","result":"` + strings.Repeat("A", 2048) + `"}`
	detail := progressDetailJSON(map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}})
	if strings.Contains(detail, "fixture-ack-secret") || !strings.Contains(detail, "[redacted]") || !strings.Contains(detail, "9007199254740993") || !strings.Contains(detail, strings.Repeat("A", 2048)) {
		t.Fatal("credential protection or full detail preservation failed")
	}
}
