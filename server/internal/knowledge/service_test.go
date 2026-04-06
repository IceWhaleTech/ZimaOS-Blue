package knowledge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryRecord struct {
	content string
	tags    []string
}

type compileHookStub struct {
	mu     sync.Mutex
	calls  int
	report *KnowledgeCompileReport
}

func (s *compileHookStub) onCompile(_ context.Context, report *KnowledgeCompileReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.report = report
}

func (s *compileHookStub) snapshot() (int, *KnowledgeCompileReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls, s.report
}

type memorySinkStub struct {
	mu      sync.Mutex
	records []memoryRecord
}

func (s *memorySinkStub) Remember(_ context.Context, content string, tags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, memoryRecord{
		content: content,
		tags:    append([]string(nil), tags...),
	})
	return nil
}

func (s *memorySinkStub) snapshot() []memoryRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]memoryRecord, len(s.records))
	copy(out, s.records)
	return out
}

func TestServiceCompileBuildsPagesManifestAndWritesMemory(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nBlue architecture and knowledge workflows.\n")
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "ARCHITECTURE.md"), "# Blue Architecture\n\nArchitecture for the Blue knowledge runtime.\n")
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "docs-site", "guides", "automation.mdx"), "# Automation\n\nBlue automation and knowledge maintenance.\n")

	sink := &memorySinkStub{}
	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
		MemorySink:   sink,
		Now: func() time.Time {
			return time.Date(2026, 4, 5, 10, 11, 12, 0, time.UTC)
		},
	})

	report, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if report == nil {
		t.Fatal("Compile() report is nil")
	}
	if len(report.Pages) < 3 {
		t.Fatalf("expected at least 3 compiled pages, got %d", len(report.Pages))
	}

	pagePath := filepath.Join(workspaceDir, "knowledge", "pages", report.Pages[0].Slug+".md")
	pageBody, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("read compiled page: %v", err)
	}
	pageText := string(pageBody)
	for _, want := range []string{
		"title:",
		"slug:",
		"page_type:",
		"summary:",
		"source_refs:",
		"backlinks:",
		"generated_at:",
		"source_hash:",
	} {
		if !strings.Contains(pageText, want) {
			t.Fatalf("compiled page missing frontmatter field %q:\n%s", want, pageText)
		}
	}

	if _, err := os.Stat(filepath.Join(workspaceDir, "knowledge", "sources", "manifest.json")); err != nil {
		t.Fatalf("manifest.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "knowledge", "indexes", "index.md")); err != nil {
		t.Fatalf("index.md missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "knowledge", "SCHEMA.md")); err != nil {
		t.Fatalf("SCHEMA.md missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "knowledge", "log.md")); err != nil {
		t.Fatalf("log.md missing: %v", err)
	}
	if got := sink.snapshot(); len(got) == 0 {
		t.Fatal("expected distilled knowledge summaries to be written to memory")
	}
	if !report.FallbackMode {
		t.Fatal("expected default compile/ingest path to report fallback mode")
	}
	if strings.TrimSpace(report.SchemaPath) == "" || strings.TrimSpace(report.LogPath) == "" {
		t.Fatalf("expected schema/log paths in report, got %+v", report)
	}
}

func TestServiceCompileFallbackGeneratesEntityAndConceptPages(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nBlue knowledge workflows keep the wiki durable for operators.\n")
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "ARCHITECTURE.md"), "# Blue Runtime\n\nBlue runtime coordinates ingest, query, and lint for the knowledge space.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
		Now: func() time.Time {
			return time.Date(2026, 4, 5, 10, 30, 0, 0, time.UTC)
		},
	})

	report, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !hasPageSummary(report.Pages, PageTypeEntity, "Blue") {
		t.Fatalf("expected fallback ingest to generate entity page for Blue, got %+v", report.Pages)
	}
	if !hasPageSummary(report.Pages, PageTypeConcept, "Knowledge") {
		t.Fatalf("expected fallback ingest to generate concept page for Knowledge, got %+v", report.Pages)
	}
	if !hasPageSummary(report.Pages, PageTypeConcept, "Runtime") {
		t.Fatalf("expected fallback ingest to generate concept page for Runtime, got %+v", report.Pages)
	}
	if !hasPageSummary(report.NewPages, PageTypeEntity, "Blue") {
		t.Fatalf("expected entity page in new_pages delta, got %+v", report.NewPages)
	}
	if overlap := overlappingPageSlugs(report.NewPages, report.UpdatedPages); len(overlap) > 0 {
		t.Fatalf("expected new and updated deltas to be disjoint, overlapping slugs: %v", overlap)
	}

	indexBody, err := os.ReadFile(filepath.Join(workspaceDir, "knowledge", "indexes", "index.md"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	indexText := string(indexBody)
	for _, want := range []string{
		"## Entities",
		"[Blue](../pages/entity-blue.md)",
		"## Concepts",
		"[Knowledge](../pages/concept-knowledge.md)",
		"[Runtime](../pages/concept-runtime.md)",
	} {
		if !strings.Contains(indexText, want) {
			t.Fatalf("expected %q in grouped index, got:\n%s", want, indexText)
		}
	}
}

func TestServiceCompileInvokesOnCompileSuccess(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nBlue knowledge runtime.\n")

	hook := &compileHookStub{}
	svc := NewService(ServiceOptions{
		WorkspaceDir:     workspaceDir,
		RepoRoot:         repoRoot,
		OnCompileSuccess: hook.onCompile,
	})

	report, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	calls, hookedReport := hook.snapshot()
	if calls != 1 {
		t.Fatalf("hook calls = %d, want 1", calls)
	}
	if hookedReport == nil {
		t.Fatal("expected non-nil compile report in hook")
	}
	if hookedReport.IndexPath != report.IndexPath {
		t.Fatalf("hooked report index path = %q, want %q", hookedReport.IndexPath, report.IndexPath)
	}
}

func TestServiceLintDetectsStaleHashAndRepairsBacklinks(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nBlue knowledge architecture system.\n")
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "ARCHITECTURE.md"), "# Blue Runtime\n\nBlue knowledge architecture runtime.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
		Now: func() time.Time {
			return time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC)
		},
	})

	compileReport, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(compileReport.Pages) < 2 {
		t.Fatalf("expected at least 2 pages, got %d", len(compileReport.Pages))
	}

	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nBlue knowledge architecture system changed.\n")
	firstPage := filepath.Join(workspaceDir, "knowledge", "pages", compileReport.Pages[0].Slug+".md")
	pageContent, err := os.ReadFile(firstPage)
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	corrupted := strings.Replace(string(pageContent), "backlinks:", "backlinks: []\n# removed", 1)
	writeKnowledgeTestFile(t, firstPage, corrupted)
	writeKnowledgeTestFile(t, filepath.Join(workspaceDir, "knowledge", "pages", "empty-page.md"), "---\ntitle: Empty Page\nslug: empty-page\npage_type: note\nsummary: \"\"\nsource_refs: []\nkeywords: []\nbacklinks: []\ngenerated_at: 2026-04-05T11:00:00Z\nsource_hash: test\n---\n")

	report, err := svc.Lint(context.Background(), LintRequest{})
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report == nil {
		t.Fatal("Lint() report is nil")
	}
	assertLintIssueKind(t, report.Issues, IssueKindStaleHash)
	assertLintIssueKind(t, report.Issues, IssueKindLowQuality)
	assertLintIssueKind(t, report.Issues, IssueKindMissingBacklinks)

	repairedContent, err := os.ReadFile(firstPage)
	if err != nil {
		t.Fatalf("read repaired page: %v", err)
	}
	if strings.Contains(string(repairedContent), "backlinks: []") {
		t.Fatalf("expected lint to repair backlinks, got:\n%s", string(repairedContent))
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "knowledge", "lint", "latest.json")); err != nil {
		t.Fatalf("latest lint report missing: %v", err)
	}
}

