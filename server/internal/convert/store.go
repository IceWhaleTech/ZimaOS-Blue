package convert

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
)

const retentionWindow = 7 * 24 * time.Hour

type Store struct {
	db     *sql.DB
	readDB *sql.DB
}

func NewStore(db *sql.DB) (*Store, error) {
	return NewStoreWithReadDB(db, db)
}

func NewStoreWithReadDB(writeDB, readDB *sql.DB) (*Store, error) {
	if readDB == nil {
		readDB = writeDB
	}
	store := &Store{db: writeDB, readDB: readDB}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("convert store migrate: %w", err)
	}
	return store, nil
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

func (s *Store) table(name string) *z.ZormTable {
	return z.Table(s.db, name)
}

func (s *Store) readTable(name string) *z.ZormTable {
	return z.Table(s.reader(), name)
}

type convertTaskRow struct {
	ID                string  `json:"id" zorm:"id"`
	UserID            string  `json:"user_id" zorm:"user_id"`
	ConversationID    string  `json:"conversation_id" zorm:"conversation_id"`
	Action            string  `json:"action" zorm:"action"`
	Status            string  `json:"status" zorm:"status"`
	TargetFormat      string  `json:"target_format" zorm:"target_format"`
	SourceSummary     string  `json:"source_summary" zorm:"source_summary"`
	Progress          float64 `json:"progress" zorm:"progress"`
	Message           string  `json:"message" zorm:"message"`
	Error             string  `json:"error" zorm:"error"`
	TranscriptPreview string  `json:"transcript_preview" zorm:"transcript_preview"`
	RequestJSON       string  `json:"request_json" zorm:"request_json"`
	OutputsJSON       string  `json:"outputs_json" zorm:"outputs_json"`
	CreatedAt         string  `json:"created_at" zorm:"created_at"`
	UpdatedAt         string  `json:"updated_at" zorm:"updated_at"`
	StartedAt         *string `json:"started_at" zorm:"started_at"`
	CompletedAt       *string `json:"completed_at" zorm:"completed_at"`
}

type storedSourceRow struct {
	ID             string `json:"id" zorm:"id"`
	UserID         string `json:"user_id" zorm:"user_id"`
	ConversationID string `json:"conversation_id" zorm:"conversation_id"`
	Name           string `json:"name" zorm:"name"`
	MimeType       string `json:"mime_type" zorm:"mime_type"`
	Path           string `json:"path" zorm:"path"`
	SizeBytes      int64  `json:"size_bytes" zorm:"size_bytes"`
	CreatedAt      string `json:"created_at" zorm:"created_at"`
}

type convertTaskIDRow struct {
	ID string `json:"id" zorm:"id"`
}

type convertSourcePathRow struct {
	Path string `json:"path" zorm:"path"`
}

func taskToValues(task *ConvertTask) (z.V, error) {
	requestJSON := "{}"
	if task.Request != nil {
		if raw, err := json.Marshal(task.Request); err == nil {
			requestJSON = string(raw)
		}
	}
	outputsJSON, err := marshalStoredOutputs(task.Outputs)
	if err != nil {
		return nil, err
	}
	return z.V{
		"id":                 task.ID,
		"user_id":            task.UserID,
		"conversation_id":    task.ConversationID,
		"action":             task.Action,
		"status":             string(task.Status),
		"target_format":      task.TargetFormat,
		"source_summary":     task.SourceSummary,
		"progress":           task.Progress,
		"message":            task.Message,
		"error":              task.Error,
		"transcript_preview": task.TranscriptPreview,
		"request_json":       requestJSON,
		"outputs_json":       outputsJSON,
		"created_at":         task.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":         task.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"started_at":         nullableTime(task.StartedAt),
		"completed_at":       nullableTime(task.CompletedAt),
	}, nil
}

