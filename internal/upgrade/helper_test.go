package upgrade

import (
	"os/exec"
	"reflect"
	"testing"
)

func TestStartApplyHelperIncludesConfigPath(t *testing.T) {
	origExecutable := startHelperExecutable
	origCommand := startHelperCommand
	t.Cleanup(func() {
		startHelperExecutable = origExecutable
		startHelperCommand = origCommand
	})

	startHelperExecutable = func() (string, error) {
		return "/tmp/csgclaw", nil
	}

	var gotName string
	var gotArgs []string
	startHelperCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return exec.Command("sh", "-c", "exit 0")
	}

	if err := StartApplyHelper(ApplyHelperOptions{ConfigPath: "/tmp/csgclaw.toml"}); err != nil {
		t.Fatalf("StartApplyHelper() error = %v", err)
	}

	if gotName != "/tmp/csgclaw" {
		t.Fatalf("command name = %q, want %q", gotName, "/tmp/csgclaw")
	}
	wantArgs := []string{"upgrade", "--config", "/tmp/csgclaw.toml"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("command args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestStartApplyHelperOmitsEmptyConfigPath(t *testing.T) {
	origExecutable := startHelperExecutable
	origCommand := startHelperCommand
	t.Cleanup(func() {
		startHelperExecutable = origExecutable
		startHelperCommand = origCommand
	})

	startHelperExecutable = func() (string, error) {
		return "/tmp/csgclaw", nil
	}

	var gotArgs []string
	startHelperCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		return exec.Command("sh", "-c", "exit 0")
	}

	if err := StartApplyHelper(ApplyHelperOptions{}); err != nil {
		t.Fatalf("StartApplyHelper() error = %v", err)
	}

	wantArgs := []string{"upgrade"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("command args = %#v, want %#v", gotArgs, wantArgs)
	}
}
