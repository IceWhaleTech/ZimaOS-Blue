package tools

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

var factoryToolNames = []string{
	"browser",
	"canvas",
	"nodes",
	"cron",
	"message",
	"tts",
	"gateway",
	"agents_list",
	"sessions_list",
	"sessions_history",
	"sessions_send",
	"sessions_spawn",
	"subagents",
	"session_status",
	"memory_search",
	"memory_get",
	"memory_write",
	"memory_forget",
	"web_search",
	"web_fetch",
	"image",
	"pdf",
}

var factoryToolNameSet = buildFactoryToolNameSet(factoryToolNames)

var unsupportedFactoryToolHints = map[string]string{
	"agents_list": "agents_list is not enabled in this runtime yet.",
	"subagents":   "subagents is not enabled in this runtime yet.",
	"pdf":         "pdf is not enabled in this runtime yet.",
}

func buildFactoryToolNameSet(names []string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = normalizeFactoryToolName(name)
		if name == "" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}

func normalizeFactoryToolName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// RegisterFactoryToolDefinitions exposes factory tool names to the model.
func RegisterFactoryToolDefinitions(registry *Registry) {
	if registry == nil {
		return
	}
	for _, def := range factoryToolDefinitions() {
		registry.ExposeDefinition(def)
	}
}

func factoryToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		factoryToolDefinition("browser", "Automate browser actions and page interactions."),
		factoryToolDefinition("canvas", "Create or update canvas-style structured artifacts."),
		factoryToolDefinition("nodes", "Manage node/graph-style workflow structures."),
		factoryToolDefinition("cron", "Create and manage scheduled jobs."),
		factoryToolDefinition("message", "Send outbound messages through connected channels."),
		factoryToolDefinition("tts", "Generate text-to-speech output."),
		factoryToolDefinition("gateway", "Read or manage gateway state and routes."),
		factoryToolDefinition("agents_list", "List available agents."),
		factoryToolDefinition("sessions_list", "List recent sessions."),
		factoryToolDefinition("sessions_history", "Read session history."),
		factoryToolDefinition("sessions_send", "Send a message to an existing session."),
		factoryToolDefinition("sessions_spawn", "Spawn a new session."),
		factoryToolDefinition("subagents", "Inspect or manage subagents."),
		factoryToolDefinition("session_status", "Get status for a session."),
		factoryToolDefinition("memory_search", "Search memory snippets by query."),
		factoryToolDefinition("memory_get", "Read a memory entry by ID or path."),
		factoryToolDefinition("memory_write", "Write a new memory entry."),
		factoryToolDefinition("memory_forget", "Delete a memory entry by ID."),
		factoryToolDefinition("web_search", "Search the web for up-to-date information."),
		factoryToolDefinition("web_fetch", "Fetch and parse a web page by URL."),
		factoryToolDefinition("image", "Analyze or process image inputs."),
		factoryToolDefinition("pdf", "Read and extract information from PDF files."),
	}
}

func factoryToolDefinition(name, desc string) ToolDefinition {
	return ToolDefinition{
		Name:        name,
		Description: desc,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Optional plain-text input.",
				},
			},
			"additionalProperties": true,
		},
	}
}

func isFactoryToolName(name string) bool {
	_, ok := factoryToolNameSet[normalizeFactoryToolName(name)]
	return ok
}

func unsupportedFactoryToolResult(name string) (string, bool) {
	key := normalizeFactoryToolName(name)
	hint, ok := unsupportedFactoryToolHints[key]
	if !ok {
		return "", false
	}
	payload := map[string]interface{}{
		"error": "tool is not available in this Blue runtime",
		"code":  "tool_unavailable",
		"tool":  key,
		"hint":  hint,
	}
	b, _ := json.Marshal(payload)
	return string(b), true
}

func invalidFactoryToolArgsResult(name, hint string) string {
	payload := map[string]interface{}{
		"error": "invalid arguments",
		"code":  "invalid_arguments",
		"tool":  normalizeFactoryToolName(name),
		"hint":  hint,
	}
	b, _ := json.Marshal(payload)
	return string(b)
}

