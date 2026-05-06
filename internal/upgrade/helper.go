package upgrade

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	startHelperExecutable = os.Executable
	startHelperCommand    = exec.Command
)

type ApplyHelperOptions struct {
	ConfigPath string
}

func StartApplyHelper(opts ApplyHelperOptions) error {
	exe, err := startHelperExecutable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	args := []string{"upgrade"}
	if configPath := strings.TrimSpace(opts.ConfigPath); configPath != "" {
		args = append(args, "--config", configPath)
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	defer devNull.Close()

	cmd := startHelperCommand(exe, args...)
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start upgrade helper: %w", err)
	}

	go func() {
		_ = cmd.Wait()
	}()

	return nil
}
