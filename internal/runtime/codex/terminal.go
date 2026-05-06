package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	acp "github.com/coder/acp-go-sdk"
)

const defaultTerminalOutputLimit = 64 * 1024

type managedTerminal struct {
	cmd       *exec.Cmd
	done      chan struct{}
	limit     int
	mu        sync.Mutex
	output    []byte
	truncated bool
	exitCode  *int
	signal    *string
}

func newManagedTerminal(limit int) *managedTerminal {
	if limit <= 0 {
		limit = defaultTerminalOutputLimit
	}
	return &managedTerminal{
		done:  make(chan struct{}),
		limit: limit,
	}
}

func (t *managedTerminal) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.output = append(t.output, p...)
	if len(t.output) <= t.limit {
		return len(p), nil
	}

	t.truncated = true
	t.output = trimToUTF8Suffix(t.output, t.limit)
	return len(p), nil
}

func (t *managedTerminal) snapshot() (string, bool, *acp.TerminalExitStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var status *acp.TerminalExitStatus
	if t.exitCode != nil || t.signal != nil {
		status = &acp.TerminalExitStatus{
			ExitCode: t.exitCode,
			Signal:   t.signal,
		}
	}
	return string(t.output), t.truncated, status
}

func (t *managedTerminal) wait() (*int, *string) {
	<-t.done
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exitCode, t.signal
}

func (t *managedTerminal) setExit(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch {
	case err == nil:
		code := 0
		t.exitCode = &code
	case errors.Is(err, exec.ErrWaitDelay):
		sig := "SIGKILL"
		t.signal = &sig
	default:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code := exitErr.ExitCode()
			t.exitCode = &code
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				sig := status.Signal().String()
				t.signal = &sig
				t.exitCode = nil
			}
		} else {
			code := 1
			t.exitCode = &code
		}
	}
	close(t.done)
}

func trimToUTF8Suffix(data []byte, limit int) []byte {
	if limit <= 0 {
		return nil
	}
	if len(data) <= limit {
		return append([]byte(nil), data...)
	}
	data = data[len(data)-limit:]
	for len(data) > 0 && !utf8.Valid(data) {
		data = data[1:]
	}
	return append([]byte(nil), data...)
}

func buildTerminalEnv(base []string, vars []acp.EnvVariable) []string {
	envMap := make(map[string]string, len(base)+len(vars))
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		envMap[key] = value
	}
	for _, variable := range vars {
		name := strings.TrimSpace(variable.Name)
		if name == "" {
			continue
		}
		envMap[name] = variable.Value
	}
	env := make([]string, 0, len(envMap))
	for key, value := range envMap {
		env = append(env, key+"="+value)
	}
	return env
}

func resolveTerminalCWD(workspaceDir string, roots []string, cwd *string) (string, error) {
	if cwd == nil || strings.TrimSpace(*cwd) == "" {
		return workspaceDir, nil
	}
	dir := filepath.Clean(strings.TrimSpace(*cwd))
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("terminal cwd must be absolute: %s", dir)
	}
	if !pathAllowed(dir, roots...) {
		return "", fmt.Errorf("terminal cwd is outside allowed roots: %s", dir)
	}
	return dir, nil
}

func pathAllowed(path string, roots ...string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(path) {
		return false
	}
	for _, root := range roots {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "" || !filepath.IsAbs(root) {
			continue
		}
		if path == root {
			return true
		}
		if rel, err := filepath.Rel(root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func killManagedProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Signal(os.Interrupt)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func waitTerminal(ctx context.Context, term *managedTerminal) (acp.WaitForTerminalExitResponse, error) {
	select {
	case <-term.done:
		exitCode, signal := term.wait()
		return acp.WaitForTerminalExitResponse{ExitCode: exitCode, Signal: signal}, nil
	case <-ctx.Done():
		return acp.WaitForTerminalExitResponse{}, ctx.Err()
	}
}
