package webpush

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestStoreSubscribeListAndReaderDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "webpush.db")

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
	if err := store.Subscribe(ctx, "user-1", &Subscription{
		Endpoint:  "https://example.test/1",
		KeyP256dh: "p256",
		KeyAuth:   "auth",
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	list, err := store.ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 1 || list[0].Endpoint != "https://example.test/1" {
		t.Fatalf("unexpected subscriptions: %+v", list)
	}

	if err := store.Unsubscribe(ctx, "https://example.test/1"); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}

	list, err = store.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty subscription list, got %+v", list)
	}
}

func TestStoreReadsStillWorkAfterWriterClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "webpush-reader-close.db")

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
	if err := store.Subscribe(ctx, "user-reader", &Subscription{
		Endpoint:  "https://example.test/reader",
		KeyP256dh: "p256",
		KeyAuth:   "auth",
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	list, err := store.ListByUser(ctx, "user-reader")
	if err != nil {
		t.Fatalf("ListByUser after writer close: %v", err)
	}
	if len(list) != 1 || list[0].Endpoint != "https://example.test/reader" {
		t.Fatalf("unexpected subscriptions via reader db: %+v", list)
	}

	all, err := store.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll after writer close: %v", err)
	}
	if len(all) != 1 || all[0].Endpoint != "https://example.test/reader" {
		t.Fatalf("unexpected full subscription list via reader db: %+v", all)
	}
}
