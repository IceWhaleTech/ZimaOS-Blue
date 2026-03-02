package main

// Lightweight IPC client for the launcher. This avoids exec-ing .bluecli for
// IPC-only commands (skill calls like `blue web_search "test"`), saving one
// full process spawn + Go runtime init.
//
// The protocol is identical to sockipc: 4-byte big-endian length prefix + JSON.
// We reimplement it here (~80 lines) instead of importing internal/sockipc to
// keep the launcher free of server dependencies.

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ipcRequest mirrors sockipc.Request.
type ipcRequest struct {
	Cmd    string            `json:"cmd"`
	Params map[string]string `json:"params,omitempty"`
}

// ipcResponse mirrors sockipc.Response.
type ipcResponse struct {
	Status string            `json:"status"`
	Error  string            `json:"error,omitempty"`
	Data   map[string]string `json:"data,omitempty"`
}

// tryIPC attempts to handle a CLI invocation via direct IPC to the running
// server. Returns true when positional command args are present, even on IPC
// errors, to prevent launcher fallback to exec .bluecli for command invocations.
// Returns false only when IPC is not applicable (e.g. no subcommand).
func tryIPC(args []string) bool {
	// Parse global flags, collect positional args (mirrors cliDispatch logic)
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dev", "--no-color", "--json", "-v", "--verbose":
			// skip flags
		case "-h", "--help":
			return false // let bluecli handle help
		case "--config", "--profile":
			if i+1 < len(args) {
				i++ // skip value
			}
		case "--":
			positional = append(positional, args[i+1:]...)
			i = len(args)
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) == 0 {
		return false // no subcommand → need exec for server start
	}

	cmd := positional[0]
	rest := positional[1:]
	if cmd == "help" {
		// Let bluecli handle `help` so it can render full command/skill manuals.
		return false
	}
	params := parseIPCArgs(rest)

	conn, err := dialSock()
	if err != nil {
		fmt.Fprintf(os.Stdout, "Error: cannot connect to running Blue service (IPC unavailable): %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(25 * time.Second))

	if err := writeMsg(conn, &ipcRequest{Cmd: cmd, Params: params}); err != nil {
		fmt.Fprintf(os.Stdout, "Error: IPC write failed: %v\n", err)
		os.Exit(1)
	}
	resp, err := readMsg(conn)
	if err != nil {
		fmt.Fprintf(os.Stdout, "Error: IPC read failed: %v\n", err)
		os.Exit(1)
	}

	if resp.Status != "ok" {
		fmt.Fprintf(os.Stdout, "Error: %s\n", resp.Error)
		os.Exit(1)
	}

	// Print data fields (same format as ipcFallback in cmd/blue)
	if resp.Data != nil {
		for k, v := range resp.Data {
			if k == "_card" || k == "success" {
				continue
			}
			fmt.Printf("%s: %s\n", k, v)
		}
	}
	return true
}

// parseIPCArgs converts CLI args to IPC params (mirrors ipcFallback in cmd/blue).
func parseIPCArgs(args []string) map[string]string {
	params := make(map[string]string)
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--") || strings.HasPrefix(a, "-") {
			key := strings.TrimLeft(a, "-")
			if key != "" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				params[key] = args[i+1]
				i++
			}
			continue
		}
		if idx := strings.IndexByte(a, '='); idx >= 0 {
			params[a[:idx]] = a[idx+1:]
		} else {
			positional = append(positional, a)
		}
	}
	if len(positional) > 0 && len(params) == 0 {
		params["query"] = strings.Join(positional, " ")
	}
	return params
}

// dialSock connects to the blue IPC socket.
func dialSock() (net.Conn, error) {
	paths := []string{"/tmp/blue.sock"}

	// Also try {dataDir}/blue.sock
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".zimaos-blue", "data", "blue.sock"))
	}

	// On Windows, try named pipe
	if runtime.GOOS == "windows" {
		paths = append([]string{`\\.\pipe\blue`}, paths...)
	}

	var lastErr error
	for _, p := range paths {
		conn, err := net.DialTimeout("unix", p, 3*time.Second)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// writeMsg writes a length-prefixed JSON message.
func writeMsg(conn net.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(data)))
	if _, err := conn.Write(hdr[:]); err != nil {
		return err
	}
	_, err = conn.Write(data)
	return err
}

// readMsg reads a length-prefixed JSON response.
func readMsg(conn net.Conn) (*ipcResponse, error) {
	var hdr [4]byte
	if _, err := readFull(conn, hdr[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(hdr[:])
	if size > 16<<20 {
		return nil, fmt.Errorf("message too large: %d", size)
	}
	buf := make([]byte, size)
	if _, err := readFull(conn, buf); err != nil {
		return nil, err
	}
	var resp ipcResponse
	if err := json.Unmarshal(buf, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// readFull reads exactly len(buf) bytes.
func readFull(conn net.Conn, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		nn, err := conn.Read(buf[n:])
		n += nn
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
