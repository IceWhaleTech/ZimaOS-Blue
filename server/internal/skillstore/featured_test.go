package skillstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFeaturedSkillsLoader_Load(t *testing.T) {
	// Create temp file with test data
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
				"id": "test-skill",
				"name": "Test Skill",
				"description": "A test skill",
				"author": "test-author",
				"category": "testing",
				"tags": ["test", "example"],
				"version": "1.0.0",
				"source_url": "https://example.com/skill.md",
				"homepage": "https://example.com",
				"stars": 100,
				"featured_rank": 1
			},
			{
				"id": "another-skill",
				"name": "Another Skill",
				"description": "Another test skill",
				"author": "test-author",
				"category": "development",
				"tags": ["dev", "code"],
				"version": "2.0.0",
				"source_url": "https://example.com/another.md",
				"homepage": "https://example.com/another",
				"stars": 50,
				"featured_rank": 2
			}
		]
	}`

	dataPath := filepath.Join(tempDir, "featured_skills.json")
	if err := os.WriteFile(dataPath, []byte(testData), 0644); err != nil {
		t.Fatalf("failed to write test data: %v", err)
	}

	loader := NewFeaturedSkillsLoader(dataPath)

	t.Run("load featured skills", func(t *testing.T) {
		err := loader.Load()
		if err != nil {
			t.Fatalf("failed to load featured skills: %v", err)
		}

		if !loader.IsLoaded() {
			t.Error("loader should be marked as loaded")
		}

		if loader.Count() != 2 {
			t.Errorf("expected 2 skills, got %d", loader.Count())
		}
	})

	t.Run("get all skills", func(t *testing.T) {
		skills := loader.GetAll()
		if len(skills) != 2 {
			t.Errorf("expected 2 skills, got %d", len(skills))
		}
	})

	t.Run("get skill by ID", func(t *testing.T) {
		skill := loader.Get("test-skill")
		if skill == nil {
			t.Fatal("expected to find test-skill")
		}
		if skill.Name != "Test Skill" {
			t.Errorf("expected name 'Test Skill', got '%s'", skill.Name)
		}
		if skill.Stars != 100 {
			t.Errorf("expected stars 100, got %d", skill.Stars)
		}
	})

	t.Run("get non-existent skill", func(t *testing.T) {
		skill := loader.Get("non-existent")
		if skill != nil {
			t.Error("expected nil for non-existent skill")
		}
	})

	t.Run("search skills", func(t *testing.T) {
		results := loader.Search("test")
		if len(results) != 2 {
			t.Errorf("expected 2 results for 'test', got %d", len(results))
		}

		results = loader.Search("another")
		if len(results) != 1 {
			t.Errorf("expected 1 result for 'another', got %d", len(results))
		}
	})

	t.Run("search by tag", func(t *testing.T) {
		results := loader.Search("dev")
		if len(results) != 1 {
			t.Errorf("expected 1 result for tag 'dev', got %d", len(results))
		}
	})

	t.Run("get by category", func(t *testing.T) {
		results := loader.GetByCategory("testing")
		if len(results) != 1 {
			t.Errorf("expected 1 skill in 'testing' category, got %d", len(results))
		}
	})

	t.Run("get categories", func(t *testing.T) {
		categories := loader.GetCategories()
		if len(categories) != 2 {
			t.Errorf("expected 2 categories, got %d", len(categories))
		}
	})
}

func TestFeaturedSkillsLoader_LoadError(t *testing.T) {
	loader := NewFeaturedSkillsLoader("/non/existent/path.json")

	err := loader.Load()
	if err == nil {
		t.Error("expected error for non-existent file")
	}

	if loader.IsLoaded() {
		t.Error("loader should not be marked as loaded after error")
	}
}

func TestLocalSkillScanner_Scan(t *testing.T) {
	// Create temp directory with test skills
	tempDir, err := os.MkdirTemp("", "local-skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a skill directory with SKILL.md
	skillDir := filepath.Join(tempDir, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}

	skillContent := `---
