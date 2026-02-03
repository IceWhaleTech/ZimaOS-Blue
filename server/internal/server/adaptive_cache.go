package server

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// AdaptiveCacheManager dynamically adjusts cache size based on memory pressure
type AdaptiveCacheManager struct {
	// Current configuration
	maxSize       int64
	currentSize   int64
	minSize       int64
	maxMemoryMB   int64
	targetMemMB   int64

	// Memory monitoring
	lastMemCheck  time.Time
	memCheckMu    sync.RWMutex
	memoryTrend   []uint64 // Last N memory readings
	maxTrendSize  int

	// Eviction policy
	evictionChan  chan EvictionRequest
	stopChan      chan struct{}

	// Metrics
	resizes       int64
	evictions     int64
	memoryPressure float64
}

// EvictionRequest represents a cache eviction request
type EvictionRequest struct {
	Key      string
	Priority int
}

// NewAdaptiveCacheManager creates a new adaptive cache manager
func NewAdaptiveCacheManager(minSize, maxSize, maxMemoryMB int64) *AdaptiveCacheManager {
	acm := &AdaptiveCacheManager{
		minSize:       minSize,
		maxSize:       maxSize,
		currentSize:   minSize,
		maxMemoryMB:   maxMemoryMB,
		targetMemMB:   maxMemoryMB * 80 / 100, // Target 80% of max
		evictionChan:  make(chan EvictionRequest, 1000),
		stopChan:      make(chan struct{}),
		memoryTrend:   make([]uint64, 0, 60), // Track last 60 readings
		maxTrendSize:  60,
	}

	// Start memory monitor
	go acm.monitorMemory()

	return acm
}

// monitorMemory periodically checks memory usage and adjusts cache size
func (acm *AdaptiveCacheManager) monitorMemory() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-acm.stopChan:
			return
		case <-ticker.C:
			acm.checkAndAdjustMemory()
		}
	}
}

// checkAndAdjustMemory checks current memory usage and adjusts cache size
func (acm *AdaptiveCacheManager) checkAndAdjustMemory() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	currentMemMB := int64(m.Alloc / 1024 / 1024)

	acm.memCheckMu.Lock()
	defer acm.memCheckMu.Unlock()

	// Track memory trend
	acm.memoryTrend = append(acm.memoryTrend, m.Alloc)
	if len(acm.memoryTrend) > acm.maxTrendSize {
		acm.memoryTrend = acm.memoryTrend[1:]
	}

	// Calculate memory pressure (0.0 to 1.0)
	pressure := float64(currentMemMB) / float64(acm.maxMemoryMB)
	acm.memoryPressure = pressure

	// Adjust cache size based on memory pressure
	if pressure > 0.9 {
		// Critical: reduce cache size aggressively
		newSize := atomic.LoadInt64(&acm.currentSize) * 70 / 100
		if newSize < acm.minSize {
			newSize = acm.minSize
		}
		acm.resizeCache(newSize)
		atomic.AddInt64(&acm.evictions, 1)
	} else if pressure > 0.8 {
		// High: reduce cache size moderately
		newSize := atomic.LoadInt64(&acm.currentSize) * 85 / 100
		if newSize < acm.minSize {
			newSize = acm.minSize
		}
		acm.resizeCache(newSize)
	} else if pressure < 0.5 && atomic.LoadInt64(&acm.currentSize) < acm.maxSize {
		// Low: increase cache size
		newSize := atomic.LoadInt64(&acm.currentSize) * 110 / 100
		if newSize > acm.maxSize {
			newSize = acm.maxSize
		}
		acm.resizeCache(newSize)
	}
}

// resizeCache adjusts the cache size
func (acm *AdaptiveCacheManager) resizeCache(newSize int64) {
	oldSize := atomic.LoadInt64(&acm.currentSize)
	if oldSize != newSize {
		atomic.StoreInt64(&acm.currentSize, newSize)
		atomic.AddInt64(&acm.resizes, 1)
	}
}

// GetCacheSize returns the current recommended cache size
func (acm *AdaptiveCacheManager) GetCacheSize() int64 {
	return atomic.LoadInt64(&acm.currentSize)
}

// GetMemoryPressure returns current memory pressure (0.0-1.0)
func (acm *AdaptiveCacheManager) GetMemoryPressure() float64 {
	acm.memCheckMu.RLock()
	defer acm.memCheckMu.RUnlock()
	return acm.memoryPressure
}

// ShouldEvict returns true if cache should evict entries
func (acm *AdaptiveCacheManager) ShouldEvict() bool {
	acm.memCheckMu.RLock()
	defer acm.memCheckMu.RUnlock()
	return acm.memoryPressure > 0.75
}

// RequestEviction requests eviction of a cache entry
func (acm *AdaptiveCacheManager) RequestEviction(key string, priority int) {
	select {
	case acm.evictionChan <- EvictionRequest{Key: key, Priority: priority}:
	default:
		// Channel full, skip
	}
}

// Stop stops the memory monitor
func (acm *AdaptiveCacheManager) Stop() {
	close(acm.stopChan)
}

// GetMetrics returns adaptive cache metrics
func (acm *AdaptiveCacheManager) GetMetrics() map[string]interface{} {
	acm.memCheckMu.RLock()
	defer acm.memCheckMu.RUnlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"current_cache_size":  atomic.LoadInt64(&acm.currentSize),
		"min_cache_size":      acm.minSize,
		"max_cache_size":      acm.maxSize,
		"memory_pressure":     acm.memoryPressure,
		"allocated_memory_mb": int64(m.Alloc / 1024 / 1024),
		"total_memory_mb":     int64(m.TotalAlloc / 1024 / 1024),
		"gc_runs":             m.NumGC,
		"resizes":             atomic.LoadInt64(&acm.resizes),
		"evictions":           atomic.LoadInt64(&acm.evictions),
	}
}
