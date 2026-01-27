//go:build windows

package server

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// reusePort sets SO_REUSEADDR socket option on Windows.
// Note: Windows does not support SO_REUSEPORT, but SO_REUSEADDR behaves similarly.
func reusePort(network, address string, c syscall.RawConn) error {
	var opErr error
	err := c.Control(func(fd uintptr) {
		// SO_REUSEADDR on Windows allows reusing the address
		if err := windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1); err != nil {
			opErr = err
			return
		}
	})
	if err != nil {
		return err
	}
	return opErr
}
