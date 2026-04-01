package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

type stubLookupSkillRegistry struct {
	skills  map[string]skillpkg.Skill
	enabled map[string]bool
}

func (s *stubLookupSkillRegistry) Get(id string) skillpkg.Skill {
	if s == nil {
		return nil
	}
	return s.skills[id]
}

func (s *stubLookupSkillRegistry) IsEnabled(id string) bool {
	if s == nil || s.enabled == nil {
		return false
	}
	return s.enabled[id]
}

type stubLookupSkill struct {
	manifest *skillpkg.Manifest
	result   *skillpkg.Result
}

func (s *stubLookupSkill) Manifest() *skillpkg.Manifest  { return s.manifest }
func (s *stubLookupSkill) Validate(map[string]any) error { return nil }
func (s *stubLookupSkill) Execute(context.Context, map[string]any) (*skillpkg.Result, error) {
	return s.result, nil
}

func TestResolveRuntimeSkillForExecution_ResolvesRegistryAliasCandidates(t *testing.T) {
	registry := &stubLookupSkillRegistry{
		skills: map[string]skillpkg.Skill{
			"web_search": &stubLookupSkill{
				manifest: &skillpkg.Manifest{ID: "web_search", Name: "Web Search"},
				result:   skillpkg.NewResult(map[string]any{"status": "ok"}),
			},
		},
		enabled: map[string]bool{"web_search": true},
	}

	resolved, err := resolveRuntimeSkillForExecution(registry, "", "web-search")
	if err != nil {
		t.Fatalf("resolveRuntimeSkillForExecution error: %v", err)
	}
	if resolved.ID != "web_search" {
		t.Fatalf("resolved.ID=%q, want web_search", resolved.ID)
	}
}

func TestRuntimeSkillExecutionWorkspace_PrefersExplicitBlueWorkdir(t *testing.T) {
	got := runtimeSkillExecutionWorkspace("/tmp/default", map[string]any{
		"__blue_workdir": "/tmp/project",
	})
	if got != "/tmp/project" {
		t.Fatalf("got=%q, want /tmp/project", got)
	}
}

func TestResolveRuntimeSkillForExecution_UsesWorkspaceManifestToCanonicalizeRegistryExecution(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "team-browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: browser
description: Browser override
---
# Browser
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	registry := &stubLookupSkillRegistry{
		skills: map[string]skillpkg.Skill{
			"browser": &stubLookupSkill{
				manifest: &skillpkg.Manifest{ID: "browser", Name: "Browser"},
				result:   skillpkg.NewResult(map[string]any{"status": "ok"}),
			},
		},
		enabled: map[string]bool{"browser": true},
	}

	resolved, err := resolveRuntimeSkillForExecution(registry, workspaceDir, "team-browser")
	if err != nil {
		t.Fatalf("resolveRuntimeSkillForExecution error: %v", err)
	}
	if resolved.ID != "browser" {
		t.Fatalf("resolved.ID=%q, want browser", resolved.ID)
	}
}

func TestResolveRuntimeSkillForExecution_ResolvesHyphenAliasToUnderscoreRegistryExecution(t *testing.T) {
	registry := &stubLookupSkillRegistry{
		skills: map[string]skillpkg.Skill{
			"word_docx": &stubLookupSkill{
				manifest: &skillpkg.Manifest{ID: "word_docx", Name: "Word DOCX"},
				result:   skillpkg.NewResult(map[string]any{"status": "ok"}),
			},
		},
		enabled: map[string]bool{"word_docx": true},
	}

	resolved, err := resolveRuntimeSkillForExecution(registry, "", "word-docx")
	if err != nil {
		t.Fatalf("resolveRuntimeSkillForExecution error: %v", err)
	}
	if resolved.ID != "word_docx" {
		t.Fatalf("resolved.ID=%q, want word_docx", resolved.ID)
	}
}

func TestResolveRuntimeSkillForExecution_FallsBackToDeclarativeManifestSkill(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: demo
version: 1.0.0
description: Demo declarative skill
invocation: blue demo
examples:
  - blue demo
capability_tags:
  - demo
interaction_mode: stateless
card_support: none
---
# Demo
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	resolved, err := resolveRuntimeSkillForExecution(nil, workspaceDir, "demo")
	if err != nil {
		t.Fatalf("resolveRuntimeSkillForExecution error: %v", err)
	}
	if resolved.ID != "demo" {
		t.Fatalf("resolved.ID=%q, want demo", resolved.ID)
	}
	result, execErr := resolved.Skill.Execute(context.Background(), map[string]any{"foo": "bar"})
	if execErr != nil {
		t.Fatalf("Execute error: %v", execErr)
	}
	data, _ := result.Data.(map[string]any)
	if data["skill"] != "demo" {
		t.Fatalf("data[skill]=%v, want demo", data["skill"])
	}
}

func TestResolveRuntimeSkillForExecution_ReturnsDisabledForManifestSkill(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: demo
description: Demo declarative skill
enabled: false
---
# Demo
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	_, err := resolveRuntimeSkillForExecution(nil, workspaceDir, "demo")
	if err == nil || err.Error() != "skill demo is disabled" {
		t.Fatalf("err=%v, want disabled error", err)
	}
}

func TestResolveRuntimeSkillForExecution_ReturnsCanonicalConflictForManifestLookup(t *testing.T) {
	workspaceDir := t.TempDir()

	agentsDir := filepath.Join(workspaceDir, ".agents", "skills", "team-browser")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "SKILL.md"), []byte(`---
name: browser
description: Browser from workspace agents
---
# Browser
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	duplicateDir := filepath.Join(workspaceDir, ".agents", "skills", "browser")
	if err := os.MkdirAll(duplicateDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(duplicateDir, "SKILL.md"), []byte(`---
name: browser
description: Browser duplicate from workspace agents
---
# Browser
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	_, err := resolveRuntimeSkillForExecution(nil, workspaceDir, "browser")
	if err == nil || !strings.Contains(err.Error(), `workspace/.agents canonical skill "browser"`) {
		t.Fatalf("expected canonical conflict error, got %v", err)
	}
}
