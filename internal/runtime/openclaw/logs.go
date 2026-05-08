package openclaw

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func streamLogFile(ctx context.Context, path string, follow bool, lines int, w io.Writer) error {
	if path == "" {
		return fmt.Errorf("log path is required")
	}
	if w == nil {
		w = io.Discard
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}

	args := []string{"-n", strconv.Itoa(lines)}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, path)
	cmd := exec.CommandContext(ctx, "tail", args...)
	cmd.Stdout = w
	cmd.Stderr = w
	err := cmd.Run()
	if follow && errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	if err != nil {
		return err
	}
	if follow {
		<-time.After(10 * time.Millisecond)
	}
	return nil
}