func resolveFactoryAliasCommand(name string, args map[string]interface{}) (string, bool, string) {
	key := normalizeFactoryToolName(name)
	switch key {
	case "canvas", "nodes":
		return resolveFactoryWorkflowAliasCommand(name, args)
	case "tts":
		return resolveFactoryTTSAliasCommand(name, args)
	case "sessions_list":
		cmd := "blue sessions list"
		if active, ok := asCompatBool(args["active"]); ok && active {
			cmd += " --active"
		}
		return cmd, true, ""
	case "sessions_history":
		id := firstCompatString(args, "id", "session_id", "session", "conversation_id")
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/session_id is required")
		}
		return "blue sessions show " + quoteCompatShellArg(id), true, ""
	case "sessions_send":
		return resolveFactorySessionsSendAliasCommand(name, args)
	case "sessions_spawn":
		return resolveFactorySessionsSpawnAliasCommand(name, args)
	case "session_status":
		if id := firstCompatString(args, "id", "session_id", "session", "conversation_id"); id != "" {
			return "blue sessions show " + quoteCompatShellArg(id), true, ""
		}
		return "blue status", true, ""
	case "memory_search", "memory_get", "memory_write", "memory_forget":
		return resolveFactoryMemoryAliasCommand(name, args)
	case "gateway":
		action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
		switch action {
		case "", "status", "list", "get":
			return "blue gateway status", true, ""
		case "start", "stop", "restart", "install", "uninstall", "run":
			return "blue gateway " + action, true, ""
		default:
			return "", true, invalidFactoryToolArgsResult(name, "action must be one of status/start/stop/restart/install/uninstall/run")
		}
	case "cron":
		return resolveFactoryCronAliasCommand(name, args)
	case "message":
		return resolveFactoryMessageAliasCommand(name, args)
	case "image":
		return resolveFactoryImageAliasCommand(name, args)
	default:
		return "", false, ""
	}
}

func resolveFactorySessionsSpawnAliasCommand(name string, args map[string]interface{}) (string, bool, string) {
	title := firstCompatString(args, "title", "name")
	if title == "" {
		title = trimCompatText(firstCompatString(args, "input", "prompt", "message", "content", "text"), 72)
	}
	if title == "" {
		title = "New Conversation"
	}
	payload := map[string]interface{}{"title": title}
	return buildCompatHTTPJSONCommand("POST", "/api/v1/conversations", payload), true, ""
}

func resolveFactorySessionsSendAliasCommand(name string, args map[string]interface{}) (string, bool, string) {
	id := firstCompatString(args, "id", "session_id", "session", "conversation_id")
	if id == "" {
		return "", true, invalidFactoryToolArgsResult(name, "id/session_id is required")
	}
	message := firstCompatString(args, "message", "content", "text", "input", "prompt")
	if message == "" {
		return "", true, invalidFactoryToolArgsResult(name, "message/content is required")
	}

	payload := map[string]interface{}{
		"message": message,
	}
	if provider := firstCompatString(args, "provider"); provider != "" {
		payload["provider"] = provider
	}
	if model := firstCompatString(args, "model"); model != "" {
		payload["model"] = model
	}
	if regenerate, ok := asCompatBool(args["regenerate"]); ok {
		payload["regenerate"] = regenerate
	}

	path := "/api/v1/conversations/" + url.PathEscape(id) + "/messages"
	return buildCompatHTTPJSONCommand("POST", path, payload), true, ""
}

