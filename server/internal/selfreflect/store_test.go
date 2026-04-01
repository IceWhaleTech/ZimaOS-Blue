package selfreflect

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestSQLiteProposalStore_CreateUpdateListAndFind(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	store, err := NewSQLiteProposalStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteProposalStore: %v", err)
	}

	ctx := context.Background()
	first := &Proposal{
		OwnerUserID:        "user-1",
		SourceKind:         "reflection",
		SourceID:           "task-1",
		TargetFile:         "AGENTS.md",
		TargetSection:      "## Research Takeaways",
		DedupKey:           "dedup-1",
		Lesson:             "Run focused verification after parser edits.",
		WhenToApply:        "After changing parser control flow.",
		Evidence:           "A focused retry caught the regression.",
		EvidenceIDs:        []string{"e-1", "e-2"},
		EvaluationSummary:  map[string]interface{}{"score": 0.9},
		CalibrationSummary: map[string]interface{}{"confidence": "high"},
		PatchPreview:       "preview",
	}
	if err := store.CreateProposal(ctx, first); err != nil {
		t.Fatalf("CreateProposal(first): %v", err)
	}
	if first.ID == "" {
		t.Fatal("expected proposal ID to be assigned")
	}
	if first.Status != ProposalStatusPending {
		t.Fatalf("status = %q, want pending", first.Status)
	}
	if first.ProposalMode != ProposalModeReviewOnly {
		t.Fatalf("proposal mode = %q, want review_only", first.ProposalMode)
	}

	second := &Proposal{
		ID:            "proposal-2",
		OwnerUserID:   "user-2",
		SourceKind:    "review",
		SourceID:      "task-2",
		ProposalMode:  ProposalModeReviewOnly,
		TargetFile:    "AGENTS.md",
		Status:        ProposalStatusRejected,
		DedupKey:      "dedup-2",
		Lesson:        "Do not repeat failing retries.",
		Evidence:      "The same retry failed twice.",
		CreatedAt:     time.Now().UTC().Add(-time.Hour),
		UpdatedAt:     time.Now().UTC().Add(-time.Hour),
		WhenToApply:   "When recovery repeats the same failure.",
		TargetSection: "## Research Takeaways",
	}
	if err := store.CreateProposal(ctx, second); err != nil {
		t.Fatalf("CreateProposal(second): %v", err)
	}

	reviewedAt := time.Now().UTC()
	first.Status = ProposalStatusApproved
	first.ReviewNote = "ship it"
	first.ReviewedAt = &reviewedAt
	first.EvaluationSummary = map[string]interface{}{"score": 1.0}
	first.CalibrationSummary = map[string]interface{}{"confidence": "very_high"}
	if err := store.UpdateProposal(ctx, first); err != nil {
		t.Fatalf("UpdateProposal(first): %v", err)
	}

	got, err := store.GetProposal(ctx, first.ID)
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if got.Status != ProposalStatusApproved {
		t.Fatalf("updated status = %q, want approved", got.Status)
	}
	if got.ReviewNote != "ship it" {
		t.Fatalf("review note = %q, want ship it", got.ReviewNote)
	}
	if got.ReviewedAt == nil {
		t.Fatal("expected reviewed_at to be loaded")
	}
	if len(got.EvidenceIDs) != 2 || got.EvidenceIDs[0] != "e-1" {
		t.Fatalf("unexpected evidence ids: %+v", got.EvidenceIDs)
	}
	if got.EvaluationSummary["score"] != 1.0 {
		t.Fatalf("unexpected evaluation summary: %+v", got.EvaluationSummary)
	}

	listed, err := store.ListProposals(ctx, ProposalFilter{
		OwnerUserID: "user-1",
		SourceKind:  "reflection",
		SourceID:    "task-1",
		Statuses:    []ProposalStatus{ProposalStatusApproved},
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListProposals: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != first.ID {
		t.Fatalf("unexpected filtered proposals: %+v", listed)
	}

	found, err := store.FindProposalByDedup(ctx, "dedup-1", "AGENTS.md")
	if err != nil {
		t.Fatalf("FindProposalByDedup: %v", err)
	}
	if found.ID != first.ID {
		t.Fatalf("found proposal id = %q, want %q", found.ID, first.ID)
	}

	if _, err := store.GetProposal(ctx, "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetProposal(missing) err = %v, want sql.ErrNoRows", err)
	}
}

func TestSQLiteProposalStore_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "selfreflect.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	defer writeDB.Close()

	if _, err := NewSQLiteProposalStore(writeDB); err != nil {
		t.Fatalf("NewSQLiteProposalStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewSQLiteProposalStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewSQLiteProposalStoreWithReadDB: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		t.Fatal("expected separate reader db")
	}

	ctx := context.Background()
	proposal := &Proposal{
		ID:           "reader-proposal",
		OwnerUserID:  "user-reader",
		SourceKind:   "reflection",
		SourceID:     "task-reader",
		TargetFile:   "AGENTS.md",
		Status:       ProposalStatusPending,
		DedupKey:     "reader-dedup",
		Lesson:       "Read through the separate pool.",
		Evidence:     "Reader DB should see committed proposal rows.",
		ProposalMode: ProposalModeReviewOnly,
	}
	if err := store.CreateProposal(ctx, proposal); err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}

	got, err := store.GetProposal(ctx, "reader-proposal")
	if err != nil {
		t.Fatalf("GetProposal via reader: %v", err)
	}
	if got == nil || got.Lesson != proposal.Lesson {
		t.Fatalf("unexpected proposal via reader: %+v", got)
	}

	listed, err := store.ListProposals(ctx, ProposalFilter{
		OwnerUserID: "user-reader",
		Limit:       5,
	})
	if err != nil {
		t.Fatalf("ListProposals via reader: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "reader-proposal" {
		t.Fatalf("unexpected proposals via reader: %+v", listed)
	}
}