func TestServiceAnswerArchivesAndReturnsCitations(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nBlue keeps a compiled knowledge workspace for architecture and docs.\n")
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "ARCHITECTURE.md"), "# Blue Architecture\n\nThe architecture explains memory, runtime, and knowledge indexing.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
		Now: func() time.Time {
			return time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
		},
	})

	compileReport, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(compileReport.Pages) == 0 {
		t.Fatal("expected compiled pages")
	}

	answerReport, err := svc.Answer(context.Background(), AnswerRequest{
		Query:         "How does Blue knowledge architecture work?",
		PageSlug:      compileReport.Pages[0].Slug,
		ArchiveAnswer: true,
	})
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if strings.TrimSpace(answerReport.Answer) == "" {
		t.Fatal("expected non-empty answer")
	}
	if len(answerReport.Citations) == 0 {
		t.Fatal("expected answer citations")
	}
	if strings.TrimSpace(answerReport.ArchivedPath) == "" {
		t.Fatal("expected archived answer path")
	}
	if _, err := os.Stat(answerReport.ArchivedPath); err != nil {
		t.Fatalf("archived answer missing: %v", err)
	}

	page, err := svc.GetPage(context.Background(), compileReport.Pages[0].Slug)
	if err != nil {
		t.Fatalf("GetPage() error = %v", err)
	}
	if len(page.Answers) == 0 {
		t.Fatal("expected page to surface linked archived answers")
	}
	if strings.TrimSpace(string(answerReport.Confidence)) == "" {
		t.Fatal("expected answer confidence")
	}
	if answerReport.OpenQuestions == nil || answerReport.ConflictNotes == nil {
		t.Fatalf("expected query diagnostics fields, got %+v", answerReport)
	}
}

