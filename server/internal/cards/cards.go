// Package cards converts tool execution results into structured "typeless card"
// blocks that the frontend renders as rich UI elements (search results, code
// previews, status badges, etc.).
//
// Cards are emitted as fenced JSON inside ```typeless``` blocks and appended to
// the assistant's streamed content.
package cards

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

// escapeBackticks replaces triple backticks with a Unicode alternative to prevent
// the frontend regex from incorrectly matching inner code fences within the JSON.
// This ensures the typeless card parser correctly identifies the outer fence boundaries.
func escapeBackticks(s string) string {
	// Use Unicode character U+200B (zero-width space) as escape marker
	// This is invisible and won't affect display, but can be unescaped on the frontend
	return strings.ReplaceAll(s, "```", "`​``")
}

// CardFunc converts tool result content into a card map. Return nil to skip.
type CardFunc func(content string) map[string]interface{}

var (
	registryMu sync.RWMutex
	registry   = map[string]CardFunc{}
)

const (
	redactedErrorText = "Error details hidden for safety"
	sensitiveValueKey = `(?:api[_-]?key|apikey|access[_-]?token|refresh[_-]?token|id[_-]?token|auth[_-]?token|session[_-]?token|token|secret|password|passwd|pwd|authorization|cookie|set-cookie|aws_access_key_id|aws_secret_access_key|aws_session_token|openai_api_key|x-api-key)`
)

type textRedactionRule struct {
	re          *regexp.Regexp
	replacement string
}

