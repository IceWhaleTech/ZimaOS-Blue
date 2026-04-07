package humanizer

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CompactForIM removes verbose execution-detail blocks from assistant output so
// IM channels default to concise, stable rendering.
func CompactForIM(text string) string {
	ensureHumanizerRegexes()
	if text == "" {
		return ""
	}
	text = processCommentBlockRe.ReplaceAllString(text, "")
	text = processFenceBlockRe.ReplaceAllString(text, "")
	return normalizeWhitespace(text)
}

// stripCodeFences handles fenced code blocks.
// IM mode: removes fence markers, keeps content.
// Voice mode: replaces entire block with a spoken indicator.
func stripCodeFences(text string, mode Mode) string {
	ensureHumanizerRegexes()
	if mode == ModeVoice {
		return codeFenceRe.ReplaceAllString(text, "(code omitted)")
	}
	// IM mode: keep content, remove ``` lines
	return codeFenceRe.ReplaceAllString(text, "$1")
}

// stripHeaders removes # header markers.
func stripHeaders(text string) string {
	ensureHumanizerRegexes()
	return headerRe.ReplaceAllString(text, "")
}

// stripHorizontalRules removes ---, ***, ___ lines.
func stripHorizontalRules(text string) string {
	ensureHumanizerRegexes()
	return horizontalRe.ReplaceAllString(text, "")
}

// stripBlockquotes removes > markers.
func stripBlockquotes(text string) string {
	ensureHumanizerRegexes()
	return blockquoteRe.ReplaceAllString(text, "")
}

// stripBold removes ** and __ bold markers.
func stripBold(text string) string {
	ensureHumanizerRegexes()
	text = boldRe.ReplaceAllString(text, "$1")
	text = boldUnderRe.ReplaceAllString(text, "$1")
	return text
}

// stripItalic removes * and _ italic markers.
// Careful not to match bold ** or already-stripped content.
func stripItalic(text string) string {
	ensureHumanizerRegexes()
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
	ensureHumanizerRegexes()
	return strikethroughRe.ReplaceAllString(text, "$1")
}

// stripInlineCode removes backtick markers around inline code.
func stripInlineCode(text string) string {
	ensureHumanizerRegexes()
	return inlineCodeRe.ReplaceAllString(text, "$1")
}

// stripLinks handles markdown links.
// IM mode: [text](url) → text (url)
// Voice mode: [text](url) → text
func stripLinks(text string, mode Mode) string {
	ensureHumanizerRegexes()
	if mode == ModeVoice {
		return linkRe.ReplaceAllString(text, "$1")
	}
	return linkRe.ReplaceAllString(text, "$1 ($2)")
}

// stripImages handles image references.
// IM mode: ![alt](url) → (image: alt)
// Voice mode: ![alt](url) → removed
func stripImages(text string, mode Mode) string {
	ensureHumanizerRegexes()
	if mode == ModeVoice {
		return imageRe.ReplaceAllString(text, "")
	}
	return imageRe.ReplaceAllString(text, "(image: $1)")
}

// stripHTMLTags removes HTML tags.
func stripHTMLTags(text string) string {
	ensureHumanizerRegexes()
	return htmlTagRe.ReplaceAllString(text, "")
}

// stripEmojis removes emoji characters.
func stripEmojis(text string) string {
	ensureHumanizerRegexes()
	return emojiRe.ReplaceAllString(text, "")
}

// stripMathBlocks replaces LaTeX math with spoken description.
// $$...$$ → "(公式已省略)", $...$ → "(公式)"
func stripMathBlocks(text string) string {
	ensureHumanizerRegexes()
	text = mathBlockRe.ReplaceAllString(text, "(公式已省略)")
	text = mathInlineRe.ReplaceAllString(text, "(公式)")
	return text
}

// stripBareURLs removes bare URLs not wrapped in markdown link syntax.
func stripBareURLs(text string) string {
	ensureHumanizerRegexes()
	return bareURLRe.ReplaceAllString(text, "")
}

