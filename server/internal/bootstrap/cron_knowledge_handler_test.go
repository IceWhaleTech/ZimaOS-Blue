package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/knowledge"
	"go.uber.org/zap"
)

func TestKnowledgeCronHandlerSkipsWhenCompiledKnowledgeNotReady(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	service := knowledge.NewService(knowledge.ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
	})

	result, err := newKnowledgeCronHandler(service, zap.NewNop())(context.Background(), &cron.Job{
		ID:      "cron_job_knowledge_skip",
		Payload: map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map[string]interface{}", result)
	}
	if got := payload["status"]; got != "skipped" {
		t.Fatalf("status = %v, want skipped", got)
	}
	if got := payload["reason"]; got != "compiled knowledge not ready" {
		t.Fatalf("reason = %v, want compiled knowledge not ready", got)
	}
}

func TestKnowledgeCronHandlerRunsLintWhenCompiledKnowledgeExists(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeBootstrapTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nCompiled knowledge for cron lint.\n")

	service := knowledge.NewService(knowledge.ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
	})
	if _, err := service.Compile(context.Background(), knowledge.CompileRequest{}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := newKnowledgeCronHandler(service, zap.NewNop())(context.Background(), &cron.Job{
		ID:      "cron_job_knowledge_lint",
		Payload: map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map[string]interface{}", result)
	}
	if got := payload["status"]; got != "completed" {
		t.Fatalf("status = %v, want completed", got)
	}
	if got := payload["issue_count"]; got != 2 {
		t.Fatalf("issue_count = %v, want 2", got)
	}
}

func TestRegisterKnowledgeCronHandlerEnablesCronCreate(t *testing.T) {
	cronSvc := cron.NewService(cron.DefaultConfig(), zap.NewNop())
	registerKnowledgeCronHandler(cronSvc, knowledge.NewService(knowledge.ServiceOptions{}), zap.NewNop())

	_, err := cronSvc.Create(
		"knowledge-nightly-lint",
		"nightly knowledge lint",
		"0 3 * * *",
		knowledgeLintHandlerName,
		nil,
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func TestKnowledgeCompileEnsuresNightlyLintJobOnlyOnce(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeBootstrapTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nCompiled knowledge for nightly lint.\n")

	cronSvc := cron.NewService(cron.DefaultConfig(), zap.NewNop())
	registerKnowledgeCronHandler(cronSvc, knowledge.NewService(knowledge.ServiceOptions{}), zap.NewNop())

	service := knowledge.NewService(knowledge.ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
		OnCompileSuccess: func(_ context.Context, _ *knowledge.KnowledgeCompileReport) {
			if err := ensureNightlyKnowledgeLintJob(cronSvc, zap.NewNop()); err != nil {
				t.Fatalf("ensureNightlyKnowledgeLintJob() error = %v", err)
			}
		},
	})

	if _, err := service.Compile(context.Background(), knowledge.CompileRequest{}); err != nil {
		t.Fatalf("first Compile() error = %v", err)
	}
	if _, err := service.Compile(context.Background(), knowledge.CompileRequest{}); err != nil {
		t.Fatalf("second Compile() error = %v", err)
	}

	jobs := cronSvc.List()
	if len(jobs) != 1 {
		t.Fatalf("expected exactly one nightly knowledge lint job, got %d", len(jobs))
	}
	if jobs[0].Handler != knowledgeLintHandlerName {
		t.Fatalf("handler = %q, want %q", jobs[0].Handler, knowledgeLintHandlerName)
	}
	if jobs[0].Schedule != knowledgeNightlyLintSchedule {
		t.Fatalf("schedule = %q, want %q", jobs[0].Schedule, knowledgeNightlyLintSchedule)
	}
}

func writeKnowledgeBootstrapTestFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
