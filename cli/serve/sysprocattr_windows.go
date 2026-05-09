//go:build windows

package serve

import "syscall"

func backgroundServeSysProcAttr() *syscall.SysProcAttr {
	return nil
}
