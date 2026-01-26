// Package workerpool provides separate worker pools for IO and compute tasks.
package workerpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Common errors
var (
	ErrPoolClosed = errors.New("pool is closed")
	ErrQueueFull  = errors.New("task queue is full")
)

// TaskType represents the type of task.
type TaskType int

const (
	TaskTypeIO      TaskType = iota // IO-bound tasks (file operations, network)
	TaskTypeCompute                 // CPU-bound tasks (calculations, processing)
)

// Task represents a task to be executed by a worker pool.
type Task struct {
	ID       string
	Type     TaskType
	Handler  func(ctx context.Context) error
	Priority int
	Timeout  time.Duration
}

// Config holds worker pool configuration.
type Config struct {
	// IOPoolSize is the number of workers for IO tasks.
	IOPoolSize int `yaml:"io_pool_size" mapstructure:"io_pool_size"`

	// ComputePoolSize is the number of workers for compute tasks.
	ComputePoolSize int `yaml:"compute_pool_size" mapstructure:"compute_pool_size"`

	// QueueSize is the maximum number of pending tasks per pool.
	QueueSize int `yaml:"queue_size" mapstructure:"queue_size"`

	// Enabled enables the worker pool.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		IOPoolSize:      4,
		ComputePoolSize: 8,
		QueueSize:       1000,
		Enabled:         true,
	}
}

// Pool manages separate worker pools for IO and compute tasks.
type Pool struct {
	config      Config
	ioPool      *workerPool
	computePool *workerPool
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	closed      atomic.Bool
}

// New creates a new worker pool manager.
func New(config Config) *Pool {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Pool{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}

	p.ioPool = newWorkerPool("io", config.IOPoolSize, config.QueueSize)
	p.computePool = newWorkerPool("compute", config.ComputePoolSize, config.QueueSize)

	return p
}

// Start starts all worker pools.
func (p *Pool) Start() {
	p.ioPool.start(p.ctx, &p.wg)
	p.computePool.start(p.ctx, &p.wg)
}

// Stop stops all worker pools and waits for completion.
func (p *Pool) Stop() {
	p.closed.Store(true)
	p.cancel()
	p.wg.Wait()
}

// Submit submits a task to the appropriate pool.
func (p *Pool) Submit(task *Task) error {
	if p.closed.Load() {
		return ErrPoolClosed
	}

	switch task.Type {
	case TaskTypeIO:
		return p.ioPool.submit(task)
	case TaskTypeCompute:
		return p.computePool.submit(task)
	default:
		return p.computePool.submit(task)
	}
}

// SubmitIO submits an IO task.
func (p *Pool) SubmitIO(id string, handler func(ctx context.Context) error) error {
	return p.Submit(&Task{
		ID:      id,
		Type:    TaskTypeIO,
		Handler: handler,
	})
}

// SubmitCompute submits a compute task.
func (p *Pool) SubmitCompute(id string, handler func(ctx context.Context) error) error {
	return p.Submit(&Task{
		ID:      id,
		Type:    TaskTypeCompute,
		Handler: handler,
	})
}

// Stats returns statistics for all pools.
func (p *Pool) Stats() PoolStats {
	return PoolStats{
		IO:      p.ioPool.stats(),
		Compute: p.computePool.stats(),
	}
}

// PoolStats holds statistics for all pools.
type PoolStats struct {
	IO      WorkerPoolStats `json:"io"`
	Compute WorkerPoolStats `json:"compute"`
}

// WorkerPoolStats holds statistics for a single pool.
type WorkerPoolStats struct {
	Name           string `json:"name"`
	Workers        int    `json:"workers"`
	QueueSize      int    `json:"queue_size"`
	QueueCapacity  int    `json:"queue_capacity"`
	ActiveWorkers  int64  `json:"active_workers"`
	TotalSubmitted int64  `json:"total_submitted"`
	TotalCompleted int64  `json:"total_completed"`
	TotalFailed    int64  `json:"total_failed"`
}

// workerPool is a pool of workers for a specific task type.
type workerPool struct {
	name           string
	size           int
	queue          chan *Task
	activeWorkers  atomic.Int64
	totalSubmitted atomic.Int64
	totalCompleted atomic.Int64
	totalFailed    atomic.Int64
}

func newWorkerPool(name string, size, queueSize int) *workerPool {
	return &workerPool{
		name:  name,
		size:  size,
		queue: make(chan *Task, queueSize),
	}
}

func (wp *workerPool) start(ctx context.Context, wg *sync.WaitGroup) {
	for i := 0; i < wp.size; i++ {
		wg.Add(1)
		go wp.worker(ctx, wg)
	}
}

func (wp *workerPool) worker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-wp.queue:
			if !ok {
				return
			}
			wp.executeTask(ctx, task)
		}
	}
}

func (wp *workerPool) executeTask(ctx context.Context, task *Task) {
	wp.activeWorkers.Add(1)
	defer wp.activeWorkers.Add(-1)

	// Create task context with timeout if specified
	taskCtx := ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	// Execute task
	err := task.Handler(taskCtx)
	if err != nil {
		wp.totalFailed.Add(1)
	} else {
		wp.totalCompleted.Add(1)
	}
}

func (wp *workerPool) submit(task *Task) error {
	select {
	case wp.queue <- task:
		wp.totalSubmitted.Add(1)
		return nil
	default:
		return ErrQueueFull
	}
}

