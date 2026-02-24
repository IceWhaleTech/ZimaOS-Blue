package builtin

import (
	"context"
	"testing"
)

func TestRegisterAll(t *testing.T) {
	registry := &mockRegistry{skills: make(map[string]bool)}

	// Verify the built-in skills can be created (calculator, system_info, datetime
	// have been migrated to native tools in tools/builtin.go)
	skills := []interface{}{
		// Productivity (4)
		NewTimer(),
		NewPushNotification(),
		NewNotes(),
		NewTasks(),
		// Utility (4)
		NewSearch(),
		NewTranslate(),
		NewNotifications(),
		NewUnitConverter(),
		// System (9)
		NewFiles(""),
		NewNetwork(),
		NewProcesses(),
		NewDocker(nil),
		NewScheduler(),
		NewWorkflows(),
		NewAutoReply(),
		NewSandbox(),
		NewBrowser(),
		// Communication (3)
		NewEmail(nil),
		NewCalendar(nil),
		NewContacts(nil),
		// Integration (4)
		NewGitHub(nil),
		NewNotion(nil),
		NewSlackSkill(nil),
		NewDiscordSkill(nil),
	}

	expected := GetSkillCount()
	if len(skills) != expected {
		t.Errorf("expected %d built-in skills, got %d", expected, len(skills))
	}

	_ = registry // Use registry to avoid unused variable error
}

type mockRegistry struct {
	skills map[string]bool
}

func TestWeather(t *testing.T) {
	weather := NewWeather(nil)

	t.Run("manifest", func(t *testing.T) {
		m := weather.Manifest()
		if m.ID != "weather" {
			t.Errorf("expected ID 'weather', got '%s'", m.ID)
		}
		if m.Name != "Weather" {
			t.Errorf("expected name 'Weather', got '%s'", m.Name)
		}
	})

	t.Run("validate missing location", func(t *testing.T) {
		err := weather.Validate(map[string]any{})
		if err == nil {
			t.Error("expected error for missing location")
		}
	})

	t.Run("validate invalid location type", func(t *testing.T) {
		err := weather.Validate(map[string]any{"location": 123})
		if err == nil {
			t.Error("expected error for invalid location type")
		}
	})

	t.Run("validate invalid units", func(t *testing.T) {
		err := weather.Validate(map[string]any{
			"location": "London",
			"units":    "invalid",
		})
		if err == nil {
			t.Error("expected error for invalid units")
		}
	})

	t.Run("get mock weather metric", func(t *testing.T) {
		result, err := weather.Execute(context.Background(), map[string]any{
			"location": "London",
			"units":    "metric",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		if !result.Success {
			t.Errorf("expected success, got error: %s", result.Error)
		}
		data := result.Data.(map[string]any)
		if data["location"] == nil {
			t.Error("expected location info")
		}
		if data["temperature"] == nil {
			t.Error("expected temperature info")
		}
		if data["mock"] != true {
			t.Error("expected mock data flag")
		}
	})

	t.Run("get mock weather imperial", func(t *testing.T) {
		result, err := weather.Execute(context.Background(), map[string]any{
			"location": "New York",
			"units":    "imperial",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		temp := data["temperature"].(map[string]any)
		if temp["unit"] != "°F" {
			t.Errorf("expected unit '°F', got '%v'", temp["unit"])
		}
	})
}

func TestSearch(t *testing.T) {
	search := NewSearch()

	t.Run("manifest", func(t *testing.T) {
		m := search.Manifest()
		if m.ID != "search" {
			t.Errorf("expected ID 'search', got '%s'", m.ID)
		}
	})

	t.Run("validate missing query", func(t *testing.T) {
		err := search.Validate(map[string]any{
			"content": "test content",
		})
		if err == nil {
			t.Error("expected error for missing query")
		}
	})

	t.Run("validate missing content", func(t *testing.T) {
		err := search.Validate(map[string]any{
			"query": "test",
		})
		if err == nil {
			t.Error("expected error for missing content")
		}
	})

	t.Run("validate invalid mode", func(t *testing.T) {
		err := search.Validate(map[string]any{
			"query":   "test",
			"content": "test content",
			"mode":    "invalid",
		})
		if err == nil {
			t.Error("expected error for invalid mode")
		}
	})

	content := `Line 1: Hello World
Line 2: This is a test
Line 3: Hello again
Line 4: Testing 123
Line 5: Final line`

	t.Run("search contains", func(t *testing.T) {
		result, err := search.Execute(context.Background(), map[string]any{
			"query":   "Hello",
			"content": content,
			"mode":    "contains",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		if !result.Success {
			t.Errorf("expected success, got error: %s", result.Error)
		}
		data := result.Data.(map[string]any)
		count := data["count"].(int)
		if count != 2 {
			t.Errorf("expected 2 matches, got %d", count)
		}
	})

	t.Run("search contains case insensitive", func(t *testing.T) {
		result, err := search.Execute(context.Background(), map[string]any{
			"query":          "hello",
			"content":        content,
			"mode":           "contains",
			"case_sensitive": false,
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		count := data["count"].(int)
		if count != 2 {
			t.Errorf("expected 2 matches, got %d", count)
		}
	})

	t.Run("search contains case sensitive", func(t *testing.T) {
		result, err := search.Execute(context.Background(), map[string]any{
			"query":          "hello",
			"content":        content,
			"mode":           "contains",
			"case_sensitive": true,
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		count := data["count"].(int)
		if count != 0 {
			t.Errorf("expected 0 matches (case sensitive), got %d", count)
		}
	})

	t.Run("search regex", func(t *testing.T) {
		result, err := search.Execute(context.Background(), map[string]any{
			"query":   "Line \\d+:",
			"content": content,
			"mode":    "regex",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		count := data["count"].(int)
		if count != 5 {
			t.Errorf("expected 5 matches, got %d", count)
		}
	})

	t.Run("search regex invalid pattern", func(t *testing.T) {
		result, err := search.Execute(context.Background(), map[string]any{
			"query":   "[invalid",
			"content": content,
			"mode":    "regex",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		if result.Success {
			t.Error("expected failure for invalid regex")
		}
	})

	t.Run("search fuzzy", func(t *testing.T) {
		result, err := search.Execute(context.Background(), map[string]any{
			"query":   "test",
			"content": content,
			"mode":    "fuzzy",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		count := data["count"].(int)
		if count == 0 {
			t.Error("expected at least 1 fuzzy match")
		}
	})

	t.Run("search with max results", func(t *testing.T) {
		result, err := search.Execute(context.Background(), map[string]any{
			"query":       "Line",
			"content":     content,
			"mode":        "contains",
			"max_results": 2,
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		count := data["count"].(int)
		if count != 2 {
			t.Errorf("expected 2 results (max), got %d", count)
		}
	})
}
