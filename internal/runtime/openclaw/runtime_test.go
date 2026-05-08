package openclaw

import (
	"net"
	"strconv"
	"testing"
)

func TestNextGatewayPortSkipsBusyPort(t *testing.T) {
	busy, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(DefaultGatewayPort)))
	if err != nil {
		t.Skipf("cannot reserve default OpenClaw port in this environment: %v", err)
	}
	defer busy.Close()

	rt := New(Dependencies{})
	got, err := rt.nextGatewayPort(DefaultGatewayPort)
	if err != nil {
		t.Fatalf("nextGatewayPort() error = %v", err)
	}
	if got != DefaultGatewayPort+1 {
		t.Fatalf("nextGatewayPort() = %d, want %d", got, DefaultGatewayPort+1)
	}
}

func TestNodeVersionAtLeast(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{version: "22.14.0", want: true},
		{version: "22.13.9", want: false},
		{version: "23.0.0", want: true},
		{version: "bad", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := nodeVersionAtLeast(tt.version, 22, 14); got != tt.want {
				t.Fatalf("nodeVersionAtLeast(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}
