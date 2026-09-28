package dsh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"csgclaw/internal/agentengine/contract"
	"csgclaw/internal/dshcli"
	agentruntime "csgclaw/internal/runtime"
)

// The real DSH process discovers and executes the managed tool against a local
// chat fixture. No external model, image credentials, or API requests are used.
func TestImageGenerationNativeDSHE2E(t *testing.T) {
	binary := os.Getenv("CSGCLAW_TEST_DSH_BINARY")
	if binary == "" {
		t.Skip("set CSGCLAW_TEST_DSH_BINARY to run native DSH image generation")
	}
	for _, mode := range []string{"success", "provider_error", "read-only"} {
		t.Run(mode, func(t *testing.T) { testImageGenerationNativeDSH(t, binary, mode) })
	}
}

func testImageGenerationNativeDSH(t *testing.T, binary, mode string) {
	var requests, calls atomic.Int64
	const prompt = "帮我生成一个李白的图片"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Tools []struct {
				Function struct{ Name string } `json:"function"`
			} `json:"tools"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			http.Error(w, "invalid request", 400)
			return
		}
		n := requests.Add(1)
		found := false
		for _, tool := range body.Tools {
			found = found || tool.Function.Name == "csgclaw_generate_image"
		}
		if found != (mode != "read-only") {
			t.Errorf("native image tool available = %v in mode %s", found, mode)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		emit := func(delta map[string]any, finish any) {
			data, _ := json.Marshal(map[string]any{"id": fmt.Sprintf("response-%d", n), "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
		last := body.Messages[len(body.Messages)-1]
		if last.Role != "tool" && mode != "read-only" {
			args, _ := json.Marshal(map[string]string{"prompt": prompt})
			emit(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprintf("call-image-%d", n), "type": "function", "function": map[string]any{"name": "csgclaw_generate_image", "arguments": string(args)}}}}, nil)
			emit(map[string]any{}, "tool_calls")
		} else {
			if mode == "success" && !strings.Contains(last.Content, "Image generated and delivered") {
				t.Errorf("DSH image tool did not confirm delivery: %s", last.Content)
			}
			if mode == "provider_error" && !strings.Contains(last.Content, "image_model_not_configured") {
				t.Errorf("DSH lost the image provider error: %s", last.Content)
			}
			emit(map[string]any{"role": "assistant", "content": "done"}, nil)
			emit(map[string]any{}, "stop")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	home := filepath.Join(t.TempDir(), "agent")
	profile := agentruntime.Profile{BaseURL: server.URL + "/v1", APIKey: "fixture-key", ModelID: "fixture-model"}
	options := map[string]any{"permission_mode": PermissionModeWorkspaceWrite}
	if mode == "read-only" {
		options["permission_mode"] = PermissionModeReadOnly
	}
	rt := New(Dependencies{
		ResolveBinary: func(context.Context) (dshcli.Info, error) { return dshcli.Info{Path: binary}, nil },
		ResolveAgent: func(agentruntime.Handle) (AgentRef, error) {
			return AgentRef{ID: "alice", RuntimeID: "rt-alice", Profile: profile, RuntimeOptions: options}, nil
		},
		AgentHome: func(string) (string, error) { return home, nil },
	})
	defer rt.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = contract.WithImageGenerationHandler(ctx, func(_ context.Context, id, gotPrompt string) error {
		calls.Add(1)
		if !strings.HasPrefix(id, "call-image-") || gotPrompt != prompt {
			t.Errorf("native image request = %q, %q", id, gotPrompt)
		}
		if mode == "provider_error" {
			return fmt.Errorf("image_model_not_configured")
		}
		return nil
	})
	if err := rt.Provision(ctx, agentruntime.ProvisionRequest{RuntimeID: "rt-alice", AgentID: "alice", Profile: profile}); err != nil {
		t.Fatal(err)
	}
	handle, err := rt.New(ctx, agentruntime.Spec{RuntimeID: "rt-alice", AgentID: "alice", Profile: profile})
	if err != nil {
		t.Fatal(err)
	}
	steps := []string{"new"}
	if mode == "success" {
		steps = append(steps, "continue", "resume", "reset")
	}
	sessionID := ""
	for i, step := range steps {
		if step == "resume" {
			if _, err := rt.Stop(ctx, handle); err != nil {
				t.Fatal(err)
			}
			if _, err := rt.Start(ctx, handle); err != nil {
				t.Fatal(err)
			}
		}
		if step == "reset" {
			if err := rt.Conversation(handle.RuntimeID).Reset(ctx, "room:image"); err != nil {
				t.Fatal(err)
			}
		}
		result := rt.Conversation(handle.RuntimeID).Run(ctx, contract.TurnRequest{ID: contract.TurnID(fmt.Sprintf("image-%d", i)), ConversationKey: "room:image", Input: []contract.InputPart{{Kind: contract.InputPartText, Text: prompt}}}, contract.EventSinkFunc(func(context.Context, contract.TurnEvent) error { return nil }))
		if result.Status != contract.TurnSucceeded || result.Output != "done" {
			log, _ := os.ReadFile(filepath.Join(home, hostStateDirName, stderrFileName))
			t.Fatalf("native image turn %s = %+v\n%s", step, result, log)
		}
		proc, err := rt.process(handle.RuntimeID)
		if err != nil {
			t.Fatal(err)
		}
		proc.mu.Lock()
		current := proc.meta.Sessions["room:image"]
		proc.mu.Unlock()
		if (step == "continue" || step == "resume") && current != sessionID {
			t.Fatalf("session changed from %s to %s", sessionID, current)
		}
		if step == "reset" && current == sessionID {
			t.Fatal("reset retained the old session")
		}
		sessionID = current
	}
	wantCalls := int64(len(steps))
	if mode == "read-only" {
		wantCalls = 0
	}
	if calls.Load() != wantCalls || requests.Load() == 0 {
		t.Fatalf("native image calls = %d, want %d; model requests = %d", calls.Load(), wantCalls, requests.Load())
	}
}
