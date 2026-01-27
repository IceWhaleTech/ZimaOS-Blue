// Package cron provides scheduled task management.
package cron

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// JobStatus represents the status of a cron job.
type JobStatus string

const (
	// StatusActive indicates the job is active and scheduled.
	StatusActive JobStatus = "active"
	// StatusPaused indicates the job is paused.
	StatusPaused JobStatus = "paused"
	// StatusRunning indicates the job is currently running.
	StatusRunning JobStatus = "running"
)

// Job represents a scheduled job.
type Job struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Schedule    string                 `json:"schedule"` // Cron expression
	Handler     string                 `json:"handler"`  // Handler name
	Payload     map[string]interface{} `json:"payload,omitempty"`
	Status      JobStatus              `json:"status"`
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	LastRunAt   *time.Time             `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time             `json:"next_run_at,omitempty"`
	RunCount    int64                  `json:"run_count"`
	FailCount   int64                  `json:"fail_count"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`

	entryID cron.EntryID
}

// JobExecution represents a single job execution.
type JobExecution struct {
	ID        string        `json:"id"`
	JobID     string        `json:"job_id"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
	Status    string        `json:"status"` // running, completed, failed
	Error     string        `json:"error,omitempty"`
	Result    interface{}   `json:"result,omitempty"`
}

// JobHandler is a function that handles job execution.
type JobHandler func(ctx context.Context, job *Job) (interface{}, error)

// Config contains cron service configuration.
type Config struct {
	// Enabled indicates if cron is enabled.
	Enabled bool `mapstructure:"enabled"`
	// MaxConcurrentJobs is the maximum number of concurrent jobs.
	MaxConcurrentJobs int `mapstructure:"max_concurrent_jobs"`
	// JobTimeoutSeconds is the default job timeout.
	JobTimeoutSeconds int `mapstructure:"job_timeout_seconds"`
	// ExecutionRetentionHours is how long to keep execution history.
	ExecutionRetentionHours int `mapstructure:"execution_retention_hours"`
	// MaxExecutionsPerJob is the maximum executions to keep per job.
	MaxExecutionsPerJob int `mapstructure:"max_executions_per_job"`
}

// DefaultConfig returns the default cron configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:                 true,
		MaxConcurrentJobs:       10,
		JobTimeoutSeconds:       300,
		ExecutionRetentionHours: 24,
		MaxExecutionsPerJob:     50,
	}
}

// Service manages cron jobs.
type Service struct {
	config     Config
	logger     *zap.Logger
	cron       *cron.Cron
	jobs       map[string]*Job
	executions map[string][]*JobExecution
	handlers   map[string]JobHandler
	mu         sync.RWMutex

	// Semaphore for limiting concurrent jobs
	sem chan struct{}

	ctx    context.Context
	cancel context.CancelFunc
}

// NewService creates a new cron service.
func NewService(cfg Config, logger *zap.Logger) *Service {
	ctx, cancel := context.WithCancel(context.Background())

	// Create cron with seconds support
	c := cron.New(cron.WithSeconds(), cron.WithChain(
		cron.Recover(cron.DefaultLogger),
	))

	return &Service{
		config:     cfg,
		logger:     logger.With(zap.String("component", "cron")),
		cron:       c,
		jobs:       make(map[string]*Job),
		executions: make(map[string][]*JobExecution),
		handlers:   make(map[string]JobHandler),
		sem:        make(chan struct{}, cfg.MaxConcurrentJobs),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// RegisterHandler registers a job handler.
func (s *Service) RegisterHandler(name string, handler JobHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[name] = handler
	s.logger.Debug("job handler registered", zap.String("name", name))
}

// Start starts the cron service.
func (s *Service) Start() error {
	if !s.config.Enabled {
		s.logger.Info("cron service is disabled")
		return nil
	}

	s.cron.Start()
	s.logger.Info("cron service started")
	return nil
}

// Stop stops the cron service.
func (s *Service) Stop(ctx context.Context) error {
	s.cancel()

	// Stop cron and wait for running jobs
	stopCtx := s.cron.Stop()

	select {
	case <-stopCtx.Done():
		s.logger.Info("cron service stopped")
	case <-ctx.Done():
		s.logger.Warn("timeout waiting for cron to stop")
		return ctx.Err()
	}

	return nil
}

// Create creates a new cron job.
func (s *Service) Create(name, description, schedule, handler string, payload map[string]interface{}) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate schedule
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(schedule)
	if err != nil {
		return nil, fmt.Errorf("invalid cron schedule: %w", err)
	}

	// Check if handler exists
	if _, exists := s.handlers[handler]; !exists {
		return nil, fmt.Errorf("handler %s not found", handler)
	}

	// Generate unique ID using crypto/rand
	idBytes := make([]byte, 8)
	rand.Read(idBytes)
	id := fmt.Sprintf("job_%x", idBytes)

	job := &Job{
		ID:          id,
		Name:        name,
		Description: description,
		Schedule:    schedule,
		Handler:     handler,
		Payload:     payload,
		Status:      StatusActive,
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	// Schedule the job
	entryID, err := s.cron.AddFunc(schedule, func() {
		s.executeJob(job.ID)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to schedule job: %w", err)
	}

	job.entryID = entryID

	// Calculate next run time
	entry := s.cron.Entry(entryID)
	if !entry.Next.IsZero() {
		job.NextRunAt = &entry.Next
	}

	s.jobs[id] = job
	s.executions[id] = make([]*JobExecution, 0)

	s.logger.Info("cron job created",
		zap.String("id", id),
		zap.String("name", name),
		zap.String("schedule", schedule))

	return job, nil
}

// Get returns a job by ID.
func (s *Service) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, exists := s.jobs[id]
	if exists {
		copy := *job
		return &copy, true
	}
	return nil, false
}

// List returns all jobs.
func (s *Service) List() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		copy := *job
		result = append(result, &copy)
	}
	return result
}

