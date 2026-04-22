package harness

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func newTestHarnessStore(t *testing.T) *SQLiteStore {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "harness-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	return store
}

func TestSQLiteStore_CreateGetListRun(t *testing.T) {
	store := newTestHarnessStore(t)
	ctx := context.Background()
	run := &Run{
		ID:            "run-1",
		RootRunID:     "run-1",
		Kind:          RunKindAgentTask,
		Status:        RunStatusPending,
		UserID:        "user-1",
		Goal:          "ship harness",
		ProviderID:    "openai-prod",
		ArtifactRoot:  "./data/harness/artifacts/run-1",
		ApprovalMode:  ApprovalModeAsk,
		MaxDuration:   2 * time.Minute,
		MaxSteps:      10,
		MaxToolRounds: 5,
	}
	if err := store.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	got, err := store.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if got.ID != run.ID || got.Kind != run.Kind || got.UserID != run.UserID {
		t.Fatalf("GetRun mismatch: %#v", got)
	}
	if got.ProviderID != "openai-prod" {
		t.Fatalf("GetRun ProviderID = %q, want openai-prod", got.ProviderID)
	}

	runs, err := store.ListRuns(ctx, RunFilter{UserID: "user-1", Kind: RunKindAgentTask, Limit: 10})
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatalf("ListRuns returned %#v", runs)
	}
	if runs[0].ProviderID != "openai-prod" {
		t.Fatalf("ListRuns ProviderID = %q, want openai-prod", runs[0].ProviderID)
	}
}

func TestSQLiteStore_UpdateRunRoundTripsLifecycleTimes(t *testing.T) {
	store := newTestHarnessStore(t)
	ctx := context.Background()
	run := &Run{
		ID:            "run-lifecycle",
		RootRunID:     "run-lifecycle",
		Kind:          RunKindAgentTask,
		Status:        RunStatusPending,
		UserID:        "user-1",
		Goal:          "persist lifecycle times",
		ArtifactRoot:  "./data/harness/artifacts/run-lifecycle",
		ApprovalMode:  ApprovalModeAsk,
		MaxDuration:   2 * time.Minute,
		MaxSteps:      10,
		MaxToolRounds: 5,
	}
	if err := store.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	startedAt := time.Date(2026, time.April, 22, 15, 12, 5, 937059000, time.UTC)
	finishedAt := startedAt.Add(350 * time.Millisecond)
	run.Status = RunStatusCompleted
	run.StartedAt = &startedAt
	run.FinishedAt = &finishedAt
	if err := store.UpdateRun(ctx, run); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}

	got, err := store.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if got.StartedAt == nil || !got.StartedAt.Equal(startedAt) {
		t.Fatalf("GetRun StartedAt = %v, want %s", got.StartedAt, startedAt.Format(time.RFC3339Nano))
	}
	if got.FinishedAt == nil || !got.FinishedAt.Equal(finishedAt) {
		t.Fatalf("GetRun FinishedAt = %v, want %s", got.FinishedAt, finishedAt.Format(time.RFC3339Nano))
	}

	runs, err := store.ListRuns(ctx, RunFilter{UserID: "user-1", Limit: 10})
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(ListRuns) = %d, want 1", len(runs))
	}
	if runs[0].StartedAt == nil || !runs[0].StartedAt.Equal(startedAt) {
		t.Fatalf("ListRuns StartedAt = %v, want %s", runs[0].StartedAt, startedAt.Format(time.RFC3339Nano))
	}
	if runs[0].FinishedAt == nil || !runs[0].FinishedAt.Equal(finishedAt) {
		t.Fatalf("ListRuns FinishedAt = %v, want %s", runs[0].FinishedAt, finishedAt.Format(time.RFC3339Nano))
	}
}

