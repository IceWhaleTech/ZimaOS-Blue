package contextpack

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAnnotationStoreUsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "annotations.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	defer writeDB.Close()

	if _, err := NewAnnotationStoreWithDB(writeDB); err != nil {
		t.Fatalf("NewAnnotationStoreWithDB(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewAnnotationStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewAnnotationStoreWithReadDB: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		t.Fatal("expected separate reader db")
	}

	ctx := context.Background()
	ann := Annotation{
		TenantID: "tenant-1",
		UserID:   "user-1",
		EntryID:  "entry-1",
		File:     "docs/ref.md",
		Note:     "reader note",
	}
	if err := store.Upsert(ctx, ann); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	anns, err := store.List(ctx, AnnotationFilter{
		TenantID: ann.TenantID,
		UserID:   ann.UserID,
		EntryID:  ann.EntryID,
		File:     ann.File,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(anns) != 1 || anns[0].Note != ann.Note {
		t.Fatalf("unexpected annotations via reader: %+v", anns)
	}
}
