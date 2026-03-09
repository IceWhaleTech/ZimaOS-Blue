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

func TestBuildProjectContext_SOULPreservesStructure(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	b.SetMaxContextTokens(0)

	out := b.buildProjectContext(map[string]string{
		workspace.FileSOUL: `# Soul

Be warm and natural.

Use a little emoji when it fits.`,
	})

	if !strings.Contains(out, `# Soul

Be warm and natural.`) {
		t.Fatalf("expected SOUL.md heading structure to be preserved, got: %s", out)
	}
	if !strings.Contains(out, `Be warm and natural.

Use a little emoji when it fits.`) {
		t.Fatalf("expected SOUL.md context to preserve structure, got: %s", out)
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

func TestBuildAgentModeGuidance_IncludesChecklistAndAskFormat(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	b.SetAgentMode(true)
	var sb strings.Builder
	b.writeAgentModeGuidanceTo(&sb)
	out := sb.String()
	if !strings.Contains(out, "FIRST output a TODO checklist using markdown checkboxes") {
		t.Fatalf("agent mode guidance should require markdown TODO planning first: %s", out)
	}
	if !strings.Contains(out, "injects `<tp>` with current task") || !strings.Contains(out, "Do NOT re-output the checklist") {
		t.Fatalf("agent mode guidance should include tp progress signal and single-checklist rule: %s", out)
	}
	if !strings.Contains(out, "Prefer ask format: {\"questions\":[{\"question\":\"...\",\"type\":\"radio\",\"options\":[...]}]}.") {
		t.Fatalf("agent mode guidance should include ask questions-format preference: %s", out)
	}
	if !strings.Contains(out, "Single-question shorthand: use \"q\" for single-select or \"mq\" for multi-select") || !strings.Contains(out, "\"a\" as the options array (2-4 strings)") {
		t.Fatalf("agent mode guidance should include ask q/mq+a contract: %s", out)
	}
	if !strings.Contains(out, "Example: {\"q\":\"Which approach?\",\"a\":[\"Option A\",\"Option B\"]}") {
		t.Fatalf("agent mode guidance should include ask example payload: %s", out)
	}
	if strings.Contains(out, "<orchestrator_fsm>") || strings.Contains(out, "<protocol>") {
		t.Fatalf("agent mode guidance should not include legacy fsm/protocol sections: %s", out)
	}
	if strings.Contains(out, "<agent_loop>") || strings.Contains(out, "<awaiting_user_input>true</awaiting_user_input>") {
		t.Fatalf("agent mode guidance should not include legacy loop/awaiting marker guidance: %s", out)
	}
	if !strings.Contains(out, "After all steps, verify: run build/tests. Fix and re-verify if needed.") {
		t.Fatalf("agent mode guidance should include verification requirement: %s", out)
	}
	if !strings.Contains(out, "Your LAST response MUST be plain text") {
		t.Fatalf("agent mode guidance should include completion format requirement: %s", out)
	}
	if strings.Contains(out, "next concrete improvement") || strings.Contains(out, "explicitly asks to stop") {
		t.Fatalf("agent mode guidance should not include legacy continuous loop stop-condition: %s", out)
	}
	if strings.Contains(out, "plan_create") || strings.Contains(out, "plan_update") || strings.Contains(out, "plan_append") {
		t.Fatalf("agent mode guidance should not mention plan IPC commands by default: %s", out)
	}
	if !strings.Contains(out, "<planning>") || !strings.Contains(out, "<execution>") || !strings.Contains(out, "<completion>") {
		t.Fatalf("agent mode guidance should preserve planning/execution/completion tags: %s", out)
	}
	if !strings.Contains(out, "<verification>") {
		t.Fatalf("agent mode guidance should preserve verification tag: %s", out)
	}
	if !strings.Contains(out, "</agent_mode>") {
		t.Fatalf("agent mode guidance should close agent_mode tag: %s", out)
	}
	if !strings.Contains(out, "No tool round limit") {
		t.Fatalf("agent mode guidance should keep unlimited autonomy preamble: %s", out)
	}
	if !strings.Contains(out, "Do NOT call exec without a concrete command") {
		t.Fatalf("agent mode guidance should enforce concrete exec command requirement: %s", out)
	}
	if !strings.Contains(out, "Ask confirmation before destructive actions") {
		t.Fatalf("agent mode guidance should include manual-confirm clause by default: %s", out)
	}
	if strings.Contains(out, "Auto-confirm enabled — execute without asking.") {
		t.Fatalf("agent mode guidance should not include auto-confirm clause when auto-confirm is disabled: %s", out)
	}
}

func TestBuildAgentModeGuidance_AutoConfirmClause(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	b.SetAgentMode(true)
	b.SetAgentAutoConfirmFunc(func() bool { return true })
	var sb strings.Builder
	b.writeAgentModeGuidanceTo(&sb)
	out := sb.String()
	if !strings.Contains(out, "Auto-confirm enabled — execute without asking.") {
		t.Fatalf("agent mode guidance should include auto-confirm clause when enabled: %s", out)
	}
	if strings.Contains(out, "Ask confirmation before destructive actions") {
		t.Fatalf("agent mode guidance should not include manual confirmation clause when auto-confirm is enabled: %s", out)
	}
	if !strings.Contains(out, "</agent_mode>") {
		t.Fatalf("agent mode guidance should close agent_mode tag: %s", out)
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

	required := []string{"<role>", "<instruction_priority>", "<grounding>", "<expressiveness>", "<blue_core_rules>"}
	for _, tag := range required {
		if !strings.Contains(static, tag) {
			t.Fatalf("expected static prompt to contain %s, got: %s", tag, static)
		}
	}
	if !strings.Contains(static, "untrusted content") {
		t.Fatalf("expected static prompt to define untrusted content boundary, got: %s", static)
	}
	if !strings.Contains(static, "Occasional natural emoji and light expressive formatting are welcome") {
		t.Fatalf("expected static prompt to include written emoji guidance, got: %s", static)
	}
}

func TestBuild_IncludesPriorityAndGroundingGuidance(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	out := b.Build(context.Background(), "")

	required := []string{"<role>", "<instruction_priority>", "<grounding>", "<expressiveness>", "<blue_core_rules>"}
	for _, tag := range required {
		if !strings.Contains(out, tag) {
			t.Fatalf("expected Build output to contain %s, got: %s", tag, out)
		}
	}
}

func TestBuildProjectContext_PrioritizesToolsBetweenAgentsAndIdentity(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	b.SetMaxContextTokens(0)

	out := b.buildProjectContext(map[string]string{
		workspace.FileAGENTS:   "agents",
		workspace.FileTOOLS:    "tools",
		workspace.FileIDENTITY: "identity",
	})

	agentsIdx := strings.Index(out, `<file name="AGENTS.md">`)
	toolsIdx := strings.Index(out, `<file name="TOOLS.md">`)
	identityIdx := strings.Index(out, `<file name="IDENTITY.md">`)
	if agentsIdx < 0 || toolsIdx < 0 || identityIdx < 0 {
		t.Fatalf("expected AGENTS/TOOLS/IDENTITY in output, got: %s", out)
	}
	if !(agentsIdx < toolsIdx && toolsIdx < identityIdx) {
		t.Fatalf("expected AGENTS -> TOOLS -> IDENTITY order, got: %s", out)
	}
}

func TestBuildProjectContext_ToolsSoftCapApplied(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	b.SetMaxContextTokens(0)

	huge := strings.Repeat("tools guidance sentence for cap testing.\n", 1200)
	out := b.buildProjectContext(map[string]string{
		workspace.FileTOOLS: huge,
	})
	if out == "" {
		t.Fatal("expected TOOLS.md context to be included")
	}
	stats := b.LastContextStats()
	if stats == nil {
		t.Fatal("expected context stats")
	}
	if stats.TotalTokens <= 0 || stats.TotalTokens > 600 {
		t.Fatalf("TOOLS.md soft cap not applied, total tokens = %d", stats.TotalTokens)
	}
}

func TestBuildStructured_StaticIncludesWebToolRoutingGuidance(t *testing.T) {
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{})
	static := b.BuildStructured(context.Background(), "").Static

	if !strings.Contains(static, "Use web_fetch for lightweight public HTTP page reads") {
		t.Fatalf("expected static prompt to mention web_fetch routing, got: %s", static)
	}
	if !strings.Contains(static, "warning_code=login_wall, challenge, or browser_required") {
		t.Fatalf("expected static prompt to mention structured browser fallback codes, got: %s", static)
	}
	if !strings.Contains(static, "browser_target_id") {
		t.Fatalf("expected static prompt to mention browser_target_id reuse, got: %s", static)
	}
}

func TestBuildSkillsSection_UsesWebFetchBrowserRouting(t *testing.T) {
	workspaceDir := t.TempDir()
	b := NewSystemPromptBuilder(&ClaudeCodeConfig{WorkspaceDir: workspaceDir})
	section := b.buildSkillsSection()

	if !strings.Contains(section, "public URL read→web_fetch, interactive/login URL→browser") {
		t.Fatalf("expected skills section to route between web_fetch and browser, got: %s", section)
	}
	if !strings.Contains(section, "If web_fetch returns warning_code=login_wall, challenge, or browser_required, switch to browser") {
		t.Fatalf("expected skills section to mention browser fallback on structured warning code, got: %s", section)
	}
}
