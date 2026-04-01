package agent

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS agent_tasks (
	id              TEXT PRIMARY KEY,
	user_id         TEXT NOT NULL,
	conversation_id TEXT DEFAULT '',
	goal            TEXT NOT NULL,
	plan            TEXT DEFAULT '[]',
	status          TEXT NOT NULL DEFAULT 'pending',
	runtime_state   TEXT DEFAULT '',
	runtime_audit   TEXT DEFAULT '[]',
	success_criteria TEXT DEFAULT '[]',
	fallback_plan   TEXT DEFAULT '[]',
	current_step    INTEGER DEFAULT 0,
	progress        INTEGER DEFAULT 0,
	result          TEXT DEFAULT '',
	verified_output TEXT DEFAULT '',
	verification_errors_json TEXT DEFAULT '[]',
	grounding_status TEXT DEFAULT '',
	ground_state_json TEXT DEFAULT '',
	metadata_json   TEXT DEFAULT '{}',
	error           TEXT DEFAULT '',
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_user ON agent_tasks(user_id);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_status ON agent_tasks(status);

CREATE TABLE IF NOT EXISTS agent_runtime_events (
	id           TEXT PRIMARY KEY,
	task_id      TEXT NOT NULL,
	step_index   INTEGER DEFAULT 0,
	planner_round INTEGER DEFAULT 0,
	event_type   TEXT NOT NULL,
	payload_json TEXT NOT NULL DEFAULT '{}',
	created_at   DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_runtime_events_task ON agent_runtime_events(task_id, created_at);
`

// Store persists agent tasks in SQLite.
type Store struct {
	db     *sql.DB
	readDB *sql.DB
}

// NewStore creates a new agent task store.
func NewStore(db *sql.DB) (*Store, error) {
	return NewStoreWithReadDB(db, db)
}

// NewStoreWithReadDB creates a new agent task store with separate write and
// read database handles.
func NewStoreWithReadDB(writeDB, readDB *sql.DB) (*Store, error) {
	if readDB == nil {
		readDB = writeDB
	}
	if _, err := writeDB.Exec(createTableSQL); err != nil {
		return nil, err
	}
	if err := ensureTaskColumns(writeDB); err != nil {
		return nil, err
	}
	if err := ensureRuntimeTables(writeDB); err != nil {
		return nil, err
	}
	return &Store{db: writeDB, readDB: readDB}, nil
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

func (s *Store) table(ctx context.Context, name string) *z.ZormTable {
	return z.TableContext(ctx, s.db, name)
}

func (s *Store) readTable(ctx context.Context, name string) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), name)
}

type agentTaskRow struct {
	ID                     string `json:"id" zorm:"id"`
	UserID                 string `json:"user_id" zorm:"user_id"`
	ConversationID         string `json:"conversation_id" zorm:"conversation_id"`
	Goal                   string `json:"goal" zorm:"goal"`
	Plan                   string `json:"plan" zorm:"plan"`
	Status                 string `json:"status" zorm:"status"`
	RuntimeState           string `json:"runtime_state" zorm:"runtime_state"`
	RuntimeAudit           string `json:"runtime_audit" zorm:"runtime_audit"`
	SuccessCriteria        string `json:"success_criteria" zorm:"success_criteria"`
	FallbackPlan           string `json:"fallback_plan" zorm:"fallback_plan"`
	CurrentStep            int    `json:"current_step" zorm:"current_step"`
	Progress               int    `json:"progress" zorm:"progress"`
	Result                 string `json:"result" zorm:"result"`
	VerifiedOutput         string `json:"verified_output" zorm:"verified_output"`
	VerificationErrorsJSON string `json:"verification_errors_json" zorm:"verification_errors_json"`
	GroundingStatus        string `json:"grounding_status" zorm:"grounding_status"`
	GroundStateJSON        string `json:"ground_state_json" zorm:"ground_state_json"`
	MetadataJSON           string `json:"metadata_json" zorm:"metadata_json"`
	Error                  string `json:"error" zorm:"error"`
	CreatedAt              string `json:"created_at" zorm:"created_at"`
	UpdatedAt              string `json:"updated_at" zorm:"updated_at"`
}

type runtimeEventRow struct {
	ID           string `json:"id" zorm:"id"`
	TaskID       string `json:"task_id" zorm:"task_id"`
	StepIndex    int    `json:"step_index" zorm:"step_index"`
	PlannerRound int    `json:"planner_round" zorm:"planner_round"`
	EventType    string `json:"event_type" zorm:"event_type"`
	PayloadJSON  string `json:"payload_json" zorm:"payload_json"`
	CreatedAt    string `json:"created_at" zorm:"created_at"`
}

func taskValues(task *Task) z.V {
	return z.V{
		"id":                       task.ID,
		"user_id":                  task.UserID,
		"conversation_id":          task.ConversationID,
		"goal":                     task.Goal,
		"plan":                     MarshalPlan(task.Plan),
		"status":                   string(task.Status),
		"runtime_state":            string(task.RuntimeState),
		"runtime_audit":            marshalAudit(task.RuntimeAudit),
		"success_criteria":         marshalStringSlice(task.SuccessCriteria),
		"fallback_plan":            marshalStringSlice(task.FallbackPlan),
		"current_step":             task.CurrentStep,
		"progress":                 task.Progress,
		"result":                   task.Result,
		"verified_output":          task.VerifiedOutput,
		"verification_errors_json": marshalStringSlice(task.VerificationErrors),
		"grounding_status":         task.GroundingStatus,
		"ground_state_json":        marshalGroundTruthState(task.GroundState),
		"metadata_json":            marshalMetadataMap(task.Metadata),
		"error":                    task.Error,
		"created_at":               task.CreatedAt,
		"updated_at":               task.UpdatedAt,
	}
}

func runtimeEventValues(event RuntimeEvent) z.V {
	return z.V{
		"id":            event.ID,
		"task_id":       event.TaskID,
		"step_index":    event.StepIndex,
		"planner_round": event.PlannerRound,
		"event_type":    event.EventType,
		"payload_json":  event.PayloadJSON,
		"created_at":    event.CreatedAt,
	}
}

func parseAgentTime(raw string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func rowToTask(row agentTaskRow) *Task {
	task := &Task{
		ID:                 row.ID,
		UserID:             row.UserID,
		ConversationID:     row.ConversationID,
		Goal:               row.Goal,
		Plan:               UnmarshalPlan(row.Plan),
		Status:             TaskStatus(row.Status),
		RuntimeState:       RuntimeState(row.RuntimeState),
		RuntimeAudit:       unmarshalAudit(row.RuntimeAudit),
		SuccessCriteria:    unmarshalStringSlice(row.SuccessCriteria),
		FallbackPlan:       unmarshalStringSlice(row.FallbackPlan),
		CurrentStep:        row.CurrentStep,
		Progress:           row.Progress,
		Result:             row.Result,
		VerifiedOutput:     row.VerifiedOutput,
		VerificationErrors: unmarshalStringSlice(row.VerificationErrorsJSON),
		GroundingStatus:    row.GroundingStatus,
		GroundState:        unmarshalGroundTruthState(row.GroundStateJSON),
		Metadata:           unmarshalMetadataMap(row.MetadataJSON),
		Error:              row.Error,
		CreatedAt:          parseAgentTime(row.CreatedAt),
		UpdatedAt:          parseAgentTime(row.UpdatedAt),
	}
	return task
}

func rowsToTasks(rows []agentTaskRow) []*Task {
	tasks := make([]*Task, 0, len(rows))
	for i := range rows {
		tasks = append(tasks, rowToTask(rows[i]))
	}
	return tasks
}

func rowToRuntimeEvent(row runtimeEventRow) RuntimeEvent {
	return RuntimeEvent{
		ID:           row.ID,
		TaskID:       row.TaskID,
		StepIndex:    row.StepIndex,
		PlannerRound: row.PlannerRound,
		EventType:    row.EventType,
		PayloadJSON:  row.PayloadJSON,
		CreatedAt:    parseAgentTime(row.CreatedAt),
	}
}

// Create inserts a new task.
func (s *Store) Create(ctx context.Context, task *Task) error {
	now := timeutil.NowTime()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	_, err := s.table(ctx, "agent_tasks").Insert(taskValues(task))
	return err
}

func normalizeTaskScope(userID []string) string {
	if len(userID) == 0 {
		return ""
	}
	return strings.TrimSpace(userID[0])
}

// Get retrieves a task by ID.
func (s *Store) Get(ctx context.Context, id string, userID ...string) (*Task, error) {
	scopedUserID := normalizeTaskScope(userID)
	conds := []interface{}{z.Eq("id", id)}
	if scopedUserID != "" {
		conds = append(conds, z.Eq("user_id", scopedUserID))
	}
	var rows []agentTaskRow
	_, err := s.readTable(ctx, "agent_tasks").Select(&rows, z.Where(conds...), z.Limit(1))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rowToTask(rows[0]), nil
}

// ListByUser returns tasks for a user, ordered by creation time descending.
func (s *Store) ListByUser(ctx context.Context, userID string, limit int) ([]*Task, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []agentTaskRow
	_, err := s.readTable(ctx, "agent_tasks").Select(&rows,
		z.Where(z.Eq("user_id", userID)),
		z.OrderBy("created_at DESC"),
		z.Limit(limit),
	)
	if err != nil {
		return nil, err
	}
	return rowsToTasks(rows), nil
}

// Update updates a task's mutable fields.
func (s *Store) Update(ctx context.Context, task *Task, userID ...string) error {
	task.UpdatedAt = timeutil.NowTime()
	scopedUserID := normalizeTaskScope(userID)
	conds := []interface{}{z.Eq("id", task.ID)}
	if scopedUserID != "" {
		conds = append(conds, z.Eq("user_id", scopedUserID))
	}
	affected, err := s.table(ctx, "agent_tasks").Update(
		taskValues(task),
		z.Fields(
			"goal",
			"plan",
			"status",
			"runtime_state",
			"runtime_audit",
			"success_criteria",
			"fallback_plan",
			"current_step",
			"progress",
			"result",
			"verified_output",
			"verification_errors_json",
			"grounding_status",
			"ground_state_json",
			"metadata_json",
			"error",
			"updated_at",
		),
		z.Where(conds...),
	)
	if err != nil {
		return err
	}
	if scopedUserID != "" && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete removes a task.
func (s *Store) Delete(ctx context.Context, id string, userID ...string) error {
	scopedUserID := normalizeTaskScope(userID)
	conds := []interface{}{z.Eq("id", id)}
	if scopedUserID != "" {
		conds = append(conds, z.Eq("user_id", scopedUserID))
	}
	affected, err := s.table(ctx, "agent_tasks").Delete(z.Where(conds...))
	if err != nil {
		return err
	}
	if scopedUserID != "" && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CountRunning returns the number of running tasks for a user.
func (s *Store) CountRunning(ctx context.Context, userID string) (int, error) {
	var count int64
	_, err := s.readTable(ctx, "agent_tasks").Select(
		&count,
		z.Fields("count(1)"),
		z.Where(
			z.Eq("user_id", userID),
			z.In(
				"status",
				string(TaskStatusPending),
				string(TaskStatusPlanning),
				string(TaskStatusExecuting),
				string(TaskStatusWaitingInput),
			),
		),
	)
	return int(count), err
}

// SetStatus is a convenience method to update just the status.
func (s *Store) SetStatus(ctx context.Context, id string, status TaskStatus, errMsg string, userID ...string) error {
	now := timeutil.NowTime()
	scopedUserID := normalizeTaskScope(userID)
	conds := []interface{}{z.Eq("id", id)}
	if scopedUserID != "" {
		conds = append(conds, z.Eq("user_id", scopedUserID))
	}
	affected, err := s.table(ctx, "agent_tasks").Update(
		z.V{
			"status":     string(status),
			"error":      errMsg,
			"updated_at": now,
		},
		z.Fields("status", "error", "updated_at"),
		z.Where(conds...),
	)
	if err != nil {
		return err
	}
	if scopedUserID != "" && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Cleanup removes tasks older than the given duration.
func (s *Store) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := timeutil.NowTime().Add(-olderThan)
	affected, err := s.table(ctx, "agent_tasks").Delete(
		z.Where(
			z.Expr("created_at < ?", cutoff),
			z.In(
				"status",
				string(TaskStatusCompleted),
				string(TaskStatusFailed),
				string(TaskStatusCancelled),
				string(TaskStatusAborted),
			),
		),
	)
	if err != nil {
		return 0, err
	}
	return int64(affected), nil
}

// RecoverStaleTasks marks any tasks stuck in running states as failed.
// Call this on startup to clean up tasks from a previous crash.
func (s *Store) RecoverStaleTasks(ctx context.Context) (int64, error) {
	now := timeutil.NowTime()
	affected, err := s.table(ctx, "agent_tasks").Update(
		z.V{
			"status":     string(TaskStatusFailed),
			"error":      "interrupted by server restart",
			"updated_at": now,
		},
		z.Fields("status", "error", "updated_at"),
		z.Where(
			z.In(
				"status",
				string(TaskStatusPending),
				string(TaskStatusPlanning),
				string(TaskStatusExecuting),
				string(TaskStatusWaitingInput),
			),
		),
	)
	if err != nil {
		return 0, err
	}
	return int64(affected), nil
}

func ensureTaskColumns(db *sql.DB) error {
	type colSpec struct {
		name       string
		definition string
	}
	cols := []colSpec{
		{name: "runtime_state", definition: "TEXT DEFAULT ''"},
		{name: "runtime_audit", definition: "TEXT DEFAULT '[]'"},
		{name: "success_criteria", definition: "TEXT DEFAULT '[]'"},
		{name: "fallback_plan", definition: "TEXT DEFAULT '[]'"},
		{name: "verified_output", definition: "TEXT DEFAULT ''"},
		{name: "verification_errors_json", definition: "TEXT DEFAULT '[]'"},
		{name: "grounding_status", definition: "TEXT DEFAULT ''"},
		{name: "ground_state_json", definition: "TEXT DEFAULT ''"},
		{name: "metadata_json", definition: "TEXT DEFAULT '{}'"},
	}
	existing := map[string]struct{}{}
	rows, err := db.Query(`PRAGMA table_info(agent_tasks)`)
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
	for _, c := range cols {
		if _, ok := existing[c.name]; ok {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE agent_tasks ADD COLUMN ` + c.name + ` ` + c.definition); err != nil {
			return err
		}
	}
	return nil
}

