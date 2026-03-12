package contextpack

import (
	"context"
	"path/filepath"
	"testing"
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