func resolveFactoryMemoryAliasCommand(name string, args map[string]interface{}) (string, bool, string) {
	switch normalizeFactoryToolName(name) {
	case "memory_search":
		query := firstCompatString(args, "query", "q", "search", "vsearch", "text", "input", "content")
		if query == "" {
			return "", true, invalidFactoryToolArgsResult(name, "query is required for memory_search")
		}
		payload := map[string]interface{}{"query": query}
		if limit, ok := coerceCompatInt(args["limit"]); ok && limit > 0 {
			payload["limit"] = limit
		}
		return buildCompatHTTPJSONCommand("POST", "/api/v1/memory/search", payload), true, ""
	case "memory_get":
		id := firstCompatString(args, "id", "path", "file", "memory_id", "memoryId", "key")
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/path is required for memory_get")
		}
		return buildCompatHTTPCommand("GET", "/api/v1/memory/"+url.PathEscape(id)), true, ""
	case "memory_write":
		content := firstCompatString(args, "content", "text", "input", "memory", "note", "message")
		if content == "" {
			return "", true, invalidFactoryToolArgsResult(name, "content/text is required for memory_write")
		}
		payload := map[string]interface{}{"content": content}
		if tags, ok := coerceCompatStringList(args["tags"]); ok && len(tags) > 0 {
			payload["tags"] = tags
		} else if category := firstCompatString(args, "category", "tag"); category != "" {
			payload["tags"] = []interface{}{category}
		}
		return buildCompatHTTPJSONCommand("POST", "/api/v1/memory/store", payload), true, ""
	case "memory_forget":
		id := firstCompatString(args, "id", "path", "file", "memory_id", "memoryId", "key")
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/path is required for memory_forget")
		}
		return buildCompatHTTPCommand("DELETE", "/api/v1/memory/"+url.PathEscape(id)), true, ""
	default:
		return "", false, ""
	}
}

func resolveFactoryImageAliasCommand(name string, args map[string]interface{}) (string, bool, string) {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	taskID := firstCompatString(args, "task_id", "id")
	prompt := firstCompatString(args, "prompt", "query", "input", "text", "message", "content")
	sourceURL := firstCompatString(args, "url", "href", "source", "link")
	inlineImage := firstCompatString(args, "image", "image_base64", "base64")

	if action == "" {
		switch {
		case taskID != "":
			action = "status"
		case inlineImage != "" || sourceURL != "":
			action = "review"
		default:
			action = "generate"
		}
	}

	switch action {
	case "status", "get":
		if taskID == "" {
			return "", true, invalidFactoryToolArgsResult(name, "task_id/id is required for status")
		}
		return "blue media status " + quoteCompatShellArg(taskID), true, ""
	case "review", "analyze":
		if inlineImage != "" {
			reviewArgs := map[string]interface{}{"image": inlineImage}
			if lang := firstCompatString(args, "lang", "language"); lang != "" {
				reviewArgs["lang"] = lang
			}
			return buildSkillCommand("ui.review_image", reviewArgs), true, ""
		}
		if sourceURL != "" {
			reviewArgs := map[string]interface{}{"url": sourceURL}
			if lang := firstCompatString(args, "lang", "language"); lang != "" {
				reviewArgs["lang"] = lang
			}
			if device := firstCompatString(args, "device"); device != "" {
				reviewArgs["device"] = device
			}
			return buildSkillCommand("ui.review_url", reviewArgs), true, ""
		}
		return "", true, invalidFactoryToolArgsResult(name, "image/base64 or url is required for review")
	case "generate", "create", "draw":
		if prompt == "" {
			return "", true, invalidFactoryToolArgsResult(name, "prompt/query is required for image generation")
		}
		return buildFactoryMediaGenerateCommand(prompt, args), true, ""
	default:
		return "", true, invalidFactoryToolArgsResult(name, "unknown image action")
	}
}

func resolveFactoryTTSAliasCommand(name string, args map[string]interface{}) (string, bool, string) {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	if action == "" {
		if firstCompatString(args, "text", "input", "content", "message", "prompt") != "" {
			action = "synthesize"
		} else {
			action = "status"
		}
	}

	switch action {
	case "speak", "say", "synthesize", "generate":
		text := firstCompatString(args, "text", "input", "content", "message", "prompt")
		if text == "" {
			return "", true, invalidFactoryToolArgsResult(name, "text/input is required for synthesize")
		}
		payload := map[string]interface{}{
			"text": text,
		}
		if format := firstCompatString(args, "format", "audio_format"); format != "" {
			payload["format"] = format
		}
		return buildCompatHTTPJSONCommand("POST", "/api/v1/voice/synthesize", payload), true, ""
	case "voices", "list":
		return buildCompatHTTPCommand("GET", "/api/v1/voice/voices"), true, ""
	case "status", "get":
		return buildCompatHTTPCommand("GET", "/api/v1/speech/status"), true, ""
	case "stop", "cancel":
		return buildCompatHTTPJSONCommand("POST", "/api/v1/voice/synthesize/stop", map[string]interface{}{}), true, ""
	case "config":
		return buildCompatHTTPCommand("GET", "/api/v1/speech/tts/config"), true, ""
	default:
		return "", true, invalidFactoryToolArgsResult(name, "unknown tts action")
	}
}

