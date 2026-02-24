//go:build !windows

package sockipc

import (
	"fmt"
	"net"
	"os"
)

// listen creates a Unix domain socket listener.
func listen(path string) (net.Listener, error) {
	// Remove stale socket from previous run
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("sockipc: remove stale socket: %w", err)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("sockipc: listen %s: %w", path, err)
	}
	// Owner-only access — Unix socket permissions are the auth boundary.
	os.Chmod(path, 0600)
	return ln, nil
}

// cleanup removes the socket file.
func cleanup(path string) {
	os.Remove(path)
}
