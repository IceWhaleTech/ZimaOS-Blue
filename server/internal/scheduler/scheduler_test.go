package scheduler

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

	if config.MaxConcurrent != 10 {
		t.Errorf("expected MaxConcurrent=10, got %d", config.MaxConcurrent)
	}
	if config.DefaultMaxRetries != 3 {
		t.Errorf("expected DefaultMaxRetries=3, got %d", config.DefaultMaxRetries)
	}
	if !config.Enabled {
		t.Error("expected Enabled=true by default")
	}
}

func TestNewScheduler(t *testing.T) {
	config := DefaultConfig()
	s := New(config)

	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}

	stats := s.Stats()
	if stats.TotalTasks != 0 {
		t.Errorf("expected 0 total tasks, got %d", stats.TotalTasks)
	}
}

func TestScheduleTask(t *testing.T) {
	config := DefaultConfig()
	s := New(config)
	s.Start()
	defer s.Stop()

	executed := make(chan bool, 1)
	handler := func(ctx context.Context, task *Task) error {
		executed <- true
		return nil
	}

	task, err := s.Schedule("test-task", handler)
	if err != nil {
		t.Fatalf("failed to schedule task: %v", err)
	}

	if task.ID == "" {
		t.Error("expected non-empty task ID")
	}
	if task.Name != "test-task" {
		t.Errorf("expected name 'test-task', got '%s'", task.Name)
	}
	if task.Status != TaskStatusPending {
		t.Errorf("expected status pending, got %s", task.Status)
	}

	// Wait for execution
	select {
	case <-executed:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("task was not executed within timeout")
	}

	// Check task completed
	time.Sleep(100 * time.Millisecond)
	task, _ = s.GetTask(task.ID)
	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status completed, got %s", task.Status)
	}
}

func TestScheduleWithPriority(t *testing.T) {
	config := DefaultConfig()
	config.MaxConcurrent = 1 // Process one at a time
	s := New(config)

	var order []string
	var mu sync.Mutex

	handler := func(name string) TaskHandler {
		return func(ctx context.Context, task *Task) error {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
			return nil
		}
	}

	// Schedule low priority first
	s.Schedule("low", handler("low"), WithPriority(PriorityLow))
	s.Schedule("high", handler("high"), WithPriority(PriorityHigh))
	s.Schedule("normal", handler("normal"), WithPriority(PriorityNormal))

	s.Start()
	time.Sleep(500 * time.Millisecond)
	s.Stop()

	mu.Lock()
	defer mu.Unlock()

	if len(order) < 3 {
		t.Fatalf("expected 3 tasks executed, got %d", len(order))
	}

	// High priority should be first
	if order[0] != "high" {
		t.Errorf("expected 'high' first, got '%s'", order[0])
	}
}

