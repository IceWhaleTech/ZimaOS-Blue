package heartbeat

import (
	"regexp"
	"strings"
)

const HeartbeatToken = "HEARTBEAT_OK"

var (
	// Matches markdown headers: lines starting with # followed by space or EOL.
	headerPattern = regexp.MustCompile(`^#+(\s|$)`)
	// Matches empty markdown list items like "- [ ]" or "* [ ]" or just "- ".
	emptyListPattern = regexp.MustCompile(`^[-*+]\s*(\[\s*[Xx]?\]\s*)?$`)
	// Matches HTML tags.
	htmlTagPattern = regexp.MustCompile(`<[^>]*>`)
	// Matches markdown edge wrappers: *, `, ~, _.
	markdownEdgeStart = regexp.MustCompile(`^[*` + "`" + `~_]+`)
	markdownEdgeEnd   = regexp.MustCompile(`[*` + "`" + `~_]+$`)
	// Collapses whitespace.
	whitespacePattern = regexp.MustCompile(`\s+`)
)

// StripResult holds the result of stripping the HEARTBEAT_OK token.
type StripResult struct {
	Text       string
	ShouldSkip bool
	DidStrip   bool
}

// IsEffectivelyEmpty checks if HEARTBEAT.md content has no actionable tasks.
// A file is effectively empty if it contains only whitespace, markdown headers, or empty list items.
func IsEffectivelyEmpty(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if headerPattern.MatchString(trimmed) {
			continue
		}
		if emptyListPattern.MatchString(trimmed) {
			continue
		}
		return false
	}
	return true
}

// StripHeartbeatToken removes HEARTBEAT_OK from the edges of a response.
// In heartbeat mode, if the remaining text is ≤ maxAckChars, it signals skip.
func StripHeartbeatToken(raw string, maxAckChars int) StripResult {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return StripResult{ShouldSkip: true}
	}

	// Also try with markup stripped for detection.
	normalized := stripMarkup(trimmed)
	hasToken := strings.Contains(trimmed, HeartbeatToken) || strings.Contains(normalized, HeartbeatToken)
	if !hasToken {
		return StripResult{Text: trimmed}
	}

	origStripped, origDid := stripTokenAtEdges(trimmed)
	normStripped, normDid := stripTokenAtEdges(normalized)

	// Prefer original if it stripped and has remaining text.
	picked, didStrip := origStripped, origDid
	if !(origDid && origStripped != "") {
		picked, didStrip = normStripped, normDid
	}

	if !didStrip {
		return StripResult{Text: trimmed}
	}
	if picked == "" {
		return StripResult{ShouldSkip: true, DidStrip: true}
	}

	rest := strings.TrimSpace(picked)
	if len(rest) <= maxAckChars {
		return StripResult{ShouldSkip: true, DidStrip: true}
	}
	return StripResult{Text: rest, DidStrip: true}
}

// stripTokenAtEdges iteratively removes HeartbeatToken from the start and end.
func stripTokenAtEdges(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" || !strings.Contains(text, HeartbeatToken) {
		return text, false
	}

	didStrip := false
	for {
		changed := false
		t := strings.TrimSpace(text)
		if strings.HasPrefix(t, HeartbeatToken) {
			text = strings.TrimSpace(t[len(HeartbeatToken):])
			didStrip = true
			changed = true
			continue
		}
		if strings.HasSuffix(t, HeartbeatToken) {
			text = strings.TrimSpace(t[:len(t)-len(HeartbeatToken)])
			didStrip = true
			changed = true
		}
		if !changed {
			break
		}
	}

	collapsed := whitespacePattern.ReplaceAllString(strings.TrimSpace(text), " ")
	return collapsed, didStrip
}

// stripMarkup removes HTML tags and markdown edge wrappers.
func stripMarkup(text string) string {
	text = htmlTagPattern.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = markdownEdgeStart.ReplaceAllString(text, "")
	text = markdownEdgeEnd.ReplaceAllString(text, "")
	return text
}