var sensitiveTextRedactionRules = []textRedactionRule{
	{re: regexp.MustCompile(`(?is)-----BEGIN(?: [A-Z0-9]+)? PRIVATE KEY-----.*?-----END(?: [A-Z0-9]+)? PRIVATE KEY-----`), replacement: "[PRIVATE_KEY_REDACTED]"},
	{re: regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._=-]{8,}`), replacement: "Bearer [TOKEN_REDACTED]"},
	{re: regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9._-]+\.[a-zA-Z0-9._-]+\b`), replacement: "[JWT_REDACTED]"},
	{re: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), replacement: "[AWS_KEY_REDACTED]"},
	{re: regexp.MustCompile(`(?i)([?&](?:token|access_token|refresh_token|api_key|apikey|secret|password|authorization)=)[^&#\s]+`), replacement: "${1}[REDACTED]"},
	{re: regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^/\s:@]+:)([^@\s/]+)@`), replacement: "${1}[REDACTED]@"},
	{re: regexp.MustCompile(`(?i)(\b(?:cookie|set-cookie)\b\s*[:=]\s*)([^\n\r]+)`), replacement: "${1}[COOKIE_REDACTED]"},
	{re: regexp.MustCompile(`(?i)(--?(?:api[-_]?key|access[-_]?token|refresh[-_]?token|id[-_]?token|auth[-_]?token|session[-_]?token|token|secret|password|passwd|pwd|authorization|cookie))(=|\s+)(\"?[^\s\"']+\"?)`), replacement: "${1}${2}[REDACTED]"},
	{re: regexp.MustCompile(fmt.Sprintf(`(?i)("%s"\s*:\s*")([^"\n\r]+)(")`, sensitiveValueKey)), replacement: "${1}[REDACTED]${3}"},
	{re: regexp.MustCompile(fmt.Sprintf(`(?i)(\b%s\b\s*[:=]\s*)(\"?[^\s\"',;]+\"?)`, sensitiveValueKey)), replacement: "${1}[REDACTED]"},
	{re: regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`), replacement: "[EMAIL_REDACTED]"},
	{re: regexp.MustCompile(`\b(?:10(?:\.\d{1,3}){3}|127(?:\.\d{1,3}){3}|169\.254(?:\.\d{1,3}){2}|172\.(?:1[6-9]|2\d|3[0-1])(?:\.\d{1,3}){2}|192\.168(?:\.\d{1,3}){2}|localhost)\b`), replacement: "[IP_REDACTED]"},
}

// RedactSensitiveText masks obvious secrets/PII while preserving readable structure.
func RedactSensitiveText(input string) string {
	if input == "" {
		return ""
	}
	redacted := input
	for _, rule := range sensitiveTextRedactionRules {
		redacted = rule.re.ReplaceAllString(redacted, rule.replacement)
	}
	return redacted
}

func hasNonEmptyError(data map[string]interface{}) bool {
	if data == nil {
		return false
	}
	errMsg, ok := data["error"].(string)
	return ok && strings.TrimSpace(errMsg) != ""
}

func buildRedactedErrorCard(cardType, title string) map[string]interface{} {
	return map[string]interface{}{
		"type":           cardType,
		"title":          title,
		"status":         "error",
		"message":        redactedErrorText,
		"error_redacted": true,
	}
}

func shouldExposeErrorDetails(toolName string) bool {
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "browser":
		return true
	default:
		return false
	}
}

func buildToolErrorCard(toolName string, data map[string]interface{}) map[string]interface{} {
	if shouldExposeErrorDetails(toolName) {
		if errMsg := strings.TrimSpace(formatValue(data["error"])); errMsg != "" {
			return map[string]interface{}{
				"type":    "result",
				"title":   toolName,
				"status":  "error",
				"message": escapeBackticks(RedactSensitiveText(errMsg)),
			}
		}
	}
	return buildRedactedErrorCard("result", toolName)
}

// Register adds a custom card formatter for a tool name.
// Registered formatters take priority over built-in ones.
func Register(toolName string, fn CardFunc) {
	registryMu.Lock()
	registry[toolName] = fn
	registryMu.Unlock()
}

// FormatTypeless converts tool results into typeless card blocks
// that can be appended to the assistant's text content for frontend rendering.
func FormatTypeless(toolCalls []llm.ToolCall, toolResults []llm.Message) string {
	var sb strings.Builder
	for i, tc := range toolCalls {
		if i >= len(toolResults) {
			break
		}
		card := ToCard(tc.Name, toolResults[i].Content)
		if card != nil && tc.Name == "exec" {
			if _, hasCommand := card["command"]; !hasCommand {
				if cmd := extractExecCommand(tc.Arguments); cmd != "" {
					card["command"] = escapeBackticks(RedactSensitiveText(cmd))
				}
			}
		}
		if card != nil {
			cardJSON, _ := json.Marshal(card)
			sb.WriteString("\n\n```typeless\n")
			sb.Write(cardJSON)
			sb.WriteString("\n```")
		}
	}
	return sb.String()
}

func extractExecCommand(arguments string) string {
	if strings.TrimSpace(arguments) == "" {
		return ""
	}
	var payload map[string]interface{}
	if json.Unmarshal([]byte(arguments), &payload) != nil {
		return ""
	}
	command, _ := payload["command"].(string)
	return strings.TrimSpace(command)
}

// ToCard converts a single tool result into a typeless card map.
// Returns nil if no card should be rendered.
func ToCard(toolName, content string) map[string]interface{} {
	// Check registered custom formatters first.
	registryMu.RLock()
	fn, ok := registry[toolName]
	registryMu.RUnlock()
	if ok {
		return fn(content)
	}

	// When the sandbox tool runs a `blue` CLI command, the IPC response
	// may include a `_card` hint telling us which card formatter to use.
	if toolName == "sandbox" {
		if card := sandboxCardDispatch(content); card != nil {
			return card
		}
		return GenericCard(toolName, content)
	}

	switch toolName {
	case "exec":
		return execCard(content)
	case "browser":
		return browserCard(content)
	case "web_fetch":
		return webFetchCard(content)
	case "web_search":
		return webSearchCard(content)
	case "deep_research", "deep-research":
		return deepResearchCard(content)
	case "ui_reviewer":
		return uiReviewCard(content)
	case "calculator":
		return calculatorCard(content)
	case "current_time":
		return currentTimeCard(content)
	case "read", "file_read":
		return fileReadCard(content)
	case "write", "file_write":
		return fileWriteCard(content)
	case "system_info":
		return systemInfoCard(content)
	case "memory_search", "memory":
		return memorySearchCard(content)
	case "reminder":
		return reminderCard(content)
	case "analyze":
		return analyzeCard(content)
	case "ask":
		return askUserQuestionCard(content)
	default:
		return GenericCard(toolName, content)
	}
}

func browserCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return GenericCard("browser", content)
	}
	if hasNonEmptyError(data) {
		return buildToolErrorCard("browser", data)
	}

	title := strings.TrimSpace(formatValue(data["title"]))
	if title == "" {
		title = "Browser"
	}
	pageURL := strings.TrimSpace(formatValue(data["url"]))
	targetID := strings.TrimSpace(formatValue(data["target_id"]))
	strategy := strings.TrimSpace(formatValue(data["strategy"]))
	tree := strings.TrimSpace(formatValue(data["tree"]))
	message := strings.TrimSpace(formatValue(data["message"]))

	card := map[string]interface{}{
		"type":   "result",
		"title":  escapeBackticks(RedactSensitiveText(title)),
		"status": "success",
	}
	if message != "" && tree == "" && !strings.Contains(message, "\n") {
		card["message"] = escapeBackticks(RedactSensitiveText(message))
	}
	if targetID != "" {
		card["id"] = "browser-tab-" + url.QueryEscape(targetID)
	}

	details := make([]map[string]interface{}, 0, 5)
	if pageURL != "" {
		details = append(details, map[string]interface{}{
			"label":    "url",
			"value":    escapeBackticks(RedactSensitiveText(pageURL)),
			"copyable": true,
		})
	}
	if targetID != "" {
		details = append(details, map[string]interface{}{
			"label":    "target_id",
			"value":    escapeBackticks(RedactSensitiveText(targetID)),
			"copyable": true,
		})
	}
	if strategy != "" {
		details = append(details, map[string]interface{}{"label": "strategy", "value": strategy})
	}
	if count, ok := data["count"]; ok {
		if countText := strings.TrimSpace(formatValue(count)); countText != "" {
			details = append(details, map[string]interface{}{"label": "count", "value": countText})
		}
	}
	if tree != "" {
		details = append(details, map[string]interface{}{
			"label":     "tree",
			"value":     escapeBackticks(RedactSensitiveText(tree)),
			"multiline": true,
		})
	}
	if len(details) > 0 {
		card["details"] = details
	}
	if pageURL != "" && targetID != "" {
		card["actions"] = []map[string]interface{}{{
			"id":      "extract_with_web_fetch",
			"label":   "Extract with web_fetch",
			"variant": "primary",
			"form_data": map[string]interface{}{
				"url":               pageURL,
				"browser_target_id": targetID,
			},
		}}
	}
	if _, hasMessage := card["message"]; !hasMessage && len(details) == 0 {
		card["message"] = "Browser tab ready"
	}

	return card
}

