package mediagen

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
)

// TaskStore persists media generation tasks to SQLite for power-failure recovery.
type TaskStore struct {
	db     *sql.DB
	readDB *sql.DB
}

type mediaTaskRow struct {
	ID           string  `json:"id" zorm:"id"`
	UserID       string  `json:"user_id" zorm:"user_id"`
	MessageID    string  `json:"message_id" zorm:"message_id"`
	Status       string  `json:"status" zorm:"status"`
	Type         string  `json:"type" zorm:"type"`
	Category     string  `json:"category" zorm:"category"`
	Provider     string  `json:"provider" zorm:"provider"`
	Model        string  `json:"model" zorm:"model"`
	UpstreamID   string  `json:"upstream_id" zorm:"upstream_id"`
	Request      string  `json:"request" zorm:"request"`
	Response     string  `json:"response" zorm:"response"`
	FallbackInfo string  `json:"fallback_info" zorm:"fallback_info"`
	Error        string  `json:"error" zorm:"error"`
	Progress     float64 `json:"progress" zorm:"progress"`
	Source       string  `json:"source" zorm:"source"`
	CreatedAt    string  `json:"created_at" zorm:"created_at"`
	UpdatedAt    string  `json:"updated_at" zorm:"updated_at"`
	CompletedAt  *string `json:"completed_at" zorm:"completed_at"`
}

type mediaTaskBucketRow struct {
	Value string `json:"value" zorm:"value"`
	Count int64  `json:"count" zorm:"count"`
}

type mediaTaskCostRow struct {
	Model    string `json:"model" zorm:"model"`
	Provider string `json:"provider" zorm:"provider"`
	Response string `json:"response" zorm:"response"`
	Type     string `json:"type" zorm:"type"`
}

func mediaTaskStringFromMapValue(row z.V, key string) string {
	value, ok := mediaTaskValueFromMapKey(row, key)
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func mediaTaskInt64FromMapValue(row z.V, key string) int64 {
	value, ok := mediaTaskValueFromMapKey(row, key)
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	case string:
		result, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return result
	case []byte:
		result, _ := strconv.ParseInt(strings.TrimSpace(string(typed)), 10, 64)
		return result
	default:
		result, _ := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(typed)), 10, 64)
		return result
	}
}

func mediaTaskValueFromMapKey(row z.V, key string) (interface{}, bool) {
	if row == nil {
		return nil, false
	}
	if value, ok := row[key]; ok {
		return value, true
	}
	for rawKey, value := range row {
		if mediaTaskNormalizeMapKey(rawKey) == key {
			return value, true
		}
	}
	return nil, false
}

func mediaTaskNormalizeMapKey(key string) string {
	key = strings.TrimSpace(strings.Trim(key, "`"))
	if key == "" {
		return ""
	}
	fields := strings.Fields(key)
	if len(fields) >= 3 && strings.EqualFold(fields[len(fields)-2], "as") {
		return strings.Trim(fields[len(fields)-1], "`")
	}
	if len(fields) >= 2 {
		return strings.Trim(fields[len(fields)-1], "`")
	}
	if dot := strings.LastIndex(key, "."); dot >= 0 {
		return strings.Trim(key[dot+1:], "`")
	}
	return key
}

// NewTaskStore creates a new task store and ensures the schema exists.
func NewTaskStore(db *sql.DB) (*TaskStore, error) {
	return NewTaskStoreWithReadDB(db, db)
}

// NewTaskStoreWithReadDB creates a new task store with separate write and read
// database handles.
func NewTaskStoreWithReadDB(writeDB, readDB *sql.DB) (*TaskStore, error) {
	if readDB == nil {
		readDB = writeDB
	}
	s := &TaskStore{db: writeDB, readDB: readDB}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("task store migration: %w", err)
	}
	return s, nil
}

func (s *TaskStore) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *TaskStore) table() *z.ZormTable {
	return z.Table(s.db, "media_tasks")
}

