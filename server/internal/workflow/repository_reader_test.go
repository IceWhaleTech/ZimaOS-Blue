package workflow

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestRepository_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "workflow.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}

	if _, err := NewRepository(writeDB); err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewRepository(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	repo, err := NewRepositoryWithReadDB(writeDB, readDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewRepositoryWithReadDB: %v", err)
	}
	if repo.readDB == nil || repo.readDB == repo.db {
		_ = writeDB.Close()
		t.Fatal("expected separate reader db")
	}

	ctx := context.Background()
	wf := &Workflow{
		TenantID: "tenant-reader",
		Name:     "Reader Workflow",
		Status:   WorkflowStatusDraft,
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
		},
	}
	if err := repo.CreateWorkflow(ctx, wf); err != nil {
		_ = writeDB.Close()
		t.Fatalf("CreateWorkflow: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	got, err := repo.GetWorkflow(ctx, wf.ID)
	if err != nil {
		t.Fatalf("GetWorkflow via reader db: %v", err)
	}
	if got.Name != wf.Name {
		t.Fatalf("unexpected workflow via reader db: %+v", got)
	}

	workflows, total, err := repo.ListWorkflows(ctx, wf.TenantID, &ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListWorkflows via reader db: %v", err)
	}
	if total != 1 || len(workflows) != 1 || workflows[0].ID != wf.ID {
		t.Fatalf("unexpected workflow list via reader db: total=%d workflows=%+v", total, workflows)
	}
}
