package convert

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const retentionWindow = 7 * 24 * time.Hour

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) (*Store, error) {
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("convert store migrate: %w", err)
	}
	return store, nil
}

func (s *Store) migrate() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("db is required")
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS convert_tasks (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			conversation_id TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			status TEXT NOT NULL,
			target_format TEXT NOT NULL DEFAULT '',
			source_summary TEXT NOT NULL DEFAULT '',
			progress REAL NOT NULL DEFAULT 0,
			message TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			transcript_preview TEXT NOT NULL DEFAULT '',
			request_json TEXT NOT NULL DEFAULT '{}',
			outputs_json TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			started_at TEXT,
			completed_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_convert_tasks_conversation_created ON convert_tasks(conversation_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_convert_tasks_user_created ON convert_tasks(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_convert_tasks_status ON convert_tasks(status)`,
		`CREATE TABLE IF NOT EXISTS convert_sources (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			conversation_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			mime_type TEXT NOT NULL DEFAULT '',
			path TEXT NOT NULL DEFAULT '',
			size_bytes INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_convert_sources_conversation_created ON convert_sources(conversation_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_convert_sources_user_created ON convert_sources(user_id, created_at DESC)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateTask(task *ConvertTask) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	now := timeutil.NowTime().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	requestJSON := "{}"
	if task.Request != nil {
		if raw, err := json.Marshal(task.Request); err == nil {
			requestJSON = string(raw)
		}
	}
	outputsJSON, err := marshalStoredOutputs(task.Outputs)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO convert_tasks (
			id, user_id, conversation_id, action, status, target_format, source_summary,
			progress, message, error, transcript_preview, request_json, outputs_json,
			created_at, updated_at, started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID,
		task.UserID,
		task.ConversationID,
		task.Action,
		string(task.Status),
		task.TargetFormat,
		task.SourceSummary,
		task.Progress,
		task.Message,
		task.Error,
		task.TranscriptPreview,
		requestJSON,
		outputsJSON,
		task.CreatedAt.Format(time.RFC3339Nano),
		task.UpdatedAt.Format(time.RFC3339Nano),
		nullableTime(task.StartedAt),
		nullableTime(task.CompletedAt),
	)
	return err
}

func (s *Store) UpdateTask(task *ConvertTask) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	task.UpdatedAt = timeutil.NowTime().UTC()
	requestJSON := "{}"
	if task.Request != nil {
		if raw, err := json.Marshal(task.Request); err == nil {
			requestJSON = string(raw)
		}
	}
	outputsJSON, err := marshalStoredOutputs(task.Outputs)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`UPDATE convert_tasks SET
			user_id=?, conversation_id=?, action=?, status=?, target_format=?, source_summary=?,
			progress=?, message=?, error=?, transcript_preview=?, request_json=?, outputs_json=?,
			updated_at=?, started_at=?, completed_at=?
		 WHERE id=?`,
		task.UserID,
		task.ConversationID,
		task.Action,
		string(task.Status),
		task.TargetFormat,
		task.SourceSummary,
		task.Progress,
		task.Message,
		task.Error,
		task.TranscriptPreview,
		requestJSON,
		outputsJSON,
		task.UpdatedAt.Format(time.RFC3339Nano),
		nullableTime(task.StartedAt),
		nullableTime(task.CompletedAt),
		task.ID,
	)
	return err
}

func (s *Store) GetTask(taskID string) (*ConvertTask, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, conversation_id, action, status, target_format, source_summary,
			progress, message, error, transcript_preview, request_json, outputs_json,
			created_at, updated_at, started_at, completed_at
		 FROM convert_tasks WHERE id=?`,
		taskID,
	)
	return scanTask(row)
}

func (s *Store) ListTasks(userID, conversationID string, limit int) ([]*ConvertTask, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT id, user_id, conversation_id, action, status, target_format, source_summary,
		progress, message, error, transcript_preview, request_json, outputs_json,
		created_at, updated_at, started_at, completed_at FROM convert_tasks`
	args := make([]interface{}, 0, 3)
	clauses := make([]string, 0, 2)
	if strings.TrimSpace(userID) != "" {
		clauses = append(clauses, "user_id=?")
		args = append(args, userID)
	}
	if strings.TrimSpace(conversationID) != "" {
		clauses = append(clauses, "conversation_id=?")
		args = append(args, conversationID)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []*ConvertTask
	for rows.Next() {
		task, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) MarkInterruptedTasksFailed(message string) error {
	now := timeutil.NowTime().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(
		`UPDATE convert_tasks
		 SET status=?, error=CASE WHEN error='' THEN ? ELSE error END, message=CASE WHEN message='' THEN ? ELSE message END, updated_at=?, completed_at=?
		 WHERE status IN (?, ?)`,
		string(StatusFailed),
		message,
		message,
		now,
		now,
		string(StatusPending),
		string(StatusProcessing),
	)
	return err
}

func (s *Store) CreateSource(source *StoredSource) error {
	if source == nil {
		return fmt.Errorf("source is nil")
	}
	if source.CreatedAt.IsZero() {
		source.CreatedAt = timeutil.NowTime().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO convert_sources (id, user_id, conversation_id, name, mime_type, path, size_bytes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		source.ID,
		source.UserID,
		source.ConversationID,
		source.Name,
		source.MimeType,
		source.Path,
		source.SizeBytes,
		source.CreatedAt.Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) GetSource(sourceID string) (*StoredSource, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, conversation_id, name, mime_type, path, size_bytes, created_at
		 FROM convert_sources WHERE id=?`,
		sourceID,
	)
	var source StoredSource
	var createdAt string
	if err := row.Scan(&source.ID, &source.UserID, &source.ConversationID, &source.Name, &source.MimeType, &source.Path, &source.SizeBytes, &createdAt); err != nil {
		return nil, err
	}
	if parsed, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		source.CreatedAt = parsed
	}
	return &source, nil
}

func (s *Store) ListSources(userID, conversationID string, limit int) ([]*StoredSource, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT id, user_id, conversation_id, name, mime_type, path, size_bytes, created_at FROM convert_sources`
	args := make([]interface{}, 0, 3)
	clauses := make([]string, 0, 2)
	if strings.TrimSpace(userID) != "" {
		clauses = append(clauses, "user_id=?")
		args = append(args, userID)
	}
	if strings.TrimSpace(conversationID) != "" {
		clauses = append(clauses, "conversation_id=?")
		args = append(args, conversationID)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sources []*StoredSource
	for rows.Next() {
		var source StoredSource
		var createdAt string
		if err := rows.Scan(&source.ID, &source.UserID, &source.ConversationID, &source.Name, &source.MimeType, &source.Path, &source.SizeBytes, &createdAt); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			source.CreatedAt = parsed
		}
		sources = append(sources, &source)
	}
	return sources, rows.Err()
}

func (s *Store) CleanupExpired(cutoff time.Time) (taskIDs []string, taskRoots []string, sourcePaths []string, err error) {
	cutoffSQL := cutoff.UTC().Format(time.RFC3339Nano)
	taskRows, err := s.db.Query(`SELECT id FROM convert_tasks WHERE created_at < ?`, cutoffSQL)
	if err != nil {
		return nil, nil, nil, err
	}
	for taskRows.Next() {
		var id string
		if scanErr := taskRows.Scan(&id); scanErr != nil {
			taskRows.Close()
			return nil, nil, nil, scanErr
		}
		taskIDs = append(taskIDs, id)
	}
	taskRows.Close()

	sourceRows, err := s.db.Query(`SELECT path FROM convert_sources WHERE created_at < ?`, cutoffSQL)
	if err != nil {
		return nil, nil, nil, err
	}
	for sourceRows.Next() {
		var path string
		if scanErr := sourceRows.Scan(&path); scanErr != nil {
			sourceRows.Close()
			return nil, nil, nil, scanErr
		}
		sourcePaths = append(sourcePaths, path)
	}
	sourceRows.Close()

	if _, err = s.db.Exec(`DELETE FROM convert_tasks WHERE created_at < ?`, cutoffSQL); err != nil {
		return nil, nil, nil, err
	}
	if _, err = s.db.Exec(`DELETE FROM convert_sources WHERE created_at < ?`, cutoffSQL); err != nil {
		return nil, nil, nil, err
	}
	return taskIDs, taskRoots, sourcePaths, nil
}

func nullableTime(ts *time.Time) interface{} {
	if ts == nil || ts.IsZero() {
		return nil
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func scanTask(row interface {
	Scan(dest ...interface{}) error
}) (*ConvertTask, error) {
	return scanTaskCommon(row)
}

func scanTaskRows(rows *sql.Rows) (*ConvertTask, error) {
	return scanTaskCommon(rows)
}

func scanTaskCommon(row interface {
	Scan(dest ...interface{}) error
}) (*ConvertTask, error) {
	var task ConvertTask
	var requestJSON, outputsJSON string
	var createdAt, updatedAt string
	var startedAt, completedAt sql.NullString
	if err := row.Scan(
		&task.ID,
		&task.UserID,
		&task.ConversationID,
		&task.Action,
		&task.Status,
		&task.TargetFormat,
		&task.SourceSummary,
		&task.Progress,
		&task.Message,
		&task.Error,
		&task.TranscriptPreview,
		&requestJSON,
		&outputsJSON,
		&createdAt,
		&updatedAt,
		&startedAt,
		&completedAt,
	); err != nil {
		return nil, err
	}
	if parsed, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		task.CreatedAt = parsed
	}
	if parsed, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		task.UpdatedAt = parsed
	}
	if startedAt.Valid {
		if parsed, err := time.Parse(time.RFC3339Nano, startedAt.String); err == nil {
			task.StartedAt = &parsed
		}
	}
	if completedAt.Valid {
		if parsed, err := time.Parse(time.RFC3339Nano, completedAt.String); err == nil {
			task.CompletedAt = &parsed
		}
	}
	if strings.TrimSpace(requestJSON) != "" && strings.TrimSpace(requestJSON) != "{}" {
		var request TaskRequest
		if err := json.Unmarshal([]byte(requestJSON), &request); err == nil {
			task.Request = &request
		}
	}
	outputs, err := unmarshalStoredOutputs(outputsJSON)
	if err != nil {
		return nil, err
	}
	task.Outputs = outputs
	return &task, nil
}
