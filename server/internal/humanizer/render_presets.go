package humanizer

import (
	"regexp"
	"strings"
)

var imageTextRe = regexp.MustCompile(`\(image: [^)]*\)`)

// --- Discord ---

var discordStyles = map[Style]StyleMarker{
	StyleBold:          {Open: "**", Close: "**"},
	StyleItalic:        {Open: "*", Close: "*"},
	StyleStrikethrough: {Open: "~~", Close: "~~"},
	StyleCode:          {Open: "`", Close: "`"},
	StyleCodeBlock:     {Open: "```\n", Close: "```"},
	StyleSpoiler:       {Open: "||", Close: "||"},
}

// RenderDiscord renders IR as Discord-flavored Markdown.
func RenderDiscord(ir IR) string {
	return Render(ir, RenderOptions{
		Styles:     discordStyles,
		EscapeText: func(s string) string { return s },
		BuildLink: func(link LinkSpan, text string) (string, string, bool) {
			href := strings.TrimSpace(link.Href)
			if href == "" {
				return "", "", false
			}
			label := text[link.Start:link.End]
			if label == href {
				return "", "", false // bare URL, no markup needed
			}
			return "[", "](" + href + ")", true
		},
	})
}

// --- Telegram ---

var telegramStyles = map[Style]StyleMarker{
	StyleBold:          {Open: "<b>", Close: "</b>"},
	StyleItalic:        {Open: "<i>", Close: "</i>"},
	StyleStrikethrough: {Open: "<s>", Close: "</s>"},
	StyleCode:          {Open: "<code>", Close: "</code>"},
	StyleCodeBlock:     {Open: "<pre><code>", Close: "</code></pre>"},
	StyleSpoiler:       {Open: "<tg-spoiler>", Close: "</tg-spoiler>"},
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func escapeHTMLAttr(s string) string {
	return strings.ReplaceAll(escapeHTML(s), "\"", "&quot;")
}

// RenderTelegram renders IR as Telegram HTML.
func RenderTelegram(ir IR) string {
	return Render(ir, RenderOptions{
		Styles:     telegramStyles,
		EscapeText: escapeHTML,
		BuildLink: func(link LinkSpan, _ string) (string, string, bool) {
			href := strings.TrimSpace(link.Href)
			if href == "" || link.Start >= link.End {
				return "", "", false
			}
			return `<a href="` + escapeHTMLAttr(href) + `">`, "</a>", true
		},
	})
}

// --- Slack ---

var slackStyles = map[Style]StyleMarker{
	StyleBold:          {Open: "*", Close: "*"},
	StyleItalic:        {Open: "_", Close: "_"},
	StyleStrikethrough: {Open: "~", Close: "~"},
	StyleCode:          {Open: "`", Close: "`"},
	StyleCodeBlock:     {Open: "```\n", Close: "```"},
}

func escapeSlackMrkdwn(s string) string {
	if !strings.ContainsAny(s, "&<>") {
		return s
	}
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// RenderSlack renders IR as Slack mrkdwn format.
func RenderSlack(ir IR) string {
	return Render(ir, RenderOptions{
		Styles:     slackStyles,
		EscapeText: escapeSlackMrkdwn,
		BuildLink: func(link LinkSpan, text string) (string, string, bool) {
			href := strings.TrimSpace(link.Href)
			if href == "" {
				return "", "", false
			}
			label := text[link.Start:link.End]
			trimmedLabel := strings.TrimSpace(label)
			// If label is same as URL, skip markup
			comparableHref := href
			if strings.HasPrefix(comparableHref, "mailto:") {
				comparableHref = comparableHref[7:]
			}
			if trimmedLabel == href || trimmedLabel == comparableHref {
				return "", "", false
			}
			return "<" + escapeSlackMrkdwn(href) + "|", ">", true
		},
	})
}

// --- Matrix ---

// RenderMatrix renders IR as Matrix HTML (same as Telegram but with different spoiler tag).
func RenderMatrix(ir IR) string {
	matrixStyles := map[Style]StyleMarker{
		StyleBold:          {Open: "<strong>", Close: "</strong>"},
		StyleItalic:        {Open: "<em>", Close: "</em>"},
		StyleStrikethrough: {Open: "<del>", Close: "</del>"},
		StyleCode:          {Open: "<code>", Close: "</code>"},
		StyleCodeBlock:     {Open: "<pre><code>", Close: "</code></pre>"},
		StyleSpoiler:       {Open: `<span data-mx-spoiler>`, Close: "</span>"},
	}
	return Render(ir, RenderOptions{
		Styles:     matrixStyles,
		EscapeText: escapeHTML,
		BuildLink: func(link LinkSpan, _ string) (string, string, bool) {
			href := strings.TrimSpace(link.Href)
			if href == "" || link.Start >= link.End {
				return "", "", false
			}
			return `<a href="` + escapeHTMLAttr(href) + `">`, "</a>", true
		},
	})
}

// --- Plain text ---

// RenderPlain renders IR as plain text with no formatting markers.
// Links are rendered as "label (url)".
func RenderPlain(ir IR) string {
	if len(ir.Links) == 0 {
		return ir.Text
	}
	return Render(ir, RenderOptions{
		Styles:     nil,
		EscapeText: func(s string) string { return s },
		BuildLink: func(link LinkSpan, text string) (string, string, bool) {
			href := strings.TrimSpace(link.Href)
			if href == "" {
				return "", "", false
			}
			label := text[link.Start:link.End]
			if label == href {
				return "", "", false
			}
			return "", " (" + href + ")", true
		},
	})
}

// --- Voice / TTS ---

// RenderVoice renders IR for TTS output.
// Code blocks are replaced with "(code omitted)".
// Inline code is stripped (content removed for voice clarity).
// Images are stripped. Bullet markers are stripped.
// Emojis are stripped. Links show only the label text.
// Bare URLs and table separators are stripped.
func RenderVoice(ir IR) string {
	text := ir.Text
	if text == "" {
		return ""
	}

	// Build a list of code_block and inline_code regions to replace
	type replacement struct {
		start int
		end   int
		text  string
	}
	var replacements []replacement
	for _, s := range ir.Styles {
		if s.End <= s.Start {
			continue
		}
		switch s.Style {
		case StyleCodeBlock:
			replacements = append(replacements, replacement{
				start: s.Start,
				end:   s.End,
				text:  "(code omitted)",
			})
		case StyleCode:
			// Strip inline code — variable names and snippets sound bad in TTS
			replacements = append(replacements, replacement{
				start: s.Start,
				end:   s.End,
				text:  "",
			})
		}
	}

	var out strings.Builder
	out.Grow(len(text))

	cursor := 0
	for _, r := range replacements {
		if r.start > cursor {
			out.WriteString(text[cursor:r.start])
		}
		out.WriteString(r.text)
		cursor = r.end
	}
	if cursor < len(text) {
		out.WriteString(text[cursor:])
	}

	result := out.String()

	// Strip "(image: ...)" patterns
	result = imageTextRe.ReplaceAllString(result, "")

	// Strip bullet markers "• "
	result = strings.ReplaceAll(result, "• ", "")

	// Strip emojis
	result = emojiRe.ReplaceAllString(result, "")

	// Strip bare URLs
	result = bareURLRe.ReplaceAllString(result, "")

	// Strip table pipe separators
	result = strings.ReplaceAll(result, " | ", ", ")

	// Clean up whitespace
	result = strings.TrimSpace(result)
	// Collapse multiple blank lines
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return result
}
