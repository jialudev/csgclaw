package api

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"csgclaw/internal/knowledgebase"
)

const maxKnowledgeRPCBytes = 32 << 20

type knowledgeRPC struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"params"`
}

func queryDeliveryScope(connection knowledgeBaseConnection, contentID string) string {
	h := sha256.New()
	for _, s := range []string{connection.AIGatewayBaseURL, connection.CSGHubAccessToken, contentID} {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// prepareKnowledgeRPC only handles local continuation and obvious type errors.
// All ordinary searches retain the upstream request, including MCP metadata.
func (h *Handler) prepareKnowledgeRPC(w http.ResponseWriter, r *http.Request, scope string) (knowledgeRPC, bool) {
	var rpc knowledgeRPC
	if r.Method != http.MethodPost || r.Body == nil {
		return rpc, false
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxKnowledgeRPCBytes+1))
	if err != nil || len(body) > maxKnowledgeRPCBytes {
		http.Error(w, "knowledge base request is too large or unreadable", http.StatusRequestEntityTooLarge)
		return rpc, true
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if json.Unmarshal(body, &rpc) != nil || rpc.Method != "tools/call" || rpc.Params.Name != "query" {
		return rpc, false
	}
	respond := func(result any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": rpc.ID, "result": result})
	}
	if raw, ok := rpc.Params.Arguments[knowledgebase.QueryCursorArgument]; ok {
		cursor, ok := raw.(string)
		if !ok || cursor == "" {
			respond(knowledgebase.QueryToolError("csgclaw_result_cursor must be a non-empty string from delivery.next_cursor."))
		} else {
			respond(h.knowledgeQueryDelivery.Continue(scope, cursor))
		}
		return rpc, true
	}
	if raw, ok := rpc.Params.Arguments["filters"]; ok && raw != nil {
		if _, ok := raw.(map[string]any); !ok {
			respond(knowledgebase.QueryToolError("Invalid query argument filters: expected a JSON object, not a string or array. For example: {\"source_ids\":[\"the source ID returned by the knowledge base\"]}."))
			return rpc, true
		}
	}
	return rpc, false
}

func (h *Handler) rewriteKnowledgeRPC(raw []byte, rpc knowledgeRPC, scope string) []byte {
	var envelope map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if !json.Valid(raw) || decoder.Decode(&envelope) != nil {
		return raw
	}
	// Never rewrite notifications or a response belonging to another request.
	id, _ := json.Marshal(envelope["id"])
	var requestID any
	idDecoder := json.NewDecoder(bytes.NewReader(rpc.ID))
	idDecoder.UseNumber()
	if len(rpc.ID) == 0 || idDecoder.Decode(&requestID) != nil {
		return raw
	}
	wanted, _ := json.Marshal(requestID)
	if !bytes.Equal(id, wanted) {
		return raw
	}
	result, ok := envelope["result"].(map[string]any)
	if !ok || envelope["id"] == nil {
		return raw
	}
	switch rpc.Method {
	case "tools/list":
		tools, _ := result["tools"].([]any)
		for _, rawTool := range tools {
			if tool, ok := rawTool.(map[string]any); ok {
				knowledgebase.DescribeQueryTool(tool)
			}
		}
	case "tools/call":
		if rpc.Params.Name != "query" {
			return raw
		}
		envelope["result"] = h.knowledgeQueryDelivery.Present(scope, result)
	default:
		return raw
	}
	out, err := json.Marshal(envelope)
	if err != nil {
		return raw
	}
	return out
}

// copyKnowledgeRPC preserves SSE framing and forwards progress immediately.
// Only complete JSON-RPC data events are rewritten; no raw byte truncation.
func (h *Handler) copyKnowledgeRPC(w io.Writer, body io.Reader, contentType string, rpc knowledgeRPC, scope string) error {
	if rpc.Method != "tools/list" && !(rpc.Method == "tools/call" && rpc.Params.Name == "query") {
		_, err := io.Copy(w, body)
		return err
	}
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		scanner := bufio.NewScanner(body)
		scanner.Buffer(make([]byte, 4096), maxKnowledgeRPCBytes)
		var lines []string
		size := 0
		flush := func() error {
			var data []string
			for _, line := range lines {
				if strings.HasPrefix(line, "data:") {
					data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
				}
			}
			if len(data) == 0 {
				_, err := io.WriteString(w, strings.Join(lines, "\n")+"\n\n")
				return err
			}
			original := []byte(strings.Join(data, "\n"))
			rewritten := h.rewriteKnowledgeRPC(original, rpc, scope)
			if bytes.Equal(original, rewritten) {
				_, err := io.WriteString(w, strings.Join(lines, "\n")+"\n\n")
				return err
			}
			written := false
			for _, line := range lines {
				if strings.HasPrefix(line, "data:") {
					if written {
						continue
					}
					line = "data: " + string(rewritten)
					written = true
				}
				if _, err := io.WriteString(w, line+"\n"); err != nil {
					return err
				}
			}
			_, err := io.WriteString(w, "\n")
			return err
		}
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if err := flush(); err != nil {
					return err
				}
				lines = nil
				size = 0
				continue
			}
			size += len(line)
			if size > maxKnowledgeRPCBytes {
				return io.ErrShortBuffer
			}
			lines = append(lines, line)
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		if len(lines) > 0 {
			return flush()
		}
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(body, maxKnowledgeRPCBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > maxKnowledgeRPCBytes {
		return io.ErrShortBuffer
	}
	_, err = w.Write(h.rewriteKnowledgeRPC(raw, rpc, scope))
	return err
}
