package metrics

import (
	"runtime"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sysinfo"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// SystemMetrics represents a snapshot of system metrics at a point in time
type SystemMetrics struct {
	Timestamp       time.Time `json:"timestamp"`
	CPUPercent      float64   `json:"cpu_percent"`
	MemoryUsedBytes uint64    `json:"memory_used_bytes"`
	MemoryRSSBytes  uint64    `json:"memory_rss_bytes"`
	MemoryTotalBytes uint64   `json:"memory_total_bytes"`
	Goroutines      int       `json:"goroutines"`
	GCPauseNs       uint64    `json:"gc_pause_ns"`
	HeapAllocBytes  uint64    `json:"heap_alloc_bytes"`
	HeapSysBytes    uint64    `json:"heap_sys_bytes"`
	StackInuseBytes uint64    `json:"stack_inuse_bytes"`
}

// MetricsHistory contains historical metrics data
type MetricsHistory struct {
	Metrics         []SystemMetrics `json:"metrics"`
	IntervalSeconds int             `json:"interval_seconds"`
}

// Collector collects and stores system metrics history
type Collector struct {
	mu              sync.RWMutex
	history         []SystemMetrics
	maxHistory      int
	interval        time.Duration
	stopCh          chan struct{}
	lastCPUTime     uint64
	lastCPUSample   time.Time
}

// NewCollector creates a new metrics collector
func NewCollector(interval time.Duration, maxHistory int) *Collector {
	return &Collector{
		history:    make([]SystemMetrics, 0, maxHistory),
		maxHistory: maxHistory,
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

// Start begins collecting metrics at the configured interval
func (c *Collector) Start() {
	// Collect initial sample
	c.collect()

	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.collect()
			case <-c.stopCh:
				return
			}
		}
	}()
}

// Stop stops the metrics collector
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already stopped
	select {
	case <-c.stopCh:
		// Already stopped
		return
	default:
		close(c.stopCh)
	}
}

// collect gathers current system metrics and stores them
func (c *Collector) collect() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	procMem := sysinfo.GetProcessMemInfo()

	metrics := SystemMetrics{
		Timestamp:        timeutil.NowTime(),
		CPUPercent:       c.estimateCPUPercent(),
		MemoryUsedBytes:  m.Alloc,
		MemoryRSSBytes:   procMem.RSSB,
		MemoryTotalBytes: m.Sys,
		Goroutines:       runtime.NumGoroutine(),
		GCPauseNs:        m.PauseTotalNs,
		HeapAllocBytes:   m.HeapAlloc,
		HeapSysBytes:     m.HeapSys,
		StackInuseBytes:  m.StackInuse,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.history = append(c.history, metrics)

	// Trim history if it exceeds max
	if len(c.history) > c.maxHistory {
		c.history = c.history[len(c.history)-c.maxHistory:]
	}
}

// estimateCPUPercent estimates CPU usage based on goroutines and GC activity
// Note: This is a rough estimate since Go doesn't provide direct CPU metrics
func (c *Collector) estimateCPUPercent() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	now := timeutil.NowTime()

	// Calculate based on GC CPU fraction and goroutine count
	gcCPU := m.GCCPUFraction * 100

	// Estimate based on number of goroutines relative to CPUs
	numCPU := float64(runtime.NumCPU())
	goroutines := float64(runtime.NumGoroutine())

	// Simple heuristic: more goroutines = potentially more CPU usage
	// Cap at 100% and scale based on CPU count
	goroutineCPU := (goroutines / (numCPU * 10)) * 100
	if goroutineCPU > 100 {
		goroutineCPU = 100
	}

	// Combine GC CPU and goroutine estimate
	cpuPercent := gcCPU + goroutineCPU*0.1
	if cpuPercent > 100 {
		cpuPercent = 100
	}

	c.lastCPUSample = now

	return cpuPercent
}

// GetHistory returns metrics history for the specified duration
func (c *Collector) GetHistory(duration time.Duration) MetricsHistory {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cutoff := timeutil.NowTime().Add(-duration)

	var filtered []SystemMetrics
	for _, m := range c.history {
		if m.Timestamp.After(cutoff) {
			filtered = append(filtered, m)
		}
	}

	return MetricsHistory{
		Metrics:         filtered,
		IntervalSeconds: int(c.interval.Seconds()),
	}
}

// GetLatest returns the most recent metrics snapshot
func (c *Collector) GetLatest() *SystemMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.history) == 0 {
		return nil
	}

	latest := c.history[len(c.history)-1]
	return &latest
}
