package workflow

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func newTestWorkflowService(t *testing.T) *WorkflowService {
	t.Helper()

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "workflow.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	svc, err := NewService(nil, repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return svc
}

func TestLazyHandlerIdleReclaimsAndRecreatesService(t *testing.T) {
	var created int
	h := NewLazyHandler(func() *WorkflowService {
		created++
		return newTestWorkflowService(t)
	})
	h.SetIdleReclaim(20 * time.Millisecond)

	first := h.GetService()
	if first == nil {
		t.Fatal("GetService() returned nil")
	}
	if created != 1 {
		t.Fatalf("created after first GetService = %d, want 1", created)
	}

	time.Sleep(80 * time.Millisecond)

	second := h.GetService()
	if second == nil {
		t.Fatal("GetService() second call returned nil")
	}
	if created != 2 {
		t.Fatalf("created after idle reclaim = %d, want 2", created)
	}
	if first == second {
		t.Fatal("expected a recreated workflow service after idle reclaim")
	}
}