func webFetchCard(content string) map[string]interface{} {
	var data struct {
		URL         string `json:"url"`
		Title       string `json:"title"`
		Content     string `json:"content"`
		ContentType string `json:"content_type"`
		ExtractMode string `json:"extract_mode"`
		Extractor   string `json:"extractor"`
		Truncated   bool   `json:"truncated"`
		Warning     string `json:"warning"`
		WarningCode string `json:"warning_code"`
		Error       string `json:"error"`
	}
	if json.Unmarshal([]byte(content), &data) != nil {
		return GenericCard("web_fetch", content)
	}

	if strings.TrimSpace(data.Error) != "" {
		return buildToolErrorCard("web_fetch", map[string]interface{}{"error": data.Error})
	}

	title := strings.TrimSpace(data.Title)
	if title == "" {
		title = "web_fetch"
	}
	fetchURL := strings.TrimSpace(data.URL)

	card := map[string]interface{}{
		"type":         "web-fetch",
		"title":        escapeBackticks(RedactSensitiveText(title)),
		"status":       "success",
		"url":          escapeBackticks(RedactSensitiveText(fetchURL)),
		"content":      escapeBackticks(RedactSensitiveText(strings.TrimSpace(data.Content))),
		"content_type": strings.TrimSpace(data.ContentType),
		"extract_mode": strings.TrimSpace(data.ExtractMode),
		"extractor":    strings.TrimSpace(data.Extractor),
	}
	if fetchURL != "" {
		card["id"] = "web-fetch-" + url.QueryEscape(fetchURL)
		card["actions"] = []map[string]interface{}{{
			"id":      "use_browser",
			"label":   "Use browser",
			"variant": "primary",
			"form_data": map[string]interface{}{
				"url": fetchURL,
			},
		}}
	}
	if data.Truncated {
		card["truncated"] = true
	}

	warning := escapeBackticks(RedactSensitiveText(strings.TrimSpace(data.Warning)))
	warningCode := strings.TrimSpace(data.WarningCode)
	if warning != "" {
		card["warning"] = warning
		card["status"] = "warning"
	}
	if warningCode != "" {
		card["warning_code"] = warningCode
		card["status"] = "warning"
	}

	return card
}

func webSearchCard(content string) map[string]interface{} {
	var resp struct {
		Query      string            `json:"query"`
		Results    []json.RawMessage `json:"results"`
		TotalCount int               `json:"total_count"`
	}
	if json.Unmarshal([]byte(content), &resp) != nil || len(resp.Results) == 0 {
		return nil
	}
	var results []interface{}
	for _, r := range resp.Results {
		var item interface{}
		json.Unmarshal(r, &item)
		results = append(results, item)
	}
	return map[string]interface{}{
		"type":        "search",
		"query":       resp.Query,
		"total_count": resp.TotalCount,
		"results":     results,
	}
}

func deepResearchCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	if hasNonEmptyError(data) {
		return buildRedactedErrorCard("result", "deep_research")
	}

	card := map[string]interface{}{
		"type": "deep-research",
	}
	for _, key := range []string{
		"job_id",
		"query",
		"mode",
		"answer",
		"confidence",
		"evidence_count",
		"citations",
		"open_questions",
		"support_count",
		"conflict_count",
		"has_conflict",
		"status",
	} {
		if v, ok := data[key]; ok {
			card[key] = v
		}
	}
	return card
}

func calculatorCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	if hasNonEmptyError(data) {
		return buildRedactedErrorCard("result", "Calculator")
	}
	expr, _ := data["expression"].(string)
	result := fmt.Sprintf("%v", data["result"])
	details := []map[string]interface{}{}
	if expr != "" {
		details = append(details, map[string]interface{}{"label": "Expression", "value": expr})
	}
	details = append(details, map[string]interface{}{"label": "Result", "value": result, "copyable": true})
	return map[string]interface{}{
		"type":    "result",
		"title":   "Calculator",
		"status":  "success",
		"message": result,
		"details": details,
	}
}

func currentTimeCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	datetime, _ := data["datetime"].(string)
	details := []map[string]interface{}{}
	for _, key := range []string{"timezone", "unix"} {
		if v, ok := data[key]; ok {
			details = append(details, map[string]interface{}{"label": key, "value": fmt.Sprintf("%v", v)})
		}
	}
	card := map[string]interface{}{
		"type":   "result",
		"title":  "Current Time",
		"status": "info",
	}
	if datetime != "" {
		card["message"] = datetime
	}
	if len(details) > 0 {
		card["details"] = details
	}
	return card
}

func fileReadCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	if hasNonEmptyError(data) {
		return buildRedactedErrorCard("result", "File Read")
	}
	fileContent, _ := data["content"].(string)
	filePath, _ := data["path"].(string)
	if fileContent == "" {
		return nil
	}
	return map[string]interface{}{
		"type":     "collapsible-code",
		"title":    "File Read",
		"filename": filePath,
		"code":     fileContent,
	}
}

func fileWriteCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	status := "success"
	msg := "File written successfully"
	if hasNonEmptyError(data) {
		status = "error"
		msg = redactedErrorText
	} else if m, ok := data["message"].(string); ok {
		msg = m
	}
	details := []map[string]interface{}{}
	if p, ok := data["path"].(string); ok {
		details = append(details, map[string]interface{}{"label": "Path", "value": p})
	}
	return map[string]interface{}{
		"type":    "result",
		"title":   "File Write",
		"status":  status,
		"message": msg,
		"details": details,
	}
}

func systemInfoCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	details := []map[string]interface{}{}
	for _, key := range []string{"os", "arch", "hostname", "cpu_cores", "memory_total", "go_version"} {
		if v, ok := data[key]; ok {
			details = append(details, map[string]interface{}{"label": key, "value": fmt.Sprintf("%v", v)})
		}
	}
	return map[string]interface{}{
		"type":    "result",
		"title":   "System Info",
		"status":  "info",
		"details": details,
	}
}

func memorySearchCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	if hasNonEmptyError(data) {
		return buildRedactedErrorCard("result", "Memory Search")
	}
	msg := "Search completed"
	if results, ok := data["results"].([]interface{}); ok {
		msg = fmt.Sprintf("Found %d results", len(results))
	}
	return map[string]interface{}{
		"type":    "result",
		"title":   "Memory Search",
		"status":  "success",
		"message": msg,
	}
}

func reminderCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	if hasNonEmptyError(data) {
		return buildRedactedErrorCard("result", "Reminder")
	}

	msg, _ := data["message"].(string)
	status := "success"
	if s, ok := data["status"].(string); ok && s != "" {
		status = s
	}

	card := map[string]interface{}{
		"type":   "result",
		"title":  "Reminder",
		"status": status,
	}
	if msg != "" {
		card["message"] = msg
	}

	// For add action: show reminder details
	if r, ok := data["reminder"].(map[string]interface{}); ok {
		details := []map[string]interface{}{}
		if m, ok := r["message"].(string); ok && m != "" {
			details = append(details, map[string]interface{}{"label": "Message", "value": m})
		}
		if fa, ok := r["fire_at"].(string); ok && fa != "" {
			details = append(details, map[string]interface{}{"label": "Fire At", "value": fa})
		}
		if rec, ok := r["recurring"].(string); ok && rec != "" {
			details = append(details, map[string]interface{}{"label": "Recurring", "value": rec})
		}
		if len(details) > 0 {
			card["details"] = details
		}
	}

	// For list action: show count
	if count, ok := data["count"].(float64); ok {
		card["message"] = fmt.Sprintf("%d reminders", int(count))
	}

	// For clear action: show cleared count
	if cleared, ok := data["cleared"].(float64); ok {
		card["message"] = fmt.Sprintf("Cleared %d reminders", int(cleared))
	}

	return card
}

func analyzeCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	if hasNonEmptyError(data) {
		return buildRedactedErrorCard("result", "analyze")
	}
	topic, _ := data["topic"].(string)
	reportURL, _ := data["report_url"].(string)

	card := map[string]interface{}{
		"type":   "result",
		"title":  "analyze",
		"status": "success",
	}
	if topic != "" {
		card["message"] = topic
	}
	if reportURL != "" {
		card["details"] = []map[string]interface{}{
			{"label": "report_url", "value": reportURL},
		}
	}
	return card
}

func askUserQuestionCard(content string) map[string]interface{} {
	var data struct {
		QA []struct {
			Q string   `json:"q"`
			O []string `json:"o"`
			A []string `json:"a"`
		} `json:"qa"`
		Silent bool `json:"silent"`
	}
	if json.Unmarshal([]byte(content), &data) != nil || len(data.QA) == 0 {
		return nil
	}

	details := make([]map[string]interface{}, 0, len(data.QA)*3)
	for _, item := range data.QA {
		details = append(details, map[string]interface{}{
			"label": "q",
			"value": item.Q,
		})
		if len(item.O) > 0 {
			details = append(details, map[string]interface{}{
				"label": "o",
				"value": strings.Join(item.O, " / "),
			})
		}
		details = append(details, map[string]interface{}{
			"label": "a",
			"value": strings.Join(item.A, ", "),
		})
	}

	card := map[string]interface{}{
		"type":    "result",
		"title":   "ask",
		"status":  "success",
		"details": details,
	}
	if data.Silent {
		card["message"] = "auto-answered (silent mode)"
	}
	return card
}

