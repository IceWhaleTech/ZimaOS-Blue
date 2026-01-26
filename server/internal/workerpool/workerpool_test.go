package workerpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.IOPoolSize != 4 {
		t.Errorf("expected IOPoolSize=4, got %d", config.IOPoolSize)
	}
	if config.ComputePoolSize != 8 {
		t.Errorf("expected ComputePoolSize=8, got %d", config.ComputePoolSize)
	}
	if config.QueueSize != 1000 {
		t.Errorf("expected QueueSize=1000, got %d", config.QueueSize)
	}
	if !config.Enabled {
		t.Error("expected Enabled=true by default")
	}
}

func TestNewPool(t *testing.T) {
	config := DefaultConfig()
	pool := New(config)
	defer pool.Stop()

	if pool == nil {
		t.Fatal("expected non-nil pool")
	}

	stats := pool.Stats()
	if stats.IO.Workers != config.IOPoolSize {
		t.Errorf("expected IO workers %d, got %d", config.IOPoolSize, stats.IO.Workers)
	}
	if stats.Compute.Workers != config.ComputePoolSize {
		t.Errorf("expected Compute workers %d, got %d", config.ComputePoolSize, stats.Compute.Workers)
	}
}

func TestSubmitIOTask(t *testing.T) {
	config := DefaultConfig()
	pool := New(config)
	pool.Start()
	defer pool.Stop()

	executed := make(chan bool, 1)
	err := pool.SubmitIO("test-io", func(ctx context.Context) error {
		executed <- true
		return nil
	})

	if err != nil {
		t.Fatalf("failed to submit IO task: %v", err)
	}

	select {
	case <-executed:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("IO task was not executed within timeout")
	}
}

func TestSubmitComputeTask(t *testing.T) {
	config := DefaultConfig()
	pool := New(config)
	pool.Start()
	defer pool.Stop()

	executed := make(chan bool, 1)
	err := pool.SubmitCompute("test-compute", func(ctx context.Context) error {
		executed <- true
		return nil
	})

	if err != nil {
		t.Fatalf("failed to submit compute task: %v", err)
	}

	select {
	case <-executed:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Compute task was not executed within timeout")
	}
}

func TestSubmitWithType(t *testing.T) {
	config := DefaultConfig()
	pool := New(config)
	pool.Start()
	defer pool.Stop()

	var ioExecuted, computeExecuted atomic.Bool

	// Submit IO task
	pool.Submit(&Task{
		ID:   "io-task",
		Type: TaskTypeIO,
		Handler: func(ctx context.Context) error {
			ioExecuted.Store(true)
			return nil
		},
	})

	// Submit compute task
	pool.Submit(&Task{
		ID:   "compute-task",
		Type: TaskTypeCompute,
		Handler: func(ctx context.Context) error {
			computeExecuted.Store(true)
			return nil
		},
	})

	time.Sleep(500 * time.Millisecond)

	if !ioExecuted.Load() {
		t.Error("IO task was not executed")
	}
	if !computeExecuted.Load() {
		t.Error("Compute task was not executed")
	}
}

func TestTaskTimeout(t *testing.T) {
	config := DefaultConfig()
	pool := New(config)
	pool.Start()
	defer pool.Stop()

	timedOut := make(chan bool, 1)
	err := pool.Submit(&Task{
		ID:      "timeout-task",
		Type:    TaskTypeCompute,
		Timeout: 100 * time.Millisecond,
		Handler: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				timedOut <- true
				return ctx.Err()
			case <-time.After(time.Second):
				return nil
			}
		},
	})

	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	select {
	case <-timedOut:
		// Success - task was cancelled due to timeout
	case <-time.After(2 * time.Second):
		t.Error("Task should have timed out")
	}
}

func TestPoolClosed(t *testing.T) {
	config := DefaultConfig()
	pool := New(config)
	pool.Start()
	pool.Stop()

	err := pool.SubmitIO("test", func(ctx context.Context) error {
		return nil
	})

	if err != ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed, got %v", err)
	}
}

