package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveLoopContext_PrefersActiveSynthesisAndDecisionPages(t *testing.T) {
	svc := newLoopContextTestService(t)
	writeLoopContextPage(t, svc, KnowledgePageSummary{
		Title:      "Agent Loop Strategy",
		Slug:       "agent-loop-strategy",
		PageType:   PageTypeSynthesis,
		Summary:    "Use a lightweight task brief during planning, then a tighter step evidence pack during execution.",
		SourceRefs: []string{"docs/agent-loop.md"},
		Keywords:   []string{"agent", "loop", "knowledge", "planning", "execution"},
		Status:     KnowledgeStatusActive,
		Confidence: KnowledgeConfidenceHigh,
	}, "# Agent Loop Strategy\n\nPrefer stage-aware knowledge injection.")
	writeLoopContextPage(t, svc, KnowledgePageSummary{
		Title:      "Knowledge Injection Decision",
		Slug:       "knowledge-injection-decision",
		PageType:   PageTypeDecision,
		Summary:    "Keep AGENTS.md and MEMORY.md in project context, and inject compiled knowledge only into user prompts when relevant.",
		SourceRefs: []string{"docs/decision.md"},
		Keywords:   []string{"knowledge", "decision", "planner", "project_context"},
		Status:     KnowledgeStatusActive,
		Confidence: KnowledgeConfidenceHigh,
	}, "# Decision\n\nKnowledge is reference evidence, not operator instruction.")
	writeLoopContextPage(t, svc, KnowledgePageSummary{
		Title:      "Knowledge Concepts",
		Slug:       "knowledge-concepts",
		PageType:   PageTypeConcept,
		Summary:    "Compiled knowledge pages are durable summaries for agents.",
		SourceRefs: []string{"docs/concepts.md"},
		Keywords:   []string{"knowledge", "compiled", "summary"},
		Status:     KnowledgeStatusActive,
		Confidence: KnowledgeConfidenceHigh,
	}, "# Concepts\n\nGeneral compiled knowledge concepts.")

	result, err := svc.ResolveLoopContext(context.Background(), LoopContextRequest{
		Stage:       LoopContextStagePlanning,
		Goal:        "Inject the right compiled knowledge into planning and execution prompts.",
		PlanSummary: "Add task-level planning knowledge and step-level execution evidence.",
		TokenBudget: 400,
	})
	if err != nil {
		t.Fatalf("ResolveLoopContext() error = %v", err)
	}
	if len(result.UsedSlugs) < 2 {
		t.Fatalf("used slugs = %v, want at least 2", result.UsedSlugs)
	}
	if result.UsedSlugs[0] != "agent-loop-strategy" || result.UsedSlugs[1] != "knowledge-injection-decision" {
		t.Fatalf("used slugs = %v, want synthesis/decision first", result.UsedSlugs)
	}
	if !strings.Contains(result.Context, "<loop_knowledge>") {
		t.Fatalf("expected loop knowledge wrapper, got %q", result.Context)
	}
}

func TestResolveLoopContext_AllowsConflictedPageWhenOnlyRelevantButAddsRiskLabel(t *testing.T) {
	svc := newLoopContextTestService(t)
	writeLoopContextPage(t, svc, KnowledgePageSummary{
		Title:      "Unrelated Active Page",
		Slug:       "unrelated-active-page",
		PageType:   PageTypeConcept,
		Summary:    "This page is about a different topic and should not win relevance.",
		SourceRefs: []string{"docs/other.md"},
		Keywords:   []string{"different", "topic"},
		Status:     KnowledgeStatusActive,
		Confidence: KnowledgeConfidenceHigh,
	}, "# Unrelated\n\nNothing about loop recovery here.")
	writeLoopContextPage(t, svc, KnowledgePageSummary{
		Title:         "Recovery Evidence Gap Notes",
		Slug:          "recovery-evidence-gap-notes",
		PageType:      PageTypeDecision,
		Summary:       "When recovery is blocked by missing evidence, re-resolve knowledge for the recovery stage and label the risk because this guidance is conflicted.",
		SourceRefs:    []string{"docs/recovery.md"},
		Keywords:      []string{"recovery", "missing evidence", "knowledge"},
		Status:        KnowledgeStatusConflicted,
		Confidence:    KnowledgeConfidenceMedium,
		ConflictsWith: []string{"old-recovery-guidance"},
	}, "# Recovery Notes\n\nThis page is conflicted but still relevant.")

	result, err := svc.ResolveLoopContext(context.Background(), LoopContextRequest{
		Stage:             LoopContextStageRecovery,
		Goal:              "Recover from missing evidence during grounded execution.",
		CurrentStep:       "refresh knowledge only when evidence is missing",
		PriorFailureHints: []string{"missing evidence"},
		TokenBudget:       240,
	})
	if err != nil {
		t.Fatalf("ResolveLoopContext() error = %v", err)
	}
	if len(result.Snippets) == 0 {
		t.Fatal("expected at least one snippet")
	}
	if result.Snippets[0].Slug != "recovery-evidence-gap-notes" {
		t.Fatalf("first snippet slug = %q, want conflicted recovery page", result.Snippets[0].Slug)
	}
	if !strings.Contains(strings.ToLower(result.Snippets[0].RiskLabel), "conflicted") {
		t.Fatalf("risk label = %q, want conflicted warning", result.Snippets[0].RiskLabel)
	}
	if !strings.Contains(result.Context, "status=conflicted") {
		t.Fatalf("expected context to surface conflicted status, got %q", result.Context)
	}
}

