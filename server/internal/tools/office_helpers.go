package tools

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func officeContainsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
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
		normalized := strings.TrimSpace(strings.ReplaceAll(typed, ",", ""))
		if normalized == "" {
			return 0, false
		}
		v, err := strconv.ParseFloat(normalized, 64)
		return v, err == nil
	default:
		return 0, false
	}
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
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(text))
	return buf.String()
}

func officeNowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func officePathExt(path string) string {
	return strings.ToLower(strings.TrimSpace(filepath.Ext(path)))
}
