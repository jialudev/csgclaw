package task

import (
	"context"
	"csgclaw/cli/command"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestScheduleCLIRequiresConfirmationAndOnlyParsePreviews(t *testing.T) {
	calls := 0
	run := &command.Context{Program: "test", Stdout: io.Discard, Stderr: io.Discard, HTTPClient: testTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/api/v1/scheduled-tasks/parse" {
			t.Fatalf("preview wrote schedule: %s", r.URL.Path)
		}
		return testJSONResponse(`{"clarification":"When?"}`), nil
	})}
	options := command.GlobalOptions{Endpoint: "http://example.test"}
	if err := NewCmd().Run(context.Background(), run, []string{"schedule", "parse", "--text", "tomorrow at 9 summarize", "--timezone", "Asia/Shanghai"}, options); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "preview.json")
	if err := os.WriteFile(file, []byte(`{"client_request_id":"test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := NewCmd().Run(context.Background(), run, []string{"schedule", "create", "--file", file}, options); err == nil {
		t.Fatal("saved without confirmation")
	}
	if calls != 1 {
		t.Fatal("confirmation failure still made request")
	}
}
