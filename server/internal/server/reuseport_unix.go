//go:build !windows

package server

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// reusePort sets SO_REUSEADDR and SO_REUSEPORT socket options on Unix systems.
func reusePort(network, address string, c syscall.RawConn) error {
	var opErr error
	err := c.Control(func(fd uintptr) {
		// SO_REUSEADDR allows reusing the address immediately after close
		if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
			opErr = err
			return
		}
		// SO_REUSEPORT allows multiple processes to bind to the same port
		if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
			opErr = err
			return
		}
	})
	if err != nil {
		return err
	}
	return opErr
}
