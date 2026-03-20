package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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
	mk("web_search", "search the web")
	mk("browser", "browse urls")

	sel := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	decision, err := sel.Select(context.Background(), "请帮我搜索最新新闻", SelectOptions{Mode: SkillSelectorModeHybrid, EnableRerank: true, ConfidenceThreshold: 0.78})
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if decision.SelectedSkill == "" {
		t.Fatalf("expected selected skill")
	}
	if decision.SelectedSkill != "web_search" {
		t.Fatalf("expected web_search, got %q", decision.SelectedSkill)
	}
}

func TestSkillSelector_LowConfidenceNeedsClarify(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	for _, id := range []string{"browser", "web_search", "analyze"} {
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

func TestBuildSkillIndex_StructuredFields(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	dir := filepath.Join(workspaceDir, ".claude", "skills", "youtube-video-analyzer")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	content := `---
name: youtube-video-analyzer
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
	if len(docs) != 1 {
		t.Fatalf("expected 1 skill doc, got %d", len(docs))
	}
	doc := docs[0]
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

func TestSkillSelector_DefinitionQueryDoesNotAutoRoute(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	for _, tc := range []struct {
		id   string
		desc string
	}{
		{id: "web_search", desc: "search the web"},
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

func TestSkillSelector_WorkspaceQueryDoesNotRouteWebSearch(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	for _, tc := range []struct {
		id   string
		desc string
	}{
		{id: "web_search", desc: "search the web"},
		{id: "analyze", desc: "analyze workspace files"},
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
	if decision.SelectedSkill == "web_search" {
		t.Fatalf("workspace file task should not route to web_search, got=%+v", decision)
	}
	if decision.SelectedSkill != "analyze" {
		t.Fatalf("expected analyze for workspace file task, got=%+v", decision)
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
		{id: "analyze", desc: "analyze workspace files"},
		{id: "web_search", desc: "search the web"},
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
