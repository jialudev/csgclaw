package boxlitecli

import (
	"os"
	"testing"
	"time"

	"csgclaw/internal/sandbox"
)

func TestParseInspectFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/detached_inspect_running.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	info, err := parseInspect(data)
	if err != nil {
		t.Fatalf("parseInspect() error = %v", err)
	}
	if info.ID != "Hc4ubUMzVQRS" || info.Name != "csgclaw-spike" || info.State != sandbox.StateRunning {
		t.Fatalf("parseInspect() = %#v", info)
	}
	want, err := time.Parse(time.RFC3339Nano, "2026-04-18T07:31:25.471080+00:00")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !info.CreatedAt.Equal(want) {
		t.Fatalf("CreatedAt = %v, want %v", info.CreatedAt, want)
	}
}

func TestParseInspectEmptyMapsNotFound(t *testing.T) {
	_, err := parseInspect([]byte("[]"))
	if !sandbox.IsNotFound(err) {
		t.Fatalf("parseInspect([]) error = %v, want not found", err)
	}
}

func TestStateMapping(t *testing.T) {
	tests := []struct {
		status string
		want   sandbox.State
	}{
		{status: "configured", want: sandbox.StateCreated},
		{status: "created", want: sandbox.StateCreated},
		{status: "running", want: sandbox.StateRunning},
		{status: "stopping", want: sandbox.StateUnknown},
		{status: "stopped", want: sandbox.StateStopped},
		{status: "exited", want: sandbox.StateExited},
		{status: "", want: sandbox.StateUnknown},
		{status: "other", want: sandbox.StateUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := mapState(tt.status); got != tt.want {
				t.Fatalf("mapState(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
