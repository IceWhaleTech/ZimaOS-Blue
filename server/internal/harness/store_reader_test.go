package harness

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestSQLiteStore_ReadsUseReaderDB(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "harness-reader.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open write sqlite: %v", err)
	}
	readDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("open read sqlite: %v", err)
	}
	t.Cleanup(func() { _ = readDB.Close() })

	store, err := NewSQLiteStoreWithReadDB(writeDB, readDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewSQLiteStoreWithReadDB failed: %v", err)
	}

	run := &Run{
		ID:            "run-1",
		RootRunID:     "run-1",
		Kind:          RunKindAgentTask,
		Status:        RunStatusPending,
		UserID:        "user-1",
		Goal:          "reader test",
		ArtifactRoot:  "./data/harness/artifacts/run-1",
		ApprovalMode:  ApprovalModeAsk,
		MaxDuration:   time.Minute,
		MaxSteps:      4,
		MaxToolRounds: 2,
	}
	if err := store.CreateRun(ctx, run); err != nil {
		_ = writeDB.Close()
		t.Fatalf("CreateRun failed: %v", err)
	}
	if err := store.AppendEvent(ctx, RunEvent{
		ID:        "ev-1",
		RunID:     run.ID,
		RootRunID: run.RootRunID,
		Type:      "run_created",
		Message:   "created",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		_ = writeDB.Close()
		t.Fatalf("AppendEvent failed: %v", err)
	}
	if err := store.AttachArtifact(ctx, ArtifactRef{
		ID:        "art-1",
		RunID:     run.ID,
		Kind:      "file",
		Label:     "log",
		PathOrURL: "/tmp/run-1.log",
	}); err != nil {
		_ = writeDB.Close()
		t.Fatalf("AttachArtifact failed: %v", err)
	}

	group := &RunGroup{
		ID:          "group-1",
		Kind:        RunGroupKindEval,
		Title:       "reader group",
		Status:      RunGroupStatusQueued,
		OwnerUserID: "user-1",
		Subject:     "reader",
		SchedulerConfig: GroupSchedulerConfig{
			MaxConcurrency: 1,
			MaxAttempts:    1,
			LeaseTTL:       15 * time.Second,
		},
		ScoringConfig: GroupScoringConfig{
			Mode:        ScoringModeRule,
			RuleProfile: "agent_task",
		},
	}
	if err := store.CreateGroup(ctx, group); err != nil {
		_ = writeDB.Close()
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
			MaxAttempts: 1,
		},
	}
	if err := store.CreateGroupItems(ctx, items); err != nil {
		_ = writeDB.Close()
		t.Fatalf("CreateGroupItems failed: %v", err)
	}
	if err := store.AttachScorecard(ctx, Scorecard{
		ID:            "score-1",
		GroupID:       group.ID,
		GroupItemID:   items[0].ID,
		RunID:         run.ID,
		Mode:          ScoringModeRule,
		Verdict:       ScoreVerdictPass,
		Score:         1,
		BreakdownJSON: `{"checks":[{"name":"contains","passed":true}]}`,
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		_ = writeDB.Close()
		t.Fatalf("AttachScorecard failed: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close write sqlite: %v", err)
	}

	gotRun, err := store.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun failed after closing write db: %v", err)
	}
	if gotRun.ID != run.ID {
		t.Fatalf("GetRun returned %q, want %q", gotRun.ID, run.ID)
	}

	runs, err := store.ListRuns(ctx, RunFilter{UserID: "user-1", RootRunID: run.RootRunID, Limit: 10})
	if err != nil {
		t.Fatalf("ListRuns failed after closing write db: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatalf("ListRuns returned %#v", runs)
	}

	events, err := store.ListEvents(ctx, run.ID, 10)
	if err != nil {
		t.Fatalf("ListEvents failed after closing write db: %v", err)
	}
	if len(events) != 1 || events[0].ID != "ev-1" {
		t.Fatalf("ListEvents returned %#v", events)
	}

	artifacts, err := store.ListArtifacts(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListArtifacts failed after closing write db: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].ID != "art-1" {
		t.Fatalf("ListArtifacts returned %#v", artifacts)
	}

	gotGroup, err := store.GetGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("GetGroup failed after closing write db: %v", err)
	}
	if gotGroup.ID != group.ID {
		t.Fatalf("GetGroup returned %q, want %q", gotGroup.ID, group.ID)
	}

	groups, err := store.ListGroups(ctx, RunGroupFilter{OwnerUserID: "user-1", Limit: 10})
	if err != nil {
		t.Fatalf("ListGroups failed after closing write db: %v", err)
	}
	if len(groups) != 1 || groups[0].ID != group.ID {
		t.Fatalf("ListGroups returned %#v", groups)
	}

	gotItem, err := store.GetGroupItem(ctx, items[0].ID)
	if err != nil {
		t.Fatalf("GetGroupItem failed after closing write db: %v", err)
	}
	if gotItem.ID != items[0].ID {
		t.Fatalf("GetGroupItem returned %q, want %q", gotItem.ID, items[0].ID)
	}

	groupItems, err := store.ListGroupItems(ctx, group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems failed after closing write db: %v", err)
	}
	if len(groupItems) != 1 || groupItems[0].ID != items[0].ID {
		t.Fatalf("ListGroupItems returned %#v", groupItems)
	}

	count, err := store.CountGroupItemsByStatuses(ctx, group.ID, []RunGroupItemStatus{RunGroupItemStatusQueued})
	if err != nil {
		t.Fatalf("CountGroupItemsByStatuses failed after closing write db: %v", err)
	}
	if count != 1 {
		t.Fatalf("CountGroupItemsByStatuses = %d, want 1", count)
	}

	scorecards, err := store.ListScorecards(ctx, group.ID)
	if err != nil {
		t.Fatalf("ListScorecards failed after closing write db: %v", err)
	}
	if len(scorecards) != 1 || scorecards[0].ID != "score-1" {
		t.Fatalf("ListScorecards returned %#v", scorecards)
	}

	latestScorecard, err := store.LatestScorecardForItem(ctx, items[0].ID)
	if err != nil {
		t.Fatalf("LatestScorecardForItem failed after closing write db: %v", err)
	}
	if latestScorecard.ID != "score-1" {
		t.Fatalf("LatestScorecardForItem returned %#v", latestScorecard)
	}
}

