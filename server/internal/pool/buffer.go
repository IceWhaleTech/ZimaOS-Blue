// Package pool provides memory and object pooling utilities.
package pool

import (
	"sync"
	"sync/atomic"
)

// BufferPool provides a pool of reusable byte buffers.
type BufferPool struct {
	pool     sync.Pool
	size     int
	maxSize  int

	// Statistics
	gets     atomic.Int64
	puts     atomic.Int64
	news     atomic.Int64
	discards atomic.Int64
}

// BufferPoolConfig holds configuration for the buffer pool.
type BufferPoolConfig struct {
	// InitialSize is the initial buffer size.
	InitialSize int `mapstructure:"initial_size"`
	// MaxSize is the maximum buffer size to pool (larger buffers are discarded).
	MaxSize int `mapstructure:"max_size"`
}

// DefaultBufferPoolConfig returns the default buffer pool configuration.
func DefaultBufferPoolConfig() BufferPoolConfig {
	return BufferPoolConfig{
		InitialSize: 4096,  // 4KB
		MaxSize:     65536, // 64KB
	}
}

// NewBufferPool creates a new buffer pool.
func NewBufferPool(config BufferPoolConfig) *BufferPool {
	bp := &BufferPool{
		size:    config.InitialSize,
		maxSize: config.MaxSize,
	}

	bp.pool.New = func() interface{} {
		bp.news.Add(1)
		return make([]byte, 0, bp.size)
	}

	return bp
}

// Get retrieves a buffer from the pool.
func (bp *BufferPool) Get() []byte {
	bp.gets.Add(1)
	buf := bp.pool.Get().([]byte)
	return buf[:0] // Reset length but keep capacity
}

// GetWithSize retrieves a buffer with at least the specified size.
func (bp *BufferPool) GetWithSize(size int) []byte {
	bp.gets.Add(1)

	if size > bp.maxSize {
		// Don't pool very large buffers
		bp.news.Add(1)
		return make([]byte, 0, size)
	}

	buf := bp.pool.Get().([]byte)
	if cap(buf) < size {
		// Buffer too small, create a new one
		bp.news.Add(1)
		return make([]byte, 0, size)
	}

	return buf[:0]
}

// Put returns a buffer to the pool.
func (bp *BufferPool) Put(buf []byte) {
	bp.puts.Add(1)

	// Don't pool very large buffers
	if cap(buf) > bp.maxSize {
		bp.discards.Add(1)
		return
	}

	bp.pool.Put(buf[:0])
}

// Stats returns pool statistics.
func (bp *BufferPool) Stats() BufferPoolStats {
	return BufferPoolStats{
		Gets:     bp.gets.Load(),
		Puts:     bp.puts.Load(),
		News:     bp.news.Load(),
		Discards: bp.discards.Load(),
		HitRate:  bp.hitRate(),
	}
}

// hitRate calculates the cache hit rate.
func (bp *BufferPool) hitRate() float64 {
	gets := bp.gets.Load()
	news := bp.news.Load()
	if gets == 0 {
		return 0
	}
	hits := gets - news
	if hits < 0 {
		hits = 0
	}
	return float64(hits) / float64(gets) * 100
}

// BufferPoolStats holds buffer pool statistics.
type BufferPoolStats struct {
	Gets     int64   `json:"gets"`
	Puts     int64   `json:"puts"`
	News     int64   `json:"news"`
	Discards int64   `json:"discards"`
	HitRate  float64 `json:"hit_rate"`
}

// ByteSlicePool provides a pool of byte slices with specific sizes.
type ByteSlicePool struct {
	pools   []*sync.Pool
	sizes   []int

	// Statistics
	gets     atomic.Int64
	puts     atomic.Int64
	news     atomic.Int64
}

// NewByteSlicePool creates a pool with predefined sizes.
// Sizes should be in ascending order.
func NewByteSlicePool(sizes []int) *ByteSlicePool {
	if len(sizes) == 0 {
		sizes = []int{64, 256, 1024, 4096, 16384, 65536}
	}

	bsp := &ByteSlicePool{
		pools: make([]*sync.Pool, len(sizes)),
		sizes: sizes,
	}

	for i, size := range sizes {
		s := size // Capture for closure
		bsp.pools[i] = &sync.Pool{
			New: func() interface{} {
				bsp.news.Add(1)
				return make([]byte, s)
			},
		}
	}

	return bsp
}

// Get retrieves a byte slice of at least the specified size.
func (bsp *ByteSlicePool) Get(size int) []byte {
	bsp.gets.Add(1)

	// Find the smallest pool that fits
	for i, poolSize := range bsp.sizes {
		if poolSize >= size {
			return bsp.pools[i].Get().([]byte)[:size]
		}
	}

	// Size too large, allocate directly
	bsp.news.Add(1)
	return make([]byte, size)
}

// Put returns a byte slice to the pool.
func (bsp *ByteSlicePool) Put(buf []byte) {
	bsp.puts.Add(1)

	size := cap(buf)
	for i, poolSize := range bsp.sizes {
		if poolSize == size {
			bsp.pools[i].Put(buf)
			return
		}
	}
	// Size doesn't match any pool, discard
}

// Stats returns pool statistics.
func (bsp *ByteSlicePool) Stats() ByteSlicePoolStats {
	return ByteSlicePoolStats{
		Gets:    bsp.gets.Load(),
		Puts:    bsp.puts.Load(),
		News:    bsp.news.Load(),
		HitRate: bsp.hitRate(),
	}
}

func (bsp *ByteSlicePool) hitRate() float64 {
	gets := bsp.gets.Load()
	news := bsp.news.Load()
	if gets == 0 {
		return 0
	}
	hits := gets - news
	if hits < 0 {
		hits = 0
	}
	return float64(hits) / float64(gets) * 100
}

// ByteSlicePoolStats holds byte slice pool statistics.
type ByteSlicePoolStats struct {
	Gets    int64   `json:"gets"`
	Puts    int64   `json:"puts"`
	News    int64   `json:"news"`
	HitRate float64 `json:"hit_rate"`
}

// Global buffer pool instance
var globalBufferPool = NewBufferPool(DefaultBufferPoolConfig())

// GetBuffer retrieves a buffer from the global pool.
func GetBuffer() []byte {
	return globalBufferPool.Get()
}

// GetBufferWithSize retrieves a buffer with at least the specified size.
func GetBufferWithSize(size int) []byte {
	return globalBufferPool.GetWithSize(size)
}

// PutBuffer returns a buffer to the global pool.
func PutBuffer(buf []byte) {
	globalBufferPool.Put(buf)
}

// GlobalBufferPoolStats returns statistics for the global buffer pool.
func GlobalBufferPoolStats() BufferPoolStats {
	return globalBufferPool.Stats()
}
