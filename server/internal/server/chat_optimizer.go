package server

import (
	"sync"
	"sync/atomic"
	"time"
)

// ChatCallChainOptimizer optimizes the Chat → Proxy Handler call chain
type ChatCallChainOptimizer struct {
	// Request deduplication
	deduplicator *RequestDeduplicator

	// Usage tracking
	usageTracker *UsageTracker

	// Performance metrics
	metrics *PerformanceMetrics

	// Adaptive cache
	adaptiveCache *AdaptiveCacheManager

	// Connection pooling
	connPool *ConnectionPool

	// Cache prefetcher
	prefetcher *CachePrefetcher

	// Request batching
	batchProcessor *BatchProcessor

	// Metrics
	optimizedRequests int64
	cachedResponses   int64
	dedupedRequests   int64

	mu sync.RWMutex
}

// NewChatCallChainOptimizer creates a new optimizer
func NewChatCallChainOptimizer() *ChatCallChainOptimizer {
	return &ChatCallChainOptimizer{
		deduplicator:   NewRequestDeduplicator(),
		usageTracker:   NewUsageTracker(),
		metrics:        NewPerformanceMetrics(),
		adaptiveCache:  NewAdaptiveCacheManager(100, 10, 500),
		connPool:       NewConnectionPool(100, 10),
		prefetcher:     NewCachePrefetcher(1000, 5, 4),
	}
}

// OptimizeRequest optimizes a chat request through the call chain
func (cco *ChatCallChainOptimizer) OptimizeRequest(provider, model string, fn func() (interface{}, error)) (interface{}, error, bool) {
	startTime := time.Now()

	// Try deduplication first
	dedupKey := provider + ":" + model
	result, err, wasDeduped := cco.deduplicator.Do(dedupKey, fn)

	duration := time.Now().Sub(startTime).Milliseconds()
	success := err == nil

	// Track usage
	cco.usageTracker.RecordRequest(provider, model, success, duration, 0, 0)

	// Track metrics
	metric := &RequestMetric{
		StartTime:  startTime,
		EndTime:    time.Now(),
		Provider:   provider,
		Model:      model,
		CacheHit:   wasDeduped,
		Success:    success,
	}
	cco.metrics.RecordRequest(metric)

	if wasDeduped {
		atomic.AddInt64(&cco.dedupedRequests, 1)
	}
	atomic.AddInt64(&cco.optimizedRequests, 1)

	return result, err, wasDeduped
}

// GetUsageStats returns usage statistics
func (cco *ChatCallChainOptimizer) GetUsageStats(provider, model string) *UsageStats {
	return cco.usageTracker.GetStats(provider, model)
}

// GetAllUsageStats returns all usage statistics
func (cco *ChatCallChainOptimizer) GetAllUsageStats() []*UsageStats {
	return cco.usageTracker.GetAllStats()
}

// GetProviderStats returns provider-level statistics
func (cco *ChatCallChainOptimizer) GetProviderStats(provider string) map[string]interface{} {
	return cco.usageTracker.GetProviderStats(provider)
}

// GetMetrics returns performance metrics
func (cco *ChatCallChainOptimizer) GetMetrics() map[string]interface{} {
	return cco.metrics.GetMetrics()
}

// GetOptimizationStats returns optimization statistics
func (cco *ChatCallChainOptimizer) GetOptimizationStats() map[string]interface{} {
	return map[string]interface{}{
		"optimized_requests": atomic.LoadInt64(&cco.optimizedRequests),
		"deduped_requests":   atomic.LoadInt64(&cco.dedupedRequests),
		"dedup_rate":         float64(atomic.LoadInt64(&cco.dedupedRequests)) / float64(atomic.LoadInt64(&cco.optimizedRequests)) * 100,
		"memory_pressure":    cco.adaptiveCache.GetMemoryPressure(),
		"cache_size":         cco.adaptiveCache.GetCacheSize(),
	}
}

// Close closes the optimizer and releases resources
func (cco *ChatCallChainOptimizer) Close() {
	cco.prefetcher.Stop()
	cco.adaptiveCache.Stop()
	cco.connPool.Close()
}
