package sync

import (
	"sync"
	"testing"
)

func TestShardedMap_Basic(t *testing.T) {
	m := NewShardedMap[string, int](32, StringHashFn)

	// Set
	m.Set("key1", 100)
	m.Set("key2", 200)

	// Get
	v, ok := m.Get("key1")
	if !ok || v != 100 {
		t.Errorf("Expected 100, got %d", v)
	}

	v, ok = m.Get("key2")
	if !ok || v != 200 {
		t.Errorf("Expected 200, got %d", v)
	}

	// Has
	if !m.Has("key1") {
		t.Error("Expected key1 to exist")
	}
	if m.Has("nonexistent") {
		t.Error("Expected nonexistent to not exist")
	}

	// Len
	if m.Len() != 2 {
		t.Errorf("Expected length 2, got %d", m.Len())
	}

	// Delete
	m.Delete("key1")
	if m.Has("key1") {
		t.Error("Expected key1 to be deleted")
	}
	if m.Len() != 1 {
		t.Errorf("Expected length 1, got %d", m.Len())
	}

	// Clear
	m.Clear()
	if m.Len() != 0 {
		t.Errorf("Expected length 0, got %d", m.Len())
	}
}

func TestShardedMap_GetOrSet(t *testing.T) {
	m := NewShardedMap[string, int](32, StringHashFn)

	// First call should set
	v, existed := m.GetOrSet("key1", 100)
	if existed {
		t.Error("Expected key to not exist")
	}
	if v != 100 {
		t.Errorf("Expected 100, got %d", v)
	}

	// Second call should get existing
	v, existed = m.GetOrSet("key1", 200)
	if !existed {
		t.Error("Expected key to exist")
	}
	if v != 100 {
		t.Errorf("Expected 100, got %d", v)
	}
}

func TestShardedMap_Range(t *testing.T) {
	m := NewShardedMap[string, int](32, StringHashFn)

	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	sum := 0
	m.Range(func(key string, value int) bool {
		sum += value
		return true
	})

	if sum != 6 {
		t.Errorf("Expected sum 6, got %d", sum)
	}
}

func TestShardedMap_Concurrent(t *testing.T) {
	m := NewShardedMap[int, int](32, func(k int) uint64 { return uint64(k) })

	var wg sync.WaitGroup
	n := 1000

	// Concurrent writes
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Set(i, i*2)
		}(i)
	}
	wg.Wait()

	// Verify
	if m.Len() != n {
		t.Errorf("Expected length %d, got %d", n, m.Len())
	}

	// Concurrent reads
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, ok := m.Get(i)
			if !ok || v != i*2 {
				t.Errorf("Expected %d, got %d", i*2, v)
			}
		}(i)
	}
	wg.Wait()
}

func TestAtomicCounter(t *testing.T) {
	c := NewAtomicCounter(0)

	if c.Value() != 0 {
		t.Errorf("Expected 0, got %d", c.Value())
	}

	c.Inc()
	if c.Value() != 1 {
		t.Errorf("Expected 1, got %d", c.Value())
	}

	c.Add(10)
	if c.Value() != 11 {
		t.Errorf("Expected 11, got %d", c.Value())
	}

	c.Dec()
	if c.Value() != 10 {
		t.Errorf("Expected 10, got %d", c.Value())
	}

	c.Set(100)
	if c.Value() != 100 {
		t.Errorf("Expected 100, got %d", c.Value())
	}

	if !c.CompareAndSwap(100, 200) {
		t.Error("Expected CAS to succeed")
	}
	if c.Value() != 200 {
		t.Errorf("Expected 200, got %d", c.Value())
	}

	if c.CompareAndSwap(100, 300) {
		t.Error("Expected CAS to fail")
	}
}

func TestAtomicCounter_Concurrent(t *testing.T) {
	c := NewAtomicCounter(0)
	var wg sync.WaitGroup
	n := 1000

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc()
		}()
	}
	wg.Wait()

	if c.Value() != int64(n) {
		t.Errorf("Expected %d, got %d", n, c.Value())
	}
}

