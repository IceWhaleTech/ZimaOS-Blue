package humanizer

import (
	"regexp"
	"strings"
)

// Pre-compiled regexes for performance.
var (
	// Block-level patterns
	codeFenceRe     = regexp.MustCompile("(?s)```[\\w]*\\n?(.*?)```")
	headerRe        = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	horizontalRe    = regexp.MustCompile(`(?m)^[\s]*([-*_]){3,}\s*$`)
	blockquoteRe    = regexp.MustCompile(`(?m)^>\s?`)
	bulletDashRe    = regexp.MustCompile(`(?m)^(\s*)[-*+]\s`)
	numberedListRe  = regexp.MustCompile(`(?m)^(\s*)\d+\.\s`)

	// Inline patterns
	boldRe          = regexp.MustCompile(`\*\*(.+?)\*\*`)
	boldUnderRe     = regexp.MustCompile(`__(.+?)__`)
	italicRe        = regexp.MustCompile(`(?:^|[^*])\*([^*]+?)\*(?:[^*]|$)`)
	italicUnderRe   = regexp.MustCompile(`(?:^|[^_])_([^_]+?)_(?:[^_]|$)`)
	strikethroughRe = regexp.MustCompile(`~~(.+?)~~`)
	inlineCodeRe    = regexp.MustCompile("`([^`]+)`")
	linkRe          = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	imageRe         = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	htmlTagRe       = regexp.MustCompile(`<[^>]+>`)

	// Emoji pattern (comprehensive Unicode ranges)
	emojiRe = regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]|[\x{FE00}-\x{FE0F}]|[\x{1F900}-\x{1F9FF}]|[\x{1FA00}-\x{1FA6F}]|[\x{1FA70}-\x{1FAFF}]|[\x{200D}]|[\x{20E3}]|[\x{FE0F}]`)

	// Whitespace cleanup
	multiBlankLineRe = regexp.MustCompile(`\n{3,}`)
	trailingSpaceRe  = regexp.MustCompile(`(?m)[ \t]+$`)

	// Italic strip patterns (used in stripItalic)
	italicStarStripRe  = regexp.MustCompile(`(?:^|\s)\*([^*\n]+?)\*(?:\s|$|[.,!?;:])`)
	italicUnderStripRe = regexp.MustCompile(`(?:^|\s)_([^_\n]+?)_(?:\s|$|[.,!?;:])`)
)

// stripCodeFences handles fenced code blocks.
// IM mode: removes fence markers, keeps content.
// Voice mode: replaces entire block with a spoken indicator.
func stripCodeFences(text string, mode Mode) string {
	if mode == ModeVoice {
		return codeFenceRe.ReplaceAllString(text, "(code omitted)")
	}
	// IM mode: keep content, remove ``` lines
	return codeFenceRe.ReplaceAllString(text, "$1")
}

// stripHeaders removes # header markers.
func stripHeaders(text string) string {
	return headerRe.ReplaceAllString(text, "")
}

// stripHorizontalRules removes ---, ***, ___ lines.
func stripHorizontalRules(text string) string {
	return horizontalRe.ReplaceAllString(text, "")
}

// stripBlockquotes removes > markers.
func stripBlockquotes(text string) string {
	return blockquoteRe.ReplaceAllString(text, "")
}

// stripBold removes ** and __ bold markers.
func stripBold(text string) string {
	text = boldRe.ReplaceAllString(text, "$1")
	text = boldUnderRe.ReplaceAllString(text, "$1")
	return text
}

// stripItalic removes * and _ italic markers.
// Careful not to match bold ** or already-stripped content.
func stripItalic(text string) string {
	// Simple approach: strip remaining single * and _ wrappers
	text = italicStarStripRe.ReplaceAllStringFunc(text, func(m string) string {
		inner := strings.TrimSpace(m)
		inner = strings.TrimPrefix(inner, "*")
		inner = strings.TrimSuffix(inner, "*")
		// Preserve surrounding whitespace
		prefix := ""
		suffix := ""
		if len(m) > 0 && m[0] == ' ' {
			prefix = " "
		}
		if len(m) > 0 && (m[len(m)-1] == ' ' || m[len(m)-1] == '.' || m[len(m)-1] == ',' || m[len(m)-1] == '!' || m[len(m)-1] == '?') {
			suffix = string(m[len(m)-1])
		}
		return prefix + strings.TrimSpace(inner) + suffix
	})
	text = italicUnderStripRe.ReplaceAllStringFunc(text, func(m string) string {
		inner := strings.TrimSpace(m)
		inner = strings.TrimPrefix(inner, "_")
		inner = strings.TrimSuffix(inner, "_")
		prefix := ""
		suffix := ""
		if len(m) > 0 && m[0] == ' ' {
			prefix = " "
		}
		if len(m) > 0 && (m[len(m)-1] == ' ' || m[len(m)-1] == '.' || m[len(m)-1] == ',' || m[len(m)-1] == '!' || m[len(m)-1] == '?') {
			suffix = string(m[len(m)-1])
		}
		return prefix + strings.TrimSpace(inner) + suffix
	})
	return text
}

// stripStrikethrough removes ~~ markers.
func stripStrikethrough(text string) string {
	return strikethroughRe.ReplaceAllString(text, "$1")
}

// stripInlineCode removes backtick markers around inline code.
func stripInlineCode(text string) string {
	return inlineCodeRe.ReplaceAllString(text, "$1")
}

// stripLinks handles markdown links.
// IM mode: [text](url) → text (url)
// Voice mode: [text](url) → text
func stripLinks(text string, mode Mode) string {
	if mode == ModeVoice {
		return linkRe.ReplaceAllString(text, "$1")
	}
	return linkRe.ReplaceAllString(text, "$1 ($2)")
}

// stripImages handles image references.
// IM mode: ![alt](url) → (image: alt)
// Voice mode: ![alt](url) → removed
func stripImages(text string, mode Mode) string {
	if mode == ModeVoice {
		return imageRe.ReplaceAllString(text, "")
	}
	return imageRe.ReplaceAllString(text, "(image: $1)")
}

// stripHTMLTags removes HTML tags.
func stripHTMLTags(text string) string {
	return htmlTagRe.ReplaceAllString(text, "")
}

// stripEmojis removes emoji characters.
func stripEmojis(text string) string {
	return emojiRe.ReplaceAllString(text, "")
}

// normalizeBullets handles bullet point markers.
// IM mode: - item → • item
// Voice mode: - item → item
func normalizeBullets(text string, mode Mode) string {
	if mode == ModeVoice {
		text = bulletDashRe.ReplaceAllString(text, "$1")
		text = numberedListRe.ReplaceAllString(text, "$1")
		return text
	}
	// IM mode: normalize to •
	text = bulletDashRe.ReplaceAllString(text, "${1}• ")
	return text
}

// normalizeWhitespace cleans up excessive whitespace.
func normalizeWhitespace(text string) string {
	// Tabs to spaces
	text = strings.ReplaceAll(text, "\t", "  ")
	// Trailing whitespace per line
	text = trailingSpaceRe.ReplaceAllString(text, "")
	// Collapse 3+ blank lines to 2 (one blank line)
	text = multiBlankLineRe.ReplaceAllString(text, "\n\n")
	// Trim leading/trailing
	text = strings.TrimSpace(text)
	return text
}
