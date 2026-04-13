package channel

import (
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

var (
	messageRegexMu sync.Mutex

	// Common AI response tags to strip.
	multiNewlineRe *regexp.Regexp
	aiTagPatterns  []*regexp.Regexp
)

func ensureChannelMessageRegexes() {
	if multiNewlineRe != nil && len(aiTagPatterns) > 0 {
		return
	}

	messageRegexMu.Lock()
	defer messageRegexMu.Unlock()

	if multiNewlineRe != nil && len(aiTagPatterns) > 0 {
		return
	}

	multiNewlineRe = regexp.MustCompile(`\n{3,}`)
	aiTagPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?s)<thinking>.*?</thinking>`),
		regexp.MustCompile(`(?s)<system>.*?</system>`),
		regexp.MustCompile(`(?s)<internal>.*?</internal>`),
		regexp.MustCompile(`(?s)<reasoning>.*?</reasoning>`),
		regexp.MustCompile(`(?s)<scratchpad>.*?</scratchpad>`),
		regexp.MustCompile(`(?s)<reflection>.*?</reflection>`),
	}
}

// StripAITags removes AI-specific tags like <thinking>, <system>, etc. from content.
func StripAITags(content string) string {
	ensureChannelMessageRegexes()

	result := content
	for _, pattern := range aiTagPatterns {
		result = pattern.ReplaceAllString(result, "")
	}
	// Clean up extra whitespace left behind
	result = strings.TrimSpace(result)
	// Replace multiple newlines with double newline
	result = multiNewlineRe.ReplaceAllString(result, "\n\n")
	return result
}

// SplitMessage splits a long message into multiple parts, respecting maxLength.
// It tries to split at paragraph boundaries, then sentence boundaries, then word boundaries.
func SplitMessage(content string, maxLength int) []string {
	if maxLength <= 0 {
		maxLength = 4096 // Default max length
	}

	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(content) <= maxLength {
		return []string{content}
	}

	var parts []string
	remaining := content

	for utf8.RuneCountInString(remaining) > 0 {
		if utf8.RuneCountInString(remaining) <= maxLength {
			parts = append(parts, strings.TrimSpace(remaining))
			break
		}

		// Find a good split point
		splitPoint := findSplitPoint(remaining, maxLength)
		part := strings.TrimSpace(remaining[:splitPoint])
		if part != "" {
			parts = append(parts, part)
		}
		remaining = strings.TrimSpace(remaining[splitPoint:])
	}

	return parts
}

// findSplitPoint finds the best point to split the message.
func findSplitPoint(content string, maxLength int) int {
	runes := []rune(content)
	if len(runes) <= maxLength {
		return len(content)
	}

	// Convert rune index to byte index
	runeToByteIndex := func(runeIdx int) int {
		return len(string(runes[:runeIdx]))
	}

	searchEnd := maxLength
	searchStart := maxLength * 3 / 4 // Search in last 25% of allowed length

	// Try to find paragraph break (double newline)
	for i := searchEnd; i >= searchStart; i-- {
		if i < len(runes)-1 && runes[i] == '\n' && runes[i+1] == '\n' {
			return runeToByteIndex(i + 2)
		}
	}

	// Try to find single newline
	for i := searchEnd; i >= searchStart; i-- {
		if runes[i] == '\n' {
			return runeToByteIndex(i + 1)
		}
	}

	// Try to find sentence end (. ! ?)
	for i := searchEnd; i >= searchStart; i-- {
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' ||
			runes[i] == '。' || runes[i] == '！' || runes[i] == '？' {
			return runeToByteIndex(i + 1)
		}
	}

	// Try to find word boundary (space)
	for i := searchEnd; i >= searchStart; i-- {
		if runes[i] == ' ' || runes[i] == '\t' {
			return runeToByteIndex(i + 1)
		}
	}

	// Hard split at maxLength
	return runeToByteIndex(maxLength)
}

// PrepareResponse strips AI tags and splits the message if needed.
func PrepareResponse(content string, maxLength int) []string {
	cleaned := StripAITags(content)
	if cleaned == "" {
		return nil
	}
	return SplitMessage(cleaned, maxLength)
}
