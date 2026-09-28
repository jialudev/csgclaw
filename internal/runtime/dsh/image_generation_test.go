package dsh

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"csgclaw/internal/agentengine/contract"
	agentruntime "csgclaw/internal/runtime"
)

func TestImageGenerationUsesExactActiveSession(t *testing.T) {
	calls := map[string]int{}
	proc := &process{active: map[string]*activeTurn{}}
	for _, session := range []string{"session-1", "session-2"} {
		ctx := contract.WithImageGenerationHandler(context.Background(), func(_ context.Context, id, prompt string) error {
			calls[session]++
			if id != "call-image" || prompt != "帮我生成一个李白的图片" {
				t.Errorf("image request = %q, %q", id, prompt)
			}
			return nil
		})
		proc.active[session] = &activeTurn{ctx: ctx}
	}
	args := imageGenerationRequest{SessionID: "session-2", CallID: "call-image", Prompt: "帮我生成一个李白的图片"}
	if err := proc.generateImage(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	if calls["session-1"] != 0 || calls["session-2"] != 1 {
		t.Fatalf("image delivered to wrong session: %v", calls)
	}
	for _, session := range []string{"unknown-session", "session-2"} {
		delete(proc.active, "session-2")
		args.SessionID = session
		if err := proc.generateImage(context.Background(), args); err == nil {
			t.Fatalf("accepted inactive session %q", session)
		}
	}
}

func TestImageGenerationRejectsInvalidAndReadOnlyRequests(t *testing.T) {
	for _, test := range []struct {
		name string
		mode string
		args imageGenerationRequest
	}{
		{name: "read-only", mode: PermissionModeReadOnly, args: imageGenerationRequest{"session-1", "call-1", "blue sky"}},
		{name: "missing session", args: imageGenerationRequest{"", "call-1", "blue sky"}},
		{name: "missing call", args: imageGenerationRequest{"session-1", "", "blue sky"}},
		{name: "empty prompt", args: imageGenerationRequest{"session-1", "call-1", " \n"}},
		{name: "oversized prompt", args: imageGenerationRequest{"session-1", "call-1", strings.Repeat("x", 32001)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := contract.WithImageGenerationHandler(context.Background(), func(context.Context, string, string) error {
				t.Error("rejected request reached the image provider")
				return nil
			})
			proc := &process{meta: runtimeMetadata{PermissionMode: test.mode}, active: map[string]*activeTurn{"session-1": {ctx: ctx}}}
			if err := proc.generateImage(context.Background(), test.args); err == nil {
				t.Fatal("request was accepted")
			}
		})
	}
}

func TestImageGenerationCancellation(t *testing.T) {
	for _, source := range []string{"turn", "tool"} {
		t.Run(source, func(t *testing.T) {
			turnCtx, cancelTurn := context.WithCancel(context.Background())
			defer cancelTurn()
			toolCtx, cancelTool := context.WithCancel(context.Background())
			defer cancelTool()
			started := make(chan struct{})
			turnCtx = contract.WithImageGenerationHandler(turnCtx, func(ctx context.Context, _, _ string) error {
				close(started)
				<-ctx.Done()
				return ctx.Err()
			})
			proc := &process{active: map[string]*activeTurn{"session-1": {ctx: turnCtx}}}
			done := make(chan error, 1)
			go func() { done <- proc.generateImage(toolCtx, imageGenerationRequest{"session-1", "call-1", "blue sky"}) }()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("image generation did not start")
			}
			if source == "turn" {
				cancelTurn()
			} else {
				cancelTool()
			}
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation error = %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("image generation ignored cancellation")
			}
		})
	}
}

func TestImageGenerationBridgeAuthenticationAndProviderError(t *testing.T) {
	calls := 0
	ctx := contract.WithImageGenerationHandler(context.Background(), func(context.Context, string, string) error {
		calls++
		return errors.New("image_model_not_configured")
	})
	proc := &process{active: map[string]*activeTurn{"session-1": {ctx: ctx}}}
	server, env, err := startImageGenerationBridge(proc)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	values := environmentMap(env)
	for _, token := range []string{"wrong-token", values[imageBridgeTokenEnvName]} {
		request, err := http.NewRequest(http.MethodPost, values[imageBridgeURLEnvName], strings.NewReader(`{"sessionId":"session-1","callId":"call-1","prompt":"blue sky"}`))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		if token == "wrong-token" {
			response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized || calls != 0 {
				t.Fatal("unauthorized caller reached the image provider")
			}
			continue
		}
		var result struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		err = json.NewDecoder(response.Body).Decode(&result)
		response.Body.Close()
		if err != nil || result.Success || result.Message != "image_model_not_configured" || calls != 1 {
			t.Fatalf("provider failure = %+v, error = %v, calls = %d", result, err, calls)
		}
	}
}

func TestImageBridgeEnvironmentCannotBeOverridden(t *testing.T) {
	t.Setenv(imageBridgeURLEnvName, "http://ambient.example")
	t.Setenv(imageBridgeTokenEnvName, "ambient-token")
	profile := agentruntime.Profile{Env: map[string]string{imageBridgeURLEnvName: "http://profile.example", imageBridgeTokenEnvName: "profile-token"}}
	env := environmentMap(buildEnvironment(profile, t.TempDir(), PermissionModeWorkspaceWrite))
	if env[imageBridgeURLEnvName] != "" || env[imageBridgeTokenEnvName] != "" {
		t.Fatal("image bridge inherited untrusted host or profile configuration")
	}
}