name: my-skill
description: A local test skill
version: 1.0.0
author: Local Author
category: testing
tags: [local, test]
---

# My Skill

This is a local skill for testing.
`
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Create another skill
	skill2Dir := filepath.Join(tempDir, "another-skill")
	if err := os.MkdirAll(skill2Dir, 0755); err != nil {
		t.Fatalf("failed to create skill2 dir: %v", err)
	}

	skill2Content := `---
name: another-skill
description: Another local skill
version: 2.0.0
---

# Another Skill
`
	skill2Path := filepath.Join(skill2Dir, "SKILL.md")
	if err := os.WriteFile(skill2Path, []byte(skill2Content), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	scanner := NewLocalSkillScanner(tempDir)

	t.Run("scan local skills", func(t *testing.T) {
		err := scanner.Scan()
		if err != nil {
			t.Fatalf("failed to scan: %v", err)
		}

		if scanner.Count() != 2 {
			t.Errorf("expected 2 skills, got %d", scanner.Count())
		}
	})

	t.Run("get all local skills", func(t *testing.T) {
		skills := scanner.GetAll()
		if len(skills) != 2 {
			t.Errorf("expected 2 skills, got %d", len(skills))
		}
	})

	t.Run("get local skill by ID", func(t *testing.T) {
		skill := scanner.Get("my-skill")
		if skill == nil {
			t.Fatal("expected to find my-skill")
		}
		if skill.Name != "my-skill" {
			t.Errorf("expected name 'my-skill', got '%s'", skill.Name)
		}
		if skill.Description != "A local test skill" {
			t.Errorf("expected description 'A local test skill', got '%s'", skill.Description)
		}
		if skill.Version != "1.0.0" {
			t.Errorf("expected version '1.0.0', got '%s'", skill.Version)
		}
		if skill.Author != "Local Author" {
			t.Errorf("expected author 'Local Author', got '%s'", skill.Author)
		}
		if len(skill.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(skill.Tags))
		}
	})

	t.Run("skill content is parsed", func(t *testing.T) {
		skill := scanner.Get("my-skill")
		if skill == nil {
			t.Fatal("expected to find my-skill")
		}
		if skill.Content == "" {
			t.Error("skill content should not be empty")
		}
		if skill.FilePath == "" {
			t.Error("skill file path should not be empty")
		}
	})
}

func TestLocalSkillScanner_EmptyDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "empty-skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	scanner := NewLocalSkillScanner(tempDir)

	err = scanner.Scan()
	if err != nil {
		t.Fatalf("scan should not fail on empty directory: %v", err)
	}

	if scanner.Count() != 0 {
		t.Errorf("expected 0 skills in empty directory, got %d", scanner.Count())
	}
}

func TestLocalSkillScanner_NonExistentDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nonexistent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	os.RemoveAll(tempDir) // Remove it immediately

	scanner := NewLocalSkillScanner(tempDir)

	// Should create the directory
	err = scanner.Scan()
	if err != nil {
		t.Fatalf("scan should create non-existent directory: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Error("directory should have been created")
	}

	// Cleanup
	os.RemoveAll(tempDir)
}

func TestMatchesFeaturedSkill(t *testing.T) {
	skill := &FeaturedSkill{
		ID:          "test-skill",
		Name:        "Test Skill",
		Description: "A skill for testing purposes",
		Author:      "Test Author",
		Category:    "testing",
		Tags:        []string{"test", "example", "demo"},
	}

	testCases := []struct {
		query    string
		expected bool
	}{
		{"test", true},
		{"TEST", true},
		{"skill", true},
		{"testing", true},
		{"author", true},
		{"example", true},
		{"demo", true},
		{"nonexistent", false},
		{"xyz", false},
		{"", false}, // Empty query handled separately
	}

	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			result := matchesFeaturedSkill(skill, tc.query)
			if result != tc.expected {
				t.Errorf("matchesFeaturedSkill(%q) = %v, want %v", tc.query, result, tc.expected)
			}
		})
	}
}
