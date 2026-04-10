package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type captureKnowledgeAuthor struct {
	calls          int
	lastProviderID string
	lastReq        KnowledgeAuthorRequest
	build          func(KnowledgeAuthorRequest) *KnowledgeAuthorResult
}

func (a *captureKnowledgeAuthor) AuthorDelta(ctx context.Context, req KnowledgeAuthorRequest) (*KnowledgeAuthorResult, error) {
	a.calls++
	a.lastProviderID = tools.GetProviderID(ctx)
	a.lastReq = req
	if a.build != nil {
		return a.build(req), nil
	}
	return &KnowledgeAuthorResult{}, nil
}

func TestServiceCreateJobPinsProviderIDForKnowledgeAuthor(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Runtime\n\nKnowledge author routing source.\n")

	author := &captureKnowledgeAuthor{
		build: func(req KnowledgeAuthorRequest) *KnowledgeAuthorResult {
			return &KnowledgeAuthorResult{
				Pages: []KnowledgePage{authoredKnowledgePage(req, "Authored page content with enough detail to avoid fallback.")},
			}
		},
	}
	svc := NewService(ServiceOptions{
		WorkspaceDir:    workspaceDir,
		RepoRoot:        repoRoot,
		KnowledgeAuthor: author,
		Now: func() time.Time {
			return time.Date(2026, 4, 7, 9, 0, 0, 0, time.UTC)
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Kind:       JobKindCompile,
		ProviderID: "openai-prod",
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	waitForTerminalKnowledgeJob(t, svc, job.ID, 2*time.Second)

	stored, err := svc.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if stored.ProviderID != "openai-prod" {
		t.Fatalf("job ProviderID = %q, want openai-prod", stored.ProviderID)
	}
	if author.calls == 0 {
		t.Fatal("expected knowledge author to be invoked")
	}
	if author.lastProviderID != "openai-prod" {
		t.Fatalf("author provider_id = %q, want openai-prod", author.lastProviderID)
	}
}

func TestServiceLintUsesKnowledgeAuthorToRepairLowQualityPages(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Runtime\n\nKnowledge lint repair source content.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
		Now: func() time.Time {
			return time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)
		},
	})

	compileReport, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(compileReport.Pages) == 0 {
		t.Fatal("expected compiled pages")
	}

	pagePath := filepath.Join(workspaceDir, "knowledge", "pages", compileReport.Pages[0].Slug+".md")
	body, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	doc, err := parsePageDocument(string(body))
	if err != nil {
		t.Fatalf("parsePageDocument() error = %v", err)
	}
	doc.Summary.Summary = ""
	doc.Content = "short"
	writeKnowledgeTestFile(t, pagePath, renderPageDocument(doc))

	repairAuthor := &captureKnowledgeAuthor{
		build: func(req KnowledgeAuthorRequest) *KnowledgeAuthorResult {
			return &KnowledgeAuthorResult{
				Pages: []KnowledgePage{authoredKnowledgePage(req, "Repaired page body with enough structure and detail to satisfy lint quality checks.")},
			}
		},
	}
	svc.author = repairAuthor

	lintReport, err := svc.Lint(context.Background(), LintRequest{ProviderID: "openai-prod"})
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if repairAuthor.calls == 0 {
		t.Fatal("expected lint to invoke knowledge author repair")
	}
	if repairAuthor.lastProviderID != "openai-prod" {
		t.Fatalf("repair author provider_id = %q, want openai-prod", repairAuthor.lastProviderID)
	}
	if hasLintIssueKind(lintReport.Issues, IssueKindLowQuality) {
		t.Fatalf("expected low-quality issue to be repaired, got %+v", lintReport.Issues)
	}

	repairedBody, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("read repaired page: %v", err)
	}
	repairedDoc, err := parsePageDocument(string(repairedBody))
	if err != nil {
		t.Fatalf("parse repaired page: %v", err)
	}
	if repairedDoc.Content == "short" || repairedDoc.Summary.Summary == "" {
		t.Fatalf("expected repaired page content, got %+v", repairedDoc)
	}
}

func TestServiceLintUsesDefaultLintProviderWhenConfigured(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Runtime\n\nKnowledge lint repair source content.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir:          workspaceDir,
		RepoRoot:              repoRoot,
		DefaultLintProviderID: func() string { return "smallmodel" },
		Now: func() time.Time {
			return time.Date(2026, 4, 7, 10, 30, 0, 0, time.UTC)
		},
	})

	compileReport, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	pagePath := filepath.Join(workspaceDir, "knowledge", "pages", compileReport.Pages[0].Slug+".md")
	body, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	doc, err := parsePageDocument(string(body))
	if err != nil {
		t.Fatalf("parsePageDocument() error = %v", err)
	}
	doc.Summary.Summary = ""
	doc.Content = "short"
	writeKnowledgeTestFile(t, pagePath, renderPageDocument(doc))

	repairAuthor := &captureKnowledgeAuthor{
		build: func(req KnowledgeAuthorRequest) *KnowledgeAuthorResult {
			return &KnowledgeAuthorResult{
				Pages: []KnowledgePage{authoredKnowledgePage(req, "Repaired page body with enough detail for the smallmodel wiki-fix path.")},
			}
		},
	}
	svc.author = repairAuthor

	lintReport, err := svc.Lint(context.Background(), LintRequest{})
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if repairAuthor.calls == 0 {
		t.Fatal("expected lint to invoke knowledge author repair")
	}
	if repairAuthor.lastProviderID != "smallmodel" {
		t.Fatalf("repair author provider_id = %q, want smallmodel", repairAuthor.lastProviderID)
	}
	if hasLintIssueKind(lintReport.Issues, IssueKindLowQuality) {
		t.Fatalf("expected low-quality issue to be repaired, got %+v", lintReport.Issues)
	}
}