func (s *TaskStore) readTable() *z.ZormTable {
	return z.Table(s.reader(), "media_tasks")
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
			fallback_info TEXT NOT NULL DEFAULT '',
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
	if _, ok := existing["fallback_info"]; !ok {
		if _, err := db.Exec(`ALTER TABLE media_tasks ADD COLUMN fallback_info TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

// PersistentTask is the DB-serializable form of a media task.
type PersistentTask struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id,omitempty"`
	MessageID    string     `json:"message_id"`
	Status       TaskStatus `json:"status"`
	Type         MediaType  `json:"type"`
	Category     string     `json:"category"`
	Provider     string     `json:"provider"`
	Model        string     `json:"model"`
	UpstreamID   string     `json:"upstream_id"`
	Request      string     `json:"request"`  // JSON-encoded MediaRequest
	Response     string     `json:"response"` // JSON-encoded MediaResponse
	FallbackInfo string     `json:"fallback_info"`
	Error        string     `json:"error"`
	Progress     float64    `json:"progress"`
	Source       string     `json:"source"` // "web" or "channel"
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

func normalizeMediaTaskScope(userID []string) (string, bool) {
	if len(userID) == 0 {
		return "", false
	}
	return strings.TrimSpace(userID[0]), true
}

func mediaTaskScopeConds(userID []string, column string) []interface{} {
	scopedUserID, scoped := normalizeMediaTaskScope(userID)
	if !scoped {
		return nil
	}
	if scopedUserID == "" {
		return []interface{}{z.Eq(column, "")}
	}
	return []interface{}{z.Eq(column, scopedUserID)}
}

func persistentTaskValues(t *PersistentTask) z.V {
	return z.V{
		"id":            t.ID,
		"user_id":       t.UserID,
		"message_id":    t.MessageID,
		"status":        string(t.Status),
		"type":          string(t.Type),
		"category":      t.Category,
		"provider":      t.Provider,
		"model":         t.Model,
		"upstream_id":   t.UpstreamID,
		"request":       t.Request,
		"response":      t.Response,
		"fallback_info": t.FallbackInfo,
		"error":         t.Error,
		"progress":      t.Progress,
		"source":        t.Source,
		"created_at":    task.TimeToSQL(t.CreatedAt),
		"updated_at":    task.TimeToSQL(t.UpdatedAt),
		"completed_at":  nullableMediaTaskTime(t.CompletedAt),
	}
}

func rowToPersistentTask(row mediaTaskRow) *PersistentTask {
	t := &PersistentTask{
		ID:           row.ID,
		UserID:       row.UserID,
		MessageID:    row.MessageID,
		Status:       TaskStatus(row.Status),
		Type:         MediaType(row.Type),
		Category:     row.Category,
		Provider:     row.Provider,
		Model:        row.Model,
		UpstreamID:   row.UpstreamID,
		Request:      row.Request,
		Response:     row.Response,
		FallbackInfo: row.FallbackInfo,
		Error:        row.Error,
		Progress:     row.Progress,
		Source:       row.Source,
		CreatedAt:    task.TimeFromSQL(row.CreatedAt),
		UpdatedAt:    task.TimeFromSQL(row.UpdatedAt),
	}
	if row.CompletedAt != nil && strings.TrimSpace(*row.CompletedAt) != "" {
		t.CompletedAt = task.NullTimeFromSQL(sql.NullString{String: *row.CompletedAt, Valid: true})
	}
	return t
}

func rowsToPersistentTasks(rows []mediaTaskRow) []*PersistentTask {
	tasks := make([]*PersistentTask, 0, len(rows))
	for i := range rows {
		tasks = append(tasks, rowToPersistentTask(rows[i]))
	}
	return tasks
}

func nullableMediaTaskTime(t *time.Time) interface{} {
	if t == nil || t.IsZero() {
		return nil
	}
	return task.TimeToSQL(*t)
}

// Create inserts a new task.
func (s *TaskStore) Create(t *PersistentTask) error {
	now := timeutil.NowTime().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	_, err := s.table().Insert(persistentTaskValues(t))
	return err
}

// UpdateStatus updates task status, progress, error, and response.
func (s *TaskStore) UpdateStatus(id string, status TaskStatus, progress float64, errMsg string, response string) error {
	now := timeutil.NowTime().UTC()
	var completedAt interface{}
	if status.IsTerminal() {
		completedAt = task.TimeToSQL(now)
	}
	_, err := s.table().Update(
		z.V{
			"status":       string(status),
			"progress":     progress,
			"error":        errMsg,
			"response":     response,
			"updated_at":   task.TimeToSQL(now),
			"completed_at": completedAt,
		},
		z.Fields("status", "progress", "error", "response", "updated_at", "completed_at"),
		z.Where(z.Eq("id", id)),
	)
	return err
}

// UpdateUpstreamID sets the upstream (vendor) task ID after generation starts.
func (s *TaskStore) UpdateUpstreamID(id, upstreamID string) error {
	_, err := s.table().Update(
		z.V{
			"upstream_id": upstreamID,
			"updated_at":  task.TimeToSQL(timeutil.NowTime()),
		},
		z.Fields("upstream_id", "updated_at"),
		z.Where(z.Eq("id", id)),
	)
	return err
}

// UpdateMessageID sets the message_id for a task (used when the assistant message is created after task creation).
func (s *TaskStore) UpdateMessageID(taskID, messageID string) error {
	_, err := s.table().Update(
		z.V{
			"message_id": messageID,
			"updated_at": task.TimeToSQL(timeutil.NowTime()),
		},
		z.Fields("message_id", "updated_at"),
		z.Where(z.Eq("id", taskID)),
	)
	return err
}

// UpdateFallbackInfo persists fallback metadata changes for an existing task.
func (s *TaskStore) UpdateFallbackInfo(taskID string, info *MediaFallbackInfo) error {
	payload := ""
	if info != nil {
		encoded, err := json.Marshal(info)
		if err != nil {
			return err
		}
		payload = string(encoded)
	}
	_, err := s.table().Update(
		z.V{
			"fallback_info": payload,
			"updated_at":    task.TimeToSQL(timeutil.NowTime()),
		},
		z.Fields("fallback_info", "updated_at"),
		z.Where(z.Eq("id", taskID)),
	)
	return err
}

// Get retrieves a single task by ID.
func (s *TaskStore) Get(id string, userID ...string) (*PersistentTask, error) {
	conds := []interface{}{z.Eq("id", id)}
	conds = append(conds, mediaTaskScopeConds(userID, "user_id")...)
	var rows []mediaTaskRow
	_, err := s.readTable().Select(&rows, z.Where(conds...), z.Limit(1))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rowToPersistentTask(rows[0]), nil
}

// GetByMessageID retrieves tasks associated with a message.
func (s *TaskStore) GetByMessageID(messageID string, userID ...string) ([]*PersistentTask, error) {
	conds := []interface{}{z.Eq("message_id", messageID)}
	conds = append(conds, mediaTaskScopeConds(userID, "user_id")...)
	var rows []mediaTaskRow
	_, err := s.readTable().Select(&rows,
		z.Where(conds...),
		z.OrderBy("created_at DESC"),
	)
	if err != nil {
		return nil, err
	}
	return rowsToPersistentTasks(rows), nil
}

// ListPending returns all non-terminal tasks (for power-failure recovery).
func (s *TaskStore) ListPending() ([]*PersistentTask, error) {
	var rows []mediaTaskRow
	_, err := s.readTable().Select(&rows,
		z.Where(z.In("status", string(TaskStatusPending), string(TaskStatusProcessing))),
		z.OrderBy("created_at ASC"),
	)
	if err != nil {
		return nil, err
	}
	return rowsToPersistentTasks(rows), nil
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
			UpdatedAt:   t.UpdatedAt,
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
	if t.FallbackInfo != "" {
		var info MediaFallbackInfo
		if json.Unmarshal([]byte(t.FallbackInfo), &info) == nil {
			mt.FallbackInfo = &info
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

	withScope := func(conds ...interface{}) []interface{} {
		result := append([]interface{}{}, conds...)
		result = append(result, mediaTaskScopeConds(userID, "user_id")...)
		return result
	}
	countByStatus := func(status TaskStatus) int64 {
		var total int64
		opts := []z.ZormItem{
			z.Fields("count(1)"),
			z.Where(withScope(z.Eq("status", string(status)))...),
		}
		if _, err := s.readTable().Select(&total, opts...); err != nil {
			return 0
		}
		return total
	}
	buildBuckets := func(valueExpr string, conds []interface{}, dest map[string]int64) {
		var rows []z.V
		opts := []z.ZormItem{
			z.Fields(valueExpr+" as value", "COUNT(*) as count"),
			z.Where(conds...),
			z.GroupBy(valueExpr),
		}
		if _, err := s.readTable().Select(&rows, opts...); err != nil {
			return
		}
		for i := range rows {
			value := strings.TrimSpace(mediaTaskStringFromMapValue(rows[i], "value"))
			if value == "" {
				continue
			}
			dest[value] = mediaTaskInt64FromMapValue(rows[i], "count")
		}
	}

	succeededConds := withScope(z.Eq("status", string(TaskStatusSucceeded)))

	// Count by status
	stats.Succeeded = countByStatus(TaskStatusSucceeded)
	stats.Failed = countByStatus(TaskStatusFailed)
	stats.TotalTasks = stats.Succeeded + stats.Failed

	// Count by type
	buildBuckets("type", succeededConds, stats.TasksByType)

	// Count by category
	buildBuckets("category", append(append([]interface{}{}, succeededConds...), z.Neq("category", "")), stats.TasksByCategory)

	// Count by provider
	buildBuckets("provider", append(append([]interface{}{}, succeededConds...), z.Neq("provider", "")), stats.TasksByProvider)

	// Calculate cost from succeeded tasks
	var costRows []mediaTaskCostRow
	if _, err := s.readTable().Select(&costRows,
		z.Fields("model", "provider", "response", "type"),
		z.Where(append(append([]interface{}{}, succeededConds...), z.Neq("response", ""))...),
	); err == nil {
		for i := range costRows {
			var resp MediaResponse
			if json.Unmarshal([]byte(costRows[i].Response), &resp) != nil {
				continue
			}
			imageCount := len(resp.Data)
			var durationSec float64
			for _, r := range resp.Data {
				durationSec += float64(r.DurationSec)
			}
			cost := CalculateMediaCost(costRows[i].Model, imageCount, durationSec)
			stats.TotalCostUSD += cost
			stats.CostByModel[costRows[i].Model] += cost
			if costRows[i].Provider != "" {
				stats.CostByProvider[costRows[i].Provider] += cost
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
	if mt.FallbackInfo != nil {
		if b, err := json.Marshal(mt.FallbackInfo); err == nil {
			t.FallbackInfo = string(b)
		}
	}
	return t
}
