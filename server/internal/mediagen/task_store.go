package mediagen

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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
			id           TEXT PRIMARY KEY,
			user_id      TEXT NOT NULL DEFAULT '',
			message_id   TEXT NOT NULL DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'pending',
			type         TEXT NOT NULL DEFAULT '',
			category     TEXT NOT NULL DEFAULT '',
			provider     TEXT NOT NULL DEFAULT '',
			model        TEXT NOT NULL DEFAULT '',
			upstream_id  TEXT NOT NULL DEFAULT '',
			request      TEXT NOT NULL DEFAULT '{}',
			response     TEXT NOT NULL DEFAULT '',
			error        TEXT NOT NULL DEFAULT '',
			progress     REAL NOT NULL DEFAULT 0,
			source       TEXT NOT NULL DEFAULT 'web',
			created_at   TEXT NOT NULL,
			updated_at   TEXT NOT NULL,
			completed_at TEXT
		)`)
	if err != nil {
		return err
	}
	if err := ensureMediaTaskColumns(s.db); err != nil {
		return err
	}
	// Index for recovery: find non-terminal tasks
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_media_tasks_status ON media_tasks(status)`)
	if err != nil {
		return err
	}
	if _, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_media_tasks_message ON media_tasks(message_id)`); err != nil {
		return err
	}
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_media_tasks_user ON media_tasks(user_id)`)
	return err
}

