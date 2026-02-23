package server

import (
	"sync"
	"sync/atomic"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ChatCallChainOptimizer optimizes the Chat → Proxy Handler call chain
type ChatCallChainOptimizer struct {
	// Request deduplication
	deduplicator *RequestDeduplicator

	// Usage tracking
	usageTracker *UsageTracker

	// Performance metrics
	metrics *PerformanceMetrics

	// Connection pooling
	connPool *ConnectionPool

	// Request batching
	batchProcessor *BatchProcessor

	// Metrics
	optimizedRequests int64
	dedupedRequests   int64

	mu sync.RWMutex
}

// NewChatCallChainOptimizer creates a new optimizer
func NewChatCallChainOptimizer() *ChatCallChainOptimizer {
	return &ChatCallChainOptimizer{
		deduplicator:   NewRequestDeduplicator(),
		usageTracker:   NewUsageTracker(),
		metrics:        NewPerformanceMetrics(),
		connPool:       NewConnectionPool(100, 10),
	}
}

// OptimizeRequest optimizes a chat request through the call chain
func (cco *ChatCallChainOptimizer) OptimizeRequest(provider, model string, fn func() (interface{}, error)) (interface{}, error, bool) {
	startTime := timeutil.NowTime()

	// Try deduplication first
	dedupKey := provider + ":" + model
	result, err, wasDeduped := cco.deduplicator.Do(dedupKey, fn)

	duration := timeutil.SinceTime(startTime).Milliseconds()
	success := err == nil

	// Track usage
	cco.usageTracker.RecordRequest(provider, model, success, duration, 0, 0)

	// Track metrics
	metric := &RequestMetric{
		StartTime:  startTime,
		EndTime:    timeutil.NowTime(),
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
	optimized := atomic.LoadInt64(&cco.optimizedRequests)
	deduped := atomic.LoadInt64(&cco.dedupedRequests)
	var dedupRate float64
	if optimized > 0 {
		dedupRate = float64(deduped) / float64(optimized) * 100
	}
	return map[string]interface{}{
		"optimized_requests": optimized,
		"deduped_requests":   deduped,
		"dedup_rate":         dedupRate,
	}
}

// Close closes the optimizer and releases resources
func (cco *ChatCallChainOptimizer) Close() {
	cco.connPool.Close()
}
