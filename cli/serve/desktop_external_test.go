package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"csgclaw/cli/command"
	"csgclaw/internal/desktop"
	"csgclaw/internal/server"
)

func TestExternalDesktopSkipsLocalBootstrap(t *testing.T) {
	if !server.UIAvailable {
		t.Skip("requires the UI host build")
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/control/capabilities" || r.Header.Get("Authorization") != "Bearer backend-test-token" {
			t.Errorf("unexpected backend handshake")
		}
		json.NewEncoder(w).Encode(map[string]any{"backend_id": "existing", "control_api_version": 1})
	}))
	defer backend.Close()
	bootstrap := desktop.BootstrapMessage{Type: desktop.MessageTypeBootstrap, ProtocolVersion: desktop.ProtocolVersion, InstanceID: strings.Repeat("i", 16), SessionToken: strings.Repeat("s", 43), BackendAPIBase: backend.URL + "/api/v1/", BackendToken: "backend-test-token"}
	raw, _ := json.Marshal(bootstrap)
	shutdown, _ := json.Marshal(desktop.ControlMessage{Type: desktop.MessageTypeShutdown, Reason: "test"})
	var stdout, stderr bytes.Buffer
	run := &command.Context{Program: "csgclaw", Stdin: bytes.NewReader(append(append(append(raw, '\n'), shutdown...), '\n')), Stdout: &stdout, Stderr: &stderr}
	path := filepath.Join(t.TempDir(), "not-created", "config.toml")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := NewDesktopServeCmd().Run(ctx, run, []string{"--config", path}, command.GlobalOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("external connection initialized a local backend configuration")
	}
	var ready desktop.ReadyMessage
	if err := json.Unmarshal(stdout.Bytes(), &ready); err != nil || ready.Type != desktop.MessageTypeReady {
		t.Fatalf("missing ready protocol: %s %v", stdout.String(), err)
	}
	if strings.Contains(stdout.String(), bootstrap.BackendToken) || strings.Contains(stderr.String(), bootstrap.BackendToken) {
		t.Fatal("backend credential reached output")
	}
}
