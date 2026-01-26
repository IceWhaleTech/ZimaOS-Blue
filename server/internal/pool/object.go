package pool

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
)

// ObjectPool provides a generic object pool using sync.Pool.
type ObjectPool[T any] struct {
	pool    sync.Pool
	reset   func(*T)

	// Statistics
	gets atomic.Int64
	puts atomic.Int64
	news atomic.Int64
}

// NewObjectPool creates a new object pool.
// The newFunc creates new objects, and resetFunc resets objects before returning to pool.
func NewObjectPool[T any](newFunc func() *T, resetFunc func(*T)) *ObjectPool[T] {
	op := &ObjectPool[T]{
		reset: resetFunc,
	}

	op.pool.New = func() interface{} {
		op.news.Add(1)
		return newFunc()
	}

	return op
}

// Get retrieves an object from the pool.
func (op *ObjectPool[T]) Get() *T {
	op.gets.Add(1)
	return op.pool.Get().(*T)
}

// Put returns an object to the pool.
func (op *ObjectPool[T]) Put(obj *T) {
	if obj == nil {
		return
	}

	op.puts.Add(1)

	if op.reset != nil {
		op.reset(obj)
	}

	op.pool.Put(obj)
}

// Stats returns pool statistics.
func (op *ObjectPool[T]) Stats() ObjectPoolStats {
	gets := op.gets.Load()
	news := op.news.Load()
	hitRate := float64(0)
	if gets > 0 {
		hits := gets - news
		if hits < 0 {
			hits = 0
		}
		hitRate = float64(hits) / float64(gets) * 100
	}

	return ObjectPoolStats{
		Gets:    gets,
		Puts:    op.puts.Load(),
		News:    news,
		HitRate: hitRate,
	}
}

// ObjectPoolStats holds object pool statistics.
type ObjectPoolStats struct {
	Gets    int64   `json:"gets"`
	Puts    int64   `json:"puts"`
	News    int64   `json:"news"`
	HitRate float64 `json:"hit_rate"`
}

// JSONEncoderPool provides a pool of JSON encoders.
type JSONEncoderPool struct {
	pool *ObjectPool[json.Encoder]
	bufPool *BufferPool
}

// NewJSONEncoderPool creates a new JSON encoder pool.
func NewJSONEncoderPool() *JSONEncoderPool {
	bufPool := NewBufferPool(DefaultBufferPoolConfig())

	return &JSONEncoderPool{
		bufPool: bufPool,
		pool: NewObjectPool(
			func() *json.Encoder {
				return json.NewEncoder(bytes.NewBuffer(nil))
			},
			nil, // Encoders don't need reset
		),
	}
}

// JSONDecoderPool provides a pool of JSON decoders.
type JSONDecoderPool struct {
	pool *ObjectPool[json.Decoder]
}

// NewJSONDecoderPool creates a new JSON decoder pool.
func NewJSONDecoderPool() *JSONDecoderPool {
	return &JSONDecoderPool{
		pool: NewObjectPool(
			func() *json.Decoder {
				return json.NewDecoder(bytes.NewReader(nil))
			},
			nil,
		),
	}
}

// BytesBufferPool provides a pool of bytes.Buffer.
var bytesBufferPool = NewObjectPool(
	func() *bytes.Buffer {
		return new(bytes.Buffer)
	},
	func(b *bytes.Buffer) {
		b.Reset()
	},
)

// GetBytesBuffer retrieves a bytes.Buffer from the pool.
func GetBytesBuffer() *bytes.Buffer {
	return bytesBufferPool.Get()
}

// PutBytesBuffer returns a bytes.Buffer to the pool.
func PutBytesBuffer(buf *bytes.Buffer) {
	bytesBufferPool.Put(buf)
}

// BytesBufferPoolStats returns statistics for the bytes.Buffer pool.
func BytesBufferPoolStats() ObjectPoolStats {
	return bytesBufferPool.Stats()
}

// StringBuilderPool provides a pool of strings.Builder.
type StringBuilderPool struct {
	pool sync.Pool

	gets atomic.Int64
	puts atomic.Int64
	news atomic.Int64
}

// NewStringBuilderPool creates a new string builder pool.
func NewStringBuilderPool() *StringBuilderPool {
	sbp := &StringBuilderPool{}
	sbp.pool.New = func() interface{} {
		sbp.news.Add(1)
		return new(strings.Builder)
	}
	return sbp
}

// Get retrieves a strings.Builder from the pool.
func (sbp *StringBuilderPool) Get() *strings.Builder {
	sbp.gets.Add(1)
	return sbp.pool.Get().(*strings.Builder)
}

