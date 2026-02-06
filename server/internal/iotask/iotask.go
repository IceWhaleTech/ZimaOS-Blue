// Package iotask provides IO task management with progress tracking and rate limiting.
package iotask

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// Common errors
var (
	ErrTaskNotFound   = errors.New("task not found")
	ErrTaskCancelled  = errors.New("task cancelled")
	ErrManagerClosed  = errors.New("manager is closed")
	ErrInvalidPath    = errors.New("invalid path")
)

// TaskType represents the type of IO task.
type TaskType string

const (
	TaskTypeCopy   TaskType = "copy"
	TaskTypeMove   TaskType = "move"
	TaskTypeDelete TaskType = "delete"
)

// TaskStatus represents the status of an IO task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Task represents an IO task.
type Task struct {
	ID          string            `json:"id"`
	Type        TaskType          `json:"type"`
	Source      string            `json:"source"`
	Destination string            `json:"destination,omitempty"`
	Status      TaskStatus        `json:"status"`
	Progress    Progress          `json:"progress"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`

	// Internal
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// Progress represents task progress.
type Progress struct {
	TotalBytes     int64   `json:"total_bytes"`
	ProcessedBytes int64   `json:"processed_bytes"`
	TotalFiles     int64   `json:"total_files"`
	ProcessedFiles int64   `json:"processed_files"`
	CurrentFile    string  `json:"current_file,omitempty"`
	Percentage     float64 `json:"percentage"`
	BytesPerSecond int64   `json:"bytes_per_second"`
}

// Config holds IO task manager configuration.
type Config struct {
	// MaxConcurrent is the maximum number of concurrent IO tasks.
	MaxConcurrent int `yaml:"max_concurrent" mapstructure:"max_concurrent"`

	// BufferSize is the buffer size for file operations.
	BufferSize int `yaml:"buffer_size" mapstructure:"buffer_size"`

	// RateLimitBytesPerSec limits the IO rate (0 = unlimited).
	RateLimitBytesPerSec int64 `yaml:"rate_limit_bytes_per_sec" mapstructure:"rate_limit_bytes_per_sec"`

	// Enabled enables the IO task manager.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		MaxConcurrent:        4,
		BufferSize:           32 * 1024, // 32KB
		RateLimitBytesPerSec: 0,         // Unlimited
		Enabled:              true,
	}
}

// Manager manages IO tasks.
type Manager struct {
	config  Config
	tasks   map[string]*Task
	mu      sync.RWMutex
	limiter *rate.Limiter
	sem     chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	closed  bool
}

// New creates a new IO task manager.
func New(config Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config: config,
		tasks:  make(map[string]*Task),
		sem:    make(chan struct{}, config.MaxConcurrent),
		ctx:    ctx,
		cancel: cancel,
	}

	// Set up rate limiter if configured
	if config.RateLimitBytesPerSec > 0 {
		m.limiter = rate.NewLimiter(rate.Limit(config.RateLimitBytesPerSec), int(config.RateLimitBytesPerSec))
	}

	return m
}

// Copy creates a copy task.
func (m *Manager) Copy(src, dst string, opts ...TaskOption) (*Task, error) {
	return m.createTask(TaskTypeCopy, src, dst, opts...)
}

// Move creates a move task.
func (m *Manager) Move(src, dst string, opts ...TaskOption) (*Task, error) {
	return m.createTask(TaskTypeMove, src, dst, opts...)
}

// Delete creates a delete task.
func (m *Manager) Delete(path string, opts ...TaskOption) (*Task, error) {
	return m.createTask(TaskTypeDelete, path, "", opts...)
}

func (m *Manager) createTask(taskType TaskType, src, dst string, opts ...TaskOption) (*Task, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, ErrManagerClosed
	}

	taskCtx, taskCancel := context.WithCancel(m.ctx)

	task := &Task{
		ID:          uuid.New().String(),
		Type:        taskType,
		Source:      src,
		Destination: dst,
		Status:      TaskStatusPending,
		CreatedAt:   time.Now(),
		Metadata:    make(map[string]string),
		cancel:      taskCancel,
	}

	for _, opt := range opts {
		opt(task)
	}

	m.tasks[task.ID] = task
	m.mu.Unlock()

	// Start task execution
	m.wg.Add(1)
	go m.executeTask(taskCtx, task)

	return task, nil
}

