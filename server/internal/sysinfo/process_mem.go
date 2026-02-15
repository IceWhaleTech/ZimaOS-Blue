package sysinfo

import (
	"os"
	"runtime"
	"syscall"
)

// ProcessMemInfo contains process-level memory information.
type ProcessMemInfo struct {
	RSSB    uint64 `json:"rss_bytes"`    // Resident Set Size in bytes (matches OS activity monitor)
	GoAlloc uint64 `json:"go_alloc"`     // Go heap allocated bytes
	GoSys   uint64 `json:"go_sys"`       // Go total memory obtained from OS
}

// GetProcessMemInfo returns the current process memory usage.
// RSS is the value that matches what the OS activity monitor shows.
func GetProcessMemInfo() ProcessMemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	info := ProcessMemInfo{
		GoAlloc: m.Alloc,
		GoSys:   m.Sys,
	}

	var rusage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage); err == nil {
		rss := uint64(rusage.Maxrss)
		// On macOS, Maxrss is in bytes; on Linux, it's in KB
		if runtime.GOOS == "linux" {
			rss *= 1024
		}
		info.RSSB = rss
	}

	// Fallback: try reading /proc/self/statm on Linux
	if info.RSSB == 0 && runtime.GOOS == "linux" {
		info.RSSB = readProcRSS()
	}

	// Final fallback: use Go's Sys as approximation
	if info.RSSB == 0 {
		info.RSSB = m.Sys
	}

	return info
}

// readProcRSS reads RSS from /proc/self/statm on Linux.
func readProcRSS() uint64 {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0
	}
	// statm format: size resident shared text lib data dt (all in pages)
	// We want the second field (resident)
	var size, resident uint64
	n := parseUint64Fields(data, &size, &resident)
	if n < 2 {
		return 0
	}
	pageSize := uint64(os.Getpagesize())
	return resident * pageSize
}

// parseUint64Fields parses space-separated uint64 values from a byte slice.
func parseUint64Fields(data []byte, fields ...*uint64) int {
	i := 0
	parsed := 0
	for _, f := range fields {
		// Skip whitespace
		for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n') {
			i++
		}
		if i >= len(data) {
			break
		}
		// Parse number
		var val uint64
		start := i
		for i < len(data) && data[i] >= '0' && data[i] <= '9' {
			val = val*10 + uint64(data[i]-'0')
			i++
		}
		if i == start {
			break
		}
		*f = val
		parsed++
	}
	return parsed
}
