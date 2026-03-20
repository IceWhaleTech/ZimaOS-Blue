package harness

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	agentpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const baseSchemaSQL = `
CREATE TABLE IF NOT EXISTS harness_runs (
	id TEXT PRIMARY KEY,
	root_run_id TEXT NOT NULL,
	parent_run_id TEXT DEFAULT '',
	group_id TEXT DEFAULT '',
	group_item_id TEXT DEFAULT '',
	attempt_index INTEGER NOT NULL DEFAULT 0,
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

CREATE TABLE IF NOT EXISTS harness_run_groups (
	id TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	title TEXT DEFAULT '',
	status TEXT NOT NULL,
	owner_user_id TEXT DEFAULT '',
	subject TEXT DEFAULT '',
	scheduler_json TEXT NOT NULL DEFAULT '{}',
	scoring_json TEXT NOT NULL DEFAULT '{}',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	summary_json TEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	started_at DATETIME,
	finished_at DATETIME
);

CREATE TABLE IF NOT EXISTS harness_run_group_items (
	id TEXT PRIMARY KEY,
	group_id TEXT NOT NULL,
	item_index INTEGER NOT NULL DEFAULT 0,
	run_kind TEXT NOT NULL,
	profile TEXT DEFAULT '',
	input_json TEXT NOT NULL DEFAULT '{}',
	expected_json TEXT NOT NULL DEFAULT '{}',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	status TEXT NOT NULL,
	latest_run_id TEXT DEFAULT '',
	attempt_count INTEGER NOT NULL DEFAULT 0,
	max_attempts INTEGER NOT NULL DEFAULT 0,
	lease_owner TEXT DEFAULT '',
	lease_expires_at DATETIME,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS harness_scorecards (
	id TEXT PRIMARY KEY,
	group_id TEXT NOT NULL,
	group_item_id TEXT NOT NULL,
	run_id TEXT DEFAULT '',
	mode TEXT NOT NULL,
	verdict TEXT NOT NULL,
	score REAL NOT NULL DEFAULT 0,
	breakdown_json TEXT DEFAULT '',
	evidence_json TEXT DEFAULT '',
	judge_trace_json TEXT DEFAULT '',
	created_at DATETIME NOT NULL
);
`

