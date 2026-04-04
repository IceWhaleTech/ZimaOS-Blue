package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cardproto"
	cardconv "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cards"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

var (
	ipcRoundTripFunc = ipcRoundTrip
	ipcExit          = os.Exit
)

const (
	defaultIPCRequestTimeout = 2 * time.Minute
	analyzeIPCRequestTimeout = 10 * time.Minute
)

// dialSock connects to the blue IPC socket.
func dialSock() (net.Conn, error) {
	paths := candidateIPCSocketPaths()
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

	// Keep the client-side deadline aligned with the server-side skill fallback
	// timeout so long-running skills do not fail locally before the gateway does.
	conn.SetDeadline(time.Now().Add(ipcRequestTimeout(req)))

	if err := sockipc.WriteJSON(conn, req); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	resp, err := sockipc.ReadJSON[sockipc.Response](conn)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return resp, nil
}

func ipcRequestTimeout(req *sockipc.Request) time.Duration {
	cmd := ""
	if req != nil {
		cmd = strings.ToLower(strings.TrimSpace(req.Cmd))
	}
	switch {
	case cmd == "analyze" || strings.HasPrefix(cmd, "analyze."):
		return analyzeIPCRequestTimeout
	default:
		return defaultIPCRequestTimeout
	}
}

// ipcFallback forwards an unrecognized CLI command as an IPC request.
// Returns true if handled (even on error), false if server not reachable.
func ipcFallback(cmd string, args []string) bool {
	return ipcDispatchCommand(cmd, args, false)
}

func ipcDispatchCommand(cmd string, args []string, requireServer bool) bool {
	cmd, params, positional := normalizeIPCCommand(cmd, args)

	// If there are positional args (not key=value), join them as "query" param.
	// This supports `blue web_search some query` → params["query"] = "some query"
	if len(positional) > 0 && len(params) == 0 {
		params["query"] = strings.Join(positional, " ")
	}
	params = prepareIPCParams(params)
	return ipcDispatchPreparedCommand(cmd, params, requireServer)
}

func prepareIPCParams(params map[string]string) map[string]string {
	if params == nil {
		params = make(map[string]string)
	}
	injectIPCContextParams(params, os.Getenv)
	injectIPCWorkdirParam(params, os.Getwd)
	injectIPCOutputParams(params)
	return params
}

func ipcDispatchPreparedCommand(cmd string, params map[string]string, requireServer bool) bool {
	req := &sockipc.Request{Cmd: cmd, Params: params}
	resp, err := ipcRoundTripFunc(req)
	if err != nil {
		// If we can't connect, let cobra handle it (might be a typo)
		if isConnectionError(err) {
			if requireServer {
				if jsonOutput {
					printJSON(map[string]string{"error": fmt.Sprintf("running Blue service is required for %s: %v", cmd, err)})
				} else {
					fmt.Fprintf(os.Stdout, "Error: running Blue service is required for %s: %v\n", cmd, err)
				}
				ipcExit(1)
			}
			return false
		}
		// Print error to stdout so exec tool can capture it as tool result.
		fmt.Fprintf(os.Stdout, "Error: %v\n", err)
		ipcExit(1)
	}

	if resp.Status != "ok" {
		// Print error to stdout so exec tool can capture it.
		if jsonOutput {
			printJSON(map[string]string{"error": resp.Error})
		} else {
			fmt.Fprintf(os.Stdout, "Error: %s\n", resp.Error)
		}
		ipcExit(1)
	}

	if printed, exitCode := printIPCStdout(resp); printed {
		if exitCode != 0 {
			ipcExit(exitCode)
		}
		return true
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
			if strings.HasPrefix(k, "__") || k == "_card" || k == "success" {
				continue
			}
			fmt.Printf("%s: %s\n", k, v)
		}
	}
	return true
}

