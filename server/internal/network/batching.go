package network

import (
	"context"
	"sync"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// BatchConfig holds configuration for request batching.
type BatchConfig struct {
	// MaxBatchSize is the maximum number of requests in a batch
	MaxBatchSize int

	// MaxWaitTime is the maximum time to wait for a batch to fill
	MaxWaitTime time.Duration

	// MaxConcurrentBatches is the maximum number of concurrent batch executions
	MaxConcurrentBatches int
}

// DefaultBatchConfig returns default batch configuration.
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		MaxBatchSize:         100,
		MaxWaitTime:          10 * time.Millisecond,
		MaxConcurrentBatches: 10,
	}
}

// BatchRequest represents a single request in a batch.
type BatchRequest[K comparable, V any] struct {
	Key    K
	Result chan BatchResult[V]
}

// BatchResult holds the result of a batch request.
type BatchResult[V any] struct {
	Value V
	Error error
}

// BatchLoader is a function that loads multiple keys at once.
type BatchLoader[K comparable, V any] func(ctx context.Context, keys []K) (map[K]V, error)

// Batcher batches multiple requests into a single batch operation.
type Batcher[K comparable, V any] struct {
	config   BatchConfig
	loader   BatchLoader[K, V]
	requests chan BatchRequest[K, V]
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	sem      chan struct{}
}

// NewBatcher creates a new batcher.
func NewBatcher[K comparable, V any](ctx context.Context, config BatchConfig, loader BatchLoader[K, V]) *Batcher[K, V] {
	ctx, cancel := context.WithCancel(ctx)

	b := &Batcher[K, V]{
		config:   config,
		loader:   loader,
		requests: make(chan BatchRequest[K, V], config.MaxBatchSize*config.MaxConcurrentBatches),
		ctx:      ctx,
		cancel:   cancel,
		sem:      make(chan struct{}, config.MaxConcurrentBatches),
	}

	b.wg.Add(1)
	go b.run()

	return b
}

// Load loads a single key, batching it with other concurrent requests.
func (b *Batcher[K, V]) Load(ctx context.Context, key K) (V, error) {
	result := make(chan BatchResult[V], 1)

	select {
	case b.requests <- BatchRequest[K, V]{Key: key, Result: result}:
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	case <-b.ctx.Done():
		var zero V
		return zero, b.ctx.Err()
	}

	select {
	case r := <-result:
		return r.Value, r.Error
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	case <-b.ctx.Done():
		var zero V
		return zero, b.ctx.Err()
	}
}

// LoadMany loads multiple keys, batching them together.
func (b *Batcher[K, V]) LoadMany(ctx context.Context, keys []K) (map[K]V, error) {
	results := make(map[K]V)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for _, key := range keys {
		wg.Add(1)
		go func(k K) {
			defer wg.Done()
			v, err := b.Load(ctx, k)
			if err != nil {
				errOnce.Do(func() { firstErr = err })
				return
			}
			mu.Lock()
			results[k] = v
			mu.Unlock()
		}(key)
	}

	wg.Wait()
	return results, firstErr
}

func (b *Batcher[K, V]) run() {
	defer b.wg.Done()

	for {
		select {
		case <-b.ctx.Done():
			return
		case req := <-b.requests:
			b.collectAndExecute(req)
		}
	}
}

func (b *Batcher[K, V]) collectAndExecute(first BatchRequest[K, V]) {
	batch := []BatchRequest[K, V]{first}
	timer := time.NewTimer(b.config.MaxWaitTime)
	defer timer.Stop()

	// Collect more requests
	for len(batch) < b.config.MaxBatchSize {
		select {
		case req := <-b.requests:
			batch = append(batch, req)
		case <-timer.C:
			goto execute
		case <-b.ctx.Done():
			// Send error to all pending requests
			for _, req := range batch {
				req.Result <- BatchResult[V]{Error: b.ctx.Err()}
			}
			return
		}
	}

execute:
	// Acquire semaphore
	select {
	case b.sem <- struct{}{}:
	case <-b.ctx.Done():
		for _, req := range batch {
			req.Result <- BatchResult[V]{Error: b.ctx.Err()}
		}
		return
	}

	// Execute batch in goroutine
	go func() {
		defer func() { <-b.sem }()
		b.executeBatch(batch)
	}()
}

func (b *Batcher[K, V]) executeBatch(batch []BatchRequest[K, V]) {
	// Collect unique keys
	keys := make([]K, 0, len(batch))
	seen := make(map[K]bool)
	for _, req := range batch {
		if !seen[req.Key] {
			seen[req.Key] = true
			keys = append(keys, req.Key)
		}
	}

	// Execute loader
	results, err := b.loader(b.ctx, keys)

	// Send results
	for _, req := range batch {
		if err != nil {
			req.Result <- BatchResult[V]{Error: err}
		} else if v, ok := results[req.Key]; ok {
			req.Result <- BatchResult[V]{Value: v}
		} else {
			var zero V
			req.Result <- BatchResult[V]{Value: zero}
		}
	}
}

