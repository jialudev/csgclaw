package modelprovider

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestContextOverflowTypeNormalizesWithoutLeakingDiagnostics(t *testing.T) {
	for _, body := range []string{
		`{"error":{"code":400,"type":"exceed_context_size_error","message":"request (95587 tokens) exceeds the available context size (87552 tokens), private diagnostic"}}`,
		`{"error":{"type":"context_window_exceeded","message":"private diagnostic"}}`,
		`{"response":{"error":{"code":"context_length_exceeded","message":"private diagnostic"}}}`,
	} {
		status, code, message, ok := UserFacingUpstreamError(&ResponsesAPIStatusError{StatusCode: http.StatusBadRequest, Body: body})
		if !ok || status != 400 || code != "context_length_exceeded" || !strings.Contains(message, "context length") || strings.Contains(message, "private") {
			t.Fatalf("wrong overflow classification: %d %s %s", status, code, message)
		}
	}
	if code := UpstreamErrorCodeForResponse(400, []byte(`{"error":{"code":400,"type":"invalid_request_error","message":"context config is wrong"}}`)); code != "invalid_request_error" {
		t.Fatal("inferred overflow from unrelated text")
	}
	if _, _, _, ok := UserFacingUpstreamError(errors.New("local context error")); ok {
		t.Fatal("classified a local error as upstream")
	}
}
