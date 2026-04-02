package agentcore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/contextpack"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

func TestBuildProjectContext_DeterministicOrderForSamePriority(t *testing.T) {
	b := NewSystemPromptBuilder(&Config{})
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

func TestStaticPromptSections_ExposeStabilityAnnotations(t *testing.T) {
	b := NewSystemPromptBuilder(&Config{})

	sections := append([]promptSection{}, b.staticCoreSections()...)
	sections = append(sections, b.staticRuntimeSections()...)
	if len(sections) == 0 {
		t.Fatal("expected static prompt sections")
	}
	for _, section := range sections {
		if section.Stability != promptSectionStable {
			t.Fatalf("section %s stability = %q, want %q", section.Name, section.Stability, promptSectionStable)
		}
		if strings.TrimSpace(section.Reason) == "" {
			t.Fatalf("section %s should record why it stays cache-stable", section.Name)
		}
		if strings.TrimSpace(section.Content) == "" {
			t.Fatalf("section %s should have content", section.Name)
		}
	}
}

func TestConfigPromptSections_ExposeStabilityAnnotations(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(tools.NewSubagentsTool(&config.Config{Agents: *config.DefaultAgentsConfig()}))
	registry.Register(tools.NewAskTool(nil))

	b := NewSystemPromptBuilder(&Config{WorkspaceDir: t.TempDir()})
	b.SetAgentMode(true)
	b.SetToolRegistry(registry)

	sections := b.configSections(map[string]string{workspace.FileUSER: "project context"}, true, true, false)
	if len(sections) == 0 {
		t.Fatal("expected config prompt sections")
	}
	required := map[string]bool{
		"agent_mode":      false,
		"workspace":       false,
		"tools":           false,
		"project_context": false,
	}
	for _, section := range sections {
		if section.Stability != promptSectionConfig {
			t.Fatalf("section %s stability = %q, want %q", section.Name, section.Stability, promptSectionConfig)
		}
		if strings.TrimSpace(section.Reason) == "" {
			t.Fatalf("section %s should record why it belongs in config", section.Name)
		}
		if _, ok := required[section.Name]; ok {
			required[section.Name] = true
		}
	}
	for name, seen := range required {
		if !seen {
			t.Fatalf("expected config sections to include %s, got %#v", name, sections)
		}
	}
}

func TestDynamicPromptSections_KeepLateBoundContentOutOfStableBlocks(t *testing.T) {
	b := NewSystemPromptBuilder(&Config{})

	sections, selection := b.dynamicSections(context.Background(), "<late_bound_hint>retry browser</late_bound_hint>")
	if selection != nil {
		t.Fatal("expected no context pack selection in this test")
	}
	if len(sections) != 2 {
		t.Fatalf("dynamic section count = %d, want 2", len(sections))
	}
	for _, section := range sections {
		if section.Stability != promptSectionDynamic {
			t.Fatalf("section %s stability = %q, want %q", section.Name, section.Stability, promptSectionDynamic)
		}
		if strings.TrimSpace(section.Reason) == "" {
			t.Fatalf("section %s should record why it stays dynamic", section.Name)
		}
	}

	res := b.BuildStructured(context.Background(), "<late_bound_hint>retry browser</late_bound_hint>")
	if !strings.Contains(res.Dynamic, "<now>") || !strings.Contains(res.Dynamic, "<late_bound_hint>retry browser</late_bound_hint>") {
		t.Fatalf("expected runtime info and late-bound prompt in dynamic block, got: %s", res.Dynamic)
	}
	for _, block := range []string{res.Static, res.Config} {
		if strings.Contains(block, "<now>") {
			t.Fatalf("unexpected timestamp in cacheable block: %s", block)
		}
		if strings.Contains(block, "<late_bound_hint>retry browser</late_bound_hint>") {
			t.Fatalf("unexpected late-bound prompt in cacheable block: %s", block)
		}
	}
}

func TestBuildProjectContext_TruncatesOversizedSingleFileToBudget(t *testing.T) {
	b := NewSystemPromptBuilder(&Config{})
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
	b := NewSystemPromptBuilder(&Config{})
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
	b := NewSystemPromptBuilder(&Config{})
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
	b := NewSystemPromptBuilder(&Config{})
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

func TestBuildStructured_ConfigCacheHitAndInvalidation(t *testing.T) {
	workspaceDir := t.TempDir()
	mgr := workspace.NewManager(workspaceDir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("failed to ensure workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, workspace.FileUSER), []byte("user context"), 0o644); err != nil {
		t.Fatalf("write USER.md: %v", err)
	}

	b := NewSystemPromptBuilder(&Config{WorkspaceDir: workspaceDir})
	b.SetWorkspace(mgr)

	first := b.BuildStructured(context.Background(), "")
	if first.Config == "" {
		t.Fatal("expected non-empty config block")
	}
	if b.configCacheHit != 0 {
		t.Fatalf("config cache hits = %d, want 0 after first build", b.configCacheHit)
	}

	second := b.BuildStructured(context.Background(), "")
	if second.Config != first.Config {
		t.Fatal("expected config cache hit to return same config")
	}
	if b.configCacheHit != 1 {
		t.Fatalf("config cache hits = %d, want 1 after second build", b.configCacheHit)
	}

	b.SetAgentMode(true)
	_ = b.BuildStructured(context.Background(), "")
	if b.configCacheHit != 1 {
		t.Fatalf("config cache hits = %d, want unchanged after config invalidation", b.configCacheHit)
	}
}

func TestBuildStructured_ContextPacksStayInDynamicBlock(t *testing.T) {
	workspaceDir := t.TempDir()
	mgr := workspace.NewManager(workspaceDir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("failed to ensure workspace: %v", err)
	}
	packDir := filepath.Join(mgr.ContextDir(), "openai", "docs", "responses-api")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatalf("mkdir packDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "DOC.md"), []byte(`---
id: openai/responses-api
type: doc
description: Responses API notes
source_trust: official
tags: [responses, tools]
---

Use compact tool outputs.`), 0o644); err != nil {
		t.Fatalf("write DOC.md: %v", err)
	}

	registry := contextpack.NewRegistry(mgr.ContextDir())
	registry.SetRefreshTTL(0)
	store, err := contextpack.NewAnnotationStore(filepath.Join(t.TempDir(), "contextpacks.db"))
	if err != nil {
		t.Fatalf("NewAnnotationStore() error = %v", err)
	}
	defer store.Close()
	resolver := contextpack.NewResolver(registry, store, contextpack.ResolverConfig{MaxFiles: 3, MaxTokens: 1200, SearchLimit: 5})

	b := NewSystemPromptBuilder(&Config{WorkspaceDir: workspaceDir})
	b.SetWorkspace(mgr)
	b.SetContextResolver(resolver)

	ctx := contextpack.WithPromptQuery(context.Background(), "responses tools")
	ctx = tools.WithLang(ctx, "en")
	res := b.BuildStructured(ctx, "")
	if res.ContextPacks == nil || len(res.ContextPacks.Files) == 0 {
		t.Fatal("expected context packs in build result")
	}
	if !strings.Contains(res.Dynamic, "<context_pack") {
		t.Fatalf("expected dynamic block to contain context pack, got: %s", res.Dynamic)
	}
	if strings.Contains(res.Config, "<context_pack") {
		t.Fatalf("expected config block to exclude context packs, got: %s", res.Config)
	}
}

func TestBuildAgentModeGuidance_IncludesChecklistAndAskFormat(t *testing.T) {
	b := NewSystemPromptBuilder(&Config{})
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
	if !strings.Contains(out, "Text-input ask format: {\"questions\":[{\"question\":\"...\",\"type\":\"text\"}]}.") {
		t.Fatalf("agent mode guidance should include ask text-format preference: %s", out)
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
	if !strings.Contains(out, "superpowers and ui-ux-pro-max-skill") {
		t.Fatalf("agent mode guidance should include coding-skill bootstrap defaults: %s", out)
	}
	if !strings.Contains(out, "ask the user once and then remember") {
		t.Fatalf("agent mode guidance should require asking and remembering stack preferences: %s", out)
	}
	if !strings.Contains(out, "backend=Go, frontend=React, mobile=React Native, client=Electron") {
		t.Fatalf("agent mode guidance should include default stack preferences: %s", out)
	}
	if !strings.Contains(out, "Python or Node.js") {
		t.Fatalf("agent mode guidance should preserve runtime-environment override guidance: %s", out)
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
	if !strings.Contains(out, "append=true") || !strings.Contains(out, "never send one huge payload") {
		t.Fatalf("agent mode guidance should include chunked write execution guidance: %s", out)
	}
	if !strings.Contains(out, "Ask confirmation before destructive actions") {
		t.Fatalf("agent mode guidance should include manual-confirm clause by default: %s", out)
	}
	if !strings.Contains(out, "asset-loss operations") || !strings.Contains(out, "secondary user confirmation") {
		t.Fatalf("agent mode guidance should require secondary confirmation for asset-loss operations: %s", out)
	}
	if strings.Contains(out, "Auto-confirm enabled — execute without asking.") {
		t.Fatalf("agent mode guidance should not include auto-confirm clause when auto-confirm is disabled: %s", out)
	}
}

func TestBuildAgentModeGuidance_AutoConfirmClause(t *testing.T) {
	b := NewSystemPromptBuilder(&Config{})
	b.SetAgentMode(true)
	b.SetAgentAutoConfirmFunc(func() bool { return true })
	var sb strings.Builder
	b.writeAgentModeGuidanceTo(&sb)
	out := sb.String()
	if !strings.Contains(out, "Auto-confirm enabled — execute without asking.") {
		t.Fatalf("agent mode guidance should include auto-confirm clause when enabled: %s", out)
	}
	if !strings.Contains(out, "asset-loss operations") || !strings.Contains(out, "secondary user confirmation") {
		t.Fatalf("agent mode guidance should require secondary confirmation for asset-loss operations even in auto-confirm mode: %s", out)
	}
	if strings.Contains(out, "Ask confirmation before destructive actions") {
		t.Fatalf("agent mode guidance should not include manual confirmation clause when auto-confirm is enabled: %s", out)
	}
	if !strings.Contains(out, "</agent_mode>") {
		t.Fatalf("agent mode guidance should close agent_mode tag: %s", out)
	}
}

func TestBuildAgentModeGuidance_IncludesCoordinatorAndScratchpadWhenSubagentsAvailable(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(tools.NewSubagentsTool(&config.Config{Agents: *config.DefaultAgentsConfig()}))

	b := NewSystemPromptBuilder(&Config{WorkspaceDir: "/tmp/workspace"})
	b.SetAgentMode(true)
	b.SetToolRegistry(registry)

	var sb strings.Builder
	b.writeAgentModeGuidanceTo(&sb)
	out := sb.String()
	required := []string{
		"delegate bounded independent work",
		"research -> synthesis -> implementation -> verification",
		"Continue the same worker when context overlap is high",
		"Based on your findings, fix the auth bug.",
		"Workers cannot see your conversation",
		".blue/scratchpad/shared",
		"task claims, interim findings, blockers, and worker handoffs",
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Fatalf("agent mode guidance should include %q, got: %s", want, out)
		}
	}
}

func TestWriteToolsInfoTo_IncludesToolUsageRules(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(tools.NewSubagentsTool(&config.Config{Agents: *config.DefaultAgentsConfig()}))
	registry.Register(tools.NewAskTool(nil))

	b := NewSystemPromptBuilder(&Config{})
	b.SetToolRegistry(registry)

	var sb strings.Builder
	if !b.writeToolsInfoTo(&sb, false) {
		t.Fatal("expected tool guidance to be written")
	}
	out := sb.String()
	required := []string{
		"Prefer dedicated tools over exec",
		"parallelize them when the runtime supports it",
		"Use subagents only for bounded independent work",
		"After research, synthesize the findings yourself before delegating follow-up work",
		"Use ask only when key requirements are missing",
		"run independent verification",
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Fatalf("expected tool guidance to include %q, got: %s", want, out)
		}
	}
}

func TestBuildStructured_LoadsWorkspaceContextWhenWorkspaceSet(t *testing.T) {
	workspaceDir := t.TempDir()
	mgr := workspace.NewManager(workspaceDir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("failed to ensure workspace: %v", err)
	}

	b := NewSystemPromptBuilder(&Config{WorkspaceDir: workspaceDir})
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
	b := NewSystemPromptBuilder(&Config{})
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
	b := NewSystemPromptBuilder(&Config{})
	out := b.Build(context.Background(), "")

	required := []string{"<role>", "<instruction_priority>", "<grounding>", "<expressiveness>", "<blue_core_rules>"}
	for _, tag := range required {
		if !strings.Contains(out, tag) {
			t.Fatalf("expected Build output to contain %s, got: %s", tag, out)
		}
	}
}

func TestBuildProjectContext_PrioritizesToolsBetweenAgentsAndIdentity(t *testing.T) {
	b := NewSystemPromptBuilder(&Config{})
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
	b := NewSystemPromptBuilder(&Config{})
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
	b := NewSystemPromptBuilder(&Config{})
	static := b.BuildStructured(context.Background(), "").Static

	if !strings.Contains(static, "Use web_query as the default web tool for both queries and URLs") {
		t.Fatalf("expected static prompt to mention web_query routing, got: %s", static)
	}
	if !strings.Contains(static, "let it discover links, read the best page, retry, and degrade automatically") {
		t.Fatalf("expected static prompt to describe web_query orchestration, got: %s", static)
	}
	if !strings.Contains(static, "retry_browser") {
		t.Fatalf("expected static prompt to mention structured browser fallback codes, got: %s", static)
	}
	if !strings.Contains(static, "Final web fallback is browser") {
		t.Fatalf("expected static prompt to mention final browser fallback, got: %s", static)
	}
	if !strings.Contains(static, "browser_target_id") {
		t.Fatalf("expected static prompt to mention browser_target_id reuse, got: %s", static)
	}
}

func TestBuildStructured_StaticPrefersDeepWikiBeforeGitHubForRepoResearch(t *testing.T) {
	b := NewSystemPromptBuilder(&Config{})
	static := b.BuildStructured(context.Background(), "").Static

	if !strings.Contains(static, "For GitHub repository research, check the corresponding DeepWiki materials first when available") {
		t.Fatalf("expected static prompt to prefer DeepWiki before github.com for repo research, got: %s", static)
	}
	if !strings.Contains(static, "deepwiki.com/&lt;owner&gt;/&lt;repo&gt;") {
		t.Fatalf("expected static prompt to include DeepWiki repo URL pattern, got: %s", static)
	}
	if !strings.Contains(static, "then use GitHub for primary-source verification") {
		t.Fatalf("expected static prompt to preserve GitHub verification guidance, got: %s", static)
	}
}

func TestBuildSkillsSection_UsesWebFetchBrowserRouting(t *testing.T) {
	workspaceDir := t.TempDir()
	b := NewSystemPromptBuilder(&Config{WorkspaceDir: workspaceDir})
	section := b.buildSkillsSection()

	if !strings.Contains(section, "normal web discovery/read→web_query") {
		t.Fatalf("expected skills section to route normal web work to web_query, got: %s", section)
	}
	if !strings.Contains(section, "If web_query reports login_wall, challenge, browser_required, or next_action=retry_browser, switch to browser") {
		t.Fatalf("expected skills section to mention browser fallback on structured web_query warnings, got: %s", section)
	}
	if !strings.Contains(section, "Final web fallback→browser") {
		t.Fatalf("expected skills section to mention final browser fallback, got: %s", section)
	}
	if !strings.Contains(section, "blue exec command='...'") {
		t.Fatalf("expected skills section to mention external CLI execution via blue exec, got: %s", section)
	}
	if !strings.Contains(section, `blue deep_research query="latest memory architecture research"`) {
		t.Fatalf("expected skills section to include explicit deep_research query example, got: %s", section)
	}
	if !strings.Contains(section, "blue media generate") {
		t.Fatalf("expected skills section to mention media subcommand routing, got: %s", section)
	}
	if !strings.Contains(section, "timer`, `datetime`, `unit_converter`, and deprecated `search` are not live runtime skills") {
		t.Fatalf("expected skills section to mention disabled placeholder skills, got: %s", section)
	}
	if !strings.Contains(section, "workspace `.agents/skills/` or `.claude/skills/`") || !strings.Contains(section, "`~/.agents/skills/` and `~/.claude/skills/`") {
		t.Fatalf("expected skills section to mention both workspace and user skill roots, got: %s", section)
	}
}

func TestBuildSkillsSection_WithoutWorkspaceDirStillIncludesPinnedSkills(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	b := NewSystemPromptBuilder(&Config{})
	section := b.buildSkillsSection()

	if !strings.Contains(section, "<skills>") {
		t.Fatalf("expected skills section without workspace dir, got: %s", section)
	}
	if !strings.Contains(section, `name="ask"`) {
		t.Fatalf("expected pinned ask skill without workspace dir, got: %s", section)
	}
}