func resolveFactoryWorkflowAliasCommand(name string, args map[string]interface{}) (string, bool, string) {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	id := firstCompatString(args, "id", "workflow_id", "canvas_id", "node_id")

	if action == "" {
		switch {
		case id != "" && hasCompatWorkflowBodyField(args):
			action = "update"
		case id != "":
			action = "get"
		case firstCompatString(args, "name", "title") != "" || args["nodes"] != nil || args["connections"] != nil:
			action = "create"
		default:
			action = "list"
		}
	}

	switch action {
	case "list", "ls", "status":
		return buildCompatHTTPCommand("GET", buildCompatWorkflowListPath(args)), true, ""
	case "templates":
		return buildCompatHTTPCommand("GET", "/api/v1/workflows/templates"), true, ""
	case "get", "show", "read":
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/workflow_id is required for get/show")
		}
		return buildCompatHTTPCommand("GET", "/api/v1/workflows/"+url.PathEscape(id)), true, ""
	case "create", "add":
		workflowName := firstCompatString(args, "name", "title")
		if workflowName == "" {
			workflowName = "Untitled Workflow"
		}
		payload := map[string]interface{}{
			"name": workflowName,
		}
		addCompatWorkflowBodyFields(payload, args)
		return buildCompatHTTPJSONCommand("POST", "/api/v1/workflows", payload), true, ""
	case "update", "edit", "patch":
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/workflow_id is required for update")
		}
		payload := make(map[string]interface{})
		addCompatWorkflowBodyFields(payload, args)
		if len(payload) == 0 {
			return "", true, invalidFactoryToolArgsResult(name, "update requires at least one mutable field")
		}
		return buildCompatHTTPJSONCommand("PUT", "/api/v1/workflows/"+url.PathEscape(id), payload), true, ""
	case "delete", "remove", "rm":
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/workflow_id is required for delete/remove")
		}
		return buildCompatHTTPCommand("DELETE", "/api/v1/workflows/"+url.PathEscape(id)), true, ""
	case "run", "execute", "trigger":
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/workflow_id is required for run/execute")
		}
		payload := map[string]interface{}{}
		if v, ok := firstCompatValue(args, "trigger_data", "data", "payload"); ok {
			if normalized := normalizeCompatRawValue(v); normalized != nil {
				payload["trigger_data"] = normalized
			}
		}
		return buildCompatHTTPJSONCommand("POST", "/api/v1/workflows/"+url.PathEscape(id)+"/execute", payload), true, ""
	default:
		return "", true, invalidFactoryToolArgsResult(name, "unknown workflow action")
	}
}

func addCompatWorkflowBodyFields(body map[string]interface{}, args map[string]interface{}) {
	if body == nil {
		return
	}
	appendCompatWorkflowBodyValue(body, "name", args["name"])
	appendCompatWorkflowBodyValue(body, "description", args["description"])
	appendCompatWorkflowBodyValue(body, "status", args["status"])
	appendCompatWorkflowBodyValue(body, "nodes", args["nodes"])
	appendCompatWorkflowBodyValue(body, "connections", args["connections"])
	appendCompatWorkflowBodyValue(body, "variables", args["variables"])
	appendCompatWorkflowBodyValue(body, "settings", args["settings"])
	appendCompatWorkflowBodyValue(body, "tags", args["tags"])
}

func hasCompatWorkflowBodyField(args map[string]interface{}) bool {
	for _, key := range []string{"name", "description", "status", "nodes", "connections", "variables", "settings", "tags"} {
		if v, ok := args[key]; ok && normalizeCompatRawValue(v) != nil {
			return true
		}
	}
	return false
}

