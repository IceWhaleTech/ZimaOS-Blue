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
	"strconv"
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
	positional, flags := parseCLIFlags(args)
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return false
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
	if !shouldAttemptIPCShortcut(cmd) {
		return false
	}

	// Compatibility: allow action-oriented skill calls in a positional form:
	//   blue reminder add message=... time=...
	// and normalize them to the canonical IPC command form:
	//   reminder.add|list|delete|clear
	var positionalAction string
	if strings.TrimSpace(cmd) == "reminder" {
		if action, remaining := consumeLauncherPositionalAction(rest); action != "" {
			positionalAction = action
			rest = remaining
		}
	}

	params := parseIPCArgs(rest)
	if positionalAction != "" {
		cmd = "reminder." + positionalAction
	} else if strings.TrimSpace(cmd) == "reminder" {
		if action, ok := normalizeLauncherReminderAction(params["action"]); ok {
			cmd = "reminder." + action
			delete(params, "action")
		}
	}

	conn, err := dialSock(flags)
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

	if printed, exitCode := printIPCStdout(resp); printed {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return true
	}

	// Print data fields (same format as ipcFallback in cmd/blue)
	if resp.Data != nil {
		for k, v := range resp.Data {
			if strings.HasPrefix(k, "__") || k == "_card" || k == "success" {
				continue
			}
			fmt.Printf("%s: %s\n", k, v)
		}
	}
	return true
}

func shouldAttemptIPCShortcut(cmd string) bool {
	switch strings.TrimSpace(cmd) {
	case "",
		"help",
		"status",
		"health",
		"version",
		"doctor",
		"config",
		"models",
		"plugins",
		"skills",
		"context",
		"sessions",
		"cron",
		"harness",
		"logs",
		"media",
		"gateway",
		"complete-bootstrap",
		"agent-sessions":
		return false
	default:
		return true
	}
}

func consumeLauncherPositionalAction(args []string) (action string, remaining []string) {
	if len(args) == 0 {
		return "", args
	}
	first := strings.ToLower(strings.TrimSpace(args[0]))
	if first == "" || strings.HasPrefix(first, "-") || strings.Contains(first, "=") {
		return "", args
	}
	switch first {
	case "list", "get", "status":
		return "list", args[1:]
	case "add", "create", "send", "notify":
		return "add", args[1:]
	case "delete", "remove", "rm":
		return "delete", args[1:]
	case "clear":
		return "clear", args[1:]
	default:
		return "", args
	}
}

func normalizeLauncherReminderAction(raw string) (string, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "list", "get", "status":
		return "list", true
	case "add", "create", "send", "notify":
		return "add", true
	case "delete", "remove", "rm":
		return "delete", true
	case "clear":
		return "clear", true
	default:
		return "", false
	}
}

func printIPCStdout(resp *ipcResponse) (bool, int) {
	if resp == nil || resp.Data == nil {
		return false, 0
	}
	stdout, ok := resp.Data["__stdout"]
	if !ok {
		return false, 0
	}
	fmt.Print(stdout)
	exitCode := 0
	if raw := strings.TrimSpace(resp.Data["__exit_code"]); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			exitCode = parsed
		}
	}
	return true, exitCode
}

// parseIPCArgs converts CLI args to IPC params (mirrors ipcFallback in cmd/blue).
func parseIPCArgs(args []string) map[string]string {
	params := make(map[string]string)
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--") || strings.HasPrefix(a, "-") {
			key := strings.TrimLeft(a, "-")
			if isBooleanIPCFlag(key) {
				params[key] = "true"
			} else if key != "" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				params[key] = args[i+1]
				i++
			} else if key != "" {
				params[key] = "true"
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

func isBooleanIPCFlag(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "active", "clear", "fix", "follow", "full", "list", "poll", "silent", "verbose", "wait":
		return true
	default:
		return false
	}
}

// dialSock connects to the blue IPC socket.
func dialSock(flags cliFlags) (net.Conn, error) {
	paths := candidateLauncherIPCSocketPaths(flags)

	var lastErr error
	for _, p := range paths {
		network := "unix"
		if runtime.GOOS == "windows" && strings.HasPrefix(p, `\\.\pipe\`) {
			network = "unix"
		}
		conn, err := net.DialTimeout(network, p, 3*time.Second)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func candidateLauncherIPCSocketPaths(flags cliFlags) []string {
	if sockPath := strings.TrimSpace(os.Getenv("BLUE_IPC_SOCKET")); sockPath != "" {
		return []string{sockPath}
	}

	paths := []string{
		filepath.Join(getDataDir(flags), "blue.sock"),
		"/tmp/blue.sock",
	}
	if runtime.GOOS == "windows" {
		paths = append([]string{`\\.\pipe\blue`}, paths...)
	}
	return dedupeLauncherSocketPaths(paths)
}

func dedupeLauncherSocketPaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		seen := false
		for _, existing := range result {
			if existing == path {
				seen = true
				break
			}
		}
		if !seen {
			result = append(result, path)
		}
	}
	return result
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
