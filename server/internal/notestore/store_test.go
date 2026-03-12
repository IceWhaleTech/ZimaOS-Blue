package notestore

import (
	"context"
	"database/sql"
	"errors"
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
