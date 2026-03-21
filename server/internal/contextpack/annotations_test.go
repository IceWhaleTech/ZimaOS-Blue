package contextpack

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAnnotationStoreUpsertListApplicableDelete(t *testing.T) {
	store, err := NewAnnotationStore(filepath.Join(t.TempDir(), "annotations.db"))
	if err != nil {
		t.Fatalf("NewAnnotationStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Upsert(ctx, Annotation{TenantID: "tenant-1", UserID: "user-1", EntryID: "openai/responses-api", Note: "Prefer compact tool outputs."}); err != nil {
		t.Fatalf("Upsert(global) error = %v", err)
	}
	if err := store.Upsert(ctx, Annotation{TenantID: "tenant-1", UserID: "user-1", EntryID: "openai/responses-api", File: "references/tools.md", Note: "Read this reference when tool drift appears."}); err != nil {
		t.Fatalf("Upsert(file) error = %v", err)
	}

	anns, err := store.List(ctx, AnnotationFilter{TenantID: "tenant-1", UserID: "user-1", EntryID: "openai/responses-api"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(anns) != 2 {
		t.Fatalf("len(anns) = %d, want 2", len(anns))
	}

	applicable, err := store.Applicable(ctx, AnnotationFilter{TenantID: "tenant-1", UserID: "user-1", EntryID: "openai/responses-api", File: "references/tools.md"})
	if err != nil {
		t.Fatalf("Applicable() error = %v", err)
	}
	if len(applicable) != 2 {
		t.Fatalf("len(applicable) = %d, want 2", len(applicable))
	}

	if err := store.Delete(ctx, AnnotationFilter{TenantID: "tenant-1", UserID: "user-1", EntryID: "openai/responses-api", File: "references/tools.md"}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	anns, err = store.List(ctx, AnnotationFilter{TenantID: "tenant-1", UserID: "user-1", EntryID: "openai/responses-api"})
	if err != nil {
		t.Fatalf("List(after delete) error = %v", err)
	}
	if len(anns) != 1 {
		t.Fatalf("len(anns after delete) = %d, want 1", len(anns))
	}
}

func TestMigrateLegacyAnnotationsImportsIntoSharedDBAndArchivesLegacyDB(t *testing.T) {
	dataDir := t.TempDir()
	legacyPath := LegacyAnnotationDBPath(dataDir)

	legacyStore, err := NewAnnotationStore(legacyPath)
	if err != nil {
		t.Fatalf("NewAnnotationStore(legacy) error = %v", err)
	}
	ctx := context.Background()
	if err := legacyStore.Upsert(ctx, Annotation{TenantID: "tenant-1", UserID: "user-1", EntryID: "openai/responses-api", Note: "Use shared blue.db annotations."}); err != nil {
		t.Fatalf("legacy Upsert() error = %v", err)
	}
	if err := legacyStore.Close(); err != nil {
		t.Fatalf("legacy Close() error = %v", err)
	}

	sharedDB, err := sql.Open("sqlite3", filepath.Join(dataDir, "blue.db"))
	if err != nil {
		t.Fatalf("sql.Open(shared) error = %v", err)
	}
	defer sharedDB.Close()

	result, err := MigrateLegacyAnnotations(ctx, sharedDB, dataDir)
	if err != nil {
		t.Fatalf("MigrateLegacyAnnotations() error = %v", err)
	}
	if result == nil {
		t.Fatal("MigrateLegacyAnnotations() returned nil result")
	}
	if result.RowsImported != 1 {
		t.Fatalf("RowsImported = %d, want 1", result.RowsImported)
	}
	if result.ArchivedPath == "" {
		t.Fatal("ArchivedPath is empty")
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy db to be archived, stat err=%v", err)
	}
	if _, err := os.Stat(result.ArchivedPath); err != nil {
		t.Fatalf("expected archived db to exist: %v", err)
	}

	sharedStore, err := NewAnnotationStoreWithDB(sharedDB)
	if err != nil {
		t.Fatalf("NewAnnotationStoreWithDB(shared) error = %v", err)
	}
	anns, err := sharedStore.List(ctx, AnnotationFilter{TenantID: "tenant-1", UserID: "user-1", EntryID: "openai/responses-api"})
	if err != nil {
		t.Fatalf("List(shared) error = %v", err)
	}
	if len(anns) != 1 {
		t.Fatalf("len(shared anns) = %d, want 1", len(anns))
	}
}

func TestMigrateLegacyAnnotationsDoesNotOverwriteExistingSharedRows(t *testing.T) {
	dataDir := t.TempDir()
	legacyPath := LegacyAnnotationDBPath(dataDir)

	legacyStore, err := NewAnnotationStore(legacyPath)
	if err != nil {
		t.Fatalf("NewAnnotationStore(legacy) error = %v", err)
	}
	ctx := context.Background()
	if err := legacyStore.Upsert(ctx, Annotation{TenantID: "tenant-1", UserID: "user-1", EntryID: "openai/responses-api", Note: "legacy note"}); err != nil {
		t.Fatalf("legacy Upsert() error = %v", err)
	}
	if err := legacyStore.Close(); err != nil {
		t.Fatalf("legacy Close() error = %v", err)
	}

	sharedDB, err := sql.Open("sqlite3", filepath.Join(dataDir, "blue.db"))
	if err != nil {
		t.Fatalf("sql.Open(shared) error = %v", err)
	}
	defer sharedDB.Close()

	sharedStore, err := NewAnnotationStoreWithDB(sharedDB)
	if err != nil {
		t.Fatalf("NewAnnotationStoreWithDB(shared) error = %v", err)
	}
	if err := sharedStore.Upsert(ctx, Annotation{TenantID: "tenant-1", UserID: "user-1", EntryID: "openai/responses-api", Note: "shared note"}); err != nil {
		t.Fatalf("shared Upsert() error = %v", err)
	}

	result, err := MigrateLegacyAnnotations(ctx, sharedDB, dataDir)
	if err != nil {
		t.Fatalf("MigrateLegacyAnnotations() error = %v", err)
	}
	if result == nil {
		t.Fatal("MigrateLegacyAnnotations() returned nil result")
	}
	if result.RowsImported != 0 {
		t.Fatalf("RowsImported = %d, want 0", result.RowsImported)
	}

	anns, err := sharedStore.List(ctx, AnnotationFilter{TenantID: "tenant-1", UserID: "user-1", EntryID: "openai/responses-api"})
	if err != nil {
		t.Fatalf("List(shared) error = %v", err)
	}
	if len(anns) != 1 {
		t.Fatalf("len(shared anns) = %d, want 1", len(anns))
	}
	if anns[0].Note != "shared note" {
		t.Fatalf("shared note = %q, want %q", anns[0].Note, "shared note")
	}
}
