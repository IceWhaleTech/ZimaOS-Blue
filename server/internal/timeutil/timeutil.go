// Package timeutil provides high-performance time functions with reduced GC pressure.
// Instead of calling time.Now() which creates temporary objects, this package
// caches the current timestamp and updates it periodically via a background goroutine.
//
// Inspired by ecache's time calibrator implementation.
// Reference: https://github.com/orcastor/orcas/blob/master/core/const.go
package timeutil

import (
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// clock stores the cached nanosecond timestamp.
// p and n are internal counters used by ecache — kept for compatibility.
var (
	clock, p, n = time.Now().UnixNano(), uint16(0), uint16(1)

	// highestSeen is a monotonically increasing nanosecond timestamp.
	// It never goes backwards even if clock drifts during calibration.
	highestSeen int64
)

func init() {
	atomic.StoreInt64(&highestSeen, clock)
	go func() {
		for {
			atomic.StoreInt64(&clock, time.Now().UnixNano()) // calibration every second
			updateHighest(atomic.LoadInt64(&clock))
			for i := 0; i < 9; i++ {
				time.Sleep(100 * time.Millisecond)
				atomic.AddInt64(&clock, int64(100*time.Millisecond))
				updateHighest(atomic.LoadInt64(&clock))
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

func updateHighest(v int64) {
	for {
		old := atomic.LoadInt64(&highestSeen)
		if v <= old {
			return
		}
		if atomic.CompareAndSwapInt64(&highestSeen, old, v) {
			return
		}
	}
}

// NowNano returns the current time as nanoseconds since Unix epoch.
// This is a cached value updated every 100ms, suitable for most use cases
// where millisecond precision is acceptable.
func NowNano() int64 {
	return atomic.LoadInt64(&clock)
}

// Now returns the current time as seconds since Unix epoch.
func Now() int64 {
	return atomic.LoadInt64(&clock) / 1e9
}

// NowMilli returns the current time as milliseconds since Unix epoch.
func NowMilli() int64 {
	return atomic.LoadInt64(&clock) / 1e6
}

// NowMicro returns the current time as microseconds since Unix epoch.
func NowMicro() int64 {
	return atomic.LoadInt64(&clock) / 1e3
}

// NowTime returns a time.Time from the cached timestamp.
// Use this when you need a time.Time but don't need nanosecond precision.
// Note: This still creates a time.Time object, so use Now/NowNano/NowMilli
// when you only need the numeric timestamp.
func NowTime() time.Time {
	return time.Unix(0, atomic.LoadInt64(&clock))
}

// DetectTimezone returns the best available timezone label for the current process.
// It prefers an explicit TZ env override, then the current local location name,
// and finally falls back to a UTC offset string.
func DetectTimezone() string {
	if tz := strings.TrimSpace(os.Getenv("TZ")); tz != "" {
		if _, err := time.LoadLocation(tz); err == nil || isUTCOffsetTimezone(tz) {
			return tz
		}
	}
	for _, loc := range []*time.Location{time.Local, NowTime().Location()} {
		if loc == nil {
			continue
		}
		name := strings.TrimSpace(loc.String())
		if name != "" && name != "Local" {
			return name
		}
	}
	return FormatUTCOffset(NowTime())
}

// FormatUTCOffset renders the time zone offset as UTC±HH:MM.
func FormatUTCOffset(t time.Time) string {
	_, offsetSeconds := t.Zone()
	sign := '+'
	if offsetSeconds < 0 {
		sign = '-'
		offsetSeconds = -offsetSeconds
	}
	hours := offsetSeconds / 3600
	minutes := (offsetSeconds % 3600) / 60
	return "UTC" + string(sign) + twoDigit(hours) + ":" + twoDigit(minutes)
}

func isUTCOffsetTimezone(value string) bool {
	if len(value) != len("UTC+00:00") || !strings.HasPrefix(value, "UTC") {
		return false
	}
	if value[3] != '+' && value[3] != '-' {
		return false
	}
	if value[6] != ':' {
		return false
	}
	for _, idx := range []int{4, 5, 7, 8} {
		if value[idx] < '0' || value[idx] > '9' {
			return false
		}
	}
	return true
}

func twoDigit(v int) string {
	if v < 10 {
		return "0" + strconv.Itoa(v)
	}
	return strconv.Itoa(v)
}

// Monotonic returns a monotonically increasing nanosecond timestamp.
// Unlike NowNano, this value never goes backwards, making it safe for
// ordering events and deduplication (highestSeen pattern).
func Monotonic() int64 {
	return atomic.LoadInt64(&highestSeen)
}

// Since returns the duration since the given nanosecond timestamp.
func Since(nanoTimestamp int64) time.Duration {
	return time.Duration(NowNano() - nanoTimestamp)
}

// SinceTime returns the duration since the given time.
func SinceTime(t time.Time) time.Duration {
	return time.Duration(NowNano() - t.UnixNano())
}

// UntilTime returns the duration until the given time.
func UntilTime(t time.Time) time.Duration {
	return time.Duration(t.UnixNano() - NowNano())
}

// UnixNano returns the nanosecond timestamp for a time.Time.
// This is a convenience wrapper around t.UnixNano().
func UnixNano(t time.Time) int64 {
	return t.UnixNano()
}
