package claudecode

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

func TestBuildProjectContext_DeterministicOrderForSamePriority(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	b.SetMaxContextTokens(0) // unlimited

	files := map[string]string{
		"USER.md":      "user",
		"BOOTSTRAP.md": "bootstrap",
	}

	first := b.buildProjectContext(files)
	for i := 0; i < 40; i++ {
		got := b.buildProjectContext(files)
		if got != first {
			t.Fatalf("non-deterministic output at iter %d", i)
		}
	}

	bootIdx := strings.Index(first, `<file name="BOOTSTRAP.md">`)
	userIdx := strings.Index(first, `<file name="USER.md">`)
	if bootIdx < 0 || userIdx < 0 {
		t.Fatalf("expected both files in output: %s", first)
	}
	if bootIdx > userIdx {
		t.Fatalf("expected BOOTSTRAP.md before USER.md, got: %s", first)
	}
}

func TestBuildProjectContext_TruncatesOversizedSingleFileToBudget(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	b.SetMaxContextTokens(120)

	huge := strings.Repeat("This is a very long memory sentence with useful details.\n", 1000)
	out := b.buildProjectContext(map[string]string{
		"SOUL.md": huge,
	})
	if out == "" {
		t.Fatal("expected non-empty project context")
	}

	stats := b.LastContextStats()
	if stats == nil {
		t.Fatal("expected context stats")
	}
	if stats.TotalTokens <= 0 || stats.TotalTokens > 120 {
		t.Fatalf("total tokens = %d, want within (0, 120]", stats.TotalTokens)
	}
	if !stats.Trimmed {
		t.Fatal("expected trimmed=true for oversized content")
	}
}

func TestTruncateToTokenBudget(t *testing.T) {
	content := strings.Repeat("alpha beta gamma delta epsilon\n", 200)
	truncated, tokens, trimmed := truncateToTokenBudget(content, 80)
	if !trimmed {
		t.Fatal("expected trimmed=true")
	}
	if tokens <= 0 || tokens > 80 {
		t.Fatalf("tokens = %d, want within (0,80]", tokens)
	}
	if truncated == "" {
		t.Fatal("expected non-empty truncated content")
	}
}

func TestBuildProjectContext_NormalizeDefaultWorkspaceTemplateToEnglish(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	b.SetMaxContextTokens(0)

	zhSoulBytes, err := os.ReadFile("../workspace/templates/zh/SOUL.md")
	if err != nil {
		t.Fatalf("read zh template: %v", err)
	}
	out := b.buildProjectContext(map[string]string{
		workspace.FileSOUL: string(zhSoulBytes),
	})

	if !strings.Contains(out, "ZimaOS AI Assistant") {
		t.Fatalf("expected english template text in project context, got: %s", out)
	}
	if strings.Contains(out, "ZimaOS AI 助手") {
		t.Fatalf("expected non-english default template content to be normalized: %s", out)
	}
}

func TestBuildProjectContext_CacheHitAndInvalidation(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	b.SetMaxContextTokens(256)

	files := map[string]string{
		"USER.md": "user context",
		"SOUL.md": "soul context",
	}
	out1 := b.buildProjectContext(files)
	if out1 == "" {
		t.Fatal("expected non-empty output")
	}
	if b.projectContextCacheHit != 0 {
		t.Fatalf("cache hits = %d, want 0 after first build", b.projectContextCacheHit)
	}

	out2 := b.buildProjectContext(files)
	if out2 != out1 {
		t.Fatal("expected same output on cache hit")
	}
	if b.projectContextCacheHit != 1 {
		t.Fatalf("cache hits = %d, want 1 after second build", b.projectContextCacheHit)
	}

	changed := map[string]string{
		"USER.md": "user context changed",
		"SOUL.md": "soul context",
	}
	_ = b.buildProjectContext(changed)
	if b.projectContextCacheHit != 1 {
		t.Fatalf("cache hits = %d, want unchanged after cache miss", b.projectContextCacheHit)
	}
}

func TestBuildAgentModeGuidance_IncludesFSMAndAskGateProtocol(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	b.SetAgentMode(true)
	var sb strings.Builder
	b.writeAgentModeGuidanceTo(&sb)
	out := sb.String()
	if !strings.Contains(out, "<orchestrator_fsm>") {
		t.Fatalf("agent mode guidance should include orchestrator_fsm tag: %s", out)
	}
	if !strings.Contains(out, "<protocol>") {
		t.Fatalf("agent mode guidance should include protocol tag: %s", out)
	}
	if !strings.Contains(out, "exactly one canonical Markdown TODO checklist") {
		t.Fatalf("agent mode guidance should force one canonical TODO checklist: %s", out)
	}
	if !strings.Contains(out, "default to status-only updates") || !strings.Contains(out, "Do not re-output duplicate TODO blocks") {
		t.Fatalf("agent mode guidance should enforce stable checklist updates without recreation: %s", out)
	}
	if !strings.Contains(out, "FIRST output a Markdown TODO checklist") || !strings.Contains(out, "may inject `<tp>` progress hints") {
		t.Fatalf("agent mode guidance should enforce markdown-first TODO planning with progress hints: %s", out)
	}
	if strings.Contains(out, "plan_create") || strings.Contains(out, "plan_update") || strings.Contains(out, "plan_append") {
		t.Fatalf("agent mode guidance should not mention plan IPC commands by default: %s", out)
	}
	if !strings.Contains(out, "<awaiting_user_input>true</awaiting_user_input>") {
		t.Fatalf("agent mode guidance should include awaiting_user_input marker contract: %s", out)
	}
	if !strings.Contains(out, "next concrete improvement") || !strings.Contains(out, "explicitly asks to stop") {
		t.Fatalf("agent mode guidance should include continuous loop stop-condition: %s", out)
	}
}

func TestBuildStructured_LoadsWorkspaceContextWhenWorkspaceSet(t *testing.T) {
	workspaceDir := t.TempDir()
	mgr := workspace.NewManager(workspaceDir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("failed to ensure workspace: %v", err)
	}

	b := NewSystemPromptBuilder(&ClaudeCodeConfig{WorkspaceDir: workspaceDir})
	b.SetWorkspace(mgr)
	result := b.BuildStructured(context.Background(), "")
	out := result.Config

	if !strings.Contains(out, "<project_context>") {
		t.Fatalf("expected BuildStructured config to include <project_context>, got: %s", out)
	}
	if !strings.Contains(out, `<file name="SOUL.md">`) {
		t.Fatalf("expected BuildStructured config to include SOUL.md section, got: %s", out)
	}
}

func TestBuildStructured_StaticIncludesPriorityAndGrounding(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	static := b.BuildStructured(context.Background(), "").Static

	required := []string{"<role>", "<instruction_priority>", "<grounding>"}
	for _, tag := range required {
		if !strings.Contains(static, tag) {
			t.Fatalf("expected static prompt to contain %s, got: %s", tag, static)
		}
	}
	if !strings.Contains(static, "untrusted content") {
		t.Fatalf("expected static prompt to define untrusted content boundary, got: %s", static)
	}
}

func TestBuild_IncludesPriorityAndGroundingGuidance(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	out := b.Build(context.Background(), "")

	required := []string{"<role>", "<instruction_priority>", "<grounding>"}
	for _, tag := range required {
		if !strings.Contains(out, tag) {
			t.Fatalf("expected Build output to contain %s, got: %s", tag, out)
		}
	}
}
