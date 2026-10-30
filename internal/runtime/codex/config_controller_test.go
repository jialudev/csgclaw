package codex

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"csgclaw/internal/modelpreset"
	"csgclaw/internal/modelprovider"
	agentruntime "csgclaw/internal/runtime"
)

func TestValidateConfigProbesExplicitChatProtocol(t *testing.T) {
	codexResponsesProbeCache.clear()
	t.Cleanup(codexResponsesProbeCache.clear)
	chatCalls, responsesCalls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/responses" {
			responsesCalls++
			http.Error(w, `{"error":{"code":"InvalidParameter","message":"Model unsupported by Responses"}}`, http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		chatCalls++
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("X-Test") != "forwarded" {
			t.Error("provider credentials or headers not forwarded")
		}
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model != "chat-only-model" {
			t.Error("probe did not use the selected model")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()
	profile := agentruntime.RuntimeProfileConfig{Provider: "api", APIProtocol: modelpreset.ChatCompletions, BaseURL: server.URL, APIKey: "test-key", ModelID: "chat-only-model", Headers: map[string]string{"X-Test": "forwarded"}}
	rt := &Runtime{}
	for i := 0; i < 2; i++ {
		if err := rt.ValidateConfig(context.Background(), agentruntime.RuntimeConfigSnapshot{Profile: profile}); err != nil {
			t.Fatalf("explicit Chat validation failed: %v", err)
		}
	}
	if chatCalls != 1 || responsesCalls != 0 {
		t.Fatalf("calls: Chat=%d Responses=%d; want 1 cached Chat probe only", chatCalls, responsesCalls)
	}
	profile.APIProtocol = modelpreset.Responses
	if err := rt.ValidateConfig(context.Background(), agentruntime.RuntimeConfigSnapshot{Profile: profile}); err == nil {
		t.Fatal("Responses configuration reused Chat success instead of probing its endpoint")
	}
	if responsesCalls != 1 {
		t.Fatalf("Responses calls=%d, want 1", responsesCalls)
	}
}

func TestValidateConfigExplicitChatRejectsAuthenticationFailure(t *testing.T) {
	codexResponsesProbeCache.clear()
	t.Cleanup(codexResponsesProbeCache.clear)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
		http.Error(w, `{"error":{"message":"Unauthorized"}}`, http.StatusUnauthorized)
	}))
	defer server.Close()
	err := (&Runtime{}).ValidateConfig(context.Background(), agentruntime.RuntimeConfigSnapshot{Profile: agentruntime.RuntimeProfileConfig{Provider: "api", APIProtocol: modelpreset.ChatCompletions, BaseURL: server.URL, ModelID: "chat-only-model"}})
	var status *modelprovider.ResponsesAPIStatusError
	if !errors.As(err, &status) || status.StatusCode != http.StatusUnauthorized {
		t.Fatalf("error=%v, want authentication rejection", err)
	}
}

