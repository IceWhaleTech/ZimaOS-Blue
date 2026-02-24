//go:build windows

package sockipc

import (
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
)

// listen creates a Windows named pipe listener.
// The path is converted to a named pipe path: \\.\pipe\<name>
func listen(path string) (net.Listener, error) {
	pipePath := `\\.\pipe\` + pipeName(path)
	ln, err := winio.ListenPipe(pipePath, nil)
	if err != nil {
		return nil, fmt.Errorf("sockipc: listen pipe %s: %w", pipePath, err)
	}
	return ln, nil
}

// cleanup is a no-op on Windows (named pipes are cleaned up automatically).
func cleanup(_ string) {}

// pipeName extracts a pipe name from a file path.
// e.g. "/data/blue.sock" -> "blue.sock"
func pipeName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
