package onboard

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"csgclaw/cli/command"
	internalonboard "csgclaw/internal/onboard"

	"golang.org/x/term"
)

var (
	isTerminalFD = term.IsTerminal
)

type cmd struct{}

func NewCmd() command.Command {
	return cmd{}
}

func (cmd) Name() string {
	return "onboard"
}

func (cmd) Summary() string {
	return "Explicitly initialize local config and bootstrap state."
}

func (c cmd) Run(ctx context.Context, run *command.Context, args []string, globals command.GlobalOptions) error {
	fs := run.NewFlagSet("onboard", run.Program+" onboard [flags]", c.Summary())
	debianRegistries := fs.String("debian-registries", "", "comma-separated OCI registries used for debian:bookworm-slim pulls (persisted to config)")
	logLevel := fs.String("log-level", "info", "log level: debug, info, warn, error")
	if err := fs.Parse(args); err != nil {
		return err
	}

	restore, err := configureOnboardLogger(run.Stderr, *logLevel)
	if err != nil {
		return err
	}
	defer restore()

	opts := internalonboard.EnsureStateOptions{ConfigPath: globals.Config}
	if strings.TrimSpace(*debianRegistries) != "" {
		opts.DebianRegistries = internalonboard.ParseRegistriesFlag(*debianRegistries)
		opts.HasDebianOverrides = true
	}

	resultState, err := internalonboard.EnsureState(ctx, opts)
	if err != nil {
		return err
	}

	result := command.ActionResult{
		Command:      "onboard",
		Action:       "initialize",
		Status:       "initialized",
		ConfigPath:   resultState.ConfigPath,
		ManagerImage: resultState.Config.Bootstrap.EffectiveManagerImage(),
		Users:        []string{"admin", "manager"},
		Message:      fmt.Sprintf("initialized config at %s", resultState.ConfigPath),
	}
	if globals.Output == "json" {
		return command.RenderAction(globals.Output, run.Stdout, result)
	}
	fmt.Fprintln(run.Stdout, result.Message)
	fmt.Fprintf(run.Stdout, "ensured bootstrap agent %q with image %q\n", "manager", resultState.Config.Bootstrap.EffectiveManagerImage())
	fmt.Fprintf(run.Stdout, "ensured IM members %q and %q\n", "admin", "manager")
	fmt.Fprintln(run.Stdout, "cleared IM invite draft data")
	return nil
}

func configureOnboardLogger(w io.Writer, level string) (func(), error) {
	parsedLevel, err := parseOnboardLogLevel(level)
	if err != nil {
		return nil, err
	}
	if w == nil {
		w = os.Stderr
	}

	prev := slog.Default()
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: parsedLevel})
	if isTerminal(w) {
		handler = nil
	}
	var logger *slog.Logger
	if handler != nil {
		logger = slog.New(handler)
	} else {
		logger = slog.New(&onboardTerminalHandler{
			writer: w,
			level:  parsedLevel,
		})
	}
	slog.SetDefault(logger)
	return func() {
		slog.SetDefault(prev)
	}, nil
}

func parseOnboardLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", level)
	}
}

type onboardTerminalHandler struct {
	writer io.Writer
	level  slog.Leveler
	attrs  []slog.Attr
	groups []string
}

func (h *onboardTerminalHandler) Enabled(_ context.Context, level slog.Level) bool {
	threshold := slog.LevelInfo
	if h != nil && h.level != nil {
		threshold = h.level.Level()
	}
	return level >= threshold
}

func (h *onboardTerminalHandler) Handle(_ context.Context, record slog.Record) error {
	if h == nil || h.writer == nil {
		return nil
	}

	attrs := make([]slog.Attr, 0, len(h.attrs)+record.NumAttrs())
	attrs = append(attrs, h.attrs...)
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	parts := make([]string, 0, len(attrs)+1)
	parts = append(parts, record.Message)
	for _, attr := range attrs {
		key := qualifyOnboardLogKey(h.groups, attr.Key)
		parts = append(parts, fmt.Sprintf("%s=%s", key, onboardLogAttrValue(attr.Value)))
	}

	_, err := fmt.Fprintf(h.writer, "%s %s\n", strings.ToUpper(record.Level.String()), strings.Join(parts, " "))
	return err
}

func (h *onboardTerminalHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}

func (h *onboardTerminalHandler) WithGroup(name string) slog.Handler {
	clone := *h
	clone.groups = append(append([]string{}, h.groups...), name)
	return &clone
}

func qualifyOnboardLogKey(groups []string, key string) string {
	if len(groups) == 0 {
		return key
	}
	return strings.Join(append(append([]string{}, groups...), key), ".")
}

func onboardLogAttrValue(value slog.Value) string {
	return fmt.Sprint(value.Any())
}

func isTerminal(value any) bool {
	file, ok := value.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	return isTerminalFD(int(file.Fd()))
}
