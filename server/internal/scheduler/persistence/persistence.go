// Package persistence provides task persistence using SQLite.
package persistence

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
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

// taskRow represents a task row for zorm scanning.
type taskRow struct {
	ID           string  `json:"id" zorm:"id"`
	Name         string  `json:"name" zorm:"name"`
	Priority     int     `json:"priority" zorm:"priority"`
	Status       string  `json:"status" zorm:"status"`
	ScheduledAt  string  `json:"scheduled_at" zorm:"scheduled_at"`
	StartedAt    *string `json:"started_at" zorm:"started_at"`
	CompletedAt  *string `json:"completed_at" zorm:"completed_at"`
	Error        string  `json:"error" zorm:"error"`
	Metadata     string  `json:"metadata" zorm:"metadata"`
	RetryCount   int     `json:"retry_count" zorm:"retry_count"`
	MaxRetries   int     `json:"max_retries" zorm:"max_retries"`
	Timeout      int64   `json:"timeout_ns" zorm:"timeout_ns"`
	Dependencies string  `json:"dependencies" zorm:"dependencies"`
	HandlerName  string  `json:"handler_name" zorm:"handler_name"`
	HandlerData  []byte  `json:"handler_data" zorm:"handler_data"`
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

const schedulerTasksTable = "scheduler_tasks"

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		DBPath:        "./data/blue.db",
		Enabled:       true,
		RetentionDays: 7,
	}
}

// Store provides task persistence.
type Store struct {
	db     *sql.DB
	readDB *sql.DB
	config Config
	mu     sync.RWMutex
	closed bool
}

// NewStore creates a new persistence store.
func NewStore(config Config) (*Store, error) {
	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(config.DBPath, config.DBPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(2)
		db.SetMaxIdleConns(1)
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return err
		}
		if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
			return err
		}
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return err
		}
		if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
			return err
		}
		if _, err := db.Exec("PRAGMA wal_autocheckpoint=1000"); err != nil {
			return err
		}

		store := &Store{db: db, config: config}
		return store.migrate()
	})
	if err != nil {
		return nil, err
	}

	readDB, readErr := openSchedulerReaderDB(config.DBPath)
	if readErr != nil || readDB == nil {
		readDB = db
	}

	return &Store{
		db:     db,
		readDB: readDB,
		config: config,
	}, nil
}

func (s *Store) table() *z.ZormTable {
	return z.Table(s.db, schedulerTasksTable)
}

func (s *Store) readTable() *z.ZormTable {
	return z.Table(s.reader(), schedulerTasksTable)
}

func (s *Store) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func openSchedulerReaderDB(dbPath string) (*sql.DB, error) {
	if dbPath == "" || dbPath == ":memory:" {
		return nil, nil
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := dbutil.OpenSQLiteWithRecovery(dsn, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(2)
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("set scheduler reader busy timeout: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// migrate creates the necessary tables.
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS scheduler_tasks (
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

	CREATE INDEX IF NOT EXISTS idx_scheduler_tasks_status ON scheduler_tasks(status);
	CREATE INDEX IF NOT EXISTS idx_scheduler_tasks_scheduled_at ON scheduler_tasks(scheduled_at);
	CREATE INDEX IF NOT EXISTS idx_scheduler_tasks_priority ON scheduler_tasks(priority DESC);

	CREATE TRIGGER IF NOT EXISTS update_scheduler_tasks_timestamp
	AFTER UPDATE ON scheduler_tasks
	BEGIN
		UPDATE scheduler_tasks SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
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
	_, err := s.readTable().Select(&rows,
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
	_, err := s.readTable().Select(&rows, opts...)
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
	var data z.V

	switch status {
	case "running":
		data = z.V{
			"status":     status,
			"started_at": now,
			"error":      errStr,
		}
	case "completed", "failed", "cancelled":
		data = z.V{
			"status":       status,
			"completed_at": now,
			"error":        errStr,
		}
	default:
		data = z.V{
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

	n, err := s.table().Update(z.V{
		"retry_count": z.U("retry_count+1"),
		"status":      "pending",
		"started_at":  z.U("NULL"),
	},
		z.Fields("retry_count", "status", "started_at"),
		z.Where(z.Eq("id", id)),
	)
	if err != nil {
		return err
	}
	if n == 0 {
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

	total, err := s.countTasks("")
	if err != nil {
		return stats, err
	}
	stats.TotalTasks = total

	stats.PendingTasks, err = s.countTasks("pending")
	if err != nil {
		return stats, err
	}

	stats.RunningTasks, err = s.countTasks("running")
	if err != nil {
		return stats, err
	}

	stats.CompletedTasks, err = s.countTasks("completed")
	if err != nil {
		return stats, err
	}

	stats.FailedTasks, err = s.countTasks("failed")
	if err != nil {
		return stats, err
	}

	stats.CancelledTasks, err = s.countTasks("cancelled")
	if err != nil {
		return stats, err
	}

	return stats, nil
}

func (s *Store) countTasks(status string) (int, error) {
	var count int64
	args := []z.ZormItem{z.Fields("count(1)")}
	if status != "" {
		args = append(args, z.Where(z.Eq("status", status)))
	}
	_, err := s.readTable().Select(&count, args...)
	if err != nil {
		return 0, err
	}
	return int(count), nil
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
	if s.readDB != nil && s.readDB != s.db {
		_ = s.readDB.Close()
	}
	return s.db.Close()
}