func TestServiceCompileSupportsIncrementalIngestForTargetPaths(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nInitial readme facts.\n")
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "ARCHITECTURE.md"), "# Blue Architecture\n\nInitial architecture facts.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
		Now: func() time.Time {
			return time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
		},
	})

	initialReport, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(initialReport.Pages) < 2 {
		t.Fatalf("expected compiled pages, got %d", len(initialReport.Pages))
	}

	var architecturePage KnowledgePageSummary
	foundArchitecture := false
	for _, page := range initialReport.Pages {
		if len(page.SourceRefs) == 1 && page.SourceRefs[0] == "ARCHITECTURE.md" {
			architecturePage = page
			foundArchitecture = true
			break
		}
	}
	if !foundArchitecture {
		t.Fatalf("expected architecture page in %+v", initialReport.Pages)
	}
	architecturePath := filepath.Join(workspaceDir, "knowledge", "pages", architecturePage.Slug+".md")
	beforeBody, err := os.ReadFile(architecturePath)
	if err != nil {
		t.Fatalf("read architecture page: %v", err)
	}

	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nReadme facts changed for incremental ingest.\n")

	incrementalReport, err := svc.Compile(context.Background(), CompileRequest{TargetPaths: []string{"README.md"}})
	if err != nil {
		t.Fatalf("incremental Compile() error = %v", err)
	}
	if len(incrementalReport.UpdatedPages) == 0 {
		t.Fatalf("expected updated pages in incremental report, got %+v", incrementalReport)
	}
	for _, page := range incrementalReport.UpdatedPages {
		for _, ref := range page.SourceRefs {
			if ref != "README.md" {
				t.Fatalf("unexpected non-target page update in %+v", incrementalReport.UpdatedPages)
			}
		}
	}

	afterBody, err := os.ReadFile(architecturePath)
	if err != nil {
		t.Fatalf("read architecture page after incremental compile: %v", err)
	}
	if string(beforeBody) != string(afterBody) {
		t.Fatalf("expected unrelated page to remain unchanged during targeted ingest")
	}
}

func TestServiceCompileSkipsUnchangedSourceAndLogsReason(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nStable readme facts.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
		Now: func() time.Time {
			return time.Date(2026, 4, 5, 12, 30, 0, 0, time.UTC)
		},
	})

	report, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(report.Pages) == 0 {
		t.Fatal("expected compiled pages")
	}

	firstPagePath := filepath.Join(workspaceDir, "knowledge", "pages", report.Pages[0].Slug+".md")
	beforeBody, err := os.ReadFile(firstPagePath)
	if err != nil {
		t.Fatalf("read page before skip compile: %v", err)
	}

	skippedReport, err := svc.Compile(context.Background(), CompileRequest{TargetPaths: []string{"README.md"}})
	if err != nil {
		t.Fatalf("skip Compile() error = %v", err)
	}
	if len(skippedReport.SkippedSources) == 0 {
		t.Fatalf("expected skipped sources in report, got %+v", skippedReport)
	}
	afterBody, err := os.ReadFile(firstPagePath)
	if err != nil {
		t.Fatalf("read page after skip compile: %v", err)
	}
	if string(beforeBody) != string(afterBody) {
		t.Fatalf("expected unchanged source compile to skip page rewrites")
	}

	logBody, err := os.ReadFile(filepath.Join(workspaceDir, "knowledge", "log.md"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(logBody), "skipped unchanged source") {
		t.Fatalf("expected skip reason in log, got:\n%s", string(logBody))
	}
}