func TestSQLiteStore_GroupReadsFallBackWhenReaderMissesFreshGroup(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	staleReader, err := sql.Open("sqlite3", filepath.Join(tmpDir, "stale-reader.db"))
	if err != nil {
		t.Fatalf("open stale reader sqlite: %v", err)
	}
	t.Cleanup(func() { _ = staleReader.Close() })
	if _, err := NewSQLiteStoreWithReadDB(staleReader, staleReader); err != nil {
		t.Fatalf("initialize stale reader schema: %v", err)
	}

	writeDB, err := sql.Open("sqlite3", filepath.Join(tmpDir, "writer.db"))
	if err != nil {
		t.Fatalf("open writer sqlite: %v", err)
	}
	t.Cleanup(func() { _ = writeDB.Close() })

	store, err := NewSQLiteStoreWithReadDB(writeDB, staleReader)
	if err != nil {
		t.Fatalf("NewSQLiteStoreWithReadDB failed: %v", err)
	}

	readerSeed := &RunGroup{
		ID:          "reader-group",
		Kind:        RunGroupKindEval,
		Title:       "reader seed",
		Status:      RunGroupStatusQueued,
		OwnerUserID: "user-1",
	}
	readerStore, err := NewSQLiteStoreWithReadDB(staleReader, staleReader)
	if err != nil {
		t.Fatalf("reader seed store: %v", err)
	}
	if err := readerStore.CreateGroup(ctx, readerSeed); err != nil {
		t.Fatalf("seed reader group: %v", err)
	}

	group := &RunGroup{
		ID:          "writer-group",
		Kind:        RunGroupKindEval,
		Title:       "writer group",
		Status:      RunGroupStatusQueued,
		OwnerUserID: "user-1",
		Subject:     "fallback",
		SchedulerConfig: GroupSchedulerConfig{
			MaxConcurrency: 1,
			MaxAttempts:    1,
			LeaseTTL:       15 * time.Second,
		},
		ScoringConfig: GroupScoringConfig{
			Mode:        ScoringModeRule,
			RuleProfile: "agent_task",
		},
	}
	if err := store.CreateGroup(ctx, group); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if err := store.CreateGroupItems(ctx, []RunGroupItem{{
		ID:          "writer-item",
		GroupID:     group.ID,
		Index:       0,
		RunKind:     RunKindAgentTask,
		Profile:     "agent_task",
		Input:       map[string]interface{}{"goal": "fallback"},
		Expected:    map[string]interface{}{"contains": "done"},
		Status:      RunGroupItemStatusQueued,
		MaxAttempts: 1,
	}}); err != nil {
		t.Fatalf("CreateGroupItems failed: %v", err)
	}

	gotGroup, err := store.GetGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("GetGroup fallback failed: %v", err)
	}
	if gotGroup.ID != group.ID {
		t.Fatalf("GetGroup returned %q, want %q", gotGroup.ID, group.ID)
	}

	groups, err := store.ListGroups(ctx, RunGroupFilter{OwnerUserID: "user-1", Limit: 10})
	if err != nil {
		t.Fatalf("ListGroups fallback failed: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("ListGroups returned %d groups, want 2 (%#v)", len(groups), groups)
	}

	count, err := store.CountGroupItemsByStatuses(ctx, group.ID, []RunGroupItemStatus{RunGroupItemStatusQueued})
	if err != nil {
		t.Fatalf("CountGroupItemsByStatuses fallback failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("CountGroupItemsByStatuses = %d, want 1", count)
	}
}
