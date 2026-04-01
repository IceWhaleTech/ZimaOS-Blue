package remindertime

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"golang.org/x/text/unicode/norm"
)

var durationTokenRE = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(milliseconds?|msecs?|msec|ms|毫秒|seconds?|secs?|sec|s|秒钟?|minutes?|mins?|min|m|分钟?|分|hours?|hrs?|hr|h|小时|小時|days?|day|d|天|weeks?|week|w|周|星期)`)
var naturalAbsoluteClockRE = regexp.MustCompile(`(?i)(\d{1,2})(?::(\d{2}))?\s*(am|pm|a\.m\.|p\.m\.|点|點|时|時|시|uhr|ora|orara|ore|മണിക്ക്|h)?`)
var naturalAbsolutePrefixRE = regexp.MustCompile(`(?i)(?:^|[\s,.;:()])(?:at|@|a\s+las|a\s+les|a|um|στις|ag|u|v|klokken|klockan|na|la|as|o|alle)\s*$`)

var commonTimeLayouts = []string{
	"2006-01-02 15:04",
	"2006-01-02 15:04:05",
}

var naturalTomorrowCues = initNaturalTomorrowCues()

var naturalMorningCues = []string{
	"morning",
	"matin",
	"mañana",
	"dema al mati",
	"dema pel mati",
	"明早",
	"明日朝",
	"朝",
	"아침",
	"രാവിലെ",
	"上午",
}

var naturalPMCues = []string{
	"pm",
	"p.m.",
	"afternoon",
	"evening",
	"tonight",
	"下午",
	"晚上",
	"今晚",
	"夕方",
	"夜",
	"저녁",
}

var nowTime = func() time.Time {
	return timeutil.NowTime().In(time.Local)
}

// Parse parses reminder time strings.
//
// Supported formats:
// 1. Go duration: "10s", "30m", "1h30m"
// 2. Natural duration phrases: "in 10 seconds", "10秒钟以后", "提醒我10秒后喝水"
// 3. RFC3339 timestamp
// 4. Common local datetime: "YYYY-MM-DD HH:MM[:SS]"
func Parse(s string) (time.Time, error) {
	return ParseAt(s, nowTime())
}

// ParseAt parses reminder time strings using the provided reference time for
// relative and natural-language expressions like "tomorrow at 9".
func ParseAt(s string, reference time.Time) (time.Time, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return time.Time{}, fmt.Errorf("invalid time format: empty value")
	}
	if reference.IsZero() {
		reference = nowTime()
	} else {
		reference = reference.In(time.Local)
	}

	if t, ok := parseNaturalAbsoluteTime(raw, reference); ok {
		return t, nil
	}

	if d, ok := parseRelativeDuration(raw); ok {
		return reference.Add(d), nil
	}

	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}

	for _, layout := range commonTimeLayouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid time format: %s (use duration like '10s'/'10秒', RFC3339, or 'YYYY-MM-DD HH:MM')", s)
}

// InferTimeStringFromMessage extracts a canonical reminder time from a full
// natural-language reminder message when the explicit time arg is missing.
func InferTimeStringFromMessage(message string) (string, bool) {
	parsed, err := Parse(message)
	if err != nil {
		return "", false
	}
	return parsed.Format("2006-01-02 15:04"), true
}

// ParseDuration parses an interval string without applying it to the current time.
func ParseDuration(s string) (time.Duration, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return 0, fmt.Errorf("invalid duration format: empty value")
	}
	if d, ok := parseRelativeDuration(raw); ok {
		return d, nil
	}
	return 0, fmt.Errorf("invalid duration format: %s (use duration like '2m', '1h30m', or '2分钟')", s)
}

func parseRelativeDuration(s string) (time.Duration, bool) {
	trimmed := strings.TrimSpace(s)
	if d, err := time.ParseDuration(trimmed); err == nil {
		return d, true
	}

	matches := durationTokenRE.FindAllStringSubmatch(trimmed, -1)
	if len(matches) == 0 {
		return 0, false
	}

	var total time.Duration
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}

		value, err := strconv.ParseFloat(m[1], 64)
		if err != nil || value <= 0 {
			continue
		}

		unitDur, ok := durationUnit(strings.ToLower(m[2]))
		if !ok {
			continue
		}

		total += time.Duration(value * float64(unitDur))
	}

	if total <= 0 {
		return 0, false
	}
	return total, true
}

func parseNaturalAbsoluteTime(s string, reference time.Time) (time.Time, bool) {
	folded := foldText(s)
	if folded == "" {
		return time.Time{}, false
	}

	window, ok := tomorrowWindow(folded)
	if !ok {
		return time.Time{}, false
	}

	hour, minute, ok := extractAbsoluteClock(window, folded)
	if !ok {
		return time.Time{}, false
	}

	if containsAnyFolded(folded, naturalPMCues...) && hour < 12 {
		hour += 12
	}
	if containsAnyFolded(folded, naturalMorningCues...) && hour == 12 {
		hour = 0
	}

	now := reference
	if now.IsZero() {
		now = nowTime()
	}
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location()).Add(24 * time.Hour)
	return target, true
}

func tomorrowWindow(folded string) (string, bool) {
	bestStart, bestEnd := -1, -1
	for _, cue := range naturalTomorrowCues {
		needle := foldText(cue)
		if needle == "" {
			continue
		}
		if idx := strings.Index(folded, needle); idx >= 0 && (bestStart == -1 || idx < bestStart) {
			bestStart = idx
			bestEnd = idx + len(needle)
		}
	}
	if bestStart < 0 {
		return "", false
	}
	windowEnd := min(bestEnd+64, len(folded))
	return folded[bestStart:windowEnd], true
}

func extractAbsoluteClock(window string, fullText string) (int, int, bool) {
	matches := naturalAbsoluteClockRE.FindAllStringSubmatchIndex(window, -1)
	for _, match := range matches {
		if len(match) < 8 {
			continue
		}
		hour, err := strconv.Atoi(window[match[2]:match[3]])
		if err != nil || hour < 0 || hour > 23 {
			continue
		}
		minute := 0
		if match[4] >= 0 && match[5] >= 0 {
			minute, err = strconv.Atoi(window[match[4]:match[5]])
			if err != nil || minute < 0 || minute > 59 {
				continue
			}
		}

		fullMatch := strings.TrimSpace(window[match[0]:match[1]])
		suffix := ""
		if match[6] >= 0 && match[7] >= 0 {
			suffix = strings.TrimSpace(window[match[6]:match[7]])
		}
		prefixStart := max(0, match[0]-24)
		prefix := strings.TrimSpace(window[prefixStart:match[0]])

		if !clockMatchHasExplicitTimeContext(fullMatch, prefix, suffix, window, fullText, minute) {
			continue
		}

		lowerSuffix := strings.ToLower(strings.TrimSpace(suffix))
		if strings.HasPrefix(lowerSuffix, "p") && hour < 12 {
			hour += 12
		}
		if strings.HasPrefix(lowerSuffix, "a") && hour == 12 {
			hour = 0
		}
		return hour, minute, true
	}
	return 0, 0, false
}

func clockMatchHasExplicitTimeContext(fullMatch string, prefix string, suffix string, window string, fullText string, minute int) bool {
	if minute > 0 || suffix != "" {
		return true
	}
	if naturalAbsolutePrefixRE.MatchString(prefix) {
		return true
	}
	if containsAnyFolded(window, naturalMorningCues...) || containsAnyFolded(window, naturalPMCues...) {
		return true
	}
	return containsAnyFolded(fullText, "明早", "明日朝")
}

func initNaturalTomorrowCues() []string {
	values := append([]string{}, routingcue.SkillTerms("reminder").Context...)
	values = append(values,
		"tomorrow",
		"mañana",
		"manana",
		"demain",
		"amarach",
		"amárach",
		"dema",
		"demà",
		"zitra",
		"zítra",
		"i morgen",
		"morgen",
		"αυριο",
		"αύριο",
		"sutra",
		"holnap",
		"domani",
		"明日",
		"明早",
		"明天",
		"내일",
		"നാളെ",
		"jutro",
		"amanha",
		"amanhã",
		"maine",
		"завтра",
		"zajtra",
		"i morgon",
	)
	return uniqueFoldedStrings(values)
}

func containsAnyFolded(text string, needles ...string) bool {
	folded := foldText(text)
	if folded == "" {
		return false
	}
	for _, needle := range needles {
		if normalized := foldText(needle); normalized != "" && strings.Contains(folded, normalized) {
			return true
		}
	}
	return false
}

func foldText(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	decomposed := norm.NFD.String(strings.ToLower(s))
	var b strings.Builder
	b.Grow(len(decomposed))
	lastSpace := false
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(b.String())
}

func uniqueFoldedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		key := foldText(value)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func durationUnit(unit string) (time.Duration, bool) {
	switch unit {
	case "ms", "millisecond", "milliseconds", "msec", "msecs", "毫秒":
		return time.Millisecond, true
	case "s", "sec", "secs", "second", "seconds", "秒", "秒钟":
		return time.Second, true
	case "m", "min", "mins", "minute", "minutes", "分", "分钟":
		return time.Minute, true
	case "h", "hr", "hrs", "hour", "hours", "小时", "小時":
		return time.Hour, true
	case "d", "day", "days", "天":
		return 24 * time.Hour, true
	case "w", "week", "weeks", "周", "星期":
		return 7 * 24 * time.Hour, true
	default:
		return 0, false
	}
}
