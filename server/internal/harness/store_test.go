package harness

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

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

	runs, err := store.ListRuns(ctx, RunFilter{UserID: "user-1", Kind: RunKindAgentTask, Limit: 10})
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatalf("ListRuns returned %#v", runs)
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
	want := "data/harness/artifacts/run-1"
	if got != "./"+want && got != want {
		t.Fatalf("ArtifactRoot() = %q", got)
	}
}

func TestNewPolicyResolverUsesSharedBlueDBPath(t *testing.T) {
	cfg := *config.DefaultHarnessConfig()
	cfg.StorePath = "./data/harness.db"
	resolver := NewPolicyResolver(cfg, nil)
	got := filepath.Clean(resolver.defaults.StorePath)
	want := filepath.Clean(filepath.Join(".", "data", "blue.db"))
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
