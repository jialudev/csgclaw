package openclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	agentruntime "csgclaw/internal/runtime"
	"csgclaw/internal/sandbox"
)

const (
	DefaultGatewayPort      = 18789
	defaultPackageJSON      = "package.json"
	runtimeDirName          = ".openclaw-runtime"
	runtimeFileName         = "runtime.json"
	openClawPackageName     = "openclaw"
	csgClawExtensionPackage = "csgclaw-extension"
	openClawPackageSpec     = "latest"
	csgClawExtensionSpec    = "latest"
)

type AgentRef struct {
	ID        string
	Name      string
	RuntimeID string
	HandleID  string
	Profile   agentruntime.Profile
}

type PrepareOptions struct {
	GatewayPort int
	PluginPaths []string
}

type PreparedAgent struct {
	AgentHome    string
	OpenClawRoot string
	WorkspaceDir string
	LogPath      string
}

type Progress struct {
	AgentName string
	Stage     string
	Message   string
	Percent   int
}

type CommandSpec struct {
	Name   string
	Args   []string
	Dir    string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

type Process interface {
	PID() int
	Wait() error
	Kill() error
}

type Dependencies struct {
	ResolveAgent func(agentruntime.Handle) (AgentRef, error)
	AgentHome    func(agentName string) (string, error)
	InstallRoot  func() (string, error)
	PrepareAgent func(agentName, botID string, profile agentruntime.Profile, opts PrepareOptions) (PreparedAgent, error)
	Progress     func(Progress)

	LookPath      func(string) (string, error)
	CommandOutput func(context.Context, CommandSpec) ([]byte, error)
	RunCommand    func(context.Context, CommandSpec) error
	StartCommand  func(context.Context, CommandSpec) (Process, error)

	MkdirAll  func(string, os.FileMode) error
	ReadFile  func(string) ([]byte, error)
	WriteFile func(string, []byte, os.FileMode) error
	RemoveAll func(string) error
	OpenFile  func(string, int, os.FileMode) (*os.File, error)
	Now       func() time.Time
}

type Runtime struct {
	deps Dependencies
}

var (
	_ agentruntime.Runtime     = (*Runtime)(nil)
	_ agentruntime.LogStreamer = (*Runtime)(nil)
)

func New(deps Dependencies) *Runtime {
	return &Runtime{deps: deps}
}

func (r *Runtime) Kind() string {
	return agentruntime.KindOpenClaw
}

func (r *Runtime) Create(ctx context.Context, spec agentruntime.Spec) (agentruntime.Handle, error) {
	if strings.TrimSpace(spec.RuntimeID) == "" {
		return agentruntime.Handle{}, fmt.Errorf("runtime id is required")
	}
	if strings.TrimSpace(spec.AgentID) == "" || strings.TrimSpace(spec.AgentName) == "" {
		return agentruntime.Handle{}, fmt.Errorf("agent id and name are required")
	}
	meta, err := r.ensureGateway(ctx, AgentRef{
		ID:        strings.TrimSpace(spec.AgentID),
		Name:      strings.TrimSpace(spec.AgentName),
		RuntimeID: strings.TrimSpace(spec.RuntimeID),
		Profile:   spec.Profile,
	})
	if err != nil {
		return agentruntime.Handle{}, err
	}
	return agentruntime.Handle{
		RuntimeID: strings.TrimSpace(spec.RuntimeID),
		HandleID:  strconv.Itoa(meta.ProcessID),
	}, nil
}

func (r *Runtime) Start(ctx context.Context, h agentruntime.Handle) (agentruntime.State, error) {
	if info, err := r.Info(ctx, h); err == nil && info.State == agentruntime.StateRunning {
		return agentruntime.StateRunning, nil
	}
	ref, err := r.resolveAgent(h)
	if err != nil {
		return agentruntime.StateUnknown, err
	}
	ref.RuntimeID = strings.TrimSpace(h.RuntimeID)
	meta, err := r.ensureGateway(ctx, ref)
	if err != nil {
		return agentruntime.StateUnknown, err
	}
	if meta.State == "" {
		return agentruntime.StateRunning, nil
	}
	return meta.State, nil
}

func (r *Runtime) Stop(ctx context.Context, h agentruntime.Handle) (agentruntime.State, error) {
	meta, err := r.readRuntimeMetadata(h.RuntimeID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return agentruntime.StateUnknown, sandbox.ErrNotFound
		}
		return agentruntime.StateUnknown, err
	}
	if meta.ProcessID > 0 {
		_ = terminateProcess(meta.ProcessID)
	}
	meta.ProcessID = 0
	meta.State = agentruntime.StateStopped
	meta.StoppedAt = r.now()
	if err := r.writeMetadata(meta); err != nil {
		return agentruntime.StateUnknown, err
	}
	return agentruntime.StateStopped, nil
}