func appendCompatWorkflowBodyValue(body map[string]interface{}, key string, value interface{}) bool {
	normalized := normalizeCompatRawValue(value)
	if normalized == nil {
		return false
	}
	if s, ok := normalized.(string); ok && strings.TrimSpace(s) == "" {
		return false
	}
	body[key] = normalized
	return true
}

func buildCompatWorkflowListPath(args map[string]interface{}) string {
	values := url.Values{}
	if limit := firstCompatString(args, "limit"); limit != "" {
		values.Set("limit", limit)
	}
	if offset := firstCompatString(args, "offset"); offset != "" {
		values.Set("offset", offset)
	}
	if status := firstCompatString(args, "status"); status != "" {
		values.Set("status", status)
	}
	if name := firstCompatString(args, "name", "query", "q"); name != "" {
		values.Set("name", name)
	}
	path := "/api/v1/workflows"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

func buildCompatHTTPJSONCommand(method, path string, payload map[string]interface{}) string {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	body, _ := json.Marshal(payload)
	return buildCompatHTTPCommandWithPayload(method, path, string(body))
}

func buildCompatHTTPCommand(method, path string) string {
	return buildCompatHTTPCommandWithPayload(method, path, "")
}

func buildCompatHTTPCommandWithPayload(method, path, payload string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "GET"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	build := func(base string) string {
		var cmd strings.Builder
		cmd.WriteString("curl -sS --fail -X ")
		cmd.WriteString(method)
		if strings.TrimSpace(payload) != "" {
			cmd.WriteString(" -H \"Content-Type: application/json\" --data ")
			cmd.WriteString(quoteCompatShellArg(payload))
		}
		cmd.WriteByte(' ')
		cmd.WriteString(quoteCompatShellArg(base + path))
		return cmd.String()
	}

	primary := build("http://127.0.0.1:8080")
	fallback := build("http://127.0.0.1:8081")
	return "(" + primary + " || " + fallback + ")"
}

func buildFactoryMediaGenerateCommand(prompt string, args map[string]interface{}) string {
	var cmd strings.Builder
	cmd.WriteString("blue media generate ")
	cmd.WriteString(quoteCompatShellArg(prompt))

	category := firstCompatString(args, "category", "mode", "type")
	if category == "" {
		category = "t2i"
	}
	cmd.WriteString(" --category ")
	cmd.WriteString(quoteCompatShellArg(category))

	if model := firstCompatString(args, "model"); model != "" {
		cmd.WriteString(" --model ")
		cmd.WriteString(quoteCompatShellArg(model))
	}
	if size := firstCompatString(args, "size"); size != "" {
		cmd.WriteString(" --size ")
		cmd.WriteString(quoteCompatShellArg(size))
	}
	if poll, ok := asCompatBool(args["poll"]); ok && poll {
		cmd.WriteString(" --poll")
	}
	return cmd.String()
}

func firstCompatValue(args map[string]interface{}, keys ...string) (interface{}, bool) {
	for _, key := range keys {
		v, ok := args[key]
		if !ok || v == nil {
			continue
		}
		return v, true
	}
	return nil, false
}

func normalizeCompatRawValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch typed := v.(type) {
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return nil
		}
		var parsed interface{}
		if json.Unmarshal([]byte(raw), &parsed) == nil {
			return parsed
		}
		return raw
	default:
		return typed
	}
}

func trimCompatText(raw string, maxRunes int) string {
	text := strings.TrimSpace(raw)
	if text == "" || maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return strings.TrimSpace(string(runes[:maxRunes]))
}

