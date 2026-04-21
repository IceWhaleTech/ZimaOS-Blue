package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
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

type toolLoopRound struct {
	toolSignature    string
	outcomeSignature string
}

// ToolLoopDetector tracks recent tool rounds to detect loop patterns at runtime.
type ToolLoopDetector struct {
	lastToolSignature       string
	duplicateRounds         int
	lastNoProgressSignature string
	noProgressRounds        int
	lastErrorSignature      string
	errorRounds             int
	recentRounds            []toolLoopRound
}

var (
	toolLoopRegexesOnce  sync.Once
	toolLoopWhitespaceRE *regexp.Regexp
	toolLoopDigitsRE     *regexp.Regexp
)

func ensureToolLoopRegexes() {
	toolLoopRegexesOnce.Do(func() {
		toolLoopWhitespaceRE = regexp.MustCompile(`\s+`)
		toolLoopDigitsRE = regexp.MustCompile(`\d+`)
	})
}

// Observe records one completed tool round and reports whether the loop should abort.
func (d *ToolLoopDetector) Observe(toolSignature, assistantDecision string, toolSummaries []string, progressMarkers ...string) ToolLoopDetection {
	normalizedToolSig := normalizeToolLoopText(toolSignature)
	normalizedDecision := normalizeToolLoopText(assistantDecision)
	exactToolSig := fingerprintToolLoopText(toolSignature)
	exactDecision := fingerprintToolLoopText(assistantDecision)
	normalizedSummaries := normalizeToolLoopSummaries(toolSummaries)
	normalizedProgress := normalizeToolLoopProgressMarkers(progressMarkers)
	if exactToolSig == "" {
		exactToolSig = normalizedToolSig
	}
	if exactDecision == "" {
		exactDecision = normalizedDecision
	}

	if len(normalizedProgress) > 0 {
		d.reset()
		return ToolLoopDetection{}
	}

	if exactToolSig != "" {
		if exactToolSig == d.lastToolSignature {
			d.duplicateRounds++
		} else {
			d.lastToolSignature = exactToolSig
			d.duplicateRounds = 1
		}
	}

	outcomeSig := strings.Join(normalizedSummaries, "|")
	if outcomeSig == "" {
		outcomeSig = "empty"
	}
	noProgressSig := exactToolSig + "|" + outcomeSig
	if noProgressSig == d.lastNoProgressSignature {
		d.noProgressRounds++
	} else {
		d.lastNoProgressSignature = noProgressSig
		d.noProgressRounds = 1
	}

	allErrors := len(normalizedSummaries) > 0
	for _, summary := range normalizedSummaries {
		if !strings.HasPrefix(summary, "error:") {
			allErrors = false
			break
		}
	}
	if allErrors {
		errorSig := exactToolSig + "|" + exactDecision + "|" + outcomeSig
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

	d.recentRounds = append(d.recentRounds, toolLoopRound{
		toolSignature:    exactToolSig,
		outcomeSignature: outcomeSig,
	})
	if len(d.recentRounds) > 4 {
		d.recentRounds = append([]toolLoopRound(nil), d.recentRounds[len(d.recentRounds)-4:]...)
	}

	if detectsToolPingPong(d.recentRounds) {
		window := d.recentRounds
		if len(window) > 4 {
			window = window[len(window)-4:]
		}
		sigs := make([]string, 0, len(window))
		for _, round := range window {
			sigs = append(sigs, round.toolSignature)
		}
		return ToolLoopDetection{
			Abort:     true,
			Reason:    ToolLoopReasonPingPong,
			Streak:    len(window),
			Signature: strings.Join(sigs, " -> "),
		}
	}

	if d.noProgressRounds >= 3 {
		return ToolLoopDetection{
			Abort:     true,
			Reason:    ToolLoopReasonPollingNoProgress,
			Streak:    d.noProgressRounds,
			Signature: noProgressSig,
		}
	}

	if !isKnownPollingToolLoopSignature(normalizedToolSig) && d.duplicateRounds >= 4 {
		return ToolLoopDetection{
			Abort:     true,
			Reason:    ToolLoopReasonIdenticalRepeat,
			Streak:    d.duplicateRounds,
			Signature: normalizedToolSig,
		}
	}

	return ToolLoopDetection{}
}

func (d *ToolLoopDetector) reset() {
	d.lastToolSignature = ""
	d.duplicateRounds = 0
	d.lastNoProgressSignature = ""
	d.noProgressRounds = 0
	d.lastErrorSignature = ""
	d.errorRounds = 0
	d.recentRounds = nil
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

		parts := make([]string, 0, 8)
		if status := extractToolLoopStringValue(obj, "status"); status != "" {
			parts = append(parts, "status:"+normalizeToolLoopText(status))
		}
		if success, ok := extractToolLoopBoolValue(obj, "success"); ok {
			if success {
				parts = append(parts, "success:true")
			} else {
				parts = append(parts, "success:false")
			}
		}
		if summary := extractToolLoopStringValue(obj, "summary"); summary != "" {
			parts = append(parts, "summary:"+normalizeToolLoopText(summary))
		}
		for _, key := range []string{"mode", "next_action", "path", "file_path", "url", "target_url", "final_url", "query", "input", "session_id", "sessionId"} {
			if value := extractToolLoopStringValue(obj, key); value != "" {
				parts = append(parts, key+":"+normalizeToolLoopText(value))
			}
		}
		for _, key := range []string{"evidence_count", "results_count", "count", "total", "total_results"} {
			if value, ok := extractToolLoopNumericValue(obj, key); ok {
				parts = append(parts, key+":"+normalizeToolLoopText(value))
			}
		}
		for _, key := range []string{"results", "items", "evidence"} {
			if length, ok := extractToolLoopArrayLength(obj, key); ok {
				parts = append(parts, key+"_len:"+normalizeToolLoopText(length))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "|")
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
	ensureToolLoopRegexes()

	normalized := strings.ToLower(strings.TrimSpace(content))
	normalized = toolLoopDigitsRE.ReplaceAllString(normalized, "#")
	normalized = toolLoopWhitespaceRE.ReplaceAllString(normalized, " ")
	if len(normalized) > 160 {
		normalized = normalized[:160]
	}
	return normalized
}

func fingerprintToolLoopText(content string) string {
	ensureToolLoopRegexes()

	normalized := strings.ToLower(strings.TrimSpace(content))
	normalized = toolLoopWhitespaceRE.ReplaceAllString(normalized, " ")
	if len(normalized) > 240 {
		normalized = normalized[:240]
	}
	return normalized
}

func normalizeToolLoopProgressMarkers(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeToolLoopText(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func detectsToolPingPong(rounds []toolLoopRound) bool {
	if len(rounds) < 4 {
		return false
	}
	window := rounds[len(rounds)-4:]
	return window[0].toolSignature != "" &&
		window[1].toolSignature != "" &&
		window[0].outcomeSignature != "" &&
		window[1].outcomeSignature != "" &&
		window[0].toolSignature != window[1].toolSignature &&
		window[0].toolSignature == window[2].toolSignature &&
		window[1].toolSignature == window[3].toolSignature &&
		window[0].outcomeSignature == window[2].outcomeSignature &&
		window[1].outcomeSignature == window[3].outcomeSignature
}

func isKnownPollingToolLoopSignature(signature string) bool {
	normalized := normalizeToolLoopText(signature)
	if normalized == "" {
		return false
	}
	if strings.HasPrefix(normalized, "command_status:") || strings.HasPrefix(normalized, "research_status:") {
		return true
	}
	if strings.HasPrefix(normalized, "deep_research:") &&
		(strings.Contains(normalized, `"action":"status"`) || strings.Contains(normalized, "action=status")) {
		return true
	}
	if !strings.HasPrefix(normalized, "process:") {
		return false
	}
	return strings.Contains(normalized, `"action":"poll"`) ||
		strings.Contains(normalized, `"action":"log"`) ||
		strings.Contains(normalized, "action=poll") ||
		strings.Contains(normalized, "action=log")
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

func extractToolLoopBoolValue(obj map[string]interface{}, key string) (bool, bool) {
	value, ok := obj[key]
	if !ok {
		return false, false
	}
	b, ok := value.(bool)
	return b, ok
}

func extractToolLoopNumericValue(obj map[string]interface{}, key string) (string, bool) {
	value, ok := obj[key]
	if !ok {
		return "", false
	}
	switch v := value.(type) {
	case float64:
		return fmt.Sprintf("%.0f", v), true
	case string:
		if strings.TrimSpace(v) == "" {
			return "", false
		}
		return strings.TrimSpace(v), true
	default:
		return "", false
	}
}

func extractToolLoopArrayLength(obj map[string]interface{}, key string) (string, bool) {
	value, ok := obj[key]
	if !ok {
		return "", false
	}
	items, ok := value.([]interface{})
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%d", len(items)), true
}
