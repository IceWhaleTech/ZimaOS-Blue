package harness

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func newTestHarnessStore(t *testing.T) *SQLiteStore {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
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

func TestNewPolicyResolverArtifactRoot(t *testing.T) {
	resolver := NewPolicyResolver(*config.DefaultHarnessConfig(), nil)
	got := resolver.ArtifactRoot("run-1")
	want := "data/harness/artifacts/run-1"
	if got != "./"+want && got != want {
		t.Fatalf("ArtifactRoot() = %q", got)
	}
}
