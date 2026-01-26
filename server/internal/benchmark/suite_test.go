package benchmark

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/cache"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/pool"
	internalSync "github.com/IceWhaleTech/ZimaOS-Echo/server/internal/sync"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/worker"
)

func TestBenchmarkSuite_Basic(t *testing.T) {
	ctx := context.Background()
	suite := NewSuite("Basic Test Suite")

	counter := 0
	suite.Add("increment", func(ctx context.Context) error {
		counter++
		return nil
	})

	config := DefaultConfig()
	config.Duration = 100 * time.Millisecond
	config.Warmup = 5

	err := suite.Run(ctx, config)
	if err != nil {
		t.Fatalf("Suite.Run() error = %v", err)
	}

	results := suite.Results()
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	result := results["increment"]
	if result == nil {
		t.Fatal("Expected result for 'increment'")
	}

	if result.Iterations == 0 {
		t.Error("Expected iterations > 0")
	}

	if result.OpsPerSecond == 0 {
		t.Error("Expected ops/sec > 0")
	}
}

func TestBenchmarkSuite_Report(t *testing.T) {
	ctx := context.Background()
	suite := NewSuite("Report Test Suite")

	suite.Add("test", func(ctx context.Context) error {
		return nil
	})

	config := DefaultConfig()
	config.Duration = 50 * time.Millisecond

	suite.Run(ctx, config)

	report := suite.Report()
	if report == "" {
		t.Error("Expected non-empty report")
	}
}

func TestComparison(t *testing.T) {
	baseline := &Result{
		Name:         "test",
		OpsPerSecond: 1000,
		AvgDuration:  time.Millisecond,
		P99Duration:  2 * time.Millisecond,
		AllocsPerOp:  10,
		BytesPerOp:   100,
	}

	current := &Result{
		Name:         "test",
		OpsPerSecond: 1100,
		AvgDuration:  900 * time.Microsecond,
		P99Duration:  1800 * time.Microsecond,
		AllocsPerOp:  9,
		BytesPerOp:   90,
	}

	comparison := Compare(baseline, current)
	if comparison == nil {
		t.Fatal("Expected comparison")
	}

	if comparison.OpsChange <= 0 {
		t.Error("Expected positive ops change")
	}

	if comparison.IsRegression(5) {
		t.Error("Expected no regression")
	}

	report := comparison.Report()
	if report == "" {
		t.Error("Expected non-empty report")
	}
}

// Benchmark tests for cache package
func BenchmarkLRUCache_Set(b *testing.B) {
	cfg := cache.Config{
		MaxSize:         10000,
		CleanupInterval: 0,
	}
	c := cache.NewLRUCache(cfg)
	defer c.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(ctx, fmt.Sprintf("key%d", i%10000), "value", 0)
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	cfg := cache.Config{
		MaxSize:         10000,
		CleanupInterval: 0,
	}
	c := cache.NewLRUCache(cfg)
	defer c.Close()

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		c.Set(ctx, fmt.Sprintf("key%d", i), "value", 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(ctx, fmt.Sprintf("key%d", i%10000))
	}
}

func BenchmarkLRUCache_SetParallel(b *testing.B) {
	cfg := cache.Config{
		MaxSize:         10000,
		CleanupInterval: 0,
	}
	c := cache.NewLRUCache(cfg)
	defer c.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Set(ctx, fmt.Sprintf("key%d", i%10000), "value", 0)
			i++
		}
	})
}

func BenchmarkLRUCache_GetParallel(b *testing.B) {
	cfg := cache.Config{
		MaxSize:         10000,
		CleanupInterval: 0,
	}
	c := cache.NewLRUCache(cfg)
	defer c.Close()

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		c.Set(ctx, fmt.Sprintf("key%d", i), "value", 0)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get(ctx, fmt.Sprintf("key%d", i%10000))
			i++
		}
	})
}

func BenchmarkLFUCache_Set(b *testing.B) {
	cfg := cache.Config{
		MaxSize:         10000,
		CleanupInterval: 0,
	}
	c := cache.NewLFUCache(cfg)
	defer c.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(ctx, fmt.Sprintf("key%d", i%10000), "value", 0)
	}
}

func BenchmarkLFUCache_Get(b *testing.B) {
	cfg := cache.Config{
		MaxSize:         10000,
		CleanupInterval: 0,
	}
	c := cache.NewLFUCache(cfg)
	defer c.Close()

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		c.Set(ctx, fmt.Sprintf("key%d", i), "value", 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(ctx, fmt.Sprintf("key%d", i%10000))
	}
}

// Benchmark tests for pool package
func BenchmarkBufferPool_GetPut(b *testing.B) {
	p := pool.NewBufferPool(pool.DefaultBufferPoolConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := p.Get()
		buf = append(buf, []byte("test data")...)
		p.Put(buf)
	}
}

func BenchmarkBufferPool_GetPutParallel(b *testing.B) {
	p := pool.NewBufferPool(pool.DefaultBufferPoolConfig())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := p.Get()
			buf = append(buf, []byte("test data")...)
			p.Put(buf)
		}
	})
}

