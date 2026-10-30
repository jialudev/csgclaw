package knowledgebase

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

func fixtureQueryResult(count int) map[string]any {
	evidence := make([]any, count)
	for i := range evidence {
		evidence[i] = map[string]any{"page_id": fmt.Sprintf("page-%d", i), "title": fmt.Sprintf("厂房-%d", i), "snippet": strings.Repeat("中文证据不得从中间省略；", 35) + fmt.Sprintf("面积=%d；层高=11.5米", 3500+i), "source_files": []any{map[string]any{"source_id": "source-original", "display_name": "房源表.xlsx"}}}
	}
	payload := map[string]any{"schema_version": "kb-query-result.v2", "kb_id": "kb-fixture", "evidence": evidence, "coverage": map[string]any{"status": "complete", "gaps": []any{}}, "truncated": false}
	raw, _ := json.Marshal(payload)
	return map[string]any{"content": []any{map[string]any{"type": "text", "text": string(raw)}}, "structuredContent": payload}
}

func collectQueryPages(t *testing.T, d *QueryDelivery, result map[string]any) ([]any, string, int) {
	t.Helper()
	var evidence []any
	var fragments strings.Builder
	count := 0
	for {
		if result["isError"] == true {
			t.Fatalf("page error: %v", result)
		}
		page := result["structuredContent"].(map[string]any)
		text := result["content"].([]any)[0].(map[string]any)["text"].(string)
		if len(text) > QueryPageTextBytes || !utf8.ValidString(text) {
			t.Fatalf("unsafe page: bytes=%d", len(text))
		}
		encoded, _ := json.Marshal(page)
		if string(encoded) != text {
			t.Fatal("text and structured evidence diverged")
		}
		if rows, ok := page["evidence"].([]any); ok {
			evidence = append(evidence, rows...)
		}
		if part, ok := page["result_fragment"].(string); ok {
			fragments.WriteString(part)
		}
		count++
		if count > 500 {
			t.Fatal("cursor loop")
		}
		info := page["delivery"].(map[string]any)
		if info["has_more"] == false {
			if info["next_cursor"] != "" {
				t.Fatal("terminal page has a cursor")
			}
			break
		}
		result = d.Continue("account-and-kb", info["next_cursor"].(string))
	}
	return evidence, fragments.String(), count
}

func TestQueryDeliveryPreservesEveryRecordAndCitation(t *testing.T) {
	var d QueryDelivery
	input := fixtureQueryResult(50)
	original, _ := json.Marshal(input)
	rows, fragment, pages := collectQueryPages(t, &d, d.Present("account-and-kb", input))
	want := input["structuredContent"].(map[string]any)["evidence"]
	if !reflect.DeepEqual(rows, want) || fragment != "" || pages < 2 {
		t.Fatalf("lost/reordered evidence: %d rows, %d pages", len(rows), pages)
	}
	after, _ := json.Marshal(input)
	if string(original) != string(after) {
		t.Fatal("mutated upstream evidence")
	}
}

func TestQueryDeliveryLargeRecordAndMetadataAreLossless(t *testing.T) {
	for _, field := range []string{"evidence", "metadata"} {
		t.Run(field, func(t *testing.T) {
			input := fixtureQueryResult(1)
			payload := input["structuredContent"].(map[string]any)
			long := strings.Repeat("中文\"引号\\\n😀", 4000)
			if field == "evidence" {
				payload["evidence"].([]any)[0].(map[string]any)["snippet"] = long
			} else {
				payload["metadata"] = long
			}
			raw, _ := json.Marshal(payload)
			input["content"].([]any)[0].(map[string]any)["text"] = string(raw)
			var d QueryDelivery
			_, fragment, pages := collectQueryPages(t, &d, d.Present("account-and-kb", input))
			if fragment != string(raw) || pages < 2 {
				t.Fatal("JSON fragments did not recover the exact result")
			}
		})
	}
}

func TestQueryDeliveryKeepsNumbersMetadataAndCacheImmutable(t *testing.T) {
	input := fixtureQueryResult(20)
	payload := input["structuredContent"].(map[string]any)
	payload["precise_source_id"] = json.Number("9007199254740993")
	input["_meta"] = map[string]any{"request_id": "source-request"}
	text := input["content"].([]any)[0].(map[string]any)
	text["annotations"] = map[string]any{"audience": []any{"assistant"}}
	raw, _ := json.Marshal(payload)
	text["text"] = string(raw)
	var d QueryDelivery
	first := d.Present("scope", input)
	cursor := first["structuredContent"].(map[string]any)["delivery"].(map[string]any)["next_cursor"].(string)
	page := d.Continue("scope", cursor)
	if !reflect.DeepEqual(page["_meta"], input["_meta"]) || !reflect.DeepEqual(page["content"].([]any)[0].(map[string]any)["annotations"], text["annotations"]) {
		t.Fatal("MCP metadata or annotations lost")
	}
	structured := page["structuredContent"].(map[string]any)
	if structured["precise_source_id"] != json.Number("9007199254740993") {
		t.Fatal("source identifier rounded")
	}
	structured["evidence"] = nil
	page["_meta"].(map[string]any)["request_id"] = "mutated"
	again := d.Continue("scope", cursor)
	if again["structuredContent"].(map[string]any)["evidence"] == nil || again["_meta"].(map[string]any)["request_id"] != "source-request" {
		t.Fatal("caller mutated cached evidence")
	}
}