func TestTaskRetry(t *testing.T) {
	config := DefaultConfig()
	config.DefaultMaxRetries = 3
	s := New(config)
	s.Start()
	defer s.Stop()

	var attempts int32
	handler := func(ctx context.Context, task *Task) error {
		atomic.AddInt32(&attempts, 1)
		if atomic.LoadInt32(&attempts) < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	task, _ := s.Schedule("retry-task", handler, WithMaxRetries(3))

	// Wait for retries
	time.Sleep(time.Second)

	task, _ = s.GetTask(task.ID)
	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status completed after retries, got %s", task.Status)
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestTaskFailure(t *testing.T) {
	config := DefaultConfig()
	config.DefaultMaxRetries = 2
	s := New(config)
	s.Start()
	defer s.Stop()

	handler := func(ctx context.Context, task *Task) error {
		return errors.New("permanent error")
	}

	task, _ := s.Schedule("fail-task", handler, WithMaxRetries(2))

	// Wait for retries to exhaust
	time.Sleep(time.Second)

	task, _ = s.GetTask(task.ID)
	if task.Status != TaskStatusFailed {
		t.Errorf("expected status failed, got %s", task.Status)
	}
	if task.Error == "" {
		t.Error("expected error message")
	}
}

func TestCancelTask(t *testing.T) {
	config := DefaultConfig()
	config.MaxConcurrent = 0 // Don't process tasks
	s := New(config)

	task, _ := s.Schedule("cancel-task", func(ctx context.Context, task *Task) error {
		return nil
	})

	err := s.Cancel(task.ID)
	if err != nil {
		t.Fatalf("failed to cancel task: %v", err)
	}

	task, _ = s.GetTask(task.ID)
	if task.Status != TaskStatusCancelled {
		t.Errorf("expected status cancelled, got %s", task.Status)
	}
}

func TestCancelNonExistentTask(t *testing.T) {
	config := DefaultConfig()
	s := New(config)

	err := s.Cancel("non-existent")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestListTasks(t *testing.T) {
	config := DefaultConfig()
	s := New(config)

	handler := func(ctx context.Context, task *Task) error {
		return nil
	}

	s.Schedule("task1", handler)
	s.Schedule("task2", handler)
	s.Schedule("task3", handler)

	tasks := s.ListTasks()
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}
}

func TestSchedulerStats(t *testing.T) {
	config := DefaultConfig()
	s := New(config)

	handler := func(ctx context.Context, task *Task) error {
		return nil
	}

	s.Schedule("task1", handler)
	s.Schedule("task2", handler)

	stats := s.Stats()
	if stats.TotalTasks != 2 {
		t.Errorf("expected 2 total tasks, got %d", stats.TotalTasks)
	}
	if stats.PendingTasks != 2 {
		t.Errorf("expected 2 pending tasks, got %d", stats.PendingTasks)
	}
	if stats.MaxConcurrent != config.MaxConcurrent {
		t.Errorf("expected max concurrent %d, got %d", config.MaxConcurrent, stats.MaxConcurrent)
	}
}

func TestSchedulerClosed(t *testing.T) {
	config := DefaultConfig()
	s := New(config)
	s.Start()
	s.Stop()

	_, err := s.Schedule("task", func(ctx context.Context, task *Task) error {
		return nil
	})

	if err != ErrSchedulerClosed {
		t.Errorf("expected ErrSchedulerClosed, got %v", err)
	}
}

func TestWithMetadata(t *testing.T) {
	config := DefaultConfig()
	s := New(config)

	task, _ := s.Schedule("task", func(ctx context.Context, task *Task) error {
		return nil
	}, WithMetadata("key1", "value1"), WithMetadata("key2", "value2"))

	if task.Metadata["key1"] != "value1" {
		t.Errorf("expected metadata key1=value1, got %s", task.Metadata["key1"])
	}
	if task.Metadata["key2"] != "value2" {
		t.Errorf("expected metadata key2=value2, got %s", task.Metadata["key2"])
	}
}

func TestWithTimeout(t *testing.T) {
	config := DefaultConfig()
	s := New(config)
	s.Start()
	defer s.Stop()

	// Task that takes longer than timeout
	handler := func(ctx context.Context, task *Task) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			return nil
		}
	}

	task, _ := s.Schedule("timeout-task", handler, WithTimeout(100*time.Millisecond), WithMaxRetries(1))

	// Wait for task to timeout and fail
	time.Sleep(500 * time.Millisecond)

	task, _ = s.GetTask(task.ID)
	if task.Status != TaskStatusFailed {
		t.Errorf("expected status failed due to timeout, got %s", task.Status)
	}
	if task.Error != "task timed out" {
		t.Errorf("expected error 'task timed out', got '%s'", task.Error)
	}
}

func TestWithDependencies(t *testing.T) {
	config := DefaultConfig()
	config.MaxConcurrent = 1
	s := New(config)

	var order []string
	var mu sync.Mutex

	handler := func(name string) TaskHandler {
		return func(ctx context.Context, task *Task) error {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
			return nil
		}
	}

	// Schedule task1 first
	task1, _ := s.Schedule("task1", handler("task1"))

	// Schedule task2 that depends on task1
	task2, _ := s.Schedule("task2", handler("task2"), WithDependencies(task1.ID))

	// Schedule task3 that depends on task2
	s.Schedule("task3", handler("task3"), WithDependencies(task2.ID))

	s.Start()
	time.Sleep(time.Second)
	s.Stop()

	mu.Lock()
	defer mu.Unlock()

	if len(order) != 3 {
		t.Fatalf("expected 3 tasks executed, got %d", len(order))
	}

	// Tasks should execute in dependency order
	if order[0] != "task1" {
		t.Errorf("expected 'task1' first, got '%s'", order[0])
	}
	if order[1] != "task2" {
		t.Errorf("expected 'task2' second, got '%s'", order[1])
	}
	if order[2] != "task3" {
		t.Errorf("expected 'task3' third, got '%s'", order[2])
	}
}

func TestDependencyNotFound(t *testing.T) {
	config := DefaultConfig()
	s := New(config)
	s.Start()
	defer s.Stop()

	executed := false
	handler := func(ctx context.Context, task *Task) error {
		executed = true
		return nil
	}

	// Schedule task with non-existent dependency
	s.Schedule("dependent-task", handler, WithDependencies("non-existent-id"))

	// Wait a bit
	time.Sleep(500 * time.Millisecond)

	// Task should not have executed because dependency doesn't exist
	if executed {
		t.Error("task should not execute when dependency is not found")
	}
}
