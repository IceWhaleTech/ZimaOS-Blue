// Package scheduler provides task scheduling capabilities.
package scheduler

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Common errors
var (
	ErrTaskNotFound   = errors.New("task not found")
	ErrSchedulerClosed = errors.New("scheduler is closed")
)

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Priority represents task priority.
type Priority int

const (
	PriorityLow    Priority = 0
	PriorityNormal Priority = 1
	PriorityHigh   Priority = 2
	PriorityCritical Priority = 3
)

// Task represents a scheduled task.
type Task struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Priority     Priority          `json:"priority"`
	Status       TaskStatus        `json:"status"`
	ScheduledAt  time.Time         `json:"scheduled_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
	Error        string            `json:"error,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	RetryCount   int               `json:"retry_count"`
	MaxRetries   int               `json:"max_retries"`
	Timeout      time.Duration     `json:"timeout,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`

	// Internal
	handler TaskHandler
	index   int // for heap
}

// TaskHandler is a function that executes a task.
type TaskHandler func(ctx context.Context, task *Task) error

// Config holds scheduler configuration.
type Config struct {
	// MaxConcurrent is the maximum number of concurrent tasks.
	MaxConcurrent int `yaml:"max_concurrent" yaml:"max_concurrent"`

	// DefaultMaxRetries is the default max retries for tasks.
	DefaultMaxRetries int `yaml:"default_max_retries" yaml:"default_max_retries"`

	// RetryDelay is the delay between retries.
	RetryDelay time.Duration `yaml:"retry_delay" yaml:"retry_delay"`

	// DefaultTimeout is the default timeout for tasks.
	DefaultTimeout time.Duration `yaml:"default_timeout" yaml:"default_timeout"`

	// Enabled enables the scheduler.
	Enabled bool `yaml:"enabled" yaml:"enabled"`
}

// DefaultConfig returns the default scheduler configuration.
func DefaultConfig() Config {
	return Config{
		MaxConcurrent:     10,
		DefaultMaxRetries: 3,
		RetryDelay:        time.Second * 5,
		DefaultTimeout:    time.Minute * 5,
		Enabled:           true,
	}
}

// Scheduler manages task scheduling and execution.
type Scheduler struct {
	config     Config
	tasks      map[string]*Task
	queue      *taskQueue
	mu         sync.RWMutex
	running    int
	runningMu  sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	closed     bool
	closedMu   sync.RWMutex
	taskChan   chan *Task
}

// New creates a new scheduler.
func New(config Config) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Scheduler{
		config:   config,
		tasks:    make(map[string]*Task),
		queue:    &taskQueue{},
		ctx:      ctx,
		cancel:   cancel,
		taskChan: make(chan *Task, config.MaxConcurrent),
	}

	heap.Init(s.queue)
	return s
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	// Start worker goroutines
	for i := 0; i < s.config.MaxConcurrent; i++ {
		s.wg.Add(1)
		go s.worker()
	}

	// Start dispatcher
	s.wg.Add(1)
	go s.dispatcher()
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	s.closedMu.Lock()
	s.closed = true
	s.closedMu.Unlock()

	s.cancel()
	close(s.taskChan)
	s.wg.Wait()
}

// Schedule schedules a new task.
func (s *Scheduler) Schedule(name string, handler TaskHandler, opts ...TaskOption) (*Task, error) {
	s.closedMu.RLock()
	if s.closed {
		s.closedMu.RUnlock()
		return nil, ErrSchedulerClosed
	}
	s.closedMu.RUnlock()

	task := &Task{
		ID:          uuid.New().String(),
		Name:        name,
		Priority:    PriorityNormal,
		Status:      TaskStatusPending,
		ScheduledAt: timeutil.NowTime(),
		MaxRetries:  s.config.DefaultMaxRetries,
		Timeout:     s.config.DefaultTimeout,
		Metadata:    make(map[string]string),
		handler:     handler,
	}

	for _, opt := range opts {
		opt(task)
	}

	s.mu.Lock()
	s.tasks[task.ID] = task
	heap.Push(s.queue, task)
	s.mu.Unlock()

	return task, nil
}

// Cancel cancels a task.
func (s *Scheduler) Cancel(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}

	if task.Status == TaskStatusPending {
		task.Status = TaskStatusCancelled
	}

	return nil
}

// GetTask returns a task by ID.
func (s *Scheduler) GetTask(taskID string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	return task, nil
}

// ListTasks returns all tasks.
func (s *Scheduler) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// Stats returns scheduler statistics.
func (s *Scheduler) Stats() SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.runningMu.Lock()
	running := s.running
	s.runningMu.Unlock()

	var pending, completed, failed, cancelled int
	for _, task := range s.tasks {
		switch task.Status {
		case TaskStatusPending:
			pending++
		case TaskStatusCompleted:
			completed++
		case TaskStatusFailed:
			failed++
		case TaskStatusCancelled:
			cancelled++
		}
	}

	return SchedulerStats{
		TotalTasks:     len(s.tasks),
		PendingTasks:   pending,
		RunningTasks:   running,
		CompletedTasks: completed,
		FailedTasks:    failed,
		CancelledTasks: cancelled,
		MaxConcurrent:  s.config.MaxConcurrent,
	}
}

