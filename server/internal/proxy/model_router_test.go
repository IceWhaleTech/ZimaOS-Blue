package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewModelRouter(t *testing.T) {
	// Test with nil config (should use defaults)
	mr, err := NewModelRouter(nil)
	if err != nil {
		t.Fatalf("NewModelRouter(nil) failed: %v", err)
	}
	if mr == nil {
		t.Fatal("NewModelRouter(nil) returned nil")
	}
	if !mr.config.Enabled {
		t.Error("Default config should have Enabled=true")
	}
	if len(mr.config.Families) == 0 {
		t.Error("Default config should have families")
	}
}

func TestNewModelRouterWithInvalidRegex(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled: true,
		RegexCustomRules: []*RegexRule{
			{
				Pattern: "[invalid",
				Target:  "test",
			},
		},
	}
	_, err := NewModelRouter(config)
	if err == nil {
		t.Error("Expected error for invalid regex pattern")
	}
}

func TestRouteModel_RegexRules(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled: true,
		RegexCustomRules: []*RegexRule{
			{
				Pattern:     "^claude-3-opus-latest$",
				Target:      "claude-3-opus-20240229",
				Provider:    "anthropic",
				Priority:    1,
				Description: "Pin opus-latest",
			},
		},
		Families: []*ModelFamily{
			{
				Name:     "claude-3",
				Patterns: []string{"^claude-3.*"},
				Provider: "anthropic",
				Fallback: "claude-3-haiku",
			},
		},
	}

	mr, err := NewModelRouter(config)
	if err != nil {
		t.Fatalf("NewModelRouter failed: %v", err)
	}

	// Test regex rule match
	route, err := mr.RouteModel("claude-3-opus-latest", false)
	if err != nil {
		t.Fatalf("RouteModel failed: %v", err)
	}
	if route.TargetModel != "claude-3-opus-20240229" {
		t.Errorf("Expected target 'claude-3-opus-20240229', got '%s'", route.TargetModel)
	}
	if route.RuleApplied != "Pin opus-latest" {
		t.Errorf("Expected rule 'Pin opus-latest', got '%s'", route.RuleApplied)
	}
}

func TestRouteModel_FamilyMatch(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled: true,
		Families: []*ModelFamily{
			{
				Name:     "claude-3",
				Patterns: []string{"^claude-3.*"},
				Provider: "anthropic",
				Fallback: "claude-3-haiku",
			},
			{
				Name:     "gpt-4",
				Patterns: []string{"^gpt-4.*"},
				Provider: "openai",
				Fallback: "gpt-4o-mini",
			},
		},
	}

	mr, err := NewModelRouter(config)
	if err != nil {
		t.Fatalf("NewModelRouter failed: %v", err)
	}

	tests := []struct {
		model    string
		provider string
		family   string
	}{
		{"claude-3-opus", "anthropic", "claude-3"},
		{"claude-3-sonnet", "anthropic", "claude-3"},
		{"gpt-4-turbo", "openai", "gpt-4"},
		{"gpt-4o", "openai", "gpt-4"},
	}

	for _, tt := range tests {
		route, err := mr.RouteModel(tt.model, false)
		if err != nil {
			t.Errorf("RouteModel(%s) failed: %v", tt.model, err)
			continue
		}
		if route.Provider != tt.provider {
			t.Errorf("RouteModel(%s): expected provider '%s', got '%s'", tt.model, tt.provider, route.Provider)
		}
		if route.Family != tt.family {
			t.Errorf("RouteModel(%s): expected family '%s', got '%s'", tt.model, tt.family, route.Family)
		}
	}
}

