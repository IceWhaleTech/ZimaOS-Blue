package agentcore

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeIndexSkill(t *testing.T, workspaceDir, rootDir, id, desc string) {
	t.Helper()

	dir := filepath.Join(workspaceDir, rootDir, "skills", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	content := "---\n" +
		"name: " + id + "\n" +
		"version: 1.0.0\n" +
		"description: " + desc + "\n" +
		"invocation: blue " + id + "\n" +
		"examples:\n  - blue " + id + "\n" +
		"capability_tags:\n  - test\n" +
		"interaction_mode: stateless\n" +
		"card_support: none\n" +
		"os: [\"" + runtime.GOOS + "\"]\n" +
		"---\n# " + id + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func findSkillDocByName(docs []SkillDoc, name string) (SkillDoc, bool) {
	for _, doc := range docs {
		if doc.Name == name {
			return doc, true
		}
	}
	return SkillDoc{}, false
}

func TestBuildSkillIndex_IncludesEmbeddedBuiltinFallback(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	docs, err := BuildSkillIndex(workspaceDir)
	if err != nil {
		t.Fatalf("BuildSkillIndex error: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected embedded builtin skills in index")
	}
	if _, ok := findSkillDocByName(docs, "browser"); !ok {
		t.Fatalf("expected embedded browser skill in index, got %v", docs)
	}
	if _, ok := findSkillDocByName(docs, "web_query"); !ok {
		t.Fatalf("expected embedded web_query skill in index, got %v", docs)
	}
}

func TestBuildSkillIndex_WorkspaceOverridesEmbeddedBuiltin(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeIndexSkill(t, workspaceDir, ".claude", "browser", "workspace browser override")

	docs, err := BuildSkillIndex(workspaceDir)
	if err != nil {
		t.Fatalf("BuildSkillIndex error: %v", err)
	}
	doc, ok := findSkillDocByName(docs, "browser")
	if !ok {
		t.Fatalf("expected browser doc in index, got %v", docs)
	}
	if doc.Description != "workspace browser override" {
		t.Fatalf("expected workspace browser override, got %q", doc.Description)
	}
}

func TestBuildSkillIndex_HomeOverridesEmbeddedBuiltin(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeIndexSkill(t, homeDir, ".claude", "web_query", "home web query override")

	docs, err := BuildSkillIndex(workspaceDir)
	if err != nil {
		t.Fatalf("BuildSkillIndex error: %v", err)
	}
	doc, ok := findSkillDocByName(docs, "web_query")
	if !ok {
		t.Fatalf("expected web_query doc in index, got %v", docs)
	}
	if doc.Description != "home web query override" {
		t.Fatalf("expected home web_query override, got %q", doc.Description)
	}
}

func TestBuildSkillIndex_ErrorsOnAmbiguousWorkspaceCanonicalSkill(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeIndexSkill(t, workspaceDir, ".agents", "team_browser", "workspace browser from agents")
	writeIndexSkill(t, workspaceDir, ".agents", "browser", "workspace browser duplicate from agents")

	agentsSkill := filepath.Join(workspaceDir, ".agents", "skills", "team_browser", "SKILL.md")
	content := "---\n" +
		"name: browser\n" +
		"version: 1.0.0\n" +
		"description: workspace browser from agents\n" +
		"invocation: blue browser\n" +
		"examples:\n  - blue browser\n" +
		"capability_tags:\n  - test\n" +
		"interaction_mode: stateless\n" +
		"card_support: none\n" +
		"os: [\"" + runtime.GOOS + "\"]\n" +
		"---\n# browser\n"
	if err := os.WriteFile(agentsSkill, []byte(content), 0o644); err != nil {
		t.Fatalf("rewrite skill: %v", err)
	}

	_, err := BuildSkillIndex(workspaceDir)
	if err == nil {
		t.Fatal("expected canonical conflict error")
	}
	if !strings.Contains(err.Error(), `workspace/.agents canonical skill "browser"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
