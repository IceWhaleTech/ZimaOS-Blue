// Package timeutil provides high-performance time functions with reduced GC pressure.
// Instead of calling time.Now() which creates temporary objects, this package
// caches the current timestamp and updates it periodically via a background goroutine.
//
// Reference: https://github.com/orcastor/orcas/blob/master/core/const.go
package timeutil

import (
	"sync"
	"sync/atomic"
	"time"
)

var (
	// clock stores the cached nanosecond timestamp
	clock    int64
	initOnce sync.Once
)

func ensureStarted() {
	initOnce.Do(func() {
		atomic.StoreInt64(&clock, time.Now().UnixNano())
		go calibrate()
	})
}

// calibrate updates the cached timestamp periodically.
// It performs full calibration every second via time.Now().UnixNano(),
// and between calibrations, increments the cached value by 100ms intervals.
func calibrate() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	count := 0
	for range ticker.C {
		count++
		if count >= 10 {
			// Full calibration every second
			atomic.StoreInt64(&clock, time.Now().UnixNano())
			count = 0
		} else {
			// Increment by 100ms between calibrations
			atomic.AddInt64(&clock, 100*1e6)
		}
	}
}

// NowNano returns the current time as nanoseconds since Unix epoch.
// This is a cached value updated every 100ms, suitable for most use cases
// where millisecond precision is acceptable.
func NowNano() int64 {
	ensureStarted()
	return atomic.LoadInt64(&clock)
}

// Now returns the current time as seconds since Unix epoch.
func Now() int64 {
	ensureStarted()
	return atomic.LoadInt64(&clock) / 1e9
}

// NowMilli returns the current time as milliseconds since Unix epoch.
func NowMilli() int64 {
	ensureStarted()
	return atomic.LoadInt64(&clock) / 1e6
}

// NowMicro returns the current time as microseconds since Unix epoch.
func NowMicro() int64 {
	ensureStarted()
	return atomic.LoadInt64(&clock) / 1e3
}

// NowTime returns a time.Time from the cached timestamp.
// Use this when you need a time.Time but don't need nanosecond precision.
// Note: This still creates a time.Time object, so use Now/NowNano/NowMilli
// when you only need the numeric timestamp.
func NowTime() time.Time {
	ensureStarted()
	return time.Unix(0, atomic.LoadInt64(&clock))
}

// Since returns the duration since the given nanosecond timestamp.
func Since(nanoTimestamp int64) time.Duration {
	ensureStarted()
	return time.Duration(NowNano() - nanoTimestamp)
}

// SinceTime returns the duration since the given time.
func SinceTime(t time.Time) time.Duration {
	ensureStarted()
	return time.Duration(NowNano() - t.UnixNano())
}

// UnixNano returns the nanosecond timestamp for a time.Time.
// This is a convenience wrapper around t.UnixNano().
func UnixNano(t time.Time) int64 {
	return t.UnixNano()
}