// Put returns a strings.Builder to the pool.
func (sbp *StringBuilderPool) Put(sb *strings.Builder) {
	if sb == nil {
		return
	}
	sbp.puts.Add(1)
	sb.Reset()
	sbp.pool.Put(sb)
}

// Stats returns pool statistics.
func (sbp *StringBuilderPool) Stats() ObjectPoolStats {
	gets := sbp.gets.Load()
	news := sbp.news.Load()
	hitRate := float64(0)
	if gets > 0 {
		hits := gets - news
		if hits < 0 {
			hits = 0
		}
		hitRate = float64(hits) / float64(gets) * 100
	}

	return ObjectPoolStats{
		Gets:    gets,
		Puts:    sbp.puts.Load(),
		News:    news,
		HitRate: hitRate,
	}
}

// MapPool provides a pool of map[string]interface{}.
type MapPool struct {
	pool sync.Pool

	gets atomic.Int64
	puts atomic.Int64
	news atomic.Int64
}

// NewMapPool creates a new map pool.
func NewMapPool() *MapPool {
	mp := &MapPool{}
	mp.pool.New = func() interface{} {
		mp.news.Add(1)
		return make(map[string]interface{})
	}
	return mp
}

// Get retrieves a map from the pool.
func (mp *MapPool) Get() map[string]interface{} {
	mp.gets.Add(1)
	return mp.pool.Get().(map[string]interface{})
}

// Put returns a map to the pool after clearing it.
func (mp *MapPool) Put(m map[string]interface{}) {
	if m == nil {
		return
	}
	mp.puts.Add(1)

	// Clear the map
	for k := range m {
		delete(m, k)
	}

	mp.pool.Put(m)
}

// Stats returns pool statistics.
func (mp *MapPool) Stats() ObjectPoolStats {
	gets := mp.gets.Load()
	news := mp.news.Load()
	hitRate := float64(0)
	if gets > 0 {
		hits := gets - news
		if hits < 0 {
			hits = 0
		}
		hitRate = float64(hits) / float64(gets) * 100
	}

	return ObjectPoolStats{
		Gets:    gets,
		Puts:    mp.puts.Load(),
		News:    news,
		HitRate: hitRate,
	}
}

// SlicePool provides a pool of []interface{}.
type SlicePool struct {
	pool    sync.Pool
	initCap int

	gets atomic.Int64
	puts atomic.Int64
	news atomic.Int64
}

// NewSlicePool creates a new slice pool with the specified initial capacity.
func NewSlicePool(initialCapacity int) *SlicePool {
	if initialCapacity <= 0 {
		initialCapacity = 16
	}

	sp := &SlicePool{initCap: initialCapacity}
	sp.pool.New = func() interface{} {
		sp.news.Add(1)
		return make([]interface{}, 0, sp.initCap)
	}
	return sp
}

// Get retrieves a slice from the pool.
func (sp *SlicePool) Get() []interface{} {
	sp.gets.Add(1)
	s := sp.pool.Get().([]interface{})
	return s[:0] // Reset length
}

// Put returns a slice to the pool.
func (sp *SlicePool) Put(s []interface{}) {
	if s == nil {
		return
	}
	sp.puts.Add(1)

	// Clear references to allow GC
	for i := range s {
		s[i] = nil
	}

	sp.pool.Put(s[:0])
}

// Stats returns pool statistics.
func (sp *SlicePool) Stats() ObjectPoolStats {
	gets := sp.gets.Load()
	news := sp.news.Load()
	hitRate := float64(0)
	if gets > 0 {
		hits := gets - news
		if hits < 0 {
			hits = 0
		}
		hitRate = float64(hits) / float64(gets) * 100
	}

	return ObjectPoolStats{
		Gets:    gets,
		Puts:    sp.puts.Load(),
		News:    news,
		HitRate: hitRate,
	}
}

// Global pools
var (
	globalStringBuilderPool = NewStringBuilderPool()
	globalMapPool           = NewMapPool()
	globalSlicePool         = NewSlicePool(16)
)

// GetStringBuilder retrieves a strings.Builder from the global pool.
func GetStringBuilder() *strings.Builder {
	return globalStringBuilderPool.Get()
}

// PutStringBuilder returns a strings.Builder to the global pool.
func PutStringBuilder(sb *strings.Builder) {
	globalStringBuilderPool.Put(sb)
}

// GetMap retrieves a map from the global pool.
func GetMap() map[string]interface{} {
	return globalMapPool.Get()
}

// PutMap returns a map to the global pool.
func PutMap(m map[string]interface{}) {
	globalMapPool.Put(m)
}

// GetSlice retrieves a slice from the global pool.
func GetSlice() []interface{} {
	return globalSlicePool.Get()
}

// PutSlice returns a slice to the global pool.
func PutSlice(s []interface{}) {
	globalSlicePool.Put(s)
}