func TestServicePromoteQueryCreatesSynthesisPageAndUpdatesIndex(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nBlue keeps a compiled knowledge workspace.\n")
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "ARCHITECTURE.md"), "# Blue Architecture\n\nArchitecture explains memory and indexing.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
		Now: func() time.Time {
			return time.Date(2026, 4, 5, 13, 0, 0, 0, time.UTC)
		},
	})

	if _, err := svc.Compile(context.Background(), CompileRequest{}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Kind:          JobKindAnswer,
		Query:         "How does Blue knowledge work?",
		PageSlug:      "readme",
		ArchiveAnswer: true,
	})
	if err != nil {
		t.Fatalf("CreateJob(answer) error = %v", err)
	}
	waitForTerminalKnowledgeJob(t, svc, job.ID, 2*time.Second)

	promoted, err := svc.PromoteQuery(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("PromoteQuery() error = %v", err)
	}
	if promoted.PageType != PageTypeSynthesis {
		t.Fatalf("promoted page_type = %q, want %q", promoted.PageType, PageTypeSynthesis)
	}
	if strings.TrimSpace(promoted.DerivedFromQuery) == "" {
		t.Fatalf("expected derived_from_query on promoted page: %+v", promoted)
	}

	indexBody, err := os.ReadFile(filepath.Join(workspaceDir, "knowledge", "indexes", "index.md"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if !strings.Contains(string(indexBody), promoted.Title) {
		t.Fatalf("expected promoted page in index, got:\n%s", string(indexBody))
	}

	readmePage, err := svc.GetPage(context.Background(), "readme")
	if err != nil {
		t.Fatalf("GetPage(readme) error = %v", err)
	}
	if !containsString(readmePage.Backlinks, promoted.Slug) {
		t.Fatalf("expected promoted synthesis in backlinks, got %+v", readmePage.Backlinks)
	}
}

func TestServiceLintCategorizesKnowledgeIssuesAndMarksConflicts(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Shared Topic\n\nThe answer is blue.\n")
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "ARCHITECTURE.md"), "# Shared Topic\n\nThe answer is green.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
		Now: func() time.Time {
			return time.Date(2026, 4, 5, 14, 0, 0, 0, time.UTC)
		},
	})

	report, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(report.Pages) < 2 {
		t.Fatalf("expected multiple compiled pages, got %d", len(report.Pages))
	}

	lintReport, err := svc.Lint(context.Background(), LintRequest{})
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	assertLintIssueCategory(t, lintReport.Issues, "review_required")
	assertLintIssueCategory(t, lintReport.Issues, "research_suggestions")

	conflicted := 0
	for _, page := range report.Pages {
		pageDetail, err := svc.GetPage(context.Background(), page.Slug)
		if err != nil {
			t.Fatalf("GetPage(%s) error = %v", page.Slug, err)
		}
		if pageDetail.Status == KnowledgeStatusConflicted {
			conflicted++
		}
	}
	if conflicted == 0 {
		t.Fatal("expected lint to mark conflicting pages")
	}
}

func TestServiceListPagesRecoversMalformedCompiledPageDocument(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nBlue compiled knowledge pages.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
	})

	compileReport, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(compileReport.Pages) == 0 {
		t.Fatal("expected compiled pages")
	}

	targetSlug := compileReport.Pages[0].Slug
	writeKnowledgeTestFile(
		t,
		filepath.Join(workspaceDir, "knowledge", "pages", targetSlug+".md"),
		strings.Join([]string{
			"---",
			`title: "Broken knowledge page"`,
			fmt.Sprintf(`slug: %q`, targetSlug),
			`page_type: "note"`,
			`summary: "This page lost its frontmatter terminator."`,
			`source_refs: ["README.md"]`,
			`keywords: ["broken"]`,
			`backlinks: []`,
			`generated_at: "2026-04-05T12:00:00Z"`,
			`source_hash: "broken"`,
			"# Broken knowledge page",
			"",
			"The compiled body should still be recoverable.",
		}, "\n"),
	)

	pages, err := svc.ListPages(context.Background())
	if err != nil {
		t.Fatalf("ListPages() error = %v", err)
	}

	found := false
	for _, page := range pages {
		if page.Slug != targetSlug {
			continue
		}
		found = true
		if strings.TrimSpace(page.Title) == "" {
			t.Fatalf("expected recovered page title for slug %q", targetSlug)
		}
		break
	}
	if !found {
		t.Fatalf("expected recovered page slug %q in %+v", targetSlug, pages)
	}
}

func TestServiceGetPageRecoversMalformedCompiledPageDocument(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nBlue compiled knowledge pages.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
	})

	compileReport, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(compileReport.Pages) == 0 {
		t.Fatal("expected compiled pages")
	}

	targetSlug := compileReport.Pages[0].Slug
	writeKnowledgeTestFile(
		t,
		filepath.Join(workspaceDir, "knowledge", "pages", targetSlug+".md"),
		strings.Join([]string{
			"---",
			`title: "Broken detail page"`,
			fmt.Sprintf(`slug: %q`, targetSlug),
			`page_type: "note"`,
			`summary: "Detail metadata is malformed."`,
			`source_refs: ["README.md"]`,
			`keywords: ["broken"]`,
			`backlinks: []`,
			`generated_at: "2026-04-05T12:00:00Z"`,
			`source_hash: "broken"`,
			"# Broken detail page",
			"",
			"The compiled body should still render in detail view.",
		}, "\n"),
	)

	page, err := svc.GetPage(context.Background(), targetSlug)
	if err != nil {
		t.Fatalf("GetPage() error = %v", err)
	}
	if page.Slug != targetSlug {
		t.Fatalf("page slug = %q, want %q", page.Slug, targetSlug)
	}
	if !strings.Contains(page.Content, "The compiled body should still render in detail view.") {
		t.Fatalf("expected recovered page content, got %q", page.Content)
	}
}

