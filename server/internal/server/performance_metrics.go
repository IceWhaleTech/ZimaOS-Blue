package server

import (
	"sync"
	"sync/atomic"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// PerformanceMetrics tracks detailed performance metrics for the chat system
type PerformanceMetrics struct {
	// Request metrics
	totalRequests      int64
	successfulRequests int64
	failedRequests     int64

	// Latency tracking (in milliseconds)
	totalLatency    int64
	minLatency      int64
	maxLatency      int64
	p50Latency      int64
	p95Latency      int64
	p99Latency      int64

	// Provider metrics
	providerCacheHits   int64
	providerCacheMisses int64

	// Memory metrics
	gcPauses      int64
	allocatedMem  int64
	freedMem      int64

	// Throughput
	tokensProcessed int64
	requestsPerSec  int64

	// Latency histogram for percentile calculation
	latencies []int64
	latencyMu sync.RWMutex

	// Time tracking
	startTime time.Time
	lastReset time.Time
}

// RequestMetric represents a single request's metrics
type RequestMetric struct {
	StartTime      time.Time
	EndTime        time.Time
	Duration       time.Duration
	Provider       string
	Model          string
	TokensUsed     int
	CacheHit       bool
	Success        bool
	ErrorMessage   string
}

// NewPerformanceMetrics creates a new performance metrics tracker
func NewPerformanceMetrics() *PerformanceMetrics {
	return &PerformanceMetrics{
		startTime:  timeutil.NowTime(),
		lastReset:  timeutil.NowTime(),
		latencies:  make([]int64, 0, 10000),
		minLatency: 1<<63 - 1, // Max int64
	}
}

// RecordRequest records metrics for a completed request
func (pm *PerformanceMetrics) RecordRequest(metric *RequestMetric) {
	duration := metric.EndTime.Sub(metric.StartTime).Milliseconds()

	atomic.AddInt64(&pm.totalRequests, 1)
	if metric.Success {
		atomic.AddInt64(&pm.successfulRequests, 1)
	} else {
		atomic.AddInt64(&pm.failedRequests, 1)
	}

	atomic.AddInt64(&pm.totalLatency, duration)
	atomic.AddInt64(&pm.tokensProcessed, int64(metric.TokensUsed))

	// Update min/max latency
	for {
		currentMin := atomic.LoadInt64(&pm.minLatency)
		if duration >= currentMin || atomic.CompareAndSwapInt64(&pm.minLatency, currentMin, duration) {
			break
		}
	}

	for {
		currentMax := atomic.LoadInt64(&pm.maxLatency)
		if duration <= currentMax || atomic.CompareAndSwapInt64(&pm.maxLatency, currentMax, duration) {
			break
		}
	}

	// Store latency for percentile calculation
	pm.latencyMu.Lock()
	pm.latencies = append(pm.latencies, duration)
	if len(pm.latencies) > 100000 {
		// Keep only recent latencies to avoid memory bloat
		pm.latencies = pm.latencies[len(pm.latencies)-50000:]
	}
	pm.latencyMu.Unlock()

	if metric.CacheHit {
		atomic.AddInt64(&pm.providerCacheHits, 1)
	} else {
		atomic.AddInt64(&pm.providerCacheMisses, 1)
	}
}

// RecordProviderCacheHit records a provider cache hit
func (pm *PerformanceMetrics) RecordProviderCacheHit() {
	atomic.AddInt64(&pm.providerCacheHits, 1)
}

// RecordProviderCacheMiss records a provider cache miss
func (pm *PerformanceMetrics) RecordProviderCacheMiss() {
	atomic.AddInt64(&pm.providerCacheMisses, 1)
}

// RecordGCPause records a garbage collection pause
func (pm *PerformanceMetrics) RecordGCPause() {
	atomic.AddInt64(&pm.gcPauses, 1)
}

// calculatePercentile calculates a percentile from latency data
func (pm *PerformanceMetrics) calculatePercentile(percentile float64) int64 {
	pm.latencyMu.RLock()
	defer pm.latencyMu.RUnlock()

	if len(pm.latencies) == 0 {
		return 0
	}

	// Simple percentile calculation (not perfectly accurate but fast)
	index := int(float64(len(pm.latencies)) * percentile / 100.0)
	if index >= len(pm.latencies) {
		index = len(pm.latencies) - 1
	}
	return pm.latencies[index]
}

// GetMetrics returns current performance metrics
func (pm *PerformanceMetrics) GetMetrics() map[string]interface{} {
	totalReqs := atomic.LoadInt64(&pm.totalRequests)
	successReqs := atomic.LoadInt64(&pm.successfulRequests)
	failedReqs := atomic.LoadInt64(&pm.failedRequests)
	totalLat := atomic.LoadInt64(&pm.totalLatency)
	tokensProc := atomic.LoadInt64(&pm.tokensProcessed)
	cacheHits := atomic.LoadInt64(&pm.providerCacheHits)
	cacheMisses := atomic.LoadInt64(&pm.providerCacheMisses)

	avgLatency := int64(0)
	if totalReqs > 0 {
		avgLatency = totalLat / totalReqs
	}

	successRate := float64(0)
	if totalReqs > 0 {
		successRate = float64(successReqs) / float64(totalReqs) * 100
	}

	cacheHitRate := float64(0)
	if cacheHits+cacheMisses > 0 {
		cacheHitRate = float64(cacheHits) / float64(cacheHits+cacheMisses) * 100
	}

	uptime := timeutil.SinceTime(pm.startTime).Seconds()
	rps := float64(0)
	if uptime > 0 {
		rps = float64(totalReqs) / uptime
	}

	return map[string]interface{}{
		"total_requests":        totalReqs,
		"successful_requests":   successReqs,
		"failed_requests":       failedReqs,
		"success_rate":          successRate,
		"avg_latency_ms":        avgLatency,
		"min_latency_ms":        atomic.LoadInt64(&pm.minLatency),
		"max_latency_ms":        atomic.LoadInt64(&pm.maxLatency),
		"p50_latency_ms":        pm.calculatePercentile(50),
		"p95_latency_ms":        pm.calculatePercentile(95),
		"p99_latency_ms":        pm.calculatePercentile(99),
		"tokens_processed":      tokensProc,
		"cache_hits":            cacheHits,
		"cache_misses":          cacheMisses,
		"cache_hit_rate":        cacheHitRate,
		"gc_pauses":             atomic.LoadInt64(&pm.gcPauses),
		"requests_per_second":   rps,
		"uptime_seconds":        uptime,
	}
}

// Reset resets all metrics
func (pm *PerformanceMetrics) Reset() {
	atomic.StoreInt64(&pm.totalRequests, 0)
	atomic.StoreInt64(&pm.successfulRequests, 0)
	atomic.StoreInt64(&pm.failedRequests, 0)
	atomic.StoreInt64(&pm.totalLatency, 0)
	atomic.StoreInt64(&pm.minLatency, 1<<63-1)
	atomic.StoreInt64(&pm.maxLatency, 0)
	atomic.StoreInt64(&pm.providerCacheHits, 0)
	atomic.StoreInt64(&pm.providerCacheMisses, 0)
	atomic.StoreInt64(&pm.tokensProcessed, 0)
	atomic.StoreInt64(&pm.gcPauses, 0)

	pm.latencyMu.Lock()
	pm.latencies = pm.latencies[:0]
	pm.latencyMu.Unlock()

	pm.lastReset = timeutil.NowTime()
}
