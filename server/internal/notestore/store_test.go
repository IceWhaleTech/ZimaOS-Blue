package notestore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestStoreOwnerScopeIsolation(t *testing.T) {
	store, err := NewStore(testDB(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	note := &Note{ID: "note-1", OwnerID: "user-a", Title: "a", Content: "secret", Created: time.Now(), Updated: time.Now()}
	if err := store.Create(ctx, note); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Get(ctx, "note-1", "user-b"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Get(wrong owner) err = %v, want sql.ErrNoRows", err)
	}
	if err := store.Update(ctx, &Note{ID: "note-1", OwnerID: "user-b", Title: "b", Content: "x", Updated: time.Now()}, "user-b"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Update(wrong owner) err = %v, want sql.ErrNoRows", err)
	}
	if err := store.Delete(ctx, "note-1", "user-b"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Delete(wrong owner) err = %v, want sql.ErrNoRows", err)
	}

	got, err := store.Get(ctx, "note-1", "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "secret" {
		t.Fatalf("content = %q, want secret", got.Content)
	}
}

func TestStoreUsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "notes.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	defer writeDB.Close()

	if _, err := NewStore(writeDB); err != nil {
		t.Fatalf("NewStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewStoreWithReadDB: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		t.Fatal("expected separate reader db")
	}

	ctx := context.Background()
	note := &Note{ID: "note-r", OwnerID: "user-a", Title: "reader", Content: "hello", Created: time.Now(), Updated: time.Now()}
	if err := store.Create(ctx, note); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	got, err := store.Get(ctx, "note-r", "user-a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.Content != "hello" {
		t.Fatalf("unexpected note via reader: %+v", got)
	}

	notes, err := store.ListByOwner(ctx, "user-a")
	if err != nil {
		t.Fatalf("ListByOwner: %v", err)
	}
	if len(notes) != 1 || notes[0].ID != "note-r" {
		t.Fatalf("unexpected notes via reader: %+v", notes)
	}
}