const indexSchemaSQL = `
CREATE INDEX IF NOT EXISTS idx_harness_runs_root ON harness_runs(root_run_id);
CREATE INDEX IF NOT EXISTS idx_harness_runs_parent ON harness_runs(parent_run_id);
CREATE INDEX IF NOT EXISTS idx_harness_runs_group ON harness_runs(group_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_runs_group_item ON harness_runs(group_item_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_runs_user ON harness_runs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_runs_kind ON harness_runs(kind, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_runs_status ON harness_runs(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_run_events_run ON harness_run_events(run_id, created_at ASC, id ASC);
CREATE INDEX IF NOT EXISTS idx_harness_artifacts_run ON harness_artifacts(run_id);
CREATE INDEX IF NOT EXISTS idx_harness_run_groups_owner ON harness_run_groups(owner_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_run_groups_status ON harness_run_groups(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_run_groups_kind ON harness_run_groups(kind, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_harness_run_group_items_group_index ON harness_run_group_items(group_id, item_index);
CREATE INDEX IF NOT EXISTS idx_harness_run_group_items_status ON harness_run_group_items(group_id, status, item_index ASC);
CREATE INDEX IF NOT EXISTS idx_harness_run_group_items_lease ON harness_run_group_items(group_id, lease_expires_at);
CREATE INDEX IF NOT EXISTS idx_harness_scorecards_group ON harness_scorecards(group_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_scorecards_item ON harness_scorecards(group_item_id, created_at DESC);
`

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(db *sql.DB) (*SQLiteStore, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}
	if _, err := db.Exec(baseSchemaSQL); err != nil {
		return nil, err
	}
	store := &SQLiteStore{db: db}
	if err := store.migrateSchema(context.Background()); err != nil {
		return nil, err
	}
	if _, err := db.Exec(indexSchemaSQL); err != nil {
		return nil, err
	}
	return store, nil
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
		id, root_run_id, parent_run_id, group_id, group_item_id, attempt_index, kind, status, runtime_state, user_id, conversation_id, session_id, agent_id,
		goal, model, result, error, depth, current_step, progress, workspace_root, artifact_root, sandbox_mode,
		approval_mode, max_duration_ns, max_steps, max_tool_rounds, max_subagents, max_depth, metadata_json,
		created_at, updated_at, started_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.RootRunID, run.ParentRunID, run.GroupID, run.GroupItemID, run.AttemptIndex, string(run.Kind), string(run.Status), string(run.RuntimeState), run.UserID,
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
		root_run_id=?, parent_run_id=?, group_id=?, group_item_id=?, attempt_index=?, kind=?, status=?, runtime_state=?, user_id=?, conversation_id=?, session_id=?, agent_id=?,
		goal=?, model=?, result=?, error=?, depth=?, current_step=?, progress=?, workspace_root=?, artifact_root=?, sandbox_mode=?,
		approval_mode=?, max_duration_ns=?, max_steps=?, max_tool_rounds=?, max_subagents=?, max_depth=?, metadata_json=?,
		updated_at=?, started_at=?, finished_at=?
		WHERE id=?`,
		run.RootRunID, run.ParentRunID, run.GroupID, run.GroupItemID, run.AttemptIndex, string(run.Kind), string(run.Status), string(run.RuntimeState), run.UserID,
		run.ConversationID, run.SessionID, run.AgentID, run.Goal, run.Model, run.Result, run.Error, run.Depth,
		run.CurrentStep, run.Progress, run.WorkspaceRoot, run.ArtifactRoot, run.SandboxMode, string(run.ApprovalMode),
		run.MaxDuration.Nanoseconds(), run.MaxSteps, run.MaxToolRounds, run.MaxSubagents, run.MaxDepth, marshalMetadata(run.Metadata),
		run.UpdatedAt, nullableTime(run.StartedAt), nullableTime(run.FinishedAt), run.ID,
	)
	return err
}

func (s *SQLiteStore) GetRun(ctx context.Context, id string) (*Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, root_run_id, parent_run_id, group_id, group_item_id, attempt_index, kind, status, runtime_state, user_id, conversation_id, session_id, agent_id,
		goal, model, result, error, depth, current_step, progress, workspace_root, artifact_root, sandbox_mode,
		approval_mode, max_duration_ns, max_steps, max_tool_rounds, max_subagents, max_depth, metadata_json,
		created_at, updated_at, started_at, finished_at
		FROM harness_runs WHERE id = ?`, id)
	return scanRun(row)
}

func (s *SQLiteStore) ListRuns(ctx context.Context, filter RunFilter) ([]Run, error) {
	query := `SELECT
		id, root_run_id, parent_run_id, group_id, group_item_id, attempt_index, kind, status, runtime_state, user_id, conversation_id, session_id, agent_id,
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
	if v := strings.TrimSpace(filter.GroupID); v != "" {
		clauses = append(clauses, "group_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.GroupItemID); v != "" {
		clauses = append(clauses, "group_item_id = ?")
		args = append(args, v)
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

func (s *SQLiteStore) CreateGroup(ctx context.Context, group *RunGroup) error {
	if group == nil {
		return fmt.Errorf("group is required")
	}
	now := timeutil.NowTime()
	if group.CreatedAt.IsZero() {
		group.CreatedAt = now
	}
	if group.UpdatedAt.IsZero() {
		group.UpdatedAt = group.CreatedAt
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO harness_run_groups (
		id, kind, title, status, owner_user_id, subject, scheduler_json, scoring_json, metadata_json, summary_json,
		created_at, updated_at, started_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		group.ID, string(group.Kind), group.Title, string(group.Status), group.OwnerUserID, group.Subject,
		marshalInterface(group.SchedulerConfig), marshalInterface(group.ScoringConfig), marshalMetadata(group.Metadata), marshalMetadata(group.Summary),
		group.CreatedAt, group.UpdatedAt, nullableTime(group.StartedAt), nullableTime(group.FinishedAt),
	)
	return err
}

func (s *SQLiteStore) UpdateGroup(ctx context.Context, group *RunGroup) error {
	if group == nil {
		return fmt.Errorf("group is required")
	}
	group.UpdatedAt = timeutil.NowTime()
	_, err := s.db.ExecContext(ctx, `UPDATE harness_run_groups SET
		kind=?, title=?, status=?, owner_user_id=?, subject=?, scheduler_json=?, scoring_json=?, metadata_json=?, summary_json=?,
		updated_at=?, started_at=?, finished_at=?
		WHERE id=?`,
		string(group.Kind), group.Title, string(group.Status), group.OwnerUserID, group.Subject,
		marshalInterface(group.SchedulerConfig), marshalInterface(group.ScoringConfig), marshalMetadata(group.Metadata), marshalMetadata(group.Summary),
		group.UpdatedAt, nullableTime(group.StartedAt), nullableTime(group.FinishedAt), group.ID,
	)
	return err
}

func (s *SQLiteStore) GetGroup(ctx context.Context, id string) (*RunGroup, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, kind, title, status, owner_user_id, subject, scheduler_json, scoring_json, metadata_json, summary_json,
		created_at, updated_at, started_at, finished_at
		FROM harness_run_groups WHERE id = ?`, id)
	return scanGroup(row)
}

