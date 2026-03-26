package agent

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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
	db *sql.DB
}

// NewStore creates a new agent task store.
func NewStore(db *sql.DB) (*Store, error) {
	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, err
	}
	if err := ensureTaskColumns(db); err != nil {
		return nil, err
	}
	if err := ensureRuntimeTables(db); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Create inserts a new task.
func (s *Store) Create(ctx context.Context, task *Task) error {
	now := timeutil.NowTime()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_tasks (id, user_id, conversation_id, goal, plan, status, runtime_state, runtime_audit, success_criteria, fallback_plan, current_step, progress, result, verified_output, verification_errors_json, grounding_status, ground_state_json, metadata_json, error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.UserID, task.ConversationID, task.Goal,
		MarshalPlan(task.Plan), string(task.Status),
		string(task.RuntimeState), marshalAudit(task.RuntimeAudit), marshalStringSlice(task.SuccessCriteria), marshalStringSlice(task.FallbackPlan),
		task.CurrentStep, task.Progress, task.Result, task.VerifiedOutput, marshalStringSlice(task.VerificationErrors), task.GroundingStatus, marshalGroundTruthState(task.GroundState), marshalMetadataMap(task.Metadata), task.Error,
		task.CreatedAt, task.UpdatedAt,
	)
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
	var row *sql.Row
	if scopedUserID != "" {
		row = s.db.QueryRowContext(ctx,
			`SELECT id, user_id, conversation_id, goal, plan, status, runtime_state, runtime_audit, success_criteria, fallback_plan, current_step, progress, result, verified_output, verification_errors_json, grounding_status, ground_state_json, metadata_json, error, created_at, updated_at
			 FROM agent_tasks WHERE id = ? AND user_id = ?`, id, scopedUserID)
	} else {
		row = s.db.QueryRowContext(ctx,
			`SELECT id, user_id, conversation_id, goal, plan, status, runtime_state, runtime_audit, success_criteria, fallback_plan, current_step, progress, result, verified_output, verification_errors_json, grounding_status, ground_state_json, metadata_json, error, created_at, updated_at
			 FROM agent_tasks WHERE id = ?`, id)
	}
	return scanTask(row)
}

