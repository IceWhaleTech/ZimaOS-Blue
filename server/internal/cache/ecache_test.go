package cache

import (
	"context"
	"testing"
	"time"
)

func TestECache_GetSet(t *testing.T) {
	config := Config{
		MaxSize:    1000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache(config)
	defer c.Close()

	ctx := context.Background()

	// Set a value
	err := c.Set(ctx, "key1", "value1")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get the value
	value, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "value1" {
		t.Errorf("Get() = %v, want %v", value, "value1")
	}
}

func TestECache_GetNotFound(t *testing.T) {
	config := Config{
		MaxSize:    1000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache(config)
	defer c.Close()

	ctx := context.Background()

	_, err := c.Get(ctx, "nonexistent")
	if err != ErrKeyNotFound {
		t.Errorf("Get() error = %v, want %v", err, ErrKeyNotFound)
	}
}

func TestECache_Delete(t *testing.T) {
	config := Config{
		MaxSize:    1000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache(config)
	defer c.Close()

	ctx := context.Background()

	c.Set(ctx, "key1", "value1")
	c.Delete(ctx, "key1")

	if c.Exists(ctx, "key1") {
		t.Error("key1 should not exist after Delete()")
	}
}

func TestECache_Exists(t *testing.T) {
	config := Config{
		MaxSize:    1000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache(config)
	defer c.Close()

	ctx := context.Background()

	c.Set(ctx, "key1", "value1")

	if !c.Exists(ctx, "key1") {
		t.Error("Exists(key1) = false, want true")
	}

	if c.Exists(ctx, "nonexistent") {
		t.Error("Exists(nonexistent) = true, want false")
	}
}

func TestECache_GetOrSet(t *testing.T) {
	config := Config{
		MaxSize:    1000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache(config)
	defer c.Close()

	ctx := context.Background()
	callCount := 0

	fn := func() (interface{}, error) {
		callCount++
		return "generated", nil
	}

	// First call should generate
	value, err := c.GetOrSet(ctx, "key1", fn)
	if err != nil {
		t.Fatalf("GetOrSet() error = %v", err)
	}
	if value != "generated" {
		t.Errorf("GetOrSet() = %v, want %v", value, "generated")
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}

	// Second call should use cache
	value, err = c.GetOrSet(ctx, "key1", fn)
	if err != nil {
		t.Fatalf("GetOrSet() error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (should not call fn again)", callCount)
	}
}

func TestECache_SetNX(t *testing.T) {
	config := Config{
		MaxSize:    1000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache(config)
	defer c.Close()

	ctx := context.Background()

	// First SetNX should succeed
	if !c.SetNX(ctx, "key1", "value1") {
		t.Error("SetNX() = false, want true")
	}

	// Second SetNX should fail
	if c.SetNX(ctx, "key1", "value2") {
		t.Error("SetNX() = true, want false")
	}

	// Value should be the first one
	value, _ := c.Get(ctx, "key1")
	if value != "value1" {
		t.Errorf("Get() = %v, want %v", value, "value1")
	}
}

func TestECache_Stats(t *testing.T) {
	config := Config{
		MaxSize:    1000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache(config)
	defer c.Close()

	ctx := context.Background()

	c.Set(ctx, "key1", "value1")
	c.Get(ctx, "key1")
	c.Get(ctx, "nonexistent")
	c.Delete(ctx, "key1")

	stats := c.Stats()

	if stats.Hits != 1 {
		t.Errorf("Stats.Hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Stats.Misses = %d, want 1", stats.Misses)
	}
	if stats.Sets != 1 {
		t.Errorf("Stats.Sets = %d, want 1", stats.Sets)
	}
	if stats.Deletes != 1 {
		t.Errorf("Stats.Deletes = %d, want 1", stats.Deletes)
	}
}

func TestECache_Clear(t *testing.T) {
	config := Config{
		MaxSize:    1000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache(config)
	defer c.Close()

	ctx := context.Background()

	c.Set(ctx, "key1", "value1")
	c.Set(ctx, "key2", "value2")
	c.Clear(ctx)

	if c.Exists(ctx, "key1") || c.Exists(ctx, "key2") {
		t.Error("Cache should be empty after Clear()")
	}
}

func TestECache2_Basic(t *testing.T) {
	config := Config{
		MaxSize:    1000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache2(config)
	defer c.Close()

	ctx := context.Background()

	// Set a value
	err := c.Set(ctx, "key1", "value1")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get the value
	value, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "value1" {
		t.Errorf("Get() = %v, want %v", value, "value1")
	}
}

func BenchmarkECache_Set(b *testing.B) {
	config := Config{
		MaxSize:    10000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache(config)
	defer c.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(ctx, string(rune(i%10000)), "value")
	}
}

func BenchmarkECache_Get(b *testing.B) {
	config := Config{
		MaxSize:    10000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache(config)
	defer c.Close()

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		c.Set(ctx, string(rune(i)), "value")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(ctx, string(rune(i%10000)))
	}
}

func BenchmarkECache_SetParallel(b *testing.B) {
	config := Config{
		MaxSize:    10000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache(config)
	defer c.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Set(ctx, string(rune(i%10000)), "value")
			i++
		}
	})
}

func BenchmarkECache_GetParallel(b *testing.B) {
	config := Config{
		MaxSize:    10000,
		DefaultTTL: 5 * time.Minute,
	}
	c := NewECache(config)
	defer c.Close()

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		c.Set(ctx, string(rune(i)), "value")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get(ctx, string(rune(i%10000)))
			i++
		}
	})
}
