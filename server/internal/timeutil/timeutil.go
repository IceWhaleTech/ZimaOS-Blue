// Package timeutil provides high-performance time functions with reduced GC pressure.
// Instead of calling time.Now() which creates temporary objects, this package
// caches the current timestamp and updates it periodically via a background goroutine.
//
// Inspired by ecache's time calibrator implementation.
// Reference: https://github.com/orcastor/orcas/blob/master/core/const.go
package timeutil

import (
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

// UnixNano returns the nanosecond timestamp for a time.Time.
// This is a convenience wrapper around t.UnixNano().
func UnixNano(t time.Time) int64 {
	return t.UnixNano()
}