func TestQueryDeliveryCompactsSmallPrettyPrintedText(t *testing.T) {
	input := fixtureQueryResult(1)
	payload := input["structuredContent"].(map[string]any)
	raw, _ := json.MarshalIndent(payload, "", strings.Repeat(" ", 2000))
	delete(input, "structuredContent")
	input["content"].([]any)[0].(map[string]any)["text"] = string(raw)
	if len(raw) <= QueryPageTextBytes {
		t.Fatal("fixture must exceed tool output limit")
	}
	var d QueryDelivery
	result := d.Present("scope", input)
	if len(result["content"].([]any)[0].(map[string]any)["text"].(string)) > QueryPageTextBytes || !reflect.DeepEqual(result["structuredContent"], payload) {
		t.Fatal("small payload still at risk of runtime truncation")
	}
}

func TestQueryDeliveryAcceptsNullStructuredContent(t *testing.T) {
	input := fixtureQueryResult(20)
	want := input["structuredContent"].(map[string]any)["evidence"]
	input["structuredContent"] = nil
	var d QueryDelivery
	rows, _, _ := collectQueryPages(t, &d, d.Present("account-and-kb", input))
	if !reflect.DeepEqual(rows, want) {
		t.Fatal("nullable structuredContent prevented text paging")
	}
}

func TestQueryDeliveryScopeExpiryAndEviction(t *testing.T) {
	now := time.Now()
	d := QueryDelivery{now: func() time.Time { return now }}
	first := d.Present("account-and-kb", fixtureQueryResult(20))
	cursor := first["structuredContent"].(map[string]any)["delivery"].(map[string]any)["next_cursor"].(string)
	if d.Continue("another-account-or-kb", cursor)["isError"] != true {
		t.Fatal("cursor crossed credentials/KB scope")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if d.Continue("account-and-kb", cursor)["isError"] == true {
				t.Error("concurrent continuation failed")
			}
		})
	}
	wg.Wait()
	now = now.Add(queryCacheTTL)
	if d.Continue("account-and-kb", cursor)["isError"] != true || d.bytes != 0 {
		t.Fatal("expired evidence retained")
	}
	for i := 0; i < queryCacheEntries+1; i++ {
		d.Present("account-and-kb", fixtureQueryResult(10))
		now = now.Add(time.Second)
	}
	if len(d.entries) > queryCacheEntries || d.bytes > queryCacheBytes {
		t.Fatal("unbounded query cache")
	}
}

func TestQueryDeliveryPassesThroughUnknownSmallAndConflictingResults(t *testing.T) {
	for _, input := range []map[string]any{fixtureQueryResult(1), {"isError": true, "content": []any{map[string]any{"type": "text", "text": "failed"}}}, {"content": []any{map[string]any{"type": "image", "data": "fixture"}}}} {
		var d QueryDelivery
		if !reflect.DeepEqual(d.Present("scope", input), input) || len(d.entries) != 0 {
			t.Fatal("unexpected result adaptation")
		}
	}
	input := fixtureQueryResult(20)
	input["structuredContent"] = map[string]any{"different": "payload"}
	var d QueryDelivery
	if !reflect.DeepEqual(d.Present("scope", input), input) {
		t.Fatal("rewrote conflicting structured result")
	}
}

func TestQuerySchemaPreservesBoundsAndObjectTypeInDescriptions(t *testing.T) {
	tool := map[string]any{"name": "query", "inputSchema": map[string]any{"type": "object", "required": []any{"query"}, "$defs": map[string]any{"Filters": map[string]any{"type": "object", "properties": map[string]any{"source_ids": map[string]any{"type": "array"}}}}, "properties": map[string]any{"query": map[string]any{"type": "string"}, "top_k": map[string]any{"type": "integer", "maximum": 50, "minimum": 1, "default": 8}, "filters": map[string]any{"$ref": "#/$defs/Filters"}}}}
	DescribeQueryTool(tool)
	schema := tool["inputSchema"].(map[string]any)
	props := schema["properties"].(map[string]any)
	desc := props["top_k"].(map[string]any)["description"].(string)
	for _, s := range []string{"integer", "maximum: 50", "minimum: 1", "default: 8"} {
		if !strings.Contains(desc, s) {
			t.Fatalf("constraint lost: %s", s)
		}
	}
	filter := props["filters"].(map[string]any)
	if filter["type"] != "object" || filter["properties"] == nil || !strings.Contains(filter["description"].(string), "not a JSON-encoded string") {
		t.Fatal("object contract lost")
	}
	if props[QueryCursorArgument] == nil || !reflect.DeepEqual(schema["required"], []any{"query"}) {
		t.Fatal("cursor or original required arguments changed")
	}
}

func TestQuerySchemaKeepsUnresolvedReferences(t *testing.T) {
	input := map[string]any{"$ref": "#/$defs/Unknown", "description": "keep the referenced schema", "default": nil}
	for range 30 {
		if got := describeProperty(input, map[string]any{}, 0); got["$ref"] != input["$ref"] {
			t.Fatal("unresolved reference removed")
		}
	}
}