// Cancel cancels a task.
func (m *Manager) Cancel(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	if task.Status == TaskStatusPending || task.Status == TaskStatusRunning {
		task.cancel()
		task.Status = TaskStatusCancelled
		now := time.Now()
		task.CompletedAt = &now
	}

	return nil
}

// GetTask returns a task by ID.
func (m *Manager) GetTask(taskID string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	return task, nil
}

// ListTasks returns all tasks.
func (m *Manager) ListTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// Stats returns manager statistics.
func (m *Manager) Stats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var pending, running, completed, failed, cancelled int
	for _, task := range m.tasks {
		task.mu.RLock()
		switch task.Status {
		case TaskStatusPending:
			pending++
		case TaskStatusRunning:
			running++
		case TaskStatusCompleted:
			completed++
		case TaskStatusFailed:
			failed++
		case TaskStatusCancelled:
			cancelled++
		}
		task.mu.RUnlock()
	}

	return ManagerStats{
		TotalTasks:     len(m.tasks),
		PendingTasks:   pending,
		RunningTasks:   running,
		CompletedTasks: completed,
		FailedTasks:    failed,
		CancelledTasks: cancelled,
		MaxConcurrent:  m.config.MaxConcurrent,
	}
}

// ManagerStats holds manager statistics.
type ManagerStats struct {
	TotalTasks     int `json:"total_tasks"`
	PendingTasks   int `json:"pending_tasks"`
	RunningTasks   int `json:"running_tasks"`
	CompletedTasks int `json:"completed_tasks"`
	FailedTasks    int `json:"failed_tasks"`
	CancelledTasks int `json:"cancelled_tasks"`
	MaxConcurrent  int `json:"max_concurrent"`
}

// Close closes the manager and waits for all tasks to complete.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()

	m.cancel()
	m.wg.Wait()
}

func (m *Manager) executeTask(ctx context.Context, task *Task) {
	defer m.wg.Done()

	// Acquire semaphore
	select {
	case m.sem <- struct{}{}:
		defer func() { <-m.sem }()
	case <-ctx.Done():
		task.mu.Lock()
		task.Status = TaskStatusCancelled
		task.mu.Unlock()
		return
	}

	// Update status to running
	task.mu.Lock()
	task.Status = TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now
	task.mu.Unlock()

	var err error
	switch task.Type {
	case TaskTypeCopy:
		err = m.executeCopy(ctx, task)
	case TaskTypeMove:
		err = m.executeMove(ctx, task)
	case TaskTypeDelete:
		err = m.executeDelete(ctx, task)
	}

	// Update final status
	task.mu.Lock()
	completedAt := time.Now()
	task.CompletedAt = &completedAt

	if ctx.Err() != nil {
		task.Status = TaskStatusCancelled
	} else if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
	} else {
		task.Status = TaskStatusCompleted
		task.Progress.Percentage = 100
	}
	task.mu.Unlock()
}

func (m *Manager) executeCopy(ctx context.Context, task *Task) error {
	// Calculate total size first
	totalSize, totalFiles, err := m.calculateSize(task.Source)
	if err != nil {
		return err
	}

	task.mu.Lock()
	task.Progress.TotalBytes = totalSize
	task.Progress.TotalFiles = totalFiles
	task.mu.Unlock()

	return m.copyPath(ctx, task, task.Source, task.Destination)
}

func (m *Manager) executeMove(ctx context.Context, task *Task) error {
	// Try rename first (fast path for same filesystem)
	err := os.Rename(task.Source, task.Destination)
	if err == nil {
		return nil
	}

	// Fall back to copy + delete
	if err := m.executeCopy(ctx, task); err != nil {
		return err
	}

	return os.RemoveAll(task.Source)
}

func (m *Manager) executeDelete(ctx context.Context, task *Task) error {
	// Calculate total size first
	totalSize, totalFiles, err := m.calculateSize(task.Source)
	if err != nil {
		return err
	}

	task.mu.Lock()
	task.Progress.TotalBytes = totalSize
	task.Progress.TotalFiles = totalFiles
	task.mu.Unlock()

	return m.deletePath(ctx, task, task.Source)
}

