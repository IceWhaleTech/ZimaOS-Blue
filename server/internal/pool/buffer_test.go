package pool

import (
	"testing"
)

func TestBufferPool_GetPut(t *testing.T) {
	config := DefaultBufferPoolConfig()
	pool := NewBufferPool(config)

	// Get a buffer
	buf := pool.Get()
	if buf == nil {
		t.Fatal("Get() returned nil")
	}

	if len(buf) != 0 {
		t.Errorf("Get() returned buffer with len %d, want 0", len(buf))
	}

	if cap(buf) < config.InitialSize {
		t.Errorf("Get() returned buffer with cap %d, want >= %d", cap(buf), config.InitialSize)
	}

	// Use the buffer
	buf = append(buf, []byte("hello world")...)

	// Put it back
	pool.Put(buf)

	// Get another buffer (should be the same one)
	buf2 := pool.Get()
	if len(buf2) != 0 {
		t.Errorf("Get() after Put() returned buffer with len %d, want 0", len(buf2))
	}
}

func TestBufferPool_GetWithSize(t *testing.T) {
	config := BufferPoolConfig{
		InitialSize: 1024,
		MaxSize:     4096,
	}
	pool := NewBufferPool(config)

	tests := []struct {
		name    string
		size    int
		wantCap int
	}{
		{
			name:    "small size",
			size:    512,
			wantCap: 1024, // Should get default size
		},
		{
			name:    "exact size",
			size:    1024,
			wantCap: 1024,
		},
		{
			name:    "larger size",
			size:    2048,
			wantCap: 2048,
		},
		{
			name:    "very large size",
			size:    8192, // Larger than maxSize
			wantCap: 8192,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := pool.GetWithSize(tt.size)
			if cap(buf) < tt.wantCap {
				t.Errorf("GetWithSize(%d) cap = %d, want >= %d", tt.size, cap(buf), tt.wantCap)
			}
			pool.Put(buf)
		})
	}
}

func TestBufferPool_Stats(t *testing.T) {
	pool := NewBufferPool(DefaultBufferPoolConfig())

	// Initial stats
	stats := pool.Stats()
	if stats.Gets != 0 {
		t.Errorf("Initial Gets = %d, want 0", stats.Gets)
	}

	// Get and put some buffers
	for i := 0; i < 10; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}

	stats = pool.Stats()
	if stats.Gets != 10 {
		t.Errorf("Gets = %d, want 10", stats.Gets)
	}
	if stats.Puts != 10 {
		t.Errorf("Puts = %d, want 10", stats.Puts)
	}
	if stats.News < 1 {
		t.Errorf("News = %d, want >= 1", stats.News)
	}
}

func TestBufferPool_LargeBufferDiscard(t *testing.T) {
	config := BufferPoolConfig{
		InitialSize: 1024,
		MaxSize:     4096,
	}
	pool := NewBufferPool(config)

	// Get a large buffer
	buf := pool.GetWithSize(8192)
	if cap(buf) < 8192 {
		t.Fatalf("GetWithSize(8192) cap = %d, want >= 8192", cap(buf))
	}

	// Put it back (should be discarded)
	pool.Put(buf)

	stats := pool.Stats()
	if stats.Discards != 1 {
		t.Errorf("Discards = %d, want 1", stats.Discards)
	}
}

func TestByteSlicePool_Get(t *testing.T) {
	pool := NewByteSlicePool([]int{64, 256, 1024, 4096})

	tests := []struct {
		name    string
		size    int
		wantCap int
	}{
		{
			name:    "small",
			size:    32,
			wantCap: 64,
		},
		{
			name:    "exact",
			size:    256,
			wantCap: 256,
		},
		{
			name:    "between",
			size:    500,
			wantCap: 1024,
		},
		{
			name:    "large",
			size:    2000,
			wantCap: 4096,
		},
		{
			name:    "very large",
			size:    10000,
			wantCap: 10000, // Allocated directly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := pool.Get(tt.size)
			if len(buf) != tt.size {
				t.Errorf("Get(%d) len = %d, want %d", tt.size, len(buf), tt.size)
			}
			if cap(buf) < tt.wantCap {
				t.Errorf("Get(%d) cap = %d, want >= %d", tt.size, cap(buf), tt.wantCap)
			}
			pool.Put(buf)
		})
	}
}

func TestByteSlicePool_Stats(t *testing.T) {
	pool := NewByteSlicePool(nil) // Use default sizes

	// Get and put some slices
	for i := 0; i < 5; i++ {
		buf := pool.Get(100)
		pool.Put(buf)
	}

	stats := pool.Stats()
	if stats.Gets != 5 {
		t.Errorf("Gets = %d, want 5", stats.Gets)
	}
	if stats.Puts != 5 {
		t.Errorf("Puts = %d, want 5", stats.Puts)
	}
}

func TestGlobalBufferPool(t *testing.T) {
	// Test global functions
	buf := GetBuffer()
	if buf == nil {
		t.Fatal("GetBuffer() returned nil")
	}

	buf = append(buf, "test"...)
	PutBuffer(buf)

	buf2 := GetBufferWithSize(1024)
	if cap(buf2) < 1024 {
		t.Errorf("GetBufferWithSize(1024) cap = %d, want >= 1024", cap(buf2))
	}
	PutBuffer(buf2)

	stats := GlobalBufferPoolStats()
	if stats.Gets < 2 {
		t.Errorf("GlobalBufferPoolStats().Gets = %d, want >= 2", stats.Gets)
	}
}

func BenchmarkBufferPool_GetPut(b *testing.B) {
	pool := NewBufferPool(DefaultBufferPoolConfig())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			buf = append(buf, "benchmark data"...)
			pool.Put(buf)
		}
	})
}

func BenchmarkBufferPool_GetWithSize(b *testing.B) {
	pool := NewBufferPool(DefaultBufferPoolConfig())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.GetWithSize(1024)
			pool.Put(buf)
		}
	})
}

func BenchmarkByteSlicePool_Get(b *testing.B) {
	pool := NewByteSlicePool(nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get(256)
			pool.Put(buf)
		}
	})
}

func BenchmarkMakeSlice(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := make([]byte, 0, 4096)
			buf = append(buf, "benchmark data"...)
			_ = buf
		}
	})
}
