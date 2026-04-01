package contactstore

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

func TestStoreUsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "contacts.db")

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
	contact := &Contact{ID: "contact-r", OwnerID: "user-a", DisplayName: "Reader", Created: time.Now(), Updated: time.Now()}
	if err := store.Create(ctx, contact); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	got, err := store.Get(ctx, "contact-r", "user-a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.DisplayName != "Reader" {
		t.Fatalf("unexpected contact via reader: %+v", got)
	}

	contacts, err := store.ListByOwner(ctx, "user-a")
	if err != nil {
		t.Fatalf("ListByOwner: %v", err)
	}
	if len(contacts) != 1 || contacts[0].ID != "contact-r" {
		t.Fatalf("unexpected contacts via reader: %+v", contacts)
	}
}
