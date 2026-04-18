package boxlitecli

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestExecRunnerCapturesOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result, err := execRunner{}.Run(context.Background(), CommandRequest{
		Path:   "/bin/sh",
		Args:   []string{"-c", "printf out; printf err >&2"},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if string(result.Stdout) != "out" || stdout.String() != "out" {
		t.Fatalf("stdout captured=%q forwarded=%q", string(result.Stdout), stdout.String())
	}
	if string(result.Stderr) != "err" || stderr.String() != "err" {
		t.Fatalf("stderr captured=%q forwarded=%q", string(result.Stderr), stderr.String())
	}
}

func TestExecRunnerPreservesNonZeroExitCode(t *testing.T) {
	result, err := execRunner{}.Run(context.Background(), CommandRequest{
		Path: "/bin/sh",
		Args: []string{"-c", "exit 7"},
	})
	if err == nil {
		t.Fatal("Run() error = nil, want exit error")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Run() error = %T, want *exec.ExitError", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", result.ExitCode)
	}
}