// normalizeBullets handles bullet point markers.
// IM mode: - item → • item
// Voice mode: - item → item
func normalizeBullets(text string, mode Mode) string {
	ensureHumanizerRegexes()
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
	ensureHumanizerRegexes()
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

// stripTypelessCards converts typeless card JSON blocks to readable text.
// Must run before stripCodeFences so the ```typeless blocks don't get generic-stripped.
func stripTypelessCards(text string, mode Mode) string {
	ensureHumanizerRegexes()
	return typelessCardRe.ReplaceAllStringFunc(text, func(match string) string {
		// Extract JSON content between markers
		sub := typelessCardRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		jsonStr := strings.TrimSpace(sub[1])

		var card struct {
			Type    string `json:"type"`
			Query   string `json:"query"`
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
			TotalCount int `json:"total_count"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &card); err != nil {
			return match // Not valid JSON, leave for stripCodeFences
		}

		switch card.Type {
		case "search":
			if mode == ModeVoice {
				return "(搜索结果已省略)"
			}
			return formatSearchForIM(card.Query, card.Results, card.TotalCount)
		default:
			return match // Unknown card type, leave for stripCodeFences
		}
	})
}

// formatSearchForIM formats search results as readable IM text.
func formatSearchForIM(query string, results []struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}, totalCount int) string {
	count := totalCount
	if count == 0 {
		count = len(results)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 搜索「%s」找到 %d 条结果：\n", query, count))
	for i, r := range results {
		if i >= 5 {
			break
		}
		sb.WriteString(fmt.Sprintf("\n%d. %s\n   %s", i+1, r.Title, r.URL))
	}
	return sb.String()
}

// Tool name localization for function_calls blocks.
var toolNameZh = map[string]string{
	"web_query":     "网页查询",
	"web_search":    "网页搜索",
	"calculator":    "计算器",
	"system_info":   "系统信息",
	"current_time":  "当前时间",
	"read":          "读取文件",
	"write":         "写入文件",
	"file_read":     "读取文件",
	"file_write":    "写入文件",
	"memory_search": "记忆搜索",
	"memory_store":  "存储记忆",
	"memory_get":    "获取记忆",
	"memory_stats":  "记忆统计",
	"memory":        "记忆系统",
	"mcp":           "MCP 调用",
}

// Parameter name localization.
var paramNameZh = map[string]string{
	"query":       "查询",
	"region":      "区域",
	"max_results": "结果条数",
	"path":        "路径",
	"content":     "内容",
	"filename":    "文件名",
	"expression":  "表达式",
	"keyword":     "关键词",
	"limit":       "数量限制",
}

// Keyword-style params: show value only, no label.
var keywordParams = map[string]bool{
	"query": true, "keyword": true, "expression": true,
}

// stripFunctionCalls converts <function_calls> XML blocks to readable text for IM/Voice.
func stripFunctionCalls(text string, mode Mode) string {
	ensureHumanizerRegexes()
	return functionCallsRe.ReplaceAllStringFunc(text, func(match string) string {
		if mode == ModeVoice {
			return "(工具调用已省略)"
		}
		invocations := invokeRe.FindAllStringSubmatch(match, -1)
		if len(invocations) == 0 {
			return ""
		}
		var sb strings.Builder
		for _, inv := range invocations {
			toolName := inv[1]
			if zh, ok := toolNameZh[toolName]; ok {
				toolName = zh
			}
			paramsBlock := inv[2]
			params := paramRe.FindAllStringSubmatch(paramsBlock, -1)
			var paramParts []string
			for _, p := range params {
				pName, pVal := p[1], strings.TrimSpace(p[2])
				if len(pVal) > 60 {
					pVal = pVal[:60] + "..."
				}
				if keywordParams[pName] {
					paramParts = append(paramParts, pVal)
				} else {
					label := pName
					if zh, ok := paramNameZh[pName]; ok {
						label = zh
					}
					paramParts = append(paramParts, label+": "+pVal)
				}
			}
			sb.WriteString("🔧 " + toolName)
			if len(paramParts) > 0 {
				sb.WriteString("（" + strings.Join(paramParts, "，") + "）")
			}
			sb.WriteString("\n")
		}
		return strings.TrimRight(sb.String(), "\n")
	})
}
