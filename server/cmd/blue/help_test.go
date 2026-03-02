package main

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
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

	manual, source, ok := findSkillManual("demo", []string{root}, nil)
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

	manual, _, ok := findSkillManual("reminder.add", []string{root}, nil)
	if !ok {
		t.Fatal("expected to find manual for dotted topic")
	}
	if manual != want {
		t.Fatalf("manual=%q, want %q", manual, want)
	}
}

func TestParseSkillDescription_UsesFrontmatterDescription(t *testing.T) {
	data := []byte("---\nname: demo\ndescription: \"demo skill description\"\n---\n# Demo\n")
	got := parseSkillDescription(data)
	if got != "demo skill description" {
		t.Fatalf("description=%q, want %q", got, "demo skill description")
	}
}

func TestListPinnedSkillSummaries_OnlyPinnedReturned(t *testing.T) {
	embedded := fstest.MapFS{
		"skills/demo/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: demo\ndescription: embedded demo\n---\n"),
		},
		"skills/foo/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: foo\ndescription: embedded foo\n---\n"),
		},
	}

	skills := listPinnedSkillSummaries([]string{"demo", "missing"}, embedded.ReadFile)
	if len(skills) != 1 {
		t.Fatalf("len(skills)=%d, want 1", len(skills))
	}
	if skills[0].Name != "demo" || skills[0].Description != "embedded demo" {
		t.Fatalf("skills[0]=%+v, want demo/embedded demo", skills[0])
	}
}