func rowToTask(row convertTaskRow) (*ConvertTask, error) {
	task := &ConvertTask{
		ID:                row.ID,
		UserID:            row.UserID,
		ConversationID:    row.ConversationID,
		Action:            row.Action,
		Status:            TaskStatus(row.Status),
		TargetFormat:      row.TargetFormat,
		SourceSummary:     row.SourceSummary,
		Progress:          row.Progress,
		Message:           row.Message,
		Error:             row.Error,
		TranscriptPreview: row.TranscriptPreview,
	}
	if parsed, err := time.Parse(time.RFC3339Nano, row.CreatedAt); err == nil {
		task.CreatedAt = parsed
	}
	if parsed, err := time.Parse(time.RFC3339Nano, row.UpdatedAt); err == nil {
		task.UpdatedAt = parsed
	}
	if row.StartedAt != nil && strings.TrimSpace(*row.StartedAt) != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, *row.StartedAt); err == nil {
			task.StartedAt = &parsed
		}
	}
	if row.CompletedAt != nil && strings.TrimSpace(*row.CompletedAt) != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, *row.CompletedAt); err == nil {
			task.CompletedAt = &parsed
		}
	}
	if strings.TrimSpace(row.RequestJSON) != "" && strings.TrimSpace(row.RequestJSON) != "{}" {
		var request TaskRequest
		if err := json.Unmarshal([]byte(row.RequestJSON), &request); err == nil {
			task.Request = &request
			if len(request.Sources) > 0 {
				task.Sources = append([]string(nil), request.Sources...)
			}
		}
	}
	outputs, err := unmarshalStoredOutputs(row.OutputsJSON)
	if err != nil {
		return nil, err
	}
	task.Outputs = outputs
	return task, nil
}

