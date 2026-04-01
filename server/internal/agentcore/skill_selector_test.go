package agentcore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	sel "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selector"
)

func writeSelectorSkill(t *testing.T, workspaceDir, id, desc, invocation string, capabilityTags ...string) {
	t.Helper()
	writeSelectorSkillWithFrontmatter(t, workspaceDir, id, id, desc, invocation, "", capabilityTags...)
}

func writeSelectorSkillWithFrontmatter(t *testing.T, workspaceDir, dirName, manifestName, desc, invocation, extraFrontmatter string, capabilityTags ...string) {
	t.Helper()

	dir := filepath.Join(workspaceDir, ".claude", "skills", dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}

	var md strings.Builder
	md.WriteString("---\n")
	md.WriteString(fmt.Sprintf("name: %s\n", manifestName))
	md.WriteString("version: \"1.0.0\"\n")
	md.WriteString(fmt.Sprintf("description: %q\n", desc))
	if strings.TrimSpace(extraFrontmatter) != "" {
		md.WriteString(strings.TrimRight(extraFrontmatter, "\n"))
		md.WriteString("\n")
	}
	if invocation != "" {
		md.WriteString(fmt.Sprintf("invocation: %q\n", invocation))
		md.WriteString("examples:\n")
		md.WriteString(fmt.Sprintf("  - %q\n", invocation))
	}
	if len(capabilityTags) > 0 {
		md.WriteString("capability_tags:\n")
		for _, tag := range capabilityTags {
			md.WriteString(fmt.Sprintf("  - %s\n", tag))
		}
	}
	md.WriteString("interaction_mode: stateless\n")
	md.WriteString("card_support: none\n")
	md.WriteString(fmt.Sprintf("os: [%q]\n", runtime.GOOS))
	md.WriteString("---\n")
	md.WriteString("# ")
	md.WriteString(manifestName)
	md.WriteString("\n")

	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md.String()), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func TestBuildSkillIndex_WorkspaceOverridesHome(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	mk := func(base, id, desc string) {
		dir := filepath.Join(base, ".claude", "skills", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir skill: %v", err)
		}
		content := "---\nname: " + id + "\ndescription: " + desc + "\nos: [\"" + runtime.GOOS + "\"]\n---\n# " + id + "\n\nblue " + id + " query=hi\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}

	mk(homeDir, "browser", "home browser")
	mk(workspaceDir, "browser", "workspace browser")

	docs, err := BuildSkillIndex(workspaceDir)
	if err != nil {
		t.Fatalf("BuildSkillIndex error: %v", err)
	}
	if len(docs) == 0 {
		t.Fatalf("expected docs")
	}
	if docs[0].Name != "browser" {
		t.Fatalf("expected browser doc, got %q", docs[0].Name)
	}
	if docs[0].Description != "workspace browser" {
		t.Fatalf("expected workspace override, got %q", docs[0].Description)
	}
}

func TestBuildSkillIndex_AgentsRootOverridesClaudeRoot(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	mk := func(base, rootDir, fileName, id, desc string) {
		dir := filepath.Join(base, rootDir, "skills", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir skill: %v", err)
		}
		content := "---\nname: " + id + "\ndescription: " + desc + "\nos: [\"" + runtime.GOOS + "\"]\n---\n# " + id + "\n"
		if err := os.WriteFile(filepath.Join(dir, fileName), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}

	mk(workspaceDir, ".claude", "SKILL.md", "browser", "workspace claude browser")
	mk(workspaceDir, ".agents", "CLAUDE.md", "browser", "workspace agents browser")

	docs, err := BuildSkillIndex(workspaceDir)
	if err != nil {
		t.Fatalf("BuildSkillIndex error: %v", err)
	}
	if len(docs) == 0 {
		t.Fatalf("expected docs")
	}
	if docs[0].Description != "workspace agents browser" {
		t.Fatalf("expected agents root override, got %q", docs[0].Description)
	}
}

func TestSkillSelector_Select_IR(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	mk := func(id, desc string) {
		dir := filepath.Join(workspaceDir, ".claude", "skills", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir skill: %v", err)
		}
		content := "---\nname: " + id + "\ndescription: " + desc + "\nos: [\"" + runtime.GOOS + "\"]\n---\n# " + id + "\n\nblue " + id + " query=hello\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}
	mk("web_query", "search the web")
	mk("browser", "browse urls")

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "请帮我搜索最新新闻", SelectOptions{Mode: SkillSelectorModeHybrid, EnableRerank: true, ConfidenceThreshold: 0.78})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill == "" {
		t.Fatalf("expected selected skill")
	}
	if decision.SelectedSkill != "web_query" {
		t.Fatalf("expected web_query, got %q", decision.SelectedSkill)
	}
}

func TestSkillSelector_LowConfidenceNeedsClarify(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	for _, id := range []string{"browser", "web_query", "analyze"} {
		dir := filepath.Join(workspaceDir, ".claude", "skills", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir skill: %v", err)
		}
		content := "---\nname: " + id + "\ndescription: generic helper\nos: [\"" + runtime.GOOS + "\"]\n---\n# " + id + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "帮我处理这个", SelectOptions{Mode: SkillSelectorModeHybrid, EnableRerank: true, ConfidenceThreshold: 0.95})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if !decision.NeedClarify {
		t.Fatalf("expected need_clarify=true, got false")
	}
}

func TestSkillSelector_LatestDocsRouteToWebQuery(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSelectorSkill(t, workspaceDir, "web_query", "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs")
	writeSelectorSkill(t, workspaceDir, "browser", "browse urls and interact with web pages", "blue browser.navigate url=https://example.com", "browser", "web")
	writeSelectorSkill(t, workspaceDir, "analyze", "analyze local reports and project files", `blue analyze topic="project report" --json`, "analysis", "report")

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "最新 OpenAI Responses API 文档", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill != "web_query" {
		t.Fatalf("expected web_query, got=%+v", decision)
	}
	if decision.NeedClarify {
		t.Fatalf("latest docs query should not need clarify, got=%+v", decision)
	}
}

func TestSkillSelector_ModelInvocableFalseIsHiddenFromSelector(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSelectorSkillWithFrontmatter(
		t,
		workspaceDir,
		"secret_docs",
		"secret_docs",
		"search the web for latest docs and official references",
		`blue secret_docs query="OpenAI Responses API docs"`,
		"model_invocable: false",
		"search", "web", "docs",
	)
	writeSelectorSkill(
		t,
		workspaceDir,
		"web_query",
		"search the web for docs and references",
		`blue web_query input="OpenAI Responses API docs"`,
		"search", "web", "docs",
	)

	selector := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := selector.Select(context.Background(), "最新 OpenAI Responses API 文档", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill != "web_query" {
		t.Fatalf("expected web_query, got=%+v", decision)
	}

	view, ok, err := selector.LookupSkillRuntimeView("secret_docs")
	if err != nil {
		t.Fatalf("LookupSkillRuntimeView error: %v", err)
	}
	if !ok {
		t.Fatal("expected runtime view for secret_docs")
	}
	if view.ModelInvocable {
		t.Fatalf("expected secret_docs model_invocable=false, got %+v", view)
	}
}

func TestSkillSelector_DynamicExposureActivatesConditionalSkillAndInvalidatesCache(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSelectorSkillWithFrontmatter(
		t,
		workspaceDir,
		"pkg_helper",
		"pkg_helper",
		"inspect Go packages under pkg",
		`blue pkg_helper target="pkg"`,
		"paths:\n  - pkg/**",
		"pkg", "go", "package",
	)

	selector := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	selector.SetDynamicExposureEnabledFunc(func() bool { return true })

	beforeDebug, err := selector.DebugState()
	if err != nil {
		t.Fatalf("DebugState before activation: %v", err)
	}
	beforeView, ok, err := selector.LookupSkillRuntimeView("pkg_helper")
	if err != nil {
		t.Fatalf("LookupSkillRuntimeView before activation: %v", err)
	}
	if !ok {
		t.Fatal("expected runtime view for pkg_helper before activation")
	}
	if beforeView.ActivationState != "dormant" || beforeView.ActivationSource != "conditional" {
		t.Fatalf("unexpected runtime view before activation: %+v", beforeView)
	}

	selector.ExposureManager().ObserveToolPath("read", filepath.Join(workspaceDir, "pkg", "main.go"))

	afterDebug, err := selector.DebugState()
	if err != nil {
		t.Fatalf("DebugState after activation: %v", err)
	}
	if afterDebug.CacheInvalidationCount <= beforeDebug.CacheInvalidationCount {
		t.Fatalf("expected cache invalidation count to increase, before=%d after=%d", beforeDebug.CacheInvalidationCount, afterDebug.CacheInvalidationCount)
	}

	view, ok, err := selector.LookupSkillRuntimeView("pkg_helper")
	if err != nil {
		t.Fatalf("LookupSkillRuntimeView error: %v", err)
	}
	if !ok {
		t.Fatal("expected runtime view for pkg_helper")
	}
	if view.ActivationState != "active" || view.ActivationSource != "conditional" {
		t.Fatalf("unexpected runtime view after activation: %+v", view)
	}
	foundActivated := false
	for _, id := range afterDebug.ActivatedConditionalSkills {
		if id == "pkg_helper" {
			foundActivated = true
			break
		}
	}
	if !foundActivated {
		t.Fatalf("expected pkg_helper in activated conditional skills, got %v", afterDebug.ActivatedConditionalSkills)
	}
}

func TestBuildSkillIndex_StructuredFields(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	dir := filepath.Join(workspaceDir, ".claude", "skills", "youtube_video_analyzer")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	content := `---
name: youtube_video_analyzer
description: Analyze YouTube videos
tags: ["youtube","transcript"]
category: media
environment: ["python3"]
os: ["` + runtime.GOOS + `"]
---

# YouTube Video Analyzer Skill

## Setup

No external dependencies required.

## Available Scripts

| Script | Purpose |
|--------|---------|
| scripts/fetch_transcript.py | Download transcripts |
| scripts/fetch_comments.py | Download comments |

## Task Routing

| User Intent | Action |
|-------------|--------|
| Download subtitle file | Run scripts/fetch_transcript.py <video> |

## Script Usage

` + "```bash" + `
python scripts/fetch_transcript.py <video_id_or_url> --save
` + "```" + `
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	docs, err := BuildSkillIndex(workspaceDir)
	if err != nil {
		t.Fatalf("BuildSkillIndex error: %v", err)
	}
	doc, ok := findSkillDocByName(docs, "youtube_video_analyzer")
	if !ok {
		t.Fatalf("expected youtube_video_analyzer in skill index, got %v", docs)
	}
	if len(doc.ScriptPaths) < 2 {
		t.Fatalf("expected parsed script paths, got %v", doc.ScriptPaths)
	}
	if len(doc.TaskRoutes) != 1 {
		t.Fatalf("expected parsed task routes, got %v", doc.TaskRoutes)
	}
	if len(doc.UsageSteps) != 1 {
		t.Fatalf("expected parsed usage steps, got %v", doc.UsageSteps)
	}
	if doc.Example == "" {
		t.Fatalf("expected example from usage command")
	}
	if len(doc.Environment) != 1 || doc.Environment[0] != "python3" {
		t.Fatalf("expected environment parsed, got %v", doc.Environment)
	}
	irText := strings.ToLower(skillDocTextForIR(doc))
	if !strings.Contains(irText, "scripts/fetch_transcript.py") {
		t.Fatalf("expected script path in IR text, got %q", irText)
	}
	if !strings.Contains(irText, "download subtitle file") {
		t.Fatalf("expected task route intent in IR text, got %q", irText)
	}
}

func TestStage0RuleRoute_DoesNotAutoRoutePlanForGenericTodoWords(t *testing.T) {
	d := stage0RuleRoute("请先给一个 todo checklist，然后开始执行")
	if d.SelectedSkill == "plan_create" {
		t.Fatalf("expected generic todo/checklist words not to auto-route plan_create, got=%+v", d)
	}

	d = stage0RuleRoute("先做个计划再执行")
	if d.SelectedSkill == "plan_create" {
		t.Fatalf("expected generic planning words not to auto-route plan_create, got=%+v", d)
	}
}

func TestStage0RuleRoute_RoutesPlanOnlyForExplicitPlanCommands(t *testing.T) {
	d := stage0RuleRoute("请用 plan_create 初始化任务")
	if d.SelectedSkill != "plan_create" {
		t.Fatalf("expected explicit plan_create command to route plan_create, got=%+v", d)
	}

	d = stage0RuleRoute("用 plan_update 更新第2项状态")
	if d.SelectedSkill != "plan_create" {
		t.Fatalf("expected explicit plan_update command to route plan_create family, got=%+v", d)
	}
}

func TestStage0RuleRoute_RoutesWorkspaceQueryToExec(t *testing.T) {
	query := "Review files in the workspace and summarize the report."
	d := stage0RuleRoute(query)
	if d.SelectedSkill != "exec" {
		t.Fatalf("expected workspace local rule to route exec, got=%+v signals=%+v", d, sel.AnalyzeQuery(query))
	}
}

func TestStage0RuleRoute_RoutesEmailCLIQuery(t *testing.T) {
	query := "Search my IMAP inbox for unread mail from Alice and reply from the terminal."
	d := stage0RuleRoute(query)
	if d.SelectedSkill != "himalaya" {
		t.Fatalf("expected email CLI rule to route himalaya, got=%+v signals=%+v", d, sel.AnalyzeQuery(query))
	}
}

func TestSkillSelector_DefinitionQueryDoesNotAutoRoute(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	for _, tc := range []struct {
		id   string
		desc string
	}{
		{id: "web_query", desc: "search the web"},
		{id: "deep_research", desc: "research with citations"},
	} {
		dir := filepath.Join(workspaceDir, ".claude", "skills", tc.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir skill: %v", err)
		}
		content := "---\nname: " + tc.id + "\ndescription: " + tc.desc + "\nos: [\"" + runtime.GOOS + "\"]\n---\n# " + tc.id + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "What is deep research?", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill != "" {
		t.Fatalf("definition query should not auto-route a skill, got=%+v", decision)
	}
}

func TestSkillSelector_WorkspaceQueryDoesNotRouteWebQuery(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	for _, tc := range []struct {
		id   string
		desc string
	}{
		{id: "web_query", desc: "search the web"},
		{id: "analyze", desc: "analyze reports and urls"},
	} {
		dir := filepath.Join(workspaceDir, ".claude", "skills", tc.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir skill: %v", err)
		}
		content := "---\nname: " + tc.id + "\ndescription: " + tc.desc + "\nos: [\"" + runtime.GOOS + "\"]\n---\n# " + tc.id + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "Review files in the workspace and summarize the report.", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill == "web_query" {
		t.Fatalf("workspace file task should not route to web_query, got=%+v", decision)
	}
	if decision.SelectedSkill != "exec" {
		t.Fatalf("expected exec for workspace file task, got=%+v", decision)
	}
}

func TestSkillSelector_WorkspaceREADMEQueryDoesNotRouteWebQuery(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSelectorSkill(t, workspaceDir, "web_query", "search the web for latest docs and references", `blue web_query input="latest docs"`, "search", "web")
	writeSelectorSkill(t, workspaceDir, "analyze", "analyze reports and urls", `blue analyze topic="url report" --json`, "analysis", "report", "url")

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "看下 workspace 里的 README，顺手总结一下项目在做什么。", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill == "web_query" {
		t.Fatalf("workspace README query should not route to web_query, got=%+v", decision)
	}
	if decision.SelectedSkill != "exec" {
		t.Fatalf("expected exec for workspace README query, got=%+v", decision)
	}
}

func TestSkillSelector_ReminderIntentRoutesReminder(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSelectorSkill(t, workspaceDir, "reminder", "schedule reminders and user notifications at a specific time", `blue reminder add message="Standup" time="2026-03-01 09:00"`, "reminder", "notify", "schedule")
	writeSelectorSkill(t, workspaceDir, "scheduler", "manage recurring cron jobs and automation schedules", `blue cron.create name=uptime_check schedule="*/10 * * * *" command="uptime"`, "cron", "automation")
	writeSelectorSkill(t, workspaceDir, "web_query", "search the web for docs and references", `blue web_query input="reminder app docs"`, "search", "web")

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "帮我明早 9 点提醒我交周报", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill != "reminder" {
		t.Fatalf("expected reminder, got=%+v", decision)
	}
	if decision.NeedClarify {
		t.Fatalf("reminder request should not need clarify, got=%+v", decision)
	}
}

func TestSkillSelector_MixedLocalAndWebIntentNeedsClarify(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSelectorSkill(t, workspaceDir, "web_query", "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs")
	writeSelectorSkill(t, workspaceDir, "analyze", "analyze reports and urls", `blue analyze topic="url report" --json`, "analysis", "report", "url")

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.90,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if !decision.NeedClarify {
		t.Fatalf("mixed local/web request should need clarify, got=%+v", decision)
	}
}

func TestSkillSelector_RoutesHimalayaForRealEmailCLIQueries(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	for _, tc := range []struct {
		id   string
		desc string
	}{
		{id: "himalaya", desc: "real email cli for imap and smtp inbox workflows"},
		{id: "analyze", desc: "analyze reports and urls"},
		{id: "web_query", desc: "search the web"},
	} {
		dir := filepath.Join(workspaceDir, ".claude", "skills", tc.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir skill: %v", err)
		}
		content := "---\nname: " + tc.id + "\ndescription: " + tc.desc + "\nos: [\"" + runtime.GOOS + "\"]\n---\n# " + tc.id + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "Search my IMAP inbox for unread mail from Alice and reply from the terminal.", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill != "himalaya" {
		t.Fatalf("expected himalaya, got=%+v", decision)
	}
}

func TestSkillSelector_UIReviewerNeedsUIEvidence(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	for _, tc := range []struct {
		id   string
		desc string
	}{
		{id: "ui_reviewer", desc: "review screenshots and layouts"},
		{id: "analyze", desc: "analyze reports"},
	} {
		dir := filepath.Join(workspaceDir, ".claude", "skills", tc.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir skill: %v", err)
		}
		content := "---\nname: " + tc.id + "\ndescription: " + tc.desc + "\nos: [\"" + runtime.GOOS + "\"]\n---\n# " + tc.id + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "Review the report in docs/ and summarize it.", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill == "ui_reviewer" {
		t.Fatalf("ui_reviewer should not trigger without UI evidence, got=%+v", decision)
	}

	decision, err = sel.Select(context.Background(), "Review this screenshot and audit the UI layout.", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill != "ui_reviewer" {
		t.Fatalf("expected ui_reviewer with screenshot/UI evidence, got=%+v", decision)
	}
}

func TestSkillSelector_URLUIReviewBypassesBrowserRule(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	for _, tc := range []struct {
		id   string
		desc string
	}{
		{id: "browser", desc: "browse urls"},
		{id: "ui_reviewer", desc: "review screenshots and layouts"},
	} {
		dir := filepath.Join(workspaceDir, ".claude", "skills", tc.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir skill: %v", err)
		}
		content := "---\nname: " + tc.id + "\ndescription: " + tc.desc + "\nos: [\"" + runtime.GOOS + "\"]\n---\n# " + tc.id + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "Please audit the accessibility and UI of https://example.com/pricing", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill != "ui_reviewer" {
		t.Fatalf("expected ui_reviewer for URL UI audit, got=%+v", decision)
	}
}

func TestSkillSelector_URLAnalyzeBypassesBrowserRule(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	for _, tc := range []struct {
		id   string
		desc string
	}{
		{id: "browser", desc: "browse urls"},
		{id: "analyze", desc: "analyze reports and urls"},
	} {
		dir := filepath.Join(workspaceDir, ".claude", "skills", tc.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir skill: %v", err)
		}
		content := "---\nname: " + tc.id + "\ndescription: " + tc.desc + "\nos: [\"" + runtime.GOOS + "\"]\n---\n# " + tc.id + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "Analyze https://example.com/blog and summarize the key findings into a short report.", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill != "analyze" {
		t.Fatalf("expected analyze for URL analysis request, got=%+v", decision)
	}
}

func TestSkillSelector_URLDeepResearchBypassesBrowserRule(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	for _, tc := range []struct {
		id   string
		desc string
	}{
		{id: "browser", desc: "browse urls"},
		{id: "analyze", desc: "analyze reports and urls"},
		{id: "deep_research", desc: "research with citations and evidence"},
	} {
		dir := filepath.Join(workspaceDir, ".claude", "skills", tc.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir skill: %v", err)
		}
		content := "---\nname: " + tc.id + "\ndescription: " + tc.desc + "\nos: [\"" + runtime.GOOS + "\"]\n---\n# " + tc.id + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "Investigate https://example.com/pricing and compare the claims with citations, evidence, and a timeline.", SelectOptions{
		Mode:                SkillSelectorModeHybrid,
		EnableRerank:        true,
		ConfidenceThreshold: 0.78,
	})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill != "deep_research" {
		t.Fatalf("expected deep_research for URL research request, got=%+v", decision)
	}
}
