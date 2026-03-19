package harness

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newWriteGuardController(t *testing.T, artifactRoot, storePath string) *Controller {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	cfg := *config.DefaultHarnessConfig()
	cfg.ArtifactRoot = artifactRoot
	cfg.StorePath = storePath
	return NewController(store, NewPolicyResolver(cfg, nil))
}

func TestWritePathGuardProtectsRunWorkspaceAndReservedPaths(t *testing.T) {
	tmpDir := t.TempDir()
	artifactBase := filepath.Join(tmpDir, "artifacts")
	storePath := filepath.Join(tmpDir, "harness.db")
	controller := newWriteGuardController(t, artifactBase, storePath)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	workspaceRoot := filepath.Join(tmpDir, "workspace")
	if err := osMkdirAll(workspaceRoot); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "root",
		UserID:        "user-1",
		WorkspaceRoot: workspaceRoot,
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	guard := NewWritePathGuard(controller)
	ctx := tools.WithRunID(context.Background(), parent.ID)

	allowedPath := filepath.Join(workspaceRoot, "notes.txt")
	if err := guard.CheckWritePath(ctx, allowedPath); err != nil {
		t.Fatalf("expected workspace path to be allowed, got %v", err)
	}

	envPath := filepath.Join(workspaceRoot, ".env.local")
	if err := guard.CheckWritePath(ctx, envPath); err == nil || !strings.Contains(err.Error(), ".env") {
		t.Fatalf("expected .env protection error, got %v", err)
	}

	gitPath := filepath.Join(workspaceRoot, ".git", "config")
	if err := guard.CheckWritePath(ctx, gitPath); err == nil || !strings.Contains(err.Error(), ".git") {
		t.Fatalf("expected .git protection error, got %v", err)
	}

	outsidePath := filepath.Join(tmpDir, "outside", "notes.txt")
	if err := guard.CheckWritePath(ctx, outsidePath); err == nil || !strings.Contains(err.Error(), "workspace_root") {
		t.Fatalf("expected workspace root denial, got %v", err)
	}
}

func TestWritePathGuardDeniesOtherRunArtifactRoots(t *testing.T) {
	tmpDir := t.TempDir()
	artifactBase := filepath.Join(tmpDir, "artifacts")
	storePath := filepath.Join(tmpDir, "harness.db")
	controller := newWriteGuardController(t, artifactBase, storePath)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	workspaceRoot := filepath.Join(tmpDir, "workspace")
	if err := osMkdirAll(workspaceRoot); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	runA, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "run-a",
		UserID:        "user-1",
		WorkspaceRoot: workspaceRoot,
	})
	if err != nil {
		t.Fatalf("Submit runA failed: %v", err)
	}
	runB, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "run-b",
		UserID:        "user-1",
		WorkspaceRoot: workspaceRoot,
	})
	if err != nil {
		t.Fatalf("Submit runB failed: %v", err)
	}

	guard := NewWritePathGuard(controller)
	ctx := tools.WithRunID(context.Background(), runA.ID)
	otherArtifactPath := filepath.Join(runB.ArtifactRoot, "report.txt")
	if err := guard.CheckWritePath(ctx, otherArtifactPath); err == nil || !strings.Contains(err.Error(), "artifact") {
		t.Fatalf("expected artifact-root denial, got %v", err)
	}
}

func osMkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}