func BenchmarkByteSlicePool_GetPut(b *testing.B) {
	p := pool.NewByteSlicePool(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := p.Get(1024)
		copy(buf, []byte("test data"))
		p.Put(buf)
	}
}

func BenchmarkByteSlicePool_GetPutParallel(b *testing.B) {
	p := pool.NewByteSlicePool(nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := p.Get(1024)
			copy(buf, []byte("test data"))
			p.Put(buf)
		}
	})
}

func BenchmarkStringBuilderPool_GetPut(b *testing.B) {
	p := pool.NewStringBuilderPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb := p.Get()
		sb.WriteString("test data")
		p.Put(sb)
	}
}

// Benchmark tests for sync package
func BenchmarkShardedMap_Set(b *testing.B) {
	m := internalSync.NewShardedMap[int, int](32, func(k int) uint64 { return uint64(k) })

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(i%10000, i)
	}
}

func BenchmarkShardedMap_Get(b *testing.B) {
	m := internalSync.NewShardedMap[int, int](32, func(k int) uint64 { return uint64(k) })

	// Pre-populate
	for i := 0; i < 10000; i++ {
		m.Set(i, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Get(i % 10000)
	}
}

func BenchmarkShardedMap_SetParallel(b *testing.B) {
	m := internalSync.NewShardedMap[int, int](32, func(k int) uint64 { return uint64(k) })

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Set(i%10000, i)
			i++
		}
	})
}

func BenchmarkShardedMap_GetParallel(b *testing.B) {
	m := internalSync.NewShardedMap[int, int](32, func(k int) uint64 { return uint64(k) })

	// Pre-populate
	for i := 0; i < 10000; i++ {
		m.Set(i, i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Get(i % 10000)
			i++
		}
	})
}

func BenchmarkAtomicCounter_Inc(b *testing.B) {
	c := internalSync.NewAtomicCounter(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Inc()
	}
}

func BenchmarkAtomicCounter_IncParallel(b *testing.B) {
	c := internalSync.NewAtomicCounter(0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Inc()
		}
	})
}

// Benchmark tests for worker package
func BenchmarkOptimizedPool_Submit(b *testing.B) {
	ctx := context.Background()
	config := worker.DefaultOptimizedPoolConfig()
	config.MinWorkers = 4
	config.MaxWorkers = 16
	config.QueueSize = 10000

	p := worker.NewOptimizedPool(ctx, config)
	defer p.Shutdown()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.SubmitFunc(func(ctx context.Context) error {
			return nil
		})
	}
}

func BenchmarkOptimizedPool_SubmitParallel(b *testing.B) {
	ctx := context.Background()
	config := worker.DefaultOptimizedPoolConfig()
	config.MinWorkers = 4
	config.MaxWorkers = 16
	config.QueueSize = 10000

	p := worker.NewOptimizedPool(ctx, config)
	defer p.Shutdown()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.SubmitFunc(func(ctx context.Context) error {
				return nil
			})
		}
	})
}

// Benchmark tests for disk cache
func BenchmarkDiskCache_Set(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "disk-cache-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := cache.DefaultL2Config()
	config.Path = tmpDir
	config.CleanupInterval = 0

	c, err := cache.NewDiskCache(config)
	if err != nil {
		b.Fatalf("NewDiskCache() error = %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(ctx, fmt.Sprintf("key%d", i%10000), "benchmark value", 0)
	}
}

func BenchmarkDiskCache_Get(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "disk-cache-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := cache.DefaultL2Config()
	config.Path = tmpDir
	config.CleanupInterval = 0

	c, err := cache.NewDiskCache(config)
	if err != nil {
		b.Fatalf("NewDiskCache() error = %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		c.Set(ctx, fmt.Sprintf("key%d", i), "benchmark value", 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(ctx, fmt.Sprintf("key%d", i%1000))
	}
}

// Benchmark tests for multi-level cache
func BenchmarkMultiLevelCache_Set(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "multilevel-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := cache.DefaultMultiLevelConfig()
	config.L1.MaxSize = 10000
	config.L1.CleanupInterval = 0
	config.L2.Path = tmpDir
	config.L2.CleanupInterval = 0

	c, err := cache.NewMultiLevelCache(config)
	if err != nil {
		b.Fatalf("NewMultiLevelCache() error = %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(ctx, fmt.Sprintf("key%d", i%10000), "benchmark value", 0)
	}
}

func BenchmarkMultiLevelCache_Get(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "multilevel-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := cache.DefaultMultiLevelConfig()
	config.L1.MaxSize = 10000
	config.L1.CleanupInterval = 0
	config.L2.Path = tmpDir
	config.L2.CleanupInterval = 0

	c, err := cache.NewMultiLevelCache(config)
	if err != nil {
		b.Fatalf("NewMultiLevelCache() error = %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		c.Set(ctx, fmt.Sprintf("key%d", i), "benchmark value", 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(ctx, fmt.Sprintf("key%d", i%1000))
	}
}
