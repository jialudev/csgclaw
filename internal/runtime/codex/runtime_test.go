package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"csgclaw/internal/codexacp"
	agentruntime "csgclaw/internal/runtime"
	"csgclaw/internal/sandbox"

	acp "github.com/coder/acp-go-sdk"
)

type fakeBinaryProvider struct {
	path string
	err  error
}

func (f fakeBinaryProvider) Ensure(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.path, nil
}

type fakeManager struct {
	start  func(context.Context, SessionSpec) (*Session, error)
	stop   func(context.Context, SessionHandle) error
	get    func(SessionHandle) (*Session, error)
	prompt func(context.Context, SessionHandle, acp.PromptRequest) (acp.PromptResponse, error)
}

func (f fakeManager) Start(ctx context.Context, spec SessionSpec) (*Session, error) {
	return f.start(ctx, spec)
}

func (f fakeManager) Stop(ctx context.Context, handle SessionHandle) error {
	if f.stop != nil {
		return f.stop(ctx, handle)
	}
	return nil
}

func (f fakeManager) Session(handle SessionHandle) (*Session, error) {
	if f.get != nil {
		return f.get(handle)
	}
	return nil, os.ErrNotExist
}

func (f fakeManager) Prompt(ctx context.Context, handle SessionHandle, req acp.PromptRequest) (acp.PromptResponse, error) {
	if f.prompt != nil {
		return f.prompt(ctx, handle, req)
	}
	return acp.PromptResponse{}, os.ErrNotExist
}

