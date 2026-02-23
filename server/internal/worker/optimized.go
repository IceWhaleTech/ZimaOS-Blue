package worker

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Task represents a unit of work to be executed by the pool.
type Task struct {
	ID       string
	Fn       func(context.Context) error
	Priority int
	Timeout  time.Duration
}

// TaskResult holds the result of a task execution.
type TaskResult struct {
	TaskID   string
	Error    error
	Duration time.Duration
}

// OptimizedPoolConfig holds configuration for the optimized pool.
type OptimizedPoolConfig struct {
	MinWorkers      int
	MaxWorkers      int
	QueueSize       int
	IdleTimeout     time.Duration
	ScaleUpThreshold   float64 // Queue utilization threshold to scale up
	ScaleDownThreshold float64 // Queue utilization threshold to scale down
	MetricsEnabled  bool
}

// DefaultOptimizedPoolConfig returns default configuration.
func DefaultOptimizedPoolConfig() OptimizedPoolConfig {
	numCPU := runtime.NumCPU()
	return OptimizedPoolConfig{
		MinWorkers:         numCPU,
		MaxWorkers:         numCPU * 4,
		QueueSize:          1000,
		IdleTimeout:        30 * time.Second,
		ScaleUpThreshold:   0.8,
		ScaleDownThreshold: 0.2,
		MetricsEnabled:     true,
	}
}

// OptimizedPool is a high-performance worker pool with auto-scaling.
type OptimizedPool struct {
	config     OptimizedPoolConfig
	tasks      chan *Task
	results    chan *TaskResult
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	workerWg   sync.WaitGroup

	// Atomic counters for lock-free stats
	activeWorkers  int64
	totalWorkers   int64
	tasksSubmitted int64
	tasksCompleted int64
	tasksFailed    int64
	totalWaitTime  int64 // nanoseconds
	totalExecTime  int64 // nanoseconds

	// Scaling control
	scaleMu     sync.Mutex
	lastScale   time.Time
	scaleCooldown time.Duration

	// Shutdown flag
	closed int32
}

// NewOptimizedPool creates a new optimized worker pool.
func NewOptimizedPool(ctx context.Context, config OptimizedPoolConfig) *OptimizedPool {
	ctx, cancel := context.WithCancel(ctx)

	p := &OptimizedPool{
		config:        config,
		tasks:         make(chan *Task, config.QueueSize),
		results:       make(chan *TaskResult, config.QueueSize),
		ctx:           ctx,
		cancel:        cancel,
		scaleCooldown: time.Second,
		lastScale:     timeutil.NowTime(),
	}

	// Start minimum number of workers
	for i := 0; i < config.MinWorkers; i++ {
		p.startWorker()
	}

	// Start auto-scaler if enabled
	if config.MetricsEnabled {
		go p.autoScaler()
	}

	return p
}

// startWorker starts a new worker goroutine.
func (p *OptimizedPool) startWorker() {
	atomic.AddInt64(&p.totalWorkers, 1)
	p.workerWg.Add(1)

	go func() {
		defer p.workerWg.Done()
		defer atomic.AddInt64(&p.totalWorkers, -1)

		idleTimer := time.NewTimer(p.config.IdleTimeout)
		defer idleTimer.Stop()

		for {
			select {
			case <-p.ctx.Done():
				return

			case task, ok := <-p.tasks:
				if !ok {
					return
				}

				// Reset idle timer
				if !idleTimer.Stop() {
					select {
					case <-idleTimer.C:
					default:
					}
				}
				idleTimer.Reset(p.config.IdleTimeout)

				// Execute task
				p.executeTask(task)

			case <-idleTimer.C:
				// Check if we can scale down
				currentWorkers := atomic.LoadInt64(&p.totalWorkers)
				if currentWorkers > int64(p.config.MinWorkers) {
					return
				}
				idleTimer.Reset(p.config.IdleTimeout)
			}
		}
	}()
}

// executeTask executes a single task.
func (p *OptimizedPool) executeTask(task *Task) {
	atomic.AddInt64(&p.activeWorkers, 1)
	defer atomic.AddInt64(&p.activeWorkers, -1)

	start := timeutil.NowTime()

	// Create context with timeout if specified
	ctx := p.ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(p.ctx, task.Timeout)
		defer cancel()
	}

	// Execute the task
	err := task.Fn(ctx)

	duration := timeutil.SinceTime(start)
	atomic.AddInt64(&p.totalExecTime, int64(duration))

	if err != nil {
		atomic.AddInt64(&p.tasksFailed, 1)
	}
	atomic.AddInt64(&p.tasksCompleted, 1)

	// Send result if results channel is being consumed
	if atomic.LoadInt32(&p.closed) == 0 {
		select {
		case p.results <- &TaskResult{
			TaskID:   task.ID,
			Error:    err,
			Duration: duration,
		}:
		default:
			// Results channel full or not being consumed
		}
	}
}

