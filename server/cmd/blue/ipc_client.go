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
	cardconv "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cards"
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
	params, positional := parseIPCArgs(args)

	// If there are positional args (not key=value), join them as "query" param.
	// This supports `blue web_search some query` → params["query"] = "some query"
	if len(positional) > 0 && len(params) == 0 {
		params["query"] = strings.Join(positional, " ")
	}
	injectIPCContextParams(params, os.Getenv)

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

func injectIPCContextParams(params map[string]string, getenv func(string) string) {
	if params == nil || getenv == nil {
		return
	}
	// Use reserved internal key to avoid colliding with skill arguments.
	if _, exists := params["__blue_user_id"]; exists {
		return
	}
	if userID := strings.TrimSpace(getenv("BLUE_USER_ID")); userID != "" {
		params["__blue_user_id"] = userID
	}
}

func parseIPCArgs(args []string) (map[string]string, []string) {
	params := make(map[string]string)
	var positional []string

	// Parse args: collect --key value pairs, key=value pairs, and positional args
	for i := 0; i < len(args); i++ {
		a := args[i]
		// --key value or -key value
		if strings.HasPrefix(a, "--") || strings.HasPrefix(a, "-") {
			key := strings.TrimLeft(a, "-")
			if key != "" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				appendIPCParam(params, key, args[i+1])
				i++ // consume next arg as value
			}
			continue
		}
		// key=value
		if idx := strings.IndexByte(a, '='); idx >= 0 {
			appendIPCParam(params, a[:idx], a[idx+1:])
		} else {
			positional = append(positional, a)
		}
	}
	return params, positional
}

func appendIPCParam(params map[string]string, key, value string) {
	if isListParamKey(key) {
		params[key] = appendListParamValue(params[key], value)
	} else {
		params[key] = value
	}
}

func isListParamKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "option", "options", "a":
		return true
	default:
		return false
	}
}

func appendListParamValue(existing, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return existing
	}
	if existing == "" {
		return value
	}

	items := make([]string, 0, 4)
	trimmedExisting := strings.TrimSpace(existing)
	if strings.HasPrefix(trimmedExisting, "[") && strings.HasSuffix(trimmedExisting, "]") {
		if err := json.Unmarshal([]byte(trimmedExisting), &items); err != nil {
			items = []string{existing}
		}
	} else {
		items = []string{existing}
	}
	items = append(items, value)

	encoded, err := json.Marshal(items)
	if err != nil {
		return existing + "," + value
	}
	return string(encoded)
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

	payload := parseIPCResponsePayload(resp.Data)
	if len(payload) == 0 {
		return false
	}

	// If the handler set a _card hint, use it as the card type and pass
	// the data through (e.g. ui_reviewer returns _card=ui_reviewer).
	if hint, ok := resp.Data["_card"]; ok && hint != "" {
		if shouldConvertIPCCardHint(hint) {
			// Legacy IPC shape: {"_card":"ui_reviewer","result":"{...json...}"}
			if raw := strings.TrimSpace(resp.Data["result"]); raw != "" {
				if card := cardconv.ToCard(hint, raw); card != nil {
					cardproto.Emit(card)
					return true
				}
			}

			if b, err := json.Marshal(payload); err == nil {
				if card := cardconv.ToCard(hint, string(b)); card != nil {
					cardproto.Emit(card)
					return true
				}
			}
		}

		// Fallback: keep explicit card hint unchanged (e.g. search).
		payload["type"] = hint
		cardproto.Emit(payload)
		return true
	}

	// No explicit hint: infer card by command name if possible.
	if b, err := json.Marshal(payload); err == nil {
		if card := cardconv.ToCard(cmd, string(b)); card != nil {
			cardproto.Emit(card)
			return true
		}
	}

	// Generic result fallback.
	details := make([]map[string]interface{}, 0, len(payload))
	for k, v := range payload {
		details = append(details, map[string]interface{}{
			"label": k,
			"value": v,
		})
	}
	cardproto.Emit(map[string]interface{}{
		"type":    "result",
		"status":  "success",
		"title":   strings.ReplaceAll(cmd, ".", " "),
		"details": details,
	})
	return true
}

func parseIPCResponsePayload(data map[string]string) map[string]interface{} {
	payload := make(map[string]interface{}, len(data))
	for k, v := range data {
		if k == "_card" || k == "success" {
			continue
		}

		var parsed interface{}
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			payload[k] = parsed
			continue
		}
		payload[k] = v
	}
	return payload
}

func shouldConvertIPCCardHint(hint string) bool {
	switch strings.TrimSpace(hint) {
	case "ui_reviewer", "deep_research", "deep-research", "analyze":
		return true
	default:
		return false
	}
}
