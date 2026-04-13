package tools

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type officeStructuredCell struct {
	Value    interface{}
	Formula  string
	Type     string
	Format   string
	HasShape bool
}

func officeContainsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func officeXMLTextCanWriteRawASCII(text string) bool {
	for i := 0; i < len(text); i++ {
		b := text[i]
		switch b {
		case '"', '\'', '&', '<', '>', '\t', '\n', '\r':
			return false
		}
		if b < 0x20 || b >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func sortStringsCaseInsensitive(values []string) {
	sort.SliceStable(values, func(i, j int) bool {
		left := strings.ToLower(values[i])
		right := strings.ToLower(values[j])
		if left == right {
			return values[i] < values[j]
		}
		return left < right
	})
}

func splitOfficeParagraphs(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	parts := strings.Split(text, "\n\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		lines := strings.Split(part, "\n")
		clean := make([]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				clean = append(clean, line)
			}
		}
		joined := officeJoinTextFragments(clean)
		if joined != "" {
			out = append(out, joined)
		}
	}
	return out
}

func inferOfficeColumnKind(column officeColumnSpec, rows [][]interface{}, columnIndex int) string {
	header := strings.ToLower(strings.TrimSpace(column.Header + " " + column.Key))
	switch {
	case officeContainsAny(header, "delta", "变化", "提升", "差距", "diff"):
		return "delta"
	case officeContainsAny(header, "status", "tone", "verdict", "state", "是否", "状态", "结果", "评级"):
		return "tone"
	case officeContainsAny(header, "score", "rate", "count", "amount", "price", "total", "value", "percent", "ratio", "分", "率", "数量", "金额", "总计", "值"):
		return "number"
	case officeContainsAny(header, "time", "date", "日期", "时间"):
		return "wrap"
	}
	for _, row := range rows {
		if columnIndex >= len(row) {
			continue
		}
		switch value := row[columnIndex].(type) {
		case float64, int, int64, int32, uint64, uint32:
			return "number"
		case string:
			text := strings.TrimSpace(value)
			if text == "" {
				continue
			}
			if _, err := strconv.ParseFloat(strings.ReplaceAll(text, ",", ""), 64); err == nil {
				return "number"
			}
			if runeCount(text) > 24 || strings.Contains(text, "\n") {
				return "wrap"
			}
		}
	}
	return "text"
}

func runeCount(text string) int {
	if text == "" {
		return 0
	}
	return utf8.RuneCountInString(text)
}

func officeCellString(value interface{}) string {
	if cell, ok := officeParseStructuredCell(value); ok {
		value = cell.Value
	}
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case bool:
		if typed {
			return "TRUE"
		}
		return "FALSE"
	default:
		return anyToStringForLLM(typed)
	}
}

func officeCellNumber(value interface{}) (float64, bool) {
	if cell, ok := officeParseStructuredCell(value); ok {
		value = cell.Value
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case string:
		return officeParseNumericString(typed)
	default:
		return 0, false
	}
}

func officeParseNumericString(text string) (float64, bool) {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return 0, false
	}
	normalized = strings.ReplaceAll(normalized, ",", "")
	if !officeLooksNumericString(normalized) {
		return 0, false
	}
	v, err := strconv.ParseFloat(normalized, 64)
	return v, err == nil
}

func officeLooksNumericString(text string) bool {
	digitCount := 0
	exponentDigits := 0
	inExponent := false
	allowSign := true
	allowDot := true

	for _, r := range text {
		switch {
		case r >= '0' && r <= '9':
			digitCount++
			if inExponent {
				exponentDigits++
			}
			allowSign = false
		case r == '+' || r == '-':
			if !allowSign {
				return false
			}
			allowSign = false
		case r == '.':
			if !allowDot {
				return false
			}
			allowDot = false
			allowSign = false
		case r == 'e' || r == 'E':
			if inExponent || digitCount == 0 {
				return false
			}
			inExponent = true
			allowSign = true
			allowDot = false
		default:
			return false
		}
	}

	if digitCount == 0 {
		return false
	}
	if inExponent && exponentDigits == 0 {
		return false
	}
	return true
}

func officeParseStructuredCell(value interface{}) (officeStructuredCell, bool) {
	m, ok := value.(map[string]interface{})
	if !ok {
		return officeStructuredCell{}, false
	}
	_, hasValue := m["value"]
	_, hasFormula := m["formula"]
	_, hasType := m["type"]
	_, hasFormat := m["format"]
	if !hasValue && !hasFormula && !hasType && !hasFormat {
		return officeStructuredCell{}, false
	}
	return officeStructuredCell{
		Value:    m["value"],
		Formula:  strings.TrimSpace(anyToStringForLLM(m["formula"])),
		Type:     strings.ToLower(strings.TrimSpace(anyToStringForLLM(m["type"]))),
		Format:   strings.ToLower(strings.TrimSpace(anyToStringForLLM(m["format"]))),
		HasShape: true,
	}, true
}

func anyToStringForLLM(v interface{}) string {
	switch typed := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return strings.TrimSpace(SafeToolPayloadString(typed, 8*1024))
	}
}

func officeXMLText(text string) string {
	var sb strings.Builder
	sb.Grow(len(text) + len(text)/8 + 8)
	officeAppendXMLText(&sb, text)
	return sb.String()
}

func officeAppendXMLText(sb *strings.Builder, text string) {
	if text == "" {
		return
	}
	if officeXMLTextCanWriteRawASCII(text) {
		sb.WriteString(text)
		return
	}
	start := 0
	for i := 0; i < len(text); {
		r, width := utf8.DecodeRuneInString(text[i:])
		escape := ""
		switch r {
		case '"':
			escape = "&#34;"
		case '\'':
			escape = "&#39;"
		case '&':
			escape = "&amp;"
		case '<':
			escape = "&lt;"
		case '>':
			escape = "&gt;"
		case '\t':
			escape = "&#x9;"
		case '\n':
			escape = "&#xA;"
		case '\r':
			escape = "&#xD;"
		default:
			if officeXMLRuneAllowed(r) && !(r == utf8.RuneError && width == 1) {
				i += width
				continue
			}
			escape = "\uFFFD"
		}
		if start < i {
			sb.WriteString(text[start:i])
		}
		sb.WriteString(escape)
		i += width
		start = i
	}
	if start < len(text) {
		sb.WriteString(text[start:])
	}
}

func officeXMLRuneAllowed(r rune) bool {
	return r == 0x09 ||
		r == 0x0A ||
		r == 0x0D ||
		r >= 0x20 && r <= 0xD7FF ||
		r >= 0xE000 && r <= 0xFFFD ||
		r >= 0x10000 && r <= 0x10FFFF
}

func officeNowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func officePathExt(path string) string {
	return strings.ToLower(strings.TrimSpace(filepath.Ext(path)))
}