func (wp *workerPool) stats() WorkerPoolStats {
	return WorkerPoolStats{
		Name:           wp.name,
		Workers:        wp.size,
		QueueSize:      len(wp.queue),
		QueueCapacity:  cap(wp.queue),
		ActiveWorkers:  wp.activeWorkers.Load(),
		TotalSubmitted: wp.totalSubmitted.Load(),
		TotalCompleted: wp.totalCompleted.Load(),
		TotalFailed:    wp.totalFailed.Load(),
	}
}

// PriorityPool is a worker pool with priority queue support.
type PriorityPool struct {
	config      Config
	ioPool      *priorityWorkerPool
	computePool *priorityWorkerPool
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	closed      atomic.Bool
}

// NewPriorityPool creates a new priority-based worker pool manager.
func NewPriorityPool(config Config) *PriorityPool {
	ctx, cancel := context.WithCancel(context.Background())

	p := &PriorityPool{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}

	p.ioPool = newPriorityWorkerPool("io", config.IOPoolSize, config.QueueSize)
	p.computePool = newPriorityWorkerPool("compute", config.ComputePoolSize, config.QueueSize)

	return p
}

// Start starts all worker pools.
func (p *PriorityPool) Start() {
	p.ioPool.start(p.ctx, &p.wg)
	p.computePool.start(p.ctx, &p.wg)
}

// Stop stops all worker pools.
func (p *PriorityPool) Stop() {
	p.closed.Store(true)
	p.cancel()
	p.wg.Wait()
}

// Submit submits a task with priority.
func (p *PriorityPool) Submit(task *Task) error {
	if p.closed.Load() {
		return ErrPoolClosed
	}

	switch task.Type {
	case TaskTypeIO:
		return p.ioPool.submit(task)
	case TaskTypeCompute:
		return p.computePool.submit(task)
	default:
		return p.computePool.submit(task)
	}
}

// Stats returns statistics for all pools.
func (p *PriorityPool) Stats() PoolStats {
	return PoolStats{
		IO:      p.ioPool.stats(),
		Compute: p.computePool.stats(),
	}
}

// priorityWorkerPool is a worker pool with priority queue.
type priorityWorkerPool struct {
	name           string
	size           int
	queues         [4]chan *Task // Priority levels 0-3
	activeWorkers  atomic.Int64
	totalSubmitted atomic.Int64
	totalCompleted atomic.Int64
	totalFailed    atomic.Int64
}

func newPriorityWorkerPool(name string, size, queueSize int) *priorityWorkerPool {
	pwp := &priorityWorkerPool{
		name: name,
		size: size,
	}

	// Create queues for each priority level
	for i := range pwp.queues {
		pwp.queues[i] = make(chan *Task, queueSize/4)
	}

	return pwp
}

func (pwp *priorityWorkerPool) start(ctx context.Context, wg *sync.WaitGroup) {
	for i := 0; i < pwp.size; i++ {
		wg.Add(1)
		go pwp.worker(ctx, wg)
	}
}

func (pwp *priorityWorkerPool) worker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		// Check queues in priority order (highest first)
		var task *Task
		var ok bool

		select {
		case <-ctx.Done():
			return
		default:
		}

		// Try to get task from highest priority queue first
		for i := 3; i >= 0; i-- {
			select {
			case task, ok = <-pwp.queues[i]:
				if ok {
					goto execute
				}
			default:
				continue
			}
		}

		// No tasks available, wait on all queues
		select {
		case <-ctx.Done():
			return
		case task = <-pwp.queues[3]:
		case task = <-pwp.queues[2]:
		case task = <-pwp.queues[1]:
		case task = <-pwp.queues[0]:
		}

	execute:
		if task != nil {
			pwp.executeTask(ctx, task)
		}
	}
}

func (pwp *priorityWorkerPool) executeTask(ctx context.Context, task *Task) {
	pwp.activeWorkers.Add(1)
	defer pwp.activeWorkers.Add(-1)

	taskCtx := ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	err := task.Handler(taskCtx)
	if err != nil {
		pwp.totalFailed.Add(1)
	} else {
		pwp.totalCompleted.Add(1)
	}
}

func (pwp *priorityWorkerPool) submit(task *Task) error {
	priority := task.Priority
	if priority < 0 {
		priority = 0
	}
	if priority > 3 {
		priority = 3
	}

	select {
	case pwp.queues[priority] <- task:
		pwp.totalSubmitted.Add(1)
		return nil
	default:
		return ErrQueueFull
	}
}

func (pwp *priorityWorkerPool) stats() WorkerPoolStats {
	var queueSize int
	for _, q := range pwp.queues {
		queueSize += len(q)
	}

	var queueCapacity int
	for _, q := range pwp.queues {
		queueCapacity += cap(q)
	}

	return WorkerPoolStats{
		Name:           pwp.name,
		Workers:        pwp.size,
		QueueSize:      queueSize,
		QueueCapacity:  queueCapacity,
		ActiveWorkers:  pwp.activeWorkers.Load(),
		TotalSubmitted: pwp.totalSubmitted.Load(),
		TotalCompleted: pwp.totalCompleted.Load(),
		TotalFailed:    pwp.totalFailed.Load(),
	}
}