func TestRuntimeCreateStartAndInfo(t *testing.T) {
	root := t.TempDir()
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	if err := os.MkdirAll(filepath.Join(hostHome, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostHome, ".codex", "auth.json"), []byte(`{"tokens":{"access_token":"access","refresh_token":"refresh"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	rt := New(Dependencies{
		BinaryProvider: fakeBinaryProvider{path: "/tmp/codex-acp"},
		AgentHome: func(agentName string) (string, error) {
			return filepath.Join(root, agentName), nil
		},
		ResolveAgent: func(h agentruntime.Handle) (AgentRef, error) {
			return AgentRef{
				ID:        "u-alice",
				Name:      "alice",
				RuntimeID: h.RuntimeID,
				Profile:   agentruntime.Profile{ModelID: "gpt-5.5"},
			}, nil
		},
		Manager: fakeManager{
			start: func(_ context.Context, spec SessionSpec) (*Session, error) {
				if spec.WorkspaceDir == "" || spec.HomeDir == "" || spec.CodexHomeDir == "" {
					t.Fatalf("expected runtime directories to be populated")
				}
				if want := filepath.Join(root, "alice", ".codex"); spec.CodexHomeDir != want {
					t.Fatalf("CodexHomeDir = %q, want %q", spec.CodexHomeDir, want)
				}
				return &Session{
					RuntimeID:    spec.RuntimeID,
					AgentID:      spec.AgentID,
					AgentName:    spec.AgentName,
					SessionID:    "sess-1",
					BinaryPath:   spec.BinaryPath,
					WorkspaceDir: spec.WorkspaceDir,
					HomeDir:      spec.HomeDir,
					CodexHomeDir: spec.CodexHomeDir,
					StderrPath:   spec.StderrPath,
					ProcessID:    os.Getpid(),
					CreatedAt:    time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC),
					StartedAt:    time.Date(2026, 5, 5, 8, 0, 1, 0, time.UTC),
				}, nil
			},
		},
	})

	handle, err := rt.Create(context.Background(), agentruntime.Spec{
		RuntimeID: "rt-u-alice",
		AgentID:   "u-alice",
		AgentName: "alice",
		Profile:   agentruntime.Profile{ModelID: "gpt-5.5"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if handle.HandleID != "sess-1" {
		t.Fatalf("Create() handle id = %q, want sess-1", handle.HandleID)
	}

	info, err := rt.Info(context.Background(), handle)
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	if info.State != agentruntime.StateRunning {
		t.Fatalf("Info() state = %q, want %q", info.State, agentruntime.StateRunning)
	}
	if info.HandleID != "sess-1" {
		t.Fatalf("Info() handle id = %q, want sess-1", info.HandleID)
	}

	metaPath := filepath.Join(root, "alice", ".codex", runtimeFileName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read runtime metadata: %v", err)
	}
	var meta runtimeMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal runtime metadata: %v", err)
	}
	if meta.BinaryPath != "/tmp/codex-acp" {
		t.Fatalf("runtime metadata binary path = %q", meta.BinaryPath)
	}

	authRaw, err := os.ReadFile(filepath.Join(root, "alice", ".codex", "auth.json"))
	if err != nil {
		t.Fatalf("read seeded runtime auth: %v", err)
	}
	if string(authRaw) != `{"tokens":{"access_token":"access","refresh_token":"refresh"}}` {
		t.Fatalf("runtime auth = %q, want copied host auth", string(authRaw))
	}
}

func TestRuntimeStopAndDelete(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	calledStop := false
	rt := New(Dependencies{
		BinaryProvider: fakeBinaryProvider{path: "/tmp/codex-acp"},
		AgentHome: func(agentName string) (string, error) {
			return filepath.Join(root, agentName), nil
		},
		ResolveAgent: func(h agentruntime.Handle) (AgentRef, error) {
			return AgentRef{ID: "u-alice", Name: "alice", RuntimeID: h.RuntimeID}, nil
		},
		Manager: fakeManager{
			start: func(_ context.Context, spec SessionSpec) (*Session, error) {
				return &Session{
					RuntimeID:    spec.RuntimeID,
					AgentID:      spec.AgentID,
					AgentName:    spec.AgentName,
					SessionID:    "sess-2",
					BinaryPath:   spec.BinaryPath,
					WorkspaceDir: spec.WorkspaceDir,
					HomeDir:      spec.HomeDir,
					CodexHomeDir: spec.CodexHomeDir,
					StderrPath:   spec.StderrPath,
					CreatedAt:    time.Now().UTC(),
					StartedAt:    time.Now().UTC(),
				}, nil
			},
			stop: func(_ context.Context, handle SessionHandle) error {
				calledStop = true
				return nil
			},
		},
	})

	handle, err := rt.Create(context.Background(), agentruntime.Spec{
		RuntimeID: "rt-u-alice",
		AgentID:   "u-alice",
		AgentName: "alice",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	state, err := rt.Stop(context.Background(), handle)
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if !calledStop {
		t.Fatalf("Stop() did not call manager.Stop")
	}
	if state != agentruntime.StateStopped {
		t.Fatalf("Stop() state = %q, want %q", state, agentruntime.StateStopped)
	}

	if err := rt.Delete(context.Background(), handle); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "alice", ".codex")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runtime dir still exists, stat err = %v", err)
	}
}

func TestRuntimeInfoNotFound(t *testing.T) {
	t.Parallel()

	rt := New(Dependencies{
		BinaryProvider: codexacp.Installer{},
		AgentHome: func(agentName string) (string, error) {
			return filepath.Join(t.TempDir(), agentName), nil
		},
		ResolveAgent: func(h agentruntime.Handle) (AgentRef, error) {
			return AgentRef{ID: "u-missing", Name: "missing", RuntimeID: h.RuntimeID}, nil
		},
	})

	_, err := rt.Info(context.Background(), agentruntime.Handle{RuntimeID: "rt-missing"})
	if !sandbox.IsNotFound(err) {
		t.Fatalf("Info() error = %v, want sandbox not found", err)
	}
}

func TestRuntimeCreateKeepsExistingRuntimeAuth(t *testing.T) {
	root := t.TempDir()
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	if err := os.MkdirAll(filepath.Join(hostHome, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostHome, ".codex", "auth.json"), []byte(`{"tokens":{"access_token":"host","refresh_token":"host-refresh"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	runtimeAuthPath := filepath.Join(root, "alice", ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(runtimeAuthPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimeAuthPath, []byte(`{"tokens":{"access_token":"runtime","refresh_token":"runtime-refresh"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	rt := New(Dependencies{
		BinaryProvider: fakeBinaryProvider{path: "/tmp/codex-acp"},
		AgentHome: func(agentName string) (string, error) {
			return filepath.Join(root, agentName), nil
		},
		ResolveAgent: func(h agentruntime.Handle) (AgentRef, error) {
			return AgentRef{
				ID:        "u-alice",
				Name:      "alice",
				RuntimeID: h.RuntimeID,
			}, nil
		},
		Manager: fakeManager{
			start: func(_ context.Context, spec SessionSpec) (*Session, error) {
				return &Session{
					RuntimeID:    spec.RuntimeID,
					AgentID:      spec.AgentID,
					AgentName:    spec.AgentName,
					SessionID:    "sess-existing-auth",
					BinaryPath:   spec.BinaryPath,
					WorkspaceDir: spec.WorkspaceDir,
					HomeDir:      spec.HomeDir,
					CodexHomeDir: spec.CodexHomeDir,
					StderrPath:   spec.StderrPath,
					ProcessID:    os.Getpid(),
					CreatedAt:    time.Now().UTC(),
					StartedAt:    time.Now().UTC(),
				}, nil
			},
		},
	})

	if _, err := rt.Create(context.Background(), agentruntime.Spec{
		RuntimeID: "rt-u-alice",
		AgentID:   "u-alice",
		AgentName: "alice",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	authRaw, err := os.ReadFile(runtimeAuthPath)
	if err != nil {
		t.Fatalf("read runtime auth: %v", err)
	}
	if string(authRaw) != `{"tokens":{"access_token":"runtime","refresh_token":"runtime-refresh"}}` {
		t.Fatalf("runtime auth = %q, want existing runtime auth preserved", string(authRaw))
	}
}

func TestRuntimeStartKeepsExistingRunningSession(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	startCalls := 0
	rt := New(Dependencies{
		BinaryProvider: fakeBinaryProvider{path: "/tmp/codex-acp"},
		AgentHome: func(agentName string) (string, error) {
			return filepath.Join(root, agentName), nil
		},
		ResolveAgent: func(h agentruntime.Handle) (AgentRef, error) {
			return AgentRef{
				ID:        "u-alice",
				Name:      "alice",
				RuntimeID: h.RuntimeID,
			}, nil
		},
		Manager: fakeManager{
			start: func(_ context.Context, spec SessionSpec) (*Session, error) {
				startCalls++
				return &Session{
					RuntimeID:    spec.RuntimeID,
					AgentID:      spec.AgentID,
					AgentName:    spec.AgentName,
					SessionID:    "sess-keep",
					BinaryPath:   spec.BinaryPath,
					WorkspaceDir: spec.WorkspaceDir,
					HomeDir:      spec.HomeDir,
					CodexHomeDir: spec.CodexHomeDir,
					StderrPath:   spec.StderrPath,
					ProcessID:    os.Getpid(),
					CreatedAt:    time.Now().UTC(),
					StartedAt:    time.Now().UTC(),
				}, nil
			},
		},
	})

	handle, err := rt.Create(context.Background(), agentruntime.Spec{
		RuntimeID: "rt-u-alice",
		AgentID:   "u-alice",
		AgentName: "alice",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if startCalls != 1 {
		t.Fatalf("Create() manager start calls = %d, want 1", startCalls)
	}

	state, err := rt.Start(context.Background(), handle)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if state != agentruntime.StateRunning {
		t.Fatalf("Start() state = %q, want %q", state, agentruntime.StateRunning)
	}
	if startCalls != 1 {
		t.Fatalf("Start() manager start calls = %d, want still 1", startCalls)
	}
}

func TestRuntimeCreateDetachesManagerStartContext(t *testing.T) {
	root := t.TempDir()
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	if err := os.MkdirAll(filepath.Join(hostHome, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostHome, ".codex", "auth.json"), []byte(`{"tokens":{"access_token":"access","refresh_token":"refresh"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var startCtx context.Context
	rt := New(Dependencies{
		BinaryProvider: fakeBinaryProvider{path: "/tmp/codex-acp"},
		AgentHome: func(agentName string) (string, error) {
			return filepath.Join(root, agentName), nil
		},
		ResolveAgent: func(h agentruntime.Handle) (AgentRef, error) {
			return AgentRef{
				ID:        "u-alice",
				Name:      "alice",
				RuntimeID: h.RuntimeID,
				Profile:   agentruntime.Profile{ModelID: "gpt-5.5"},
			}, nil
		},
		Manager: fakeManager{
			start: func(ctx context.Context, spec SessionSpec) (*Session, error) {
				startCtx = ctx
				return &Session{
					RuntimeID:    spec.RuntimeID,
					AgentID:      spec.AgentID,
					AgentName:    spec.AgentName,
					SessionID:    "sess-ctx",
					BinaryPath:   spec.BinaryPath,
					WorkspaceDir: spec.WorkspaceDir,
					HomeDir:      spec.HomeDir,
					CodexHomeDir: spec.CodexHomeDir,
					StderrPath:   spec.StderrPath,
					ProcessID:    os.Getpid(),
					CreatedAt:    time.Now().UTC(),
					StartedAt:    time.Now().UTC(),
				}, nil
			},
		},
	})

	parentCtx, cancel := context.WithCancel(context.Background())
	if _, err := rt.Create(parentCtx, agentruntime.Spec{
		RuntimeID: "rt-u-alice",
		AgentID:   "u-alice",
		AgentName: "alice",
		Profile:   agentruntime.Profile{ModelID: "gpt-5.5"},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cancel()

	if startCtx == nil {
		t.Fatal("manager start context was not captured")
	}
	select {
	case <-startCtx.Done():
		t.Fatal("manager start context was canceled with parent request context")
	default:
	}
}

func TestRuntimeInfoMarksExitedAndFailedWhenProcessIsGone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		exitCode int
		want     agentruntime.State
	}{
		{name: "exited", exitCode: 0, want: agentruntime.StateExited},
		{name: "failed", exitCode: 7, want: agentruntime.StateFailed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			rt := New(Dependencies{
				AgentHome: func(agentName string) (string, error) {
					return filepath.Join(root, agentName), nil
				},
				ResolveAgent: func(h agentruntime.Handle) (AgentRef, error) {
					return AgentRef{
						ID:        "u-alice",
						Name:      "alice",
						RuntimeID: h.RuntimeID,
					}, nil
				},
			})

			meta := runtimeMetadata{
				RuntimeID: "rt-u-alice",
				AgentID:   "u-alice",
				AgentName: "alice",
				SessionID: "sess-1",
				ProcessID: 999999,
				State:     agentruntime.StateRunning,
				ExitCode:  tc.exitCode,
				CreatedAt: time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC),
				StartedAt: time.Date(2026, 5, 5, 8, 0, 1, 0, time.UTC),
			}
			if err := rt.ensureRuntimeHome("alice"); err != nil {
				t.Fatalf("ensureRuntimeHome() error = %v", err)
			}
			if err := rt.writeMetadata(meta); err != nil {
				t.Fatalf("writeMetadata() error = %v", err)
			}

			info, err := rt.Info(context.Background(), agentruntime.Handle{RuntimeID: "rt-u-alice"})
			if err != nil {
				t.Fatalf("Info() error = %v", err)
			}
			if info.State != tc.want {
				t.Fatalf("Info() state = %q, want %q", info.State, tc.want)
			}

			saved, err := rt.readRuntimeMetadata("rt-u-alice")
			if err != nil {
				t.Fatalf("readRuntimeMetadata() error = %v", err)
			}
			if saved.State != tc.want {
				t.Fatalf("saved state = %q, want %q", saved.State, tc.want)
			}
			if saved.ProcessID != 0 {
				t.Fatalf("saved pid = %d, want 0", saved.ProcessID)
			}
		})
	}
}
