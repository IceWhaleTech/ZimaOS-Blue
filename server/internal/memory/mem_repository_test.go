package memory

import (
	"context"
	"testing"
	"time"
)

func TestMemoryRepository_CreateAndGet(t *testing.T) {
	db := newTestDB(t)
	repo, err := NewMemoryRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	e := NewMemoryEntry("ns1", "hello world")
	e.Category = "fact"
	e.Tags = []string{"test", "greeting"}
	e.ComputeExpiresAt()

	if err := repo.Create(ctx, e); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(ctx, e.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != "hello world" {
		t.Errorf("Content = %q, want %q", got.Content, "hello world")
	}
	if got.Category != "fact" {
		t.Errorf("Category = %q, want %q", got.Category, "fact")
	}
	if len(got.Tags) != 2 {
		t.Errorf("Tags len = %d, want 2", len(got.Tags))
	}
}

func TestMemoryRepository_Update(t *testing.T) {
	db := newTestDB(t)
	repo, _ := NewMemoryRepository(db)
	ctx := context.Background()

	e := NewMemoryEntry("ns1", "v1 content")
	repo.Create(ctx, e)

	v2, err := repo.Update(ctx, e.ID, "v2 content")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if v2.Version != 2 {
		t.Errorf("Version = %d, want 2", v2.Version)
	}
	if v2.ParentID != e.ID {
		t.Errorf("ParentID = %q, want %q", v2.ParentID, e.ID)
	}
	if v2.Content != "v2 content" {
		t.Errorf("Content = %q, want %q", v2.Content, "v2 content")
	}
}

func TestMemoryRepository_IdempotentUpdate(t *testing.T) {
	db := newTestDB(t)
	repo, _ := NewMemoryRepository(db)
	ctx := context.Background()

	e := NewMemoryEntry("ns1", "same content")
	repo.Create(ctx, e)

	// Update with same content should be idempotent
	v2, err := repo.Update(ctx, e.ID, "same content")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if v2.Version != 1 {
		t.Errorf("idempotent update should keep Version=1, got %d", v2.Version)
	}
}

func TestMemoryRepository_Delete(t *testing.T) {
	db := newTestDB(t)
	repo, _ := NewMemoryRepository(db)
	ctx := context.Background()

	e := NewMemoryEntry("ns1", "to delete")
	repo.Create(ctx, e)

	if err := repo.Delete(ctx, e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := repo.Get(ctx, e.ID)
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if got.Status != EntryStatusDeleted {
		t.Errorf("Status = %q, want %q", got.Status, EntryStatusDeleted)
	}
}

func TestMemoryRepository_List(t *testing.T) {
	db := newTestDB(t)
	repo, _ := NewMemoryRepository(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		e := NewMemoryEntry("ns1", "entry")
		repo.Create(ctx, e)
	}
	// Different namespace
	e2 := NewMemoryEntry("ns2", "other")
	repo.Create(ctx, e2)

	list, err := repo.List(ctx, ListParams{Namespace: "ns1", Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 5 {
		t.Errorf("List len = %d, want 5", len(list))
	}
}

func TestMemoryRepository_History(t *testing.T) {
	db := newTestDB(t)
	repo, _ := NewMemoryRepository(db)
	ctx := context.Background()

	e := NewMemoryEntry("ns1", "v1")
	repo.Create(ctx, e)
	v2, _ := repo.Update(ctx, e.ID, "v2")
	v3, _ := repo.Update(ctx, v2.ID, "v3")

	chain, err := repo.History(ctx, v3.ID)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(chain) != 3 {
		t.Fatalf("History len = %d, want 3", len(chain))
	}
	if chain[0].Version != 3 || chain[1].Version != 2 || chain[2].Version != 1 {
		t.Errorf("versions = [%d,%d,%d], want [3,2,1]",
			chain[0].Version, chain[1].Version, chain[2].Version)
	}
}

func TestMemoryRepository_PurgeExpired(t *testing.T) {
	db := newTestDB(t)
	repo, _ := NewMemoryRepository(db)
	ctx := context.Background()

	// Create an expired entry
	e := NewMemoryEntry("ns1", "expired")
	e.TTL = Duration(1 * time.Millisecond)
	e.ComputeExpiresAt()
	repo.Create(ctx, e)

	// Create a permanent entry
	e2 := NewMemoryEntry("ns1", "permanent")
	repo.Create(ctx, e2)

	time.Sleep(5 * time.Millisecond)

	n, err := repo.PurgeExpired(ctx)
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if n != 1 {
		t.Errorf("purged = %d, want 1", n)
	}

	// Permanent entry should still exist
	_, err = repo.Get(ctx, e2.ID)
	if err != nil {
		t.Errorf("permanent entry should still exist: %v", err)
	}
}

func TestMemoryRepository_Stats(t *testing.T) {
	db := newTestDB(t)
	repo, _ := NewMemoryRepository(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		e := NewMemoryEntry("ns1", "content")
		repo.Create(ctx, e)
	}

	stats, err := repo.Stats(ctx, "ns1")
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.EntryCount != 3 {
		t.Errorf("EntryCount = %d, want 3", stats.EntryCount)
	}
	if stats.ActiveCount != 3 {
		t.Errorf("ActiveCount = %d, want 3", stats.ActiveCount)
	}
}
