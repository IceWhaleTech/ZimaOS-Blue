package contactstore

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
	contact := &Contact{ID: "contact-1", OwnerID: "user-a", DisplayName: "Alice", Created: time.Now(), Updated: time.Now()}
	if err := store.Create(ctx, contact); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Get(ctx, "contact-1", "user-b"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Get(wrong owner) err = %v, want sql.ErrNoRows", err)
	}
	if err := store.Update(ctx, &Contact{ID: "contact-1", OwnerID: "user-b", DisplayName: "Bob", Updated: time.Now()}, "user-b"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Update(wrong owner) err = %v, want sql.ErrNoRows", err)
	}
	if err := store.Delete(ctx, "contact-1", "user-b"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Delete(wrong owner) err = %v, want sql.ErrNoRows", err)
	}

	got, err := store.Get(ctx, "contact-1", "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Alice" {
		t.Fatalf("display_name = %q, want Alice", got.DisplayName)
	}
}