func ensureRuntimeTables(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS agent_runtime_events (
	id            TEXT PRIMARY KEY,
	task_id       TEXT NOT NULL,
	step_index    INTEGER DEFAULT 0,
	planner_round INTEGER DEFAULT 0,
	event_type    TEXT NOT NULL,
	payload_json  TEXT NOT NULL DEFAULT '{}',
	created_at    DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_runtime_events_task ON agent_runtime_events(task_id, created_at);
`)
	return err
}

func (s *Store) AppendRuntimeEvent(ctx context.Context, event RuntimeEvent) error {
	if strings.TrimSpace(event.ID) == "" {
		return sql.ErrNoRows
	}
	_, err := s.table(ctx, "agent_runtime_events").Insert(runtimeEventValues(event))
	return err
}

func (s *Store) ListRuntimeEvents(ctx context.Context, taskID string) ([]RuntimeEvent, error) {
	var rows []runtimeEventRow
	_, err := s.readTable(ctx, "agent_runtime_events").Select(
		&rows,
		z.Where(z.Eq("task_id", taskID)),
		z.OrderBy("created_at ASC, id ASC"),
	)
	if err != nil {
		return nil, err
	}
	out := make([]RuntimeEvent, 0, len(rows))
	for i := range rows {
		out = append(out, rowToRuntimeEvent(rows[i]))
	}
	return out, nil
}
