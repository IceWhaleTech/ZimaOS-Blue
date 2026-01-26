// Package resources provides resource limit management for the application.
package resources

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Config holds resource limit configuration.
type Config struct {
	// MaxMemoryMB is the maximum memory in megabytes (0 = no limit).
	MaxMemoryMB int64 `yaml:"max_memory_mb" mapstructure:"max_memory_mb"`

	// MaxCPUPercent is the maximum CPU percentage (0 = no limit).
	MaxCPUPercent int `yaml:"max_cpu_percent" mapstructure:"max_cpu_percent"`

	// MaxOpenFiles is the maximum number of open files.
	MaxOpenFiles uint64 `yaml:"max_open_files" mapstructure:"max_open_files"`

	// MaxGoroutines is the maximum number of goroutines (0 = no limit).
	MaxGoroutines int `yaml:"max_goroutines" mapstructure:"max_goroutines"`

	// GCPercent is the garbage collection target percentage.
	GCPercent int `yaml:"gc_percent" mapstructure:"gc_percent"`
}

// DefaultConfig returns the default resource configuration.
func DefaultConfig() Config {
	return Config{
		MaxMemoryMB:   512,
		MaxCPUPercent: 50,
		MaxOpenFiles:  65536,
		MaxGoroutines: 10000,
		GCPercent:     100,
	}
}

// Limiter manages resource limits for the application.
type Limiter struct {
	config Config
}

// NewLimiter creates a new resource limiter.
func NewLimiter(config Config) *Limiter {
	return &Limiter{
		config: config,
	}
}

// Apply applies the resource limits.
func (l *Limiter) Apply() error {
	// Set Go runtime memory limit
	if l.config.MaxMemoryMB > 0 {
		memLimit := l.config.MaxMemoryMB * 1024 * 1024
		debug.SetMemoryLimit(memLimit)
	}

	// Set GC percentage
	if l.config.GCPercent > 0 {
		debug.SetGCPercent(l.config.GCPercent)
	}

	// Set GOMAXPROCS based on CPU limit
	if l.config.MaxCPUPercent > 0 && l.config.MaxCPUPercent < 100 {
		numCPU := runtime.NumCPU()
		maxProcs := (numCPU * l.config.MaxCPUPercent) / 100
		if maxProcs < 1 {
			maxProcs = 1
		}
		runtime.GOMAXPROCS(maxProcs)
	}

	// Apply OS-specific limits (rlimit)
	if err := l.applyRlimits(); err != nil {
		return fmt.Errorf("failed to apply rlimits: %w", err)
	}

	return nil
}

// Stats returns current resource usage statistics.
func (l *Limiter) Stats() ResourceStats {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return ResourceStats{
		AllocMB:       int64(memStats.Alloc / 1024 / 1024),
		TotalAllocMB:  int64(memStats.TotalAlloc / 1024 / 1024),
		SysMB:         int64(memStats.Sys / 1024 / 1024),
		NumGoroutines: runtime.NumGoroutine(),
		NumCPU:        runtime.NumCPU(),
		GOMAXPROCS:    runtime.GOMAXPROCS(0),
		NumGC:         memStats.NumGC,
		GCPauseNs:     memStats.PauseNs[(memStats.NumGC+255)%256],
	}
}

// ResourceStats holds current resource usage statistics.
type ResourceStats struct {
	AllocMB       int64  `json:"alloc_mb"`
	TotalAllocMB  int64  `json:"total_alloc_mb"`
	SysMB         int64  `json:"sys_mb"`
	NumGoroutines int    `json:"num_goroutines"`
	NumCPU        int    `json:"num_cpu"`
	GOMAXPROCS    int    `json:"gomaxprocs"`
	NumGC         uint32 `json:"num_gc"`
	GCPauseNs     uint64 `json:"gc_pause_ns"`
}

// CheckGoroutineLimit checks if the goroutine limit has been exceeded.
func (l *Limiter) CheckGoroutineLimit() error {
	if l.config.MaxGoroutines > 0 {
		current := runtime.NumGoroutine()
		if current > l.config.MaxGoroutines {
			return fmt.Errorf("goroutine limit exceeded: %d > %d", current, l.config.MaxGoroutines)
		}
	}
	return nil
}

// Config returns the current configuration.
func (l *Limiter) Config() Config {
	return l.config
}
