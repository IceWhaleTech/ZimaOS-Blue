//go:build !windows

package tools

import "syscall"

// newSysProcAttr returns platform-specific SysProcAttr for process execution.
// On Unix, it sets Setpgid to create a new process group.
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