func ensureMediaTaskColumns(db *sql.DB) error {
	existing := map[string]struct{}{}
	rows, err := db.Query(`PRAGMA table_info(media_tasks)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return err
		}
		existing[name] = struct{}{}
	}
	if _, ok := existing["user_id"]; !ok {
		if _, err := db.Exec(`ALTER TABLE media_tasks ADD COLUMN user_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

// PersistentTask is the DB-serializable form of a media task.
type PersistentTask struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id,omitempty"`
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

func normalizeMediaTaskScope(userID []string) (string, bool) {
	if len(userID) == 0 {
		return "", false
	}
	return strings.TrimSpace(userID[0]), true
}

func mediaTaskScopeClause(userID []string, column string) (string, []any) {
	scopedUserID, scoped := normalizeMediaTaskScope(userID)
	if !scoped {
		return "", nil
	}
	if scopedUserID == "" {
		return fmt.Sprintf(" AND %s = ''", column), nil
	}
	return fmt.Sprintf(" AND %s = ?", column), []any{scopedUserID}
}

// Create inserts a new task.
func (s *TaskStore) Create(t *PersistentTask) error {
	now := timeutil.NowTime().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	_, err := s.db.Exec(`
		INSERT INTO media_tasks (id, user_id, message_id, status, type, category, provider, model,
			upstream_id, request, response, error, progress, source, created_at, updated_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.UserID, t.MessageID, string(t.Status), string(t.Type), t.Category, t.Provider, t.Model,
		t.UpstreamID, t.Request, t.Response, t.Error, t.Progress, t.Source,
		task.TimeToSQL(t.CreatedAt), task.TimeToSQL(t.UpdatedAt), task.NullTimeToSQL(t.CompletedAt),
	)
	return err
}

// UpdateStatus updates task status, progress, error, and response.
func (s *TaskStore) UpdateStatus(id string, status TaskStatus, progress float64, errMsg string, response string) error {
	now := task.TimeToSQL(timeutil.NowTime())
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
		upstreamID, task.TimeToSQL(timeutil.NowTime()), id)
	return err
}

// UpdateMessageID sets the message_id for a task (used when the assistant message is created after task creation).
func (s *TaskStore) UpdateMessageID(taskID, messageID string) error {
	_, err := s.db.Exec(`UPDATE media_tasks SET message_id=?, updated_at=? WHERE id=?`,
		messageID, task.TimeToSQL(timeutil.NowTime()), taskID)
	return err
}

// Get retrieves a single task by ID.
func (s *TaskStore) Get(id string, userID ...string) (*PersistentTask, error) {
	clause, args := mediaTaskScopeClause(userID, "user_id")
	query := `SELECT id, user_id, message_id, status, type, category, provider, model,
		upstream_id, request, response, error, progress, source, created_at, updated_at, completed_at
		FROM media_tasks WHERE id=?` + clause
	queryArgs := []any{id}
	queryArgs = append(queryArgs, args...)
	row := s.db.QueryRow(query, queryArgs...)
	return scanTask(row)
}

// GetByMessageID retrieves tasks associated with a message.
func (s *TaskStore) GetByMessageID(messageID string, userID ...string) ([]*PersistentTask, error) {
	clause, args := mediaTaskScopeClause(userID, "user_id")
	query := `SELECT id, user_id, message_id, status, type, category, provider, model,
		upstream_id, request, response, error, progress, source, created_at, updated_at, completed_at
		FROM media_tasks WHERE message_id=?` + clause + ` ORDER BY created_at DESC`
	queryArgs := []any{messageID}
	queryArgs = append(queryArgs, args...)
	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

// ListPending returns all non-terminal tasks (for power-failure recovery).
func (s *TaskStore) ListPending() ([]*PersistentTask, error) {
	rows, err := s.db.Query(`SELECT id, user_id, message_id, status, type, category, provider, model,
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
	err := row.Scan(&t.ID, &t.UserID, &t.MessageID, &t.Status, &t.Type, &t.Category, &t.Provider, &t.Model,
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
		err := rows.Scan(&t.ID, &t.UserID, &t.MessageID, &t.Status, &t.Type, &t.Category, &t.Provider, &t.Model,
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
		UserID:     t.UserID,
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

// MediaStats holds aggregated media generation statistics.
type MediaStats struct {
	TotalTasks      int64              `json:"total_tasks"`
	Succeeded       int64              `json:"succeeded"`
	Failed          int64              `json:"failed"`
	TotalCostUSD    float64            `json:"total_cost_usd"`
	CostByModel     map[string]float64 `json:"cost_by_model,omitempty"`
	TasksByType     map[string]int64   `json:"tasks_by_type,omitempty"`
	TasksByCategory map[string]int64   `json:"tasks_by_category,omitempty"`
	TasksByProvider map[string]int64   `json:"tasks_by_provider,omitempty"`
	CostByProvider  map[string]float64 `json:"cost_by_provider,omitempty"`
}

// GetStats aggregates media generation statistics from the task store.
func (s *TaskStore) GetStats(userID ...string) (*MediaStats, error) {
	stats := &MediaStats{
		CostByModel:     make(map[string]float64),
		TasksByType:     make(map[string]int64),
		TasksByCategory: make(map[string]int64),
		TasksByProvider: make(map[string]int64),
		CostByProvider:  make(map[string]float64),
	}

	clause, args := mediaTaskScopeClause(userID, "user_id")
	queryRowWithScope := func(base string, dest *int64) {
		row := s.db.QueryRow(base+clause, args...)
		_ = row.Scan(dest)
	}
	queryWithScope := func(base string) (*sql.Rows, error) {
		return s.db.Query(base+clause, args...)
	}

	// Count by status
	queryRowWithScope(`SELECT COUNT(*) FROM media_tasks WHERE status='succeeded'`, &stats.Succeeded)
	queryRowWithScope(`SELECT COUNT(*) FROM media_tasks WHERE status='failed'`, &stats.Failed)
	stats.TotalTasks = stats.Succeeded + stats.Failed

	// Count by type
	rows, err := queryWithScope(`SELECT type, COUNT(*) FROM media_tasks WHERE status='succeeded' GROUP BY type`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var c int64
			if rows.Scan(&t, &c) == nil {
				stats.TasksByType[t] = c
			}
		}
	}

	// Count by category
	rows3, err := queryWithScope(`SELECT category, COUNT(*) FROM media_tasks WHERE status='succeeded' AND category != '' GROUP BY category`)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var cat string
			var c int64
			if rows3.Scan(&cat, &c) == nil {
				stats.TasksByCategory[cat] = c
			}
		}
	}

	// Count by provider
	rows4, err := queryWithScope(`SELECT provider, COUNT(*) FROM media_tasks WHERE status='succeeded' AND provider != '' GROUP BY provider`)
	if err == nil {
		defer rows4.Close()
		for rows4.Next() {
			var prov string
			var c int64
			if rows4.Scan(&prov, &c) == nil {
				stats.TasksByProvider[prov] = c
			}
		}
	}

	// Calculate cost from succeeded tasks
	rows2, err := queryWithScope(`SELECT model, provider, response, type FROM media_tasks WHERE status='succeeded' AND response != ''`)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var model, provider, respJSON, mediaType string
			if rows2.Scan(&model, &provider, &respJSON, &mediaType) != nil {
				continue
			}
			var resp MediaResponse
			if json.Unmarshal([]byte(respJSON), &resp) != nil {
				continue
			}
			imageCount := len(resp.Data)
			var durationSec float64
			for _, r := range resp.Data {
				durationSec += float64(r.DurationSec)
			}
			cost := CalculateMediaCost(model, imageCount, durationSec)
			stats.TotalCostUSD += cost
			stats.CostByModel[model] += cost
			if provider != "" {
				stats.CostByProvider[provider] += cost
			}
		}
	}

	return stats, nil
}

// FromMediaTask converts an in-memory MediaTask to a PersistentTask.
func FromMediaTask(mt *MediaTask) *PersistentTask {
	t := &PersistentTask{
		ID:          mt.ID,
		UserID:      mt.UserID,
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
