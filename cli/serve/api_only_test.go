package serve

import (
	"slices"
	"testing"
)

func TestAPIOnlyPropagatesToDaemonAndSuppressesBrowser(t *testing.T) {
	args := backgroundServeArgs("cfg.yaml", "pid", "info", serveOptions{APIOnly: true, OpenBrowser: true})
	if !slices.Contains(args, "--api-only") || slices.Contains(args, "--browser") {
		t.Fatalf("daemon arguments: %v", args)
	}
}
