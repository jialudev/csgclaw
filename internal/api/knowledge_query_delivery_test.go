package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"csgclaw/internal/knowledgebase"
)

// Synthetic evidence only; never put a customer export in a test fixture.
func knowledgeQueryFixture() map[string]any {
	rows := make([]any, 50)
	for i := range rows {
		rows[i] = map[string]any{"page_id": fmt.Sprintf("source-row-%02d", i), "snippet": strings.Repeat("完整的中文来源证据。", 20), "area": fmt.Sprintf("%d.62", 4000+i), "source_files": []any{map[string]any{"source_id": "original-table", "display_name": "fixture.xlsx"}}}
	}
	payload := map[string]any{"schema_version": "kb-query-result.v2", "evidence": rows, "coverage": map[string]any{"status": "complete"}, "generation_id": "fixture-generation", "truncated": false}
	raw, _ := json.Marshal(payload)
	return map[string]any{"content": []any{map[string]any{"type": "text", "text": string(raw)}}, "structuredContent": payload}
}

func knowledgeQueryToolFixture() map[string]any {
	return map[string]any{"name": "query", "inputSchema": map[string]any{"type": "object", "additionalProperties": false, "required": []any{"query"}, "properties": map[string]any{"query": map[string]any{"type": "string"}, "top_k": map[string]any{"type": "integer", "minimum": 1, "maximum": 50, "default": 8}, "filters": map[string]any{"$ref": "#/$defs/Filters"}}, "$defs": map[string]any{"Filters": map[string]any{"type": "object", "properties": map[string]any{"source_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}}}}}
}

func TestKnowledgeQueryProxyPagesJSONAndSSEWithoutRepeatingSearch(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("sse=%v", stream), func(t *testing.T) {
			oldLoader, oldClient := loadKnowledgeBaseConnection, knowledgeBaseProxyHTTPClient
			t.Cleanup(func() { loadKnowledgeBaseConnection, knowledgeBaseProxyHTTPClient = oldLoader, oldClient })
			connection := knowledgeBaseConnection{AIGatewayBaseURL: "https://gateway.example.test/v1", CSGHubAccessToken: "account-one"}
			loadKnowledgeBaseConnection = func(context.Context) (knowledgeBaseConnection, error) { return connection, nil }
			calls := 0
			fixture := knowledgeQueryFixture()
			knowledgeBaseProxyHTTPClient = &http.Client{Transport: knowledgeBaseRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				raw, _ := io.ReadAll(r.Body)
				if !bytes.Contains(raw, []byte(`"progressToken":"test-progress"`)) {
					t.Error("original MCP metadata lost")
				}
				if r.Header.Get("Authorization") != "Bearer account-one" || r.Header.Get("Cookie") != "" {
					t.Error("wrong upstream credentials")
				}
				body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": json.Number("9007199254740993"), "result": fixture})
				contentType := "application/json"
				if stream {
					contentType = "text/event-stream"
					body = []byte("event: message\nid: event-1\ndata: " + string(body) + "\n\n")
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {contentType}, "Content-Length": {fmt.Sprint(len(body))}, "Mcp-Session-Id": {"session-1"}}, Body: io.NopCloser(bytes.NewReader(body))}, nil
			})}
			h := &Handler{serverAccessToken: "local-token"}
			request := func(kb, token string, args map[string]any) (map[string]any, int) {
				t.Helper()
				body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": json.Number("9007199254740993"), "method": "tools/call", "params": map[string]any{"name": "query", "arguments": args, "_meta": map[string]any{"progressToken": "test-progress"}}})
				r := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
				r.SetPathValue("content_id", kb)
				r.Header.Set("Authorization", "Bearer "+token)
				r.Header.Set("Cookie", "private=1")
				w := httptest.NewRecorder()
				h.handleKnowledgeBaseMCPProxy(w, r)
				if w.Code != 200 {
					return nil, w.Code
				}
				response := w.Body.Bytes()
				if strings.Contains(w.Header().Get("Content-Type"), "event-stream") {
					for _, line := range strings.Split(string(response), "\n") {
						if strings.HasPrefix(line, "data: ") {
							response = []byte(strings.TrimPrefix(line, "data: "))
							break
						}
					}
				}
				var envelope struct {
					ID     json.RawMessage `json:"id"`
					Result map[string]any  `json:"result"`
				}
				if err := json.Unmarshal(response, &envelope); err != nil {
					t.Fatal(err)
				}
				if string(envelope.ID) != "9007199254740993" {
					t.Fatal("JSON-RPC ID rounded")
				}
				if w.Header().Get("Content-Length") != "" {
					t.Fatal("stale upstream content length")
				}
				return envelope.Result, w.Code
			}
			args := map[string]any{"query": "all fixture rows", "top_k": 50}
			result, _ := request("kb-one", "local-token", args)
			var rows []any
			cursor := ""
			for pages := 0; ; pages++ {
				if pages > 100 {
					t.Fatal("cursor loop")
				}
				text := result["content"].([]any)[0].(map[string]any)["text"].(string)
				if len(text) > knowledgebase.QueryPageTextBytes {
					t.Fatal("oversized page")
				}
				page := result["structuredContent"].(map[string]any)
				rows = append(rows, page["evidence"].([]any)...)
				if page["query_coverage"].(map[string]any)["status"] != "complete" || page["coverage"].(map[string]any)["status"] != "partial" {
					t.Fatal("query and delivery coverage conflated")
				}
				delivery := page["delivery"].(map[string]any)
				if delivery["has_more"] == false {
					break
				}
				cursor = delivery["next_cursor"].(string)
				args[knowledgebase.QueryCursorArgument] = cursor
				result, _ = request("kb-one", "local-token", args)
			}
			if !reflect.DeepEqual(rows, fixture["structuredContent"].(map[string]any)["evidence"]) {
				t.Fatal("evidence records or citations lost")
			}
			args[knowledgebase.QueryCursorArgument] = cursor
			if _, status := request("kb-one", "wrong-local-token", args); status != 401 {
				t.Fatal("local authentication bypassed")
			}
			if result, _ := request("kb-two", "local-token", args); result["isError"] != true {
				t.Fatal("cursor leaked across knowledge bases")
			}
			connection.CSGHubAccessToken = "account-two"
			if result, _ := request("kb-one", "local-token", args); result["isError"] != true {
				t.Fatal("cursor leaked across accounts")
			}
			if calls != 1 {
				t.Fatalf("upstream searches=%d, want one snapshot", calls)
			}
		})
	}
}

