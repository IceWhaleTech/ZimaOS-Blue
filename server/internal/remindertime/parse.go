package remindertime

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

var durationTokenRE = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(milliseconds?|msecs?|msec|ms|毫秒|seconds?|secs?|sec|s|秒钟?|minutes?|mins?|min|m|分钟?|分|hours?|hrs?|hr|h|小时|小時|days?|day|d|天|weeks?|week|w|周|星期)`)

var commonTimeLayouts = []string{
	"2006-01-02 15:04",
	"2006-01-02 15:04:05",
}

// Parse parses reminder time strings.
//
// Supported formats:
// 1. Go duration: "10s", "30m", "1h30m"
// 2. Natural duration phrases: "in 10 seconds", "10秒钟以后", "提醒我10秒后喝水"
// 3. RFC3339 timestamp
// 4. Common local datetime: "YYYY-MM-DD HH:MM[:SS]"
func Parse(s string) (time.Time, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return time.Time{}, fmt.Errorf("invalid time format: empty value")
	}

	if d, ok := parseRelativeDuration(raw); ok {
		return timeutil.NowTime().Add(d), nil
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
