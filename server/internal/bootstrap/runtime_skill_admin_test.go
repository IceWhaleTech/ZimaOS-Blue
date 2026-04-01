package bootstrap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

func TestListRuntimeAdminSkills_IncludesManifestOnlyLocalSkill(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: demo
version: 1.0.0
description: Demo skill
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

	skills, err := listRuntimeAdminSkills(skillpkg.NewRegistry(), workspaceDir)
	if err != nil {
		t.Fatalf("listRuntimeAdminSkills error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills)=%d, want 1 (%#v)", len(skills), skills)
	}
	if skills[0].ID != "demo" || skills[0].Name != "demo" || !skills[0].Enabled || skills[0].Builtin {
		t.Fatalf("skills[0]=%#v, want demo/enabled/non-builtin", skills[0])
	}
}

func TestListRuntimeAdminSkills_LocalOverrideBeatsBuiltinMetadata(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "team-browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: browser
version: 1.0.0
description: Workspace browser
category: workspace
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

	registry := skillpkg.NewRegistry()
	if err := registry.Register(skillpkg.NewManifestSkill(&skillpkg.Manifest{ID: "browser", Name: "Built-in Browser", Category: "builtin"}), true); err != nil {
		t.Fatalf("register builtin browser: %v", err)
	}

	skills, err := listRuntimeAdminSkills(registry, workspaceDir)
	if err != nil {
		t.Fatalf("listRuntimeAdminSkills error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills)=%d, want 1 (%#v)", len(skills), skills)
	}
	if skills[0].ID != "browser" || skills[0].Name != "browser" || skills[0].Category != "workspace" || skills[0].Builtin {
		t.Fatalf("skills[0]=%#v, want workspace override metadata", skills[0])
	}
}

