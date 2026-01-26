package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewPool(t *testing.T) {
	ctx := context.Background()
	p := NewPool(ctx, 5)

	if p == nil {
		t.Fatal("NewPool() returned nil")
	}
	if p.Size() != 5 {
		t.Errorf("Size() = %v, want 5", p.Size())
	}
}

func TestPool_Submit(t *testing.T) {
	ctx := context.Background()
	p := NewPool(ctx, 5)

	var executed atomic.Bool
	p.Submit(func(ctx context.Context) error {
		executed.Store(true)
		return nil
	})

	err := p.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v", err)
	}

	if !executed.Load() {
		t.Error("Task was not executed")
	}
}

func TestPool_Concurrency(t *testing.T) {
	ctx := context.Background()
	poolSize := 3
	p := NewPool(ctx, poolSize)

	var maxConcurrent atomic.Int32
	var current atomic.Int32

	for i := 0; i < 10; i++ {
		p.Submit(func(ctx context.Context) error {
			c := current.Add(1)
			if c > maxConcurrent.Load() {
				maxConcurrent.Store(c)
			}
			time.Sleep(50 * time.Millisecond)
			current.Add(-1)
			return nil
		})
	}

	err := p.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v", err)
	}

	if maxConcurrent.Load() > int32(poolSize) {
		t.Errorf("Max concurrent = %v, should not exceed pool size %v", maxConcurrent.Load(), poolSize)
	}
}

func TestPool_Stats(t *testing.T) {
	ctx := context.Background()
	p := NewPool(ctx, 5)

	for i := 0; i < 3; i++ {
		p.Submit(func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}

	// Check stats while running
	time.Sleep(5 * time.Millisecond)
	stats := p.Stats()
	if stats.PoolSize != 5 {
		t.Errorf("Stats.PoolSize = %v, want 5", stats.PoolSize)
	}

	err := p.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v", err)
	}

	stats = p.Stats()
	if stats.Total != 3 {
		t.Errorf("Stats.Total = %v, want 3", stats.Total)
	}
	if stats.Running != 0 {
		t.Errorf("Stats.Running = %v, want 0", stats.Running)
	}
}

func TestPool_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := NewPool(ctx, 5)

	var cancelled atomic.Bool
	p.Submit(func(ctx context.Context) error {
		<-ctx.Done()
		cancelled.Store(true)
		return ctx.Err()
	})

	time.Sleep(10 * time.Millisecond)
	cancel()

	err := p.Wait()
	if err == nil {
		t.Error("Wait() should return error after cancellation")
	}

	if !cancelled.Load() {
		t.Error("Task should detect context cancellation")
	}
}
