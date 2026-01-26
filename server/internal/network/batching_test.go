package network

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBatcher_Basic(t *testing.T) {
	ctx := context.Background()
	config := DefaultBatchConfig()
	config.MaxBatchSize = 10
	config.MaxWaitTime = 50 * time.Millisecond

	loadCount := int32(0)

	loader := func(ctx context.Context, keys []int) (map[int]string, error) {
		atomic.AddInt32(&loadCount, 1)
		result := make(map[int]string)
		for _, k := range keys {
			result[k] = "value"
		}
		return result, nil
	}

	batcher := NewBatcher[int, string](ctx, config, loader)
	defer batcher.Close()

	// Load single key
	v, err := batcher.Load(ctx, 1)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if v != "value" {
		t.Errorf("Load() = %v, want 'value'", v)
	}
}

func TestBatcher_Batching(t *testing.T) {
	ctx := context.Background()
	config := DefaultBatchConfig()
	config.MaxBatchSize = 10
	config.MaxWaitTime = 100 * time.Millisecond

	loadCount := int32(0)
	var loadedKeys []int
	var mu sync.Mutex

	loader := func(ctx context.Context, keys []int) (map[int]string, error) {
		atomic.AddInt32(&loadCount, 1)
		mu.Lock()
		loadedKeys = append(loadedKeys, keys...)
		mu.Unlock()

		result := make(map[int]string)
		for _, k := range keys {
			result[k] = "value"
		}
		return result, nil
	}

	batcher := NewBatcher[int, string](ctx, config, loader)
	defer batcher.Close()

	// Load multiple keys concurrently
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(key int) {
			defer wg.Done()
			batcher.Load(ctx, key)
		}(i)
	}
	wg.Wait()

	// Should have batched the requests
	if atomic.LoadInt32(&loadCount) > 2 {
		t.Errorf("Expected at most 2 batch loads, got %d", loadCount)
	}
}

func TestBatcher_LoadMany(t *testing.T) {
	ctx := context.Background()
	config := DefaultBatchConfig()

	loader := func(ctx context.Context, keys []int) (map[int]string, error) {
		result := make(map[int]string)
		for _, k := range keys {
			result[k] = "value"
		}
		return result, nil
	}

	batcher := NewBatcher[int, string](ctx, config, loader)
	defer batcher.Close()

	results, err := batcher.LoadMany(ctx, []int{1, 2, 3, 4, 5})
	if err != nil {
		t.Fatalf("LoadMany() error = %v", err)
	}

	if len(results) != 5 {
		t.Errorf("LoadMany() returned %d results, want 5", len(results))
	}
}

func TestBatcher_Error(t *testing.T) {
	ctx := context.Background()
	config := DefaultBatchConfig()

	expectedErr := errors.New("load error")

	loader := func(ctx context.Context, keys []int) (map[int]string, error) {
		return nil, expectedErr
	}

	batcher := NewBatcher[int, string](ctx, config, loader)
	defer batcher.Close()

	_, err := batcher.Load(ctx, 1)
	if err != expectedErr {
		t.Errorf("Load() error = %v, want %v", err, expectedErr)
	}
}

func TestSingleFlightGroup_Basic(t *testing.T) {
	g := NewSingleFlightGroup[string, int]()

	callCount := int32(0)

	fn := func() (int, error) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(50 * time.Millisecond)
		return 42, nil
	}

	// Call concurrently
	var wg sync.WaitGroup
	results := make([]int, 10)
	errs := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = g.Do("key", fn)
		}(i)
	}
	wg.Wait()

	// Should only call once
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// All results should be the same
	for i, r := range results {
		if r != 42 {
			t.Errorf("Result[%d] = %d, want 42", i, r)
		}
		if errs[i] != nil {
			t.Errorf("Error[%d] = %v, want nil", i, errs[i])
		}
	}
}

func TestSingleFlightGroup_DifferentKeys(t *testing.T) {
	g := NewSingleFlightGroup[string, int]()

	callCount := int32(0)

	fn := func() (int, error) {
		atomic.AddInt32(&callCount, 1)
		return 42, nil
	}

	// Call with different keys
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			g.Do(key, fn)
		}(string(rune('a' + i)))
	}
	wg.Wait()

	// Should call for each key
	if atomic.LoadInt32(&callCount) != 5 {
		t.Errorf("Expected 5 calls, got %d", callCount)
	}
}