func normalizeIPCCommand(cmd string, args []string) (string, map[string]string, []string) {
	params, positional := parseIPCArgs(args)
	trimmedCmd := strings.TrimSpace(cmd)
	if strings.HasPrefix(trimmedCmd, "browser.") {
		return normalizeBrowserDottedIPCCommand(trimmedCmd, params, positional)
	}
	switch trimmedCmd {
	case "context":
		return normalizeContextIPCCommand(params, positional)
	case "browser":
		return normalizeBrowserIPCCommand(params, positional)
	case "reminder":
		return normalizeReminderIPCCommand(params, positional)
	case "/install":
		return normalizeSlashSkillCommand("skill.install", "id", params, positional)
	case "/install-url", "/install_url":
		positional = promotePositionalIPCParam(params, positional, "url")
		positional = promotePositionalIPCParam(params, positional, "name")
		return "skill.install_url", params, positional
	case "/uninstall":
		return normalizeSlashSkillCommand("skill.uninstall", "id", params, positional)
	case "/update":
		return normalizeSlashSkillCommand("skill.update", "id", params, positional)
	case "/enable":
		return normalizeSlashSkillCommand("skill.enable", "id", params, positional)
	case "/disable":
		return normalizeSlashSkillCommand("skill.disable", "id", params, positional)
	case "/info":
		return normalizeSlashSkillCommand("skill.info", "id", params, positional)
	case "/list":
		return "skill.list", params, positional
	case "/search":
		if strings.TrimSpace(params["query"]) == "" && len(positional) > 0 {
			params["query"] = strings.Join(positional, " ")
			positional = nil
		}
		return "skill.search", params, positional
	default:
		return cmd, params, positional
	}
}

func normalizeBrowserIPCCommand(params map[string]string, positional []string) (string, map[string]string, []string) {
	return normalizeBrowserIPCAction("", params, positional)
}

func normalizeBrowserDottedIPCCommand(cmd string, params map[string]string, positional []string) (string, map[string]string, []string) {
	action := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(cmd), "browser."))
	return normalizeBrowserIPCAction(action, params, positional)
}

func normalizeBrowserIPCAction(defaultAction string, params map[string]string, positional []string) (string, map[string]string, []string) {
	if params == nil {
		params = make(map[string]string)
	}

	actionRaw := strings.ToLower(strings.TrimSpace(params["action"]))
	if actionRaw == "" {
		actionRaw = strings.ToLower(strings.TrimSpace(defaultAction))
	}
	if actionRaw == "" && len(positional) > 0 {
		first := strings.TrimSpace(positional[0])
		switch {
		case browserPositionalLooksLikeURL(first):
			if strings.TrimSpace(params["url"]) == "" {
				params["url"] = first
			}
			positional = append([]string(nil), positional[1:]...)
			actionRaw = "navigate"
		default:
			mappedAction, ok := normalizeBrowserActionAlias(first)
			if ok {
				actionRaw = mappedAction
				positional = append([]string(nil), positional[1:]...)
			}
		}
	}
	if actionRaw == "" {
		actionRaw = promoteBrowserActionKeyParam(params)
	}

	// Keep bare `blue browser url=...` behavior routed through the skill fallback,
	// where the browser skill can still default url-only input to navigate.
	if actionRaw == "" {
		return "browser", params, positional
	}

	if mappedAction, ok := normalizeBrowserActionAlias(actionRaw); ok {
		actionRaw = mappedAction
	}

	actType := strings.TrimSpace(params["act_type"])
	if actType == "" {
		actType = strings.TrimSpace(params["actType"])
	}
	action, canonicalActType := tools.CanonicalizeBrowserAction(actionRaw, actType)
	delete(params, "action")
	if strings.TrimSpace(params["act_type"]) == "" && canonicalActType != "" {
		params["act_type"] = canonicalActType
	}
	if strings.TrimSpace(params["actType"]) != "" {
		delete(params, "actType")
	}
	positional = normalizeBrowserPositionalArgs(action, params, positional)
	normalizeBrowserIPCRef(params)

	return "browser." + action, params, positional
}

func normalizeBrowserActionAlias(action string) (string, bool) {
	return tools.NormalizeBrowserActionAlias(action)
}

