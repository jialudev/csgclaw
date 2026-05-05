package runtime

import (
	"context"
	"io"
	"time"
)

const (
	KindPicoClawSandbox = "picoclaw_sandbox"
	KindOpenClawSandbox = "openclaw_sandbox"
	KindCodex           = "codex"
)

type Runtime interface {
	Kind() string

	Create(ctx context.Context, spec Spec) (Handle, error)
	Start(ctx context.Context, h Handle) (State, error)
	Stop(ctx context.Context, h Handle) (State, error)
	Delete(ctx context.Context, h Handle) error
	State(ctx context.Context, h Handle) (State, error)
	Info(ctx context.Context, h Handle) (Info, error)
}

type LogStreamer interface {
	StreamLogs(ctx context.Context, h Handle, opts LogOptions) error
}

type Handle struct {
	RuntimeID string `json:"runtime_id"`
	HandleID  string `json:"handle_id,omitempty"`
}

type Info struct {
	HandleID  string
	State     State
	CreatedAt time.Time
}

type State string

const (
	StateUnknown State = "unknown"
	StateCreated State = "created"
	StateRunning State = "running"
	StateStopped State = "stopped"
	StateExited  State = "exited"
	StateFailed  State = "failed"
)

type Profile struct {
	ModelID string
	Env     map[string]string
}

type Spec struct {
	RuntimeID string
	AgentID   string
	AgentName string
	Image     string
	Profile   Profile
}

type LogOptions struct {
	Follow bool
	Tail   int
	Writer io.Writer
}