func TestRouteModel_BackgroundDowngrade(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled: true,
		Families: []*ModelFamily{
			{
				Name:     "claude-3",
				Patterns: []string{"^claude-3.*"},
				Provider: "anthropic",
				Fallback: "claude-3-haiku",
			},
		},
	}

	mr, err := NewModelRouter(config)
	if err != nil {
		t.Fatalf("NewModelRouter failed: %v", err)
	}

	// Normal request - no downgrade
	route, _ := mr.RouteModel("claude-3-opus", false)
	if route.TargetModel != "claude-3-opus" {
		t.Errorf("Normal request should not downgrade, got '%s'", route.TargetModel)
	}
	if route.Downgraded {
		t.Error("Normal request should not be marked as downgraded")
	}

	// Background request - should downgrade
	route, _ = mr.RouteModel("claude-3-opus", true)
	if route.TargetModel != "claude-3-haiku" {
		t.Errorf("Background request should downgrade to 'claude-3-haiku', got '%s'", route.TargetModel)
	}
	if !route.Downgraded {
		t.Error("Background request should be marked as downgraded")
	}
}

func TestRouteModel_NoFallback(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled: true,
		Families: []*ModelFamily{
			{
				Name:     "gemini-flash",
				Patterns: []string{"^gemini-.*-flash.*"},
				Provider: "google",
				Fallback: "", // No fallback - already a flash model
			},
		},
	}

	mr, err := NewModelRouter(config)
	if err != nil {
		t.Fatalf("NewModelRouter failed: %v", err)
	}

	// Background request with no fallback - should not downgrade
	route, _ := mr.RouteModel("gemini-1.5-flash", true)
	if route.TargetModel != "gemini-1.5-flash" {
		t.Errorf("No fallback should keep original model, got '%s'", route.TargetModel)
	}
	if route.Downgraded {
		t.Error("No fallback should not be marked as downgraded")
	}
}

func TestRouteModel_Disabled(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled: false,
		Families: []*ModelFamily{
			{
				Name:     "claude-3",
				Patterns: []string{"^claude-3.*"},
				Provider: "anthropic",
				Fallback: "claude-3-haiku",
			},
		},
	}

	mr, err := NewModelRouter(config)
	if err != nil {
		t.Fatalf("NewModelRouter failed: %v", err)
	}

	// When disabled, should return model as-is
	route, _ := mr.RouteModel("claude-3-opus", false)
	if route.TargetModel != "claude-3-opus" {
		t.Errorf("Disabled router should return model as-is, got '%s'", route.TargetModel)
	}
	if route.Provider != "" {
		t.Errorf("Disabled router should not set provider, got '%s'", route.Provider)
	}
}

func TestRouteModel_NoMatch(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled: true,
		Families: []*ModelFamily{
			{
				Name:     "claude-3",
				Patterns: []string{"^claude-3.*"},
				Provider: "anthropic",
			},
		},
	}

	mr, err := NewModelRouter(config)
	if err != nil {
		t.Fatalf("NewModelRouter failed: %v", err)
	}

	// Unknown model - should return as-is
	route, _ := mr.RouteModel("unknown-model", false)
	if route.TargetModel != "unknown-model" {
		t.Errorf("Unknown model should return as-is, got '%s'", route.TargetModel)
	}
}

func TestIsBackgroundRequest(t *testing.T) {
	mr, _ := NewModelRouter(nil)

	tests := []struct {
		name       string
		headers    map[string]string
		path       string
		query      string
		isBackground bool
	}{
		{
			name:       "X-Background-Task header",
			headers:    map[string]string{"X-Background-Task": "true"},
			path:       "/v1/chat/completions",
			isBackground: true,
		},
		{
			name:       "X-Request-Type background",
			headers:    map[string]string{"X-Request-Type": "background"},
			path:       "/v1/chat/completions",
			isBackground: true,
		},
		{
			name:       "X-Request-Type async",
			headers:    map[string]string{"X-Request-Type": "async"},
			path:       "/v1/chat/completions",
			isBackground: true,
		},
		{
			name:       "Title endpoint",
			headers:    map[string]string{},
			path:       "/v1/title/generate",
			isBackground: true,
		},
		{
			name:       "Summarize endpoint",
			headers:    map[string]string{},
			path:       "/v1/summarize",
			isBackground: true,
		},
		{
			name:       "Embed endpoint",
			headers:    map[string]string{},
			path:       "/v1/embed",
			isBackground: true,
		},
		{
			name:       "Background query param",
			headers:    map[string]string{},
			path:       "/v1/chat/completions",
			query:      "background=true",
			isBackground: true,
		},
		{
			name:       "Normal request",
			headers:    map[string]string{},
			path:       "/v1/chat/completions",
			isBackground: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.path
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodPost, url, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := mr.IsBackgroundRequest(req)
			if result != tt.isBackground {
				t.Errorf("IsBackgroundRequest() = %v, want %v", result, tt.isBackground)
			}
		})
	}
}

