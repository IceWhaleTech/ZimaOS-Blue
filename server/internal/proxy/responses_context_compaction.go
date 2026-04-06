package proxy

import (
	"regexp"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	responsesContinuationAssistantGeneralMaxRunes = 1600
	responsesContinuationAssistantChoiceMaxRunes  = 700
	responsesContinuationToolOutputMaxRunes       = 12000
	responsesContinuationToolArgumentsMaxRunes    = 6000
	responsesContinuationTrimMarker               = "\n...[context trimmed]...\n"
	responsesContinuationToolTrimMarker           = "\n...[tool content trimmed]...\n"
)

var (
	reContinuationChoice             *regexp.Regexp
	reContinuationContextCue         *regexp.Regexp
	reContinuationOrdinalCue         *regexp.Regexp
	reAssistantOptionLine            *regexp.Regexp
	responsesContinuationRegexesOnce sync.Once
)

func ensureResponsesContinuationRegexes() {
	responsesContinuationRegexesOnce.Do(func() {
		reContinuationChoice = regexp.MustCompile(`(?i)^(?:[a-e](?:[\.\)])?|[1-9][0-9]?)$`)
		reContinuationContextCue = regexp.MustCompile(`(?i)\b(above|previous|same|continue|that|this|former|latter)\b|上面|上文|刚才|之前|继续|这个|那个|同上`)
		reContinuationOrdinalCue = regexp.MustCompile(`(?i)\b(first|second|third|fourth|fifth|option)\b|第[一二三四五六七八九十0-9]+个|选项`)
		reAssistantOptionLine = regexp.MustCompile(`(?m)^[ \t]*(?:[A-Ea-e]|[1-9][0-9]?)[\.\)]\s+\S.*$`)
	})
}

type ResponsesAssistantCompressionMode string

const (
	ResponsesAssistantCompressionModeGeneral ResponsesAssistantCompressionMode = "general"
	ResponsesAssistantCompressionModeChoice  ResponsesAssistantCompressionMode = "choice"
)

type ResponsesAssistantCompressionInput struct {
	AssistantText string
	UserText      string
	Mode          ResponsesAssistantCompressionMode
	MaxRunes      int
}

// ResponsesContextCompressor is an abstraction layer for continuation context
// compression. The default implementation is heuristic; callers can plug in a
// local small model compressor (for example qwen 0.8B) later.
type ResponsesContextCompressor interface {
	CompressAssistantContext(input ResponsesAssistantCompressionInput) (string, error)
}

type heuristicResponsesContextCompressor struct{}

func (heuristicResponsesContextCompressor) CompressAssistantContext(input ResponsesAssistantCompressionInput) (string, error) {
	assistantText := strings.TrimSpace(input.AssistantText)
	if assistantText == "" {
		return "", nil
	}
	maxRunes := input.MaxRunes
	if maxRunes <= 0 {
		return "", nil
	}
	if input.Mode == ResponsesAssistantCompressionModeChoice {
		ensureResponsesContinuationRegexes()
		if optionLines := strings.TrimSpace(strings.Join(reAssistantOptionLine.FindAllString(assistantText, 10), "\n")); optionLines != "" {
			return truncateContinuationRunes(optionLines, maxRunes, responsesContinuationTrimMarker), nil
		}
	}
	return truncateContinuationRunes(assistantText, maxRunes, responsesContinuationTrimMarker), nil
}

var defaultResponsesContinuationCompactor = newResponsesContinuationCompactor(nil)

type responsesContinuationCompactor struct {
	compressorMu sync.RWMutex
	compressor   ResponsesContextCompressor
}

func newResponsesContinuationCompactor(compressor ResponsesContextCompressor) *responsesContinuationCompactor {
	c := &responsesContinuationCompactor{}
	c.SetCompressor(compressor)
	return c
}

func (c *responsesContinuationCompactor) SetCompressor(compressor ResponsesContextCompressor) {
	if compressor == nil {
		compressor = heuristicResponsesContextCompressor{}
	}
	c.compressorMu.Lock()
	c.compressor = compressor
	c.compressorMu.Unlock()
}

func (c *responsesContinuationCompactor) compressorOrDefault() ResponsesContextCompressor {
	c.compressorMu.RLock()
	compressor := c.compressor
	c.compressorMu.RUnlock()
	if compressor != nil {
		return compressor
	}
	return heuristicResponsesContextCompressor{}
}

