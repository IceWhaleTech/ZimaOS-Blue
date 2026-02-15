// Package persistence provides task persistence using SQLite.
package persistence

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

// Config holds persistence configuration.
type Config struct {
	// DBPath is the path to the SQLite database file.
	DBPath string `yaml:"db_path" mapstructure:"db_path"`

	// Enabled enables task persistence.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`

	// RetentionDays is the number of days to keep completed tasks.
	RetentionDays int `yaml:"retention_days" mapstructure:"retention_days"`
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

	query := `
	INSERT INTO tasks (
		id, name, priority, status, scheduled_at, started_at, completed_at,
		error, metadata, retry_count, max_retries, timeout_ns, dependencies,
		handler_name, handler_data
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		priority = excluded.priority,
		status = excluded.status,
		scheduled_at = excluded.scheduled_at,
		started_at = excluded.started_at,
		completed_at = excluded.completed_at,
		error = excluded.error,
		metadata = excluded.metadata,
		retry_count = excluded.retry_count,
		max_retries = excluded.max_retries,
		timeout_ns = excluded.timeout_ns,
		dependencies = excluded.dependencies,
		handler_name = excluded.handler_name,
		handler_data = excluded.handler_data
	`

	_, err = s.db.Exec(query,
		task.ID, task.Name, task.Priority, task.Status, task.ScheduledAt,
		task.StartedAt, task.CompletedAt, task.Error, string(metadataJSON),
		task.RetryCount, task.MaxRetries, task.Timeout, string(depsJSON),
		task.HandlerName, task.HandlerData,
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

	query := `
	SELECT id, name, priority, status, scheduled_at, started_at, completed_at,
		error, metadata, retry_count, max_retries, timeout_ns, dependencies,
		handler_name, handler_data
	FROM tasks WHERE id = ?
	`

	var task TaskRecord
	var metadataJSON, depsJSON string
	var startedAt, completedAt sql.NullTime

	err := s.db.QueryRow(query, id).Scan(
		&task.ID, &task.Name, &task.Priority, &task.Status, &task.ScheduledAt,
		&startedAt, &completedAt, &task.Error, &metadataJSON,
		&task.RetryCount, &task.MaxRetries, &task.Timeout, &depsJSON,
		&task.HandlerName, &task.HandlerData,
	)

	if err == sql.ErrNoRows {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}

	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}

	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &task.Metadata); err != nil {
			return nil, err
		}
	}

	if depsJSON != "" {
		if err := json.Unmarshal([]byte(depsJSON), &task.Dependencies); err != nil {
			return nil, err
		}
	}

	return &task, nil
}

// Delete removes a task from the store.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	result, err := s.db.Exec("DELETE FROM tasks WHERE id = ?", id)
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

// List returns tasks matching the given filter.
func (s *Store) List(filter TaskFilter) ([]*TaskRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	query := `
	SELECT id, name, priority, status, scheduled_at, started_at, completed_at,
		error, metadata, retry_count, max_retries, timeout_ns, dependencies,
		handler_name, handler_data
	FROM tasks
	WHERE 1=1
	`

	args := []interface{}{}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}

	if filter.HandlerName != "" {
		query += " AND handler_name = ?"
		args = append(args, filter.HandlerName)
	}

	if !filter.Since.IsZero() {
		query += " AND scheduled_at >= ?"
		args = append(args, filter.Since)
	}

	if !filter.Until.IsZero() {
		query += " AND scheduled_at <= ?"
		args = append(args, filter.Until)
	}

	query += " ORDER BY priority DESC, scheduled_at ASC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*TaskRecord
	for rows.Next() {
		var task TaskRecord
		var metadataJSON, depsJSON string
		var startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&task.ID, &task.Name, &task.Priority, &task.Status, &task.ScheduledAt,
			&startedAt, &completedAt, &task.Error, &metadataJSON,
			&task.RetryCount, &task.MaxRetries, &task.Timeout, &depsJSON,
			&task.HandlerName, &task.HandlerData,
		)
		if err != nil {
			return nil, err
		}

		if startedAt.Valid {
			task.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}

		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &task.Metadata); err != nil {
				return nil, err
			}
		}

		if depsJSON != "" {
			if err := json.Unmarshal([]byte(depsJSON), &task.Dependencies); err != nil {
				return nil, err
			}
		}

		tasks = append(tasks, &task)
	}

	return tasks, rows.Err()
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

	var query string
	var args []interface{}

	switch status {
	case "running":
		query = "UPDATE tasks SET status = ?, started_at = ?, error = ? WHERE id = ?"
		args = []interface{}{status, time.Now(), errStr, id}
	case "completed", "failed", "cancelled":
		query = "UPDATE tasks SET status = ?, completed_at = ?, error = ? WHERE id = ?"
		args = []interface{}{status, time.Now(), errStr, id}
	default:
		query = "UPDATE tasks SET status = ?, error = ? WHERE id = ?"
		args = []interface{}{status, errStr, id}
	}

	result, execErr := s.db.Exec(query, args...)
	if execErr != nil {
		return execErr
	}

	rows, execErr := result.RowsAffected()
	if execErr != nil {
		return execErr
	}

	if rows == 0 {
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

	cutoff := time.Now().AddDate(0, 0, -s.config.RetentionDays)

	result, err := s.db.Exec(
		"DELETE FROM tasks WHERE status IN ('completed', 'failed', 'cancelled') AND completed_at < ?",
		cutoff,
	)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
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
