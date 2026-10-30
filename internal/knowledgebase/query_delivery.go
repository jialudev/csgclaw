package knowledgebase

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Leave room for the runtime's wrapper within its 10 KB tool-output budget.
// Raising that budget alone makes repeated queries overflow model context.
const QueryPageTextBytes = 8000
const QueryCursorArgument = "csgclaw_result_cursor"

const (
	queryCacheBytes   = 32 << 20
	queryCacheEntries = 64
	queryCacheTTL     = 15 * time.Minute
)

type queryPages struct {
	scope    string
	expires  time.Time
	pages    []string
	envelope []byte
	bytes    int
}

// QueryDelivery holds immutable pages of an already executed query. Cursors
// never execute another search, so evidence cannot drift between pages.
// The zero value is ready to use. Callers bind scope to the KB and credentials.
type QueryDelivery struct {
	mu      sync.Mutex
	entries map[string]queryPages
	bytes   int
	now     func() time.Time
}

func (d *QueryDelivery) clock() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now()
}

func QueryToolError(message string) map[string]any {
	return map[string]any{"isError": true, "content": []any{map[string]any{"type": "text", "text": message}}}
}

// Continue returns only a cached page authorized for this exact scope.
func (d *QueryDelivery) Continue(scope, cursor string) map[string]any {
	id, index, ok := strings.Cut(cursor, ".")
	n, err := strconv.Atoi(index)
	if !ok || err != nil || n < 1 {
		return QueryToolError("Invalid query result cursor. Run the original query again.")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.expire(d.clock())
	entry, ok := d.entries[id]
	if !ok || entry.scope != scope || n >= len(entry.pages) {
		return QueryToolError("Query result cursor expired or is not available for this knowledge base and account. Run the original query again; do not treat missing pages as missing evidence.")
	}
	return queryPageResult(entry.pages[n], entry.envelope)
}

func (d *QueryDelivery) expire(now time.Time) {
	for id, entry := range d.entries {
		if !now.Before(entry.expires) {
			d.bytes -= entry.bytes
			delete(d.entries, id)
		}
	}
}

// Present only adapts the recognized KB query contract. Other MCP tools,
// errors, images and unknown result formats are passed through unchanged.
func (d *QueryDelivery) Present(scope string, result map[string]any) map[string]any {
	if result["isError"] == true {
		return result
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) != 1 {
		return result
	}
	text, ok := content[0].(map[string]any)
	if !ok || text["type"] != "text" {
		return result
	}
	var payload map[string]any
	s, ok := text["text"].(string)
	decoder := json.NewDecoder(strings.NewReader(s))
	decoder.UseNumber()
	if !ok || !json.Valid([]byte(s)) || decoder.Decode(&payload) != nil || payload["schema_version"] != "kb-query-result.v2" {
		return result
	}
	// A conflicting structured payload is not safe to reinterpret.
	if structured, ok := result["structuredContent"]; ok && structured != nil {
		a, _ := json.Marshal(structured)
		b, _ := json.Marshal(payload)
		if !bytes.Equal(a, b) {
			return result
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return result
	}
	// Preserve MCP annotations and metadata, without retaining the full text
	// twice in the cache. Normalizing even a small result avoids pretty-printed
	// or doubly escaped text exceeding the runtime's output limit.
	envelope := make(map[string]any, len(result))
	for k, v := range result {
		if k != "content" && k != "structuredContent" {
			envelope[k] = v
		}
	}
	textMetadata := make(map[string]any, len(text))
	for k, v := range text {
		if k != "text" {
			textMetadata[k] = v
		}
	}
	envelope["content"] = []any{textMetadata}
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return result
	}
	if len(raw) <= QueryPageTextBytes {
		return queryPageResult(string(raw), envelopeJSON)
	}
	if len(raw) > queryCacheBytes {
		return QueryToolError("Query result is too large to deliver safely. Narrow the query or reduce top_k; no complete evidence was delivered.")
	}
	id := rand.Text()
	split := splitQueryPages(payload, id)
	pages := make([]string, len(split))
	size := len(envelopeJSON)
	for i, page := range split {
		b, _ := json.Marshal(page)
		pages[i] = string(b)
		size += len(b)
	}
	if size > queryCacheBytes {
		return QueryToolError("Query result is too large to deliver safely. Narrow the query or reduce top_k; no complete evidence was delivered.")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.clock()
	d.expire(now)
	if d.entries == nil {
		d.entries = make(map[string]queryPages)
	}
	for len(d.entries) >= queryCacheEntries || d.bytes+size > queryCacheBytes {
		var oldest string
		var expiry time.Time
		for key, entry := range d.entries {
			if oldest == "" || entry.expires.Before(expiry) {
				oldest, expiry = key, entry.expires
			}
		}
		d.bytes -= d.entries[oldest].bytes
		delete(d.entries, oldest)
	}
	d.entries[id] = queryPages{scope: scope, expires: now.Add(queryCacheTTL), pages: pages, envelope: envelopeJSON, bytes: size}
	d.bytes += size
	return queryPageResult(pages[0], envelopeJSON)
}

func queryPageResult(page string, envelope []byte) map[string]any {
	var result, structured map[string]any
	decoder := json.NewDecoder(bytes.NewReader(envelope))
	decoder.UseNumber()
	_ = decoder.Decode(&result)
	decoder = json.NewDecoder(strings.NewReader(page))
	decoder.UseNumber()
	_ = decoder.Decode(&structured)
	result["content"].([]any)[0].(map[string]any)["text"] = page
	result["structuredContent"] = structured
	return result
}

func deliveryInfo(id string, page, offset, total int, mode string, more bool) map[string]any {
	next := ""
	if more {
		next = id + "." + strconv.Itoa(page+1)
	}
	return map[string]any{
		"mode": mode, "page": page + 1, "offset": offset, "total": total, "has_more": more, "next_cursor": next,
		"notice": "This is a page of one query result, not a complete-table scan. To read remaining evidence call query with the original query and csgclaw_result_cursor=next_cursor. Do not conclude all matches until the required pages are read. Cursors expire after 15 minutes.",
	}
}

func splitQueryPages(payload map[string]any, id string) []map[string]any {
	evidence, ok := payload["evidence"].([]any)
	if !ok || len(evidence) == 0 {
		return fragmentQuery(payload, id)
	}
	var pages []map[string]any
	for offset := 0; offset < len(evidence); {
		base := make(map[string]any, len(payload)+2)
		for k, v := range payload {
			base[k] = v
		}
		// Keep backend coverage separate from delivery completeness.
		base["query_coverage"] = payload["coverage"]
		base["coverage"] = map[string]any{"status": "partial", "gaps": []any{map[string]any{"code": "RESULT_PAGED", "message": "Read remaining query result pages; backend coverage is in query_coverage."}}}
		start := offset
		for offset < len(evidence) {
			base["evidence"] = evidence[start : offset+1]
			base["delivery"] = deliveryInfo(id, len(pages), start, len(evidence), "evidence", offset+1 < len(evidence))
			b, _ := json.Marshal(base)
			if len(b) > QueryPageTextBytes {
				break
			}
			offset++
		}
		// Preserve oversized records/metadata losslessly as UTF-8 JSON fragments.
		if offset == start {
			return fragmentQuery(payload, id)
		}
		base["evidence"] = evidence[start:offset]
		base["delivery"] = deliveryInfo(id, len(pages), start, len(evidence), "evidence", offset < len(evidence))
		pages = append(pages, base)
	}
	return pages
}

func fragmentQuery(payload map[string]any, id string) []map[string]any {
	raw, _ := json.Marshal(payload)
	var pages []map[string]any
	for offset := 0; offset < len(raw); {
		makePage := func(end int) map[string]any {
			return map[string]any{"delivery": deliveryInfo(id, len(pages), offset, len(raw), "json_fragment", end < len(raw)), "result_fragment": string(raw[offset:end])}
		}
		// JSON escaping counts toward the actual text budget; never split UTF-8.
		lo, hi := offset+1, min(len(raw), offset+QueryPageTextBytes)
		for lo <= hi {
			mid := (lo + hi) / 2
			b, _ := json.Marshal(makePage(mid))
			if len(b) <= QueryPageTextBytes {
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
		end := hi
		for end > offset && end < len(raw) && !utf8.RuneStart(raw[end]) {
			end--
		}
		pages = append(pages, makePage(end))
		offset = end
	}
	return pages
}
