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
		// Inject command into exec card from tool call arguments.
		if card != nil && tc.Name == "exec" {
			var args struct {
				Command string `json:"command"`
			}
			if json.Unmarshal([]byte(tc.Arguments), &args) == nil && args.Command != "" {
				card["command"] = escapeBackticks(args.Command)
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
	case "file_read":
		return fileReadCard(content)
	case "file_write":
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
	if errMsg, ok := data["error"].(string); ok && strings.TrimSpace(errMsg) != "" {
		return map[string]interface{}{
			"type":    "result",
			"title":   "deep_research",
			"status":  "error",
			"message": errMsg,
		}
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
	if errMsg, ok := data["error"].(string); ok {
		return map[string]interface{}{
			"type":    "result",
			"title":   "Calculator",
			"status":  "error",
			"message": errMsg,
		}
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
	if errMsg, ok := data["error"].(string); ok {
		return map[string]interface{}{
			"type":    "result",
			"title":   "File Read",
			"status":  "error",
			"message": errMsg,
		}
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
	if errMsg, ok := data["error"].(string); ok {
		status = "error"
		msg = errMsg
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
	if errMsg, ok := data["error"].(string); ok {
		return map[string]interface{}{
			"type":    "result",
			"title":   "Memory Search",
			"status":  "error",
			"message": errMsg,
		}
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
	if errMsg, ok := data["error"].(string); ok {
		return map[string]interface{}{
			"type":    "result",
			"title":   "Reminder",
			"status":  "error",
			"message": errMsg,
		}
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
	if errMsg, ok := data["error"].(string); ok {
		return map[string]interface{}{
			"type":    "result",
			"title":   "analyze",
			"status":  "error",
			"message": errMsg,
		}
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
		if errMsg, ok := data["error"].(string); ok {
			return map[string]interface{}{
				"type":    "result",
				"title":   toolName,
				"status":  "error",
				"message": errMsg,
			}
		}
		// Extract message field if present
		msg, _ := data["message"].(string)
		// Build details from remaining fields, skipping internal ones
		hiddenFields := map[string]bool{
			"id": true, "cron": true, "trigger_at": true,
			"status": true, "result": true, "message": true,
			"created_at": true, "updated_at": true,
			"owner_id": true, "user_id": true,
		}
		details := []map[string]interface{}{}
		for key, v := range data {
			if hiddenFields[key] {
				continue
			}
			details = append(details, map[string]interface{}{"label": key, "value": formatValue(v)})
		}
		card := map[string]interface{}{
			"type":   "result",
			"title":  toolName,
			"status": "success",
		}
		if msg != "" {
			card["message"] = msg
		}
		if len(details) > 0 {
			card["details"] = details
		}
		return card
	}
	display := content
	if len(display) > 500 {
		display = display[:500] + "..."
	}
	return map[string]interface{}{
		"type":    "result",
		"title":   toolName,
		"status":  "info",
		"message": display,
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

	// If exec returned a plain error (e.g. "command is required") with no
	// actual execution data, suppress the card entirely — it's LLM noise.
	if data.ExitCode == nil && data.Stdout == "" && data.Stderr == "" && data.Command == "" {
		return nil
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
	if data.Status == "failed" || (data.ExitCode != nil && *data.ExitCode != 0) {
		status = "error"
	}

	card := map[string]interface{}{
		"type":   "exec",
		"status": status,
	}

	if data.ExitCode != nil {
		card["exit_code"] = *data.ExitCode
	}
	if data.Stdout != "" {
		card["stdout"] = escapeBackticks(data.Stdout)
	}
	if data.Stderr != "" {
		card["stderr"] = escapeBackticks(data.Stderr)
	}
	if data.Duration > 0 {
		card["duration_ms"] = data.Duration
	}
	if data.Truncated {
		card["truncated"] = true
	}
	if len(data.Warnings) > 0 {
		card["warnings"] = data.Warnings
	}
	if data.Host != "" {
		card["host"] = data.Host
	}
	if data.RiskLevel != "" {
		card["risk_level"] = data.RiskLevel
	}
	if data.SessionID != "" {
		card["session_id"] = data.SessionID
	}

	return card
}

func uiReviewCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}

	// Check for error result
	if errMsg, ok := data["error"].(string); ok {
		return map[string]interface{}{
			"type":    "ui-review",
			"status":  "error",
			"message": errMsg,
			"actions": []map[string]interface{}{
				{"id": "recheck", "label": "Retry", "variant": "primary"},
			},
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