func (m *Manager) calculateSize(path string) (int64, int64, error) {
	var totalSize int64
	var totalFiles int64

	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, err
	}

	// If it's a single file, return its size
	if !info.IsDir() {
		return info.Size(), 1, nil
	}

	// Use queue-based iteration instead of recursive walk
	queue := []string{path}

	for len(queue) > 0 {
		currentDir := queue[0]
		queue = queue[1:]

		entries, err := os.ReadDir(currentDir)
		if err != nil {
			return totalSize, totalFiles, err
		}

		for _, entry := range entries {
			entryPath := filepath.Join(currentDir, entry.Name())

			if entry.IsDir() {
				queue = append(queue, entryPath)
				continue
			}

			info, err := entry.Info()
			if err != nil {
				return totalSize, totalFiles, err
			}
			totalSize += info.Size()
			totalFiles++
		}
	}

	return totalSize, totalFiles, nil
}

func (m *Manager) copyPath(ctx context.Context, task *Task, src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return m.copyDir(ctx, task, src, dst, info.Mode())
	}

	return m.copyFile(ctx, task, src, dst, info.Mode())
}

func (m *Manager) copyDir(ctx context.Context, task *Task, src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(dst, mode); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if err := m.copyPath(ctx, task, srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) copyFile(ctx context.Context, task *Task, src, dst string, mode os.FileMode) error {
	// Update current file
	task.mu.Lock()
	task.Progress.CurrentFile = src
	task.mu.Unlock()

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy with progress tracking
	buf := make([]byte, m.config.BufferSize)
	startTime := time.Now()
	var copied int64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Rate limiting
		if m.limiter != nil {
			if err := m.limiter.WaitN(ctx, len(buf)); err != nil {
				return err
			}
		}

		n, err := srcFile.Read(buf)
		if n > 0 {
			written, writeErr := dstFile.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			copied += int64(written)

			// Update progress
			task.mu.Lock()
			task.Progress.ProcessedBytes += int64(written)
			elapsed := time.Since(startTime).Seconds()
			if elapsed > 0 {
				task.Progress.BytesPerSecond = int64(float64(task.Progress.ProcessedBytes) / elapsed)
			}
			if task.Progress.TotalBytes > 0 {
				task.Progress.Percentage = float64(task.Progress.ProcessedBytes) / float64(task.Progress.TotalBytes) * 100
			}
			task.mu.Unlock()
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// Update processed files count
	task.mu.Lock()
	task.Progress.ProcessedFiles++
	task.mu.Unlock()

	return nil
}

func (m *Manager) deletePath(ctx context.Context, task *Task, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return m.deleteDir(ctx, task, path)
	}

	return m.deleteFile(ctx, task, path, info.Size())
}

func (m *Manager) deleteDir(ctx context.Context, task *Task, path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		entryPath := filepath.Join(path, entry.Name())
		if err := m.deletePath(ctx, task, entryPath); err != nil {
			return err
		}
	}

	return os.Remove(path)
}

func (m *Manager) deleteFile(ctx context.Context, task *Task, path string, size int64) error {
	task.mu.Lock()
	task.Progress.CurrentFile = path
	task.mu.Unlock()

	if err := os.Remove(path); err != nil {
		return err
	}

	task.mu.Lock()
	task.Progress.ProcessedBytes += size
	task.Progress.ProcessedFiles++
	if task.Progress.TotalBytes > 0 {
		task.Progress.Percentage = float64(task.Progress.ProcessedBytes) / float64(task.Progress.TotalBytes) * 100
	}
	task.mu.Unlock()

	return nil
}

// TaskOption is a function that configures a task.
type TaskOption func(*Task)

// WithMetadata sets task metadata.
func WithMetadata(key, value string) TaskOption {
	return func(t *Task) {
		t.Metadata[key] = value
	}
}

// ProgressReader wraps an io.Reader to track progress.
type ProgressReader struct {
	reader    io.Reader
	total     int64
	processed atomic.Int64
	onUpdate  func(processed, total int64)
}

// NewProgressReader creates a new progress reader.
func NewProgressReader(r io.Reader, total int64, onUpdate func(processed, total int64)) *ProgressReader {
	return &ProgressReader{
		reader:   r,
		total:    total,
		onUpdate: onUpdate,
	}
}

// Read implements io.Reader.
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		processed := pr.processed.Add(int64(n))
		if pr.onUpdate != nil {
			pr.onUpdate(processed, pr.total)
		}
	}
	return n, err
}

// Processed returns the number of bytes processed.
func (pr *ProgressReader) Processed() int64 {
	return pr.processed.Load()
}

// Percentage returns the progress percentage.
func (pr *ProgressReader) Percentage() float64 {
	if pr.total == 0 {
		return 0
	}
	return float64(pr.processed.Load()) / float64(pr.total) * 100
}