func TestSQLiteStore_EventsAndArtifacts(t *testing.T) {
	store := newTestHarnessStore(t)
	ctx := context.Background()
	run := &Run{
		ID:           "run-1",
		RootRunID:    "run-1",
		Kind:         RunKindResearch,
		Status:       RunStatusPending,
		Goal:         "research harness",
		ArtifactRoot: "./data/harness/artifacts/run-1",
	}
	if err := store.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}
	event := RunEvent{
		ID:        "ev-1",
		RunID:     run.ID,
		RootRunID: run.RootRunID,
		Type:      "run_created",
		CreatedAt: time.Now().UTC(),
	}
	if err := store.AppendEvent(ctx, event); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}
	artifact := ArtifactRef{
		ID:        "art-1",
		RunID:     run.ID,
		Kind:      "file",
		Label:     "report",
		PathOrURL: "/tmp/report.md",
	}
	if err := store.AttachArtifact(ctx, artifact); err != nil {
		t.Fatalf("AttachArtifact failed: %v", err)
	}

	events, err := store.ListEvents(ctx, run.ID, 10)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("ListEvents returned %#v", events)
	}

	artifacts, err := store.ListArtifacts(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListArtifacts failed: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].ID != artifact.ID {
		t.Fatalf("ListArtifacts returned %#v", artifacts)
	}
}

func TestSQLiteStore_DeleteRunCascadesArtifactsAndEvents(t *testing.T) {
	store := newTestHarnessStore(t)
	ctx := context.Background()
	run := &Run{
		ID:           "run-1",
		RootRunID:    "run-1",
		Kind:         RunKindAgentTask,
		Status:       RunStatusPending,
		Goal:         "cleanup",
		ArtifactRoot: "./data/harness/artifacts/run-1",
	}
	if err := store.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}
	_ = store.AppendEvent(ctx, RunEvent{ID: "ev-1", RunID: run.ID, RootRunID: run.RootRunID, Type: "run_created", CreatedAt: time.Now().UTC()})
	_ = store.AttachArtifact(ctx, ArtifactRef{ID: "art-1", RunID: run.ID, Kind: "file", PathOrURL: "/tmp/a"})
	if err := store.DeleteRun(ctx, run.ID); err != nil {
		t.Fatalf("DeleteRun failed: %v", err)
	}
	if _, err := store.GetRun(ctx, run.ID); err == nil {
		t.Fatal("expected GetRun to fail after delete")
	}
	events, err := store.ListEvents(ctx, run.ID, 10)
	if err != nil {
		t.Fatalf("ListEvents after delete failed: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events after delete, got %#v", events)
	}
	artifacts, err := store.ListArtifacts(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListArtifacts after delete failed: %v", err)
	}
	if len(artifacts) != 0 {
		t.Fatalf("expected no artifacts after delete, got %#v", artifacts)
	}
}

func TestSQLiteStore_RecoversRunTreeAfterRestart(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "harness.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	parent := &Run{
		ID:            "run-parent",
		RootRunID:     "run-parent",
		Kind:          RunKindAgentTask,
		Status:        RunStatusExecuting,
		UserID:        "user-1",
		Goal:          "parent run",
		WorkspaceRoot: filepath.Join(filepath.Dir(dbPath), "workspace"),
		ArtifactRoot:  filepath.Join(filepath.Dir(dbPath), "artifacts", "run-parent"),
	}
	child := &Run{
		ID:            "run-child",
		RootRunID:     "run-parent",
		ParentRunID:   "run-parent",
		Kind:          RunKindSubagent,
		Status:        RunStatusPending,
		UserID:        "user-1",
		Goal:          "child run",
		WorkspaceRoot: parent.WorkspaceRoot,
		ArtifactRoot:  filepath.Join(filepath.Dir(dbPath), "artifacts", "run-child"),
	}

	if err := store.CreateRun(ctx, parent); err != nil {
		t.Fatalf("CreateRun parent failed: %v", err)
	}
	if err := store.CreateRun(ctx, child); err != nil {
		t.Fatalf("CreateRun child failed: %v", err)
	}
	if err := store.AppendEvent(ctx, RunEvent{
		ID:        "event-parent",
		RunID:     parent.ID,
		RootRunID: parent.RootRunID,
		Type:      "child_spawned",
		Message:   child.ID,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}
	if err := store.AttachArtifact(ctx, ArtifactRef{
		ID:        "artifact-child",
		RunID:     child.ID,
		Kind:      "log",
		PathOrURL: filepath.Join(filepath.Dir(dbPath), "artifacts", "run-child", "trace.log"),
	}); err != nil {
		t.Fatalf("AttachArtifact failed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite: %v", err)
	}

	reopenedDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("reopen sqlite: %v", err)
	}
	defer func() { _ = reopenedDB.Close() }()
	reopened, err := NewSQLiteStore(reopenedDB)
	if err != nil {
		t.Fatalf("reopen store failed: %v", err)
	}

	runs, err := reopened.ListRuns(ctx, RunFilter{RootRunID: parent.RootRunID, Limit: 10})
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("runs len = %d, want 2", len(runs))
	}

	recoveredChild, err := reopened.GetRun(ctx, child.ID)
	if err != nil {
		t.Fatalf("GetRun child failed: %v", err)
	}
	if recoveredChild.ParentRunID != parent.ID || recoveredChild.RootRunID != parent.RootRunID {
		t.Fatalf("unexpected recovered child: %#v", recoveredChild)
	}

	events, err := reopened.ListEvents(ctx, parent.ID, 10)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if len(events) != 1 || events[0].Type != "child_spawned" {
		t.Fatalf("unexpected recovered events: %#v", events)
	}

	artifacts, err := reopened.ListArtifacts(ctx, child.ID)
	if err != nil {
		t.Fatalf("ListArtifacts failed: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].ID != "artifact-child" {
		t.Fatalf("unexpected recovered artifacts: %#v", artifacts)
	}
}

