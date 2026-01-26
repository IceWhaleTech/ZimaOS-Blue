package pool

import (
	"bytes"
	"strings"
	"testing"
)

func TestObjectPool_GetPut(t *testing.T) {
	type testObj struct {
		Value int
		Name  string
	}

	pool := NewObjectPool(
		func() *testObj {
			return &testObj{}
		},
		func(obj *testObj) {
			obj.Value = 0
			obj.Name = ""
		},
	)

	// Get an object
	obj := pool.Get()
	if obj == nil {
		t.Fatal("Get() returned nil")
	}

	// Modify it
	obj.Value = 42
	obj.Name = "test"

	// Put it back
	pool.Put(obj)

	// Get another object (should be reset)
	obj2 := pool.Get()
	if obj2.Value != 0 {
		t.Errorf("Get() after Put() returned object with Value = %d, want 0", obj2.Value)
	}
	if obj2.Name != "" {
		t.Errorf("Get() after Put() returned object with Name = %q, want empty", obj2.Name)
	}
}

func TestObjectPool_Stats(t *testing.T) {
	pool := NewObjectPool(
		func() *int {
			v := 0
			return &v
		},
		nil,
	)

	// Get and put some objects
	for i := 0; i < 10; i++ {
		obj := pool.Get()
		pool.Put(obj)
	}

	stats := pool.Stats()
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

func TestObjectPool_NilPut(t *testing.T) {
	pool := NewObjectPool(
		func() *int {
			v := 0
			return &v
		},
		nil,
	)

	// Put nil should not panic
	pool.Put(nil)

	stats := pool.Stats()
	if stats.Puts != 0 {
		t.Errorf("Puts after nil Put = %d, want 0", stats.Puts)
	}
}

func TestBytesBufferPool(t *testing.T) {
	buf := GetBytesBuffer()
	if buf == nil {
		t.Fatal("GetBytesBuffer() returned nil")
	}

	buf.WriteString("hello world")
	if buf.String() != "hello world" {
		t.Errorf("Buffer content = %q, want %q", buf.String(), "hello world")
	}

	PutBytesBuffer(buf)

	// Get another buffer (should be reset)
	buf2 := GetBytesBuffer()
	if buf2.Len() != 0 {
		t.Errorf("GetBytesBuffer() after Put() returned buffer with len %d, want 0", buf2.Len())
	}
}

func TestStringBuilderPool(t *testing.T) {
	pool := NewStringBuilderPool()

	sb := pool.Get()
	if sb == nil {
		t.Fatal("Get() returned nil")
	}

	sb.WriteString("hello")
	sb.WriteString(" world")
	if sb.String() != "hello world" {
		t.Errorf("StringBuilder content = %q, want %q", sb.String(), "hello world")
	}

	pool.Put(sb)

	// Get another builder (should be reset)
	sb2 := pool.Get()
	if sb2.Len() != 0 {
		t.Errorf("Get() after Put() returned builder with len %d, want 0", sb2.Len())
	}
}

func TestStringBuilderPool_Stats(t *testing.T) {
	pool := NewStringBuilderPool()

	for i := 0; i < 5; i++ {
		sb := pool.Get()
		pool.Put(sb)
	}

	stats := pool.Stats()
	if stats.Gets != 5 {
		t.Errorf("Gets = %d, want 5", stats.Gets)
	}
	if stats.Puts != 5 {
		t.Errorf("Puts = %d, want 5", stats.Puts)
	}
}

func TestMapPool(t *testing.T) {
	pool := NewMapPool()

	m := pool.Get()
	if m == nil {
		t.Fatal("Get() returned nil")
	}

	m["key1"] = "value1"
	m["key2"] = 42

	pool.Put(m)

	// Get another map (should be cleared)
	m2 := pool.Get()
	if len(m2) != 0 {
		t.Errorf("Get() after Put() returned map with len %d, want 0", len(m2))
	}
}

func TestMapPool_Stats(t *testing.T) {
	pool := NewMapPool()

	for i := 0; i < 5; i++ {
		m := pool.Get()
		m["key"] = i
		pool.Put(m)
	}

	stats := pool.Stats()
	if stats.Gets != 5 {
		t.Errorf("Gets = %d, want 5", stats.Gets)
	}
	if stats.Puts != 5 {
		t.Errorf("Puts = %d, want 5", stats.Puts)
	}
}

func TestSlicePool(t *testing.T) {
	pool := NewSlicePool(8)

	s := pool.Get()
	if s == nil {
		t.Fatal("Get() returned nil")
	}

	if len(s) != 0 {
		t.Errorf("Get() returned slice with len %d, want 0", len(s))
	}

	if cap(s) < 8 {
		t.Errorf("Get() returned slice with cap %d, want >= 8", cap(s))
	}

	s = append(s, "a", "b", "c")
	pool.Put(s)

	// Get another slice (should be reset)
	s2 := pool.Get()
	if len(s2) != 0 {
		t.Errorf("Get() after Put() returned slice with len %d, want 0", len(s2))
	}
}

func TestSlicePool_DefaultCapacity(t *testing.T) {
	pool := NewSlicePool(0) // Should use default

	s := pool.Get()
	if cap(s) < 16 {
		t.Errorf("Get() with default capacity returned slice with cap %d, want >= 16", cap(s))
	}
}

func TestGlobalPools(t *testing.T) {
	// Test global string builder
	sb := GetStringBuilder()
	sb.WriteString("test")
	PutStringBuilder(sb)

	// Test global map
	m := GetMap()
	m["key"] = "value"
	PutMap(m)

	// Test global slice
	s := GetSlice()
	s = append(s, "item")
	PutSlice(s)
}

func BenchmarkObjectPool_GetPut(b *testing.B) {
	pool := NewObjectPool(
		func() *bytes.Buffer {
			return new(bytes.Buffer)
		},
		func(buf *bytes.Buffer) {
			buf.Reset()
		},
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			buf.WriteString("benchmark data")
			pool.Put(buf)
		}
	})
}

func BenchmarkBytesBufferPool(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := GetBytesBuffer()
			buf.WriteString("benchmark data")
			PutBytesBuffer(buf)
		}
	})
}

func BenchmarkNewBytesBuffer(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := new(bytes.Buffer)
			buf.WriteString("benchmark data")
			_ = buf
		}
	})
}

func BenchmarkStringBuilderPool(b *testing.B) {
	pool := NewStringBuilderPool()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sb := pool.Get()
			sb.WriteString("benchmark data")
			pool.Put(sb)
		}
	})
}

func BenchmarkNewStringBuilder(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sb := new(strings.Builder)
			sb.WriteString("benchmark data")
			_ = sb
		}
	})
}

func BenchmarkMapPool(b *testing.B) {
	pool := NewMapPool()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m := pool.Get()
			m["key"] = "value"
			pool.Put(m)
		}
	})
}

func BenchmarkNewMap(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m := make(map[string]interface{})
			m["key"] = "value"
			_ = m
		}
	})
}
