package proxy

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheSingleflight_BasicDedup(t *testing.T) {
	sf := NewCacheSingleflight()
	var callCount int64

	fn := func() (*CCCacheEntry, error) {
		atomic.AddInt64(&callCount, 1)
		time.Sleep(50 * time.Millisecond)
		return &CCCacheEntry{Body: []byte("result"), StatusCode: 200}, nil
	}

	var wg sync.WaitGroup
	results := make([]*CCCacheEntry, 5)
	errs := make([]error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = sf.Do(context.Background(), "same-key", 5*time.Second, fn)
		}(i)
	}
	wg.Wait()

	if callCount != 1 {
		t.Errorf("expected fn called once, got %d", callCount)
	}

	for i, r := range results {
		if errs[i] != nil {
			t.Errorf("goroutine %d got error: %v", i, errs[i])
		}
		if r == nil || string(r.Body) != "result" {
			t.Errorf("goroutine %d got unexpected result", i)
		}
	}
}

func TestCacheSingleflight_DifferentKeys(t *testing.T) {
	sf := NewCacheSingleflight()
	var callCount int64

	fn := func() (*CCCacheEntry, error) {
		atomic.AddInt64(&callCount, 1)
		time.Sleep(20 * time.Millisecond)
		return &CCCacheEntry{Body: []byte("ok"), StatusCode: 200}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "key-" + string(rune('a'+idx))
			sf.Do(context.Background(), key, 5*time.Second, fn)
		}(i)
	}
	wg.Wait()

	if callCount != 3 {
		t.Errorf("different keys should each call fn: expected 3, got %d", callCount)
	}
}

func TestCacheSingleflight_ContextCancellation(t *testing.T) {
	sf := NewCacheSingleflight()

	ctx, cancel := context.WithCancel(context.Background())

	// Start a slow call
	go func() {
		sf.Do(context.Background(), "slow", 10*time.Second, func() (*CCCacheEntry, error) {
			time.Sleep(500 * time.Millisecond)
			return &CCCacheEntry{}, nil
		})
	}()

	time.Sleep(10 * time.Millisecond) // Let the first call register

	// Cancel the waiting context
	cancel()

	_, err := sf.Do(ctx, "slow", 10*time.Second, func() (*CCCacheEntry, error) {
		return &CCCacheEntry{}, nil
	})

	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestCacheSingleflight_Timeout(t *testing.T) {
	sf := NewCacheSingleflight()

	// Start a very slow call
	go func() {
		sf.Do(context.Background(), "timeout-key", 0, func() (*CCCacheEntry, error) {
			time.Sleep(2 * time.Second)
			return &CCCacheEntry{}, nil
		})
	}()

	time.Sleep(10 * time.Millisecond) // Let the first call register

	_, err := sf.Do(context.Background(), "timeout-key", 50*time.Millisecond, func() (*CCCacheEntry, error) {
		return &CCCacheEntry{}, nil
	})

	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestCacheSingleflight_ErrorPropagation(t *testing.T) {
	sf := NewCacheSingleflight()

	var wg sync.WaitGroup
	errs := make([]error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = sf.Do(context.Background(), "err-key", 5*time.Second, func() (*CCCacheEntry, error) {
				time.Sleep(20 * time.Millisecond)
				return nil, context.DeadlineExceeded
			})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != context.DeadlineExceeded {
			t.Errorf("goroutine %d: expected DeadlineExceeded, got %v", i, err)
		}
	}
}

func TestCacheSingleflight_SequentialSameKey(t *testing.T) {
	sf := NewCacheSingleflight()
	var callCount int64

	fn := func() (*CCCacheEntry, error) {
		atomic.AddInt64(&callCount, 1)
		return &CCCacheEntry{Body: []byte("ok")}, nil
	}

	// Sequential calls with same key should each execute
	sf.Do(context.Background(), "seq", 5*time.Second, fn)
	sf.Do(context.Background(), "seq", 5*time.Second, fn)

	if callCount != 2 {
		t.Errorf("sequential calls should each execute: expected 2, got %d", callCount)
	}
}
