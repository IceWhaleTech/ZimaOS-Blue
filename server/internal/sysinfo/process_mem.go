package sysinfo

// ProcessMemInfo contains process-level memory information.
type ProcessMemInfo struct {
	RSSB    uint64 `json:"rss_bytes"`    // Resident Set Size in bytes (matches OS activity monitor)
	GoAlloc uint64 `json:"go_alloc"`     // Go heap allocated bytes
	GoSys   uint64 `json:"go_sys"`       // Go total memory obtained from OS
}
