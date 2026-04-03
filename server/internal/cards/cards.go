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

func firstCardNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// CardFunc converts tool result content into a card map. Return nil to skip.
type CardFunc func(content string) map[string]interface{}

var (
	registryMu sync.RWMutex
	registry   = map[string]CardFunc{}
)

var genericCardImageURLSuffix = regexp.MustCompile(`(?i)\.(png|jpe?g|gif|webp|bmp|svg)(?:[?#].*)?$`)
var genericCardWindowsAbsPath = regexp.MustCompile(`^(?:[a-zA-Z]:[\\/]|\\\\)`)

const (
	noErrorDetailsText = "Operation failed (no error details provided)"
	sensitiveValueKey  = `(?:api[_-]?key|apikey|access[_-]?token|refresh[_-]?token|id[_-]?token|auth[_-]?token|session[_-]?token|token|secret|password|passwd|pwd|authorization|cookie|set-cookie|aws_access_key_id|aws_secret_access_key|aws_session_token|openai_api_key|x-api-key)`
	lsPreviewEntries   = 80
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
	return sanitizedErrorMessage(data) != ""
}

func sanitizedErrorMessage(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	if errMsg := strings.TrimSpace(formatValue(data["error"])); errMsg != "" {
		return escapeBackticks(RedactSensitiveText(errMsg))
	}
	return ""
}

func buildErrorCard(cardType, title string, data map[string]interface{}) map[string]interface{} {
	errMsg := sanitizedErrorMessage(data)
	if errMsg == "" {
		errMsg = noErrorDetailsText
	}
	return map[string]interface{}{
		"type":    cardType,
		"title":   title,
		"status":  "error",
		"message": errMsg,
	}
}

func buildToolErrorCard(toolName string, data map[string]interface{}) map[string]interface{} {
	return buildErrorCard("result", toolName, data)
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

func normalizeCardToolName(toolName string) string {
	name := strings.ToLower(strings.TrimSpace(toolName))
	if strings.HasPrefix(name, "browser.") {
		return "browser"
	}
	switch name {
	case "web-fetch":
		return "web_fetch"
	case "deep-research":
		return "deep_research"
	case "file-read":
		return "file_read"
	case "file-write":
		return "file_write"
	case "image-generate":
		return "image_generate"
	default:
		return name
	}
}

func normalizeGenericCardFieldKey(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	input = strings.ReplaceAll(input, "-", "_")
	input = strings.ReplaceAll(input, " ", "_")
	return input
}

func isGenericCardImageField(label string) bool {
	switch normalizeGenericCardFieldKey(label) {
	case "image", "images", "media_url", "preview_image", "preview_url", "screenshot", "screenshots", "thumbnail", "thumbnail_url":
		return true
	default:
		return false
	}
}

func isGenericCardLocalAbsolutePath(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "/api/") {
		return false
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return false
	}
	if genericCardWindowsAbsPath.MatchString(trimmed) {
		return true
	}
	return strings.HasPrefix(trimmed, "/")
}

func isGenericCardAPIPath(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	return strings.HasPrefix(trimmed, "/api/")
}

func normalizeEmbeddedImageSource(raw, mimeType string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "data:image/") ||
		strings.HasPrefix(trimmed, "http://") ||
		strings.HasPrefix(trimmed, "https://") ||
		isGenericCardAPIPath(trimmed) ||
		isGenericCardLocalAbsolutePath(trimmed) {
		return trimmed
	}
	mimeType = strings.TrimSpace(mimeType)
	if strings.HasPrefix(mimeType, "image/") {
		return "data:" + mimeType + ";base64," + trimmed
	}
	return "data:image/png;base64," + trimmed
}

func looksLikeBase64ImagePayload(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < 64 {
		return false
	}
	for _, r := range trimmed {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '+', r == '/', r == '=', r == '\r', r == '\n':
		default:
			return false
		}
	}
	return true
}

func looksLikeImageString(raw, label, mimeType string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "data:image/") {
		return true
	}
	if strings.HasPrefix(mimeType, "image/") {
		return true
	}
	if strings.HasPrefix(trimmed, "http://") ||
		strings.HasPrefix(trimmed, "https://") ||
		isGenericCardAPIPath(trimmed) ||
		isGenericCardLocalAbsolutePath(trimmed) {
		return isGenericCardImageField(label) || genericCardImageURLSuffix.MatchString(trimmed)
	}
	return isGenericCardImageField(label) && looksLikeBase64ImagePayload(trimmed)
}

