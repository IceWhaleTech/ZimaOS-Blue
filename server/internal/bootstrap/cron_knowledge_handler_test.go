package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/knowledge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/labstack/echo/v4"
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

func TestKnowledgeCronHandlerUsesSmallModelWikiFixSettingForLint(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(repoRoot, "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeBootstrapTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nCompiled knowledge for cron lint.\n")

	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	e := echo.New()
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/settings",
		strings.NewReader(`{"small_model_enabled":true,"small_model_knowledge_fix_enabled":true}`),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	if err := settings.Patch(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Patch() error = %v", err)
	}

	llmCaller := &captureKnowledgeAuthorLLM{
		response: `{"pages":[{"title":"Blue Knowledge","slug":"readme","page_type":"source_summary","summary":"Runtime-authored summary","keywords":["blue","knowledge"],"content":"# Blue Knowledge\n\nRuntime-authored page body with enough detail."}],"conflicts":[],"gaps":[],"fallback_mode":false}`,
	}
	service := newRuntimeKnowledgeService(runtimeTaskSurfaceOptions{
		workspaceDir: workspaceDir,
		runtimeTaskSurfaceKnowledgeDeps: runtimeTaskSurfaceKnowledgeDeps{
			settingsHandler: settings,
			knowledgeAuthor: newRuntimeKnowledgeAuthor(llmCaller),
		},
	})
	if service == nil {
		t.Fatal("expected knowledge service")
	}

	compileReport, err := service.Compile(context.Background(), knowledge.CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	pagePath := filepath.Join(workspaceDir, "knowledge", "pages", compileReport.Pages[0].Slug+".md")
	body, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	parts := strings.SplitN(string(body), "\n---\n", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected page format: %q", string(body))
	}
	degradedFrontmatter := regexp.MustCompile(`(?m)^summary:.*$`).ReplaceAllString(parts[0], `summary: ""`)
	writeKnowledgeBootstrapTestFile(t, pagePath, degradedFrontmatter+"\n---\nshort\n")

	llmCaller.calls = 0
	llmCaller.lastProviderID = ""
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
	if llmCaller.calls == 0 {
		t.Fatal("expected cron lint repair to invoke runtime knowledge author llm")
	}
	if llmCaller.lastProviderID != smallmodelProviderID {
		t.Fatalf("provider_id = %q, want %q", llmCaller.lastProviderID, smallmodelProviderID)
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
