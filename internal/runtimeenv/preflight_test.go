package runtimeenv

import (
	"context"
	"csgclaw/internal/apitypes"
	"errors"
	"testing"
)

func TestPreflightExplainsMissingAndOldDependencies(t *testing.T) {
	deps := []apitypes.RuntimeDependency{{Name: "node", MinVersion: "22.13.0"}, {Name: "python", Optional: true}}
	report := Inspect(context.Background(), deps, func(_ context.Context, name string) (string, error) {
		if name == "node" {
			return "20.0.0", nil
		}
		return "", errors.New("missing")
	})
	if report.Ready || report.Checks[0].State != "version_mismatch" || report.Checks[1].State != "missing" {
		t.Fatalf("report: %+v", report)
	}
	deps[0].MinVersion = "18.0.0"
	report = Inspect(context.Background(), deps, func(_ context.Context, name string) (string, error) {
		if name == "node" {
			return "20.0.0", nil
		}
		return "", errors.New("missing")
	})
	if !report.Ready {
		t.Fatal("optional dependency blocked readiness")
	}
}
func TestDependenciesCannotContainInstallationCommands(t *testing.T) {
	for _, deps := range [][]apitypes.RuntimeDependency{{{Name: "node; curl example.com"}}, {{Name: "python", MinVersion: ">=3;sh"}}, {{Name: "node"}, {Name: "node"}}} {
		if Validate(deps) == nil {
			t.Fatalf("accepted %+v", deps)
		}
	}
	for _, pair := range [][3]string{{"22.13.0", "22.13", "true"}, {"20.2.0", "22.0.0", "false"}, {"3.12.1", "3.11.0", "true"}} {
		if AtLeast(pair[0], pair[1]) != (pair[2] == "true") {
			t.Fatal(pair)
		}
	}
}