func appendGenericCardImage(images []map[string]interface{}, seen map[string]struct{}, raw, label, mimeType, thumbnail, caption string) []map[string]interface{} {
	if !looksLikeImageString(raw, label, mimeType) {
		return images
	}
	src := normalizeEmbeddedImageSource(raw, mimeType)
	if src == "" {
		return images
	}
	if _, exists := seen[src]; exists {
		return images
	}
	seen[src] = struct{}{}

	image := map[string]interface{}{
		"src": src,
	}
	if alt := strings.TrimSpace(label); alt != "" {
		image["alt"] = alt
	}
	if thumb := normalizeEmbeddedImageSource(thumbnail, mimeType); thumb != "" {
		image["thumbnail"] = thumb
	}
	if caption = strings.TrimSpace(caption); caption != "" {
		image["caption"] = caption
	}
	return append(images, image)
}

func extractGenericCardImages(images []map[string]interface{}, seen map[string]struct{}, value interface{}, label string) []map[string]interface{} {
	switch v := value.(type) {
	case string:
		return appendGenericCardImage(images, seen, v, label, "", "", "")
	case []interface{}:
		for _, entry := range v {
			images = extractGenericCardImages(images, seen, entry, label)
		}
		return images
	case map[string]interface{}:
		mimeType := strings.TrimSpace(formatValue(v["mime_type"]))
		if mimeType == "" {
			mimeType = strings.TrimSpace(formatValue(v["mimeType"]))
		}
		thumbnail := strings.TrimSpace(formatValue(v["thumbnail"]))
		if thumbnail == "" {
			thumbnail = strings.TrimSpace(formatValue(v["thumbnail_url"]))
		}
		caption := strings.TrimSpace(formatValue(v["caption"]))
		if caption == "" {
			caption = strings.TrimSpace(formatValue(v["title"]))
		}
		for _, key := range []string{"src", "url", "image", "screenshot", "data", "base64"} {
			raw := strings.TrimSpace(formatValue(v[key]))
			if raw == "" {
				continue
			}
			images = appendGenericCardImage(images, seen, raw, label, mimeType, thumbnail, caption)
		}
		if nested, ok := v["images"].([]interface{}); ok {
			for _, entry := range nested {
				images = extractGenericCardImages(images, seen, entry, label)
			}
		}
		if nested, ok := v["screenshots"].([]interface{}); ok {
			for _, entry := range nested {
				images = extractGenericCardImages(images, seen, entry, label)
			}
		}
		return images
	default:
		return images
	}
}

// ToCard converts a single tool result into a typeless card map.
// Returns nil if no card should be rendered.
func ToCard(toolName, content string) map[string]interface{} {
	normalizedToolName := normalizeCardToolName(toolName)

	// Check registered custom formatters first.
	registryMu.RLock()
	fn, ok := registry[toolName]
	if !ok && normalizedToolName != toolName {
		fn, ok = registry[normalizedToolName]
	}
	registryMu.RUnlock()
	if ok {
		return fn(content)
	}

	// When the sandbox tool runs a `blue` CLI command, the IPC response
	// may include a `_card` hint telling us which card formatter to use.
	if normalizedToolName == "sandbox" {
		if card := sandboxCardDispatch(content); card != nil {
			return card
		}
		return GenericCard(normalizedToolName, content)
	}

	switch normalizedToolName {
	case "bash", "exec":
		return execCard(content)
	case "browser":
		return browserCard(content)
	case "web_query":
		return webQueryCard(content)
	case "web_fetch":
		return webFetchCard(content)
	case "web_search":
		return webSearchCard(content)
	case "deep_research", "deep-research":
		return deepResearchCard(content)
	case "ui_reviewer":
		return uiReviewCard(content)
	case "image", "image_generate":
		return imageCard(content)
	case "ppt":
		return pptCard(content)
	case "calculator":
		return calculatorCard(content)
	case "current_time":
		return currentTimeCard(content)
	case "read", "file_read":
		return fileReadCard(content)
	case "write", "file_write":
		return fileWriteCard(content)
	case "write_commit":
		return fileWriteCard(content)
	case "office":
		return fileWriteCard(content)
	case "ls":
		return lsCard(content)
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
	case "convert":
		return convertTaskCard(content)
	default:
		return GenericCard(normalizedToolName, content)
	}
}

func convertTaskCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return GenericCard("convert", content)
	}
	taskID := strings.TrimSpace(formatValue(data["task_id"]))
	if taskID == "" {
		return GenericCard("convert", content)
	}
	status := strings.TrimSpace(formatValue(data["status"]))
	if status == "" {
		status = "pending"
	}
	card := map[string]interface{}{
		"type":               "convert-task",
		"id":                 "convert-task-" + taskID,
		"task_id":            taskID,
		"status":             status,
		"action":             strings.TrimSpace(formatValue(data["action"])),
		"sources":            stringSliceValue(data["sources"]),
		"source_summary":     strings.TrimSpace(formatValue(data["source_summary"])),
		"target_format":      strings.TrimSpace(formatValue(data["target_format"])),
		"message":            strings.TrimSpace(formatValue(data["message"])),
		"error":              strings.TrimSpace(formatValue(data["error"])),
		"progress":           data["progress"],
		"outputs":            data["outputs"],
		"transcript_preview": strings.TrimSpace(formatValue(data["transcript_preview"])),
	}
	return card
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
	screenshot := strings.TrimSpace(formatValue(data["screenshot"]))

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
	if screenshot != "" {
		card["image"] = normalizeEmbeddedImageSource(screenshot, "image/png")
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
	content = unwrapUntrustedToolCardContent(content, "web_search")
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

func webQueryCard(content string) map[string]interface{} {
	var data struct {
		Status     string `json:"status"`
		Mode       string `json:"mode"`
		Input      string `json:"input"`
		Query      string `json:"query"`
		TargetURL  string `json:"target_url"`
		FinalURL   string `json:"final_url"`
		Title      string `json:"title"`
		Content    string `json:"content"`
		NextAction string `json:"next_action"`
		Media      struct {
			Platform string `json:"platform"`
			Kind     string `json:"kind"`
			Language string `json:"language"`
			Summary  string `json:"summary"`
			Items    []struct {
				URL      string `json:"url"`
				Alt      string `json:"alt"`
				Source   string `json:"source"`
				Analysis string `json:"analysis"`
			} `json:"items"`
		} `json:"media"`
		Page struct {
			Title         string `json:"title"`
			Content       string `json:"content"`
			Source        string `json:"source"`
			TargetURL     string `json:"target_url"`
			FinalURL      string `json:"final_url"`
			ContentFormat string `json:"content_format"`
		} `json:"page"`
		Transcript struct {
			Status   string `json:"status"`
			Source   string `json:"source"`
			Language string `json:"language"`
			Text     string `json:"text"`
		} `json:"transcript"`
		Sources []struct {
			Title    string `json:"title"`
			URL      string `json:"url"`
			FinalURL string `json:"final_url"`
			Snippet  string `json:"snippet"`
			Source   string `json:"source"`
			Selected bool   `json:"selected"`
		} `json:"sources"`
		Warnings []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"warnings"`
	}
	if json.Unmarshal([]byte(content), &data) != nil {
		return GenericCard("web_query", content)
	}

	bodyText := firstCardNonEmpty(
		strings.TrimSpace(data.Content),
		strings.TrimSpace(data.Transcript.Text),
		strings.TrimSpace(data.Page.Content),
	)
	if bodyText == "" && len(data.Sources) > 0 && len(data.Warnings) == 0 && data.NextAction != "retry_browser" {
		results := make([]map[string]interface{}, 0, len(data.Sources))
		for _, source := range data.Sources {
			target := strings.TrimSpace(source.FinalURL)
			if target == "" {
				target = strings.TrimSpace(source.URL)
			}
			if target == "" {
				continue
			}

			title := strings.TrimSpace(source.Title)
			if title == "" {
				title = target
			}

			result := map[string]interface{}{
				"title": escapeBackticks(RedactSensitiveText(title)),
				"url":   escapeBackticks(RedactSensitiveText(target)),
			}
			if description := firstCardNonEmpty(strings.TrimSpace(source.Snippet), strings.TrimSpace(source.Source)); description != "" {
				result["description"] = escapeBackticks(RedactSensitiveText(description))
			}
			results = append(results, result)
		}

		if len(results) > 0 {
			query := firstCardNonEmpty(
				strings.TrimSpace(data.Query),
				strings.TrimSpace(data.Input),
				strings.TrimSpace(data.Title),
				strings.TrimSpace(data.TargetURL),
			)

			card := map[string]interface{}{
				"type":        "search",
				"query":       escapeBackticks(RedactSensitiveText(query)),
				"total_count": len(results),
				"results":     results,
			}
			if query != "" {
				card["id"] = "web-query-search-" + url.QueryEscape(query)
			}
			return card
		}
	}

	title := strings.TrimSpace(data.Title)
	if title == "" {
		title = "web_query"
	}
	status := "info"
	switch strings.ToLower(strings.TrimSpace(data.Status)) {
	case "ok":
		status = "success"
	case "partial":
		status = "warning"
	case "needs_browser", "error":
		status = "warning"
	}

	card := map[string]interface{}{
		"type":    "result",
		"title":   escapeBackticks(RedactSensitiveText(title)),
		"status":  status,
		"message": escapeBackticks(RedactSensitiveText(bodyText)),
	}
	cardURL := strings.TrimSpace(data.FinalURL)
	if cardURL == "" {
		cardURL = strings.TrimSpace(data.TargetURL)
	}
	if cardURL != "" {
		card["id"] = "web-query-" + url.QueryEscape(cardURL)
	}

	if len(data.Warnings) > 0 {
		first := data.Warnings[0]
		if warning := strings.TrimSpace(first.Message); warning != "" {
			card["warning"] = escapeBackticks(RedactSensitiveText(warning))
		}
		if code := strings.TrimSpace(first.Code); code != "" {
			card["warning_code"] = code
		}
	}

	details := make([]map[string]interface{}, 0, 5)
	if cardURL != "" {
		details = append(details, map[string]interface{}{
			"label":    "url",
			"value":    escapeBackticks(RedactSensitiveText(cardURL)),
			"copyable": true,
		})
	}
	if source := strings.TrimSpace(data.Transcript.Source); source != "" {
		details = append(details, map[string]interface{}{
			"label": "transcript_source",
			"value": escapeBackticks(RedactSensitiveText(source)),
		})
	}
	if lang := firstCardNonEmpty(strings.TrimSpace(data.Transcript.Language), strings.TrimSpace(data.Media.Language)); lang != "" {
		details = append(details, map[string]interface{}{
			"label": "language",
			"value": escapeBackticks(RedactSensitiveText(lang)),
		})
	}
	if platform := strings.TrimSpace(data.Media.Platform); platform != "" {
		details = append(details, map[string]interface{}{
			"label": "platform",
			"value": escapeBackticks(RedactSensitiveText(platform)),
		})
	}
	if mediaSummary := strings.TrimSpace(data.Media.Summary); mediaSummary != "" {
		details = append(details, map[string]interface{}{
			"label":     "media_summary",
			"value":     escapeBackticks(RedactSensitiveText(mediaSummary)),
			"multiline": true,
		})
	}
	if pageSummary := strings.TrimSpace(data.Page.Content); pageSummary != "" {
		details = append(details, map[string]interface{}{
			"label":     "page_summary",
			"value":     escapeBackticks(RedactSensitiveText(pageSummary)),
			"multiline": true,
		})
	}
	if len(data.Sources) > 0 {
		lines := make([]string, 0, len(data.Sources))
		for idx, source := range data.Sources {
			if idx >= 4 {
				break
			}
			label := strings.TrimSpace(source.Title)
			if label == "" {
				label = strings.TrimSpace(source.FinalURL)
			}
			if label == "" {
				label = strings.TrimSpace(source.URL)
			}
			if label == "" {
				continue
			}
			prefix := ""
			if source.Selected {
				prefix = "* "
			}
			target := strings.TrimSpace(source.FinalURL)
			if target == "" {
				target = strings.TrimSpace(source.URL)
			}
			if target != "" {
				lines = append(lines, prefix+label+" - "+target)
			} else {
				lines = append(lines, prefix+label)
			}
		}
		if len(lines) > 0 {
			details = append(details, map[string]interface{}{
				"label":     "sources",
				"value":     escapeBackticks(RedactSensitiveText(strings.Join(lines, "\n"))),
				"multiline": true,
			})
		}
	}
	if len(details) > 0 {
		card["details"] = details
	}
	if len(data.Media.Items) == 1 {
		image := map[string]interface{}{
			"src": strings.TrimSpace(data.Media.Items[0].URL),
		}
		if caption := firstCardNonEmpty(strings.TrimSpace(data.Media.Items[0].Analysis), strings.TrimSpace(data.Media.Items[0].Alt)); caption != "" {
			image["caption"] = escapeBackticks(RedactSensitiveText(caption))
		}
		if src, ok := image["src"].(string); ok && src != "" {
			card["image"] = src
			card["images"] = []map[string]interface{}{image}
		}
	} else if len(data.Media.Items) > 1 {
		images := make([]map[string]interface{}, 0, len(data.Media.Items))
		for _, item := range data.Media.Items {
			src := strings.TrimSpace(item.URL)
			if src == "" {
				continue
			}
			image := map[string]interface{}{"src": src}
			if caption := firstCardNonEmpty(strings.TrimSpace(item.Analysis), strings.TrimSpace(item.Alt)); caption != "" {
				image["caption"] = escapeBackticks(RedactSensitiveText(caption))
			}
			images = append(images, image)
		}
		if len(images) > 0 {
			card["images"] = images
		}
	}
	if data.NextAction == "retry_browser" && cardURL != "" {
		card["actions"] = []map[string]interface{}{{
			"id":      "use_browser",
			"label":   "Use browser",
			"variant": "primary",
			"form_data": map[string]interface{}{
				"url": cardURL,
			},
		}}
	}

	return card
}

func unwrapUntrustedToolCardContent(content, toolName string) string {
	var payload map[string]interface{}
	if json.Unmarshal([]byte(content), &payload) != nil {
		return content
	}
	if !strings.EqualFold(strings.TrimSpace(formatValue(payload["tool"])), strings.TrimSpace(toolName)) {
		return content
	}
	data, ok := payload["data"]
	if !ok {
		return content
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return content
	}
	return string(encoded)
}

func deepResearchCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	if hasNonEmptyError(data) {
		return buildErrorCard("result", "deep_research", data)
	}

	card := map[string]interface{}{
		"type": "deep-research",
	}
	if id := deepResearchCardID(data); id != "" {
		card["id"] = id
	}
	for _, key := range []string{
		"job_id",
		"conversation_id",
		"query",
		"mode",
		"progress",
		"iteration",
		"latest_gap",
		"latest_action",
		"answer",
		"confidence",
		"evidence_count",
		"iterations",
		"stop_reason",
		"citations",
		"open_questions",
		"support_count",
		"conflict_count",
		"has_conflict",
		"citation_coverage",
		"entity_disambiguation",
		"stage_errors",
		"timeline_sections",
		"research_trace",
		"verification_summary",
		"strict_entity",
		"time_windows",
		"report_style",
		"status",
	} {
		if v, ok := data[key]; ok {
			card[key] = v
		}
	}
	return card
}

func deepResearchCardID(data map[string]interface{}) string {
	jobID := strings.TrimSpace(formatValue(data["job_id"]))
	if jobID != "" {
		return "deep-research-" + url.QueryEscape(jobID)
	}
	query := strings.TrimSpace(formatValue(data["query"]))
	if query != "" {
		return "deep-research-" + url.QueryEscape(query)
	}
	return ""
}

func calculatorCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	if hasNonEmptyError(data) {
		return buildErrorCard("result", "Calculator", data)
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
		return buildErrorCard("result", "File Read", data)
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
		if errMsg := sanitizedErrorMessage(data); errMsg != "" {
			msg = errMsg
		} else {
			msg = noErrorDetailsText
		}
	} else if m, ok := data["message"].(string); ok {
		msg = m
	}

	displayPath := strings.TrimSpace(formatValue(data["original_path"]))
	resolvedPath := strings.TrimSpace(formatValue(data["path"]))
	absolutePath := strings.TrimSpace(formatValue(data["absolute_path"]))
	if displayPath == "" {
		displayPath = resolvedPath
	}
	details := []map[string]interface{}{}
	if displayPath != "" {
		pathDetail := map[string]interface{}{
			"label": "Path",
			"value": escapeBackticks(RedactSensitiveText(displayPath)),
		}
		if status == "success" && isGenericCardLocalAbsolutePath(absolutePath) {
			pathDetail["reveal_path"] = absolutePath
		}
		details = append(details, pathDetail)
	}

	card := map[string]interface{}{
		"type":    "result",
		"title":   "File Write",
		"status":  status,
		"message": msg,
		"details": details,
	}
	if resolvedPath != "" || absolutePath != "" || displayPath != "" {
		artifact := map[string]interface{}{}
		switch {
		case resolvedPath != "":
			artifact["path"] = resolvedPath
		case displayPath != "":
			artifact["path"] = displayPath
		}
		if absolutePath != "" {
			artifact["local_path"] = absolutePath
		}
		if displayPath != "" {
			artifact["original_path"] = displayPath
		}
		if len(artifact) > 0 {
			card["artifacts"] = []map[string]interface{}{artifact}
		}
	}
	return card
}

func lsCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return GenericCard("ls", content)
	}
	if hasNonEmptyError(data) {
		return buildToolErrorCard("ls", data)
	}

	toInt := func(v interface{}) int {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		default:
			return 0
		}
	}

	basePath := strings.TrimSpace(formatValue(data["base_path"]))
	if basePath == "" {
		basePath = "."
	}
	count := toInt(data["count"])
	maxDepth := toInt(data["max_depth"])
	maxEntries := toInt(data["max_entries"])
	includeHidden, _ := data["include_hidden"].(bool)
	truncated, _ := data["truncated"].(bool)

	entriesRaw, _ := data["entries"].([]interface{})
	previewLines := make([]string, 0, min(len(entriesRaw), lsPreviewEntries))
	for i := 0; i < len(entriesRaw) && len(previewLines) < lsPreviewEntries; i++ {
		entry, ok := entriesRaw[i].(map[string]interface{})
		if !ok {
			continue
		}
		path := strings.TrimSpace(formatValue(entry["path"]))
		if path == "" {
			continue
		}
		entryType := strings.TrimSpace(formatValue(entry["type"]))
		if entryType == "" {
			entryType = "file"
		}
		if entryType == "dir" {
			previewLines = append(previewLines, "[dir] "+path)
			continue
		}
		size := toInt(entry["size"])
		if size > 0 {
			previewLines = append(previewLines, fmt.Sprintf("[file] %s (%d B)", path, size))
		} else {
			previewLines = append(previewLines, "[file] "+path)
		}
	}

	message := fmt.Sprintf("%d entries in %s", count, basePath)
	if count == 1 {
		message = "1 entry in " + basePath
	} else if count == 0 {
		message = "No entries in " + basePath
	}
	if truncated {
		message = fmt.Sprintf("Showing first %d entries in %s (more omitted)", count, basePath)
	}

	details := []map[string]interface{}{
		{"label": "path", "value": basePath},
		{"label": "count", "value": fmt.Sprintf("%d", count)},
		{"label": "max_depth", "value": fmt.Sprintf("%d", maxDepth)},
		{"label": "include_hidden", "value": fmt.Sprintf("%t", includeHidden)},
	}
	if maxEntries > 0 {
		details = append(details, map[string]interface{}{
			"label": "max_entries",
			"value": fmt.Sprintf("%d", maxEntries),
		})
	}
	if len(previewLines) > 0 {
		details = append(details, map[string]interface{}{
			"label":     "entries",
			"value":     strings.Join(previewLines, "\n"),
			"multiline": true,
		})
	}
	if count > len(previewLines) {
		details = append(details, map[string]interface{}{
			"label": "entries_hidden_in_card",
			"value": fmt.Sprintf("%d", count-len(previewLines)),
		})
	}

	card := map[string]interface{}{
		"type":    "result",
		"title":   "ls",
		"status":  "success",
		"message": message,
		"details": details,
	}
	if truncated {
		card["warning_code"] = "output_truncated"
		card["warning"] = "Listing was truncated; narrow the path or increase max_entries."
	}
	return card
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
		return buildErrorCard("result", "Memory Search", data)
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
		return buildErrorCard("result", "Reminder", data)
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
		return buildErrorCard("result", "analyze", data)
	}
	topic, _ := data["topic"].(string)
	reportURL, _ := data["report_url"].(string)

	card := map[string]interface{}{
		"type":   "result",
		"title":  "analyze",
		"status": "success",
	}
	if id := analyzeCardID(data); id != "" {
		card["id"] = id
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

func analyzeCardID(data map[string]interface{}) string {
	reportURL := strings.TrimSpace(formatValue(data["report_url"]))
	if reportURL != "" {
		return "analyze-" + url.QueryEscape(reportURL)
	}
	topic := strings.TrimSpace(formatValue(data["topic"]))
	if topic != "" {
		return "analyze-" + url.QueryEscape(topic)
	}
	return ""
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

func stringSliceValue(v interface{}) []string {
	switch typed := v.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			value := strings.TrimSpace(formatValue(item))
			if value == "" {
				continue
			}
			out = append(out, value)
		}
		return out
	default:
		return nil
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

		images := make([]map[string]interface{}, 0, 2)
		imageSeen := map[string]struct{}{}
		keys := make([]string, 0, len(data))
		for k, v := range data {
			if k == "message" || k == "error" {
				continue
			}
			if isGenericCardImageField(k) {
				before := len(images)
				images = extractGenericCardImages(images, imageSeen, v, k)
				if len(images) > before {
					continue
				}
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
		if len(images) == 1 {
			if src, ok := images[0]["src"].(string); ok && strings.TrimSpace(src) != "" {
				card["image"] = src
			}
		} else if len(images) > 1 {
			card["images"] = images
		}

		if _, hasMessage := card["message"]; !hasMessage && len(details) == 0 && len(images) == 0 {
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
		card := map[string]interface{}{
			"type":   "ui-review",
			"status": "error",
			"actions": []map[string]interface{}{
				{"id": "recheck", "label": "Retry", "variant": "primary"},
			},
		}
		if errMsg := sanitizedErrorMessage(data); errMsg != "" {
			card["message"] = errMsg
		} else {
			card["message"] = noErrorDetailsText
		}
		return card
	}

	card := map[string]interface{}{
		"type": "ui-review",
	}

	// Stable card ID from URL
	if reviewURL, ok := data["url"].(string); ok && reviewURL != "" {
		card["id"] = "ui-review-" + url.QueryEscape(reviewURL)
	}

	// Copy relevant fields
	for _, key := range []string{
		"url", "overall", "pass", "threshold",
		"visual", "functional", "accessibility",
		"issues", "suggestions", "steps", "viewports",
		"media_url", "thumbnail_url", "screenshots", "device", "channel", "human",
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

func imageCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return GenericCard("image", content)
	}

	task, _ := data["task"].(map[string]interface{})
	images := collectImageCardImages(data, task)

	errMsg := sanitizedErrorMessage(data)
	if errMsg == "" && task != nil {
		if taskErr := strings.TrimSpace(formatValue(task["error"])); taskErr != "" {
			errMsg = escapeBackticks(RedactSensitiveText(taskErr))
		}
	}

	message := strings.TrimSpace(formatValue(data["message"]))
	if message == "" && task != nil {
		message = strings.TrimSpace(formatValue(task["message"]))
	}
	if message == "" {
		message = errMsg
	}

	taskID := strings.TrimSpace(formatValue(data["task_id"]))
	if taskID == "" && task != nil {
		taskID = strings.TrimSpace(formatValue(task["id"]))
	}

	rawStatus := strings.TrimSpace(formatValue(data["status"]))
	if rawStatus == "" && task != nil {
		rawStatus = strings.TrimSpace(formatValue(task["status"]))
	}
	status := normalizeMediaGenerateStatus(rawStatus, len(images) > 0, errMsg != "")

	// If there's nothing visual and no async task to track, keep the generic card.
	if len(images) == 0 && taskID == "" && status != "error" {
		return GenericCard("image", content)
	}

	idSeed := taskID
	if idSeed == "" && len(images) > 0 {
		idSeed = strings.TrimSpace(formatValue(images[0]["src"]))
	}
	if idSeed == "" {
		idSeed = "latest"
	}

	card := map[string]interface{}{
		"type":       "media-generate",
		"id":         "image-generate-" + url.QueryEscape(idSeed),
		"media_type": "image",
		"status":     status,
	}
	if taskID != "" {
		card["task_id"] = taskID
	}
	if len(images) > 0 {
		card["images"] = images
	}
	if message != "" {
		card["message"] = message
	}
	if status == "error" && message == "" {
		card["message"] = noErrorDetailsText
	}
	if elapsed, ok := data["elapsed_ms"]; ok {
		card["elapsed_ms"] = elapsed
	}

	return card
}

func normalizeMediaGenerateStatus(raw string, hasImages bool, hasError bool) string {
	if hasError {
		return "error"
	}

	status := strings.ToLower(strings.TrimSpace(raw))
	switch status {
	case "success", "succeeded", "completed", "done", "ok":
		return "success"
	case "error", "failed", "fail", "cancelled", "canceled":
		return "error"
	case "processing", "pending", "queued", "running", "in_progress", "generating", "created":
		return "generating"
	}

	if strings.Contains(status, "fail") || strings.Contains(status, "error") || strings.Contains(status, "cancel") {
		return "error"
	}
	if strings.Contains(status, "success") || strings.Contains(status, "succeed") || strings.Contains(status, "complete") || strings.Contains(status, "done") {
		return "success"
	}
	if strings.Contains(status, "process") || strings.Contains(status, "pend") || strings.Contains(status, "queue") || strings.Contains(status, "run") || strings.Contains(status, "generat") || strings.Contains(status, "progress") {
		return "generating"
	}
	if hasImages {
		return "success"
	}
	return "generating"
}

func collectImageCardImages(data map[string]interface{}, task map[string]interface{}) []map[string]interface{} {
	images := make([]map[string]interface{}, 0)
	seen := make(map[string]struct{})

	addImage := func(src, thumbnail, caption string) {
		src = strings.TrimSpace(src)
		if src == "" {
			return
		}
		if _, ok := seen[src]; ok {
			return
		}
		seen[src] = struct{}{}

		image := map[string]interface{}{"src": src}
		if strings.TrimSpace(thumbnail) != "" {
			image["thumbnail"] = strings.TrimSpace(thumbnail)
		}
		if strings.TrimSpace(caption) != "" {
			image["caption"] = strings.TrimSpace(caption)
		}
		images = append(images, image)
	}

	addFromList := func(raw interface{}) {
		entries, ok := raw.([]interface{})
		if !ok {
			return
		}
		for _, entry := range entries {
			switch item := entry.(type) {
			case string:
				addImage(item, "", "")
			case map[string]interface{}:
				src := firstNonEmptyString(item, "src", "url", "original_url", "image_url")
				thumbnail := firstNonEmptyString(item, "thumbnail", "thumbnail_url")
				caption := firstNonEmptyString(item, "caption", "revised_prompt", "prompt", "message")
				addImage(src, thumbnail, caption)
			}
		}
	}

	addFromList(data["images"])
	addFromList(data["image_urls"])
	if task != nil {
		addFromList(task["outputs"])
		addFromList(task["images"])
		addFromList(task["image_urls"])
	}

	return images
}

func firstNonEmptyString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := data[key]
		if !ok {
			continue
		}
		if text := stringFromAny(value); text != "" {
			return text
		}
	}
	return ""
}

func stringFromAny(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64, float32, int, int64, int32, uint, uint64, uint32, bool:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	default:
		return ""
	}
}

func pptCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return GenericCard("ppt", content)
	}
	if hasNonEmptyError(data) {
		return buildToolErrorCard("ppt", data)
	}
	status := "success"
	if skipped, _ := data["skipped"].(bool); skipped {
		status = "error"
	}
	images := make([]map[string]interface{}, 0)
	urls, _ := data["image_urls"].([]interface{})
	thumbs, _ := data["thumbnail_urls"].([]interface{})
	for i, raw := range urls {
		src := strings.TrimSpace(formatValue(raw))
		if src == "" {
			continue
		}
		image := map[string]interface{}{"src": src}
		if i < len(thumbs) {
			thumb := strings.TrimSpace(formatValue(thumbs[i]))
			if thumb != "" {
				image["thumbnail"] = thumb
			}
		}
		if summary := strings.TrimSpace(formatValue(data["review_summary"])); summary != "" {
			image["caption"] = summary
		}
		images = append(images, image)
	}
	card := map[string]interface{}{
		"type":       "media-generate",
		"id":         "ppt-asset-" + url.QueryEscape(strings.TrimSpace(formatValue(data["task_id"]))),
		"media_type": "image",
		"status":     status,
	}
	if taskID := strings.TrimSpace(formatValue(data["task_id"])); taskID != "" {
		card["task_id"] = taskID
	}
	if len(images) > 0 {
		card["images"] = images
	}
	message := strings.TrimSpace(formatValue(data["review_summary"]))
	if message == "" {
		message = strings.TrimSpace(formatValue(data["skip_reason"]))
	}
	if message == "" {
		message = strings.TrimSpace(formatValue(data["error"]))
	}
	if message != "" {
		card["message"] = message
	}
	return card
}

func slideAssetCard(content string) map[string]interface{} {
	return pptCard(content)
}
