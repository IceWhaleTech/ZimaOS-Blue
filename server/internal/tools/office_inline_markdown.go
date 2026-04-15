package tools

import (
	"regexp"
	"strings"
)

type officeInlineRun struct {
	Text    string
	Bold    bool
	Italic  bool
	Code    bool
	Strike  bool
	LinkURL string
}

var officeMarkdownHyperlinkPattern = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

type officeInlineTextChunk struct {
	text    string
	linkURL string
}

func parseOfficeInlineMarkdown(text string) []officeInlineRun {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if text == "" {
		return nil
	}

	chunks := officeSplitInlineMarkdownHyperlinks(text)
	runs := make([]officeInlineRun, 0, len(chunks)*2)
	for _, chunk := range chunks {
		parsed := parseOfficeInlineStyledText(chunk.text)
		for idx := range parsed {
			parsed[idx].LinkURL = strings.TrimSpace(chunk.linkURL)
		}
		runs = append(runs, parsed...)
	}
	return officeMergeInlineRuns(runs)
}

func officeSplitInlineMarkdownHyperlinks(text string) []officeInlineTextChunk {
	matches := officeMarkdownHyperlinkPattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return []officeInlineTextChunk{{text: text}}
	}

	chunks := make([]officeInlineTextChunk, 0, len(matches)*2+1)
	lastEnd := 0
	for _, match := range matches {
		if len(match) < 6 {
			continue
		}
		start, end := match[0], match[1]
		textStart, textEnd := match[2], match[3]
		urlStart, urlEnd := match[4], match[5]
		if start > lastEnd {
			chunks = append(chunks, officeInlineTextChunk{text: text[lastEnd:start]})
		}
		chunks = append(chunks, officeInlineTextChunk{
			text:    text[textStart:textEnd],
			linkURL: officeNormalizeMarkdownLinkURL(text[urlStart:urlEnd]),
		})
		lastEnd = end
	}
	if lastEnd < len(text) {
		chunks = append(chunks, officeInlineTextChunk{text: text[lastEnd:]})
	}
	return chunks
}

func parseOfficeInlineStyledText(text string) []officeInlineRun {
	if text == "" {
		return nil
	}

	runs := make([]officeInlineRun, 0, 4)
	var buf strings.Builder
	style := officeInlineRun{}

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		runs = append(runs, officeInlineRun{
			Text:   buf.String(),
			Bold:   style.Bold,
			Italic: style.Italic,
			Code:   style.Code,
			Strike: style.Strike,
		})
		buf.Reset()
	}

	for idx := 0; idx < len(text); {
		switch {
		case text[idx] == '`' && officeInlineMarkdownCanToggle(text[idx+1:], "`", style.Code):
			flush()
			style.Code = !style.Code
			idx++
		case !style.Code && strings.HasPrefix(text[idx:], "~~") && officeInlineMarkdownCanToggle(text[idx+2:], "~~", style.Strike):
			flush()
			style.Strike = !style.Strike
			idx += 2
		case !style.Code && strings.HasPrefix(text[idx:], "**") && officeInlineMarkdownCanToggle(text[idx+2:], "**", style.Bold):
			flush()
			style.Bold = !style.Bold
			idx += 2
		case !style.Code && strings.HasPrefix(text[idx:], "__") && officeInlineMarkdownCanToggle(text[idx+2:], "__", style.Bold):
			flush()
			style.Bold = !style.Bold
			idx += 2
		case !style.Code && text[idx] == '*' && !strings.HasPrefix(text[idx:], "**") && officeInlineMarkdownCanToggle(text[idx+1:], "*", style.Italic):
			flush()
			style.Italic = !style.Italic
			idx++
		case !style.Code && text[idx] == '_' && !strings.HasPrefix(text[idx:], "__") && officeInlineMarkdownCanToggle(text[idx+1:], "_", style.Italic):
			flush()
			style.Italic = !style.Italic
			idx++
		default:
			buf.WriteByte(text[idx])
			idx++
		}
	}

	flush()
	return runs
}

func officeInlineMarkdownCanToggle(rest, marker string, active bool) bool {
	if active {
		return true
	}
	return strings.Contains(rest, marker)
}

func officeMergeInlineRuns(runs []officeInlineRun) []officeInlineRun {
	if len(runs) == 0 {
		return nil
	}
	merged := make([]officeInlineRun, 0, len(runs))
	for _, run := range runs {
		if run.Text == "" {
			continue
		}
		if len(merged) == 0 {
			merged = append(merged, run)
			continue
		}
		last := &merged[len(merged)-1]
		if last.Bold == run.Bold &&
			last.Italic == run.Italic &&
			last.Code == run.Code &&
			last.Strike == run.Strike &&
			last.LinkURL == run.LinkURL {
			last.Text += run.Text
			continue
		}
		merged = append(merged, run)
	}
	return merged
}

func officeNormalizeMarkdownLinkURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var sb strings.Builder
	for _, r := range raw {
		if r == '\r' || r == '\n' || r == '\t' || r == ' ' {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func officeResolvedMonospaceFont(font string) string {
	if font = strings.TrimSpace(font); font != "" {
		return font
	}
	return resolveOfficeTheme("", "").MonospaceFont
}