// Update updates a job.
func (s *Service) Update(id, name, description, schedule string, payload map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return fmt.Errorf("job %s not found", id)
	}

	// If schedule changed, reschedule
	if schedule != job.Schedule {
		parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		_, err := parser.Parse(schedule)
		if err != nil {
			return fmt.Errorf("invalid cron schedule: %w", err)
		}

		// Remove old schedule
		s.cron.Remove(job.entryID)

		// Add new schedule
		entryID, err := s.cron.AddFunc(schedule, func() {
			s.executeJob(job.ID)
		})
		if err != nil {
			return fmt.Errorf("failed to reschedule job: %w", err)
		}

		job.entryID = entryID
		job.Schedule = schedule

		// Update next run time
		entry := s.cron.Entry(entryID)
		if !entry.Next.IsZero() {
			job.NextRunAt = &entry.Next
		}
	}

	job.Name = name
	job.Description = description
	job.Payload = payload
	job.UpdatedAt = time.Now()

	return nil
}

// Delete deletes a job.
func (s *Service) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return fmt.Errorf("job %s not found", id)
	}

	s.cron.Remove(job.entryID)
	delete(s.jobs, id)
	delete(s.executions, id)

	s.logger.Info("cron job deleted", zap.String("id", id))
	return nil
}

// Enable enables a job.
func (s *Service) Enable(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return fmt.Errorf("job %s not found", id)
	}

	if job.Enabled {
		return nil
	}

	// Reschedule
	entryID, err := s.cron.AddFunc(job.Schedule, func() {
		s.executeJob(job.ID)
	})
	if err != nil {
		return fmt.Errorf("failed to enable job: %w", err)
	}

	job.entryID = entryID
	job.Enabled = true
	job.Status = StatusActive
	job.UpdatedAt = time.Now()

	// Update next run time
	entry := s.cron.Entry(entryID)
	if !entry.Next.IsZero() {
		job.NextRunAt = &entry.Next
	}

	return nil
}

// Disable disables a job.
func (s *Service) Disable(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return fmt.Errorf("job %s not found", id)
	}

	if !job.Enabled {
		return nil
	}

	s.cron.Remove(job.entryID)
	job.Enabled = false
	job.Status = StatusPaused
	job.NextRunAt = nil
	job.UpdatedAt = time.Now()

	return nil
}

// Trigger manually triggers a job.
func (s *Service) Trigger(id string) error {
	s.mu.RLock()
	_, exists := s.jobs[id]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job %s not found", id)
	}

	go s.executeJob(id)
	return nil
}

// executeJob executes a job.
func (s *Service) executeJob(id string) {
	// Acquire semaphore
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-s.ctx.Done():
		return
	}

	s.mu.Lock()
	job, exists := s.jobs[id]
	if !exists || !job.Enabled {
		s.mu.Unlock()
		return
	}

	handler, hasHandler := s.handlers[job.Handler]
	if !hasHandler {
		s.mu.Unlock()
		s.logger.Error("job handler not found",
			zap.String("job_id", id),
			zap.String("handler", job.Handler))
		return
	}

	// Create execution record
	execID := fmt.Sprintf("exec_%d", time.Now().UnixNano())
	exec := &JobExecution{
		ID:        execID,
		JobID:     id,
		StartedAt: time.Now(),
		Status:    "running",
	}

	job.Status = StatusRunning
	now := time.Now()
	job.LastRunAt = &now
	job.RunCount++

	s.executions[id] = append(s.executions[id], exec)

	// Trim old executions
	if len(s.executions[id]) > s.config.MaxExecutionsPerJob {
		s.executions[id] = s.executions[id][len(s.executions[id])-s.config.MaxExecutionsPerJob:]
	}
	s.mu.Unlock()

	// Execute with timeout
	ctx, cancel := context.WithTimeout(s.ctx, time.Duration(s.config.JobTimeoutSeconds)*time.Second)
	defer cancel()

	s.logger.Debug("executing job",
		zap.String("job_id", id),
		zap.String("exec_id", execID))

	result, err := handler(ctx, job)

	// Update execution record
	s.mu.Lock()
	endedAt := time.Now()
	exec.EndedAt = &endedAt
	exec.Duration = endedAt.Sub(exec.StartedAt)

	if err != nil {
		exec.Status = "failed"
		exec.Error = err.Error()
		job.FailCount++
		s.logger.Error("job execution failed",
			zap.String("job_id", id),
			zap.String("exec_id", execID),
			zap.Error(err))
	} else {
		exec.Status = "completed"
		exec.Result = result
		s.logger.Debug("job execution completed",
			zap.String("job_id", id),
			zap.String("exec_id", execID),
			zap.Duration("duration", exec.Duration))
	}

	job.Status = StatusActive

	// Update next run time
	entry := s.cron.Entry(job.entryID)
	if !entry.Next.IsZero() {
		job.NextRunAt = &entry.Next
	}
	s.mu.Unlock()
}

// GetExecutions returns executions for a job.
func (s *Service) GetExecutions(jobID string, limit int) ([]*JobExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	executions, exists := s.executions[jobID]
	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	if limit <= 0 || limit > len(executions) {
		limit = len(executions)
	}

	// Return most recent executions
	result := make([]*JobExecution, limit)
	for i := 0; i < limit; i++ {
		idx := len(executions) - limit + i
		copy := *executions[idx]
		result[i] = &copy
	}

	return result, nil
}