func (c *responsesContinuationCompactor) TrimInput(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	if strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String()) == "" {
		return body
	}
	items, ok := responsesInputItems(body)
	if !ok {
		return body
	}
	if len(items) == 0 {
		return body
	}

	lastAssistant := -1
	hasToolInput := false
	for i := len(items) - 1; i >= 0; i-- {
		itemType := strings.TrimSpace(items[i].Get("type").String())
		if itemType == "function_call_output" || itemType == "function_call" {
			hasToolInput = true
		}
		if strings.EqualFold(strings.TrimSpace(items[i].Get("role").String()), "assistant") {
			lastAssistant = i
			break
		}
	}

	// REGRESSION-GUARD: Tool continuation payloads are often "tool-only"
	// (function_call_output + optional latest user follow-up, no assistant role).
	// Some chat->responses conversion paths accidentally carry stale user history
	// in this shape. Do NOT fall back to applyOverflowGuards() here: that keeps
	// old user turns and re-sends oversized history on continuation rounds,
	// which can trigger relays to drop previous_response_id on the first tool
	// follow-up turn. Keep only tool payload + latest user follow-up.
	if lastAssistant < 0 && hasToolInput {
		return c.compactToolOnlyContinuationInput(body, items)
	}

	start := len(items) - 1
	if lastAssistant >= 0 {
		if lastAssistant+1 >= len(items) {
			out, err := sjson.DeleteBytes(body, "input")
			if err != nil {
				return body
			}
			return out
		}
		if shouldCarryAssistantContextForContinuation(items[lastAssistant+1:]) {
			start = lastAssistant
		} else {
			start = lastAssistant + 1
		}
	}

	trimmedRaw := make([]string, 0, len(items)-start)
	for i := start; i < len(items); i++ {
		raw := strings.TrimSpace(items[i].Raw)
		if raw == "" || raw == "null" {
			continue
		}
		if lastAssistant >= 0 && i == lastAssistant && start == lastAssistant {
			raw = c.compactAssistantItem(raw, items[i], items[lastAssistant+1:])
		} else {
			raw = compactOverflowForContinuationItem(raw, items[i])
		}
		if raw == "" || raw == "null" {
			continue
		}
		trimmedRaw = append(trimmedRaw, raw)
	}

	return setResponsesInputRaw(body, trimmedRaw)
}

func (c *responsesContinuationCompactor) compactToolOnlyContinuationInput(body []byte, items []gjson.Result) []byte {
	if len(items) == 0 {
		return body
	}

	latestUserIdx := -1
	hasFunctionCallOutput := false
	for i := len(items) - 1; i >= 0; i-- {
		itemType := strings.TrimSpace(items[i].Get("type").String())
		if itemType == "function_call_output" {
			hasFunctionCallOutput = true
		}
		if latestUserIdx >= 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(items[i].Get("role").String()), "user") {
			latestUserIdx = i
		}
	}

	trimmedRaw := make([]string, 0, len(items))
	for i, item := range items {
		itemType := strings.TrimSpace(item.Get("type").String())
		keep := false
		switch itemType {
		case "function_call_output":
			keep = true
		case "function_call":
			// REGRESSION-GUARD: Prefer function_call_output over function_call
			// when both are present in continuation payloads; echoing both can
			// bloat input and confuse some OpenAI-compatible relays.
			// Keep function_call only when outputs are absent so we don't drop
			// all tool context.
			keep = !hasFunctionCallOutput
		default:
			role := strings.ToLower(strings.TrimSpace(item.Get("role").String()))
			keep = role == "user" && i == latestUserIdx
		}
		if !keep {
			continue
		}
		raw := strings.TrimSpace(item.Raw)
		if raw == "" || raw == "null" {
			continue
		}
		raw = compactOverflowForContinuationItem(raw, item)
		if raw == "" || raw == "null" {
			continue
		}
		trimmedRaw = append(trimmedRaw, raw)
	}

	if len(trimmedRaw) == 0 {
		return c.applyOverflowGuards(body, items)
	}
	return setResponsesInputRaw(body, trimmedRaw)
}

