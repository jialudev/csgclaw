//go:build windows

package codex

import "syscall"

func newSessionSysProcAttr() *syscall.SysProcAttr {
	return nil
}
