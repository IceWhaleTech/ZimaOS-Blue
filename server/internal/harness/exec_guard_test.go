package harness

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestExecPathGuardProtectsRunWorkspaceAndReservedPaths(t *testing.T) {
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
	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "exec-root",
		UserID:        "user-1",
		WorkspaceRoot: workspaceRoot,
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	guard := NewExecPathGuard(controller)
	ctx := tools.WithRunID(context.Background(), run.ID)

	if err := guard.CheckExecWorkdir(ctx, workspaceRoot); err != nil {
		t.Fatalf("expected workspace workdir to be allowed, got %v", err)
	}

	outsidePath := filepath.Join(tmpDir, "outside")
	if err := guard.CheckExecWorkdir(ctx, outsidePath); err == nil || !strings.Contains(err.Error(), "workspace_root") {
		t.Fatalf("expected workspace_root denial, got %v", err)
	}

	envPath := filepath.Join(workspaceRoot, ".env.local")
	if err := guard.CheckExecPath(ctx, envPath); err == nil || !strings.Contains(err.Error(), ".env") {
		t.Fatalf("expected .env protection error, got %v", err)
	}
}

func TestExecPathGuardDeniesOtherRunArtifactRoots(t *testing.T) {
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

	guard := NewExecPathGuard(controller)
	ctx := tools.WithRunID(context.Background(), runA.ID)
	otherArtifactPath := filepath.Join(runB.ArtifactRoot, "report.txt")
	if err := guard.CheckExecPath(ctx, otherArtifactPath); err == nil || !strings.Contains(err.Error(), "artifact") {
		t.Fatalf("expected artifact-root denial, got %v", err)
	}
}