func TestListRuntimeAdminSkills_ReturnsCanonicalConflictForRegistryAliasLookup(t *testing.T) {
	workspaceDir := t.TempDir()

	firstDir := filepath.Join(workspaceDir, ".agents", "skills", "team-browser")
	if err := os.MkdirAll(firstDir, 0o755); err != nil {
		t.Fatalf("mkdir first skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(firstDir, "SKILL.md"), []byte(`---
name: browser
version: 1.0.0
description: Browser alias from workspace
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
		t.Fatalf("write first SKILL.md: %v", err)
	}

	secondDir := filepath.Join(workspaceDir, ".agents", "skills", "browser")
	if err := os.MkdirAll(secondDir, 0o755); err != nil {
		t.Fatalf("mkdir second skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secondDir, "SKILL.md"), []byte(`---
name: browser
version: 1.0.0
description: Browser duplicate from workspace
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
		t.Fatalf("write second SKILL.md: %v", err)
	}

	registry := skillpkg.NewRegistry()
	if err := registry.Register(skillpkg.NewManifestSkill(&skillpkg.Manifest{ID: "browser", Name: "browser"}), true); err != nil {
		t.Fatalf("register browser: %v", err)
	}

	_, err := listRuntimeAdminSkills(registry, workspaceDir)
	if err == nil || !strings.Contains(err.Error(), "canonical skill conflicts detected") {
		t.Fatalf("expected canonical conflict error, got %v", err)
	}
}

func TestEnsureRuntimeManagedSkillRegisteredWithRoots_RegistersManifestOnlySkill(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: demo
version: 1.0.0
description: Demo skill
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

	registry := skillpkg.NewRegistry()
	id, err := ensureRuntimeManagedSkillRegisteredWithRoots(registry, []string{root}, "demo")
	if err != nil {
		t.Fatalf("ensureRuntimeManagedSkillRegisteredWithRoots error: %v", err)
	}
	if id != "demo" {
		t.Fatalf("id=%q, want demo", id)
	}
	if registry.Get("demo") == nil || !registry.IsEnabled("demo") {
		t.Fatalf("expected demo to be registered and enabled")
	}
}

func TestEnsureRuntimeManagedSkillRegisteredWithRoots_ReturnsCanonicalConflict(t *testing.T) {
	root := t.TempDir()

	firstDir := filepath.Join(root, "team-browser")
	if err := os.MkdirAll(firstDir, 0o755); err != nil {
		t.Fatalf("mkdir first skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(firstDir, "SKILL.md"), []byte(`---
name: browser
version: 1.0.0
description: Browser alias
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
		t.Fatalf("write first SKILL.md: %v", err)
	}

	secondDir := filepath.Join(root, "browser")
	if err := os.MkdirAll(secondDir, 0o755); err != nil {
		t.Fatalf("mkdir second skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secondDir, "SKILL.md"), []byte(`---
name: browser
version: 1.0.0
description: Browser duplicate
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
		t.Fatalf("write second SKILL.md: %v", err)
	}

	_, err := ensureRuntimeManagedSkillRegisteredWithRoots(skillpkg.NewRegistry(), []string{root}, "browser")
	if err == nil || !strings.Contains(err.Error(), "canonical skill conflicts detected") {
		t.Fatalf("expected canonical conflict error, got %v", err)
	}
}

func TestMgmtSkillAdapter_EnableDisable_UsesCanonicalSkillLookup(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "team-browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: browser
version: 1.0.0
description: Browser skill
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

	registry := skillpkg.NewRegistry()
	adapter := &mgmtSkillAdapter{registry: registry, workspaceDir: workspaceDir}

	if err := adapter.DisableSkill(context.Background(), "team-browser"); err != nil {
		t.Fatalf("DisableSkill error: %v", err)
	}
	if registry.Get("browser") == nil || registry.IsEnabled("browser") {
		t.Fatalf("expected browser skill to be registered and disabled")
	}
	if err := adapter.EnableSkill(context.Background(), "browser"); err != nil {
		t.Fatalf("EnableSkill error: %v", err)
	}
	if !registry.IsEnabled("browser") {
		t.Fatalf("expected browser skill to be enabled")
	}
}

func TestSkillManagerAdapter_EnableDisable_UsesCanonicalSkillLookup(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "team-browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: browser
version: 1.0.0
description: Browser skill
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

	registry := skillpkg.NewRegistry()
	adapter := &skillManagerAdapter{registry: registry, dir: root}

	if err := adapter.Disable(context.Background(), "team-browser"); err != nil {
		t.Fatalf("Disable error: %v", err)
	}
	if registry.Get("browser") == nil || registry.IsEnabled("browser") {
		t.Fatalf("expected browser skill to be registered and disabled")
	}
	if err := adapter.Enable(context.Background(), "browser"); err != nil {
		t.Fatalf("Enable error: %v", err)
	}
	if !registry.IsEnabled("browser") {
		t.Fatalf("expected browser skill to be enabled")
	}
}

func TestSkillManagerAdapter_ListAndInfo_UseCanonicalManifestMetadata(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "team-browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: browser
version: 1.2.3
description: Browser skill
category: workspace
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

	adapter := &skillManagerAdapter{registry: skillpkg.NewRegistry(), dir: root}

	listJSON, err := adapter.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	var list []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Enabled     bool   `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(listJSON), &list); err != nil {
		t.Fatalf("unmarshal List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list)=%d, want 1 (%s)", len(list), listJSON)
	}
	if list[0].ID != "browser" || list[0].Name != "browser" || list[0].Category != "workspace" || !list[0].Enabled {
		t.Fatalf("list[0]=%#v, want canonical browser metadata", list[0])
	}

	infoJSON, err := adapter.Info(context.Background(), "browser")
	if err != nil {
		t.Fatalf("Info error: %v", err)
	}
	var detail struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Version     string `json:"version"`
		Installed   bool   `json:"installed"`
		Description string `json:"description"`
		Content     string `json:"content"`
	}
	if err := json.Unmarshal([]byte(infoJSON), &detail); err != nil {
		t.Fatalf("unmarshal Info: %v", err)
	}
	if detail.ID != "browser" || detail.Name != "browser" || detail.Version != "1.2.3" || !detail.Installed {
		t.Fatalf("detail=%#v, want installed canonical browser metadata", detail)
	}
	if detail.Description != "Browser skill" || detail.Content == "" {
		t.Fatalf("detail=%#v, want description and content", detail)
	}
}

func TestSkillManagerAdapter_Uninstall_UsesCanonicalSkillLookup(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "team-browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: browser
version: 1.0.0
description: Browser skill
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

	registry := skillpkg.NewRegistry()
	if err := registry.Register(skillpkg.NewManifestSkill(&skillpkg.Manifest{ID: "browser", Name: "browser"}), false); err != nil {
		t.Fatalf("register browser: %v", err)
	}

	adapter := &skillManagerAdapter{registry: registry, dir: root}
	if err := adapter.Uninstall(context.Background(), "browser"); err != nil {
		t.Fatalf("Uninstall error: %v", err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatalf("expected alias dir to be removed, stat err=%v", err)
	}
	if registry.Get("browser") != nil {
		t.Fatalf("expected browser skill to be unregistered")
	}
}

func TestSkillManagerAdapter_Info_ReturnsCanonicalConflictForAmbiguousInstalledSkill(t *testing.T) {
	root := t.TempDir()

	firstDir := filepath.Join(root, "team-browser")
	if err := os.MkdirAll(firstDir, 0o755); err != nil {
		t.Fatalf("mkdir first skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(firstDir, "SKILL.md"), []byte(`---
name: browser
version: 1.0.0
description: Browser alias
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
		t.Fatalf("write first SKILL.md: %v", err)
	}

	secondDir := filepath.Join(root, "browser")
	if err := os.MkdirAll(secondDir, 0o755); err != nil {
		t.Fatalf("mkdir second skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secondDir, "SKILL.md"), []byte(`---
name: browser
version: 1.0.0
description: Browser duplicate
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
		t.Fatalf("write second SKILL.md: %v", err)
	}

	adapter := &skillManagerAdapter{registry: skillpkg.NewRegistry(), dir: root}
	_, err := adapter.Info(context.Background(), "browser")
	if err == nil || !strings.Contains(err.Error(), "canonical skill conflicts detected") {
		t.Fatalf("expected canonical conflict error, got %v", err)
	}
}
