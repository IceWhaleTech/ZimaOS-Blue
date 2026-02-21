package builtin

import (
	"context"
	"testing"
)

func TestCalculator(t *testing.T) {
	calc := NewCalculator()

	t.Run("manifest", func(t *testing.T) {
		m := calc.Manifest()
		if m.ID != "calculator" {
			t.Errorf("expected ID 'calculator', got '%s'", m.ID)
		}
		if m.Name != "Calculator" {
			t.Errorf("expected name 'Calculator', got '%s'", m.Name)
		}
	})

	t.Run("validate missing expression", func(t *testing.T) {
		err := calc.Validate(map[string]any{})
		if err == nil {
			t.Error("expected error for missing expression")
		}
	})

	t.Run("validate invalid expression type", func(t *testing.T) {
		err := calc.Validate(map[string]any{"expression": 123})
		if err == nil {
			t.Error("expected error for invalid expression type")
		}
	})

	t.Run("addition", func(t *testing.T) {
		result, err := calc.Execute(context.Background(), map[string]any{
			"expression": "2 + 3",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		if !result.Success {
			t.Errorf("expected success, got error: %s", result.Error)
		}
		data := result.Data.(map[string]any)
		if data["result"] != float64(5) {
			t.Errorf("expected 5, got %v", data["result"])
		}
	})

	t.Run("subtraction", func(t *testing.T) {
		result, err := calc.Execute(context.Background(), map[string]any{
			"expression": "10 - 4",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		if data["result"] != float64(6) {
			t.Errorf("expected 6, got %v", data["result"])
		}
	})

	t.Run("multiplication", func(t *testing.T) {
		result, err := calc.Execute(context.Background(), map[string]any{
			"expression": "3 * 4",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		if data["result"] != float64(12) {
			t.Errorf("expected 12, got %v", data["result"])
		}
	})

	t.Run("division", func(t *testing.T) {
		result, err := calc.Execute(context.Background(), map[string]any{
			"expression": "20 / 4",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		if data["result"] != float64(5) {
			t.Errorf("expected 5, got %v", data["result"])
		}
	})

	t.Run("division by zero", func(t *testing.T) {
		result, err := calc.Execute(context.Background(), map[string]any{
			"expression": "10 / 0",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		if result.Success {
			t.Error("expected failure for division by zero")
		}
	})

	t.Run("complex expression", func(t *testing.T) {
		result, err := calc.Execute(context.Background(), map[string]any{
			"expression": "2 + 3 * 4",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		// Note: simple parser evaluates left to right, so 2+3*4 = 2+12 = 14
		if data["result"] != float64(14) {
			t.Errorf("expected 14, got %v", data["result"])
		}
	})
}

func TestSystemInfo(t *testing.T) {
	sysInfo := NewSystemInfo()

	t.Run("manifest", func(t *testing.T) {
		m := sysInfo.Manifest()
		if m.ID != "system-info" {
			t.Errorf("expected ID 'system-info', got '%s'", m.ID)
		}
	})

	t.Run("validate invalid type", func(t *testing.T) {
		err := sysInfo.Validate(map[string]any{"type": "invalid"})
		if err == nil {
			t.Error("expected error for invalid type")
		}
	})

	t.Run("get all info", func(t *testing.T) {
		result, err := sysInfo.Execute(context.Background(), map[string]any{
			"type": "all",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		if !result.Success {
			t.Errorf("expected success, got error: %s", result.Error)
		}
		data := result.Data.(map[string]any)
		if data["os"] == nil {
			t.Error("expected os info")
		}
		if data["hardware"] == nil {
			t.Error("expected hardware info")
		}
	})

	t.Run("get os info", func(t *testing.T) {
		result, err := sysInfo.Execute(context.Background(), map[string]any{
			"type": "os",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		if data["os"] == nil {
			t.Error("expected os info")
		}
	})

	t.Run("get memory info", func(t *testing.T) {
		result, err := sysInfo.Execute(context.Background(), map[string]any{
			"type": "memory",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		if data["memory"] == nil {
			t.Error("expected memory info")
		}
	})

	t.Run("get cpu info", func(t *testing.T) {
		result, err := sysInfo.Execute(context.Background(), map[string]any{
			"type": "cpu",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		if data["cpu"] == nil {
			t.Error("expected cpu info")
		}
	})
}

func TestDateTime(t *testing.T) {
	dt := NewDateTime()

	t.Run("manifest", func(t *testing.T) {
		m := dt.Manifest()
		if m.ID != "datetime" {
			t.Errorf("expected ID 'datetime', got '%s'", m.ID)
		}
	})

	t.Run("validate invalid format", func(t *testing.T) {
		err := dt.Validate(map[string]any{"format": "invalid"})
		if err == nil {
			t.Error("expected error for invalid format")
		}
	})

	t.Run("get datetime iso format", func(t *testing.T) {
		result, err := dt.Execute(context.Background(), map[string]any{
			"format": "iso",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		if !result.Success {
			t.Errorf("expected success, got error: %s", result.Error)
		}
		data := result.Data.(map[string]any)
		if data["year"] == nil {
			t.Error("expected year")
		}
		if data["formatted"] == nil {
			t.Error("expected formatted")
		}
	})

	t.Run("get datetime unix format", func(t *testing.T) {
		result, err := dt.Execute(context.Background(), map[string]any{
			"format": "unix",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		if data["unix"] == nil {
			t.Error("expected unix timestamp")
		}
	})

	t.Run("get datetime with timezone", func(t *testing.T) {
		result, err := dt.Execute(context.Background(), map[string]any{
			"timezone": "UTC",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		data := result.Data.(map[string]any)
		if data["timezone"] != "UTC" {
			t.Errorf("expected timezone 'UTC', got '%v'", data["timezone"])
		}
	})

	t.Run("invalid timezone", func(t *testing.T) {
		result, err := dt.Execute(context.Background(), map[string]any{
			"timezone": "Invalid/Timezone",
		})
		if err != nil {
			t.Fatalf("failed to execute: %v", err)
		}
		if result.Success {
			t.Error("expected failure for invalid timezone")
		}
	})
}

func TestRegisterAll(t *testing.T) {
	registry := &mockRegistry{skills: make(map[string]bool)}

	// We can't directly test RegisterAll without importing skill package
	// This test verifies the built-in skills can be created
	skills := []interface{}{
		// Original (5)
		NewCalculator(),
		NewSystemInfo(),
		NewDateTime(),
		NewWeather(nil),
		NewSearch(),
		// Productivity (4)
		NewTimer(),
		NewReminders(),
		NewNotes(),
		NewTasks(),
		// Utility (3)
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
		// Information (3)
		NewNews(nil),
		NewStocks(nil),
		NewCrypto(nil),
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
