package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestOptimizedPool_Basic(t *testing.T) {
	ctx := context.Background()
	config := DefaultOptimizedPoolConfig()
	config.MinWorkers = 2
	config.MaxWorkers = 4
	config.QueueSize = 10

	pool := NewOptimizedPool(ctx, config)
	defer pool.Shutdown()

	var counter int64

	// Submit tasks
	for i := 0; i < 10; i++ {
		ok := pool.SubmitFunc(func(ctx context.Context) error {
			atomic.AddInt64(&counter, 1)
			return nil
		})
		if !ok {
			t.Fatal("Failed to submit task")
		}
	}

	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt64(&counter) != 10 {
		t.Errorf("Expected 10 tasks completed, got %d", counter)
	}

	stats := pool.Stats()
	if stats.TasksSubmitted != 10 {
		t.Errorf("Expected 10 tasks submitted, got %d", stats.TasksSubmitted)
	}
	if stats.TasksCompleted != 10 {
		t.Errorf("Expected 10 tasks completed, got %d", stats.TasksCompleted)
	}
}

func TestOptimizedPool_WithTimeout(t *testing.T) {
	ctx := context.Background()
	config := DefaultOptimizedPoolConfig()
	config.MinWorkers = 2
	config.MaxWorkers = 4

	pool := NewOptimizedPool(ctx, config)
	defer pool.Shutdown()

	// Submit a task that takes too long
	ok := pool.SubmitFuncWithTimeout(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			return nil
		}
	}, 50*time.Millisecond)

	if !ok {
		t.Fatal("Failed to submit task")
	}

	// Wait for task to timeout
	time.Sleep(100 * time.Millisecond)

	stats := pool.Stats()
	if stats.TasksFailed != 1 {
		t.Errorf("Expected 1 failed task, got %d", stats.TasksFailed)
	}
}

func TestOptimizedPool_TaskWithError(t *testing.T) {
	ctx := context.Background()
	config := DefaultOptimizedPoolConfig()
	config.MinWorkers = 2

	pool := NewOptimizedPool(ctx, config)
	defer pool.Shutdown()

	expectedErr := errors.New("task error")

	ok := pool.SubmitFunc(func(ctx context.Context) error {
		return expectedErr
	})

	if !ok {
		t.Fatal("Failed to submit task")
	}

	// Wait for completion
	time.Sleep(50 * time.Millisecond)

	stats := pool.Stats()
	if stats.TasksFailed != 1 {
		t.Errorf("Expected 1 failed task, got %d", stats.TasksFailed)
	}
}

func TestOptimizedPool_Results(t *testing.T) {
	ctx := context.Background()
	config := DefaultOptimizedPoolConfig()
	config.MinWorkers = 2
	config.QueueSize = 10

	pool := NewOptimizedPool(ctx, config)
	defer pool.Shutdown()

	// Submit task with ID
	pool.Submit(&Task{
		ID: "test-task-1",
		Fn: func(ctx context.Context) error {
			return nil
		},
	})

	// Read result
	select {
	case result := <-pool.Results():
		if result.TaskID != "test-task-1" {
			t.Errorf("Expected task ID 'test-task-1', got '%s'", result.TaskID)
		}
		if result.Error != nil {
			t.Errorf("Expected no error, got %v", result.Error)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for result")
	}
}

func TestOptimizedPool_Stats(t *testing.T) {
	ctx := context.Background()
	config := DefaultOptimizedPoolConfig()
	config.MinWorkers = 2
	config.MaxWorkers = 4
	config.QueueSize = 100

	pool := NewOptimizedPool(ctx, config)
	defer pool.Shutdown()

	// Submit some tasks
	for i := 0; i < 5; i++ {
		pool.SubmitFunc(func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	stats := pool.Stats()

	if stats.TotalWorkers < int64(config.MinWorkers) {
		t.Errorf("Expected at least %d workers, got %d", config.MinWorkers, stats.TotalWorkers)
	}

	if stats.TasksSubmitted != 5 {
		t.Errorf("Expected 5 tasks submitted, got %d", stats.TasksSubmitted)
	}

	if stats.TasksCompleted != 5 {
		t.Errorf("Expected 5 tasks completed, got %d", stats.TasksCompleted)
	}

	if stats.QueueCapacity != 100 {
		t.Errorf("Expected queue capacity 100, got %d", stats.QueueCapacity)
	}
}

func TestOptimizedPool_Shutdown(t *testing.T) {
	ctx := context.Background()
	config := DefaultOptimizedPoolConfig()
	config.MinWorkers = 2

	pool := NewOptimizedPool(ctx, config)

	// Submit a task
	pool.SubmitFunc(func(ctx context.Context) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	// Shutdown
	pool.Shutdown()

	// Try to submit after shutdown
	ok := pool.SubmitFunc(func(ctx context.Context) error {
		return nil
	})

	if ok {
		t.Error("Expected submit to fail after shutdown")
	}
}

func TestOptimizedPool_ShutdownWithTimeout(t *testing.T) {
	ctx := context.Background()
	config := DefaultOptimizedPoolConfig()
	config.MinWorkers = 2

	pool := NewOptimizedPool(ctx, config)

	// Submit a long-running task
	pool.SubmitFunc(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	})

	// Shutdown with short timeout
	err := pool.ShutdownWithTimeout(50 * time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}

func TestOptimizedPool_Concurrent(t *testing.T) {
	ctx := context.Background()
	config := DefaultOptimizedPoolConfig()
	config.MinWorkers = 4
	config.MaxWorkers = 8
	config.QueueSize = 1000

	pool := NewOptimizedPool(ctx, config)
	defer pool.Shutdown()

	var counter int64
	taskCount := 100

	// Submit tasks concurrently
	for i := 0; i < taskCount; i++ {
		go func() {
			pool.SubmitFunc(func(ctx context.Context) error {
				atomic.AddInt64(&counter, 1)
				return nil
			})
		}()
	}

	// Wait for completion
	time.Sleep(500 * time.Millisecond)

	if atomic.LoadInt64(&counter) != int64(taskCount) {
		t.Errorf("Expected %d tasks completed, got %d", taskCount, counter)
	}
}

func BenchmarkOptimizedPool_Submit(b *testing.B) {
	ctx := context.Background()
	config := DefaultOptimizedPoolConfig()
	config.MinWorkers = 4
	config.MaxWorkers = 16
	config.QueueSize = 10000

	pool := NewOptimizedPool(ctx, config)
	defer pool.Shutdown()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.SubmitFunc(func(ctx context.Context) error {
			return nil
		})
	}
}

func BenchmarkOptimizedPool_SubmitParallel(b *testing.B) {
	ctx := context.Background()
	config := DefaultOptimizedPoolConfig()
	config.MinWorkers = 4
	config.MaxWorkers = 16
	config.QueueSize = 10000

	pool := NewOptimizedPool(ctx, config)
	defer pool.Shutdown()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pool.SubmitFunc(func(ctx context.Context) error {
				return nil
			})
		}
	})
}
