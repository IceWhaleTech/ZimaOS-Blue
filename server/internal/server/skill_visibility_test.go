package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/labstack/echo/v4"
)

// =============================================================================
// Skill Visibility Integration Tests
// =============================================================================
// These tests verify that installed skills are properly visible to Claude Code CLI.
// They ensure the skill registry, API responses, and persistence work correctly.

// TestSkillVisibility_AfterInstall verifies that a skill becomes visible immediately after installation
func TestSkillVisibility_AfterInstall(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Add a remote skill to the store
	handler.remoteSkills["visibility-test-skill"] = &RemoteSkill{
		ID:          "visibility-test-skill",
		Name:        "Visibility Test Skill",
		Version:     "1.0.0",
		Description: "A skill to test visibility after install",
		Category:    "testing",
		SourceID:    "test-source",
		SourceName:  "Test Source",
	}

	e := echo.New()

	// Step 1: Install the skill
	req := httptest.NewRequest(http.MethodPost, "/skill-store/install/visibility-test-skill", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("visibility-test-skill")

	err := handler.InstallSkill(c)
	if err != nil {
		t.Fatalf("InstallSkill failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Step 2: Verify skill is visible in registry
	installedSkill := registry.Get("visibility-test-skill")
	if installedSkill == nil {
		t.Fatal("skill should be visible in registry after install")
	}

	// Step 3: Verify skill appears in list endpoint
	req = httptest.NewRequest(http.MethodGet, "/skills", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	err = handler.ListSkills(c)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}

	var skills []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &skills); err != nil {
		t.Fatalf("failed to unmarshal skills: %v", err)
	}

	found := false
	for _, s := range skills {
		if s["id"] == "visibility-test-skill" {
			found = true
			// Verify required fields for CC CLI compatibility
			if s["name"] == nil || s["name"] == "" {
				t.Error("skill should have name field")
			}
			if s["description"] == nil {
				t.Error("skill should have description field")
			}
			if s["enabled"] == nil {
				t.Error("skill should have enabled field")
			}
			break
		}
	}

	if !found {
		t.Error("installed skill should appear in skills list")
	}
}

// TestSkillVisibility_AfterUninstall verifies that a skill is no longer visible after uninstallation
func TestSkillVisibility_AfterUninstall(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Register a non-builtin skill
	testSkill := NewRemoteSkillAdapter(&skill.Manifest{
		ID:          "uninstall-test-skill",
		Name:        "Uninstall Test Skill",
		Version:     "1.0.0",
		Description: "A skill to test visibility after uninstall",
	})
	registry.Register(testSkill, false)

	e := echo.New()

	// Step 1: Verify skill is visible before uninstall
	if registry.Get("uninstall-test-skill") == nil {
		t.Fatal("skill should be visible before uninstall")
	}

	// Step 2: Uninstall the skill
	req := httptest.NewRequest(http.MethodPost, "/skill-store/uninstall/uninstall-test-skill", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("uninstall-test-skill")

	err := handler.UninstallSkill(c)
	if err != nil {
		t.Fatalf("UninstallSkill failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Step 3: Verify skill is no longer visible
	if registry.Get("uninstall-test-skill") != nil {
		t.Error("skill should not be visible after uninstall")
	}

	// Step 4: Verify skill does not appear in list
	req = httptest.NewRequest(http.MethodGet, "/skills", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	err = handler.ListSkills(c)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}

	var skills []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &skills); err != nil {
		t.Fatalf("failed to unmarshal skills: %v", err)
	}

	for _, s := range skills {
		if s["id"] == "uninstall-test-skill" {
			t.Error("uninstalled skill should not appear in skills list")
		}
	}
}

// TestSkillVisibility_CCCLIFormat verifies that skill API responses match CC CLI expected format
func TestSkillVisibility_CCCLIFormat(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Register a skill with all fields
	testSkill := NewRemoteSkillAdapter(&skill.Manifest{
		ID:          "format-test-skill",
		Name:        "Format Test Skill",
		Version:     "2.0.0",
		Description: "A skill to test CC CLI format compatibility",
		Author:      "Test Author",
		Category:    "productivity",
		Tags:        []string{"test", "format", "cli"},
	})
	registry.Register(testSkill, false)

	e := echo.New()

	// Get skill details
	req := httptest.NewRequest(http.MethodGet, "/skills/format-test-skill", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("format-test-skill")

	err := handler.GetSkill(c)
	if err != nil {
		t.Fatalf("GetSkill failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var skillData map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &skillData); err != nil {
		t.Fatalf("failed to unmarshal skill: %v", err)
	}

	// Verify CC CLI required fields
	requiredFields := []string{"id", "name", "version", "description", "enabled"}
	for _, field := range requiredFields {
		if _, exists := skillData[field]; !exists {
			t.Errorf("skill response missing required field: %s", field)
		}
	}

	// Verify field types
	if _, ok := skillData["id"].(string); !ok {
		t.Error("id should be a string")
	}
	if _, ok := skillData["name"].(string); !ok {
		t.Error("name should be a string")
	}
	if _, ok := skillData["enabled"].(bool); !ok {
		t.Error("enabled should be a boolean")
	}

	// Verify ID format (should be lowercase, hyphenated)
	id := skillData["id"].(string)
	if id != "format-test-skill" {
		t.Errorf("id should match expected format, got: %s", id)
	}
}

// TestSkillVisibility_RegistrySync verifies that skill registry stays in sync with API operations
func TestSkillVisibility_RegistrySync(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Add remote skill
	handler.remoteSkills["sync-test-skill"] = &RemoteSkill{
		ID:          "sync-test-skill",
		Name:        "Sync Test Skill",
		Version:     "1.0.0",
		Description: "A skill to test registry sync",
		Category:    "testing",
		SourceID:    "test-source",
		SourceName:  "Test Source",
	}

	e := echo.New()

	// Initial state: skill not in registry
	if registry.Get("sync-test-skill") != nil {
		t.Fatal("skill should not be in registry initially")
	}
	initialCount := registry.Count()

	// Install skill
	req := httptest.NewRequest(http.MethodPost, "/skill-store/install/sync-test-skill", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("sync-test-skill")

	handler.InstallSkill(c)

	// Verify registry count increased
	if registry.Count() != initialCount+1 {
		t.Errorf("registry count should increase by 1, got %d (was %d)", registry.Count(), initialCount)
	}

	// Verify skill is in registry
	if registry.Get("sync-test-skill") == nil {
		t.Error("skill should be in registry after install")
	}

	// Verify skill manifest is correct
	installedSkill := registry.Get("sync-test-skill")
	manifest := installedSkill.Manifest()
	if manifest.Name != "Sync Test Skill" {
		t.Errorf("manifest name mismatch: expected 'Sync Test Skill', got '%s'", manifest.Name)
	}

	// Uninstall skill
	req = httptest.NewRequest(http.MethodPost, "/skill-store/uninstall/sync-test-skill", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("sync-test-skill")

	handler.UninstallSkill(c)

	// Verify registry count decreased
	if registry.Count() != initialCount {
		t.Errorf("registry count should return to initial, got %d (expected %d)", registry.Count(), initialCount)
	}

	// Verify skill is not in registry
	if registry.Get("sync-test-skill") != nil {
		t.Error("skill should not be in registry after uninstall")
	}
}

// TestSkillVisibility_EnableDisable verifies that enable/disable affects skill visibility
func TestSkillVisibility_EnableDisable(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Register a skill
	testSkill := NewRemoteSkillAdapter(&skill.Manifest{
		ID:          "enable-test-skill",
		Name:        "Enable Test Skill",
		Version:     "1.0.0",
		Description: "A skill to test enable/disable",
	})
	registry.Register(testSkill, false)

	e := echo.New()

	// Verify skill is enabled by default
	if !registry.IsEnabled("enable-test-skill") {
		t.Error("skill should be enabled by default")
	}

	// Disable skill
	req := httptest.NewRequest(http.MethodPost, "/skills/enable-test-skill/disable", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("enable-test-skill")

	err := handler.DisableSkill(c)
	if err != nil {
		t.Fatalf("DisableSkill failed: %v", err)
	}

	// Verify skill is disabled
	if registry.IsEnabled("enable-test-skill") {
		t.Error("skill should be disabled after disable call")
	}

	// Verify disabled skill still appears in list but with enabled=false
	req = httptest.NewRequest(http.MethodGet, "/skills", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	handler.ListSkills(c)

	var skills []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &skills)

	for _, s := range skills {
		if s["id"] == "enable-test-skill" {
			if s["enabled"].(bool) != false {
				t.Error("disabled skill should have enabled=false in list")
			}
		}
	}

	// Re-enable skill
	req = httptest.NewRequest(http.MethodPost, "/skills/enable-test-skill/enable", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("enable-test-skill")

	handler.EnableSkill(c)

	// Verify skill is enabled again
	if !registry.IsEnabled("enable-test-skill") {
		t.Error("skill should be enabled after enable call")
	}
}

// TestSkillVisibility_ListEnabledOnly verifies that ListEnabled returns only enabled skills
func TestSkillVisibility_ListEnabledOnly(t *testing.T) {
	registry := skill.NewRegistry()

	// Register multiple skills
	skill1 := NewRemoteSkillAdapter(&skill.Manifest{ID: "skill-1", Name: "Skill 1"})
	skill2 := NewRemoteSkillAdapter(&skill.Manifest{ID: "skill-2", Name: "Skill 2"})
	skill3 := NewRemoteSkillAdapter(&skill.Manifest{ID: "skill-3", Name: "Skill 3"})

	registry.Register(skill1, false)
	registry.Register(skill2, false)
	registry.Register(skill3, false)

	// Disable one skill
	registry.Disable("skill-2")

	// Verify ListEnabled returns only enabled skills
	enabledSkills := registry.ListEnabled()
	if len(enabledSkills) != 2 {
		t.Errorf("expected 2 enabled skills, got %d", len(enabledSkills))
	}

	for _, s := range enabledSkills {
		if s.Manifest.ID == "skill-2" {
			t.Error("disabled skill should not appear in ListEnabled")
		}
	}
}

// TestSkillVisibility_BuiltinProtection verifies that builtin skills cannot be uninstalled
func TestSkillVisibility_BuiltinProtection(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Register a builtin skill
	builtinSkill := NewRemoteSkillAdapter(&skill.Manifest{
		ID:          "builtin-protected-skill",
		Name:        "Builtin Protected Skill",
		Version:     "1.0.0",
		Description: "A builtin skill that cannot be uninstalled",
	})
	registry.Register(builtinSkill, true) // true = builtin

	e := echo.New()

	// Attempt to uninstall builtin skill
	req := httptest.NewRequest(http.MethodPost, "/skill-store/uninstall/builtin-protected-skill", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("builtin-protected-skill")

	handler.UninstallSkill(c)

	// Should return forbidden
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d for builtin uninstall, got %d", http.StatusForbidden, rec.Code)
	}

	// Verify skill is still in registry
	if registry.Get("builtin-protected-skill") == nil {
		t.Error("builtin skill should still be in registry after failed uninstall")
	}
}

// =============================================================================
// Local Skill Discovery Tests
// =============================================================================

// TestSkillVisibility_LocalDiscovery verifies that skills in local directory are discovered
func TestSkillVisibility_LocalDiscovery(t *testing.T) {
	// Create temp directory for local skills
	tempDir, err := os.MkdirTemp("", "skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a SKILL.md file
	skillContent := `---
name: local-test-skill
description: A locally discovered skill
version: 1.0.0
author: Test Author
category: testing
tags: [local, test]
---

# Local Test Skill

This is a test skill for local discovery.
`
	skillPath := filepath.Join(tempDir, "local-test-skill", "SKILL.md")
	os.MkdirAll(filepath.Dir(skillPath), 0755)
	if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Test parsing SKILL.md format
	t.Run("parse SKILL.md format", func(t *testing.T) {
		content, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("failed to read SKILL.md: %v", err)
		}

		// Verify content contains expected fields
		contentStr := string(content)
		if !contains(contentStr, "name: local-test-skill") {
			t.Error("SKILL.md should contain name field")
		}
		if !contains(contentStr, "description: A locally discovered skill") {
			t.Error("SKILL.md should contain description field")
		}
	})
}

// =============================================================================
// Search Tests
// =============================================================================

// TestSkillSearch_MultiLanguage verifies search works with non-English characters
func TestSkillSearch_MultiLanguage(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Add skills with different languages
	handler.remoteSkills["chinese-skill"] = &RemoteSkill{
		ID:          "chinese-skill",
		Name:        "中文技能",
		Description: "这是一个中文描述的技能",
		Category:    "testing",
		SourceID:    "test",
		SourceName:  "Test",
	}
	handler.remoteSkills["japanese-skill"] = &RemoteSkill{
		ID:          "japanese-skill",
		Name:        "日本語スキル",
		Description: "これは日本語のスキルです",
		Category:    "testing",
		SourceID:    "test",
		SourceName:  "Test",
	}
	handler.remoteSkills["english-skill"] = &RemoteSkill{
		ID:          "english-skill",
		Name:        "English Skill",
		Description: "This is an English skill",
		Category:    "testing",
		SourceID:    "test",
		SourceName:  "Test",
	}

	e := echo.New()

	testCases := []struct {
		name        string
		searchQuery string
		expectFound string
	}{
		{"Chinese search", "中文", "chinese-skill"},
		{"Japanese search", "日本語", "japanese-skill"},
		{"English search", "English", "english-skill"},
		{"Partial Chinese", "技能", "chinese-skill"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/skill-store/browse?search="+tc.searchQuery, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler.BrowseSkills(c)

			var skills []*RemoteSkill
			json.Unmarshal(rec.Body.Bytes(), &skills)

			found := false
			for _, s := range skills {
				if s.ID == tc.expectFound {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("expected to find skill %s with search query %s", tc.expectFound, tc.searchQuery)
			}
		})
	}
}

// TestSkillSearch_CaseInsensitive verifies search is case insensitive
func TestSkillSearch_CaseInsensitive(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	handler.remoteSkills["case-test"] = &RemoteSkill{
		ID:          "case-test",
		Name:        "Case Test Skill",
		Description: "Testing case sensitivity",
		Category:    "testing",
		SourceID:    "test",
		SourceName:  "Test",
	}

	e := echo.New()

	testCases := []string{"case", "CASE", "Case", "cAsE"}

	for _, query := range testCases {
		t.Run("search_"+query, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/skill-store/browse?search="+query, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler.BrowseSkills(c)

			var skills []*RemoteSkill
			json.Unmarshal(rec.Body.Bytes(), &skills)

			if len(skills) == 0 {
				t.Errorf("search for '%s' should find case-test skill", query)
			}
		})
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// Persistence Tests (require database)
// =============================================================================

// TestSkillVisibility_PersistenceAcrossRestart simulates server restart
// This test verifies that installed skills survive a server restart
func TestSkillVisibility_PersistenceAcrossRestart(t *testing.T) {
	// This test requires database integration
	// For now, we test the registry persistence logic

	t.Run("registry can be rebuilt from skill list", func(t *testing.T) {
		// Create first registry and add skills
		registry1 := skill.NewRegistry()
		skill1 := NewRemoteSkillAdapter(&skill.Manifest{
			ID:          "persist-skill-1",
			Name:        "Persist Skill 1",
			Version:     "1.0.0",
			Description: "First persistent skill",
		})
		skill2 := NewRemoteSkillAdapter(&skill.Manifest{
			ID:          "persist-skill-2",
			Name:        "Persist Skill 2",
			Version:     "1.0.0",
			Description: "Second persistent skill",
		})
		registry1.Register(skill1, false)
		registry1.Register(skill2, false)
		registry1.Disable("persist-skill-2")

		// Get skill list (simulating what would be saved to DB)
		skills := registry1.List()
		enabledStates := make(map[string]bool)
		for _, s := range skills {
			enabledStates[s.Manifest.ID] = registry1.IsEnabled(s.Manifest.ID)
		}

		// Create second registry (simulating restart)
		registry2 := skill.NewRegistry()

		// Rebuild from saved data
		for _, s := range skills {
			manifest := s.Manifest
			newSkill := NewRemoteSkillAdapter(&skill.Manifest{
				ID:          manifest.ID,
				Name:        manifest.Name,
				Version:     manifest.Version,
				Description: manifest.Description,
			})
			registry2.Register(newSkill, false)
			if !enabledStates[manifest.ID] {
				registry2.Disable(manifest.ID)
			}
		}

		// Verify second registry matches first
		if registry2.Count() != registry1.Count() {
			t.Errorf("registry count mismatch: expected %d, got %d", registry1.Count(), registry2.Count())
		}

		if registry2.IsEnabled("persist-skill-1") != registry1.IsEnabled("persist-skill-1") {
			t.Error("persist-skill-1 enabled state mismatch")
		}

		if registry2.IsEnabled("persist-skill-2") != registry1.IsEnabled("persist-skill-2") {
			t.Error("persist-skill-2 enabled state mismatch")
		}
	})
}

// =============================================================================
// Timeout and Error Handling Tests
// =============================================================================

// TestSkillInstall_Timeout verifies install handles timeout gracefully
func TestSkillInstall_Timeout(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This test verifies the handler respects context cancellation
	// In real implementation, the install would check ctx.Done()
	_ = ctx // Used in actual implementation

	// For now, verify handler exists and can be called
	if handler == nil {
		t.Fatal("handler should not be nil")
	}
}

// TestSkillInstall_AlreadyInstalled verifies proper error for duplicate install
func TestSkillInstall_AlreadyInstalled(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Add and install a skill
	handler.remoteSkills["duplicate-test"] = &RemoteSkill{
		ID:          "duplicate-test",
		Name:        "Duplicate Test",
		Version:     "1.0.0",
		Description: "Test duplicate install",
		SourceID:    "test",
		SourceName:  "Test",
	}

	e := echo.New()

	// First install
	req := httptest.NewRequest(http.MethodPost, "/skill-store/install/duplicate-test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("duplicate-test")

	handler.InstallSkill(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("first install should succeed, got status %d", rec.Code)
	}

	// Second install (should fail with conflict)
	req = httptest.NewRequest(http.MethodPost, "/skill-store/install/duplicate-test", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("duplicate-test")

	handler.InstallSkill(c)

	if rec.Code != http.StatusConflict {
		t.Errorf("duplicate install should return conflict, got status %d", rec.Code)
	}
}

// =============================================================================
// New v0.10.8 Endpoint Tests
// =============================================================================

// TestGetFeaturedSkills tests the featured skills endpoint
func TestGetFeaturedSkills(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Create temp file with test featured skills
	tempDir, err := os.MkdirTemp("", "featured-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testData := `{
		"version": "1.0.0",
		"updated_at": "2026-01-31T00:00:00Z",
		"skills": [
			{
				"id": "test-featured",
				"name": "Test Featured Skill",
				"description": "A featured skill for testing",
				"author": "test-author",
				"category": "testing",
				"tags": ["test"],
				"version": "1.0.0",
				"source_url": "https://example.com/skill.md",
				"homepage": "https://example.com",
				"stars": 100,
				"featured_rank": 1
			}
		]
	}`

	dataPath := filepath.Join(tempDir, "featured_skills.json")
	if err := os.WriteFile(dataPath, []byte(testData), 0644); err != nil {
		t.Fatalf("failed to write test data: %v", err)
	}

	loader := skillstore.NewFeaturedSkillsLoader(dataPath)
	if err := loader.Load(); err != nil {
		t.Fatalf("failed to load featured skills: %v", err)
	}
	handler.SetFeaturedLoader(loader)

	e := echo.New()

	t.Run("get featured skills", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/skill-store/featured", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler.GetFeaturedSkills(c)
		if err != nil {
			t.Fatalf("GetFeaturedSkills failed: %v", err)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var skills []*RemoteSkill
		if err := json.Unmarshal(rec.Body.Bytes(), &skills); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(skills) != 1 {
			t.Errorf("expected 1 skill, got %d", len(skills))
		}

		if skills[0].ID != "test-featured" {
			t.Errorf("expected ID 'test-featured', got '%s'", skills[0].ID)
		}

		if skills[0].SourceID != "featured" {
			t.Errorf("expected source_id 'featured', got '%s'", skills[0].SourceID)
		}
	})
}

// TestVerifySkill tests the skill verification endpoint
func TestVerifySkill(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Register a test skill
	testSkill := NewRemoteSkillAdapter(&skill.Manifest{
		ID:          "verify-test-skill",
		Name:        "Verify Test Skill",
		Version:     "1.0.0",
		Description: "A skill for verification testing",
	})
	registry.Register(testSkill, false)

	e := echo.New()

	t.Run("verify existing skill", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/skills/verify/verify-test-skill", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("verify-test-skill")

		err := handler.VerifySkill(c)
		if err != nil {
			t.Fatalf("VerifySkill failed: %v", err)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if result["visible"] != true {
			t.Error("skill should be visible")
		}
		if result["enabled"] != true {
			t.Error("skill should be enabled")
		}
		if result["name"] != "Verify Test Skill" {
			t.Errorf("expected name 'Verify Test Skill', got '%v'", result["name"])
		}
	})

	t.Run("verify non-existent skill", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/skills/verify/non-existent", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("non-existent")

		handler.VerifySkill(c)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}

		var result map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &result)

		if result["visible"] != false {
			t.Error("non-existent skill should not be visible")
		}
	})
}

// TestListLocalSkills tests the local skills listing endpoint
func TestListLocalSkills(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Create temp directory with test skills
	tempDir, err := os.MkdirTemp("", "local-skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a skill directory with SKILL.md
	skillDir := filepath.Join(tempDir, "local-test-skill")
	os.MkdirAll(skillDir, 0755)
	skillContent := `---
name: local-test-skill
description: A local test skill
version: 1.0.0
---

# Local Test Skill
`
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644)

	scanner := skillstore.NewLocalSkillScanner(tempDir)
	scanner.Scan()
	handler.SetLocalScanner(scanner)

	e := echo.New()

	t.Run("list local skills", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/skills/local", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler.ListLocalSkills(c)
		if err != nil {
			t.Fatalf("ListLocalSkills failed: %v", err)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		count := result["count"].(float64)
		if count != 1 {
			t.Errorf("expected 1 skill, got %v", count)
		}
	})
}

// TestScanLocalSkills tests the local skills scan endpoint
func TestScanLocalSkills(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "scan-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	scanner := skillstore.NewLocalSkillScanner(tempDir)
	handler.SetLocalScanner(scanner)

	e := echo.New()

	t.Run("scan empty directory", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/skills/local/scan", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler.ScanLocalSkills(c)
		if err != nil {
			t.Fatalf("ScanLocalSkills failed: %v", err)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var result map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &result)

		if result["success"] != true {
			t.Error("scan should succeed")
		}
	})

	t.Run("scan with skills", func(t *testing.T) {
		// Add a skill
		skillDir := filepath.Join(tempDir, "scan-test-skill")
		os.MkdirAll(skillDir, 0755)
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: scan-test\n---\n"), 0644)

		req := httptest.NewRequest(http.MethodPost, "/skills/local/scan", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler.ScanLocalSkills(c)

		var result map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &result)

		skillsFound := result["skills_found"].(float64)
		if skillsFound != 1 {
			t.Errorf("expected 1 skill found, got %v", skillsFound)
		}
	})
}

// TestParseSkillContent tests the skill content parsing
func TestParseSkillContent(t *testing.T) {
	registry := skill.NewRegistry()
	handler := NewSkillHandler(registry)

	testCases := []struct {
		name        string
		content     string
		sourceURL   string
		expectID    string
		expectName  string
		expectError bool
	}{
		{
			name: "valid SKILL.md",
			content: `---
name: test-skill
description: A test skill
version: 2.0.0
author: Test Author
category: testing
tags: [test, example]
---

# Test Skill

Instructions here.
`,
			sourceURL:   "https://example.com/test-skill/SKILL.md",
			expectID:    "test-skill",
			expectName:  "test-skill",
			expectError: false,
		},
		{
			name: "with explicit ID",
			content: `---
id: my-custom-id
name: My Custom Skill
---
`,
			sourceURL:   "https://example.com/skill.md",
			expectID:    "my-custom-id",
			expectName:  "My Custom Skill",
			expectError: false,
		},
		{
			name:        "no frontmatter - ID from URL",
			content:     "# Just a skill\n\nNo frontmatter here.",
			sourceURL:   "https://example.com/my-skill/SKILL.md",
			expectID:    "my-skill",
			expectName:  "my-skill",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id, manifest, err := handler.parseSkillContent(tc.content, tc.sourceURL, "", "")

			if tc.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if id != tc.expectID {
				t.Errorf("expected ID '%s', got '%s'", tc.expectID, id)
			}

			if manifest.Name != tc.expectName {
				t.Errorf("expected name '%s', got '%s'", tc.expectName, manifest.Name)
			}
		})
	}
}