func promoteBrowserActionKeyParam(params map[string]string) string {
	if params == nil {
		return ""
	}

	for _, key := range []string{
		"navigate", "open", "goto", "go", "visit",
		"snapshot", "inspect", "tree",
		"snapshot_interactive", "interactive", "elements",
		"snapshot_auto", "read", "page",
		"act", "click", "type", "focus", "hover", "scroll", "select",
		"screenshot", "shot", "capture", "screen",
		"tabs", "list", "ls", "tab", "status",
		"close", "remove", "rm", "delete",
		"recipe", "run_recipe", "run-recipe",
		"recipes", "list_recipes", "list-recipes",
	} {
		value, ok := params[key]
		if !ok {
			continue
		}
		actionRaw := strings.ToLower(strings.TrimSpace(key))
		mappedAction, ok := normalizeBrowserActionAlias(actionRaw)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch mappedAction {
		case "navigate":
			if strings.TrimSpace(params["url"]) == "" && value != "" {
				params["url"] = value
			}
		case "snapshot", "snapshot_interactive", "snapshot_auto", "close":
			if strings.TrimSpace(params["target_id"]) == "" && value != "" {
				params["target_id"] = value
			}
		case "screenshot":
			if strings.TrimSpace(params["url"]) == "" && strings.TrimSpace(params["target_id"]) == "" && value != "" {
				if browserPositionalLooksLikeURL(value) {
					params["url"] = value
				} else {
					params["target_id"] = value
				}
			}
		case "recipe":
			if strings.TrimSpace(params["recipe"]) == "" && value != "" {
				params["recipe"] = value
			}
		case "act", "click", "type", "focus", "hover", "scroll", "select":
			if strings.TrimSpace(params["ref"]) == "" && value != "" {
				params["ref"] = value
			}
		}
		delete(params, key)
		return actionRaw
	}

	return ""
}

func normalizeBrowserPositionalArgs(action string, params map[string]string, positional []string) []string {
	switch action {
	case "navigate":
		if strings.TrimSpace(params["url"]) == "" {
			positional = promotePositionalIPCParam(params, positional, "url")
		}
	case "snapshot", "snapshot_interactive", "snapshot_auto", "close":
		if strings.TrimSpace(params["target_id"]) == "" {
			positional = promotePositionalIPCParam(params, positional, "target_id")
		}
	case "screenshot":
		if strings.TrimSpace(params["url"]) == "" && strings.TrimSpace(params["target_id"]) == "" && len(positional) > 0 {
			first := strings.TrimSpace(positional[0])
			if browserPositionalLooksLikeURL(first) {
				params["url"] = first
			} else {
				params["target_id"] = first
			}
			positional = append([]string(nil), positional[1:]...)
		}
	case "recipe":
		if strings.TrimSpace(params["recipe"]) == "" {
			positional = promotePositionalIPCParam(params, positional, "recipe")
		}
	case "act":
		if strings.TrimSpace(params["ref"]) == "" {
			positional = promotePositionalIPCParam(params, positional, "ref")
		}
		if strings.TrimSpace(params["act_type"]) == "" && strings.TrimSpace(params["actType"]) == "" {
			positional = promotePositionalIPCParam(params, positional, "act_type")
		}
		if strings.TrimSpace(params["value"]) == "" && len(positional) > 0 {
			params["value"] = strings.Join(positional, " ")
			positional = nil
		}
	}

	return positional
}

func normalizeBrowserIPCRef(params map[string]string) {
	if params == nil {
		return
	}
	ref := strings.TrimSpace(params["ref"])
	if ref == "" {
		return
	}
	ref = strings.TrimPrefix(ref, "@")
	if ref != "" {
		params["ref"] = ref
	}
}

func browserPositionalLooksLikeURL(raw string) bool {
	return tools.LooksLikeBrowserURL(raw)
}

func normalizeReminderIPCCommand(params map[string]string, positional []string) (string, map[string]string, []string) {
	if params == nil {
		params = make(map[string]string)
	}

	// Prefer explicit action=... when present, otherwise accept positional action:
	//   blue reminder add ...
	//   blue reminder list
	//   blue reminder delete <id>
	actionRaw := strings.ToLower(strings.TrimSpace(params["action"]))
	if actionRaw == "" && len(positional) > 0 {
		actionRaw = strings.ToLower(strings.TrimSpace(positional[0]))
		if actionRaw != "" && !strings.HasPrefix(actionRaw, "-") && !strings.Contains(actionRaw, "=") {
			positional = append([]string(nil), positional[1:]...)
		} else {
			actionRaw = ""
		}
	}

	var action string
	switch actionRaw {
	case "list", "get", "status":
		action = "list"
	case "add", "create", "send", "notify":
		action = "add"
	case "delete", "remove", "rm":
		action = "delete"
	case "clear":
		action = "clear"
	case "":
		// No action provided; fall back to legacy dotted commands or skill fallback.
		return "reminder", params, positional
	default:
		// Unknown action token; keep it positional so query mapping can apply if appropriate.
		return "reminder", params, append([]string{actionRaw}, positional...)
	}

	// Optional ergonomic alias: `blue reminder delete <id>` (positional id).
	if action == "delete" && strings.TrimSpace(params["id"]) == "" && len(positional) > 0 {
		if id := strings.TrimSpace(positional[0]); id != "" && !strings.HasPrefix(id, "-") && !strings.Contains(id, "=") {
			params["id"] = id
			positional = positional[1:]
		}
	}

	delete(params, "action")
	return "reminder." + action, params, positional
}

