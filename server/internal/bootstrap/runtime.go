package bootstrap

import (
	"context"
	"runtime"
	"runtime/debug"
	"slices"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sysinfo"
	"go.uber.org/zap"
)

// TuneGC configures Go runtime GC for lower memory usage.
// GOGC=50 balances memory vs CPU; 128MB soft limit avoids excessive GC thrashing.
// Should be called early in main/runServer before allocations ramp up.
func TuneGC() {
	debug.SetGCPercent(50)
	debug.SetMemoryLimit(128 * 1024 * 1024)
}

var defaultStartupMemoryTrimSchedule = []time.Duration{
	8 * time.Second,
	20 * time.Second,
}

// StartupMemoryTrimSchedule returns the default post-start trim checkpoints.
func StartupMemoryTrimSchedule() []time.Duration {
	return append([]time.Duration(nil), defaultStartupMemoryTrimSchedule...)
}

// RunStartupMemoryTrimLoop schedules best-effort heap reclamation after startup.
// It is intended to run in a lifecycle-managed goroutine once the server is ready.
func RunStartupMemoryTrimLoop(ctx context.Context, logger *zap.Logger, delays ...time.Duration) {
	runStartupMemoryTrimLoop(ctx, normalizeStartupMemoryTrimDelays(delays), func(delay time.Duration) {
		trimStartupMemory(logger, delay)
	}, waitForStartupMemoryTrimDelay)
}

func normalizeStartupMemoryTrimDelays(delays []time.Duration) []time.Duration {
	out := make([]time.Duration, 0, len(delays))
	for _, delay := range delays {
		if delay > 0 {
			out = append(out, delay)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func runStartupMemoryTrimLoop(
	ctx context.Context,
	delays []time.Duration,
	trim func(time.Duration),
	wait func(context.Context, time.Duration) bool,
) {
	if len(delays) == 0 || trim == nil || wait == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	prev := time.Duration(0)
	for _, delay := range delays {
		if !wait(ctx, delay-prev) {
			return
		}
		trim(delay)
		prev = delay
	}
}

func waitForStartupMemoryTrimDelay(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx == nil || ctx.Err() == nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	if ctx == nil {
		<-timer.C
		return true
	}

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func trimStartupMemory(logger *zap.Logger, delay time.Duration) {
	beforeProc := sysinfo.GetProcessMemInfo()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	debug.FreeOSMemory()

	afterProc := sysinfo.GetProcessMemInfo()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	if logger != nil {
		logger.Info("Startup memory trim completed",
			zap.Duration("scheduled_after", delay),
			zap.Uint64("heap_alloc_before_bytes", before.Alloc),
			zap.Uint64("heap_alloc_after_bytes", after.Alloc),
			zap.Uint64("heap_sys_before_bytes", before.HeapSys),
			zap.Uint64("heap_sys_after_bytes", after.HeapSys),
			zap.Uint64("rss_before_bytes", beforeProc.RSSB),
			zap.Uint64("rss_after_bytes", afterProc.RSSB),
			zap.Uint32("num_gc_before", before.NumGC),
			zap.Uint32("num_gc_after", after.NumGC),
		)
	}
}
