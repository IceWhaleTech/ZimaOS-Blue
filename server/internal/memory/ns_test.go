package memory

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNamespaceStore_CRUD(t *testing.T) {
	db := newTestDB(t)
	store, err := NewNamespaceStore(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Create
	ns := &Namespace{
		ID:     "test-ns",
		Config: DefaultNamespaceConfig(),
	}
	if err := store.Create(ctx, ns); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Get
	got, err := store.Get(ctx, "test-ns")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "test-ns" {
		t.Errorf("ID = %q, want %q", got.ID, "test-ns")
	}
	if got.Config.MaxEntries != 100000 {
		t.Errorf("MaxEntries = %d, want 100000", got.Config.MaxEntries)
	}

	// List
	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List len = %d, want 1", len(list))
	}

	// UpdateConfig
	newCfg := DefaultNamespaceConfig()
	newCfg.MaxEntries = 50000
	if err := store.UpdateConfig(ctx, "test-ns", newCfg); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	got2, _ := store.Get(ctx, "test-ns")
	if got2.Config.MaxEntries != 50000 {
		t.Errorf("after update MaxEntries = %d, want 50000", got2.Config.MaxEntries)
	}

	// Delete
	if err := store.Delete(ctx, "test-ns"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = store.Get(ctx, "test-ns")
	if err == nil {
		t.Error("Get after Delete should return error")
	}
}

func TestNamespaceStore_NotFound(t *testing.T) {
	db := newTestDB(t)
	store, _ := NewNamespaceStore(db)
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("Get nonexistent should return error")
	}

	err = store.Delete(ctx, "nonexistent")
	if err == nil {
		t.Error("Delete nonexistent should return error")
	}
}