func sourceToValues(source *StoredSource) z.V {
	return z.V{
		"id":              source.ID,
		"user_id":         source.UserID,
		"conversation_id": source.ConversationID,
		"name":            source.Name,
		"mime_type":       source.MimeType,
		"path":            source.Path,
		"size_bytes":      source.SizeBytes,
		"created_at":      source.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func rowToSource(row storedSourceRow) *StoredSource {
	source := &StoredSource{
		ID:             row.ID,
		UserID:         row.UserID,
		ConversationID: row.ConversationID,
		Name:           row.Name,
		MimeType:       row.MimeType,
		Path:           row.Path,
		SizeBytes:      row.SizeBytes,
	}
	if parsed, err := time.Parse(time.RFC3339Nano, row.CreatedAt); err == nil {
		source.CreatedAt = parsed
	}
	return source
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
	values, err := taskToValues(task)
	if err != nil {
		return err
	}
	_, err = s.table("convert_tasks").Insert(values)
	return err
}

func (s *Store) UpdateTask(task *ConvertTask) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	task.UpdatedAt = timeutil.NowTime().UTC()
	values, err := taskToValues(task)
	if err != nil {
		return err
	}
	_, err = s.table("convert_tasks").Update(
		values,
		z.Fields(
			"user_id",
			"conversation_id",
			"action",
			"status",
			"target_format",
			"source_summary",
			"progress",
			"message",
			"error",
			"transcript_preview",
			"request_json",
			"outputs_json",
			"updated_at",
			"started_at",
			"completed_at",
		),
		z.Where(z.Eq("id", task.ID)),
	)
	return err
}

func (s *Store) GetTask(taskID string) (*ConvertTask, error) {
	var rows []convertTaskRow
	_, err := s.readTable("convert_tasks").Select(&rows,
		z.Where(z.Eq("id", taskID)),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rowToTask(rows[0])
}

func (s *Store) ListTasks(userID, conversationID string, limit int) ([]*ConvertTask, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	opts := []z.ZormItem{
		z.OrderBy("created_at DESC"),
		z.Limit(limit),
	}
	conds := make([]interface{}, 0, 2)
	if strings.TrimSpace(userID) != "" {
		conds = append(conds, z.Eq("user_id", userID))
	}
	if strings.TrimSpace(conversationID) != "" {
		conds = append(conds, z.Eq("conversation_id", conversationID))
	}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	var rows []convertTaskRow
	_, err := s.readTable("convert_tasks").Select(&rows, opts...)
	if err != nil {
		return nil, err
	}
	tasks := make([]*ConvertTask, 0, len(rows))
	for i := range rows {
		task, err := rowToTask(rows[i])
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (s *Store) MarkInterruptedTasksFailed(message string) error {
	var rows []convertTaskRow
	if _, err := s.readTable("convert_tasks").Select(&rows,
		z.Fields("id", "message", "error"),
		z.Where(z.In("status", string(StatusPending), string(StatusProcessing))),
	); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	txTable := z.Table(tx, "convert_tasks")
	now := timeutil.NowTime().UTC().Format(time.RFC3339Nano)
	for i := range rows {
		updateValues := z.V{
			"status":       string(StatusFailed),
			"updated_at":   now,
			"completed_at": now,
		}
		if rows[i].Error == "" {
			updateValues["error"] = message
		}
		if rows[i].Message == "" {
			updateValues["message"] = message
		}
		fields := []string{"status", "updated_at", "completed_at"}
		if rows[i].Error == "" {
			fields = append(fields, "error")
		}
		if rows[i].Message == "" {
			fields = append(fields, "message")
		}
		if _, err := txTable.Update(updateValues, z.Fields(fields...), z.Where(z.Eq("id", rows[i].ID))); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) CreateSource(source *StoredSource) error {
	if source == nil {
		return fmt.Errorf("source is nil")
	}
	if source.CreatedAt.IsZero() {
		source.CreatedAt = timeutil.NowTime().UTC()
	}
	_, err := s.table("convert_sources").Insert(sourceToValues(source))
	return err
}

func (s *Store) GetSource(sourceID string) (*StoredSource, error) {
	var rows []storedSourceRow
	_, err := s.readTable("convert_sources").Select(&rows,
		z.Where(z.Eq("id", sourceID)),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rowToSource(rows[0]), nil
}

func (s *Store) ListSources(userID, conversationID string, limit int) ([]*StoredSource, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	opts := []z.ZormItem{
		z.OrderBy("created_at DESC"),
		z.Limit(limit),
	}
	conds := make([]interface{}, 0, 2)
	if strings.TrimSpace(userID) != "" {
		conds = append(conds, z.Eq("user_id", userID))
	}
	if strings.TrimSpace(conversationID) != "" {
		conds = append(conds, z.Eq("conversation_id", conversationID))
	}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	var rows []storedSourceRow
	_, err := s.readTable("convert_sources").Select(&rows, opts...)
	if err != nil {
		return nil, err
	}
	sources := make([]*StoredSource, 0, len(rows))
	for i := range rows {
		sources = append(sources, rowToSource(rows[i]))
	}
	return sources, nil
}

func (s *Store) CleanupExpired(cutoff time.Time) (taskIDs []string, taskRoots []string, sourcePaths []string, err error) {
	cutoffSQL := cutoff.UTC().Format(time.RFC3339Nano)
	var taskRows []convertTaskIDRow
	if _, err := s.readTable("convert_tasks").Select(&taskRows,
		z.Fields("id"),
		z.Where(z.Lt("created_at", cutoffSQL)),
	); err != nil {
		return nil, nil, nil, err
	}
	for i := range taskRows {
		taskIDs = append(taskIDs, taskRows[i].ID)
	}
	var sourceRows []convertSourcePathRow
	if _, err := s.readTable("convert_sources").Select(&sourceRows,
		z.Fields("path"),
		z.Where(z.Lt("created_at", cutoffSQL)),
	); err != nil {
		return nil, nil, nil, err
	}
	for i := range sourceRows {
		sourcePaths = append(sourcePaths, sourceRows[i].Path)
	}
	if _, err = s.table("convert_tasks").Delete(z.Where(z.Lt("created_at", cutoffSQL))); err != nil {
		return nil, nil, nil, err
	}
	if _, err = s.table("convert_sources").Delete(z.Where(z.Lt("created_at", cutoffSQL))); err != nil {
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