func TestPoolStats(t *testing.T) {
	config := DefaultConfig()
	config.IOPoolSize = 2
	config.ComputePoolSize = 4
	pool := New(config)
	pool.Start()
	defer pool.Stop()

	// Submit some tasks
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		pool.SubmitIO("io-"+string(rune('0'+i)), func(ctx context.Context) error {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		pool.SubmitCompute("compute-"+string(rune('0'+i)), func(ctx context.Context) error {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	stats := pool.Stats()

	if stats.IO.TotalSubmitted != 5 {
		t.Errorf("expected IO TotalSubmitted=5, got %d", stats.IO.TotalSubmitted)
	}
	if stats.Compute.TotalSubmitted != 5 {
		t.Errorf("expected Compute TotalSubmitted=5, got %d", stats.Compute.TotalSubmitted)
	}
	if stats.IO.TotalCompleted != 5 {
		t.Errorf("expected IO TotalCompleted=5, got %d", stats.IO.TotalCompleted)
	}
	if stats.Compute.TotalCompleted != 5 {
		t.Errorf("expected Compute TotalCompleted=5, got %d", stats.Compute.TotalCompleted)
	}
}

func TestTaskFailure(t *testing.T) {
	config := DefaultConfig()
	pool := New(config)
	pool.Start()
	defer pool.Stop()

	done := make(chan bool, 1)
	pool.SubmitCompute("fail-task", func(ctx context.Context) error {
		defer func() { done <- true }()
		return errors.New("task failed")
	})

	<-done
	time.Sleep(100 * time.Millisecond)

	stats := pool.Stats()
	if stats.Compute.TotalFailed != 1 {
		t.Errorf("expected TotalFailed=1, got %d", stats.Compute.TotalFailed)
	}
}

func TestConcurrentExecution(t *testing.T) {
	config := DefaultConfig()
	config.IOPoolSize = 4
	pool := New(config)
	pool.Start()
	defer pool.Stop()

	var maxConcurrent atomic.Int64
	var current atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		pool.SubmitIO("task-"+string(rune('0'+i)), func(ctx context.Context) error {
			defer wg.Done()
			c := current.Add(1)
			for {
				max := maxConcurrent.Load()
				if c <= max || maxConcurrent.CompareAndSwap(max, c) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			current.Add(-1)
			return nil
		})
	}

	wg.Wait()

	// Max concurrent should not exceed pool size
	if maxConcurrent.Load() > int64(config.IOPoolSize) {
		t.Errorf("max concurrent %d exceeded pool size %d", maxConcurrent.Load(), config.IOPoolSize)
	}
}

// Priority Pool Tests

func TestNewPriorityPool(t *testing.T) {
	config := DefaultConfig()
	pool := NewPriorityPool(config)
	defer pool.Stop()

	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
}

func TestPriorityPoolSubmit(t *testing.T) {
	config := DefaultConfig()
	pool := NewPriorityPool(config)
	pool.Start()
	defer pool.Stop()

	executed := make(chan bool, 1)
	err := pool.Submit(&Task{
		ID:       "priority-task",
		Type:     TaskTypeCompute,
		Priority: 3,
		Handler: func(ctx context.Context) error {
			executed <- true
			return nil
		},
	})

	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	select {
	case <-executed:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Task was not executed within timeout")
	}
}

func TestPriorityOrdering(t *testing.T) {
	config := DefaultConfig()
	config.ComputePoolSize = 1 // Single worker to ensure ordering
	config.QueueSize = 100
	pool := NewPriorityPool(config)

	var order []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Submit tasks in reverse priority order
	for i := 0; i < 4; i++ {
		priority := i
		wg.Add(1)
		pool.Submit(&Task{
			ID:       "task-" + string(rune('0'+i)),
			Type:     TaskTypeCompute,
			Priority: priority,
			Handler: func(ctx context.Context) error {
				defer wg.Done()
				mu.Lock()
				order = append(order, priority)
				mu.Unlock()
				return nil
			},
		})
	}

	// Start pool after all tasks are queued
	pool.Start()
	wg.Wait()
	pool.Stop()

	mu.Lock()
	defer mu.Unlock()

	// Higher priority tasks should execute first
	if len(order) != 4 {
		t.Fatalf("expected 4 tasks executed, got %d", len(order))
	}

	// First task should be highest priority (3)
	if order[0] != 3 {
		t.Errorf("expected first task priority 3, got %d", order[0])
	}
}

func TestPriorityPoolClosed(t *testing.T) {
	config := DefaultConfig()
	pool := NewPriorityPool(config)
	pool.Start()
	pool.Stop()

	err := pool.Submit(&Task{
		ID:   "test",
		Type: TaskTypeCompute,
		Handler: func(ctx context.Context) error {
			return nil
		},
	})

	if err != ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed, got %v", err)
	}
}

func TestPriorityPoolStats(t *testing.T) {
	config := DefaultConfig()
	pool := NewPriorityPool(config)
	pool.Start()
	defer pool.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		pool.Submit(&Task{
			ID:       "task-" + string(rune('0'+i)),
			Type:     TaskTypeCompute,
			Priority: i % 4,
			Handler: func(ctx context.Context) error {
				defer wg.Done()
				return nil
			},
		})
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	stats := pool.Stats()
	if stats.Compute.TotalSubmitted != 10 {
		t.Errorf("expected TotalSubmitted=10, got %d", stats.Compute.TotalSubmitted)
	}
	if stats.Compute.TotalCompleted != 10 {
		t.Errorf("expected TotalCompleted=10, got %d", stats.Compute.TotalCompleted)
	}
}

func TestQueueFull(t *testing.T) {
	config := Config{
		IOPoolSize:      1,
		ComputePoolSize: 1,
		QueueSize:       4, // Very small queue
		Enabled:         true,
	}
	pool := New(config)
	// Don't start the pool so tasks stay in queue

	// Fill the queue
	for i := 0; i < 4; i++ {
		err := pool.SubmitIO("task-"+string(rune('0'+i)), func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Fatalf("failed to submit task %d: %v", i, err)
		}
	}

	// Next submission should fail
	err := pool.SubmitIO("overflow", func(ctx context.Context) error {
		return nil
	})
	if err != ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}

	pool.Stop()
}

func TestPriorityBounds(t *testing.T) {
	config := DefaultConfig()
	pool := NewPriorityPool(config)
	pool.Start()
	defer pool.Stop()

	var wg sync.WaitGroup

	// Test priority below 0
	wg.Add(1)
	err := pool.Submit(&Task{
		ID:       "low-priority",
		Type:     TaskTypeCompute,
		Priority: -5, // Should be clamped to 0
		Handler: func(ctx context.Context) error {
			defer wg.Done()
			return nil
		},
	})
	if err != nil {
		t.Errorf("failed to submit low priority task: %v", err)
	}

	// Test priority above 3
	wg.Add(1)
	err = pool.Submit(&Task{
		ID:       "high-priority",
		Type:     TaskTypeCompute,
		Priority: 10, // Should be clamped to 3
		Handler: func(ctx context.Context) error {
			defer wg.Done()
			return nil
		},
	})
	if err != nil {
		t.Errorf("failed to submit high priority task: %v", err)
	}

	wg.Wait()
}
