package proxy

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CacheSingleflight prevents cache stampede by deduplicating concurrent
// requests for the same cache key. Only one goroutine calls upstream;
// others wait for the result.
type CacheSingleflight struct {
	mu    sync.Mutex
	calls map[string]*sfCall
}

type sfCall struct {
	wg  sync.WaitGroup
	val *CCCacheEntry
	err error
}

// NewCacheSingleflight creates a new singleflight instance.
func NewCacheSingleflight() *CacheSingleflight {
	return &CacheSingleflight{
		calls: make(map[string]*sfCall),
	}
}

// Do executes fn once for a given key. Concurrent callers with the same key
// block until the first caller completes, then all receive the same result.
// timeout controls the maximum wait time (0 = no timeout).
func (sf *CacheSingleflight) Do(ctx context.Context, key string, timeout time.Duration, fn func() (*CCCacheEntry, error)) (*CCCacheEntry, error) {
	sf.mu.Lock()
	if call, ok := sf.calls[key]; ok {
		sf.mu.Unlock()
		// Wait for in-flight request with timeout
		return sf.waitForCall(ctx, call, timeout)
	}

	call := &sfCall{}
	call.wg.Add(1)
	sf.calls[key] = call
	sf.mu.Unlock()

	// Execute the function
	call.val, call.err = fn()
	call.wg.Done()

	// Cleanup
	sf.mu.Lock()
	delete(sf.calls, key)
	sf.mu.Unlock()

	return call.val, call.err
}

func (sf *CacheSingleflight) waitForCall(ctx context.Context, call *sfCall, timeout time.Duration) (*CCCacheEntry, error) {
	done := make(chan struct{})
	go func() {
		call.wg.Wait()
		close(done)
	}()

	var timer <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timer = t.C
	}

	select {
	case <-done:
		return call.val, call.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer:
		return nil, fmt.Errorf("singleflight: timeout waiting for key")
	}
}