func (s *SQLiteStore) ListGroups(ctx context.Context, filter RunGroupFilter) ([]RunGroup, error) {
	query := `SELECT
		id, kind, title, status, owner_user_id, subject, scheduler_json, scoring_json, metadata_json, summary_json,
		created_at, updated_at, started_at, finished_at
		FROM harness_run_groups`
	var (
		clauses []string
		args    []interface{}
	)
	if v := strings.TrimSpace(filter.OwnerUserID); v != "" {
		clauses = append(clauses, "owner_user_id = ?")
		args = append(args, v)
	}
	if len(filter.Kinds) > 0 {
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
		for _, status := range filter.Statuses {
			if status == "" {
				continue
			}
			parts = append(parts, "?")
			args = append(args, string(status))
		}
		if len(parts) > 0 {
			clauses = append(clauses, "status IN ("+strings.Join(parts, ",")+")")
		}
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
	var out []RunGroup
	for rows.Next() {
		group, scanErr := scanGroupRows(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *group)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) CreateGroupItems(ctx context.Context, items []RunGroupItem) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	now := timeutil.NowTime()
	for i := range items {
		item := items[i]
		if item.CreatedAt.IsZero() {
			item.CreatedAt = now
		}
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = item.CreatedAt
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO harness_run_group_items (
			id, group_id, item_index, run_kind, profile, input_json, expected_json, metadata_json, status, latest_run_id,
			attempt_count, max_attempts, lease_owner, lease_expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.GroupID, item.Index, string(item.RunKind), item.Profile, marshalMetadata(item.Input), marshalMetadata(item.Expected),
			marshalMetadata(item.Metadata), string(item.Status), item.LatestRunID, item.AttemptCount, item.MaxAttempts, item.LeaseOwner,
			nullableTime(item.LeaseExpiresAt), item.CreatedAt, item.UpdatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) UpdateGroupItem(ctx context.Context, item *RunGroupItem) error {
	if item == nil {
		return fmt.Errorf("group item is required")
	}
	item.UpdatedAt = timeutil.NowTime()
	_, err := s.db.ExecContext(ctx, `UPDATE harness_run_group_items SET
		group_id=?, item_index=?, run_kind=?, profile=?, input_json=?, expected_json=?, metadata_json=?, status=?, latest_run_id=?,
		attempt_count=?, max_attempts=?, lease_owner=?, lease_expires_at=?, updated_at=?
		WHERE id=?`,
		item.GroupID, item.Index, string(item.RunKind), item.Profile, marshalMetadata(item.Input), marshalMetadata(item.Expected),
		marshalMetadata(item.Metadata), string(item.Status), item.LatestRunID, item.AttemptCount, item.MaxAttempts, item.LeaseOwner,
		nullableTime(item.LeaseExpiresAt), item.UpdatedAt, item.ID,
	)
	return err
}

func (s *SQLiteStore) GetGroupItem(ctx context.Context, id string) (*RunGroupItem, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, group_id, item_index, run_kind, profile, input_json, expected_json, metadata_json, status, latest_run_id,
		attempt_count, max_attempts, lease_owner, lease_expires_at, created_at, updated_at
		FROM harness_run_group_items WHERE id = ?`, id)
	return scanGroupItem(row)
}

func (s *SQLiteStore) ListGroupItems(ctx context.Context, groupID string) ([]RunGroupItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, group_id, item_index, run_kind, profile, input_json, expected_json, metadata_json, status, latest_run_id,
		attempt_count, max_attempts, lease_owner, lease_expires_at, created_at, updated_at
		FROM harness_run_group_items WHERE group_id = ? ORDER BY item_index ASC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunGroupItem
	for rows.Next() {
		item, scanErr := scanGroupItemRows(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) CountGroupItemsByStatuses(ctx context.Context, groupID string, statuses []RunGroupItemStatus) (int, error) {
	if strings.TrimSpace(groupID) == "" || len(statuses) == 0 {
		return 0, nil
	}
	parts := make([]string, 0, len(statuses))
	args := make([]interface{}, 0, len(statuses)+1)
	args = append(args, groupID)
	for _, status := range statuses {
		if status == "" {
			continue
		}
		parts = append(parts, "?")
		args = append(args, string(status))
	}
	if len(parts) == 0 {
		return 0, nil
	}
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM harness_run_group_items WHERE group_id = ? AND status IN (`+strings.Join(parts, ",")+`)`, args...)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *SQLiteStore) ClaimNextGroupItem(ctx context.Context, groupID, workerID string, leaseTTL time.Duration, now time.Time) (*RunGroupItem, error) {
	if strings.TrimSpace(groupID) == "" {
		return nil, sql.ErrNoRows
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	row := tx.QueryRowContext(ctx, `SELECT
		id, group_id, item_index, run_kind, profile, input_json, expected_json, metadata_json, status, latest_run_id,
		attempt_count, max_attempts, lease_owner, lease_expires_at, created_at, updated_at
		FROM harness_run_group_items
		WHERE group_id = ?
			AND status = ?
			AND (lease_expires_at IS NULL OR lease_expires_at <= ?)
		ORDER BY item_index ASC
		LIMIT 1`, groupID, string(RunGroupItemStatusQueued), now)
	item, err := scanGroupItem(row)
	if err != nil {
		if errorsIsNoRows(err) {
			_ = tx.Rollback()
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	item.Status = RunGroupItemStatusRunning
	item.LeaseOwner = workerID
	leaseUntil := now.Add(leaseTTL)
	item.LeaseExpiresAt = &leaseUntil
	item.UpdatedAt = now
	if _, err = tx.ExecContext(ctx, `UPDATE harness_run_group_items SET
		status=?, lease_owner=?, lease_expires_at=?, updated_at=?
		WHERE id=?`, string(item.Status), item.LeaseOwner, nullableTime(item.LeaseExpiresAt), item.UpdatedAt, item.ID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *SQLiteStore) AttachScorecard(ctx context.Context, scorecard Scorecard) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO harness_scorecards (
		id, group_id, group_item_id, run_id, mode, verdict, score, breakdown_json, evidence_json, judge_trace_json, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		scorecard.ID, scorecard.GroupID, scorecard.GroupItemID, scorecard.RunID, string(scorecard.Mode), string(scorecard.Verdict),
		scorecard.Score, scorecard.BreakdownJSON, scorecard.EvidenceJSON, scorecard.JudgeTraceJSON, scorecard.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) ListScorecards(ctx context.Context, groupID string) ([]Scorecard, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, group_id, group_item_id, run_id, mode, verdict, score, breakdown_json, evidence_json, judge_trace_json, created_at
		FROM harness_scorecards WHERE group_id = ? ORDER BY created_at DESC, id DESC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Scorecard
	for rows.Next() {
		var scorecard Scorecard
		var mode, verdict string
		if err := rows.Scan(&scorecard.ID, &scorecard.GroupID, &scorecard.GroupItemID, &scorecard.RunID, &mode, &verdict, &scorecard.Score, &scorecard.BreakdownJSON, &scorecard.EvidenceJSON, &scorecard.JudgeTraceJSON, &scorecard.CreatedAt); err != nil {
			return nil, err
		}
		scorecard.Mode = ScoringMode(mode)
		scorecard.Verdict = ScoreVerdict(verdict)
		out = append(out, scorecard)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) LatestScorecardForItem(ctx context.Context, groupItemID string) (*Scorecard, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, group_id, group_item_id, run_id, mode, verdict, score, breakdown_json, evidence_json, judge_trace_json, created_at
		FROM harness_scorecards WHERE group_item_id = ? ORDER BY created_at DESC, id DESC LIMIT 1`, groupItemID)
	var scorecard Scorecard
	var mode, verdict string
	if err := row.Scan(&scorecard.ID, &scorecard.GroupID, &scorecard.GroupItemID, &scorecard.RunID, &mode, &verdict, &scorecard.Score, &scorecard.BreakdownJSON, &scorecard.EvidenceJSON, &scorecard.JudgeTraceJSON, &scorecard.CreatedAt); err != nil {
		return nil, err
	}
	scorecard.Mode = ScoringMode(mode)
	scorecard.Verdict = ScoreVerdict(verdict)
	return &scorecard, nil
}

func (s *SQLiteStore) migrateSchema(ctx context.Context) error {
	if err := s.ensureColumn(ctx, "harness_runs", "group_id", `ALTER TABLE harness_runs ADD COLUMN group_id TEXT DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "harness_runs", "group_item_id", `ALTER TABLE harness_runs ADD COLUMN group_item_id TEXT DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "harness_runs", "attempt_index", `ALTER TABLE harness_runs ADD COLUMN attempt_index INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) ensureColumn(ctx context.Context, table string, column string, ddl string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid        int
			name       string
			colType    string
			notNull    int
			defaultV   sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultV, &primaryKey); err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(column)) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, ddl)
	return err
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
		&run.ID, &run.RootRunID, &run.ParentRunID, &run.GroupID, &run.GroupItemID, &run.AttemptIndex, &kind, &status, &runtimeState, &run.UserID, &run.ConversationID, &run.SessionID, &run.AgentID,
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

func scanGroup(scanner rowScanner) (*RunGroup, error) {
	var (
		group                                    RunGroup
		kind, status                             string
		schedulerJSON, scoringJSON, metadataJSON string
		summaryJSON                              string
		startedAt, finishedAt                    sql.NullTime
	)
	if err := scanner.Scan(
		&group.ID, &kind, &group.Title, &status, &group.OwnerUserID, &group.Subject, &schedulerJSON, &scoringJSON, &metadataJSON, &summaryJSON,
		&group.CreatedAt, &group.UpdatedAt, &startedAt, &finishedAt,
	); err != nil {
		return nil, err
	}
	group.Kind = RunGroupKind(kind)
	group.Status = RunGroupStatus(status)
	_ = unmarshalInto(schedulerJSON, &group.SchedulerConfig)
	_ = unmarshalInto(scoringJSON, &group.ScoringConfig)
	group.Metadata = unmarshalMetadata(metadataJSON)
	group.Summary = unmarshalMetadata(summaryJSON)
	if startedAt.Valid {
		ts := startedAt.Time
		group.StartedAt = &ts
	}
	if finishedAt.Valid {
		ts := finishedAt.Time
		group.FinishedAt = &ts
	}
	return &group, nil
}

func scanGroupRows(rows *sql.Rows) (*RunGroup, error) {
	return scanGroup(rows)
}

func scanGroupItem(scanner rowScanner) (*RunGroupItem, error) {
	var (
		item                                  RunGroupItem
		runKind, status                       string
		inputJSON, expectedJSON, metadataJSON string
		leaseExpiresAt                        sql.NullTime
	)
	if err := scanner.Scan(
		&item.ID, &item.GroupID, &item.Index, &runKind, &item.Profile, &inputJSON, &expectedJSON, &metadataJSON, &status, &item.LatestRunID,
		&item.AttemptCount, &item.MaxAttempts, &item.LeaseOwner, &leaseExpiresAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.RunKind = RunKind(runKind)
	item.Status = RunGroupItemStatus(status)
	item.Input = unmarshalMetadata(inputJSON)
	item.Expected = unmarshalMetadata(expectedJSON)
	item.Metadata = unmarshalMetadata(metadataJSON)
	if leaseExpiresAt.Valid {
		ts := leaseExpiresAt.Time
		item.LeaseExpiresAt = &ts
	}
	return &item, nil
}

func scanGroupItemRows(rows *sql.Rows) (*RunGroupItem, error) {
	return scanGroupItem(rows)
}

func marshalInterface(value interface{}) string {
	if value == nil {
		return "{}"
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func unmarshalInto(raw string, dest interface{}) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), dest)
}

func errorsIsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