// formatValue converts a value to a display string.
// For simple types, uses fmt.Sprintf; for complex types (slices, maps),
// uses JSON serialization to avoid Go's default map[...] format.
func formatValue(v interface{}) string {
	switch v.(type) {
	case string, float64, bool:
		return fmt.Sprintf("%v", v)
	default:
		if v == nil {
			return ""
		}
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

// GenericCard creates a result card for any unrecognized tool.
func GenericCard(toolName, content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) == nil {
		if hasNonEmptyError(data) {
			return buildToolErrorCard(toolName, data)
		}
		card := map[string]interface{}{
			"type":   "result",
			"title":  toolName,
			"status": "success",
		}

		if message, ok := data["message"].(string); ok && strings.TrimSpace(message) != "" {
			card["message"] = message
		}

		keys := make([]string, 0, len(data))
		for k := range data {
			if k == "message" || k == "error" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)

		details := make([]map[string]interface{}, 0, len(keys))
		for _, k := range keys {
			details = append(details, map[string]interface{}{
				"label": k,
				"value": formatValue(data[k]),
			})
		}
		if len(details) > 0 {
			card["details"] = details
		}

		if _, hasMessage := card["message"]; !hasMessage && len(details) == 0 {
			card["message"] = "No result data"
		}

		return card
	}

	message := strings.TrimSpace(content)
	if message == "" {
		message = "No result data"
	}

	return map[string]interface{}{
		"type":    "result",
		"title":   toolName,
		"status":  "info",
		"message": message,
	}
}

// sandboxCardDispatch extracts the _card hint from a sandbox tool result.
// When the LLM runs `blue <cmd>` via the sandbox tool, the IPC response
// includes a `_card` field in the JSON output. We parse the sandbox result's
// stdout to find it and delegate to the appropriate card formatter.
func sandboxCardDispatch(content string) map[string]interface{} {
	// Sandbox result shape: {"result":{"stdout":"...","stderr":"...","exit_code":0,...},...}
	var outer map[string]interface{}
	if json.Unmarshal([]byte(content), &outer) != nil {
		return nil
	}

	// Extract stdout from the sandbox result
	var stdout string
	if result, ok := outer["result"]; ok {
		switch r := result.(type) {
		case map[string]interface{}:
			stdout, _ = r["stdout"].(string)
		}
	}
	if stdout == "" {
		return nil
	}

	// The stdout may be the full IPC JSON response: {"status":"ok","data":{"_card":"...","result":"..."}}
	var ipcResp struct {
		Status string            `json:"status"`
		Data   map[string]string `json:"data"`
	}
	if json.Unmarshal([]byte(stdout), &ipcResp) != nil || ipcResp.Data == nil {
		return nil
	}

	cardType := ipcResp.Data["_card"]
	resultJSON := ipcResp.Data["result"]
	if cardType == "" || resultJSON == "" {
		return nil
	}

	return ToCard(cardType, resultJSON)
}

func execCard(content string) map[string]interface{} {
	var data struct {
		SessionID string   `json:"session_id"`
		Status    string   `json:"status"`
		ExitCode  *int     `json:"exit_code"`
		Stdout    string   `json:"stdout"`
		Stderr    string   `json:"stderr"`
		Error     string   `json:"error"`
		Duration  int64    `json:"duration_ms"`
		Truncated bool     `json:"truncated"`
		Warnings  []string `json:"warnings"`
		Host      string   `json:"host"`
		RiskLevel string   `json:"risk_level"`
		Command   string   `json:"command"`
	}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}

	// If exec returned a "command is required" validation error with no
	// execution data, suppress the card entirely — it's LLM noise.
	if data.ExitCode == nil && data.Stdout == "" && data.Stderr == "" && data.Command == "" {
		errMsg := strings.TrimSpace(data.Error)
		if errMsg == "" || strings.EqualFold(errMsg, "command is required") {
			return nil
		}
	}

	// When a `blue` subcommand ran and produced __CARD__ lines, the card
	// protocol already pushed a streaming result card to the client.
	// The __CARD__ lines are stripped from stdout by readIntoBufferWithCards,
	// so stdout is empty. Suppress the redundant exec card in this case.
	if data.Stdout == "" && data.Stderr == "" &&
		data.ExitCode != nil && *data.ExitCode == 0 {
		return nil
	}

	status := "success"
	if data.Status == "failed" || (data.ExitCode != nil && *data.ExitCode != 0) || strings.TrimSpace(data.Error) != "" {
		status = "error"
	}

	card := map[string]interface{}{
		"type":   "exec",
		"status": status,
	}

	if data.ExitCode != nil {
		card["exit_code"] = *data.ExitCode
	}
	if strings.TrimSpace(data.Command) != "" {
		card["command"] = escapeBackticks(RedactSensitiveText(data.Command))
	}
	hasOutput := false
	if data.Stdout != "" {
		card["stdout"] = escapeBackticks(RedactSensitiveText(data.Stdout))
		hasOutput = true
	}
	if data.Stderr != "" {
		card["stderr"] = escapeBackticks(RedactSensitiveText(data.Stderr))
		hasOutput = true
	}
	if !hasOutput && status == "error" {
		if errMsg := strings.TrimSpace(data.Error); errMsg != "" {
			card["message"] = escapeBackticks(RedactSensitiveText(errMsg))
		} else {
			card["message"] = "Command failed"
		}
	}
	if data.Duration > 0 {
		card["duration_ms"] = data.Duration
	}
	if data.Truncated {
		card["truncated"] = true
	}
	if len(data.Warnings) > 0 {
		warnings := make([]string, 0, len(data.Warnings))
		for _, warning := range data.Warnings {
			warning = strings.TrimSpace(warning)
			if warning == "" {
				continue
			}
			warnings = append(warnings, escapeBackticks(RedactSensitiveText(warning)))
		}
		if len(warnings) > 0 {
			card["warning_count"] = len(warnings)
			card["warnings"] = warnings
		}
	}
	if data.Host != "" {
		card["host"] = data.Host
	}
	if data.RiskLevel != "" {
		card["risk_level"] = data.RiskLevel
	}

	return card
}

func uiReviewCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}

	// Check for error result
	if hasNonEmptyError(data) {
		return map[string]interface{}{
			"type":    "ui-review",
			"status":  "error",
			"message": redactedErrorText,
			"actions": []map[string]interface{}{
				{"id": "recheck", "label": "Retry", "variant": "primary"},
			},
			"error_redacted": true,
		}
	}

	card := map[string]interface{}{
		"type": "ui-review",
	}

	// Stable card ID from URL
	if url, ok := data["url"].(string); ok && url != "" {
		card["id"] = "ui-review-" + url
	}

	// Copy relevant fields
	for _, key := range []string{
		"url", "overall", "pass", "threshold",
		"visual", "functional", "accessibility",
		"issues", "suggestions", "steps", "viewports",
	} {
		if v, ok := data[key]; ok {
			card[key] = v
		}
	}

	// Include screenshot if present (for thumbnail)
	if ss, ok := data["screenshot"].(string); ok && len(ss) > 0 {
		card["screenshot"] = ss
	}

	// Interactive action buttons
	card["actions"] = []map[string]interface{}{
		{"id": "recheck", "label": "Re-check", "variant": "primary"},
		{"id": "check_a11y", "label": "Accessibility Only", "variant": "secondary"},
		{"id": "full_report", "label": "Full Report", "variant": "secondary"},
	}

	return card
}
