package mediagen

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
)

// TaskStore persists media generation tasks to SQLite for power-failure recovery.
type TaskStore struct {
	db *sql.DB
}

// NewTaskStore creates a new task store and ensures the schema exists.
func NewTaskStore(db *sql.DB) (*TaskStore, error) {
	s := &TaskStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("task store migration: %w", err)
	}
	return s, nil
}

func (s *TaskStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS media_tasks (
			id          TEXT PRIMARY KEY,
			message_id  TEXT NOT NULL DEFAULT '',
			status      TEXT NOT NULL DEFAULT 'pending',
			type        TEXT NOT NULL DEFAULT '',
			category    TEXT NOT NULL DEFAULT '',
			provider    TEXT NOT NULL DEFAULT '',
			model       TEXT NOT NULL DEFAULT '',
			upstream_id TEXT NOT NULL DEFAULT '',
			request     TEXT NOT NULL DEFAULT '{}',
			response    TEXT NOT NULL DEFAULT '',
			error       TEXT NOT NULL DEFAULT '',
			progress    REAL NOT NULL DEFAULT 0,
			source      TEXT NOT NULL DEFAULT 'web',
			created_at  TEXT NOT NULL,
			updated_at  TEXT NOT NULL,
			completed_at TEXT
		)`)
	if err != nil {
		return err
	}
	// Index for recovery: find non-terminal tasks
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_media_tasks_status ON media_tasks(status)`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_media_tasks_message ON media_tasks(message_id)`)
	return err
}

// PersistentTask is the DB-serializable form of a media task.
type PersistentTask struct {
	ID          string     `json:"id"`
	MessageID   string     `json:"message_id"`
	Status      TaskStatus `json:"status"`
	Type        MediaType  `json:"type"`
	Category    string     `json:"category"`
	Provider    string     `json:"provider"`
	Model       string     `json:"model"`
	UpstreamID  string     `json:"upstream_id"`
	Request     string     `json:"request"`  // JSON-encoded MediaRequest
	Response    string     `json:"response"` // JSON-encoded MediaResponse
	Error       string     `json:"error"`
	Progress    float64    `json:"progress"`
	Source      string     `json:"source"` // "web" or "channel"
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Create inserts a new task.
func (s *TaskStore) Create(t *PersistentTask) error {
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	_, err := s.db.Exec(`
		INSERT INTO media_tasks (id, message_id, status, type, category, provider, model,
			upstream_id, request, response, error, progress, source, created_at, updated_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.MessageID, string(t.Status), string(t.Type), t.Category, t.Provider, t.Model,
		t.UpstreamID, t.Request, t.Response, t.Error, t.Progress, t.Source,
		task.TimeToSQL(t.CreatedAt), task.TimeToSQL(t.UpdatedAt), task.NullTimeToSQL(t.CompletedAt),
	)
	return err
}

// UpdateStatus updates task status, progress, error, and response.
func (s *TaskStore) UpdateStatus(id string, status TaskStatus, progress float64, errMsg string, response string) error {
	now := task.TimeToSQL(time.Now())
	var completedAt sql.NullString
	if status.IsTerminal() {
		completedAt = sql.NullString{String: now, Valid: true}
	}
	_, err := s.db.Exec(`
		UPDATE media_tasks SET status=?, progress=?, error=?, response=?, updated_at=?, completed_at=?
		WHERE id=?`,
		string(status), progress, errMsg, response, now, completedAt, id,
	)
	return err
}

// UpdateUpstreamID sets the upstream (vendor) task ID after generation starts.
func (s *TaskStore) UpdateUpstreamID(id, upstreamID string) error {
	_, err := s.db.Exec(`UPDATE media_tasks SET upstream_id=?, updated_at=? WHERE id=?`,
		upstreamID, task.TimeToSQL(time.Now()), id)
	return err
}

// UpdateMessageID sets the message_id for a task (used when the assistant message is created after task creation).
func (s *TaskStore) UpdateMessageID(taskID, messageID string) error {
	_, err := s.db.Exec(`UPDATE media_tasks SET message_id=?, updated_at=? WHERE id=?`,
		messageID, task.TimeToSQL(time.Now()), taskID)
	return err
}

// Get retrieves a single task by ID.
func (s *TaskStore) Get(id string) (*PersistentTask, error) {
	row := s.db.QueryRow(`SELECT id, message_id, status, type, category, provider, model,
		upstream_id, request, response, error, progress, source, created_at, updated_at, completed_at
		FROM media_tasks WHERE id=?`, id)
	return scanTask(row)
}

