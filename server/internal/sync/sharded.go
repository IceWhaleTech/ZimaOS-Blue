package sync

import (
	"sync"
	"sync/atomic"
)

// ShardedMap is a concurrent map that reduces lock contention by sharding.
type ShardedMap[K comparable, V any] struct {
	shards    []*mapShard[K, V]
	shardMask uint64
	hashFn    func(K) uint64
}

type mapShard[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

// NewShardedMap creates a new sharded map with the specified number of shards.
// shardCount should be a power of 2 for optimal performance.
func NewShardedMap[K comparable, V any](shardCount int, hashFn func(K) uint64) *ShardedMap[K, V] {
	// Ensure shard count is power of 2
	if shardCount <= 0 {
		shardCount = 32
	}
	shardCount = nextPowerOf2(shardCount)

	shards := make([]*mapShard[K, V], shardCount)
	for i := range shards {
		shards[i] = &mapShard[K, V]{
			items: make(map[K]V),
		}
	}

	return &ShardedMap[K, V]{
		shards:    shards,
		shardMask: uint64(shardCount - 1),
		hashFn:    hashFn,
	}
}

func nextPowerOf2(n int) int {
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	return n
}

func (m *ShardedMap[K, V]) getShard(key K) *mapShard[K, V] {
	hash := m.hashFn(key)
	return m.shards[hash&m.shardMask]
}

// Set sets a value in the map.
func (m *ShardedMap[K, V]) Set(key K, value V) {
	shard := m.getShard(key)
	shard.mu.Lock()
	shard.items[key] = value
	shard.mu.Unlock()
}

// Get retrieves a value from the map.
func (m *ShardedMap[K, V]) Get(key K) (V, bool) {
	shard := m.getShard(key)
	shard.mu.RLock()
	value, ok := shard.items[key]
	shard.mu.RUnlock()
	return value, ok
}

// Delete removes a key from the map.
func (m *ShardedMap[K, V]) Delete(key K) {
	shard := m.getShard(key)
	shard.mu.Lock()
	delete(shard.items, key)
	shard.mu.Unlock()
}

// Has checks if a key exists in the map.
func (m *ShardedMap[K, V]) Has(key K) bool {
	shard := m.getShard(key)
	shard.mu.RLock()
	_, ok := shard.items[key]
	shard.mu.RUnlock()
	return ok
}

// Len returns the total number of items in the map.
func (m *ShardedMap[K, V]) Len() int {
	total := 0
	for _, shard := range m.shards {
		shard.mu.RLock()
		total += len(shard.items)
		shard.mu.RUnlock()
	}
	return total
}

// Clear removes all items from the map.
func (m *ShardedMap[K, V]) Clear() {
	for _, shard := range m.shards {
		shard.mu.Lock()
		shard.items = make(map[K]V)
		shard.mu.Unlock()
	}
}

// Range iterates over all items in the map.
// The callback should return true to continue iteration.
func (m *ShardedMap[K, V]) Range(fn func(key K, value V) bool) {
	for _, shard := range m.shards {
		shard.mu.RLock()
		for k, v := range shard.items {
			if !fn(k, v) {
				shard.mu.RUnlock()
				return
			}
		}
		shard.mu.RUnlock()
	}
}

// GetOrSet returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
func (m *ShardedMap[K, V]) GetOrSet(key K, value V) (V, bool) {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if existing, ok := shard.items[key]; ok {
		return existing, true
	}
	shard.items[key] = value
	return value, false
}

// AtomicCounter is a lock-free counter.
type AtomicCounter struct {
	value int64
}

// NewAtomicCounter creates a new atomic counter.
func NewAtomicCounter(initial int64) *AtomicCounter {
	return &AtomicCounter{value: initial}
}

// Inc increments the counter by 1.
func (c *AtomicCounter) Inc() int64 {
	return atomic.AddInt64(&c.value, 1)
}

// Dec decrements the counter by 1.
func (c *AtomicCounter) Dec() int64 {
	return atomic.AddInt64(&c.value, -1)
}

// Add adds delta to the counter.
func (c *AtomicCounter) Add(delta int64) int64 {
	return atomic.AddInt64(&c.value, delta)
}

// Value returns the current value.
func (c *AtomicCounter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// Set sets the counter to a new value.
func (c *AtomicCounter) Set(value int64) {
	atomic.StoreInt64(&c.value, value)
}

// CompareAndSwap atomically compares and swaps the value.
func (c *AtomicCounter) CompareAndSwap(old, new int64) bool {
	return atomic.CompareAndSwapInt64(&c.value, old, new)
}

// AtomicFlag is a lock-free boolean flag.
type AtomicFlag struct {
	value int32
}

// NewAtomicFlag creates a new atomic flag.
func NewAtomicFlag(initial bool) *AtomicFlag {
	f := &AtomicFlag{}
	if initial {
		f.value = 1
	}
	return f
}

// Set sets the flag to true.
func (f *AtomicFlag) Set() {
	atomic.StoreInt32(&f.value, 1)
}

// Clear sets the flag to false.
func (f *AtomicFlag) Clear() {
	atomic.StoreInt32(&f.value, 0)
}

// IsSet returns true if the flag is set.
func (f *AtomicFlag) IsSet() bool {
	return atomic.LoadInt32(&f.value) == 1
}

// Toggle toggles the flag and returns the new value.
func (f *AtomicFlag) Toggle() bool {
	for {
		old := atomic.LoadInt32(&f.value)
		new := int32(1)
		if old == 1 {
			new = 0
		}
		if atomic.CompareAndSwapInt32(&f.value, old, new) {
			return new == 1
		}
	}
}

// SetOnce sets the flag to true only once.
func (f *AtomicFlag) SetOnce() bool {
	return atomic.CompareAndSwapInt32(&f.value, 0, 1)
}

// SpinLock is a simple spinlock for very short critical sections.
type SpinLock struct {
	locked int32
}

// Lock acquires the spinlock.
func (s *SpinLock) Lock() {
	for !atomic.CompareAndSwapInt32(&s.locked, 0, 1) {
		// Spin
	}
}

// Unlock releases the spinlock.
func (s *SpinLock) Unlock() {
	atomic.StoreInt32(&s.locked, 0)
}

// TryLock attempts to acquire the spinlock without blocking.
func (s *SpinLock) TryLock() bool {
	return atomic.CompareAndSwapInt32(&s.locked, 0, 1)
}

// RWSpinLock is a reader-writer spinlock.
type RWSpinLock struct {
	state int64 // negative = write locked, positive = read count
}

// RLock acquires a read lock.
func (rw *RWSpinLock) RLock() {
	for {
		state := atomic.LoadInt64(&rw.state)
		if state >= 0 && atomic.CompareAndSwapInt64(&rw.state, state, state+1) {
			return
		}
	}
}

// RUnlock releases a read lock.
func (rw *RWSpinLock) RUnlock() {
	atomic.AddInt64(&rw.state, -1)
}

// Lock acquires a write lock.
func (rw *RWSpinLock) Lock() {
	for !atomic.CompareAndSwapInt64(&rw.state, 0, -1) {
		// Spin
	}
}

// Unlock releases a write lock.
func (rw *RWSpinLock) Unlock() {
	atomic.StoreInt64(&rw.state, 0)
}

// TryLock attempts to acquire a write lock without blocking.
func (rw *RWSpinLock) TryLock() bool {
	return atomic.CompareAndSwapInt64(&rw.state, 0, -1)
}

// TryRLock attempts to acquire a read lock without blocking.
func (rw *RWSpinLock) TryRLock() bool {
	state := atomic.LoadInt64(&rw.state)
	return state >= 0 && atomic.CompareAndSwapInt64(&rw.state, state, state+1)
}

// StringHashFn is a hash function for strings.
func StringHashFn(s string) uint64 {
	// FNV-1a hash
	var hash uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= 1099511628211
	}
	return hash
}

// Int64HashFn is a hash function for int64.
func Int64HashFn(n int64) uint64 {
	// Mix bits
	x := uint64(n)
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	x = x ^ (x >> 31)
	return x
}

// Uint64HashFn is a hash function for uint64.
func Uint64HashFn(n uint64) uint64 {
	// Mix bits
	x := n
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	x = x ^ (x >> 31)
	return x
}
