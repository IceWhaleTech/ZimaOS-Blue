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

	switch toolName {
	case "web_search":
		return webSearchCard(content)
	case "ui_reviewer":
		return uiReviewCard(content)
	case "image_generate":
		return imageGenerateCard(content)
	case "video_generate":
		return videoGenerateCard(content)
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
	case "workspace_file":
		return workspaceFileCard(content)
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

func workspaceFileCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	if errMsg, ok := data["error"].(string); ok {
		return map[string]interface{}{
			"type":    "result",
			"title":   "Workspace File",
			"status":  "error",
			"message": errMsg,
		}
	}
	// Read action: show file content as collapsible markdown
	if fileContent, ok := data["content"].(string); ok && fileContent != "" {
		filename, _ := data["filename"].(string)
		return map[string]interface{}{
			"type":     "collapsible-code",
			"title":    "Workspace File",
			"filename": filename,
			"language": "markdown",
			"code":     fileContent,
		}
	}
	// Write/other actions: show result card
	msg, _ := data["message"].(string)
	if msg == "" {
		if s, ok := data["status"].(string); ok && s == "ok" {
			msg = "Done"
		}
	}
	details := []map[string]interface{}{}
	if fn, ok := data["filename"].(string); ok {
		details = append(details, map[string]interface{}{"label": "filename", "value": fn})
	}
	if b, ok := data["bytes"]; ok {
		details = append(details, map[string]interface{}{"label": "bytes", "value": fmt.Sprintf("%v", b)})
	}
	card := map[string]interface{}{
		"type":   "result",
		"title":  "Workspace File",
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
			details = append(details, map[string]interface{}{"label": key, "value": fmt.Sprintf("%v", v)})
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

func imageGenerateCard(content string) map[string]interface{} {
	var data struct {
		Images []struct {
			URL           string `json:"url"`
			ThumbnailURL  string `json:"thumbnail_url"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"images"`
		Error   string `json:"error"`
		TaskID  string `json:"task_id"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}

	if data.Error != "" || data.Status == "failed" {
		msg := data.Error
		if msg == "" {
			msg = data.Message
		}
		return map[string]interface{}{
			"type":       "media-generate",
			"media_type": "image",
			"status":     "error",
			"message":    msg,
		}
	}

	// Still processing — show animated placeholder
	if data.Status == "processing" || (data.TaskID != "" && len(data.Images) == 0) {
		return map[string]interface{}{
			"type":       "media-generate",
			"media_type": "image",
			"status":     "generating",
			"task_id":    data.TaskID,
		}
	}

	if len(data.Images) == 0 {
		return nil
	}

	images := make([]map[string]interface{}, 0, len(data.Images))
	for _, img := range data.Images {
		entry := map[string]interface{}{
			"src": img.URL,
		}
		if img.ThumbnailURL != "" {
			entry["thumbnail"] = img.ThumbnailURL
		}
		if img.RevisedPrompt != "" {
			entry["caption"] = img.RevisedPrompt
		}
		images = append(images, entry)
	}

	return map[string]interface{}{
		"type":       "media-generate",
		"media_type": "image",
		"status":     "success",
		"images":     images,
	}
}

func videoGenerateCard(content string) map[string]interface{} {
	var data struct {
		TaskID  string `json:"task_id"`
		Status  string `json:"status"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}

	if data.Error != "" || data.Status == "failed" {
		msg := data.Error
		if msg == "" {
			msg = data.Message
		}
		return map[string]interface{}{
			"type":       "media-generate",
			"media_type": "video",
			"status":     "error",
			"message":    msg,
		}
	}

	return map[string]interface{}{
		"type":       "media-generate",
		"media_type": "video",
		"status":     "generating",
		"task_id":    data.TaskID,
		"message":    data.Message,
	}
}