func (r *Runtime) Delete(ctx context.Context, h agentruntime.Handle) error {
	_, _ = r.Stop(ctx, h)
	dir, err := r.runtimeDirForHandle(h)
	if err != nil {
		return err
	}
	if err := r.removeAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (r *Runtime) State(ctx context.Context, h agentruntime.Handle) (agentruntime.State, error) {
	info, err := r.Info(ctx, h)
	if err != nil {
		return agentruntime.StateUnknown, err
	}
	return info.State, nil
}

func (r *Runtime) Info(_ context.Context, h agentruntime.Handle) (agentruntime.Info, error) {
	meta, err := r.readRuntimeMetadata(h.RuntimeID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return agentruntime.Info{}, sandbox.ErrNotFound
		}
		return agentruntime.Info{}, err
	}
	state := meta.State
	if state == agentruntime.StateRunning && !processAlive(meta.ProcessID) {
		state = agentruntime.StateExited
		meta.State = state
		meta.ProcessID = 0
		_ = r.writeMetadata(meta)
	}
	handleID := ""
	if meta.ProcessID > 0 {
		handleID = strconv.Itoa(meta.ProcessID)
	}
	return agentruntime.Info{
		HandleID:  handleID,
		State:     state,
		CreatedAt: meta.CreatedAt,
	}, nil
}

func (r *Runtime) StreamLogs(ctx context.Context, h agentruntime.Handle, opts agentruntime.LogOptions) error {
	meta, err := r.readRuntimeMetadata(h.RuntimeID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sandbox.ErrNotFound
		}
		return err
	}
	lines := opts.Tail
	if lines <= 0 {
		lines = 20
	}
	return streamLogFile(ctx, meta.LogPath, opts.Follow, lines, opts.Writer)
}

func (r *Runtime) ensureGateway(ctx context.Context, ref AgentRef) (runtimeMetadata, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ref.Profile = ref.Profile.Normalized()
	r.progress(ref.Name, "node", "checking Node.js", 5)
	if err := r.ensureNode(ctx); err != nil {
		return runtimeMetadata{}, err
	}
	r.progress(ref.Name, "pnpm", "checking pnpm", 15)
	if err := r.ensurePNPM(ctx); err != nil {
		return runtimeMetadata{}, err
	}
	r.progress(ref.Name, "install_root", "preparing OpenClaw install directory", 25)
	installRoot, err := r.installRoot()
	if err != nil {
		return runtimeMetadata{}, err
	}
	if err := r.mkdirAll(installRoot, 0o755); err != nil {
		return runtimeMetadata{}, fmt.Errorf("create openclaw install root: %w", err)
	}
	r.progress(ref.Name, "pnpm_install", "installing openclaw packages", 45)
	if err := r.ensurePackages(ctx, installRoot); err != nil {
		return runtimeMetadata{}, err
	}
	port, err := r.nextGatewayPort(DefaultGatewayPort)
	if err != nil {
		return runtimeMetadata{}, err
	}
	pluginPath := filepath.Join(installRoot, "node_modules", csgClawExtensionPackage)
	r.progress(ref.Name, "config", fmt.Sprintf("writing OpenClaw config on port %d", port), 70)
	prepared, err := r.prepareAgent(ref, port, pluginPath)
	if err != nil {
		return runtimeMetadata{}, err
	}
	r.progress(ref.Name, "start", "starting OpenClaw gateway", 90)
	meta, err := r.startGateway(ctx, ref, installRoot, prepared, port)
	if err != nil {
		return runtimeMetadata{}, err
	}
	r.progress(ref.Name, "ready", "OpenClaw gateway is running", 100)
	return meta, nil
}

