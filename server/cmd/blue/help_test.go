package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCandidateSkillIDs_DottedTopicAddsBaseSkill(t *testing.T) {
	ids := candidateSkillIDs("reminder.add")
	if len(ids) != 2 {
		t.Fatalf("len(ids)=%d, want 2 (%v)", len(ids), ids)
	}
	if ids[0] != "reminder.add" {
		t.Fatalf("ids[0]=%q, want %q", ids[0], "reminder.add")
	}
	if ids[1] != "reminder" {
		t.Fatalf("ids[1]=%q, want %q", ids[1], "reminder")
	}
}

func TestFindSkillManual_FromFilesystemRoot(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := "---\nname: demo\n---\n# Demo Skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(want), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	manual, source, ok, err := findSkillManual("demo", []string{root})
	if err != nil {
		t.Fatalf("findSkillManual error: %v", err)
	}
	if !ok {
		t.Fatal("expected to find manual")
	}
	if manual != want {
		t.Fatalf("manual mismatch:\n got: %q\nwant: %q", manual, want)
	}
	if source != filepath.Join(skillDir, "SKILL.md") {
		t.Fatalf("source=%q, want %q", source, filepath.Join(skillDir, "SKILL.md"))
	}
}

func TestFindSkillManual_DottedTopicFallsBackToBaseSkill(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "reminder")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := "# Reminder Skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(want), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	manual, _, ok, err := findSkillManual("reminder.add", []string{root})
	if err != nil {
		t.Fatalf("findSkillManual error: %v", err)
	}
	if !ok {
		t.Fatal("expected to find manual for dotted topic")
	}
	if manual != want {
		t.Fatalf("manual=%q, want %q", manual, want)
	}
}

func TestFindSkillManual_UsesManifestNameFromAliasedDirectory(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "team-browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := "---\nname: browser\ndescription: aliased browser\n---\n# Browser Skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(want), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	manual, source, ok, err := findSkillManual("browser", []string{root})
	if err != nil {
		t.Fatalf("findSkillManual error: %v", err)
	}
	if !ok {
		t.Fatal("expected to find manual for aliased directory")
	}
	if manual != want {
		t.Fatalf("manual mismatch:\n got: %q\nwant: %q", manual, want)
	}
	if source != filepath.Join(skillDir, "SKILL.md") {
		t.Fatalf("source=%q, want %q", source, filepath.Join(skillDir, "SKILL.md"))
	}
}

func TestFindSkillManual_AcceptsHyphenAliasForUnderscoreSkill(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "word_docx")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := "---\nid: word_docx\nname: Word DOCX\n---\n# Word DOCX Skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(want), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	manual, source, ok, err := findSkillManual("word-docx", []string{root})
	if err != nil {
		t.Fatalf("findSkillManual error: %v", err)
	}
	if !ok {
		t.Fatal("expected to find manual for hyphen alias")
	}
	if manual != want {
		t.Fatalf("manual mismatch:\n got: %q\nwant: %q", manual, want)
	}
	if source != filepath.Join(skillDir, "SKILL.md") {
		t.Fatalf("source=%q, want %q", source, filepath.Join(skillDir, "SKILL.md"))
	}
}

func TestFindSkillManual_IncludesDisabledSkill(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := "---\nname: demo\nenabled: false\ndescription: disabled demo\n---\n# Demo Skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(want), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	manual, _, ok, err := findSkillManual("demo", []string{root})
	if err != nil {
		t.Fatalf("findSkillManual error: %v", err)
	}
	if !ok {
		t.Fatal("expected disabled skill manual to resolve")
	}
	if manual != want {
		t.Fatalf("manual mismatch:\n got: %q\nwant: %q", manual, want)
	}
}

func TestParseSkillDescription_UsesFrontmatterDescription(t *testing.T) {
	data := []byte("---\nname: demo\ndescription: \"demo skill description\"\n---\n# Demo\n")
	got := parseSkillDescription(data)
	if got != "demo skill description" {
		t.Fatalf("description=%q, want %q", got, "demo skill description")
	}
}

func TestRootHelpGuideLines_DescribeBuiltinsSubcommandsAndExec(t *testing.T) {
	lines := rootHelpGuideLines()
	joined := strings.Join(lines, "\n")

	for _, want := range []string{
		"blue help <skill>",
		"blue <skill> ...",
		"blue media generate",
		"blue exec command='tool ...'",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected root help guides to contain %q, got:\n%s", want, joined)
		}
	}
}

func TestListPinnedSkillSummaries_OnlyPinnedReturned(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\ndescription: workspace demo\n---\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	skills := listPinnedSkillSummaries(workspaceDir, []string{"demo", "missing"})
	if len(skills) != 1 {
		t.Fatalf("len(skills)=%d, want 1", len(skills))
	}
	if skills[0].Name != "demo" || skills[0].Description != "workspace demo" {
		t.Fatalf("skills[0]=%+v, want demo/workspace demo", skills[0])
	}
}

func TestListPinnedSkillSummaries_IncludesDisabledPinnedSkill(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\nenabled: false\ndescription: disabled demo\n---\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	skills := listPinnedSkillSummaries(workspaceDir, []string{"demo"})
	if len(skills) != 1 {
		t.Fatalf("len(skills)=%d, want 1", len(skills))
	}
	if skills[0].Name != "demo" || skills[0].Description != "disabled demo" {
		t.Fatalf("skills[0]=%+v, want demo/disabled demo", skills[0])
	}
}

func TestResolveHelpSkill_UsesWorkspaceLookupForNonPinnedSkill(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := "---\nname: demo\nversion: 1.0.0\ndescription: demo skill\ninvocation: blue demo\nexamples:\n  - blue demo\ncapability_tags:\n  - demo\ninteraction_mode: stateless\ncard_support: none\n---\n# Demo Skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(want), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	manual, source, ok, err := resolveHelpSkill("demo", workspaceDir)
	if err != nil {
		t.Fatalf("resolveHelpSkill error: %v", err)
	}
	if !ok {
		t.Fatal("expected resolveHelpSkill to find local non-pinned skill")
	}
	if manual != want {
		t.Fatalf("manual mismatch:\n got: %q\nwant: %q", manual, want)
	}
	if source != filepath.Join(skillDir, "SKILL.md") {
		t.Fatalf("source=%q, want %q", source, filepath.Join(skillDir, "SKILL.md"))
	}
}

func TestResolveHelpSkillStrict_ReportsCanonicalConflict(t *testing.T) {
	workspaceDir := t.TempDir()

	agentsDir := filepath.Join(workspaceDir, ".agents", "skills", "team-browser")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "SKILL.md"), []byte(`---
name: browser
version: 1.0.0
description: browser from workspace agents
invocation: blue browser
examples:
  - blue browser
capability_tags:
  - browser
interaction_mode: stateless
card_support: none
---
# Browser
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	duplicateDir := filepath.Join(workspaceDir, ".agents", "skills", "browser")
	if err := os.MkdirAll(duplicateDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(duplicateDir, "SKILL.md"), []byte(`---
name: browser
version: 1.0.0
description: browser duplicate from workspace agents
invocation: blue browser
examples:
  - blue browser
capability_tags:
  - browser
interaction_mode: stateless
card_support: none
---
# Browser
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	_, _, ok, err := resolveHelpSkillStrict("browser", workspaceDir)
	if err == nil || ok {
		t.Fatalf("expected strict help conflict, ok=%v err=%v", ok, err)
	}
	if !strings.Contains(err.Error(), `workspace/.agents canonical skill "browser"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
