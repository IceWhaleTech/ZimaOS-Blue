package memory

import (
	"context"
	"testing"
	"time"
)

func TestPurgeScheduler(t *testing.T) {
	db := newTestDB(t)
	repo, _ := NewMemoryRepository(db)
	ctx := context.Background()

	// Create an expired entry
	e := NewMemoryEntry("ns1", "will expire")
	e.TTL = Duration(1 * time.Millisecond)
	e.ComputeExpiresAt()
	repo.Create(ctx, e)

	// Create a permanent entry
	e2 := NewMemoryEntry("ns1", "permanent")
	repo.Create(ctx, e2)

	time.Sleep(5 * time.Millisecond)

	// Run scheduler with very short interval
	sched := NewPurgeScheduler(repo, 50*time.Millisecond)
	schedCtx, cancel := context.WithCancel(ctx)
	sched.Start(schedCtx)

	// Wait for at least one tick
	time.Sleep(150 * time.Millisecond)
	cancel()

	// Verify expired entry was purged
	_, err := repo.Get(ctx, e.ID)
	if err == nil {
		t.Error("expired entry should have been purged")
	}

	// Verify permanent entry still exists
	got, err := repo.Get(ctx, e2.ID)
	if err != nil {
		t.Fatalf("permanent entry should still exist: %v", err)
	}
	if got.Content != "permanent" {
		t.Errorf("Content = %q, want %q", got.Content, "permanent")
	}
}

func TestV2Bridge_OnRemember(t *testing.T) {
	db := newTestDB(t)
	repo, _ := NewMemoryRepository(db)
	ctx := context.Background()

	bridge := NewV2Bridge(repo)
	bridge.OnRemember(ctx, "test memory content", []string{"tag1", "tag2"}, "test-source")

	// Verify entry was created
	entries, err := repo.List(ctx, ListParams{Namespace: "default", Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Content != "test memory content" {
		t.Errorf("Content = %q", entries[0].Content)
	}
	if entries[0].Source != "test-source" {
		t.Errorf("Source = %q, want %q", entries[0].Source, "test-source")
	}
	if entries[0].Category != "auto" {
		t.Errorf("Category = %q, want %q", entries[0].Category, "auto")
	}
}

func TestV2Bridge_OnSessionEnd(t *testing.T) {
	db := newTestDB(t)
	repo, _ := NewMemoryRepository(db)
	ctx := context.Background()

	bridge := NewV2Bridge(repo)
	bridge.OnSessionEnd(ctx, "session summary", []string{"session", "archive"})

	entries, err := repo.List(ctx, ListParams{Namespace: "default", Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Source != "session" {
		t.Errorf("Source = %q, want %q", entries[0].Source, "session")
	}
	if entries[0].Category != "session" {
		t.Errorf("Category = %q, want %q", entries[0].Category, "session")
	}
}
