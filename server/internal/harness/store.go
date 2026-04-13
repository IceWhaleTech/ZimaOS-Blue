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
	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
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
	provider_id TEXT DEFAULT '',
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

CREATE TABLE IF NOT EXISTS harness_datasets (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT DEFAULT '',
	owner_user_id TEXT DEFAULT '',
	subject TEXT DEFAULT '',
	default_run_kind TEXT DEFAULT '',
	default_profile TEXT DEFAULT '',
	active_version_id TEXT DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS harness_dataset_versions (
	id TEXT PRIMARY KEY,
	dataset_id TEXT NOT NULL,
	version TEXT NOT NULL,
	manifest_sha256 TEXT DEFAULT '',
	item_count INTEGER NOT NULL DEFAULT 0,
	source_type TEXT DEFAULT '',
	source_ref TEXT DEFAULT '',
	manifest_json TEXT NOT NULL DEFAULT '{}',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_by TEXT DEFAULT '',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS harness_eval_specs (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	owner_user_id TEXT DEFAULT '',
	subject TEXT DEFAULT '',
	run_kind TEXT NOT NULL,
	profile TEXT DEFAULT '',
	dataset_id TEXT DEFAULT '',
	dataset_version_id TEXT DEFAULT '',
	scheduler_json TEXT NOT NULL DEFAULT '{}',
	scoring_json TEXT NOT NULL DEFAULT '{}',
	runtime_policy_json TEXT NOT NULL DEFAULT '{}',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS harness_eval_runs (
	id TEXT PRIMARY KEY,
	eval_spec_id TEXT NOT NULL,
	group_id TEXT NOT NULL,
	dataset_version_id TEXT DEFAULT '',
	baseline_eval_run_id TEXT DEFAULT '',
	title TEXT DEFAULT '',
	owner_user_id TEXT DEFAULT '',
	status TEXT NOT NULL,
	trigger_kind TEXT DEFAULT '',
	trigger_ref TEXT DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	summary_json TEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	started_at DATETIME,
	finished_at DATETIME
);

CREATE TABLE IF NOT EXISTS harness_baselines (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	subject TEXT DEFAULT '',
	owner_user_id TEXT DEFAULT '',
	eval_spec_id TEXT NOT NULL,
	eval_run_id TEXT NOT NULL,
	is_default INTEGER NOT NULL DEFAULT 0,
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS harness_comparison_reports (
	id TEXT PRIMARY KEY,
	owner_user_id TEXT DEFAULT '',
	baseline_id TEXT DEFAULT '',
	eval_spec_id TEXT NOT NULL,
	base_eval_run_id TEXT NOT NULL,
	target_eval_run_id TEXT NOT NULL,
	summary_json TEXT NOT NULL DEFAULT '{}',
	regressions_json TEXT NOT NULL DEFAULT '[]',
	improvements_json TEXT NOT NULL DEFAULT '[]',
	scorer_delta_json TEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS harness_skill_revisions (
	id TEXT PRIMARY KEY,
	skill_id TEXT NOT NULL,
	status TEXT NOT NULL,
	source_path TEXT DEFAULT '',
	candidate_id TEXT DEFAULT '',
	base_content_sha256 TEXT DEFAULT '',
	origin_case_id TEXT DEFAULT '',
	parent_revision_id TEXT DEFAULT '',
	backup_of_revision_id TEXT DEFAULT '',
	eval_run_id TEXT DEFAULT '',
	optimization_run_id TEXT DEFAULT '',
	followup_gate TEXT DEFAULT '',
	optimization_surface TEXT DEFAULT '',
	decision_action TEXT DEFAULT '',
	review_note TEXT DEFAULT '',
	reviewed_by TEXT DEFAULT '',
	decision_log_json TEXT DEFAULT '',
	content TEXT NOT NULL DEFAULT '',
	content_sha256 TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL,
	reviewed_at DATETIME,
	promoted_at DATETIME
);

CREATE TABLE IF NOT EXISTS harness_skill_evolution_cases (
	id TEXT PRIMARY KEY,
	skill_id TEXT NOT NULL,
	owner_user_id TEXT DEFAULT '',
	mode TEXT NOT NULL,
	reason TEXT NOT NULL,
	source_kind TEXT DEFAULT '',
	source_id TEXT DEFAULT '',
	candidate_id TEXT DEFAULT '',
	base_content_sha256 TEXT DEFAULT '',
	failure_signature TEXT DEFAULT '',
	dedup_key TEXT DEFAULT '',
	summary TEXT DEFAULT '',
	evidence_json TEXT NOT NULL DEFAULT '{}',
	revision_id TEXT DEFAULT '',
	status TEXT NOT NULL,
	skipped_reason TEXT DEFAULT '',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
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
CREATE INDEX IF NOT EXISTS idx_harness_datasets_owner ON harness_datasets(owner_user_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_harness_dataset_versions_dataset_version ON harness_dataset_versions(dataset_id, version);
CREATE INDEX IF NOT EXISTS idx_harness_dataset_versions_dataset ON harness_dataset_versions(dataset_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_eval_specs_owner ON harness_eval_specs(owner_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_eval_specs_dataset ON harness_eval_specs(dataset_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_eval_runs_spec ON harness_eval_runs(eval_spec_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_harness_eval_runs_group ON harness_eval_runs(group_id);
CREATE INDEX IF NOT EXISTS idx_harness_eval_runs_owner_status ON harness_eval_runs(owner_user_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_baselines_owner ON harness_baselines(owner_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_baselines_spec ON harness_baselines(eval_spec_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_harness_baselines_default_spec ON harness_baselines(eval_spec_id) WHERE is_default = 1;
CREATE INDEX IF NOT EXISTS idx_harness_comparison_reports_target ON harness_comparison_reports(target_eval_run_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_comparison_reports_spec ON harness_comparison_reports(eval_spec_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_skill_revisions_skill ON harness_skill_revisions(skill_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_skill_revisions_status ON harness_skill_revisions(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_skill_revisions_eval_run ON harness_skill_revisions(eval_run_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_skill_revisions_optimization_run ON harness_skill_revisions(optimization_run_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_skill_evolution_cases_skill ON harness_skill_evolution_cases(skill_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_skill_evolution_cases_owner ON harness_skill_evolution_cases(owner_user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_skill_evolution_cases_status ON harness_skill_evolution_cases(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_skill_evolution_cases_revision ON harness_skill_evolution_cases(revision_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_skill_evolution_cases_candidate ON harness_skill_evolution_cases(candidate_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_harness_skill_evolution_cases_dedup ON harness_skill_evolution_cases(skill_id, dedup_key, updated_at DESC);
`

type SQLiteStore struct {
	db     *sql.DB
	readDB *sql.DB
}

const (
	harnessSQLiteBusyTimeoutMS      = 5000
	harnessSQLiteBusyRetryAttempts  = 4
	harnessSQLiteBusyRetryBaseDelay = 25 * time.Millisecond
)

func NewSQLiteStore(db *sql.DB) (*SQLiteStore, error) {
	return NewSQLiteStoreWithReadDB(db, db)
}

func NewSQLiteStoreWithReadDB(writeDB, readDB *sql.DB) (*SQLiteStore, error) {
	if writeDB == nil {
		return nil, fmt.Errorf("db is required")
	}
	if readDB == nil {
		readDB = writeDB
	}
	configureSQLiteConnectionPool(writeDB)
	store := &SQLiteStore{db: writeDB, readDB: readDB}
	if err := store.configureSQLite(context.Background()); err != nil {
		return nil, err
	}
	if _, err := store.execContext(context.Background(), baseSchemaSQL); err != nil {
		return nil, err
	}
	if err := store.migrateSchema(context.Background()); err != nil {
		return nil, err
	}
	if _, err := store.execContext(context.Background(), indexSchemaSQL); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *SQLiteStore) hasSeparateReader() bool {
	return s != nil && s.db != nil && s.reader() != nil && s.reader() != s.db
}

func (s *SQLiteStore) shouldFallbackToWriter(err error, empty bool) bool {
	if !s.hasSeparateReader() {
		return false
	}
	if err == nil {
		return empty
	}
	return errorsIsNoRows(err) || dbutil.IsSQLiteCorruptionError(err)
}

func mergeUniqueRowsByKey[T any](primary []T, secondary []T, limit int, keyFn func(T) string) []T {
	if len(secondary) == 0 {
		return primary
	}
	out := make([]T, 0, len(primary)+len(secondary))
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	appendRows := func(rows []T) {
		for _, row := range rows {
			if limit > 0 && len(out) >= limit {
				return
			}
			key := strings.TrimSpace(keyFn(row))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, row)
		}
	}
	appendRows(primary)
	appendRows(secondary)
	return out
}

type harnessRunRow struct {
	ID             string       `zorm:"id"`
	RootRunID      string       `zorm:"root_run_id"`
	ParentRunID    string       `zorm:"parent_run_id"`
	GroupID        string       `zorm:"group_id"`
	GroupItemID    string       `zorm:"group_item_id"`
	AttemptIndex   int          `zorm:"attempt_index"`
	Kind           string       `zorm:"kind"`
	Status         string       `zorm:"status"`
	RuntimeState   string       `zorm:"runtime_state"`
	UserID         string       `zorm:"user_id"`
	ConversationID string       `zorm:"conversation_id"`
	SessionID      string       `zorm:"session_id"`
	AgentID        string       `zorm:"agent_id"`
	Goal           string       `zorm:"goal"`
	ProviderID     string       `zorm:"provider_id"`
	Model          string       `zorm:"model"`
	Result         string       `zorm:"result"`
	Error          string       `zorm:"error"`
	Depth          int          `zorm:"depth"`
	CurrentStep    int          `zorm:"current_step"`
	Progress       int          `zorm:"progress"`
	WorkspaceRoot  string       `zorm:"workspace_root"`
	ArtifactRoot   string       `zorm:"artifact_root"`
	SandboxMode    string       `zorm:"sandbox_mode"`
	ApprovalMode   string       `zorm:"approval_mode"`
	MaxDurationNS  int64        `zorm:"max_duration_ns"`
	MaxSteps       int          `zorm:"max_steps"`
	MaxToolRounds  int          `zorm:"max_tool_rounds"`
	MaxSubagents   int          `zorm:"max_subagents"`
	MaxDepth       int          `zorm:"max_depth"`
	MetadataJSON   string       `zorm:"metadata_json"`
	CreatedAt      time.Time    `zorm:"created_at"`
	UpdatedAt      time.Time    `zorm:"updated_at"`
	StartedAt      sql.NullTime `zorm:"started_at"`
	FinishedAt     sql.NullTime `zorm:"finished_at"`
}

type harnessEventRow struct {
	ID             string    `zorm:"id"`
	RunID          string    `zorm:"run_id"`
	RootRunID      string    `zorm:"root_run_id"`
	ParentRunID    string    `zorm:"parent_run_id"`
	Type           string    `zorm:"type"`
	StepIndex      int       `zorm:"step_index"`
	ToolName       string    `zorm:"tool_name"`
	CapabilityKind string    `zorm:"capability_kind"`
	Message        string    `zorm:"message"`
	PayloadJSON    string    `zorm:"payload_json"`
	CreatedAt      time.Time `zorm:"created_at"`
}

type harnessArtifactRow struct {
	ID           string `zorm:"id"`
	RunID        string `zorm:"run_id"`
	Kind         string `zorm:"kind"`
	Label        string `zorm:"label"`
	PathOrURL    string `zorm:"path_or_url"`
	MIMEType     string `zorm:"mime_type"`
	SizeBytes    int64  `zorm:"size_bytes"`
	MetadataJSON string `zorm:"metadata_json"`
}

type harnessGroupRow struct {
	ID            string       `zorm:"id"`
	Kind          string       `zorm:"kind"`
	Title         string       `zorm:"title"`
	Status        string       `zorm:"status"`
	OwnerUserID   string       `zorm:"owner_user_id"`
	Subject       string       `zorm:"subject"`
	SchedulerJSON string       `zorm:"scheduler_json"`
	ScoringJSON   string       `zorm:"scoring_json"`
	MetadataJSON  string       `zorm:"metadata_json"`
	SummaryJSON   string       `zorm:"summary_json"`
	CreatedAt     time.Time    `zorm:"created_at"`
	UpdatedAt     time.Time    `zorm:"updated_at"`
	StartedAt     sql.NullTime `zorm:"started_at"`
	FinishedAt    sql.NullTime `zorm:"finished_at"`
}

type harnessGroupItemRow struct {
	ID             string       `zorm:"id"`
	GroupID        string       `zorm:"group_id"`
	Index          int          `zorm:"item_index"`
	RunKind        string       `zorm:"run_kind"`
	Profile        string       `zorm:"profile"`
	InputJSON      string       `zorm:"input_json"`
	ExpectedJSON   string       `zorm:"expected_json"`
	MetadataJSON   string       `zorm:"metadata_json"`
	Status         string       `zorm:"status"`
	LatestRunID    string       `zorm:"latest_run_id"`
	AttemptCount   int          `zorm:"attempt_count"`
	MaxAttempts    int          `zorm:"max_attempts"`
	LeaseOwner     string       `zorm:"lease_owner"`
	LeaseExpiresAt sql.NullTime `zorm:"lease_expires_at"`
	CreatedAt      time.Time    `zorm:"created_at"`
	UpdatedAt      time.Time    `zorm:"updated_at"`
}

type harnessScorecardRow struct {
	ID             string    `zorm:"id"`
	GroupID        string    `zorm:"group_id"`
	GroupItemID    string    `zorm:"group_item_id"`
	RunID          string    `zorm:"run_id"`
	Mode           string    `zorm:"mode"`
	Verdict        string    `zorm:"verdict"`
	Score          float64   `zorm:"score"`
	BreakdownJSON  string    `zorm:"breakdown_json"`
	EvidenceJSON   string    `zorm:"evidence_json"`
	JudgeTraceJSON string    `zorm:"judge_trace_json"`
	CreatedAt      time.Time `zorm:"created_at"`
}

func harnessRunFromRow(row harnessRunRow) Run {
	run := Run{
		ID:             row.ID,
		RootRunID:      row.RootRunID,
		ParentRunID:    row.ParentRunID,
		GroupID:        row.GroupID,
		GroupItemID:    row.GroupItemID,
		AttemptIndex:   row.AttemptIndex,
		Kind:           RunKind(row.Kind),
		Status:         RunStatus(row.Status),
		RuntimeState:   agentpkg.RuntimeState(row.RuntimeState),
		UserID:         row.UserID,
		ConversationID: row.ConversationID,
		SessionID:      row.SessionID,
		AgentID:        row.AgentID,
		Goal:           row.Goal,
		ProviderID:     row.ProviderID,
		Model:          row.Model,
		Result:         row.Result,
		Error:          row.Error,
		Depth:          row.Depth,
		CurrentStep:    row.CurrentStep,
		Progress:       row.Progress,
		WorkspaceRoot:  row.WorkspaceRoot,
		ArtifactRoot:   row.ArtifactRoot,
		SandboxMode:    row.SandboxMode,
		ApprovalMode:   ApprovalMode(row.ApprovalMode),
		MaxDuration:    time.Duration(row.MaxDurationNS),
		MaxSteps:       row.MaxSteps,
		MaxToolRounds:  row.MaxToolRounds,
		MaxSubagents:   row.MaxSubagents,
		MaxDepth:       row.MaxDepth,
		Metadata:       unmarshalMetadata(row.MetadataJSON),
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	if row.StartedAt.Valid {
		ts := row.StartedAt.Time
		run.StartedAt = &ts
	}
	if row.FinishedAt.Valid {
		ts := row.FinishedAt.Time
		run.FinishedAt = &ts
	}
	return run
}

func harnessRunsFromRows(rows []harnessRunRow) []Run {
	out := make([]Run, 0, len(rows))
	for i := range rows {
		out = append(out, harnessRunFromRow(rows[i]))
	}
	return out
}

func harnessEventFromRow(row harnessEventRow) RunEvent {
	return RunEvent{
		ID:             row.ID,
		RunID:          row.RunID,
		RootRunID:      row.RootRunID,
		ParentRunID:    row.ParentRunID,
		Type:           row.Type,
		StepIndex:      row.StepIndex,
		ToolName:       row.ToolName,
		CapabilityKind: row.CapabilityKind,
		Message:        row.Message,
		PayloadJSON:    row.PayloadJSON,
		CreatedAt:      row.CreatedAt,
	}
}

func harnessEventsFromRows(rows []harnessEventRow) []RunEvent {
	out := make([]RunEvent, 0, len(rows))
	for i := range rows {
		out = append(out, harnessEventFromRow(rows[i]))
	}
	return out
}

func harnessArtifactFromRow(row harnessArtifactRow) ArtifactRef {
	return ArtifactRef{
		ID:           row.ID,
		RunID:        row.RunID,
		Kind:         row.Kind,
		Label:        row.Label,
		PathOrURL:    row.PathOrURL,
		MIMEType:     row.MIMEType,
		SizeBytes:    row.SizeBytes,
		MetadataJSON: row.MetadataJSON,
	}
}

func harnessArtifactsFromRows(rows []harnessArtifactRow) []ArtifactRef {
	out := make([]ArtifactRef, 0, len(rows))
	for i := range rows {
		out = append(out, harnessArtifactFromRow(rows[i]))
	}
	return out
}

func harnessGroupFromRow(row harnessGroupRow) RunGroup {
	group := RunGroup{
		ID:          row.ID,
		Kind:        RunGroupKind(row.Kind),
		Title:       row.Title,
		Status:      RunGroupStatus(row.Status),
		OwnerUserID: row.OwnerUserID,
		Subject:     row.Subject,
		Metadata:    unmarshalMetadata(row.MetadataJSON),
		Summary:     unmarshalMetadata(row.SummaryJSON),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
	_ = unmarshalInto(row.SchedulerJSON, &group.SchedulerConfig)
	_ = unmarshalInto(row.ScoringJSON, &group.ScoringConfig)
	if row.StartedAt.Valid {
		ts := row.StartedAt.Time
		group.StartedAt = &ts
	}
	if row.FinishedAt.Valid {
		ts := row.FinishedAt.Time
		group.FinishedAt = &ts
	}
	return group
}

func harnessGroupsFromRows(rows []harnessGroupRow) []RunGroup {
	out := make([]RunGroup, 0, len(rows))
	for i := range rows {
		out = append(out, harnessGroupFromRow(rows[i]))
	}
	return out
}

func harnessGroupItemFromRow(row harnessGroupItemRow) RunGroupItem {
	item := RunGroupItem{
		ID:           row.ID,
		GroupID:      row.GroupID,
		Index:        row.Index,
		RunKind:      RunKind(row.RunKind),
		Profile:      row.Profile,
		Input:        unmarshalMetadata(row.InputJSON),
		Expected:     unmarshalMetadata(row.ExpectedJSON),
		Metadata:     unmarshalMetadata(row.MetadataJSON),
		Status:       RunGroupItemStatus(row.Status),
		LatestRunID:  row.LatestRunID,
		AttemptCount: row.AttemptCount,
		MaxAttempts:  row.MaxAttempts,
		LeaseOwner:   row.LeaseOwner,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
	if row.LeaseExpiresAt.Valid {
		ts := row.LeaseExpiresAt.Time
		item.LeaseExpiresAt = &ts
	}
	return item
}

func harnessGroupItemsFromRows(rows []harnessGroupItemRow) []RunGroupItem {
	out := make([]RunGroupItem, 0, len(rows))
	for i := range rows {
		out = append(out, harnessGroupItemFromRow(rows[i]))
	}
	return out
}

func harnessScorecardFromRow(row harnessScorecardRow) Scorecard {
	return Scorecard{
		ID:             row.ID,
		GroupID:        row.GroupID,
		GroupItemID:    row.GroupItemID,
		RunID:          row.RunID,
		Mode:           ScoringMode(row.Mode),
		Verdict:        ScoreVerdict(row.Verdict),
		Score:          row.Score,
		BreakdownJSON:  row.BreakdownJSON,
		EvidenceJSON:   row.EvidenceJSON,
		JudgeTraceJSON: row.JudgeTraceJSON,
		CreatedAt:      row.CreatedAt,
	}
}

func harnessScorecardsFromRows(rows []harnessScorecardRow) []Scorecard {
	out := make([]Scorecard, 0, len(rows))
	for i := range rows {
		out = append(out, harnessScorecardFromRow(rows[i]))
	}
	return out
}

func configureSQLiteConnectionPool(db *sql.DB) {
	if db == nil {
		return
	}
	// Harness relies on connection-scoped pragmas such as busy_timeout and
	// foreign_keys. Keep a single shared SQLite connection so later pooled
	// operations cannot bypass the configured runtime boundary.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
}

func (s *SQLiteStore) configureSQLite(ctx context.Context) error {
	pragmas := []string{
		fmt.Sprintf("PRAGMA busy_timeout=%d", harnessSQLiteBusyTimeoutMS),
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=FULL",
		"PRAGMA wal_autocheckpoint=1000",
	}
	for _, pragma := range pragmas {
		if _, err := s.execContext(ctx, pragma); err != nil {
			return fmt.Errorf("exec %q: %w", pragma, err)
		}
	}
	return nil
}

func (s *SQLiteStore) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	var (
		result sql.Result
		err    error
	)
	runErr := withSQLiteBusyRetry(ctx, func() error {
		result, err = s.db.ExecContext(ctx, query, args...)
		return err
	})
	if runErr != nil {
		return nil, runErr
	}
	return result, nil
}

func (s *SQLiteStore) beginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	var (
		tx  *sql.Tx
		err error
	)
	runErr := withSQLiteBusyRetry(ctx, func() error {
		tx, err = s.db.BeginTx(ctx, opts)
		return err
	})
	if runErr != nil {
		return nil, runErr
	}
	return tx, nil
}

func (s *SQLiteStore) withTx(ctx context.Context, fn func(*sql.Tx) error) (err error) {
	if s == nil {
		return fmt.Errorf("harness store is not configured")
	}
	tx, err := s.beginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	err = tx.Commit()
	return err
}

func txExecContextWithBusyRetry(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) (sql.Result, error) {
	var (
		result sql.Result
		err    error
	)
	runErr := withSQLiteBusyRetry(ctx, func() error {
		result, err = tx.ExecContext(ctx, query, args...)
		return err
	})
	if runErr != nil {
		return nil, runErr
	}
	return result, nil
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
	_, err := s.execContext(ctx, `INSERT INTO harness_runs (
		id, root_run_id, parent_run_id, group_id, group_item_id, attempt_index, kind, status, runtime_state, user_id, conversation_id, session_id, agent_id,
		goal, provider_id, model, result, error, depth, current_step, progress, workspace_root, artifact_root, sandbox_mode,
		approval_mode, max_duration_ns, max_steps, max_tool_rounds, max_subagents, max_depth, metadata_json,
		created_at, updated_at, started_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.RootRunID, run.ParentRunID, run.GroupID, run.GroupItemID, run.AttemptIndex, string(run.Kind), string(run.Status), string(run.RuntimeState), run.UserID,
		run.ConversationID, run.SessionID, run.AgentID, run.Goal, run.ProviderID, run.Model, run.Result, run.Error, run.Depth,
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
	_, err := s.execContext(ctx, `UPDATE harness_runs SET
		root_run_id=?, parent_run_id=?, group_id=?, group_item_id=?, attempt_index=?, kind=?, status=?, runtime_state=?, user_id=?, conversation_id=?, session_id=?, agent_id=?,
		goal=?, provider_id=?, model=?, result=?, error=?, depth=?, current_step=?, progress=?, workspace_root=?, artifact_root=?, sandbox_mode=?,
		approval_mode=?, max_duration_ns=?, max_steps=?, max_tool_rounds=?, max_subagents=?, max_depth=?, metadata_json=?,
		updated_at=?, started_at=?, finished_at=?
		WHERE id=?`,
		run.RootRunID, run.ParentRunID, run.GroupID, run.GroupItemID, run.AttemptIndex, string(run.Kind), string(run.Status), string(run.RuntimeState), run.UserID,
		run.ConversationID, run.SessionID, run.AgentID, run.Goal, run.ProviderID, run.Model, run.Result, run.Error, run.Depth,
		run.CurrentStep, run.Progress, run.WorkspaceRoot, run.ArtifactRoot, run.SandboxMode, string(run.ApprovalMode),
		run.MaxDuration.Nanoseconds(), run.MaxSteps, run.MaxToolRounds, run.MaxSubagents, run.MaxDepth, marshalMetadata(run.Metadata),
		run.UpdatedAt, nullableTime(run.StartedAt), nullableTime(run.FinishedAt), run.ID,
	)
	return err
}

func (s *SQLiteStore) UpdateRunIfMaterialStateMatches(ctx context.Context, expected *Run, next *Run) (bool, error) {
	if expected == nil {
		return false, fmt.Errorf("expected run is required")
	}
	if next == nil {
		return false, fmt.Errorf("next run is required")
	}
	next.UpdatedAt = timeutil.NowTime()
	result, err := s.execContext(ctx, `UPDATE harness_runs SET
		root_run_id=?, parent_run_id=?, group_id=?, group_item_id=?, attempt_index=?, kind=?, status=?, runtime_state=?, user_id=?, conversation_id=?, session_id=?, agent_id=?,
		goal=?, provider_id=?, model=?, result=?, error=?, depth=?, current_step=?, progress=?, workspace_root=?, artifact_root=?, sandbox_mode=?,
		approval_mode=?, max_duration_ns=?, max_steps=?, max_tool_rounds=?, max_subagents=?, max_depth=?, metadata_json=?,
		updated_at=?, started_at=?, finished_at=?
		WHERE id=? AND status=? AND runtime_state=? AND current_step=? AND progress=? AND result=? AND error=? AND metadata_json=?`,
		next.RootRunID, next.ParentRunID, next.GroupID, next.GroupItemID, next.AttemptIndex, string(next.Kind), string(next.Status), string(next.RuntimeState), next.UserID,
		next.ConversationID, next.SessionID, next.AgentID, next.Goal, next.ProviderID, next.Model, next.Result, next.Error, next.Depth,
		next.CurrentStep, next.Progress, next.WorkspaceRoot, next.ArtifactRoot, next.SandboxMode, string(next.ApprovalMode),
		next.MaxDuration.Nanoseconds(), next.MaxSteps, next.MaxToolRounds, next.MaxSubagents, next.MaxDepth, marshalMetadata(next.Metadata),
		next.UpdatedAt, nullableTime(next.StartedAt), nullableTime(next.FinishedAt),
		next.ID, string(expected.Status), string(expected.RuntimeState), expected.CurrentStep, expected.Progress, expected.Result, expected.Error, marshalMetadata(expected.Metadata),
	)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (s *SQLiteStore) GetRun(ctx context.Context, id string) (*Run, error) {
	rows, err := s.selectRunRows(ctx, s.reader(), id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 && s.reader() != s.db {
		rows, err = s.selectRunRows(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	run := harnessRunFromRow(rows[0])
	return &run, nil
}

func (s *SQLiteStore) selectRunRows(ctx context.Context, db *sql.DB, id string) ([]harnessRunRow, error) {
	var rows []harnessRunRow
	if _, err := z.TableContext(ctx, db, "harness_runs").Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SQLiteStore) ListRuns(ctx context.Context, filter RunFilter) ([]Run, error) {
	rows, err := s.listRunRows(ctx, s.reader(), filter)
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.listRunRows(ctx, s.db, filter)
		if err != nil {
			return nil, err
		}
	} else if s.hasSeparateReader() {
		writerRows, writerErr := s.listRunRows(ctx, s.db, filter)
		if writerErr == nil {
			rows = mergeUniqueRowsByKey(rows, writerRows, filter.Limit, func(row harnessRunRow) string { return row.ID })
		}
	}
	return harnessRunsFromRows(rows), nil
}

func (s *SQLiteStore) listRunRows(ctx context.Context, db *sql.DB, filter RunFilter) ([]harnessRunRow, error) {
	var conds []interface{}
	if v := strings.TrimSpace(filter.UserID); v != "" {
		conds = append(conds, z.Eq("user_id", v))
	}
	if filter.Kind != "" {
		conds = append(conds, z.Eq("kind", string(filter.Kind)))
	} else if len(filter.Kinds) > 0 {
		kinds := make([]interface{}, 0, len(filter.Kinds))
		for _, kind := range filter.Kinds {
			if kind == "" {
				continue
			}
			kinds = append(kinds, string(kind))
		}
		if len(kinds) > 0 {
			conds = append(conds, z.In("kind", kinds...))
		}
	}
	if len(filter.Statuses) > 0 {
		statuses := make([]interface{}, 0, len(filter.Statuses))
		for _, st := range filter.Statuses {
			if st == "" {
				continue
			}
			statuses = append(statuses, string(st))
		}
		if len(statuses) > 0 {
			conds = append(conds, z.In("status", statuses...))
		}
	}
	if v := strings.TrimSpace(filter.ConversationID); v != "" {
		conds = append(conds, z.Eq("conversation_id", v))
	}
	if v := strings.TrimSpace(filter.GroupID); v != "" {
		conds = append(conds, z.Eq("group_id", v))
	}
	if v := strings.TrimSpace(filter.GroupItemID); v != "" {
		conds = append(conds, z.Eq("group_item_id", v))
	}
	if v := strings.TrimSpace(filter.ParentRunID); v != "" {
		conds = append(conds, z.Eq("parent_run_id", v))
	}
	if v := strings.TrimSpace(filter.RootRunID); v != "" {
		conds = append(conds, z.Eq("root_run_id", v))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	opts := []z.ZormItem{
		z.OrderBy("created_at DESC"),
		z.Limit(limit),
	}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	var rows []harnessRunRow
	if _, err := z.TableContext(ctx, db, "harness_runs").Select(&rows, opts...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SQLiteStore) FindRunByMetadata(ctx context.Context, kind RunKind, key, value string) (*Run, error) {
	if s == nil || s.reader() == nil {
		return nil, fmt.Errorf("harness store is not configured")
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return nil, nil
	}
	query := `SELECT
		id, root_run_id, parent_run_id, group_id, group_item_id, attempt_index, kind, status, runtime_state, user_id, conversation_id, session_id, agent_id,
		goal, provider_id, model, result, error, depth, current_step, progress, workspace_root, artifact_root, sandbox_mode,
		approval_mode, max_duration_ns, max_steps, max_tool_rounds, max_subagents, max_depth, metadata_json,
		created_at, updated_at, started_at, finished_at
	FROM harness_runs
	WHERE kind = ? AND json_extract(metadata_json, ?) = ?
	ORDER BY created_at DESC
	LIMIT 1`
	rows, err := s.reader().QueryContext(ctx, query, string(kind), "$."+key, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return scanRunRows(rows)
}

func (s *SQLiteStore) AppendEvent(ctx context.Context, event RunEvent) error {
	_, err := s.execContext(ctx, `INSERT INTO harness_run_events (
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
	var rows []harnessEventRow
	if _, err := z.TableContext(ctx, s.reader(), "harness_run_events").Select(&rows,
		z.Where(z.Eq("run_id", runID)),
		z.OrderBy("created_at ASC", "id ASC"),
		z.Limit(limit),
	); err != nil {
		return nil, err
	}
	return harnessEventsFromRows(rows), nil
}

func (s *SQLiteStore) AttachArtifact(ctx context.Context, ref ArtifactRef) error {
	_, err := s.execContext(ctx, `INSERT OR REPLACE INTO harness_artifacts (
		id, run_id, kind, label, path_or_url, mime_type, size_bytes, metadata_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ref.ID, ref.RunID, ref.Kind, ref.Label, ref.PathOrURL, ref.MIMEType, ref.SizeBytes, ref.MetadataJSON,
	)
	return err
}

func (s *SQLiteStore) ListArtifacts(ctx context.Context, runID string) ([]ArtifactRef, error) {
	var rows []harnessArtifactRow
	if _, err := z.TableContext(ctx, s.reader(), "harness_artifacts").Select(&rows,
		z.Where(z.Eq("run_id", runID)),
		z.OrderBy("id ASC"),
	); err != nil {
		return nil, err
	}
	return harnessArtifactsFromRows(rows), nil
}

func (s *SQLiteStore) DeleteRun(ctx context.Context, id string) error {
	tx, err := s.beginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = txExecContextWithBusyRetry(ctx, tx, `DELETE FROM harness_run_events WHERE run_id = ?`, id); err != nil {
		return err
	}
	if _, err = txExecContextWithBusyRetry(ctx, tx, `DELETE FROM harness_artifacts WHERE run_id = ?`, id); err != nil {
		return err
	}
	if _, err = txExecContextWithBusyRetry(ctx, tx, `DELETE FROM harness_runs WHERE id = ?`, id); err != nil {
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
	_, err := s.execContext(ctx, `INSERT INTO harness_run_groups (
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
	_, err := s.execContext(ctx, `UPDATE harness_run_groups SET
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
	rows, err := s.selectGroupRows(ctx, s.reader(), id)
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.selectGroupRows(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
	} else if len(rows) == 0 && s.hasSeparateReader() {
		rows, err = s.selectGroupRows(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	group := harnessGroupFromRow(rows[0])
	return &group, nil
}

func (s *SQLiteStore) ListGroups(ctx context.Context, filter RunGroupFilter) ([]RunGroup, error) {
	rows, err := s.listGroupRows(ctx, s.reader(), filter)
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.listGroupRows(ctx, s.db, filter)
		if err != nil {
			return nil, err
		}
	} else if s.hasSeparateReader() {
		writerRows, writerErr := s.listGroupRows(ctx, s.db, filter)
		if writerErr == nil {
			rows = mergeUniqueRowsByKey(rows, writerRows, filter.Limit, func(row harnessGroupRow) string { return row.ID })
		}
	}
	return harnessGroupsFromRows(rows), nil
}

func (s *SQLiteStore) listGroupRows(ctx context.Context, db *sql.DB, filter RunGroupFilter) ([]harnessGroupRow, error) {
	var conds []interface{}
	if v := strings.TrimSpace(filter.OwnerUserID); v != "" {
		conds = append(conds, z.Eq("owner_user_id", v))
	}
	if len(filter.Kinds) > 0 {
		kinds := make([]interface{}, 0, len(filter.Kinds))
		for _, kind := range filter.Kinds {
			if kind == "" {
				continue
			}
			kinds = append(kinds, string(kind))
		}
		if len(kinds) > 0 {
			conds = append(conds, z.In("kind", kinds...))
		}
	}
	if len(filter.Statuses) > 0 {
		statuses := make([]interface{}, 0, len(filter.Statuses))
		for _, status := range filter.Statuses {
			if status == "" {
				continue
			}
			statuses = append(statuses, string(status))
		}
		if len(statuses) > 0 {
			conds = append(conds, z.In("status", statuses...))
		}
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	opts := []z.ZormItem{
		z.OrderBy("created_at DESC"),
		z.Limit(limit),
	}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	var rows []harnessGroupRow
	if _, err := z.TableContext(ctx, db, "harness_run_groups").Select(&rows, opts...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SQLiteStore) selectGroupRows(ctx context.Context, db *sql.DB, id string) ([]harnessGroupRow, error) {
	var rows []harnessGroupRow
	if _, err := z.TableContext(ctx, db, "harness_run_groups").Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SQLiteStore) CreateGroupItems(ctx context.Context, items []RunGroupItem) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.beginTx(ctx, nil)
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
		if _, err = txExecContextWithBusyRetry(ctx, tx, `INSERT INTO harness_run_group_items (
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
	_, err := s.execContext(ctx, `UPDATE harness_run_group_items SET
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
	rows, err := s.reader().QueryContext(ctx, `SELECT
		id, group_id, item_index, run_kind, profile, input_json, expected_json, metadata_json, status, latest_run_id,
		attempt_count, max_attempts, lease_owner, lease_expires_at, created_at, updated_at
	FROM harness_run_group_items
	WHERE id = ?
	LIMIT 1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	return scanGroupItemRows(rows)
}

func (s *SQLiteStore) ListGroupItems(ctx context.Context, groupID string) ([]RunGroupItem, error) {
	rows, err := s.reader().QueryContext(ctx, `SELECT
		id, group_id, item_index, run_kind, profile, input_json, expected_json, metadata_json, status, latest_run_id,
		attempt_count, max_attempts, lease_owner, lease_expires_at, created_at, updated_at
	FROM harness_run_group_items
	WHERE group_id = ?
	ORDER BY item_index ASC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RunGroupItem, 0)
	for rows.Next() {
		item, err := scanGroupItemRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *SQLiteStore) CountGroupItemsByStatuses(ctx context.Context, groupID string, statuses []RunGroupItemStatus) (int, error) {
	if strings.TrimSpace(groupID) == "" || len(statuses) == 0 {
		return 0, nil
	}
	return s.countGroupItemsByStatuses(ctx, s.reader(), groupID, statuses)
}

func (s *SQLiteStore) countGroupItemsByStatuses(ctx context.Context, db *sql.DB, groupID string, statuses []RunGroupItemStatus) (int, error) {
	if strings.TrimSpace(groupID) == "" || len(statuses) == 0 {
		return 0, nil
	}
	statusValues := make([]interface{}, 0, len(statuses))
	for _, status := range statuses {
		if status == "" {
			continue
		}
		statusValues = append(statusValues, string(status))
	}
	if len(statusValues) == 0 {
		return 0, nil
	}
	var count int64
	if _, err := z.TableContext(ctx, db, "harness_run_group_items").Select(&count,
		z.Fields("COUNT(1)"),
		z.Where(
			z.Eq("group_id", strings.TrimSpace(groupID)),
			z.In("status", statusValues...),
		),
	); err != nil {
		return 0, err
	}
	if count == 0 && s.hasSeparateReader() && db == s.reader() {
		writerCount, writerErr := s.countGroupItemsByStatuses(ctx, s.db, groupID, statuses)
		if writerErr == nil {
			return writerCount, nil
		}
	}
	return int(count), nil
}

func (s *SQLiteStore) ClaimNextGroupItem(ctx context.Context, groupID, workerID string, leaseTTL time.Duration, now time.Time) (*RunGroupItem, error) {
	if strings.TrimSpace(groupID) == "" {
		return nil, sql.ErrNoRows
	}
	tx, err := s.beginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	var rows []harnessGroupItemRow
	if _, err = z.TableContext(ctx, tx, "harness_run_group_items").Select(&rows,
		z.Where(
			"group_id = ? AND status = ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?)",
			groupID,
			string(RunGroupItemStatusQueued),
			now,
		),
		z.OrderBy("item_index ASC"),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		_ = tx.Rollback()
		return nil, sql.ErrNoRows
	}
	item := harnessGroupItemFromRow(rows[0])
	item.Status = RunGroupItemStatusRunning
	item.LeaseOwner = workerID
	leaseUntil := now.Add(leaseTTL)
	item.LeaseExpiresAt = &leaseUntil
	item.UpdatedAt = now
	if _, err = z.TableContext(ctx, tx, "harness_run_group_items").Update(
		z.V{
			"status":           string(item.Status),
			"lease_owner":      item.LeaseOwner,
			"lease_expires_at": nullableTime(item.LeaseExpiresAt),
			"updated_at":       item.UpdatedAt,
		},
		z.Fields("status", "lease_owner", "lease_expires_at", "updated_at"),
		z.Where(z.Eq("id", item.ID)),
	); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *SQLiteStore) AttachScorecard(ctx context.Context, scorecard Scorecard) error {
	_, err := s.execContext(ctx, `INSERT OR REPLACE INTO harness_scorecards (
		id, group_id, group_item_id, run_id, mode, verdict, score, breakdown_json, evidence_json, judge_trace_json, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		scorecard.ID, scorecard.GroupID, scorecard.GroupItemID, scorecard.RunID, string(scorecard.Mode), string(scorecard.Verdict),
		scorecard.Score, scorecard.BreakdownJSON, scorecard.EvidenceJSON, scorecard.JudgeTraceJSON, scorecard.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) ListScorecards(ctx context.Context, groupID string) ([]Scorecard, error) {
	var rows []harnessScorecardRow
	if _, err := z.TableContext(ctx, s.reader(), "harness_scorecards").Select(&rows,
		z.Where(z.Eq("group_id", groupID)),
		z.OrderBy("created_at DESC", "id DESC"),
	); err != nil {
		return nil, err
	}
	return harnessScorecardsFromRows(rows), nil
}

func (s *SQLiteStore) LatestScorecardForItem(ctx context.Context, groupItemID string) (*Scorecard, error) {
	var rows []harnessScorecardRow
	if _, err := z.TableContext(ctx, s.reader(), "harness_scorecards").Select(&rows,
		z.Where(z.Eq("group_item_id", groupItemID)),
		z.OrderBy("created_at DESC", "id DESC"),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	scorecard := harnessScorecardFromRow(rows[0])
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
	if err := s.ensureColumn(ctx, "harness_runs", "provider_id", `ALTER TABLE harness_runs ADD COLUMN provider_id TEXT DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "harness_skill_revisions", "base_content_sha256", `ALTER TABLE harness_skill_revisions ADD COLUMN base_content_sha256 TEXT DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "harness_skill_revisions", "origin_case_id", `ALTER TABLE harness_skill_revisions ADD COLUMN origin_case_id TEXT DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "harness_skill_revisions", "followup_gate", `ALTER TABLE harness_skill_revisions ADD COLUMN followup_gate TEXT DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "harness_skill_revisions", "optimization_surface", `ALTER TABLE harness_skill_revisions ADD COLUMN optimization_surface TEXT DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "harness_skill_revisions", "decision_action", `ALTER TABLE harness_skill_revisions ADD COLUMN decision_action TEXT DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "harness_skill_revisions", "review_note", `ALTER TABLE harness_skill_revisions ADD COLUMN review_note TEXT DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "harness_skill_revisions", "reviewed_by", `ALTER TABLE harness_skill_revisions ADD COLUMN reviewed_by TEXT DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "harness_skill_revisions", "decision_log_json", `ALTER TABLE harness_skill_revisions ADD COLUMN decision_log_json TEXT DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "harness_skill_revisions", "reviewed_at", `ALTER TABLE harness_skill_revisions ADD COLUMN reviewed_at DATETIME`); err != nil {
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
	_, err = s.execContext(ctx, ddl)
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
		&run.Goal, &run.ProviderID, &run.Model, &run.Result, &run.Error, &run.Depth, &run.CurrentStep, &run.Progress, &run.WorkspaceRoot, &run.ArtifactRoot, &run.SandboxMode,
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

func withSQLiteBusyRetry(ctx context.Context, fn func() error) error {
	if fn == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	delay := harnessSQLiteBusyRetryBaseDelay
	var lastErr error
	for attempt := 0; attempt < harnessSQLiteBusyRetryAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil || !isSQLiteBusyError(lastErr) || attempt == harnessSQLiteBusyRetryAttempts-1 {
			return lastErr
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			if err := ctx.Err(); err != nil {
				return err
			}
			return lastErr
		case <-timer.C:
		}
		delay *= 2
	}
	return lastErr
}

func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	if isSQLiteBusyDriverError(err) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "database table is locked") ||
		strings.Contains(message, "database schema is locked")
}
