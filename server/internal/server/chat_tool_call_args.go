package server

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func mergeStreamingToolCallArguments(current, next string) string {
	trimmedNext := strings.TrimSpace(next)
	if trimmedNext == "" {
		return current
	}

	trimmedCurrent := strings.TrimSpace(current)
	if trimmedCurrent == "" {
		return next
	}

	if isEmptyStructuredToolArgs(trimmedCurrent) && !isEmptyStructuredToolArgs(trimmedNext) {
		return next
	}
	if isEmptyStructuredToolArgs(trimmedNext) && !isEmptyStructuredToolArgs(trimmedCurrent) {
		return current
	}

	if looksLikeCompleteJSONPayload(trimmedNext) {
		return next
	}
	if shouldDeduplicateStreamingToolArgs(current, next) {
		return current
	}
	return current + next
}

func shouldDeduplicateStreamingToolArgs(current, next string) bool {
	if !strings.HasSuffix(current, next) {
		return false
	}
	// A trailing `\"` means the current chunk ends with an escaped quote inside
	// the JSON string value. If the next chunk is a bare `"`, we still need to
	// append it because it closes the JSON string itself.
	if next == `"` && endsWithEscapedDoubleQuote(current) {
		return false
	}
	return true
}

func endsWithEscapedDoubleQuote(raw string) bool {
	if len(raw) == 0 || raw[len(raw)-1] != '"' {
		return false
	}
	backslashes := 0
	for i := len(raw) - 2; i >= 0 && raw[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func looksLikeCompleteJSONPayload(raw string) bool {
	return (strings.HasPrefix(raw, "{") && strings.HasSuffix(raw, "}")) ||
		(strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]"))
}

func isEmptyStructuredToolArgs(raw string) bool {
	return raw == "{}" || raw == "[]"
}

func normalizeToolCallArgumentsForExecution(raw string) string {
	if normalized, ok := tools.CanonicalizeToolArgumentsJSON(raw); ok {
		return normalized
	}
	return raw
}
