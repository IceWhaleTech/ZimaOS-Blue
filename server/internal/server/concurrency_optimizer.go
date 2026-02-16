package server

import (
	"sync"
	"sync/atomic"
)

// ConcurrencyOptimizer provides optimizations for concurrent request handling
type ConcurrencyOptimizer struct {
	// Request queue for batching
	requestQueue chan interface{}
	queueSize    int
	batchSize    int
	batchTimeout int64 // milliseconds

	// Metrics
	totalRequests   int64
	cachedRequests  int64
	batchedRequests int64

	// Lazy init
	initOnce sync.Once
}

// NewConcurrencyOptimizer creates a new concurrency optimizer.
// The request queue channel is lazily allocated on first use.
func NewConcurrencyOptimizer(queueSize, batchSize int) *ConcurrencyOptimizer {
	return &ConcurrencyOptimizer{
		queueSize:    queueSize,
		batchSize:    batchSize,
		batchTimeout: 100, // 100ms
	}
}

func (co *ConcurrencyOptimizer) ensureInit() {
	co.initOnce.Do(func() {
		co.requestQueue = make(chan interface{}, co.queueSize)
	})
}

// RecordRequest records a request for metrics
func (co *ConcurrencyOptimizer) RecordRequest(cached bool) {
	atomic.AddInt64(&co.totalRequests, 1)
	if cached {
		atomic.AddInt64(&co.cachedRequests, 1)
	}
}

// GetMetrics returns current metrics
func (co *ConcurrencyOptimizer) GetMetrics() map[string]int64 {
	return map[string]int64{
		"total_requests":   atomic.LoadInt64(&co.totalRequests),
		"cached_requests":  atomic.LoadInt64(&co.cachedRequests),
		"batched_requests": atomic.LoadInt64(&co.batchedRequests),
	}
}

// FastPathCache provides ultra-fast caching for frequently accessed data
type FastPathCache struct {
	data sync.Map // Lock-free map for high concurrency
	hits int64
	miss int64
}

// NewFastPathCache creates a new fast path cache
func NewFastPathCache() *FastPathCache {
	return &FastPathCache{}
}

// Get retrieves a value from cache
func (fpc *FastPathCache) Get(key string) (interface{}, bool) {
	val, ok := fpc.data.Load(key)
	if ok {
		atomic.AddInt64(&fpc.hits, 1)
	} else {
		atomic.AddInt64(&fpc.miss, 1)
	}
	return val, ok
}

// Set stores a value in cache
func (fpc *FastPathCache) Set(key string, value interface{}) {
	fpc.data.Store(key, value)
}

// Delete removes a value from cache
func (fpc *FastPathCache) Delete(key string) {
	fpc.data.Delete(key)
}

// GetStats returns cache statistics
func (fpc *FastPathCache) GetStats() map[string]int64 {
	hits := atomic.LoadInt64(&fpc.hits)
	miss := atomic.LoadInt64(&fpc.miss)
	total := hits + miss
	hitRate := int64(0)
	if total > 0 {
		hitRate = (hits * 100) / total
	}
	return map[string]int64{
		"hits":     hits,
		"miss":     miss,
		"total":    total,
		"hit_rate": hitRate,
	}
}

// RequestBatcher batches multiple requests for efficient processing
type RequestBatcher struct {
	batch      []interface{}
	batchMu    sync.Mutex
	batchSize  int
	flushChan  chan struct{}
	resultChan chan []interface{}
}

// NewRequestBatcher creates a new request batcher
func NewRequestBatcher(batchSize int) *RequestBatcher {
	return &RequestBatcher{
		batch:      make([]interface{}, 0, batchSize),
		batchSize:  batchSize,
		flushChan:  make(chan struct{}, 1),
		resultChan: make(chan []interface{}, 1),
	}
}

// Add adds a request to the batch
func (rb *RequestBatcher) Add(req interface{}) bool {
	rb.batchMu.Lock()
	defer rb.batchMu.Unlock()

	rb.batch = append(rb.batch, req)
	if len(rb.batch) >= rb.batchSize {
		return true // Batch is full
	}
	return false
}

// Flush flushes the current batch
func (rb *RequestBatcher) Flush() []interface{} {
	rb.batchMu.Lock()
	defer rb.batchMu.Unlock()

	if len(rb.batch) == 0 {
		return nil
	}

	result := make([]interface{}, len(rb.batch))
	copy(result, rb.batch)
	rb.batch = rb.batch[:0]
	return result
}

// GetBatchSize returns current batch size
func (rb *RequestBatcher) GetBatchSize() int {
	rb.batchMu.Lock()
	defer rb.batchMu.Unlock()
	return len(rb.batch)
}