func TestSingleFlightGroup_Forget(t *testing.T) {
	g := NewSingleFlightGroup[string, int]()

	callCount := int32(0)

	fn := func() (int, error) {
		atomic.AddInt32(&callCount, 1)
		return 42, nil
	}

	// First call
	g.Do("key", fn)

	// Forget and call again
	g.Forget("key")
	g.Do("key", fn)

	// Should call twice
	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestRequestCoalescer_Basic(t *testing.T) {
	ctx := context.Background()
	callCount := int32(0)

	loader := func(ctx context.Context, key string) (int, error) {
		atomic.AddInt32(&callCount, 1)
		return 42, nil
	}

	coalescer := NewRequestCoalescer[string, int](loader, 100*time.Millisecond)

	// First call
	v, err := coalescer.Load(ctx, "key")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if v != 42 {
		t.Errorf("Load() = %d, want 42", v)
	}

	// Second call should use cache
	v, err = coalescer.Load(ctx, "key")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if v != 42 {
		t.Errorf("Load() = %d, want 42", v)
	}

	// Should only call loader once
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func TestRequestCoalescer_Expiration(t *testing.T) {
	ctx := context.Background()
	callCount := int32(0)

	loader := func(ctx context.Context, key string) (int, error) {
		atomic.AddInt32(&callCount, 1)
		return 42, nil
	}

	coalescer := NewRequestCoalescer[string, int](loader, 50*time.Millisecond)

	// First call
	coalescer.Load(ctx, "key")

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Second call should reload
	coalescer.Load(ctx, "key")

	// Should call loader twice
	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestRequestCoalescer_Invalidate(t *testing.T) {
	ctx := context.Background()
	callCount := int32(0)

	loader := func(ctx context.Context, key string) (int, error) {
		atomic.AddInt32(&callCount, 1)
		return 42, nil
	}

	coalescer := NewRequestCoalescer[string, int](loader, time.Hour)

	// First call
	coalescer.Load(ctx, "key")

	// Invalidate
	coalescer.Invalidate("key")

	// Second call should reload
	coalescer.Load(ctx, "key")

	// Should call loader twice
	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestRequestCoalescer_Concurrent(t *testing.T) {
	ctx := context.Background()
	callCount := int32(0)

	loader := func(ctx context.Context, key string) (int, error) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(50 * time.Millisecond)
		return 42, nil
	}

	coalescer := NewRequestCoalescer[string, int](loader, time.Hour)

	// Concurrent calls
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			coalescer.Load(ctx, "key")
		}()
	}
	wg.Wait()

	// Should only call loader once
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func BenchmarkBatcher_Load(b *testing.B) {
	ctx := context.Background()
	config := DefaultBatchConfig()

	loader := func(ctx context.Context, keys []int) (map[int]string, error) {
		result := make(map[int]string)
		for _, k := range keys {
			result[k] = "value"
		}
		return result, nil
	}

	batcher := NewBatcher[int, string](ctx, config, loader)
	defer batcher.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batcher.Load(ctx, i%1000)
	}
}

func BenchmarkBatcher_LoadParallel(b *testing.B) {
	ctx := context.Background()
	config := DefaultBatchConfig()

	loader := func(ctx context.Context, keys []int) (map[int]string, error) {
		result := make(map[int]string)
		for _, k := range keys {
			result[k] = "value"
		}
		return result, nil
	}

	batcher := NewBatcher[int, string](ctx, config, loader)
	defer batcher.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			batcher.Load(ctx, i%1000)
			i++
		}
	})
}

func BenchmarkSingleFlightGroup_Do(b *testing.B) {
	g := NewSingleFlightGroup[int, int]()

	fn := func() (int, error) {
		return 42, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Do(i%100, fn)
	}
}

func BenchmarkRequestCoalescer_Load(b *testing.B) {
	ctx := context.Background()

	loader := func(ctx context.Context, key int) (int, error) {
		return 42, nil
	}

	coalescer := NewRequestCoalescer[int, int](loader, time.Hour)

	// Pre-populate cache
	for i := 0; i < 100; i++ {
		coalescer.Load(ctx, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		coalescer.Load(ctx, i%100)
	}
}
