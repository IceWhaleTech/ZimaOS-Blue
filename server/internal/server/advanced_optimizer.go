package server

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// RequestPipeline implements request pipelining for concurrent processing
type RequestPipeline struct {
	stages    []PipelineStage
	workers   int
	queue     chan *PipelineRequest
	stopChan  chan struct{}
	wg        sync.WaitGroup
	processed int64
}

// PipelineStage represents a processing stage
type PipelineStage interface {
	Process(ctx context.Context, req *PipelineRequest) error
	Name() string
}

// PipelineRequest represents a request in the pipeline
type PipelineRequest struct {
	ID       string
	Provider string
	Model    string
	Data     interface{}
	Result   interface{}
	Error    error
	Context  context.Context
}

// NewRequestPipeline creates a new request pipeline
func NewRequestPipeline(workers int, stages ...PipelineStage) *RequestPipeline {
	rp := &RequestPipeline{
		stages:   stages,
		workers:  workers,
		queue:    make(chan *PipelineRequest, workers*2),
		stopChan: make(chan struct{}),
	}

	for i := 0; i < workers; i++ {
		rp.wg.Add(1)
		go rp.worker()
	}

	return rp
}

// Submit submits a request to the pipeline
func (rp *RequestPipeline) Submit(req *PipelineRequest) error {
	select {
	case rp.queue <- req:
		return nil
	case <-rp.stopChan:
		return ErrProcessorStopped
	}
}

// worker processes requests through all stages
func (rp *RequestPipeline) worker() {
	defer rp.wg.Done()

	for {
		select {
		case req := <-rp.queue:
			if req == nil {
				return
			}

			// Process through all stages
			for _, stage := range rp.stages {
				if err := stage.Process(req.Context, req); err != nil {
					req.Error = err
					break
				}
			}

			atomic.AddInt64(&rp.processed, 1)

		case <-rp.stopChan:
			return
		}
	}
}

// Stop stops the pipeline
func (rp *RequestPipeline) Stop() {
	close(rp.stopChan)
	rp.wg.Wait()
}

// GetProcessed returns number of processed requests
func (rp *RequestPipeline) GetProcessed() int64 {
	return atomic.LoadInt64(&rp.processed)
}

// SmartCacheStrategy implements intelligent cache strategy
type SmartCacheStrategy struct {
	hitThreshold   float64
	missThreshold  float64
	adaptiveSize   bool
	prefetchRatio  float64
	stats          map[string]*CacheStrategyStats
	mu             sync.RWMutex
}

// CacheStrategyStats tracks cache strategy statistics
type CacheStrategyStats struct {
	Hits       int64
	Misses     int64
	Prefetches int64
	LastUpdate time.Time
}

// NewSmartCacheStrategy creates a new smart cache strategy
func NewSmartCacheStrategy() *SmartCacheStrategy {
	return &SmartCacheStrategy{
		hitThreshold:  0.7,
		missThreshold: 0.3,
		adaptiveSize:  true,
		prefetchRatio: 0.2,
		stats:         make(map[string]*CacheStrategyStats),
	}
}

// ShouldPrefetch determines if a key should be prefetched
func (scs *SmartCacheStrategy) ShouldPrefetch(key string) bool {
	scs.mu.RLock()
	defer scs.mu.RUnlock()

	stats, exists := scs.stats[key]
	if !exists {
		return false
	}

	total := stats.Hits + stats.Misses
	if total == 0 {
		return false
	}

	hitRate := float64(stats.Hits) / float64(total)
	return hitRate > scs.hitThreshold
}

// RecordAccess records cache access
func (scs *SmartCacheStrategy) RecordAccess(key string, hit bool) {
	scs.mu.Lock()
	defer scs.mu.Unlock()

	stats, exists := scs.stats[key]
	if !exists {
		stats = &CacheStrategyStats{}
		scs.stats[key] = stats
	}

	if hit {
		atomic.AddInt64(&stats.Hits, 1)
	} else {
		atomic.AddInt64(&stats.Misses, 1)
	}
	stats.LastUpdate = timeutil.NowTime()
}

// GetStrategy returns cache strategy for a key
func (scs *SmartCacheStrategy) GetStrategy(key string) map[string]interface{} {
	scs.mu.RLock()
	defer scs.mu.RUnlock()

	stats, exists := scs.stats[key]
	if !exists {
		return map[string]interface{}{
			"should_cache":   true,
			"should_prefetch": false,
			"priority":       "normal",
		}
	}

	total := stats.Hits + stats.Misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(stats.Hits) / float64(total)
	}

	priority := "low"
	if hitRate > 0.8 {
		priority = "high"
	} else if hitRate > 0.5 {
		priority = "medium"
	}

	return map[string]interface{}{
		"should_cache":    true,
		"should_prefetch": hitRate > scs.hitThreshold,
		"priority":        priority,
		"hit_rate":        hitRate,
		"total_accesses":  total,
	}
}

// CircuitBreaker implements circuit breaker pattern for provider failover
type CircuitBreaker struct {
	provider      string
	failureCount  int64
	successCount  int64
	lastFailTime  time.Time
	state         string // "closed", "open", "half-open"
	threshold     int64
	timeout       time.Duration
	mu            sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(provider string, threshold int64, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		provider:  provider,
		state:     "closed",
		threshold: threshold,
		timeout:   timeout,
	}
}

// RecordSuccess records a successful call
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.AddInt64(&cb.successCount, 1)
	if cb.state == "half-open" {
		cb.state = "closed"
		atomic.StoreInt64(&cb.failureCount, 0)
	}
}

// RecordFailure records a failed call
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.AddInt64(&cb.failureCount, 1)
	cb.lastFailTime = timeutil.NowTime()

	if atomic.LoadInt64(&cb.failureCount) >= cb.threshold {
		cb.state = "open"
	}
}

// CanExecute checks if a call can be executed
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == "closed" {
		return true
	}

	if cb.state == "open" {
		if timeutil.SinceTime(cb.lastFailTime) > cb.timeout {
			cb.state = "half-open"
			return true
		}
		return false
	}

	return cb.state == "half-open"
}

// GetState returns circuit breaker state
func (cb *CircuitBreaker) GetState() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"provider":      cb.provider,
		"state":         cb.state,
		"failure_count": atomic.LoadInt64(&cb.failureCount),
		"success_count": atomic.LoadInt64(&cb.successCount),
		"last_fail":     cb.lastFailTime,
	}
}
