package cron

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

func resolveJobName(name, description, handler string, payload map[string]interface{}) string {
	if text := normalizeJobTitleText(name); text != "" {
		return text
	}
	if text := normalizeJobTitleText(description); text != "" {
		return limitJobTitleRunes(text, 60)
	}

	switch handler {
	case "command":
		if title := commandJobTitle(payload); title != "" {
			return title
		}
		return "Scheduled command"
	case "http":
		if title := httpJobTitle(payload); title != "" {
			return title
		}
		return "Scheduled request"
	default:
		return "Scheduled task"
	}
}

func commandJobTitle(payload map[string]interface{}) string {
	command := normalizeJobTitleText(stringPayloadValue(payload, "command"))
	if command == "" {
		return ""
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}

	base := baseCommand(parts[0])
	switch base {
	case "uptime":
		return "Check uptime"
	case "date":
		return "Capture current time"
	case "df":
		return "Check disk usage"
	case "free":
		return "Check memory usage"
	case "ps":
		return "List processes"
	case "curl", "wget":
		if rawURL := firstURL(parts[1:]); rawURL != "" {
			return "Request " + summarizeURL(rawURL)
		}
	}

	return "Run " + limitJobTitleRunes(command, 48)
}

func httpJobTitle(payload map[string]interface{}) string {
	rawURL := normalizeJobTitleText(stringPayloadValue(payload, "url"))
	if rawURL == "" {
		return ""
	}

	method := strings.ToUpper(normalizeJobTitleText(stringPayloadValue(payload, "method")))
	if method == "" || method == "GET" {
		return "Request " + summarizeURL(rawURL)
	}

	return fmt.Sprintf("%s %s", method, summarizeURL(rawURL))
}

func stringPayloadValue(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	value, _ := payload[key].(string)
	return value
}

func normalizeJobTitleText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func limitJobTitleRunes(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "..."
}

func baseCommand(value string) string {
	value = strings.TrimSpace(strings.Trim(value, `"'`))
	value = strings.TrimSuffix(value, "/")
	value = path.Base(value)
	value = strings.TrimSuffix(value, ".exe")
	return strings.ToLower(value)
}

func firstURL(parts []string) string {
	for _, part := range parts {
		part = strings.Trim(part, `"'`)
		if strings.HasPrefix(part, "http://") || strings.HasPrefix(part, "https://") {
			return part
		}
	}
	return ""
}

func summarizeURL(raw string) string {
	raw = strings.TrimSpace(strings.Trim(raw, `"'`))
	parsed, err := url.Parse(raw)
	if err != nil {
		return limitJobTitleRunes(raw, 48)
	}

	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	pathValue := strings.TrimSpace(parsed.EscapedPath())
	if pathValue == "" || pathValue == "/" {
		if host != "" {
			return host
		}
		return limitJobTitleRunes(raw, 48)
	}

	segments := make([]string, 0, 2)
	for _, segment := range strings.Split(strings.Trim(pathValue, "/"), "/") {
		if segment == "" {
			continue
		}
		segments = append(segments, segment)
		if len(segments) == 2 {
			break
		}
	}

	summaryPath := "/" + strings.Join(segments, "/")
	if host == "" {
		return limitJobTitleRunes(summaryPath, 48)
	}
	return limitJobTitleRunes(host+summaryPath, 48)
}