func normalizeContextIPCCommand(params map[string]string, positional []string) (string, map[string]string, []string) {
	if len(positional) == 0 {
		return "context", params, positional
	}

	subcommand := strings.TrimSpace(positional[0])
	positional = append([]string(nil), positional[1:]...)

	switch subcommand {
	case "search":
		if strings.TrimSpace(params["query"]) == "" && len(positional) > 0 {
			params["query"] = strings.Join(positional, " ")
			positional = nil
		}
		return "context.search", params, positional
	case "get":
		positional = promotePositionalIPCParam(params, positional, "id")
		return "context.get", params, positional
	case "annotate":
		positional = promotePositionalIPCParam(params, positional, "id")
		if strings.TrimSpace(params["note"]) == "" && len(positional) > 0 {
			params["note"] = strings.Join(positional, " ")
			positional = nil
		}
		return "context.annotate", params, positional
	case "import":
		positional = promotePositionalIPCParam(params, positional, "path")
		return "context.import", params, positional
	case "validate":
		positional = promotePositionalIPCParam(params, positional, "path")
		return "context.validate", params, positional
	default:
		return "context", params, append([]string{subcommand}, positional...)
	}
}

func normalizeSlashSkillCommand(cmd, key string, params map[string]string, positional []string) (string, map[string]string, []string) {
	if strings.TrimSpace(params[key]) == "" {
		positional = promotePositionalIPCParam(params, positional, key)
	}
	return cmd, params, positional
}

func promotePositionalIPCParam(params map[string]string, positional []string, key string) []string {
	if len(positional) == 0 {
		return positional
	}
	params[key] = positional[0]
	return append([]string(nil), positional[1:]...)
}

func injectIPCContextParams(params map[string]string, getenv func(string) string) {
	if params == nil || getenv == nil {
		return
	}
	// Use reserved internal key to avoid colliding with skill arguments.
	if _, exists := params["__blue_user_id"]; exists {
		// keep explicit value
	} else if userID := strings.TrimSpace(getenv("BLUE_USER_ID")); userID != "" {
		params["__blue_user_id"] = userID
	}
	// session_id routes reminders and other session-scoped skill outputs back
	// to the originating conversation when commands go through IPC fallback.
	if _, exists := params["session_id"]; !exists {
		if sessionID := strings.TrimSpace(getenv("BLUE_SESSION_ID")); sessionID != "" {
			params["session_id"] = sessionID
		}
	}
}

func injectIPCWorkdirParam(params map[string]string, getwd func() (string, error)) {
	if params == nil || getwd == nil {
		return
	}
	if _, exists := params["__blue_workdir"]; exists {
		return
	}
	workdir, err := getwd()
	if err != nil {
		return
	}
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		return
	}
	params["__blue_workdir"] = workdir
}

func injectIPCOutputParams(params map[string]string) {
	if params == nil {
		return
	}
	params["__blue_json"] = strconv.FormatBool(jsonOutput)
	params["__blue_no_color"] = strconv.FormatBool(noColor)
	params["__blue_verbose"] = strconv.FormatBool(verbose)
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
			if isBooleanIPCFlag(key) {
				appendIPCParam(params, key, "true")
			} else if key != "" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				appendIPCParam(params, key, args[i+1])
				i++ // consume next arg as value
			} else if key != "" {
				appendIPCParam(params, key, "true")
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

func isBooleanIPCFlag(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "active", "clear", "fix", "follow", "full", "list", "poll", "silent", "verbose", "wait":
		return true
	default:
		return false
	}
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

func printIPCStdout(resp *sockipc.Response) (bool, int) {
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
		if strings.HasPrefix(k, "__") || k == "_card" || k == "success" {
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
