// Package persistence provides task persistence using SQLite.
package persistence

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	z "github.com/IceWhaleTech/zorm"
	_ "github.com/mattn/go-sqlite3"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Common errors
var (
	ErrTaskNotFound = errors.New("task not found")
	ErrStoreClosed  = errors.New("store is closed")
)

// TaskRecord represents a persisted task.
type TaskRecord struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Priority     int               `json:"priority"`
	Status       string            `json:"status"`
	ScheduledAt  time.Time         `json:"scheduled_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
	Error        string            `json:"error,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	RetryCount   int               `json:"retry_count"`
	MaxRetries   int               `json:"max_retries"`
	Timeout      int64             `json:"timeout_ns,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	HandlerName  string            `json:"handler_name"`
	HandlerData  []byte            `json:"handler_data,omitempty"`
}

// taskRow represents a task row for zorm scanning.
type taskRow struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Priority     int     `json:"priority"`
	Status       string  `json:"status"`
	ScheduledAt  string  `json:"scheduled_at"`
	StartedAt    *string `json:"started_at"`
	CompletedAt  *string `json:"completed_at"`
	Error        string  `json:"error"`
	Metadata     string  `json:"metadata"`
	RetryCount   int     `json:"retry_count"`
	MaxRetries   int     `json:"max_retries"`
	Timeout      int64   `json:"timeout_ns"`
	Dependencies string  `json:"dependencies"`
	HandlerName  string  `json:"handler_name"`
	HandlerData  []byte  `json:"handler_data"`
}

// parseTaskTime parses time strings in multiple formats.
func parseTaskTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02T15:04:05Z", s)
	}
	return t
}

// rowToTask converts a taskRow to a TaskRecord.
func rowToTask(row taskRow) (*TaskRecord, error) {
	task := &TaskRecord{
		ID:          row.ID,
		Name:        row.Name,
		Priority:    row.Priority,
		Status:      row.Status,
		ScheduledAt: parseTaskTime(row.ScheduledAt),
		Error:       row.Error,
		RetryCount:  row.RetryCount,
		MaxRetries:  row.MaxRetries,
		Timeout:     row.Timeout,
		HandlerName: row.HandlerName,
		HandlerData: row.HandlerData,
	}

	if row.StartedAt != nil {
		t := parseTaskTime(*row.StartedAt)
		if !t.IsZero() {
			task.StartedAt = &t
		}
	}
	if row.CompletedAt != nil {
		t := parseTaskTime(*row.CompletedAt)
		if !t.IsZero() {
			task.CompletedAt = &t
		}
	}

	if row.Metadata != "" {
		if err := json.Unmarshal([]byte(row.Metadata), &task.Metadata); err != nil {
			return nil, err
		}
	}
	if row.Dependencies != "" {
		if err := json.Unmarshal([]byte(row.Dependencies), &task.Dependencies); err != nil {
			return nil, err
		}
	}

	return task, nil
}

// Config holds persistence configuration.
type Config struct {
	// DBPath is the path to the SQLite database file.
	DBPath string `yaml:"db_path" yaml:"db_path"`

	// Enabled enables task persistence.
	Enabled bool `yaml:"enabled" yaml:"enabled"`

	// RetentionDays is the number of days to keep completed tasks.
	RetentionDays int `yaml:"retention_days" yaml:"retention_days"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		DBPath:        "./data/scheduler.db",
		Enabled:       true,
		RetentionDays: 7,
	}
}

// Store provides task persistence.
type Store struct {
	db     *sql.DB
	config Config
	mu     sync.RWMutex
	closed bool
}

// NewStore creates a new persistence store.
func NewStore(config Config) (*Store, error) {
	db, err := sql.Open("sqlite3", config.DBPath)
	if err != nil {
		return nil, err
	}

	// Connection pool limits
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, err
	}

	store := &Store{
		db:     db,
		config: config,
	}

	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) table() *z.ZormTable {
	return z.Table(s.db, "tasks")
}

