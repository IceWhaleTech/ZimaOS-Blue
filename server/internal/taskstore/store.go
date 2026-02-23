// Package taskstore provides SQLite persistence for tasks.
package taskstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// Task represents a persisted task.
type Task struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority,omitempty"`
	DueDate     string    `json:"due_date,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
	Completed   time.Time `json:"completed,omitempty"`
}

// Store provides SQLite-backed task persistence.
type Store struct {
	db *sql.DB
}

// NewStore creates the tasks table and returns a Store.
func NewStore(db *sql.DB) (*Store, error) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			owner_id TEXT NOT NULL DEFAULT 'default',
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			priority TEXT DEFAULT 'medium',
			due_date TEXT DEFAULT '',
			tags TEXT DEFAULT '[]',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			completed_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_tasks_owner ON tasks(owner_id, status);
	`)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Create inserts a new task.
func (s *Store) Create(ctx context.Context, t *Task) error {
	tagsJSON, _ := json.Marshal(t.Tags)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tasks (id, owner_id, title, description, status, priority, due_date, tags, created_at, updated_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.OwnerID, t.Title, t.Description, t.Status, t.Priority, t.DueDate,
		string(tagsJSON), t.Created, t.Updated, t.Completed)
	return err
}

// Get retrieves a task by ID.
func (s *Store) Get(ctx context.Context, id string) (*Task, error) {
	var t Task
	var tagsJSON string
	var completedAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id, owner_id, title, description, status, priority, due_date, tags, created_at, updated_at, completed_at
		 FROM tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.OwnerID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.DueDate,
			&tagsJSON, &t.Created, &t.Updated, &completedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(tagsJSON), &t.Tags)
	if completedAt.Valid {
		t.Completed = completedAt.Time
	}
	return &t, nil
}

// Update modifies an existing task.
func (s *Store) Update(ctx context.Context, t *Task) error {
	tagsJSON, _ := json.Marshal(t.Tags)
	var completedAt interface{}
	if !t.Completed.IsZero() {
		completedAt = t.Completed
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET title = ?, description = ?, status = ?, priority = ?, due_date = ?,
		 tags = ?, updated_at = ?, completed_at = ? WHERE id = ?`,
		t.Title, t.Description, t.Status, t.Priority, t.DueDate,
		string(tagsJSON), t.Updated, completedAt, t.ID)
	return err
}

// Delete removes a task by ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	return err
}

// ListByOwner returns tasks for an owner, optionally filtered by status.
func (s *Store) ListByOwner(ctx context.Context, ownerID, status string) ([]*Task, error) {
	var rows *sql.Rows
	var err error
	if status != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, owner_id, title, description, status, priority, due_date, tags, created_at, updated_at, completed_at
			 FROM tasks WHERE owner_id = ? AND status = ? ORDER BY created_at DESC`, ownerID, status)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, owner_id, title, description, status, priority, due_date, tags, created_at, updated_at, completed_at
			 FROM tasks WHERE owner_id = ? ORDER BY created_at DESC`, ownerID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func scanTasks(rows *sql.Rows) ([]*Task, error) {
	var tasks []*Task
	for rows.Next() {
		var t Task
		var tagsJSON string
		var completedAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.OwnerID, &t.Title, &t.Description, &t.Status, &t.Priority,
			&t.DueDate, &tagsJSON, &t.Created, &t.Updated, &completedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tagsJSON), &t.Tags)
		if completedAt.Valid {
			t.Completed = completedAt.Time
		}
		tasks = append(tasks, &t)
	}
	return tasks, rows.Err()
}
