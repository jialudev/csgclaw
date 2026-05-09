//go:build !windows

package serve

import "syscall"

func backgroundServeSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