func TestModelRouter_AddRule(t *testing.T) {
	mr, _ := NewModelRouter(&ModelRouterConfig{Enabled: true})

	rule := &RegexRule{
		Pattern:     "^test-model$",
		Target:      "target-model",
		Provider:    "test-provider",
		Priority:    1,
		Description: "Test rule",
	}

	err := mr.AddRule(rule)
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	// Verify rule was added
	route, _ := mr.RouteModel("test-model", false)
	if route.TargetModel != "target-model" {
		t.Errorf("Expected target 'target-model', got '%s'", route.TargetModel)
	}
}

func TestModelRouter_AddRuleInvalidRegex(t *testing.T) {
	mr, _ := NewModelRouter(&ModelRouterConfig{Enabled: true})

	rule := &RegexRule{
		Pattern: "[invalid",
		Target:  "target",
	}

	err := mr.AddRule(rule)
	if err == nil {
		t.Error("Expected error for invalid regex pattern")
	}
}

func TestModelRouter_RemoveRule(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled: true,
		RegexCustomRules: []*RegexRule{
			{
				Pattern: "^test-model$",
				Target:  "target-model",
			},
		},
	}

	mr, _ := NewModelRouter(config)

	// Remove the rule
	removed := mr.RemoveRule("^test-model$")
	if !removed {
		t.Error("RemoveRule should return true for existing rule")
	}

	// Verify rule was removed
	route, _ := mr.RouteModel("test-model", false)
	if route.TargetModel != "test-model" {
		t.Errorf("After removal, model should return as-is, got '%s'", route.TargetModel)
	}

	// Try to remove non-existent rule
	removed = mr.RemoveRule("non-existent")
	if removed {
		t.Error("RemoveRule should return false for non-existent rule")
	}
}

func TestModelRouter_GetFamilies(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled: true,
		Families: []*ModelFamily{
			{Name: "family1"},
			{Name: "family2"},
		},
	}

	mr, _ := NewModelRouter(config)
	families := mr.GetFamilies()

	if len(families) != 2 {
		t.Errorf("Expected 2 families, got %d", len(families))
	}
}

func TestModelRouter_GetRules(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled: true,
		RegexCustomRules: []*RegexRule{
			{Pattern: "^rule1$"},
			{Pattern: "^rule2$"},
		},
	}

	mr, _ := NewModelRouter(config)
	rules := mr.GetRules()

	if len(rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rules))
	}
}

func TestModelRouter_Stats(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled:       true,
		DefaultFamily: "claude-3",
		Families: []*ModelFamily{
			{Name: "claude-3", Provider: "anthropic"},
		},
		RegexCustomRules: []*RegexRule{
			{Pattern: "^test$"},
		},
		BackgroundModels: []string{"haiku", "mini"},
	}

	mr, _ := NewModelRouter(config)
	stats := mr.Stats()

	if stats["enabled"] != true {
		t.Error("Stats should show enabled=true")
	}
	if stats["default_family"] != "claude-3" {
		t.Error("Stats should show correct default_family")
	}
	if stats["regex_rules_count"] != 1 {
		t.Errorf("Stats should show 1 regex rule, got %v", stats["regex_rules_count"])
	}
}