func TestValidateConfigDoesNotReusePreviousCredentials(t *testing.T) {
	codexResponsesProbeCache.clear()
	t.Cleanup(codexResponsesProbeCache.clear)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer valid-key" || r.Header.Get("X-Account") != "valid-account" {
			http.Error(w, `{"error":{"message":"Unauthorized"}}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()
	profile := agentruntime.RuntimeProfileConfig{Provider: "api", APIProtocol: modelpreset.ChatCompletions, BaseURL: server.URL, APIKey: "valid-key", ModelID: "fixture-model", Headers: map[string]string{"X-Account": "valid-account"}}
	validate := func() error {
		return (&Runtime{}).ValidateConfig(context.Background(), agentruntime.RuntimeConfigSnapshot{Profile: profile})
	}
	if err := validate(); err != nil {
		t.Fatal(err)
	}
	profile.APIKey = "wrong-key"
	if validate() == nil {
		t.Fatal("changed API key reused cached success")
	}
	profile.APIKey = "valid-key"
	profile.Headers["X-Account"] = "wrong-account"
	if validate() == nil {
		t.Fatal("changed authentication header reused cached success")
	}
}

func TestReconcileConfigRotatesConversationsWhenModelChanges(t *testing.T) {
	root := t.TempDir()
	rt := newTestCodexRuntime(root, func(h agentruntime.Handle) (AgentRef, error) {
		return AgentRef{ID: "agent-manager", Name: "manager", RuntimeID: h.RuntimeID}, nil
	})
	runtimeID := "rt-agent-manager"
	if err := rt.mkdirAll(filepath.Join(root, "agent-manager", hostStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir runtime dir: %v", err)
	}
	if err := rt.writeSessionMetadata(sessionMetadata{
		DynamicToolsVersion:         engineDynamicToolsVersion,
		RuntimeID:                   runtimeID,
		ConversationSessions:        map[string]string{"room-1": "gpt-thread"},
		FilePublishingConversations: map[string]bool{"room-1": true},
	}); err != nil {
		t.Fatalf("writeSessionMetadata() error = %v", err)
	}

	err := rt.ReconcileConfig(context.Background(), agentruntime.Handle{RuntimeID: runtimeID}, agentruntime.RuntimeConfigChange{
		Previous: agentruntime.RuntimeConfigSnapshot{Profile: agentruntime.RuntimeProfileConfig{Provider: "codex", BaseURL: "https://old.example/v1", ModelID: "gpt-5"}},
		Current:  agentruntime.RuntimeConfigSnapshot{Profile: agentruntime.RuntimeProfileConfig{Provider: "csghub", BaseURL: "https://new.example/v1/", ModelID: "glm-5.1"}},
	})
	if err != nil {
		t.Fatalf("ReconcileConfig() error = %v", err)
	}
	meta, err := rt.readSessionMetadata(runtimeID)
	if err != nil {
		t.Fatalf("readSessionMetadata() error = %v", err)
	}
	if len(meta.ConversationSessions) != 0 || len(meta.FilePublishingConversations) != 0 {
		t.Fatalf("persisted conversations = %#v, publishing = %#v; want both cleared", meta.ConversationSessions, meta.FilePublishingConversations)
	}
}

func TestReconcileConfigPreservesConversationsWhenOnlyReasoningChanges(t *testing.T) {
	root := t.TempDir()
	rt := newTestCodexRuntime(root, func(h agentruntime.Handle) (AgentRef, error) {
		return AgentRef{ID: "agent-manager", Name: "manager", RuntimeID: h.RuntimeID}, nil
	})
	runtimeID := "rt-agent-manager"
	if err := rt.mkdirAll(filepath.Join(root, "agent-manager", hostStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir runtime dir: %v", err)
	}
	if err := rt.writeSessionMetadata(sessionMetadata{
		DynamicToolsVersion:         engineDynamicToolsVersion,
		RuntimeID:                   runtimeID,
		ConversationSessions:        map[string]string{"room-1": "existing-thread"},
		FilePublishingConversations: map[string]bool{"room-1": true},
	}); err != nil {
		t.Fatalf("writeSessionMetadata() error = %v", err)
	}

	err := rt.ReconcileConfig(context.Background(), agentruntime.Handle{RuntimeID: runtimeID}, agentruntime.RuntimeConfigChange{
		Previous: agentruntime.RuntimeConfigSnapshot{Profile: agentruntime.RuntimeProfileConfig{Provider: "api", BaseURL: "https://example.test/v1", ModelID: "gpt-5", ReasoningEffort: "low"}},
		Current:  agentruntime.RuntimeConfigSnapshot{Profile: agentruntime.RuntimeProfileConfig{Provider: "api", BaseURL: "https://example.test/v1/", ModelID: "gpt-5", ReasoningEffort: "high"}},
	})
	if err != nil {
		t.Fatalf("ReconcileConfig() error = %v", err)
	}
	meta, err := rt.readSessionMetadata(runtimeID)
	if err != nil {
		t.Fatalf("readSessionMetadata() error = %v", err)
	}
	if got := meta.ConversationSessions["room-1"]; got != "existing-thread" {
		t.Fatalf("conversation thread = %q, want existing-thread", got)
	}
	if !meta.FilePublishingConversations["room-1"] {
		t.Fatal("file publishing conversation was cleared for a reasoning-only change")
	}
}

func TestValidateConfigResolvesOpenCSGCredentialsForResponsesProbe(t *testing.T) {
	restoreProbe := TestOnlySetResponsesAPIProbe(func(_ context.Context, baseURL, apiKey, modelID string, headers map[string]string) error {
		if baseURL != "https://ai.space.opencsg.com/v1" {
			t.Fatalf("probe baseURL = %q, want OpenCSG AIGateway", baseURL)
		}
		if apiKey != "gateway-key" {
			t.Fatalf("probe apiKey = %q, want gateway-key", apiKey)
		}
		if modelID != "glm-5.1" {
			t.Fatalf("probe modelID = %q, want glm-5.1", modelID)
		}
		if len(headers) != 0 {
			t.Fatalf("probe headers = %#v, want empty", headers)
		}
		return nil
	})
	defer restoreProbe()

	previousCreds := openCSGCredentialsForResponsesProbe
	openCSGCredentialsForResponsesProbe = func(context.Context) (string, string, bool, error) {
		return "https://ai.space.opencsg.com/v1", "gateway-key", true, nil
	}
	defer func() {
		openCSGCredentialsForResponsesProbe = previousCreds
	}()

	rt := &Runtime{}
	err := rt.ValidateConfig(context.Background(), agentruntime.RuntimeConfigSnapshot{
		Profile: agentruntime.RuntimeProfileConfig{
			Provider: "csghub",
			ModelID:  "glm-5.1",
		},
	})
	if err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}

func TestValidateConfigRejectsProviderWithoutUsableResponsesOrChatEndpoint(t *testing.T) {
	restoreProbe := TestOnlySetResponsesAPIProbe(func(context.Context, string, string, string, map[string]string) error {
		return modelprovider.ErrResponsesAPIUnsupported
	})
	defer restoreProbe()

	rt := &Runtime{}
	err := rt.ValidateConfig(context.Background(), agentruntime.RuntimeConfigSnapshot{
		Profile: agentruntime.RuntimeProfileConfig{
			Provider: "api",
			BaseURL:  "https://wrong.example/v1",
			APIKey:   "test-key",
			ModelID:  "qwen-test",
		},
	})
	if !errors.Is(err, modelprovider.ErrResponsesAPIUnsupported) {
		t.Fatalf("ValidateConfig() error = %v, want unsupported provider rejection", err)
	}
}

func TestRestartRequiredWhenMemoryModeChanges(t *testing.T) {
	rt := &Runtime{}
	restart, err := rt.RestartRequired(agentruntime.RuntimeConfigChange{
		Previous: agentruntime.RuntimeConfigSnapshot{Options: map[string]any{memoryModeOptionKey: MemoryModeEnabled}},
		Current:  agentruntime.RuntimeConfigSnapshot{Options: map[string]any{memoryModeOptionKey: MemoryModeDisabled}},
	})
	if err != nil {
		t.Fatalf("RestartRequired() error = %v", err)
	}
	if !restart {
		t.Fatal("RestartRequired() = false, want true for memory mode change")
	}
}

func TestValidateConfigDefersModelCheck(t *testing.T) {
	t.Cleanup(TestOnlySetResponsesAPIProbe(func(context.Context, string, string, string, map[string]string) error {
		t.Fatal("model probe called during platform template creation")
		return nil
	}))
	rt := &Runtime{}
	if err := rt.ValidateConfig(context.Background(), agentruntime.RuntimeConfigSnapshot{SkipModelAvailabilityCheck: true}); err != nil {
		t.Fatal(err)
	}
}

func TestDeferredModelCheckStillValidatesRuntimeOptions(t *testing.T) {
	t.Setenv("CSGCLAW_EXECUTION_MODE", ExecutionModeStandard)
	rt := &Runtime{}
	err := rt.ValidateConfig(context.Background(), agentruntime.RuntimeConfigSnapshot{
		SkipModelAvailabilityCheck: true,
		Options:                    map[string]any{"memory_mode": "invalid"},
	})
	if err == nil {
		t.Fatal("invalid runtime options accepted")
	}
}
