package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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