func TestAtomicFlag(t *testing.T) {
	f := NewAtomicFlag(false)

	if f.IsSet() {
		t.Error("Expected flag to be false")
	}

	f.Set()
	if !f.IsSet() {
		t.Error("Expected flag to be true")
	}

	f.Clear()
	if f.IsSet() {
		t.Error("Expected flag to be false")
	}

	// Toggle
	result := f.Toggle()
	if !result || !f.IsSet() {
		t.Error("Expected toggle to set flag")
	}

	result = f.Toggle()
	if result || f.IsSet() {
		t.Error("Expected toggle to clear flag")
	}

	// SetOnce
	if !f.SetOnce() {
		t.Error("Expected SetOnce to succeed")
	}
	if f.SetOnce() {
		t.Error("Expected SetOnce to fail on second call")
	}
}

func TestSpinLock(t *testing.T) {
	var lock SpinLock
	counter := 0

	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock.Lock()
			counter++
			lock.Unlock()
		}()
	}
	wg.Wait()

	if counter != n {
		t.Errorf("Expected %d, got %d", n, counter)
	}
}

func TestSpinLock_TryLock(t *testing.T) {
	var lock SpinLock

	if !lock.TryLock() {
		t.Error("Expected TryLock to succeed")
	}

	if lock.TryLock() {
		t.Error("Expected TryLock to fail when locked")
	}

	lock.Unlock()

	if !lock.TryLock() {
		t.Error("Expected TryLock to succeed after unlock")
	}
}

func TestRWSpinLock(t *testing.T) {
	var lock RWSpinLock
	counter := 0

	var wg sync.WaitGroup

	// Multiple readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock.RLock()
			_ = counter
			lock.RUnlock()
		}()
	}

	// Single writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		lock.Lock()
		counter++
		lock.Unlock()
	}()

	wg.Wait()

	if counter != 1 {
		t.Errorf("Expected 1, got %d", counter)
	}
}

func TestHashFunctions(t *testing.T) {
	// String hash
	h1 := StringHashFn("hello")
	h2 := StringHashFn("hello")
	h3 := StringHashFn("world")

	if h1 != h2 {
		t.Error("Same string should produce same hash")
	}
	if h1 == h3 {
		t.Error("Different strings should produce different hashes")
	}

	// Int64 hash
	i1 := Int64HashFn(123)
	i2 := Int64HashFn(123)
	i3 := Int64HashFn(456)

	if i1 != i2 {
		t.Error("Same int should produce same hash")
	}
	if i1 == i3 {
		t.Error("Different ints should produce different hashes")
	}

	// Uint64 hash
	u1 := Uint64HashFn(123)
	u2 := Uint64HashFn(123)
	u3 := Uint64HashFn(456)

	if u1 != u2 {
		t.Error("Same uint should produce same hash")
	}
	if u1 == u3 {
		t.Error("Different uints should produce different hashes")
	}
}

func BenchmarkShardedMap_Set(b *testing.B) {
	m := NewShardedMap[int, int](32, func(k int) uint64 { return uint64(k) })

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(i%10000, i)
	}
}

func BenchmarkShardedMap_Get(b *testing.B) {
	m := NewShardedMap[int, int](32, func(k int) uint64 { return uint64(k) })

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
	m := NewShardedMap[int, int](32, func(k int) uint64 { return uint64(k) })

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
	m := NewShardedMap[int, int](32, func(k int) uint64 { return uint64(k) })

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

func BenchmarkSyncMap_Set(b *testing.B) {
	var m sync.Map

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Store(i%10000, i)
	}
}

func BenchmarkSyncMap_Get(b *testing.B) {
	var m sync.Map

	// Pre-populate
	for i := 0; i < 10000; i++ {
		m.Store(i, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Load(i % 10000)
	}
}

func BenchmarkAtomicCounter_Inc(b *testing.B) {
	c := NewAtomicCounter(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Inc()
	}
}

func BenchmarkAtomicCounter_IncParallel(b *testing.B) {
	c := NewAtomicCounter(0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Inc()
		}
	})
}

func BenchmarkSpinLock(b *testing.B) {
	var lock SpinLock
	counter := 0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lock.Lock()
		counter++
		lock.Unlock()
	}
}

func BenchmarkMutex(b *testing.B) {
	var mu sync.Mutex
	counter := 0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		counter++
		mu.Unlock()
	}
}
