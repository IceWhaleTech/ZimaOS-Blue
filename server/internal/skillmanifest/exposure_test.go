package skillmanifest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeExposureSkill(t *testing.T, skillsRoot, dirName, frontmatter string) string {
	t.Helper()

	skillDir := filepath.Join(skillsRoot, dirName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	content := "---\n" + frontmatter + "\n---\n# " + dirName + "\n"
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	return path
}

func findExposureView(snapshot SkillExposureSnapshot, name string) (SkillExposureView, bool) {
	for _, view := range snapshot.VisibleSkills {
		if view.Document.ID == name || view.Document.Name == name {
			return view, true
		}
	}
	return SkillExposureView{}, false
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestSkillExposureManager_ConditionalActivation(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeExposureSkill(t, filepath.Join(workspaceDir, ".claude", "skills"), "pkg_helper", "name: pkg_helper\ndescription: inspect files under pkg\npaths:\n  - pkg/**\nos:\n  - "+runtime.GOOS)

	manager := NewSkillExposureManager(workspaceDir)
	manager.SetDynamicExposureEnabledFunc(func() bool { return true })

	before, err := manager.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot before activation: %v", err)
	}
	view, ok := findExposureView(before, "pkg_helper")
	if !ok {
		t.Fatalf("expected conditional skill in snapshot")
	}
	if view.ActivationState != string(SkillActivationStateDormant) {
		t.Fatalf("activation_state=%q, want dormant", view.ActivationState)
	}
	if view.ActivationSource != string(SkillActivationSourceConditional) {
		t.Fatalf("activation_source=%q, want conditional", view.ActivationSource)
	}

	manager.ObserveToolPath("read", filepath.Join(workspaceDir, "pkg", "main.go"))

	after, err := manager.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot after activation: %v", err)
	}
	view, ok = findExposureView(after, "pkg_helper")
	if !ok {
		t.Fatalf("expected activated skill in snapshot")
	}
	if view.ActivationState != string(SkillActivationStateActive) {
		t.Fatalf("activation_state=%q, want active", view.ActivationState)
	}
	if view.ActivationSource != string(SkillActivationSourceConditional) {
		t.Fatalf("activation_source=%q, want conditional", view.ActivationSource)
	}
	if !hasString(after.ActivatedConditionalSkills, "pkg_helper") {
		t.Fatalf("expected pkg_helper in activated conditional skills, got %v", after.ActivatedConditionalSkills)
	}
}

func TestSkillExposureManager_InvalidEscapePathsNeverActivate(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeExposureSkill(t, filepath.Join(workspaceDir, ".claude", "skills"), "escape_only", "name: escape_only\ndescription: invalid conditional skill\npaths:\n  - ../secret/**\nos:\n  - "+runtime.GOOS)

	manager := NewSkillExposureManager(workspaceDir)
	manager.SetDynamicExposureEnabledFunc(func() bool { return true })

	before, err := manager.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot before activation: %v", err)
	}
	view, ok := findExposureView(before, "escape_only")
	if !ok {
		t.Fatalf("expected invalid-path skill in snapshot")
	}
	if view.ActivationState != string(SkillActivationStateDormant) {
		t.Fatalf("activation_state=%q, want dormant", view.ActivationState)
	}

	manager.ObserveToolPath("read", filepath.Join(workspaceDir, "pkg", "main.go"))

	after, err := manager.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot after read: %v", err)
	}
	view, ok = findExposureView(after, "escape_only")
	if !ok {
		t.Fatalf("expected invalid-path skill after read")
	}
	if view.ActivationState != string(SkillActivationStateDormant) {
		t.Fatalf("activation_state=%q, want dormant after read", view.ActivationState)
	}
	if hasString(after.ActivatedConditionalSkills, "escape_only") {
		t.Fatalf("did not expect invalid-path skill to activate, got %v", after.ActivatedConditionalSkills)
	}
}

func TestSkillExposureManager_DiscoversNestedSkillsAndSkipsGitignoredRoots(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	if err := os.WriteFile(filepath.Join(workspaceDir, ".gitignore"), []byte("ignored/\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	nestedRoot := filepath.Join(workspaceDir, "apps", "site", ".claude", "skills")
	writeExposureSkill(t, nestedRoot, "nested_web", "name: nested_web\ndescription: nested web helper\nos:\n  - "+runtime.GOOS)

	ignoredRoot := filepath.Join(workspaceDir, "ignored", "secret", ".agents", "skills")
	writeExposureSkill(t, ignoredRoot, "hidden_skill", "name: hidden_skill\ndescription: should stay hidden\nos:\n  - "+runtime.GOOS)

	manager := NewSkillExposureManager(workspaceDir)
	manager.SetDynamicExposureEnabledFunc(func() bool { return true })

	before, err := manager.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot before discovery: %v", err)
	}
	if _, ok := findExposureView(before, "nested_web"); ok {
		t.Fatalf("did not expect nested skill before discovery")
	}

	manager.ObserveToolPath("edit", filepath.Join(workspaceDir, "apps", "site", "src", "page.tsx"))
	manager.ObserveToolPath("read", filepath.Join(workspaceDir, "ignored", "secret", "file.txt"))

	after, err := manager.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot after discovery: %v", err)
	}
	view, ok := findExposureView(after, "nested_web")
	if !ok {
		t.Fatalf("expected nested skill after discovery")
	}
	if view.ActivationState != string(SkillActivationStateActive) {
		t.Fatalf("activation_state=%q, want active", view.ActivationState)
	}
	if view.ActivationSource != string(SkillActivationSourceDynamicDiscovered) {
		t.Fatalf("activation_source=%q, want dynamic_discovered", view.ActivationSource)
	}
	if !hasString(after.DiscoveredDirs, nestedRoot) {
		t.Fatalf("expected discovered dirs to include %q, got %v", nestedRoot, after.DiscoveredDirs)
	}
	if _, ok := findExposureView(after, "hidden_skill"); ok {
		t.Fatalf("did not expect gitignored nested skill to be discovered")
	}
	if hasString(after.DiscoveredDirs, ignoredRoot) {
		t.Fatalf("did not expect gitignored dir %q in discovered dirs %v", ignoredRoot, after.DiscoveredDirs)
	}
}
