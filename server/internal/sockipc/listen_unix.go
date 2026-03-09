//go:build !windows

package sockipc

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

// listen creates a Unix domain socket listener.
func listen(path string) (net.Listener, error) {
	if _, err := os.Stat(path); err == nil {
		conn, dialErr := net.DialTimeout("unix", path, 250*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return nil, fmt.Errorf("sockipc: socket already in use: %s", path)
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("sockipc: remove stale socket: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("sockipc: stat %s: %w", path, err)
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("sockipc: listen %s: %w", path, err)
	}
	// Owner-only access — Unix socket permissions are the auth boundary.
	_ = os.Chmod(path, 0600)
	return ln, nil
}

// cleanup removes the socket file.
func cleanup(path string) {
	_ = os.Remove(path)
}
