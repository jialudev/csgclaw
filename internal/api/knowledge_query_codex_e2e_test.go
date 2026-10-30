package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"csgclaw/internal/knowledgebase"
	agentruntime "csgclaw/internal/runtime"
	"csgclaw/internal/runtime/codex"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type knowledgeTestBinary string

func (b knowledgeTestBinary) Ensure(context.Context) (string, error) { return string(b), nil }

// Opt in with the bundled binary. The real Codex process connects only to
// isolated local model/MCP fixtures. No customer credentials or deployment.
func TestKnowledgeQueryBundledCodexE2E(t *testing.T) {
	binary := os.Getenv("CSGCLAW_TEST_CODEX_BINARY")
	if binary == "" {
		t.Skip("set CSGCLAW_TEST_CODEX_BINARY to the bundled Codex binary")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, "host-codex"))
	fixture := knowledgeQueryFixture()
	var searches atomic.Int32
	upstream := mcp.NewServer(&mcp.Implementation{Name: "fixture-knowledge-base", Version: "1"}, nil)
	rawTool, _ := json.Marshal(knowledgeQueryToolFixture())
	var tool mcp.Tool
	if err := json.Unmarshal(rawTool, &tool); err != nil {
		t.Fatal(err)
	}
	upstream.AddTool(&tool, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		searches.Add(1)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fixture["content"].([]any)[0].(map[string]any)["text"].(string)}}, StructuredContent: fixture["structuredContent"]}, nil
	})
	upstreamServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return upstream }, &mcp.StreamableHTTPOptions{JSONResponse: true}))
	defer upstreamServer.Close()
	oldLoader := loadKnowledgeBaseConnection
	t.Cleanup(func() { loadKnowledgeBaseConnection = oldLoader })
	loadKnowledgeBaseConnection = func(context.Context) (knowledgeBaseConnection, error) {
		return knowledgeBaseConnection{AIGatewayBaseURL: upstreamServer.URL, CSGHubAccessToken: "fixture-account"}, nil
	}
	h := &Handler{serverAccessToken: "fixture-local-token"}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("content_id", "fixture-kb")
		h.handleKnowledgeBaseMCPProxy(w, r)
	}))
	defer proxy.Close()
	var requests atomic.Int32
	var collected []any
	var resultError string
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		n := requests.Add(1)
		cursor := ""
		finished := false
		if n == 1 {
			schema, _ := json.Marshal(body["tools"])
			for _, want := range []string{knowledgebase.QueryCursorArgument, "maximum: 50", "not a JSON-encoded string"} {
				if !strings.Contains(string(schema), want) {
					t.Errorf("model tool schema lost %q", want)
				}
			}
		} else {
			var output any
			input, _ := body["input"].([]any)
			for _, raw := range input {
				item, _ := raw.(map[string]any)
				if item["type"] == "function_call_output" {
					output = item["output"]
				}
			}
			text, ok := output.(string)
			if !ok {
				raw, _ := json.Marshal(output)
				text = string(raw)
			}
			// Codex adds a wall-time wrapper before serializing the MCP payload.
			start := strings.Index(text, `{"coverage"`)
			if start < 0 {
				start = strings.Index(text, `{"delivery"`)
			}
			var page map[string]any
			if start < 0 || json.Unmarshal([]byte(text[start:]), &page) != nil {
				resultError = fmt.Sprintf("model received a truncated/unreadable page: %s", text)
				finished = true
			} else {
				rows, _ := page["evidence"].([]any)
				collected = append(collected, rows...)
				info, _ := page["delivery"].(map[string]any)
				cursor, _ = info["next_cursor"].(string)
				finished = info["has_more"] == false
				if len(rows) == 0 {
					resultError = "no evidence reached model"
					finished = true
				}
			}
		}
		if n > 30 {
			resultError = "paging loop"
			finished = true
		}
		w.Header().Set("Content-Type", "text/event-stream")
		emit := func(value any) { raw, _ := json.Marshal(value); fmt.Fprintf(w, "data: %s\n\n", raw) }
		emit(map[string]any{"type": "response.created", "response": map[string]any{"id": fmt.Sprintf("r-%d", n)}})
		if finished {
			emit(map[string]any{"type": "response.output_item.done", "item": map[string]any{"type": "message", "role": "assistant", "id": "answer", "content": []any{map[string]any{"type": "output_text", "text": "Fixture complete."}}}})
		} else {
			args := map[string]any{"query": "all fixture rows", "top_k": 50}
			if cursor != "" {
				args[knowledgebase.QueryCursorArgument] = cursor
			}
			raw, _ := json.Marshal(args)
			emit(map[string]any{"type": "response.output_item.done", "item": map[string]any{"type": "function_call", "call_id": fmt.Sprintf("c-%d", n), "namespace": "mcp__fixture", "name": "query", "arguments": string(raw)}})
		}
		emit(map[string]any{"type": "response.completed", "response": map[string]any{"id": fmt.Sprintf("r-%d", n), "usage": map[string]any{"input_tokens": 1000, "output_tokens": 20, "total_tokens": 1020}}})
	}))
	defer model.Close()
	profile := agentruntime.Profile{Provider: "api", ModelID: "fixture-model", BaseURL: model.URL + "/v1", APIKey: "fixture-key"}
	rt := codex.New(codex.Dependencies{BinaryProvider: knowledgeTestBinary(binary), AgentHome: func(string) (string, error) { return filepath.Join(home, "agent"), nil }, ResolveAgent: func(agentruntime.Handle) (codex.AgentRef, error) {
		return codex.AgentRef{ID: "fixture-agent", Name: "fixture", Profile: profile, RuntimeOptions: map[string]any{"memory_mode": "disabled"}, MCPServers: map[string]any{"fixture": map[string]any{"url": proxy.URL, "headers": map[string]any{"Authorization": "Bearer fixture-local-token"}}}}, nil
	}})
	defer rt.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	handle, err := rt.New(ctx, agentruntime.Spec{RuntimeID: "rt-fixture", AgentID: "fixture-agent", AgentName: "fixture", Profile: profile})
	if err != nil {
		t.Fatal(err)
	}
	thread, err := rt.EnsureEngineSession(ctx, handle.RuntimeID, "room:fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.Prompt(ctx, handle.RuntimeID, thread, "Read every fixture evidence page using only the knowledge-base query tool."); err != nil {
		t.Fatal(err)
	}
	if resultError != "" {
		t.Fatal(resultError)
	}
	if searches.Load() != 1 || requests.Load() < 3 {
		t.Fatalf("searches=%d model requests=%d", searches.Load(), requests.Load())
	}
	if !reflect.DeepEqual(collected, fixture["structuredContent"].(map[string]any)["evidence"]) {
		t.Fatalf("model received %d of 50 rows, or fields/citations changed", len(collected))
	}
	t.Logf("real Codex received all 50 rows and citations in %d pages from %d backend search", requests.Load()-1, searches.Load())
}