func (r *Runtime) ensureNode(ctx context.Context) error {
	if _, err := r.lookPath("node"); err != nil {
		return fmt.Errorf("node is required for openclaw runtime; install Node.js 22.14 or newer: %w", err)
	}
	out, err := r.commandOutput(ctx, CommandSpec{Name: "node", Args: []string{"--version"}})
	if err != nil {
		return fmt.Errorf("check node version: %w", err)
	}
	version := strings.TrimSpace(strings.TrimPrefix(string(out), "v"))
	if !nodeVersionAtLeast(version, 22, 14) {
		return fmt.Errorf("openclaw runtime requires Node.js >= 22.14; found %q", strings.TrimSpace(string(out)))
	}
	return nil
}

func (r *Runtime) ensurePNPM(ctx context.Context) error {
	if _, err := r.lookPath("pnpm"); err != nil {
		return fmt.Errorf("pnpm is required for openclaw runtime; install pnpm or enable it with corepack: %w", err)
	}
	if _, err := r.commandOutput(ctx, CommandSpec{Name: "pnpm", Args: []string{"--version"}}); err != nil {
		return fmt.Errorf("check pnpm version: %w", err)
	}
	return nil
}

func (r *Runtime) ensurePackages(ctx context.Context, installRoot string) error {
	packageJSONPath := filepath.Join(installRoot, defaultPackageJSON)
	want, err := json.MarshalIndent(map[string]any{
		"private":        true,
		"packageManager": "pnpm@10.32.1",
		"dependencies": map[string]string{
			openClawPackageName:     openClawPackageSpec,
			csgClawExtensionPackage: csgClawExtensionSpec,
		},
	}, "", "  ")
	if err != nil {
		return err
	}
	want = append(want, '\n')
	changed := true
	if existing, readErr := r.readFile(packageJSONPath); readErr == nil && bytes.Equal(existing, want) {
		changed = false
	}
	if changed {
		if err := r.writeFile(packageJSONPath, want, 0o644); err != nil {
			return fmt.Errorf("write openclaw package.json: %w", err)
		}
	}
	if !changed && r.packageInstalled(installRoot, openClawPackageName) && r.packageInstalled(installRoot, csgClawExtensionPackage) {
		return nil
	}
	if err := r.runCommand(ctx, CommandSpec{
		Name:   "pnpm",
		Args:   []string{"install", "--prod"},
		Dir:    installRoot,
		Stdout: os.Stderr,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("pnpm install openclaw packages: %w", err)
	}
	return nil
}

func (r *Runtime) packageInstalled(root, name string) bool {
	_, err := r.readFile(filepath.Join(root, "node_modules", name, "package.json"))
	return err == nil
}

func (r *Runtime) prepareAgent(ref AgentRef, port int, pluginPath string) (PreparedAgent, error) {
	if r.deps.PrepareAgent == nil {
		return PreparedAgent{}, fmt.Errorf("openclaw agent preparer is required")
	}
	return r.deps.PrepareAgent(ref.Name, ref.ID, ref.Profile, PrepareOptions{
		GatewayPort: port,
		PluginPaths: []string{pluginPath},
	})
}

func (r *Runtime) startGateway(ctx context.Context, ref AgentRef, installRoot string, prepared PreparedAgent, port int) (runtimeMetadata, error) {
	runtimeDir, err := r.runtimeDirForAgent(ref.Name)
	if err != nil {
		return runtimeMetadata{}, err
	}
	if err := r.mkdirAll(runtimeDir, 0o755); err != nil {
		return runtimeMetadata{}, fmt.Errorf("create openclaw runtime dir: %w", err)
	}
	logFile, err := r.openFile(prepared.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return runtimeMetadata{}, fmt.Errorf("open openclaw log: %w", err)
	}
	env := append(openClawEnv(prepared.AgentHome, ref.Profile.Env),
		"CSGCLAW_OPENCLAW_GATEWAY_PORT="+strconv.Itoa(port),
	)
	_ = r.runCommand(ctx, CommandSpec{
		Name:   "pnpm",
		Args:   []string{"--dir", installRoot, "exec", "openclaw", "gateway", "stop"},
		Dir:    prepared.WorkspaceDir,
		Env:    env,
		Stdout: logFile,
		Stderr: logFile,
	})
	proc, err := r.startCommand(context.WithoutCancel(ctx), CommandSpec{
		Name:   "pnpm",
		Args:   []string{"--dir", installRoot, "exec", "openclaw", "gateway", "--allow-unconfigured", "--bind", "lan", "--port", strconv.Itoa(port)},
		Dir:    prepared.WorkspaceDir,
		Env:    env,
		Stdout: logFile,
		Stderr: logFile,
	})
	if err != nil {
		_ = logFile.Close()
		return runtimeMetadata{}, fmt.Errorf("start openclaw gateway: %w", err)
	}
	meta := runtimeMetadata{
		RuntimeID:    strings.TrimSpace(ref.RuntimeID),
		AgentID:      strings.TrimSpace(ref.ID),
		AgentName:    strings.TrimSpace(ref.Name),
		ProcessID:    proc.PID(),
		GatewayPort:  port,
		InstallRoot:  installRoot,
		OpenClawRoot: prepared.OpenClawRoot,
		WorkspaceDir: prepared.WorkspaceDir,
		LogPath:      prepared.LogPath,
		State:        agentruntime.StateRunning,
		CreatedAt:    r.now(),
	}
	if err := r.writeMetadata(meta); err != nil {
		_ = terminateProcess(proc.PID())
		_ = logFile.Close()
		return runtimeMetadata{}, err
	}
	go func() {
		_ = proc.Wait()
		_ = logFile.Close()
		if current, err := r.readRuntimeMetadata(meta.RuntimeID); err == nil && current.ProcessID == meta.ProcessID {
			current.ProcessID = 0
			current.State = agentruntime.StateExited
			current.StoppedAt = r.now()
			_ = r.writeMetadata(current)
		}
	}()
	return meta, nil
}

func openClawEnv(agentHome string, profileEnv map[string]string) []string {
	env := []string{
		"HOME=" + agentHome,
	}
	for key, value := range profileEnv {
		key = strings.TrimSpace(key)
		if key == "" || strings.Contains(key, "=") {
			continue
		}
		env = append(env, key+"="+value)
	}
	return env
}

func (r *Runtime) nextGatewayPort(start int) (int, error) {
	if start <= 0 {
		start = DefaultGatewayPort
	}
	for port := start; port < start+100; port++ {
		ln, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(port)))
		if err != nil {
			continue
		}
		_ = ln.Close()
		return port, nil
	}
	return 0, fmt.Errorf("no available OpenClaw gateway port in range %d-%d", start, start+99)
}

