package tools

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

const (
	ToolLoopReasonUnknown           = "unknown"
	ToolLoopReasonIdenticalRepeat   = "identical_repeat"
	ToolLoopReasonPollingNoProgress = "polling_no_progress"
	ToolLoopReasonErrorRepeat       = "error_repeat"
	ToolLoopReasonPingPong          = "ab_ping_pong"
)

type ToolLoopDetection struct {
	Abort     bool   `json:"abort"`
	Reason    string `json:"reason,omitempty"`
	Streak    int    `json:"streak,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// ToolLoopDetector tracks recent tool rounds to detect loop patterns at runtime.
type ToolLoopDetector struct {
	lastToolSignature     string
	duplicateRounds       int
	lastCombinedSignature string
	stableRounds          int
	lastErrorSignature    string
	errorRounds           int
	recentToolSignatures  []string
}

var (
	toolLoopWhitespaceRE = regexp.MustCompile(`\s+`)
	toolLoopDigitsRE     = regexp.MustCompile(`\d+`)
)

// Observe records one completed tool round and reports whether the loop should abort.
func (d *ToolLoopDetector) Observe(toolSignature, assistantDecision string, toolSummaries []string) ToolLoopDetection {
	normalizedToolSig := normalizeToolLoopText(toolSignature)
	normalizedDecision := normalizeToolLoopText(assistantDecision)
	normalizedSummaries := normalizeToolLoopSummaries(toolSummaries)

	if normalizedToolSig != "" {
		if normalizedToolSig == d.lastToolSignature {
			d.duplicateRounds++
		} else {
			d.lastToolSignature = normalizedToolSig
			d.duplicateRounds = 1
		}
		d.recentToolSignatures = append(d.recentToolSignatures, normalizedToolSig)
		if len(d.recentToolSignatures) > 4 {
			d.recentToolSignatures = append([]string(nil), d.recentToolSignatures[len(d.recentToolSignatures)-4:]...)
		}
	}

	combined := normalizedToolSig + "|" + strings.Join(normalizedSummaries, "|")
	if combined == d.lastCombinedSignature {
		d.stableRounds++
	} else {
		d.lastCombinedSignature = combined
		d.stableRounds = 1
	}

	allErrors := len(normalizedSummaries) > 0
	for _, summary := range normalizedSummaries {
		if !strings.HasPrefix(summary, "error:") {
			allErrors = false
			break
		}
	}
	if allErrors {
		errorSig := normalizedDecision + "|" + strings.Join(normalizedSummaries, "|")
		if errorSig == d.lastErrorSignature {
			d.errorRounds++
		} else {
			d.lastErrorSignature = errorSig
			d.errorRounds = 1
		}
		if d.errorRounds >= 3 {
			return ToolLoopDetection{
				Abort:     true,
				Reason:    ToolLoopReasonErrorRepeat,
				Streak:    d.errorRounds,
				Signature: errorSig,
			}
		}
	} else {
		d.lastErrorSignature = ""
		d.errorRounds = 0
	}

	if detectsToolPingPong(d.recentToolSignatures) {
		return ToolLoopDetection{
			Abort:     true,
			Reason:    ToolLoopReasonPingPong,
			Streak:    len(d.recentToolSignatures),
			Signature: strings.Join(d.recentToolSignatures, " -> "),
		}
	}

	if d.stableRounds >= 3 {
		return ToolLoopDetection{
			Abort:     true,
			Reason:    ToolLoopReasonPollingNoProgress,
			Streak:    d.stableRounds,
			Signature: combined,
		}
	}

	if d.duplicateRounds >= 3 {
		return ToolLoopDetection{
			Abort:     true,
			Reason:    ToolLoopReasonIdenticalRepeat,
			Streak:    d.duplicateRounds,
			Signature: normalizedToolSig,
		}
	}

	return ToolLoopDetection{}
}

// NormalizeToolProgressSummary extracts a compact, stable summary for loop detection.
func NormalizeToolProgressSummary(content string) string {
	trimmed := trimStructuredToolLoopContent(content)
	if trimmed == "" {
		return "empty"
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &obj); err == nil && len(obj) > 0 {
		if errMsg := extractToolLoopStringValue(obj, "error"); errMsg != "" {
			return "error:" + normalizeToolLoopText(errMsg)
		}
		if status := extractToolLoopStringValue(obj, "status"); status != "" {
			return "status:" + normalizeToolLoopText(status)
		}
		if summary := extractToolLoopStringValue(obj, "summary"); summary != "" {
			return "summary:" + normalizeToolLoopText(summary)
		}
		keys := make([]string, 0, len(obj))
		for key := range obj {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) > 4 {
			keys = keys[:4]
		}
		return "json:" + strings.Join(keys, ",")
	}

	return normalizeToolLoopText(trimmed)
}

func normalizeToolLoopSummaries(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "error:") ||
			strings.HasPrefix(value, "status:") ||
			strings.HasPrefix(value, "summary:") ||
			strings.HasPrefix(value, "json:") ||
			value == "empty" {
			out = append(out, value)
			continue
		}
		out = append(out, NormalizeToolProgressSummary(value))
	}
	return out
}

func normalizeToolLoopText(content string) string {
	normalized := strings.ToLower(strings.TrimSpace(content))
	normalized = toolLoopDigitsRE.ReplaceAllString(normalized, "#")
	normalized = toolLoopWhitespaceRE.ReplaceAllString(normalized, " ")
	if len(normalized) > 160 {
		normalized = normalized[:160]
	}
	return normalized
}

func detectsToolPingPong(signatures []string) bool {
	if len(signatures) < 4 {
		return false
	}
	window := signatures[len(signatures)-4:]
	return window[0] != "" &&
		window[1] != "" &&
		window[0] != window[1] &&
		window[0] == window[2] &&
		window[1] == window[3]
}

func trimStructuredToolLoopContent(content string) string {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

func extractToolLoopStringValue(obj map[string]interface{}, key string) string {
	value, ok := obj[key]
	if !ok {
		return ""
	}
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(str)
}