func (c *responsesContinuationCompactor) InjectAssistantContext(body []byte, assistantText string) []byte {
	if len(body) == 0 {
		return body
	}
	if strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String()) == "" {
		return body
	}
	assistantText = strings.TrimSpace(assistantText)
	if assistantText == "" {
		return body
	}

	items, ok := responsesInputItems(body)
	if !ok {
		return body
	}
	if len(items) == 0 {
		return body
	}

	hasAssistant := false
	hasToolPayload := false
	hasUser := false
	for _, item := range items {
		itemType := strings.TrimSpace(item.Get("type").String())
		if itemType == "function_call_output" || itemType == "function_call" {
			hasToolPayload = true
			continue
		}
		role := strings.ToLower(strings.TrimSpace(item.Get("role").String()))
		if role == "assistant" {
			hasAssistant = true
		}
		if role == "user" {
			hasUser = true
		}
	}
	if hasAssistant || hasToolPayload || !hasUser {
		return c.applyOverflowGuards(body, items)
	}
	if !shouldCarryAssistantContextForContinuation(items) {
		return c.applyOverflowGuards(body, items)
	}

	compressedAssistant := c.compressAssistantContextForContinuation(assistantText, items)
	if compressedAssistant == "" {
		return c.applyOverflowGuards(body, items)
	}

	escaped, err := json.Marshal(compressedAssistant)
	if err != nil {
		return c.applyOverflowGuards(body, items)
	}
	assistantItem := `{"role":"assistant","content":[{"type":"input_text","text":` + string(escaped) + `}]}`

	trimmedRaw := make([]string, 0, len(items)+1)
	trimmedRaw = append(trimmedRaw, assistantItem)
	for _, item := range items {
		raw := strings.TrimSpace(item.Raw)
		if raw == "" || raw == "null" {
			continue
		}
		raw = compactOverflowForContinuationItem(raw, item)
		if raw == "" || raw == "null" {
			continue
		}
		trimmedRaw = append(trimmedRaw, raw)
	}
	return setResponsesInputRaw(body, trimmedRaw)
}

func (c *responsesContinuationCompactor) applyOverflowGuards(body []byte, items []gjson.Result) []byte {
	trimmedRaw := make([]string, 0, len(items))
	changed := false
	for _, item := range items {
		raw := strings.TrimSpace(item.Raw)
		if raw == "" || raw == "null" {
			continue
		}
		compacted := compactOverflowForContinuationItem(raw, item)
		if compacted != raw {
			changed = true
		}
		if compacted == "" || compacted == "null" {
			continue
		}
		trimmedRaw = append(trimmedRaw, compacted)
	}
	if !changed {
		return body
	}
	return setResponsesInputRaw(body, trimmedRaw)
}

func (c *responsesContinuationCompactor) compactAssistantItem(raw string, item gjson.Result, userItems []gjson.Result) string {
	text := strings.TrimSpace(extractResponsesInputMessageText(item))
	if text == "" {
		return raw
	}
	compressed := c.compressAssistantContextForContinuation(text, userItems)
	if compressed == "" {
		return raw
	}
	escaped, err := json.Marshal(compressed)
	if err != nil {
		return raw
	}
	return `{"role":"assistant","content":[{"type":"input_text","text":` + string(escaped) + `}]}`
}

func (c *responsesContinuationCompactor) compressAssistantContextForContinuation(assistantText string, userItems []gjson.Result) string {
	assistantText = strings.TrimSpace(assistantText)
	if assistantText == "" {
		return ""
	}

	combinedUser := collectContinuationUserText(userItems)
	mode := ResponsesAssistantCompressionModeGeneral
	maxRunes := responsesContinuationAssistantGeneralMaxRunes
	if isLikelyContextDependentContinuationText(combinedUser) {
		mode = ResponsesAssistantCompressionModeChoice
		maxRunes = responsesContinuationAssistantChoiceMaxRunes
	}

	input := ResponsesAssistantCompressionInput{
		AssistantText: assistantText,
		UserText:      combinedUser,
		Mode:          mode,
		MaxRunes:      maxRunes,
	}

	compressed, err := c.compressorOrDefault().CompressAssistantContext(input)
	if err != nil || strings.TrimSpace(compressed) == "" {
		compressed = truncateContinuationRunes(assistantText, maxRunes, responsesContinuationTrimMarker)
	}
	compressed = strings.TrimSpace(compressed)
	if compressed == "" {
		return ""
	}
	return truncateContinuationRunes(compressed, maxRunes, responsesContinuationTrimMarker)
}

func responsesInputItems(body []byte) ([]gjson.Result, bool) {
	input := gjson.GetBytes(body, "input")
	if !input.Exists() || !input.IsArray() {
		return nil, false
	}
	return input.Array(), true
}

func setResponsesInputRaw(body []byte, rawItems []string) []byte {
	if len(rawItems) == 0 {
		out, err := sjson.DeleteBytes(body, "input")
		if err != nil {
			return body
		}
		return out
	}
	out, err := sjson.SetRawBytes(body, "input", []byte("["+strings.Join(rawItems, ",")+"]"))
	if err != nil {
		return body
	}
	return out
}