func TestSQLiteStore_GroupRoundTrip(t *testing.T) {
	store := newTestHarnessStore(t)
	ctx := context.Background()
	group := &RunGroup{
		ID:          "group-1",
		Kind:        RunGroupKindEval,
		Title:       "eval batch",
		Status:      RunGroupStatusQueued,
		OwnerUserID: "user-1",
		Subject:     "score tasks",
		SchedulerConfig: GroupSchedulerConfig{
			MaxConcurrency: 2,
			MaxAttempts:    2,
			LeaseTTL:       30 * time.Second,
			RetryBackoff:   2 * time.Second,
		},
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeHybrid,
			RuleProfile:   "agent_task",
			JudgeModel:    "heuristic",
			PassThreshold: 0.6,
		},
		Metadata: map[string]interface{}{"suite": "smoke"},
	}
	if err := store.CreateGroup(ctx, group); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	items := []RunGroupItem{
		{
			ID:          "item-1",
			GroupID:     group.ID,
			Index:       0,
			RunKind:     RunKindAgentTask,
			Profile:     "agent_task",
			Input:       map[string]interface{}{"goal": "hello"},
			Expected:    map[string]interface{}{"contains": "done"},
			Status:      RunGroupItemStatusQueued,
			MaxAttempts: 2,
		},
	}
	if err := store.CreateGroupItems(ctx, items); err != nil {
		t.Fatalf("CreateGroupItems failed: %v", err)
	}
	if err := store.AttachScorecard(ctx, Scorecard{
		ID:            "score-1",
		GroupID:       group.ID,
		GroupItemID:   items[0].ID,
		RunID:         "run-1",
		Mode:          ScoringModeRule,
		Verdict:       ScoreVerdictPass,
		Score:         1,
		BreakdownJSON: `{"checks":[{"name":"contains","passed":true}]}`,
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AttachScorecard failed: %v", err)
	}

	gotGroup, err := store.GetGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("GetGroup failed: %v", err)
	}
	if gotGroup.Kind != group.Kind || gotGroup.OwnerUserID != group.OwnerUserID {
		t.Fatalf("GetGroup mismatch: %#v", gotGroup)
	}

	gotItems, err := store.ListGroupItems(ctx, group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(gotItems) != 1 || gotItems[0].ID != items[0].ID {
		t.Fatalf("ListGroupItems returned %#v", gotItems)
	}

	scorecards, err := store.ListScorecards(ctx, group.ID)
	if err != nil {
		t.Fatalf("ListScorecards failed: %v", err)
	}
	if len(scorecards) != 1 || scorecards[0].ID != "score-1" {
		t.Fatalf("ListScorecards returned %#v", scorecards)
	}
}

func TestNewSQLiteStore_ConfiguresSQLitePragmas(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "harness-pragmas.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer func() { _ = db.Close() }()

	if _, err := NewSQLiteStore(db); err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	var busyTimeout int
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout != harnessSQLiteBusyTimeoutMS {
		t.Fatalf("busy_timeout = %d, want %d", busyTimeout, harnessSQLiteBusyTimeoutMS)
	}

	var foreignKeys int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	var journalMode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
}

func TestNewSQLiteStore_ConstrainsConnectionPool(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "harness-pool.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := NewSQLiteStore(db); err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	stats := db.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Fatalf("max_open_connections = %d, want 1", stats.MaxOpenConnections)
	}
}