func TestServiceGetPageSkipsMalformedArchivedAnswerDocument(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nBlue keeps a compiled knowledge workspace for architecture and docs.\n")
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "ARCHITECTURE.md"), "# Blue Architecture\n\nThe architecture explains memory, runtime, and knowledge indexing.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
		Now: func() time.Time {
			return time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
		},
	})

	compileReport, err := svc.Compile(context.Background(), CompileRequest{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(compileReport.Pages) == 0 {
		t.Fatal("expected compiled pages")
	}

	answerReport, err := svc.Answer(context.Background(), AnswerRequest{
		Query:         "How does Blue knowledge architecture work?",
		PageSlug:      compileReport.Pages[0].Slug,
		ArchiveAnswer: true,
	})
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if strings.TrimSpace(answerReport.ArchivedPath) == "" {
		t.Fatal("expected archived answer path")
	}

	writeKnowledgeTestFile(
		t,
		answerReport.ArchivedPath,
		strings.Join([]string{
			"---",
			`title: "Broken archived answer"`,
			fmt.Sprintf(`page_slug: %q`, compileReport.Pages[0].Slug),
			`query: "How does Blue knowledge architecture work?"`,
			`summary: "This archived answer lost its terminator."`,
			`generated_at: "2026-04-05T12:00:00Z"`,
			"",
			"The archived answer body should be ignored instead of crashing the page.",
		}, "\n"),
	)

	page, err := svc.GetPage(context.Background(), compileReport.Pages[0].Slug)
	if err != nil {
		t.Fatalf("GetPage() error = %v", err)
	}
	if len(page.Answers) != 0 {
		t.Fatalf("expected malformed archived answers to be skipped, got %+v", page.Answers)
	}
}

func TestServiceJobsLifecycle(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nBlue knowledge docs.\n")

	svc := NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
	})

	events, unsubscribe, err := svc.Subscribe("knowledge-job-test")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	defer unsubscribe()

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		RequestedID: "knowledge-job-test",
		Query:       "compile knowledge",
		Kind:        JobKindCompile,
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if job.ID == "" {
		t.Fatal("expected job id")
	}

	deadline := time.After(5 * time.Second)
	terminalSeen := false
	for !terminalSeen {
		select {
		case evt := <-events:
			if evt.Type == "job_completed" {
				terminalSeen = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for job completion event")
		}
	}

	got, err := svc.GetJobForUser(job.ID, "", "")
	if err != nil {
		t.Fatalf("GetJobForUser() error = %v", err)
	}
	if got.Kind != JobKindIngest {
		t.Fatalf("expected compile alias to normalize to ingest, got %s", got.Kind)
	}
	if got.Status != JobStatusCompleted {
		t.Fatalf("expected completed job, got %s", got.Status)
	}
	if _, err := svc.GetReportForUser(job.ID, "", ""); err != nil {
		t.Fatalf("GetReportForUser() error = %v", err)
	}
}

func writeKnowledgeTestFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertLintIssueKind(t *testing.T, issues []LintIssue, want IssueKind) {
	t.Helper()
	for _, issue := range issues {
		if issue.Kind == want {
			return
		}
	}
	t.Fatalf("expected lint issue %q in %+v", want, issues)
}

func assertLintIssueCategory(t *testing.T, issues []LintIssue, want string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Category == want {
			return
		}
	}
	t.Fatalf("expected lint issue category %q in %+v", want, issues)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func hasPageSummary(values []KnowledgePageSummary, pageType string, title string) bool {
	for _, value := range values {
		if value.PageType == pageType && value.Title == title {
			return true
		}
	}
	return false
}

func overlappingPageSlugs(left []KnowledgePageSummary, right []KnowledgePageSummary) []string {
	set := make(map[string]struct{}, len(left))
	for _, value := range left {
		set[value.Slug] = struct{}{}
	}
	overlap := make([]string, 0)
	for _, value := range right {
		if _, ok := set[value.Slug]; ok {
			overlap = append(overlap, value.Slug)
		}
	}
	return overlap
}