func compactOverflowForContinuationItem(raw string, item gjson.Result) string {
	itemType := strings.TrimSpace(item.Get("type").String())
	switch itemType {
	case "function_call_output":
		output := item.Get("output")
		if !output.Exists() || output.Type != gjson.String {
			return raw
		}
		trimmed := truncateContinuationRunes(output.String(), responsesContinuationToolOutputMaxRunes, responsesContinuationToolTrimMarker)
		if trimmed == output.String() {
			return raw
		}
		out, err := sjson.SetBytes([]byte(raw), "output", trimmed)
		if err != nil {
			return raw
		}
		return strings.TrimSpace(string(out))
	case "function_call":
		args := item.Get("arguments")
		if !args.Exists() || args.Type != gjson.String {
			return raw
		}
		name := strings.ToLower(strings.TrimSpace(item.Get("name").String()))
		if compacted, ok := compactWriteFunctionCallArguments(name, args.String()); ok {
			out, err := sjson.SetBytes([]byte(raw), "arguments", compacted)
			if err != nil {
				return raw
			}
			return strings.TrimSpace(string(out))
		}
		trimmed := truncateContinuationRunes(args.String(), responsesContinuationToolArgumentsMaxRunes, responsesContinuationToolTrimMarker)
		if trimmed == args.String() {
			return raw
		}
		out, err := sjson.SetBytes([]byte(raw), "arguments", trimmed)
		if err != nil {
			return raw
		}
		return strings.TrimSpace(string(out))
	default:
		return raw
	}
}

func compactWriteFunctionCallArguments(name, rawArgs string) (string, bool) {
	if !isContinuationWriteToolName(name) {
		return "", false
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(rawArgs), &payload); err != nil {
		return "", false
	}

	content, ok := payload["content"].(string)
	if !ok {
		return "", false
	}
	if len([]rune(rawArgs)) <= responsesContinuationToolArgumentsMaxRunes && len(content) <= responsesContinuationToolArgumentsMaxRunes/2 {
		return "", false
	}

	payload["content"] = "[omitted large write payload already executed]"
	payload["content_chars"] = len([]rune(content))
	payload["content_bytes"] = len(content)

	normalized, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	return string(normalized), true
}

func isContinuationWriteToolName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "write", "file_write", "write_begin", "write_chunk", "write_commit", "write_abort":
		return true
	default:
		return false
	}
}

func normalizeContinuationContextText(s string) string {
	trimmed := strings.TrimSpace(s)
	trimmed = strings.Trim(trimmed, " \t\r\n.,!?;:，。！？；：、~～`'\"“”‘’()（）[]【】")
	return strings.TrimSpace(trimmed)
}

func isLikelyContextDependentContinuationText(text string) bool {
	s := normalizeContinuationContextText(text)
	if s == "" {
		return true
	}
	ensureResponsesContinuationRegexes()
	if reContinuationChoice.MatchString(s) {
		return true
	}
	if reContinuationContextCue.MatchString(s) || reContinuationOrdinalCue.MatchString(s) {
		return true
	}
	// Short replies are often acknowledgements or terse follow-ups that rely on
	// prior context; longer standalone asks should avoid carrying assistant text.
	return len([]rune(s)) <= 22
}

func collectContinuationUserText(items []gjson.Result) string {
	var sb strings.Builder
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Get("role").String()), "user") {
			text := extractResponsesInputMessageText(item)
			if strings.TrimSpace(text) == "" {
				continue
			}
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(text)
		}
	}
	return strings.TrimSpace(sb.String())
}

func shouldCarryAssistantContextForContinuation(items []gjson.Result) bool {
	sawUser := false
	hasToolPayload := false
	for _, item := range items {
		itemType := strings.TrimSpace(item.Get("type").String())
		if itemType == "function_call_output" || itemType == "function_call" {
			hasToolPayload = true
			continue
		}
		role := strings.ToLower(strings.TrimSpace(item.Get("role").String()))
		if role != "user" {
			continue
		}
		sawUser = true
		text := strings.TrimSpace(extractResponsesInputMessageText(item))
		// Empty/multimodal user payload is ambiguous: keep assistant context.
		if text == "" {
			return true
		}
		if isLikelyContextDependentContinuationText(text) {
			return true
		}
	}
	if !sawUser && hasToolPayload {
		return false
	}
	return !sawUser
}

func truncateContinuationRunes(s string, maxRunes int, marker string) string {
	if maxRunes <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	if maxRunes <= 120 {
		return string(r[len(r)-maxRunes:])
	}
	markerRunes := []rune(marker)
	head := maxRunes / 3
	tail := maxRunes - head - len(markerRunes)
	if tail < 0 {
		tail = 0
	}
	if head < 0 {
		head = 0
	}
	if head+tail > len(r) {
		tail = len(r) - head
		if tail < 0 {
			tail = 0
		}
	}
	return string(r[:head]) + marker + string(r[len(r)-tail:])
}