func resolveFactoryCronAliasCommand(name string, args map[string]interface{}) (string, bool, string) {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	switch action {
	case "", "list", "ls":
		return "blue cron list", true, ""
	case "status":
		return "blue cron status", true, ""
	case "create", "add":
		jobName := firstCompatString(args, "name", "title")
		schedule := firstCompatString(args, "schedule", "cron", "expression")
		if jobName == "" || schedule == "" {
			return "", true, invalidFactoryToolArgsResult(name, "name and schedule are required for create/add")
		}
		handler := firstCompatString(args, "handler")
		if handler == "" {
			handler = "command"
		}
		var cmd strings.Builder
		cmd.WriteString("blue cron add")
		cmd.WriteString(" --name ")
		cmd.WriteString(quoteCompatShellArg(jobName))
		cmd.WriteString(" --cron ")
		cmd.WriteString(quoteCompatShellArg(schedule))
		cmd.WriteString(" --handler ")
		cmd.WriteString(quoteCompatShellArg(handler))
		if payload := buildCompatPayloadJSON(args["payload"]); payload != "" {
			cmd.WriteString(" --payload ")
			cmd.WriteString(quoteCompatShellArg(payload))
		}
		return cmd.String(), true, ""
	case "delete", "remove", "rm":
		id := firstCompatString(args, "id", "job_id", "cron_id")
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/job_id is required for delete/remove")
		}
		return "blue cron rm " + quoteCompatShellArg(id), true, ""
	case "enable":
		id := firstCompatString(args, "id", "job_id", "cron_id")
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/job_id is required for enable")
		}
		return "blue cron enable " + quoteCompatShellArg(id), true, ""
	case "disable":
		id := firstCompatString(args, "id", "job_id", "cron_id")
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/job_id is required for disable")
		}
		return "blue cron disable " + quoteCompatShellArg(id), true, ""
	case "run", "trigger":
		id := firstCompatString(args, "id", "job_id", "cron_id")
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/job_id is required for run/trigger")
		}
		return "blue cron run " + quoteCompatShellArg(id), true, ""
	case "runs", "executions", "history":
		id := firstCompatString(args, "id", "job_id", "cron_id")
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/job_id is required for runs/executions")
		}
		return "blue cron runs " + quoteCompatShellArg(id), true, ""
	default:
		return "", true, invalidFactoryToolArgsResult(name, "unknown cron action")
	}
}

func resolveFactoryMessageAliasCommand(name string, args map[string]interface{}) (string, bool, string) {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	if action == "" {
		if firstCompatString(args, "id", "message_id", "reminder_id") != "" {
			action = "delete"
		} else if firstCompatString(args, "message", "content", "text", "input") != "" {
			action = "add"
		} else {
			action = "list"
		}
	}

	switch action {
	case "list", "status", "get":
		return buildSkillCommand("reminder", map[string]interface{}{"action": "list"}), true, ""
	case "delete", "remove", "rm":
		id := firstCompatString(args, "id", "message_id", "reminder_id")
		if id == "" {
			return "", true, invalidFactoryToolArgsResult(name, "id/message_id is required for delete")
		}
		return buildSkillCommand("reminder", map[string]interface{}{"action": "delete", "id": id}), true, ""
	case "clear":
		return buildSkillCommand("reminder", map[string]interface{}{"action": "clear"}), true, ""
	case "add", "create", "send", "notify":
		message := firstCompatString(args, "message", "content", "text", "input")
		timeText := firstCompatString(args, "time", "at", "when", "delay", "in")
		if message == "" || timeText == "" {
			return "", true, invalidFactoryToolArgsResult(name, "message/content and time are required for send/add")
		}
		reminderArgs := map[string]interface{}{
			"action":  "add",
			"message": message,
			"time":    timeText,
		}
		if recurring := firstCompatString(args, "recurring", "repeat"); recurring != "" {
			reminderArgs["recurring"] = recurring
		}
		if sessionID := firstCompatString(args, "session_id", "session", "conversation_id"); sessionID != "" {
			reminderArgs["session_id"] = sessionID
		}
		return buildSkillCommand("reminder", reminderArgs), true, ""
	default:
		return "", true, invalidFactoryToolArgsResult(name, "unknown message action")
	}
}

func buildCompatPayloadJSON(v interface{}) string {
	if v == nil {
		return ""
	}
	switch typed := v.(type) {
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return ""
		}
		return raw
	default:
		b, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func quoteCompatShellArg(raw string) string {
	return strconv.Quote(strings.TrimSpace(raw))
}

func asCompatBool(v interface{}) (bool, bool) {
	switch typed := v.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "y", "on":
			return true, true
		case "0", "false", "no", "n", "off":
			return false, true
		default:
			return false, false
		}
	case float64:
		return typed != 0, true
	case int:
		return typed != 0, true
	default:
		return false, false
	}
}
