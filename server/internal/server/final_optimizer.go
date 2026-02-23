package server

import (
	"sync"
	"sync/atomic"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// StreamingOptimizer optimizes real-time streaming responses
type StreamingOptimizer struct {
	bufferSize    int
	flushInterval time.Duration
	metrics       map[string]*StreamMetrics
	mu            sync.RWMutex
}

// StreamMetrics tracks streaming metrics
type StreamMetrics struct {
	ChunksProcessed int64
	BytesSent       int64
	AvgChunkSize    int64
	FlushCount      int64
	LastFlushTime   time.Time
}

// NewStreamingOptimizer creates a new streaming optimizer
func NewStreamingOptimizer(bufferSize int, flushInterval time.Duration) *StreamingOptimizer {
	return &StreamingOptimizer{
		bufferSize:    bufferSize,
		flushInterval: flushInterval,
		metrics:       make(map[string]*StreamMetrics),
	}
}

// RecordChunk records a streamed chunk
func (so *StreamingOptimizer) RecordChunk(streamID string, size int64) {
	so.mu.Lock()
	defer so.mu.Unlock()

	metrics, exists := so.metrics[streamID]
	if !exists {
		metrics = &StreamMetrics{}
		so.metrics[streamID] = metrics
	}

	atomic.AddInt64(&metrics.ChunksProcessed, 1)
	atomic.AddInt64(&metrics.BytesSent, size)
	metrics.LastFlushTime = timeutil.NowTime()
}

// LoadBalancer implements dynamic load balancing across providers
type LoadBalancer struct {
	providers map[string]*ProviderLoad
	mu        sync.RWMutex
}

// ProviderLoad tracks provider load
type ProviderLoad struct {
	Name            string
	ActiveRequests  int64
	TotalRequests   int64
	AvgResponseTime int64
	ErrorRate       float64
	Weight          float64
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer() *LoadBalancer {
	return &LoadBalancer{
		providers: make(map[string]*ProviderLoad),
	}
}

// RegisterProvider registers a provider
func (lb *LoadBalancer) RegisterProvider(name string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.providers[name] = &ProviderLoad{
		Name:   name,
		Weight: 1.0,
	}
}

// SelectProvider selects best provider based on load
func (lb *LoadBalancer) SelectProvider() string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	var best string
	var bestScore float64 = -1

	for name, load := range lb.providers {
		// Score = weight / (active_requests + 1) * (1 - error_rate)
		score := load.Weight / float64(atomic.LoadInt64(&load.ActiveRequests)+1) * (1 - load.ErrorRate)
		if score > bestScore {
			bestScore = score
			best = name
		}
	}

	return best
}

// RecordRequest records a request to provider
func (lb *LoadBalancer) RecordRequest(provider string, success bool, responseTime int64) {
	lb.mu.Lock()
	load, exists := lb.providers[provider]
	lb.mu.Unlock()

	if !exists {
		return
	}

	atomic.AddInt64(&load.TotalRequests, 1)
	atomic.AddInt64(&load.AvgResponseTime, responseTime)

	if !success {
		total := atomic.LoadInt64(&load.TotalRequests)
		errors := int64(float64(total) * load.ErrorRate)
		load.ErrorRate = float64(errors+1) / float64(total)
	}
}

// AcquireProvider acquires a provider (increments active count)
func (lb *LoadBalancer) AcquireProvider(provider string) {
	lb.mu.RLock()
	load, exists := lb.providers[provider]
	lb.mu.RUnlock()

	if exists {
		atomic.AddInt64(&load.ActiveRequests, 1)
	}
}

// ReleaseProvider releases a provider (decrements active count)
func (lb *LoadBalancer) ReleaseProvider(provider string) {
	lb.mu.RLock()
	load, exists := lb.providers[provider]
	lb.mu.RUnlock()

	if exists {
		atomic.AddInt64(&load.ActiveRequests, -1)
	}
}

// GetProviderStats returns provider statistics
func (lb *LoadBalancer) GetProviderStats(provider string) map[string]interface{} {
	lb.mu.RLock()
	load, exists := lb.providers[provider]
	lb.mu.RUnlock()

	if !exists {
		return nil
	}

	return map[string]interface{}{
		"name":             load.Name,
		"active_requests":  atomic.LoadInt64(&load.ActiveRequests),
		"total_requests":   atomic.LoadInt64(&load.TotalRequests),
		"avg_response_ms":  atomic.LoadInt64(&load.AvgResponseTime),
		"error_rate":       load.ErrorRate,
		"weight":           load.Weight,
	}
}

// DynamicThrottler implements dynamic request throttling
type DynamicThrottler struct {
	maxConcurrent int64
	current       int64
	queue         chan struct{}
	mu            sync.RWMutex
}

// NewDynamicThrottler creates a new dynamic throttler
func NewDynamicThrottler(maxConcurrent int64) *DynamicThrottler {
	return &DynamicThrottler{
		maxConcurrent: maxConcurrent,
		queue:         make(chan struct{}, maxConcurrent),
	}
}

// Acquire acquires a slot
func (dt *DynamicThrottler) Acquire() {
	dt.queue <- struct{}{}
	atomic.AddInt64(&dt.current, 1)
}

// Release releases a slot
func (dt *DynamicThrottler) Release() {
	<-dt.queue
	atomic.AddInt64(&dt.current, -1)
}

// GetUtilization returns current utilization
func (dt *DynamicThrottler) GetUtilization() float64 {
	current := atomic.LoadInt64(&dt.current)
	return float64(current) / float64(dt.maxConcurrent)
}

// AdjustLimit adjusts max concurrent limit
func (dt *DynamicThrottler) AdjustLimit(newLimit int64) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if newLimit > dt.maxConcurrent {
		// Increase limit
		for i := int64(0); i < newLimit-dt.maxConcurrent; i++ {
			dt.queue <- struct{}{}
		}
	} else if newLimit < dt.maxConcurrent {
		// Decrease limit
		for i := int64(0); i < dt.maxConcurrent-newLimit; i++ {
			<-dt.queue
		}
	}
	dt.maxConcurrent = newLimit
}