func (r *Runtime) progress(agentName, stage, message string, percent int) {
	progress := Progress{
		AgentName: strings.TrimSpace(agentName),
		Stage:     strings.TrimSpace(stage),
		Message:   strings.TrimSpace(message),
		Percent:   percent,
	}
	if r.deps.Progress != nil {
		r.deps.Progress(progress)
		return
	}
	slog.Info("openclaw runtime progress", "agent", progress.AgentName, "stage", progress.Stage, "percent", progress.Percent, "message", progress.Message)
}

func (r *Runtime) StreamLogPath(h agentruntime.Handle) (string, error) {
	meta, err := r.readRuntimeMetadata(h.RuntimeID)
	if err != nil {
		return "", err
	}
	return meta.LogPath, nil
}

func (r *Runtime) resolveAgent(h agentruntime.Handle) (AgentRef, error) {
	if r.deps.ResolveAgent == nil {
		return AgentRef{}, fmt.Errorf("agent resolver is required")
	}
	got, err := r.deps.ResolveAgent(h)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return AgentRef{}, sandbox.ErrNotFound
		}
		return AgentRef{}, err
	}
	if strings.TrimSpace(got.ID) == "" || strings.TrimSpace(got.Name) == "" {
		return AgentRef{}, fmt.Errorf("resolved agent is incomplete")
	}
	return got, nil
}