func TestWithSQLiteBusyRetryRetriesBusyErrors(t *testing.T) {
	calls := 0
	err := withSQLiteBusyRetry(context.Background(), func() error {
		calls++
		if calls < 3 {
			return newSQLiteBusyErrorForTest()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withSQLiteBusyRetry() error = %v", err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func TestWithSQLiteBusyRetryStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := withSQLiteBusyRetry(ctx, func() error {
		calls++
		cancel()
		return newSQLiteBusyErrorForTest()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestSQLiteStore_MigratesLegacyRunSchema(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "legacy-harness.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	legacySchema := `
CREATE TABLE harness_runs (
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
);`
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("create legacy schema failed: %v", err)
	}
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO harness_runs (
		id, root_run_id, parent_run_id, kind, status, runtime_state, user_id, conversation_id, session_id, agent_id,
		goal, model, result, error, depth, current_step, progress, workspace_root, artifact_root, sandbox_mode,
		approval_mode, max_duration_ns, max_steps, max_tool_rounds, max_subagents, max_depth, metadata_json,
		created_at, updated_at, started_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-legacy", "run-legacy", "", string(RunKindAgentTask), string(RunStatusCompleted), "", "user-1", "conv-1", "sess-1", "agent-1",
		"legacy run", "model-x", "done", "", 0, 1, 100, "/tmp/work", "/tmp/artifacts/run-legacy", "workspace",
		string(ApprovalModeAsk), int64(time.Minute), 5, 5, 0, 0, `{}`,
		now, now, now, now,
	); err != nil {
		t.Fatalf("insert legacy row failed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite: %v", err)
	}

	reopenedDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("reopen sqlite: %v", err)
	}
	defer func() { _ = reopenedDB.Close() }()
	store, err := NewSQLiteStore(reopenedDB)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	run, err := store.GetRun(ctx, "run-legacy")
	if err != nil {
		t.Fatalf("GetRun failed after migration: %v", err)
	}
	if run.GroupID != "" || run.GroupItemID != "" || run.AttemptIndex != 0 {
		t.Fatalf("unexpected migrated run fields: %#v", run)
	}
}

func TestNewPolicyResolverArtifactRoot(t *testing.T) {
	resolver := NewPolicyResolver(*config.DefaultHarnessConfig(), nil)
	got := resolver.ArtifactRoot("run-1")
	want, err := filepath.Abs(filepath.Join(".", "data", "harness", "artifacts", "run-1"))
	if err != nil {
		t.Fatalf("filepath.Abs failed: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("ArtifactRoot() = %q, want %q", got, want)
	}
}

func TestNewPolicyResolverDefaultWorkspaceRootIsAbsolute(t *testing.T) {
	resolver := NewPolicyResolver(*config.DefaultHarnessConfig(), nil)
	got := resolver.defaultWorkspaceRoot()
	want, err := filepath.Abs(filepath.Join(".", "data", "workspace"))
	if err != nil {
		t.Fatalf("filepath.Abs failed: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("defaultWorkspaceRoot() = %q, want %q", got, want)
	}
}

func TestNewPolicyResolverUsesSharedBlueDBPath(t *testing.T) {
	cfg := *config.DefaultHarnessConfig()
	cfg.StorePath = "./data/harness.db"
	resolver := NewPolicyResolver(cfg, nil)
	got := filepath.Clean(resolver.defaults.StorePath)
	want, err := filepath.Abs(filepath.Join(".", "data", "blue.db"))
	if err != nil {
		t.Fatalf("filepath.Abs failed: %v", err)
	}
	want = filepath.Clean(want)
	if got != want {
		t.Fatalf("defaults.StorePath = %q, want %q", got, want)
	}
}

func TestMigrateLegacyStoreImportsIntoSharedDBAndArchivesLegacyDB(t *testing.T) {
	dataDir := t.TempDir()
	legacyPath := LegacyStoreDBPath(dataDir)

	legacyDB, err := sql.Open("sqlite3", legacyPath)
	if err != nil {
		t.Fatalf("open legacy harness db: %v", err)
	}
	legacyStore, err := NewSQLiteStore(legacyDB)
	if err != nil {
		t.Fatalf("NewSQLiteStore(legacy) failed: %v", err)
	}

	ctx := context.Background()
	run := &Run{
		ID:           "run-legacy",
		RootRunID:    "run-legacy",
		Kind:         RunKindAgentTask,
		Status:       RunStatusCompleted,
		UserID:       "user-1",
		Goal:         "migrate legacy harness db",
		ArtifactRoot: "./data/harness/artifacts/run-legacy",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := legacyStore.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun(legacy) failed: %v", err)
	}
	if err := legacyStore.AppendEvent(ctx, RunEvent{
		ID:        "event-legacy",
		RunID:     run.ID,
		RootRunID: run.RootRunID,
		Type:      "run_completed",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent(legacy) failed: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	sharedDB, err := sql.Open("sqlite3", filepath.Join(dataDir, "blue.db"))
	if err != nil {
		t.Fatalf("open shared db: %v", err)
	}
	defer sharedDB.Close()

	result, err := MigrateLegacyStore(ctx, sharedDB, dataDir)
	if err != nil {
		t.Fatalf("MigrateLegacyStore() failed: %v", err)
	}
	if result == nil {
		t.Fatal("MigrateLegacyStore() returned nil result")
	}
	if result.RowsImported != 2 {
		t.Fatalf("RowsImported = %d, want 2", result.RowsImported)
	}
	if result.ArchivedPath == "" {
		t.Fatal("ArchivedPath is empty")
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy harness db to be archived, stat err=%v", err)
	}
	if _, err := os.Stat(result.ArchivedPath); err != nil {
		t.Fatalf("expected archived harness db to exist: %v", err)
	}

	sharedStore, err := NewSQLiteStore(sharedDB)
	if err != nil {
		t.Fatalf("NewSQLiteStore(shared) failed: %v", err)
	}
	gotRun, err := sharedStore.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun(shared) failed: %v", err)
	}
	if gotRun.Goal != run.Goal {
		t.Fatalf("shared run goal = %q, want %q", gotRun.Goal, run.Goal)
	}
	events, err := sharedStore.ListEvents(ctx, run.ID, 10)
	if err != nil {
		t.Fatalf("ListEvents(shared) failed: %v", err)
	}
	if len(events) != 1 || events[0].ID != "event-legacy" {
		t.Fatalf("unexpected shared events: %#v", events)
	}
}

func TestSQLiteStore_MigrateSchemaAddsProviderIDToLegacyHarnessRuns(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "legacy-harness-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
CREATE TABLE harness_runs (
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
)`)
	if err != nil {
		t.Fatalf("create legacy harness_runs failed: %v", err)
	}

	now := time.Now().UTC()
	_, err = db.Exec(`
INSERT INTO harness_runs (
	id, root_run_id, parent_run_id, kind, status, runtime_state, user_id, conversation_id, session_id, agent_id,
	goal, model, result, error, depth, current_step, progress, workspace_root, artifact_root, sandbox_mode,
	approval_mode, max_duration_ns, max_steps, max_tool_rounds, max_subagents, max_depth, metadata_json,
	created_at, updated_at, started_at, finished_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"legacy-run", "legacy-run", "", string(RunKindAgentTask), string(RunStatusPending), "", "user-1", "", "", "",
		"legacy goal", "", "", "", 0, 0, 0, "", "", "",
		string(ApprovalModeAsk), 0, 0, 0, 0, 0, `{}`,
		now, now, nil, nil,
	)
	if err != nil {
		t.Fatalf("insert legacy run failed: %v", err)
	}

	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	legacyRun, err := store.GetRun(ctx, "legacy-run")
	if err != nil {
		t.Fatalf("GetRun(legacy) failed: %v", err)
	}
	if legacyRun.ProviderID != "" {
		t.Fatalf("legacy ProviderID = %q, want empty", legacyRun.ProviderID)
	}

	upgraded := &Run{
		ID:         "upgraded-run",
		RootRunID:  "upgraded-run",
		Kind:       RunKindAgentTask,
		Status:     RunStatusPending,
		UserID:     "user-1",
		Goal:       "post-migration write",
		ProviderID: "openai-prod",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := store.CreateRun(ctx, upgraded); err != nil {
		t.Fatalf("CreateRun(upgraded) failed: %v", err)
	}

	got, err := store.GetRun(ctx, upgraded.ID)
	if err != nil {
		t.Fatalf("GetRun(upgraded) failed: %v", err)
	}
	if got.ProviderID != "openai-prod" {
		t.Fatalf("upgraded ProviderID = %q, want openai-prod", got.ProviderID)
	}
}