// SchedulerStats holds scheduler statistics.
type SchedulerStats struct {
	TotalTasks     int `json:"total_tasks"`
	PendingTasks   int `json:"pending_tasks"`
	RunningTasks   int `json:"running_tasks"`
	CompletedTasks int `json:"completed_tasks"`
	FailedTasks    int `json:"failed_tasks"`
	CancelledTasks int `json:"cancelled_tasks"`
	MaxConcurrent  int `json:"max_concurrent"`
}

// dispatcher dispatches tasks to workers.
func (s *Scheduler) dispatcher() {
	defer s.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.dispatchTasks()
		}
	}
}

func (s *Scheduler) dispatchTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.runningMu.Lock()
	available := s.config.MaxConcurrent - s.running
	s.runningMu.Unlock()

	// Collect tasks that need to be re-queued due to unmet dependencies
	var requeue []*Task

	for available > 0 && s.queue.Len() > 0 {
		task := heap.Pop(s.queue).(*Task)

		if task.Status == TaskStatusCancelled {
			continue
		}

		// Check dependencies
		if !s.checkDependencies(task) {
			requeue = append(requeue, task)
			continue
		}

		select {
		case s.taskChan <- task:
			available--
		default:
			// Channel full, put task back
			heap.Push(s.queue, task)
			// Also re-queue dependency-blocked tasks
			for _, t := range requeue {
				heap.Push(s.queue, t)
			}
			return
		}
	}

	// Re-queue tasks with unmet dependencies
	for _, t := range requeue {
		heap.Push(s.queue, t)
	}
}

// checkDependencies checks if all task dependencies are completed
func (s *Scheduler) checkDependencies(task *Task) bool {
	if len(task.Dependencies) == 0 {
		return true
	}

	for _, depID := range task.Dependencies {
		depTask, ok := s.tasks[depID]
		if !ok {
			// Dependency not found, consider it failed
			return false
		}
		if depTask.Status != TaskStatusCompleted {
			return false
		}
	}

	return true
}

// worker processes tasks.
func (s *Scheduler) worker() {
	defer s.wg.Done()

	for task := range s.taskChan {
		s.executeTask(task)
	}
}

func (s *Scheduler) executeTask(task *Task) {
	s.runningMu.Lock()
	s.running++
	s.runningMu.Unlock()

	defer func() {
		s.runningMu.Lock()
		s.running--
		s.runningMu.Unlock()
	}()

	// Update task status
	s.mu.Lock()
	task.Status = TaskStatusRunning
	now := timeutil.NowTime()
	task.StartedAt = &now
	s.mu.Unlock()

	// Create context with timeout if specified
	ctx := s.ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(s.ctx, task.Timeout)
		defer cancel()
	}

	// Execute task
	err := task.handler(ctx, task)

	s.mu.Lock()
	defer s.mu.Unlock()

	completedAt := timeutil.NowTime()
	task.CompletedAt = &completedAt

	if err != nil {
		// Check if it was a timeout
		if ctx.Err() == context.DeadlineExceeded {
			task.Error = "task timed out"
		} else {
			task.Error = err.Error()
		}
		task.RetryCount++

		if task.RetryCount < task.MaxRetries {
			// Retry
			task.Status = TaskStatusPending
			task.StartedAt = nil
			task.CompletedAt = nil
			heap.Push(s.queue, task)
		} else {
			task.Status = TaskStatusFailed
		}
	} else {
		task.Status = TaskStatusCompleted
	}
}

// TaskOption is a function that configures a task.
type TaskOption func(*Task)

// WithPriority sets the task priority.
func WithPriority(p Priority) TaskOption {
	return func(t *Task) {
		t.Priority = p
	}
}

// WithMaxRetries sets the max retries.
func WithMaxRetries(n int) TaskOption {
	return func(t *Task) {
		t.MaxRetries = n
	}
}

// WithMetadata sets task metadata.
func WithMetadata(key, value string) TaskOption {
	return func(t *Task) {
		t.Metadata[key] = value
	}
}

// WithTimeout sets the task timeout.
func WithTimeout(timeout time.Duration) TaskOption {
	return func(t *Task) {
		t.Timeout = timeout
	}
}

// WithDependencies sets task dependencies.
func WithDependencies(deps ...string) TaskOption {
	return func(t *Task) {
		t.Dependencies = deps
	}
}

// taskQueue implements heap.Interface for priority queue.
type taskQueue []*Task

func (q taskQueue) Len() int { return len(q) }

func (q taskQueue) Less(i, j int) bool {
	// Higher priority first, then earlier scheduled time
	if q[i].Priority != q[j].Priority {
		return q[i].Priority > q[j].Priority
	}
	return q[i].ScheduledAt.Before(q[j].ScheduledAt)
}

func (q taskQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *taskQueue) Push(x interface{}) {
	n := len(*q)
	task := x.(*Task)
	task.index = n
	*q = append(*q, task)
}

func (q *taskQueue) Pop() interface{} {
	old := *q
	n := len(old)
	task := old[n-1]
	old[n-1] = nil
	task.index = -1
	*q = old[0 : n-1]
	return task
}
