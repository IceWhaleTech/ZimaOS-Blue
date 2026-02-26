package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cardproto"
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

	// Set read deadline so we don't hang if the server is slow.
	// The exec tool's default timeout is 30s; leave headroom for the response.
	conn.SetDeadline(time.Now().Add(25 * time.Second))

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

	// Parse args: collect --key value pairs, key=value pairs, and positional args
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		// --key value or -key value → params[key] = value
		if strings.HasPrefix(a, "--") || strings.HasPrefix(a, "-") {
			key := strings.TrimLeft(a, "-")
			if key != "" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				params[key] = args[i+1]
				i++ // consume next arg as value
			}
			continue
		}
		// key=value
		if idx := strings.IndexByte(a, '='); idx >= 0 {
			params[a[:idx]] = a[idx+1:]
		} else {
			positional = append(positional, a)
		}
	}

	// If there are positional args (not key=value), join them as "query" param.
	// This supports `blue web_search some query` → params["query"] = "some query"
	if len(positional) > 0 && len(params) == 0 {
		params["query"] = strings.Join(positional, " ")
	}

	req := &sockipc.Request{Cmd: cmd, Params: params}
	resp, err := ipcRoundTrip(req)
	if err != nil {
		// If we can't connect, let cobra handle it (might be a typo)
		if isConnectionError(err) {
			return false
		}
		// Print error to stdout so exec tool can capture it as tool result.
		fmt.Fprintf(os.Stdout, "Error: %v\n", err)
		os.Exit(1)
	}

	if resp.Status != "ok" {
		// Print error to stdout so exec tool can capture it.
		fmt.Fprintf(os.Stdout, "Error: %s\n", resp.Error)
		os.Exit(1)
	}

	// Emit a typeless card via __CARD__ protocol so the exec tool's
	// stdout scanner can extract it and push it to the SSE stream.
	emitIPCCard(cmd, resp)

	if jsonOutput {
		printJSON(resp)
	} else if resp.Data != nil {
		// Always print data to stdout so the exec tool captures it as
		// the tool result text that the LLM can read. Cards are extracted
		// separately for the UI — the LLM only sees plain stdout.
		for k, v := range resp.Data {
			// Skip internal hints — not useful for the LLM.
			if k == "_card" || k == "success" {
				continue
			}
			fmt.Printf("%s: %s\n", k, v)
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

// emitIPCCard converts an IPC response into a __CARD__ line on stdout.
// The exec tool's readIntoBufferWithCards extracts these and pushes them
// to the SSE stream as streaming typeless cards. Returns true if a card
// was emitted.
func emitIPCCard(cmd string, resp *sockipc.Response) bool {
	if resp.Data == nil || len(resp.Data) == 0 {
		return false
	}

	// If the handler set a _card hint, use it as the card type and pass
	// the data through (e.g. ui_reviewer returns _card=ui_reviewer).
	if hint, ok := resp.Data["_card"]; ok && hint != "" {
		card := make(map[string]interface{}, len(resp.Data))
		for k, v := range resp.Data {
			if k == "_card" || k == "success" {
				continue
			}
			// Try to recover structured data that was JSON-serialized
			// through the map[string]string IPC transport.
			if len(v) > 0 && (v[0] == '[' || v[0] == '{') {
				var parsed interface{}
				if json.Unmarshal([]byte(v), &parsed) == nil {
					card[k] = parsed
					continue
				}
			}
			card[k] = v
		}
		card["type"] = hint
		cardproto.Emit(card)
		return true
	}

	// Build a generic result card from the IPC response.
	details := make([]map[string]interface{}, 0, len(resp.Data))
	for k, v := range resp.Data {
		details = append(details, map[string]interface{}{
			"label": k,
			"value": v,
		})
	}

	// Use the IPC command name (e.g. "browser.navigate") as the title,
	// replacing dots with spaces for readability.
	title := strings.ReplaceAll(cmd, ".", " ")

	cardproto.Emit(map[string]interface{}{
		"type":    "result",
		"status":  "success",
		"title":   title,
		"details": details,
	})
	return true
}