func (r *Runtime) runtimeDirForHandle(h agentruntime.Handle) (string, error) {
	got, err := r.resolveAgent(h)
	if err != nil {
		return "", err
	}
	return r.runtimeDirForAgent(got.Name)
}

func (r *Runtime) runtimeDirForAgent(agentName string) (string, error) {
	if r.deps.AgentHome == nil {
		return "", fmt.Errorf("agent home resolver is required")
	}
	home, err := r.deps.AgentHome(strings.TrimSpace(agentName))
	if err != nil {
		return "", err
	}
	return filepath.Join(home, runtimeDirName), nil
}

func (r *Runtime) runtimeMetadataPath(runtimeID string) (string, error) {
	dir, err := r.runtimeDirForHandle(agentruntime.Handle{RuntimeID: strings.TrimSpace(runtimeID)})
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, runtimeFileName), nil
}

func (r *Runtime) readRuntimeMetadata(runtimeID string) (runtimeMetadata, error) {
	path, err := r.runtimeMetadataPath(runtimeID)
	if err != nil {
		return runtimeMetadata{}, err
	}
	var meta runtimeMetadata
	if err := readJSONFile(r.readFile, path, &meta); err != nil {
		return runtimeMetadata{}, err
	}
	return normalizeRuntimeMetadata(meta), nil
}

func (r *Runtime) writeMetadata(meta runtimeMetadata) error {
	path, err := r.runtimeMetadataPath(meta.RuntimeID)
	if err != nil {
		return err
	}
	meta = normalizeRuntimeMetadata(meta)
	return writeJSONFile(r.writeFile, path, meta)
}

func (r *Runtime) installRoot() (string, error) {
	if r.deps.InstallRoot == nil {
		return "", fmt.Errorf("openclaw install root resolver is required")
	}
	return r.deps.InstallRoot()
}

func (r *Runtime) lookPath(name string) (string, error) {
	if r.deps.LookPath != nil {
		return r.deps.LookPath(name)
	}
	return exec.LookPath(name)
}

func (r *Runtime) commandOutput(ctx context.Context, spec CommandSpec) ([]byte, error) {
	if r.deps.CommandOutput != nil {
		return r.deps.CommandOutput(ctx, spec)
	}
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), spec.Env...)
	return cmd.Output()
}

func (r *Runtime) runCommand(ctx context.Context, spec CommandSpec) error {
	if r.deps.RunCommand != nil {
		return r.deps.RunCommand(ctx, spec)
	}
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	return cmd.Run()
}

func (r *Runtime) startCommand(ctx context.Context, spec CommandSpec) (Process, error) {
	if r.deps.StartCommand != nil {
		return r.deps.StartCommand(ctx, spec)
	}
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return execProcess{cmd: cmd}, nil
}

func (r *Runtime) mkdirAll(path string, mode os.FileMode) error {
	if r.deps.MkdirAll != nil {
		return r.deps.MkdirAll(path, mode)
	}
	return os.MkdirAll(path, mode)
}

func (r *Runtime) readFile(path string) ([]byte, error) {
	if r.deps.ReadFile != nil {
		return r.deps.ReadFile(path)
	}
	return os.ReadFile(path)
}

func (r *Runtime) writeFile(path string, data []byte, mode os.FileMode) error {
	if r.deps.WriteFile != nil {
		return r.deps.WriteFile(path, data, mode)
	}
	return os.WriteFile(path, data, mode)
}

func (r *Runtime) removeAll(path string) error {
	if r.deps.RemoveAll != nil {
		return r.deps.RemoveAll(path)
	}
	return os.RemoveAll(path)
}

func (r *Runtime) openFile(path string, flag int, mode os.FileMode) (*os.File, error) {
	if r.deps.OpenFile != nil {
		return r.deps.OpenFile(path, flag, mode)
	}
	return os.OpenFile(path, flag, mode)
}

func (r *Runtime) now() time.Time {
	if r.deps.Now != nil {
		return r.deps.Now().UTC()
	}
	return time.Now().UTC()
}

type execProcess struct {
	cmd *exec.Cmd
}

func (p execProcess) PID() int {
	if p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func (p execProcess) Wait() error {
	if p.cmd == nil {
		return nil
	}
	return p.cmd.Wait()
}

func (p execProcess) Kill() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}
