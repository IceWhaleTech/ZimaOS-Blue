package server

import (
	"sync"
	"sync/atomic"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// RequestCoalescing combines multiple identical concurrent requests into one
type RequestCoalescing struct {
	pending map[string]*CoalescedRequest
	mu      sync.RWMutex
	merged  int64
}

// CoalescedRequest represents a coalesced request
type CoalescedRequest struct {
	result    interface{}
	err       error
	done      chan struct{}
	waiters   int
}

// NewRequestCoalescing creates a new request coalescing
func NewRequestCoalescing() *RequestCoalescing {
	return &RequestCoalescing{
		pending: make(map[string]*CoalescedRequest),
	}
}

// Do executes a function, coalescing identical concurrent requests
func (rc *RequestCoalescing) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	rc.mu.Lock()

	if pending, exists := rc.pending[key]; exists {
		pending.waiters++
		rc.mu.Unlock()
		<-pending.done
		atomic.AddInt64(&rc.merged, 1)
		return pending.result, pending.err
	}

	pending := &CoalescedRequest{
		done: make(chan struct{}),
	}
	rc.pending[key] = pending
	rc.mu.Unlock()

	pending.result, pending.err = fn()
	close(pending.done)

	rc.mu.Lock()
	delete(rc.pending, key)
	rc.mu.Unlock()

	return pending.result, pending.err
}

// GetMerged returns number of merged requests
func (rc *RequestCoalescing) GetMerged() int64 {
	return atomic.LoadInt64(&rc.merged)
}

// LatencyOptimizer optimizes latency through predictive loading
type LatencyOptimizer struct {
	predictions map[string]*LatencyPrediction
	mu          sync.RWMutex
}

// LatencyPrediction stores latency predictions
type LatencyPrediction struct {
	Provider      string
	Model         string
	AvgLatency    int64
	P95Latency    int64
	P99Latency    int64
	LastUpdated   time.Time
	SampleCount   int64
}

// NewLatencyOptimizer creates a new latency optimizer
func NewLatencyOptimizer() *LatencyOptimizer {
	return &LatencyOptimizer{
		predictions: make(map[string]*LatencyPrediction),
	}
}

// RecordLatency records latency for prediction
func (lo *LatencyOptimizer) RecordLatency(provider, model string, latency int64) {
	key := provider + ":" + model
	lo.mu.Lock()
	defer lo.mu.Unlock()

	pred, exists := lo.predictions[key]
	if !exists {
		pred = &LatencyPrediction{
			Provider: provider,
			Model:    model,
		}
		lo.predictions[key] = pred
	}

	// Simple exponential moving average
	if pred.SampleCount == 0 {
		pred.AvgLatency = latency
	} else {
		pred.AvgLatency = (pred.AvgLatency*9 + latency) / 10
	}

	pred.SampleCount++
	pred.LastUpdated = timeutil.NowTime()
}

// GetPrediction gets latency prediction
func (lo *LatencyOptimizer) GetPrediction(provider, model string) *LatencyPrediction {
	key := provider + ":" + model
	lo.mu.RLock()
	defer lo.mu.RUnlock()

	return lo.predictions[key]
}

// ResourcePoolManager manages resource pools for efficiency
type ResourcePoolManager struct {
	pools map[string]*ResourcePool
	mu    sync.RWMutex
}

// ResourcePool represents a resource pool
type ResourcePool struct {
	name      string
	available chan interface{}
	total     int
	inUse     int64
}

// NewResourcePoolManager creates a new resource pool manager
func NewResourcePoolManager() *ResourcePoolManager {
	return &ResourcePoolManager{
		pools: make(map[string]*ResourcePool),
	}
}

// CreatePool creates a new resource pool
func (rpm *ResourcePoolManager) CreatePool(name string, size int, factory func() interface{}) {
	rpm.mu.Lock()
	defer rpm.mu.Unlock()

	pool := &ResourcePool{
		name:      name,
		available: make(chan interface{}, size),
		total:     size,
	}

	for i := 0; i < size; i++ {
		pool.available <- factory()
	}

	rpm.pools[name] = pool
}

// Acquire acquires a resource from pool
func (rpm *ResourcePoolManager) Acquire(name string) (interface{}, error) {
	rpm.mu.RLock()
	pool, exists := rpm.pools[name]
	rpm.mu.RUnlock()

	if !exists {
		return nil, ErrPoolNotFound
	}

	resource := <-pool.available
	atomic.AddInt64(&pool.inUse, 1)
	return resource, nil
}

// Release releases a resource back to pool
func (rpm *ResourcePoolManager) Release(name string, resource interface{}) error {
	rpm.mu.RLock()
	pool, exists := rpm.pools[name]
	rpm.mu.RUnlock()

	if !exists {
		return ErrPoolNotFound
	}

	pool.available <- resource
	atomic.AddInt64(&pool.inUse, -1)
	return nil
}

// GetPoolStats returns pool statistics
func (rpm *ResourcePoolManager) GetPoolStats(name string) map[string]interface{} {
	rpm.mu.RLock()
	pool, exists := rpm.pools[name]
	rpm.mu.RUnlock()

	if !exists {
		return nil
	}

	return map[string]interface{}{
		"name":      pool.name,
		"total":     pool.total,
		"in_use":    atomic.LoadInt64(&pool.inUse),
		"available": len(pool.available),
	}
}

// Error definitions
var (
	ErrPoolNotFound = &PoolError{Code: "POOL_NOT_FOUND", Message: "resource pool not found"}
)

// PoolError represents a pool error
type PoolError struct {
	Code    string
	Message string
}

func (e *PoolError) Error() string {
	return e.Message
}