// Submit submits a task to the pool.
func (p *OptimizedPool) Submit(task *Task) bool {
	if atomic.LoadInt32(&p.closed) == 1 {
		return false
	}

	start := timeutil.NowTime()

	select {
	case p.tasks <- task:
		atomic.AddInt64(&p.tasksSubmitted, 1)
		atomic.AddInt64(&p.totalWaitTime, int64(timeutil.SinceTime(start)))
		return true
	case <-p.ctx.Done():
		return false
	}
}

// SubmitFunc is a convenience method to submit a function directly.
func (p *OptimizedPool) SubmitFunc(fn func(context.Context) error) bool {
	return p.Submit(&Task{Fn: fn})
}

// SubmitFuncWithTimeout submits a function with a timeout.
func (p *OptimizedPool) SubmitFuncWithTimeout(fn func(context.Context) error, timeout time.Duration) bool {
	return p.Submit(&Task{Fn: fn, Timeout: timeout})
}

// autoScaler monitors queue utilization and scales workers.
func (p *OptimizedPool) autoScaler() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.checkScale()
		}
	}
}

// checkScale checks if scaling is needed.
func (p *OptimizedPool) checkScale() {
	p.scaleMu.Lock()
	defer p.scaleMu.Unlock()

	// Check cooldown
	if timeutil.SinceTime(p.lastScale) < p.scaleCooldown {
		return
	}

	queueLen := len(p.tasks)
	queueCap := cap(p.tasks)
	utilization := float64(queueLen) / float64(queueCap)

	currentWorkers := atomic.LoadInt64(&p.totalWorkers)

	// Scale up
	if utilization > p.config.ScaleUpThreshold && currentWorkers < int64(p.config.MaxWorkers) {
		workersToAdd := int64(p.config.MinWorkers)
		if currentWorkers+workersToAdd > int64(p.config.MaxWorkers) {
			workersToAdd = int64(p.config.MaxWorkers) - currentWorkers
		}
		for i := int64(0); i < workersToAdd; i++ {
			p.startWorker()
		}
		p.lastScale = timeutil.NowTime()
	}
}

// Results returns the results channel.
func (p *OptimizedPool) Results() <-chan *TaskResult {
	return p.results
}

// OptimizedPoolStats holds pool statistics.
type OptimizedPoolStats struct {
	ActiveWorkers   int64         `json:"active_workers"`
	TotalWorkers    int64         `json:"total_workers"`
	TasksSubmitted  int64         `json:"tasks_submitted"`
	TasksCompleted  int64         `json:"tasks_completed"`
	TasksFailed     int64         `json:"tasks_failed"`
	TasksPending    int           `json:"tasks_pending"`
	QueueCapacity   int           `json:"queue_capacity"`
	QueueUtilization float64      `json:"queue_utilization"`
	AvgWaitTime     time.Duration `json:"avg_wait_time"`
	AvgExecTime     time.Duration `json:"avg_exec_time"`
}

// Stats returns current pool statistics.
func (p *OptimizedPool) Stats() OptimizedPoolStats {
	submitted := atomic.LoadInt64(&p.tasksSubmitted)
	completed := atomic.LoadInt64(&p.tasksCompleted)
	totalWait := atomic.LoadInt64(&p.totalWaitTime)
	totalExec := atomic.LoadInt64(&p.totalExecTime)

	var avgWait, avgExec time.Duration
	if submitted > 0 {
		avgWait = time.Duration(totalWait / submitted)
	}
	if completed > 0 {
		avgExec = time.Duration(totalExec / completed)
	}

	queueLen := len(p.tasks)
	queueCap := cap(p.tasks)

	return OptimizedPoolStats{
		ActiveWorkers:    atomic.LoadInt64(&p.activeWorkers),
		TotalWorkers:     atomic.LoadInt64(&p.totalWorkers),
		TasksSubmitted:   submitted,
		TasksCompleted:   completed,
		TasksFailed:      atomic.LoadInt64(&p.tasksFailed),
		TasksPending:     queueLen,
		QueueCapacity:    queueCap,
		QueueUtilization: float64(queueLen) / float64(queueCap),
		AvgWaitTime:      avgWait,
		AvgExecTime:      avgExec,
	}
}

// Shutdown gracefully shuts down the pool.
func (p *OptimizedPool) Shutdown() {
	if !atomic.CompareAndSwapInt32(&p.closed, 0, 1) {
		return
	}

	close(p.tasks)
	p.cancel()
	p.workerWg.Wait()
	close(p.results)
}

// ShutdownWithTimeout shuts down with a timeout.
func (p *OptimizedPool) ShutdownWithTimeout(timeout time.Duration) error {
	if !atomic.CompareAndSwapInt32(&p.closed, 0, 1) {
		return nil
	}

	close(p.tasks)

	done := make(chan struct{})
	go func() {
		p.workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.cancel()
		close(p.results)
		return nil
	case <-time.After(timeout):
		p.cancel()
		close(p.results)
		return context.DeadlineExceeded
	}
}