// migrate creates the necessary tables.
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		priority INTEGER NOT NULL DEFAULT 1,
		status TEXT NOT NULL DEFAULT 'pending',
		scheduled_at DATETIME NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		error TEXT,
		metadata TEXT,
		retry_count INTEGER NOT NULL DEFAULT 0,
		max_retries INTEGER NOT NULL DEFAULT 3,
		timeout_ns INTEGER,
		dependencies TEXT,
		handler_name TEXT NOT NULL,
		handler_data BLOB,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_scheduled_at ON tasks(scheduled_at);
	CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority DESC);

	CREATE TRIGGER IF NOT EXISTS update_tasks_timestamp
	AFTER UPDATE ON tasks
	BEGIN
		UPDATE tasks SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
	END;
	`

	_, err := s.db.Exec(schema)
	return err
}

// Save saves a task to the store.
func (s *Store) Save(task *TaskRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	metadataJSON, err := json.Marshal(task.Metadata)
	if err != nil {
		return err
	}

	depsJSON, err := json.Marshal(task.Dependencies)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"id":           task.ID,
		"name":         task.Name,
		"priority":     task.Priority,
		"status":       task.Status,
		"scheduled_at": task.ScheduledAt,
		"started_at":   task.StartedAt,
		"completed_at": task.CompletedAt,
		"error":        task.Error,
		"metadata":     string(metadataJSON),
		"retry_count":  task.RetryCount,
		"max_retries":  task.MaxRetries,
		"timeout_ns":   task.Timeout,
		"dependencies": string(depsJSON),
		"handler_name": task.HandlerName,
		"handler_data": task.HandlerData,
	}

	_, err = s.table().Insert(data,
		z.OnConflictDoUpdateSet(
			[]string{"id"},
			[]string{"name", "priority", "status", "scheduled_at", "started_at", "completed_at",
				"error", "metadata", "retry_count", "max_retries", "timeout_ns", "dependencies",
				"handler_name", "handler_data"},
		),
	)

	return err
}

// Get retrieves a task by ID.
func (s *Store) Get(id string) (*TaskRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	var rows []taskRow
	_, err := s.table().Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, ErrTaskNotFound
	}

	return rowToTask(rows[0])
}

// Delete removes a task from the store.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	n, err := s.table().Delete(z.Where(z.Eq("id", id)))
	if err != nil {
		return err
	}

	if n == 0 {
		return ErrTaskNotFound
	}

	return nil
}

// List returns tasks matching the given filter.
func (s *Store) List(filter TaskFilter) ([]*TaskRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	var conds []interface{}

	if filter.Status != "" {
		conds = append(conds, z.Eq("status", filter.Status))
	}
	if filter.HandlerName != "" {
		conds = append(conds, z.Eq("handler_name", filter.HandlerName))
	}
	if !filter.Since.IsZero() {
		conds = append(conds, z.Gte("scheduled_at", filter.Since))
	}
	if !filter.Until.IsZero() {
		conds = append(conds, z.Lte("scheduled_at", filter.Until))
	}

	opts := []z.ZormItem{z.OrderBy("priority DESC", "scheduled_at ASC")}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	if filter.Limit > 0 {
		if filter.Offset > 0 {
			opts = append(opts, z.Limit(filter.Limit, filter.Offset))
		} else {
			opts = append(opts, z.Limit(filter.Limit))
		}
	} else if filter.Offset > 0 {
		opts = append(opts, z.Limit(-1, filter.Offset))
	}

	var rows []taskRow
	_, err := s.table().Select(&rows, opts...)
	if err != nil {
		return nil, err
	}

	tasks := make([]*TaskRecord, 0, len(rows))
	for _, row := range rows {
		task, err := rowToTask(row)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// TaskFilter defines criteria for listing tasks.
type TaskFilter struct {
	Status      string
	HandlerName string
	Since       time.Time
	Until       time.Time
	Limit       int
	Offset      int
}

// GetPendingTasks returns all pending tasks ordered by priority.
func (s *Store) GetPendingTasks() ([]*TaskRecord, error) {
	return s.List(TaskFilter{Status: "pending"})
}

// UpdateStatus updates the status of a task.
func (s *Store) UpdateStatus(id, status string, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	var errStr string
	if err != nil {
		errStr = err.Error()
	}

	now := timeutil.NowTime()
	var data map[string]interface{}

	switch status {
	case "running":
		data = map[string]interface{}{
			"status":     status,
			"started_at": now,
			"error":      errStr,
		}
	case "completed", "failed", "cancelled":
		data = map[string]interface{}{
			"status":       status,
			"completed_at": now,
			"error":        errStr,
		}
	default:
		data = map[string]interface{}{
			"status": status,
			"error":  errStr,
		}
	}

	n, execErr := s.table().Update(data, z.Where(z.Eq("id", id)))
	if execErr != nil {
		return execErr
	}

	if n == 0 {
		return ErrTaskNotFound
	}

	return nil
}

// IncrementRetry increments the retry count for a task.
func (s *Store) IncrementRetry(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	result, err := s.db.Exec(
		"UPDATE tasks SET retry_count = retry_count + 1, status = 'pending', started_at = NULL WHERE id = ?",
		id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrTaskNotFound
	}

	return nil
}

// Cleanup removes old completed tasks.
func (s *Store) Cleanup() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, ErrStoreClosed
	}

	cutoff := timeutil.NowTime().AddDate(0, 0, -s.config.RetentionDays)

	n, err := s.table().Delete(
		z.Where(
			z.In("status", "completed", "failed", "cancelled"),
			z.Lt("completed_at", cutoff),
		),
	)
	return int64(n), err
}

// Stats returns store statistics.
func (s *Store) Stats() (StoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return StoreStats{}, ErrStoreClosed
	}

	var stats StoreStats

	err := s.db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&stats.TotalTasks)
	if err != nil {
		return stats, err
	}

	err = s.db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'pending'").Scan(&stats.PendingTasks)
	if err != nil {
		return stats, err
	}

	err = s.db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'running'").Scan(&stats.RunningTasks)
	if err != nil {
		return stats, err
	}

	err = s.db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'completed'").Scan(&stats.CompletedTasks)
	if err != nil {
		return stats, err
	}

	err = s.db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'failed'").Scan(&stats.FailedTasks)
	if err != nil {
		return stats, err
	}

	err = s.db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'cancelled'").Scan(&stats.CancelledTasks)
	if err != nil {
		return stats, err
	}

	return stats, nil
}

// StoreStats holds store statistics.
type StoreStats struct {
	TotalTasks     int `json:"total_tasks"`
	PendingTasks   int `json:"pending_tasks"`
	RunningTasks   int `json:"running_tasks"`
	CompletedTasks int `json:"completed_tasks"`
	FailedTasks    int `json:"failed_tasks"`
	CancelledTasks int `json:"cancelled_tasks"`
}

// Close closes the store.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	return s.db.Close()
}