func TestKnowledgeQueryProxySchemaErrorsAndUnrelatedMessages(t *testing.T) {
	h := &Handler{}
	rpc := knowledgeRPC{ID: json.RawMessage(`"list-1"`), Method: "tools/list"}
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "list-1", "result": map[string]any{"tools": []any{knowledgeQueryToolFixture(), map[string]any{"name": "other", "inputSchema": map[string]any{"type": "object"}}}}})
	var envelope map[string]any
	if err := json.Unmarshal(h.rewriteKnowledgeRPC(raw, rpc, "scope"), &envelope); err != nil {
		t.Fatal(err)
	}
	tools := envelope["result"].(map[string]any)["tools"].([]any)
	props := tools[0].(map[string]any)["inputSchema"].(map[string]any)["properties"].(map[string]any)
	if props[knowledgebase.QueryCursorArgument] == nil || props["filters"].(map[string]any)["type"] != "object" || !strings.Contains(props["top_k"].(map[string]any)["description"].(string), "maximum: 50") {
		t.Fatal("runtime-readable query schema missing")
	}
	if _, added := tools[1].(map[string]any)["description"]; added {
		t.Fatal("unrelated tool modified")
	}
	rpc.ID = json.RawMessage(`"wrong-id"`)
	if !bytes.Equal(h.rewriteKnowledgeRPC(raw, rpc, "scope"), raw) {
		t.Fatal("unrelated response rewritten")
	}
	for _, args := range []string{`{"filters":"{\"source_ids\":[\"one\"]}"}`, `{"filters":[]}`, `{"csgclaw_result_cursor":4}`} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/mcp", strings.NewReader(`{"id":1,"method":"tools/call","params":{"name":"query","arguments":`+args+`}}`))
		_, handled := h.prepareKnowledgeRPC(w, r, "scope")
		if !handled || !strings.Contains(w.Body.String(), `"isError":true`) {
			t.Fatal("invalid arguments sent to backend")
		}
	}
}

func TestKnowledgeQuerySSEPreservesMultilineProgressAndFlushesBeforeResult(t *testing.T) {
	h := &Handler{}
	rpc := knowledgeRPC{ID: json.RawMessage(`1`), Method: "tools/call"}
	rpc.Params.Name = "query"
	progress := ": keepalive\nevent: message\nid: progress-1\ndata: {\"jsonrpc\":\"2.0\",\n" + "data: \"method\":\"notifications/progress\",\"params\":{\"progress\":1}}\n\n"
	reader, writer := io.Pipe()
	defer reader.Close()
	output := &knowledgeSignalWriter{written: make(chan string, 8)}
	done := make(chan error, 1)
	go func() { done <- h.copyKnowledgeRPC(output, reader, "text/event-stream", rpc, "scope") }()
	go func() { _, _ = io.WriteString(writer, progress) }()
	select {
	case got := <-output.written:
		if got != progress {
			t.Fatalf("progress framing changed: %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("progress buffered until query completes")
	}
	_ = writer.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type knowledgeSignalWriter struct{ written chan string }

func (w *knowledgeSignalWriter) Write(p []byte) (int, error) {
	w.written <- string(p)
	return len(p), nil
}
