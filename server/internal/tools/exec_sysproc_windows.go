//go:build windows

package tools

import "syscall"

// newSysProcAttr returns platform-specific SysProcAttr for process execution.
// On Windows, it returns an empty SysProcAttr.
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
