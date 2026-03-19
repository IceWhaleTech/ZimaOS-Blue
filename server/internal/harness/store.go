package harness

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	agentpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS harness_runs (
	id TEXT PRIMARY KEY,
	root_run_id TEXT NOT NULL,
	parent_run_id TEXT DEFAULT '',
	kind TEXT NOT NULL,
	status TEXT NOT NULL,
	runtime_state TEXT DEFAULT '',
	user_id TEXT DEFAULT '',
	conversation_id TEXT DEFAULT '',
	session_id TEXT DEFAULT '',
	agent_id TEXT DEFAULT '',
	goal TEXT NOT NULL,
	model TEXT DEFAULT '',
	result TEXT DEFAULT '',
	error TEXT DEFAULT '',
	depth INTEGER NOT NULL DEFAULT 0,
	current_step INTEGER NOT NULL DEFAULT 0,
	progress INTEGER NOT NULL DEFAULT 0,
	workspace_root TEXT DEFAULT '',
	artifact_root TEXT DEFAULT '',
	sandbox_mode TEXT DEFAULT '',
	approval_mode TEXT DEFAULT '',
	max_duration_ns INTEGER NOT NULL DEFAULT 0,
	max_steps INTEGER NOT NULL DEFAULT 0,
	max_tool_rounds INTEGER NOT NULL DEFAULT 0,
	max_subagents INTEGER NOT NULL DEFAULT 0,
	max_depth INTEGER NOT NULL DEFAULT 0,
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	started_at DATETIME,
	finished_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_harness_runs_root ON harness_runs(root_run_id);
CREATE INDEX IF NOT EXISTS idx_harness_runs_parent ON harness_runs(parent_run_id);
CREATE INDEX IF NOT EXISTS idx_harness_runs_user ON harness_runs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_runs_kind ON harness_runs(kind, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_runs_status ON harness_runs(status, created_at DESC);

CREATE TABLE IF NOT EXISTS harness_run_events (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	root_run_id TEXT DEFAULT '',
	parent_run_id TEXT DEFAULT '',
	type TEXT NOT NULL,
	step_index INTEGER NOT NULL DEFAULT 0,
	tool_name TEXT DEFAULT '',
	capability_kind TEXT DEFAULT '',
	message TEXT DEFAULT '',
	payload_json TEXT DEFAULT '',
	created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_harness_run_events_run ON harness_run_events(run_id, created_at ASC, id ASC);

CREATE TABLE IF NOT EXISTS harness_artifacts (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	label TEXT DEFAULT '',
	path_or_url TEXT DEFAULT '',
	mime_type TEXT DEFAULT '',
	size_bytes INTEGER NOT NULL DEFAULT 0,
	metadata_json TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_harness_artifacts_run ON harness_artifacts(run_id);
`

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(db *sql.DB) (*SQLiteStore, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) CreateRun(ctx context.Context, run *Run) error {
	if run == nil {
		return fmt.Errorf("run is required")
	}
	now := timeutil.NowTime()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = run.CreatedAt
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO harness_runs (
		id, root_run_id, parent_run_id, kind, status, runtime_state, user_id, conversation_id, session_id, agent_id,
		goal, model, result, error, depth, current_step, progress, workspace_root, artifact_root, sandbox_mode,
		approval_mode, max_duration_ns, max_steps, max_tool_rounds, max_subagents, max_depth, metadata_json,
		created_at, updated_at, started_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.RootRunID, run.ParentRunID, string(run.Kind), string(run.Status), string(run.RuntimeState), run.UserID,
		run.ConversationID, run.SessionID, run.AgentID, run.Goal, run.Model, run.Result, run.Error, run.Depth,
		run.CurrentStep, run.Progress, run.WorkspaceRoot, run.ArtifactRoot, run.SandboxMode, string(run.ApprovalMode),
		run.MaxDuration.Nanoseconds(), run.MaxSteps, run.MaxToolRounds, run.MaxSubagents, run.MaxDepth, marshalMetadata(run.Metadata),
		run.CreatedAt, run.UpdatedAt, nullableTime(run.StartedAt), nullableTime(run.FinishedAt),
	)
	return err
}

func (s *SQLiteStore) UpdateRun(ctx context.Context, run *Run) error {
	if run == nil {
		return fmt.Errorf("run is required")
	}
	run.UpdatedAt = timeutil.NowTime()
	_, err := s.db.ExecContext(ctx, `UPDATE harness_runs SET
		root_run_id=?, parent_run_id=?, kind=?, status=?, runtime_state=?, user_id=?, conversation_id=?, session_id=?, agent_id=?,
		goal=?, model=?, result=?, error=?, depth=?, current_step=?, progress=?, workspace_root=?, artifact_root=?, sandbox_mode=?,
		approval_mode=?, max_duration_ns=?, max_steps=?, max_tool_rounds=?, max_subagents=?, max_depth=?, metadata_json=?,
		updated_at=?, started_at=?, finished_at=?
		WHERE id=?`,
		run.RootRunID, run.ParentRunID, string(run.Kind), string(run.Status), string(run.RuntimeState), run.UserID,
		run.ConversationID, run.SessionID, run.AgentID, run.Goal, run.Model, run.Result, run.Error, run.Depth,
		run.CurrentStep, run.Progress, run.WorkspaceRoot, run.ArtifactRoot, run.SandboxMode, string(run.ApprovalMode),
		run.MaxDuration.Nanoseconds(), run.MaxSteps, run.MaxToolRounds, run.MaxSubagents, run.MaxDepth, marshalMetadata(run.Metadata),
		run.UpdatedAt, nullableTime(run.StartedAt), nullableTime(run.FinishedAt), run.ID,
	)
	return err
}

func (s *SQLiteStore) GetRun(ctx context.Context, id string) (*Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, root_run_id, parent_run_id, kind, status, runtime_state, user_id, conversation_id, session_id, agent_id,
		goal, model, result, error, depth, current_step, progress, workspace_root, artifact_root, sandbox_mode,
		approval_mode, max_duration_ns, max_steps, max_tool_rounds, max_subagents, max_depth, metadata_json,
		created_at, updated_at, started_at, finished_at
		FROM harness_runs WHERE id = ?`, id)
	return scanRun(row)
}

func (s *SQLiteStore) ListRuns(ctx context.Context, filter RunFilter) ([]Run, error) {
	query := `SELECT
		id, root_run_id, parent_run_id, kind, status, runtime_state, user_id, conversation_id, session_id, agent_id,
		goal, model, result, error, depth, current_step, progress, workspace_root, artifact_root, sandbox_mode,
		approval_mode, max_duration_ns, max_steps, max_tool_rounds, max_subagents, max_depth, metadata_json,
		created_at, updated_at, started_at, finished_at
		FROM harness_runs`
	var (
		clauses []string
		args    []interface{}
	)
	if v := strings.TrimSpace(filter.UserID); v != "" {
		clauses = append(clauses, "user_id = ?")
		args = append(args, v)
	}
	if filter.Kind != "" {
		clauses = append(clauses, "kind = ?")
		args = append(args, string(filter.Kind))
	} else if len(filter.Kinds) > 0 {
		parts := make([]string, 0, len(filter.Kinds))
		for _, kind := range filter.Kinds {
			if kind == "" {
				continue
			}
			parts = append(parts, "?")
			args = append(args, string(kind))
		}
		if len(parts) > 0 {
			clauses = append(clauses, "kind IN ("+strings.Join(parts, ",")+")")
		}
	}
	if len(filter.Statuses) > 0 {
		parts := make([]string, 0, len(filter.Statuses))
		for _, st := range filter.Statuses {
			if st == "" {
				continue
			}
			parts = append(parts, "?")
			args = append(args, string(st))
		}
		if len(parts) > 0 {
			clauses = append(clauses, "status IN ("+strings.Join(parts, ",")+")")
		}
	}
	if v := strings.TrimSpace(filter.ParentRunID); v != "" {
		clauses = append(clauses, "parent_run_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.RootRunID); v != "" {
		clauses = append(clauses, "root_run_id = ?")
		args = append(args, v)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC"
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query += " LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		run, err := scanRunRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *run)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) AppendEvent(ctx context.Context, event RunEvent) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO harness_run_events (
		id, run_id, root_run_id, parent_run_id, type, step_index, tool_name, capability_kind, message, payload_json, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.RunID, event.RootRunID, event.ParentRunID, event.Type, event.StepIndex, event.ToolName,
		event.CapabilityKind, event.Message, event.PayloadJSON, event.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) ListEvents(ctx context.Context, runID string, limit int) ([]RunEvent, error) {
	if limit <= 0 {
		limit = defaultEventLimit
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, run_id, root_run_id, parent_run_id, type, step_index, tool_name, capability_kind, message, payload_json, created_at
		FROM harness_run_events WHERE run_id = ? ORDER BY created_at ASC, id ASC LIMIT ?`, runID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunEvent
	for rows.Next() {
		var ev RunEvent
		if err := rows.Scan(&ev.ID, &ev.RunID, &ev.RootRunID, &ev.ParentRunID, &ev.Type, &ev.StepIndex, &ev.ToolName, &ev.CapabilityKind, &ev.Message, &ev.PayloadJSON, &ev.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) AttachArtifact(ctx context.Context, ref ArtifactRef) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO harness_artifacts (
		id, run_id, kind, label, path_or_url, mime_type, size_bytes, metadata_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ref.ID, ref.RunID, ref.Kind, ref.Label, ref.PathOrURL, ref.MIMEType, ref.SizeBytes, ref.MetadataJSON,
	)
	return err
}

func (s *SQLiteStore) ListArtifacts(ctx context.Context, runID string) ([]ArtifactRef, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, run_id, kind, label, path_or_url, mime_type, size_bytes, metadata_json
		FROM harness_artifacts WHERE run_id = ? ORDER BY id ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArtifactRef
	for rows.Next() {
		var ref ArtifactRef
		if err := rows.Scan(&ref.ID, &ref.RunID, &ref.Kind, &ref.Label, &ref.PathOrURL, &ref.MIMEType, &ref.SizeBytes, &ref.MetadataJSON); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) DeleteRun(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `DELETE FROM harness_run_events WHERE run_id = ?`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM harness_artifacts WHERE run_id = ?`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM harness_runs WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func nullableTime(value *time.Time) interface{} {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanRun(scanner rowScanner) (*Run, error) {
	var (
		run                   Run
		kind, status          string
		runtimeState          string
		approvalMode          string
		metadataJSON          string
		maxDurationNs         int64
		startedAt, finishedAt sql.NullTime
	)
	err := scanner.Scan(
		&run.ID, &run.RootRunID, &run.ParentRunID, &kind, &status, &runtimeState, &run.UserID, &run.ConversationID, &run.SessionID, &run.AgentID,
		&run.Goal, &run.Model, &run.Result, &run.Error, &run.Depth, &run.CurrentStep, &run.Progress, &run.WorkspaceRoot, &run.ArtifactRoot, &run.SandboxMode,
		&approvalMode, &maxDurationNs, &run.MaxSteps, &run.MaxToolRounds, &run.MaxSubagents, &run.MaxDepth, &metadataJSON,
		&run.CreatedAt, &run.UpdatedAt, &startedAt, &finishedAt,
	)
	if err != nil {
		return nil, err
	}
	run.Kind = RunKind(kind)
	run.Status = RunStatus(status)
	run.RuntimeState = agentpkg.RuntimeState(runtimeState)
	run.ApprovalMode = ApprovalMode(approvalMode)
	run.MaxDuration = time.Duration(maxDurationNs)
	run.Metadata = unmarshalMetadata(metadataJSON)
	if startedAt.Valid {
		ts := startedAt.Time
		run.StartedAt = &ts
	}
	if finishedAt.Valid {
		ts := finishedAt.Time
		run.FinishedAt = &ts
	}
	return &run, nil
}

func scanRunRows(rows *sql.Rows) (*Run, error) {
	return scanRun(rows)
}