// GetByMessageID retrieves tasks associated with a message.
func (s *TaskStore) GetByMessageID(messageID string) ([]*PersistentTask, error) {
	rows, err := s.db.Query(`SELECT id, message_id, status, type, category, provider, model,
		upstream_id, request, response, error, progress, source, created_at, updated_at, completed_at
		FROM media_tasks WHERE message_id=? ORDER BY created_at DESC`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

// ListPending returns all non-terminal tasks (for power-failure recovery).
func (s *TaskStore) ListPending() ([]*PersistentTask, error) {
	rows, err := s.db.Query(`SELECT id, message_id, status, type, category, provider, model,
		upstream_id, request, response, error, progress, source, created_at, updated_at, completed_at
		FROM media_tasks WHERE status IN ('pending', 'processing') ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func scanTask(row *sql.Row) (*PersistentTask, error) {
	t := &PersistentTask{}
	var createdAt, updatedAt string
	var completedAt sql.NullString
	err := row.Scan(&t.ID, &t.MessageID, &t.Status, &t.Type, &t.Category, &t.Provider, &t.Model,
		&t.UpstreamID, &t.Request, &t.Response, &t.Error, &t.Progress, &t.Source,
		&createdAt, &updatedAt, &completedAt)
	if err != nil {
		return nil, err
	}
	t.CreatedAt = task.TimeFromSQL(createdAt)
	t.UpdatedAt = task.TimeFromSQL(updatedAt)
	t.CompletedAt = task.NullTimeFromSQL(completedAt)
	return t, nil
}

func scanTasks(rows *sql.Rows) ([]*PersistentTask, error) {
	var tasks []*PersistentTask
	for rows.Next() {
		t := &PersistentTask{}
		var createdAt, updatedAt string
		var completedAt sql.NullString
		err := rows.Scan(&t.ID, &t.MessageID, &t.Status, &t.Type, &t.Category, &t.Provider, &t.Model,
			&t.UpstreamID, &t.Request, &t.Response, &t.Error, &t.Progress, &t.Source,
			&createdAt, &updatedAt, &completedAt)
		if err != nil {
			return tasks, err
		}
		t.CreatedAt = task.TimeFromSQL(createdAt)
		t.UpdatedAt = task.TimeFromSQL(updatedAt)
		t.CompletedAt = task.NullTimeFromSQL(completedAt)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ToMediaTask converts a PersistentTask to an in-memory MediaTask.
func (t *PersistentTask) ToMediaTask() *MediaTask {
	mt := &MediaTask{
		BaseTask: task.BaseTask{
			ID:          t.ID,
			Status:      t.Status,
			Error:       t.Error,
			Progress:    t.Progress,
			CreatedAt:   t.CreatedAt,
			CompletedAt: t.CompletedAt,
		},
		Type:       t.Type,
		Provider:   t.Provider,
		Model:      t.Model,
		UpstreamID: t.UpstreamID,
		MessageID:  t.MessageID,
		Category:   t.Category,
		Source:     t.Source,
	}
	if t.Request != "" && t.Request != "{}" {
		var req MediaRequest
		if json.Unmarshal([]byte(t.Request), &req) == nil {
			mt.Request = &req
		}
	}
	if t.Response != "" {
		var resp MediaResponse
		if json.Unmarshal([]byte(t.Response), &resp) == nil {
			mt.Response = &resp
		}
	}
	return mt
}

// FromMediaTask converts an in-memory MediaTask to a PersistentTask.
func FromMediaTask(mt *MediaTask) *PersistentTask {
	t := &PersistentTask{
		ID:          mt.ID,
		MessageID:   mt.MessageID,
		Status:      mt.Status,
		Type:        mt.Type,
		Category:    mt.Category,
		Provider:    mt.Provider,
		Model:       mt.Model,
		UpstreamID:  mt.UpstreamID,
		Error:       mt.Error,
		Progress:    mt.Progress,
		Source:      mt.Source,
		CreatedAt:   mt.CreatedAt,
		CompletedAt: mt.CompletedAt,
	}
	if mt.Request != nil {
		if b, err := json.Marshal(mt.Request); err == nil {
			t.Request = string(b)
		}
	}
	if mt.Response != nil {
		if b, err := json.Marshal(mt.Response); err == nil {
			t.Response = string(b)
		}
	}
	return t
}