func TestServiceLintPrefersExplicitProviderOverDefaultLintProvider(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Runtime\n\nKnowledge lint repair source content.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir:          workspaceDir,
		RepoRoot:              repoRoot,
		DefaultLintProviderID: func() string { return "smallmodel" },
		Now: func() time.Time {
			return time.Date(2026, 4, 7, 10, 45, 0, 0, time.UTC)
		},
	})

	compileReport, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	pagePath := filepath.Join(workspaceDir, "knowledge", "pages", compileReport.Pages[0].Slug+".md")
	body, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	doc, err := parsePageDocument(string(body))
	if err != nil {
		t.Fatalf("parsePageDocument() error = %v", err)
	}
	doc.Summary.Summary = ""
	doc.Content = "short"
	writeKnowledgeTestFile(t, pagePath, renderPageDocument(doc))

	repairAuthor := &captureKnowledgeAuthor{
		build: func(req KnowledgeAuthorRequest) *KnowledgeAuthorResult {
			return &KnowledgeAuthorResult{
				Pages: []KnowledgePage{authoredKnowledgePage(req, "Repaired page body with enough detail for the explicit-provider wiki-fix path.")},
			}
		},
	}
	svc.author = repairAuthor

	lintReport, err := svc.Lint(context.Background(), LintRequest{ProviderID: "openai-prod"})
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if repairAuthor.calls == 0 {
		t.Fatal("expected lint to invoke knowledge author repair")
	}
	if repairAuthor.lastProviderID != "openai-prod" {
		t.Fatalf("repair author provider_id = %q, want openai-prod", repairAuthor.lastProviderID)
	}
	if hasLintIssueKind(lintReport.Issues, IssueKindLowQuality) {
		t.Fatalf("expected low-quality issue to be repaired, got %+v", lintReport.Issues)
	}
}

func TestServiceCreateLintJobUsesDefaultLintProviderWhenConfigured(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Runtime\n\nKnowledge lint routing source content.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir:          workspaceDir,
		RepoRoot:              repoRoot,
		DefaultLintProviderID: func() string { return "smallmodel" },
		Now: func() time.Time {
			return time.Date(2026, 4, 7, 11, 0, 0, 0, time.UTC)
		},
	})

	if _, err := svc.Compile(context.Background(), CompileRequest{}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{Kind: JobKindLint})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	waitForTerminalKnowledgeJob(t, svc, job.ID, 2*time.Second)

	stored, err := svc.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if stored.ProviderID != "smallmodel" {
		t.Fatalf("job ProviderID = %q, want smallmodel", stored.ProviderID)
	}
}

func TestServiceCreateRepairConflictsJobUsesDefaultLintProviderWhenConfigured(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(
		t,
		filepath.Join(repoRoot, "README.md"),
		"# Shared Topic\n\nBlue is the canonical answer.\n\nThis source carries more detail and supporting context.\n",
	)
	writeKnowledgeTestFile(
		t,
		filepath.Join(repoRoot, "ARCHITECTURE.md"),
		"# Shared Topic\n\nBlue is mentioned here too.\n",
	)

	svc := NewService(ServiceOptions{
		WorkspaceDir:          workspaceDir,
		RepoRoot:              repoRoot,
		DefaultLintProviderID: func() string { return "smallmodel" },
		Now: func() time.Time {
			return time.Date(2026, 4, 7, 11, 15, 0, 0, time.UTC)
		},
	})

	if _, err := svc.Compile(context.Background(), CompileRequest{}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if _, err := svc.Lint(context.Background(), LintRequest{}); err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{Kind: JobKindRepairConflicts})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	waitForTerminalKnowledgeJob(t, svc, job.ID, 2*time.Second)

	stored, err := svc.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if stored.ProviderID != "smallmodel" {
		t.Fatalf("job ProviderID = %q, want smallmodel", stored.ProviderID)
	}
}

func authoredKnowledgePage(req KnowledgeAuthorRequest, content string) KnowledgePage {
	return KnowledgePage{
		KnowledgePageSummary: KnowledgePageSummary{
			Title:       req.Source.Title,
			Slug:        req.Source.Slug,
			PageType:    PageTypeSourceSummary,
			Summary:     "Authored knowledge summary for " + req.Source.Title,
			SourceRefs:  []string{req.Source.Ref},
			Keywords:    append([]string(nil), req.Source.Keywords...),
			GeneratedAt: req.GeneratedAt.UTC(),
			UpdatedAt:   req.GeneratedAt.UTC(),
			SourceHash:  req.Source.SourceHash,
			Status:      KnowledgeStatusActive,
			Confidence:  KnowledgeConfidenceMedium,
		},
		Content: content,
	}
}

func hasLintIssueKind(issues []LintIssue, want IssueKind) bool {
	for _, issue := range issues {
		if issue.Kind == want {
			return true
		}
	}
	return false
}
