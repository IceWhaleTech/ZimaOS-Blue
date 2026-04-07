package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/knowledge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/labstack/echo/v4"
)

type captureKnowledgeAuthorLLM struct {
	calls          int
	lastProviderID string
	lastReq        llm.ChatRequest
	response       string
}

func (c *captureKnowledgeAuthorLLM) Name() string { return "capture-knowledge-author" }

func (c *captureKnowledgeAuthorLLM) Models() []string { return []string{"capture-knowledge-author"} }

func (c *captureKnowledgeAuthorLLM) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.calls++
	c.lastProviderID = proxy.GetPinnedProvider(ctx)
	c.lastReq = req
	return &llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: c.response},
	}, nil
}

func (c *captureKnowledgeAuthorLLM) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("stream unsupported")
}

func (c *captureKnowledgeAuthorLLM) ChatStreamCallback(context.Context, llm.ChatRequest, llm.StreamCallback) error {
	return fmt.Errorf("stream unsupported")
}

func TestRuntimeKnowledgeAuthorUsesPinnedProviderAndParsesPages(t *testing.T) {
	llmCaller := &captureKnowledgeAuthorLLM{
		response: `{"pages":[{"title":"Runtime Knowledge","slug":"readme","page_type":"source_summary","summary":"Runtime-authored summary","keywords":["runtime","knowledge"],"content":"# Runtime Knowledge\n\nRuntime-authored page body with enough detail."}],"conflicts":["review duplicate"],"gaps":["missing comparison"],"fallback_mode":false}`,
	}
	author := newRuntimeKnowledgeAuthor(llmCaller)

	result, err := author.AuthorDelta(proxy.WithPinnedProvider(context.Background(), "openai-prod"), knowledge.KnowledgeAuthorRequest{
		Schema:      "# Schema\n\nKeep concise.",
		GeneratedAt: time.Date(2026, 4, 7, 11, 0, 0, 0, time.UTC),
		Source: knowledge.SourceSnapshot{
			Ref:        "README.md",
			Title:      "Runtime Knowledge",
			PageType:   knowledge.PageTypeSourceSummary,
			Content:    "Knowledge runtime source document",
			Summary:    "Knowledge runtime source summary",
			Keywords:   []string{"runtime", "knowledge"},
			SourceHash: "hash-1",
			Slug:       "readme",
		},
	})
	if err != nil {
		t.Fatalf("AuthorDelta() error = %v", err)
	}
	if llmCaller.calls != 1 {
		t.Fatalf("llm calls = %d, want 1", llmCaller.calls)
	}
	if llmCaller.lastProviderID != "openai-prod" {
		t.Fatalf("provider_id = %q, want openai-prod", llmCaller.lastProviderID)
	}
	if result == nil || len(result.Pages) != 1 {
		t.Fatalf("result = %#v, want 1 page", result)
	}
	if got := result.Pages[0].SourceRefs; len(got) != 1 || got[0] != "README.md" {
		t.Fatalf("source_refs = %#v, want [README.md]", got)
	}
	if got := result.Pages[0].SourceHash; got != "hash-1" {
		t.Fatalf("source_hash = %q, want hash-1", got)
	}
}

func TestNewRuntimeKnowledgeServiceCompilesWithKnowledgeAuthor(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(repoRoot, "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeBootstrapTestFile(t, filepath.Join(repoRoot, "README.md"), "# Runtime Knowledge\n\nKnowledge runtime source document.\n")

	llmCaller := &captureKnowledgeAuthorLLM{
		response: `{"pages":[{"title":"Runtime Knowledge","slug":"readme","page_type":"source_summary","summary":"Runtime-authored summary","keywords":["runtime","knowledge"],"content":"# Runtime Knowledge\n\nRuntime-authored page body with enough detail."}],"conflicts":[],"gaps":[],"fallback_mode":false}`,
	}
	service := newRuntimeKnowledgeService(runtimeTaskSurfaceOptions{
		workspaceDir: workspaceDir,
		runtimeTaskSurfaceKnowledgeDeps: runtimeTaskSurfaceKnowledgeDeps{
			knowledgeAuthor: newRuntimeKnowledgeAuthor(llmCaller),
		},
	})
	if service == nil {
		t.Fatal("expected knowledge service")
	}

	report, err := service.Compile(context.Background(), knowledge.CompileRequest{ProviderID: "openai-prod"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if llmCaller.calls == 0 {
		t.Fatal("expected runtime knowledge author llm to be invoked")
	}
	if llmCaller.lastProviderID != "openai-prod" {
		t.Fatalf("provider_id = %q, want openai-prod", llmCaller.lastProviderID)
	}
	if report.FallbackMode {
		t.Fatalf("FallbackMode = %v, want false", report.FallbackMode)
	}
}

func TestNewRuntimeKnowledgeServiceUsesSmallModelWikiFixSettingForLint(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(repoRoot, "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeBootstrapTestFile(t, filepath.Join(repoRoot, "README.md"), "# Runtime Knowledge\n\nKnowledge runtime source document.\n")

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
		response: `{"pages":[{"title":"Runtime Knowledge","slug":"readme","page_type":"source_summary","summary":"Runtime-authored summary","keywords":["runtime","knowledge"],"content":"# Runtime Knowledge\n\nRuntime-authored page body with enough detail."}],"conflicts":[],"gaps":[],"fallback_mode":false}`,
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
	if _, err := service.Lint(context.Background(), knowledge.LintRequest{}); err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if llmCaller.calls == 0 {
		t.Fatal("expected lint repair to invoke runtime knowledge author llm")
	}
	if llmCaller.lastProviderID != smallmodelProviderID {
		t.Fatalf("provider_id = %q, want %q", llmCaller.lastProviderID, smallmodelProviderID)
	}
}

func TestRuntimeKnowledgeLintProviderSelectorRequiresBothSwitches(t *testing.T) {
	if selector := runtimeKnowledgeLintProviderSelector(nil); selector != nil {
		t.Fatal("expected nil selector when settings handler is nil")
	}

	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	selector := runtimeKnowledgeLintProviderSelector(settings)
	if selector == nil {
		t.Fatal("expected selector")
	}

	if got := selector(); got != "" {
		t.Fatalf("selector() with defaults = %q, want empty", got)
	}

	e := echo.New()
	patch := func(body string) {
		req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		if err := settings.Patch(e.NewContext(req, rec)); err != nil {
			t.Fatalf("Patch(%s) error = %v", body, err)
		}
	}

	patch(`{"small_model_enabled":true}`)
	if got := selector(); got != "" {
		t.Fatalf("selector() with only small_model_enabled = %q, want empty", got)
	}

	patch(`{"small_model_knowledge_fix_enabled":true}`)
	if got := selector(); got != smallmodelProviderID {
		t.Fatalf("selector() with both switches = %q, want %q", got, smallmodelProviderID)
	}

	patch(`{"small_model_enabled":false}`)
	if got := selector(); got != "" {
		t.Fatalf("selector() with small_model_enabled disabled = %q, want empty", got)
	}
}
