package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
)

// dialSock connects to the blue IPC socket.
// Tries /tmp/blue.sock first (server default), then {dataDir}/blue.sock.
func dialSock() (net.Conn, error) {
	paths := []string{
		"/tmp/blue.sock",
		filepath.Join(getDataDir(), "blue.sock"),
	}
	var lastErr error
	for _, p := range paths {
		conn, err := net.DialTimeout("unix", p, 5*time.Second)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("cannot connect to blue.sock: %w", lastErr)
}

// ipcRoundTrip sends a JSON request and reads the JSON response.
func ipcRoundTrip(req *sockipc.Request) (*sockipc.Response, error) {
	conn, err := dialSock()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := sockipc.WriteJSON(conn, req); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	resp, err := sockipc.ReadJSON[sockipc.Response](conn)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return resp, nil
}

// ipcFallback forwards an unrecognized CLI command as an IPC request.
// Returns true if handled (even on error), false if server not reachable.
func ipcFallback(cmd string, args []string) bool {
	params := make(map[string]string)

	// Parse args: collect key=value pairs, skip flags
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--") || strings.HasPrefix(a, "-") {
			continue // skip flags
		}
		idx := strings.IndexByte(a, '=')
		if idx < 0 {
			fmt.Fprintf(os.Stderr, "Error: argument %q must be key=value\n", a)
			os.Exit(1)
		}
		params[a[:idx]] = a[idx+1:]
	}

	req := &sockipc.Request{Cmd: cmd, Params: params}
	resp, err := ipcRoundTrip(req)
	if err != nil {
		// If we can't connect, let cobra handle it (might be a typo)
		if isConnectionError(err) {
			return false
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if resp.Status != "ok" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}

	if jsonOutput {
		printJSON(resp)
	} else {
		if resp.Data != nil {
			for k, v := range resp.Data {
				fmt.Printf("%s: %s\n", k, v)
			}
		}
	}
	return true
}

func isConnectionError(err error) bool {
	s := err.Error()
	return strings.Contains(s, "cannot connect") ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "no such file")
}

