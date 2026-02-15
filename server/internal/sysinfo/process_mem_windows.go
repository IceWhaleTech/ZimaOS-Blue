//go:build windows

package sysinfo

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	psapi                     = windows.NewLazySystemDLL("psapi.dll")
	procGetProcessMemoryInfo  = psapi.NewProc("GetProcessMemoryInfo")
)

// PROCESS_MEMORY_COUNTERS structure
type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
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

	handle, err := windows.GetCurrentProcess()
	if err == nil {
		var pmc processMemoryCounters
		pmc.CB = uint32(unsafe.Sizeof(pmc))
		ret, _, _ := procGetProcessMemoryInfo.Call(
			uintptr(handle),
			uintptr(unsafe.Pointer(&pmc)),
			uintptr(pmc.CB),
		)
		if ret != 0 {
			info.RSSB = uint64(pmc.WorkingSetSize)
		}
	}

	// Fallback: use Go's Sys as approximation
	if info.RSSB == 0 {
		info.RSSB = m.Sys
	}

	return info
}
