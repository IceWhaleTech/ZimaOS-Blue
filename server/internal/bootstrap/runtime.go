package bootstrap

import "runtime/debug"

// TuneGC configures Go runtime GC for lower memory usage.
// GOGC=50 balances memory vs CPU; 128MB soft limit avoids excessive GC thrashing.
// Should be called early in main/runServer before allocations ramp up.
func TuneGC() {
	debug.SetGCPercent(50)
	debug.SetMemoryLimit(128 * 1024 * 1024)
}