// Close shuts down the batcher.
func (b *Batcher[K, V]) Close() {
	b.cancel()
	b.wg.Wait()
}

// SingleFlightGroup prevents duplicate function calls for the same key.
type SingleFlightGroup[K comparable, V any] struct {
	mu    sync.Mutex
	calls map[K]*singleFlightCall[V]
}

type singleFlightCall[V any] struct {
	wg     sync.WaitGroup
	result V
	err    error
}

// NewSingleFlightGroup creates a new single flight group.
func NewSingleFlightGroup[K comparable, V any]() *SingleFlightGroup[K, V] {
	return &SingleFlightGroup[K, V]{
		calls: make(map[K]*singleFlightCall[V]),
	}
}

// Do executes the function for the given key, deduplicating concurrent calls.
func (g *SingleFlightGroup[K, V]) Do(key K, fn func() (V, error)) (V, error) {
	g.mu.Lock()

	if call, ok := g.calls[key]; ok {
		g.mu.Unlock()
		call.wg.Wait()
		return call.result, call.err
	}

	call := &singleFlightCall[V]{}
	call.wg.Add(1)
	g.calls[key] = call
	g.mu.Unlock()

	call.result, call.err = fn()
	call.wg.Done()

	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()

	return call.result, call.err
}

// DoWithContext executes the function with context support.
func (g *SingleFlightGroup[K, V]) DoWithContext(ctx context.Context, key K, fn func(context.Context) (V, error)) (V, error) {
	g.mu.Lock()

	if call, ok := g.calls[key]; ok {
		g.mu.Unlock()
		call.wg.Wait()
		return call.result, call.err
	}

	call := &singleFlightCall[V]{}
	call.wg.Add(1)
	g.calls[key] = call
	g.mu.Unlock()

	call.result, call.err = fn(ctx)
	call.wg.Done()

	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()

	return call.result, call.err
}

// Forget removes a key from the group, allowing a new call to be made.
func (g *SingleFlightGroup[K, V]) Forget(key K) {
	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
}

// RequestCoalescer coalesces multiple identical requests into one.
type RequestCoalescer[K comparable, V any] struct {
	group   *SingleFlightGroup[K, V]
	loader  func(context.Context, K) (V, error)
	ttl     time.Duration
	cache   map[K]*coalescedEntry[V]
	cacheMu sync.RWMutex
}

type coalescedEntry[V any] struct {
	value     V
	err       error
	expiresAt time.Time
}

// NewRequestCoalescer creates a new request coalescer.
func NewRequestCoalescer[K comparable, V any](loader func(context.Context, K) (V, error), ttl time.Duration) *RequestCoalescer[K, V] {
	return &RequestCoalescer[K, V]{
		group:  NewSingleFlightGroup[K, V](),
		loader: loader,
		ttl:    ttl,
		cache:  make(map[K]*coalescedEntry[V]),
	}
}

// Load loads a value, coalescing concurrent requests and caching results.
func (c *RequestCoalescer[K, V]) Load(ctx context.Context, key K) (V, error) {
	// Check cache first
	c.cacheMu.RLock()
	if entry, ok := c.cache[key]; ok && timeutil.NowTime().Before(entry.expiresAt) {
		c.cacheMu.RUnlock()
		return entry.value, entry.err
	}
	c.cacheMu.RUnlock()

	// Use single flight to deduplicate
	return c.group.DoWithContext(ctx, key, func(ctx context.Context) (V, error) {
		// Double-check cache
		c.cacheMu.RLock()
		if entry, ok := c.cache[key]; ok && timeutil.NowTime().Before(entry.expiresAt) {
			c.cacheMu.RUnlock()
			return entry.value, entry.err
		}
		c.cacheMu.RUnlock()

		// Load value
		value, err := c.loader(ctx, key)

		// Cache result
		c.cacheMu.Lock()
		c.cache[key] = &coalescedEntry[V]{
			value:     value,
			err:       err,
			expiresAt: timeutil.NowTime().Add(c.ttl),
		}
		c.cacheMu.Unlock()

		return value, err
	})
}

// Invalidate removes a key from the cache.
func (c *RequestCoalescer[K, V]) Invalidate(key K) {
	c.cacheMu.Lock()
	delete(c.cache, key)
	c.cacheMu.Unlock()
	c.group.Forget(key)
}

// Clear clears the entire cache.
func (c *RequestCoalescer[K, V]) Clear() {
	c.cacheMu.Lock()
	c.cache = make(map[K]*coalescedEntry[V])
	c.cacheMu.Unlock()
}