// ListByUser returns tasks for a user, ordered by creation time descending.
func (s *Store) ListByUser(ctx context.Context, userID string, limit int) ([]*Task, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, conversation_id, goal, plan, status, runtime_state, runtime_audit, success_criteria, fallback_plan, current_step, progress, result, verified_output, verification_errors_json, grounding_status, ground_state_json, metadata_json, error, created_at, updated_at
		 FROM agent_tasks WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// Update updates a task's mutable fields.
func (s *Store) Update(ctx context.Context, task *Task, userID ...string) error {
	task.UpdatedAt = timeutil.NowTime()
	scopedUserID := normalizeTaskScope(userID)
	var (
		res sql.Result
		err error
	)
	if scopedUserID != "" {
		res, err = s.db.ExecContext(ctx,
			`UPDATE agent_tasks SET goal=?, plan=?, status=?, runtime_state=?, runtime_audit=?, success_criteria=?, fallback_plan=?, current_step=?, progress=?, result=?, verified_output=?, verification_errors_json=?, grounding_status=?, ground_state_json=?, metadata_json=?, error=?, updated_at=?
			 WHERE id=? AND user_id=?`,
			task.Goal, MarshalPlan(task.Plan), string(task.Status),
			string(task.RuntimeState), marshalAudit(task.RuntimeAudit), marshalStringSlice(task.SuccessCriteria), marshalStringSlice(task.FallbackPlan),
			task.CurrentStep, task.Progress, task.Result, task.VerifiedOutput, marshalStringSlice(task.VerificationErrors), task.GroundingStatus, marshalGroundTruthState(task.GroundState), marshalMetadataMap(task.Metadata), task.Error,
			task.UpdatedAt, task.ID, scopedUserID,
		)
	} else {
		res, err = s.db.ExecContext(ctx,
			`UPDATE agent_tasks SET goal=?, plan=?, status=?, runtime_state=?, runtime_audit=?, success_criteria=?, fallback_plan=?, current_step=?, progress=?, result=?, verified_output=?, verification_errors_json=?, grounding_status=?, ground_state_json=?, metadata_json=?, error=?, updated_at=?
			 WHERE id=?`,
			task.Goal, MarshalPlan(task.Plan), string(task.Status),
			string(task.RuntimeState), marshalAudit(task.RuntimeAudit), marshalStringSlice(task.SuccessCriteria), marshalStringSlice(task.FallbackPlan),
			task.CurrentStep, task.Progress, task.Result, task.VerifiedOutput, marshalStringSlice(task.VerificationErrors), task.GroundingStatus, marshalGroundTruthState(task.GroundState), marshalMetadataMap(task.Metadata), task.Error,
			task.UpdatedAt, task.ID,
		)
	}
	if err != nil {
		return err
	}
	if scopedUserID != "" {
		if affected, rowsErr := res.RowsAffected(); rowsErr == nil && affected == 0 {
			return sql.ErrNoRows
		}
	}
	return err
}

// Delete removes a task.
func (s *Store) Delete(ctx context.Context, id string, userID ...string) error {
	scopedUserID := normalizeTaskScope(userID)
	var (
		res sql.Result
		err error
	)
	if scopedUserID != "" {
		res, err = s.db.ExecContext(ctx, `DELETE FROM agent_tasks WHERE id=? AND user_id=?`, id, scopedUserID)
	} else {
		res, err = s.db.ExecContext(ctx, `DELETE FROM agent_tasks WHERE id=?`, id)
	}
	if err != nil {
		return err
	}
	if scopedUserID != "" {
		if affected, rowsErr := res.RowsAffected(); rowsErr == nil && affected == 0 {
			return sql.ErrNoRows
		}
	}
	return err
}

func scanTask(row *sql.Row) (*Task, error) {
	var t Task
	var planJSON, status, runtimeState, runtimeAudit, successCriteria, fallbackPlan string
	var verificationErrors, groundingStatus, groundStateJSON, metadataJSON string
	err := row.Scan(&t.ID, &t.UserID, &t.ConversationID, &t.Goal,
		&planJSON, &status, &runtimeState, &runtimeAudit, &successCriteria, &fallbackPlan, &t.CurrentStep, &t.Progress,
		&t.Result, &t.VerifiedOutput, &verificationErrors, &groundingStatus, &groundStateJSON, &metadataJSON, &t.Error, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.Status = TaskStatus(status)
	t.RuntimeState = RuntimeState(runtimeState)
	t.Plan = UnmarshalPlan(planJSON)
	t.RuntimeAudit = unmarshalAudit(runtimeAudit)
	t.SuccessCriteria = unmarshalStringSlice(successCriteria)
	t.FallbackPlan = unmarshalStringSlice(fallbackPlan)
	t.VerificationErrors = unmarshalStringSlice(verificationErrors)
	t.GroundingStatus = groundingStatus
	t.GroundState = unmarshalGroundTruthState(groundStateJSON)
	t.Metadata = unmarshalMetadataMap(metadataJSON)
	return &t, nil
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanTaskRows(rows *sql.Rows) (*Task, error) {
	var t Task
	var planJSON, status, runtimeState, runtimeAudit, successCriteria, fallbackPlan string
	var verificationErrors, groundingStatus, groundStateJSON, metadataJSON string
	err := rows.Scan(&t.ID, &t.UserID, &t.ConversationID, &t.Goal,
		&planJSON, &status, &runtimeState, &runtimeAudit, &successCriteria, &fallbackPlan, &t.CurrentStep, &t.Progress,
		&t.Result, &t.VerifiedOutput, &verificationErrors, &groundingStatus, &groundStateJSON, &metadataJSON, &t.Error, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.Status = TaskStatus(status)
	t.RuntimeState = RuntimeState(runtimeState)
	t.Plan = UnmarshalPlan(planJSON)
	t.RuntimeAudit = unmarshalAudit(runtimeAudit)
	t.SuccessCriteria = unmarshalStringSlice(successCriteria)
	t.FallbackPlan = unmarshalStringSlice(fallbackPlan)
	t.VerificationErrors = unmarshalStringSlice(verificationErrors)
	t.GroundingStatus = groundingStatus
	t.GroundState = unmarshalGroundTruthState(groundStateJSON)
	t.Metadata = unmarshalMetadataMap(metadataJSON)
	return &t, nil
}

// CountRunning returns the number of running tasks for a user.
func (s *Store) CountRunning(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM agent_tasks WHERE user_id=? AND status IN ('pending','planning','executing','waiting_input')`,
		userID).Scan(&count)
	return count, err
}

// SetStatus is a convenience method to update just the status.
func (s *Store) SetStatus(ctx context.Context, id string, status TaskStatus, errMsg string, userID ...string) error {
	now := timeutil.NowTime()
	scopedUserID := normalizeTaskScope(userID)
	var (
		res sql.Result
		err error
	)
	if scopedUserID != "" {
		res, err = s.db.ExecContext(ctx,
			`UPDATE agent_tasks SET status=?, error=?, updated_at=? WHERE id=? AND user_id=?`,
			string(status), errMsg, now, id, scopedUserID)
	} else {
		res, err = s.db.ExecContext(ctx,
			`UPDATE agent_tasks SET status=?, error=?, updated_at=? WHERE id=?`,
			string(status), errMsg, now, id)
	}
	if err != nil {
		return err
	}
	if scopedUserID != "" {
		if affected, rowsErr := res.RowsAffected(); rowsErr == nil && affected == 0 {
			return sql.ErrNoRows
		}
	}
	return err
}

// Cleanup removes tasks older than the given duration.
func (s *Store) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := timeutil.NowTime().Add(-olderThan)
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM agent_tasks WHERE created_at < ? AND status IN ('completed','failed','cancelled','aborted')`,
		cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// RecoverStaleTasks marks any tasks stuck in running states as failed.
// Call this on startup to clean up tasks from a previous crash.
func (s *Store) RecoverStaleTasks(ctx context.Context) (int64, error) {
	now := timeutil.NowTime()
	result, err := s.db.ExecContext(ctx,
		`UPDATE agent_tasks SET status='failed', error='interrupted by server restart', updated_at=?
		 WHERE status IN ('pending','planning','executing','waiting_input')`, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
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
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_runtime_events (id, task_id, step_index, planner_round, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.TaskID, event.StepIndex, event.PlannerRound, event.EventType, event.PayloadJSON, event.CreatedAt,
	)
	return err
}

func (s *Store) ListRuntimeEvents(ctx context.Context, taskID string) ([]RuntimeEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_id, step_index, planner_round, event_type, payload_json, created_at
		 FROM agent_runtime_events WHERE task_id = ? ORDER BY created_at ASC, id ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RuntimeEvent
	for rows.Next() {
		var event RuntimeEvent
		if err := rows.Scan(&event.ID, &event.TaskID, &event.StepIndex, &event.PlannerRound, &event.EventType, &event.PayloadJSON, &event.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}