func TestResolveLoopContext_RespectsBudgetAndSkipsLatestWebFirstTasks(t *testing.T) {
	svc := newLoopContextTestService(t)
	writeLoopContextPage(t, svc, KnowledgePageSummary{
		Title:      "Planner Brief",
		Slug:       "planner-brief",
		PageType:   PageTypeSynthesis,
		Summary:    strings.Repeat("planning knowledge ", 20),
		SourceRefs: []string{"docs/planner.md"},
		Keywords:   []string{"planning", "knowledge", "brief"},
		Status:     KnowledgeStatusActive,
		Confidence: KnowledgeConfidenceHigh,
	}, "# Planner Brief\n\nLong summary body.")
	writeLoopContextPage(t, svc, KnowledgePageSummary{
		Title:      "Execution Evidence Pack",
		Slug:       "execution-evidence-pack",
		PageType:   PageTypeDecision,
		Summary:    strings.Repeat("execution evidence ", 20),
		SourceRefs: []string{"docs/execution.md"},
		Keywords:   []string{"execution", "evidence", "knowledge"},
		Status:     KnowledgeStatusActive,
		Confidence: KnowledgeConfidenceHigh,
	}, "# Execution Evidence\n\nLong summary body.")
	writeLoopContextPage(t, svc, KnowledgePageSummary{
		Title:      "Overflow Page",
		Slug:       "overflow-page",
		PageType:   PageTypeConcept,
		Summary:    strings.Repeat("overflow details ", 20),
		SourceRefs: []string{"docs/overflow.md"},
		Keywords:   []string{"knowledge", "details"},
		Status:     KnowledgeStatusActive,
		Confidence: KnowledgeConfidenceHigh,
	}, "# Overflow\n\nLong summary body.")

	result, err := svc.ResolveLoopContext(context.Background(), LoopContextRequest{
		Stage:       LoopContextStageExecution,
		Goal:        "Add a compact execution evidence pack to the active step.",
		CurrentStep: "inject the smallest relevant knowledge block",
		TokenBudget: 120,
	})
	if err != nil {
		t.Fatalf("ResolveLoopContext() error = %v", err)
	}
	if result.UsedCount > 2 {
		t.Fatalf("used count = %d, want at most 2 snippets within budget", result.UsedCount)
	}
	if strings.Contains(result.Context, "overflow-page") {
		t.Fatalf("expected low-priority overflow page to be truncated by budget, got %q", result.Context)
	}

	skipped, err := svc.ResolveLoopContext(context.Background(), LoopContextRequest{
		Stage:       LoopContextStagePlanning,
		Goal:        "Search the latest OpenAI Responses API documentation on the public web.",
		TokenBudget: 120,
	})
	if err != nil {
		t.Fatalf("ResolveLoopContext() latest/live query error = %v", err)
	}
	if skipped.Context != "" {
		t.Fatalf("expected latest/live-web-first task to skip loop knowledge, got %q", skipped.Context)
	}
	if skipped.SkipReason == "" {
		t.Fatal("expected latest/live-web-first task to record a skip reason")
	}
}

func newLoopContextTestService(t *testing.T) *Service {
	t.Helper()
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	return NewService(ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     repoRoot,
		Now: func() time.Time {
			return time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
		},
	})
}

func writeLoopContextPage(t *testing.T, svc *Service, summary KnowledgePageSummary, body string) {
	t.Helper()
	if summary.GeneratedAt.IsZero() {
		summary.GeneratedAt = time.Date(2026, 4, 10, 11, 0, 0, 0, time.UTC)
	}
	if summary.UpdatedAt.IsZero() {
		summary.UpdatedAt = summary.GeneratedAt
	}
	if summary.Status == "" {
		summary.Status = KnowledgeStatusActive
	}
	if summary.Confidence == "" {
		summary.Confidence = KnowledgeConfidenceHigh
	}
	writeKnowledgeTestFile(t, filepath.Join(svc.pagesDir(), summary.Slug+".md"), renderPageDocument(pageDocument{
		Summary: summary,
		Content: body,
	}))
}
