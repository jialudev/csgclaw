package knowledgebase

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DescribeQueryTool keeps constraints understandable to runtimes whose JSON
// schema subset drops numeric bounds or does not expand local references.
// Only the managed knowledge-base query tool gets the local cursor argument.
func DescribeQueryTool(tool map[string]any) {
	if tool["name"] != "query" {
		return
	}
	schema, ok := tool["inputSchema"].(map[string]any)
	if !ok {
		return
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	if _, exists := props[QueryCursorArgument]; exists {
		return
	}
	for name, raw := range props {
		if property, ok := raw.(map[string]any); ok {
			props[name] = describeProperty(property, schema, 0)
		}
	}
	props[QueryCursorArgument] = map[string]any{"type": "string", "description": "Optional continuation cursor from delivery.next_cursor. Pass the original query and this cursor to read the next page of the SAME cached result, without repeating the search. Do not invent a cursor."}
	description, _ := tool["description"].(string)
	tool["description"] = description + " Large query results are paged by CSGClaw. If delivery.has_more is true, continue with csgclaw_result_cursor=delivery.next_cursor before making completeness claims. For delivery.mode=json_fragment, concatenate result_fragment values in offset order to recover the original JSON. coverage=complete only describes this query, not a full-table scan."
	// Page delivery is a union of the original result and a JSON-fragment page.
	// The raw upstream schema cannot validate that union; content remains valid MCP.
	delete(tool, "outputSchema")
}

func describeProperty(property, root map[string]any, depth int) map[string]any {
	if depth > 12 {
		return property
	}
	out := make(map[string]any, len(property)+1)
	resolved := false
	if ref, ok := property["$ref"].(string); ok && strings.HasPrefix(ref, "#/$defs/") {
		defs, _ := root["$defs"].(map[string]any)
		name := strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(ref, "#/$defs/"), "~1", "/"), "~0", "~")
		if target, ok := defs[name].(map[string]any); ok {
			resolved = true
			for k, v := range describeProperty(target, root, depth+1) {
				out[k] = v
			}
		}
	}
	for k, v := range property {
		if k != "$ref" || !resolved {
			out[k] = v
		}
	}
	var constraints []string
	if typ, ok := out["type"].(string); ok {
		constraints = append(constraints, "Type: "+typ+".")
		if typ == "object" {
			constraints = append(constraints, "Pass a JSON object, not a JSON-encoded string.")
		}
	}
	for _, key := range []string{"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "default", "minItems", "maxItems"} {
		if value, ok := out[key]; ok {
			raw, _ := json.Marshal(value)
			constraints = append(constraints, fmt.Sprintf("%s: %s.", key, raw))
		}
	}
	description, _ := out["description"].(string)
	out["description"] = strings.TrimSpace(description + " " + strings.Join(constraints, " "))
	if props, ok := out["properties"].(map[string]any); ok {
		copy := make(map[string]any, len(props))
		for k, v := range props {
			if child, ok := v.(map[string]any); ok {
				copy[k] = describeProperty(child, root, depth+1)
			} else {
				copy[k] = v
			}
		}
		out["properties"] = copy
	}
	return out
}
