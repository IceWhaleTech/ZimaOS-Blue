package config

import (
	"testing"
	"time"
)

func TestFlagEvaluator_IsEnabled_BasicFlag(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: true,
		Flags: []FeatureFlag{
			{
				Name:    "feature_a",
				Enabled: true,
				Percentage: 100,
			},
			{
				Name:    "feature_b",
				Enabled: false,
				Percentage: 100,
			},
		},
	}

	evaluator := NewFlagEvaluator(config)
	ctx := &EvaluationContext{UserID: "user1"}

	if !evaluator.IsEnabled("feature_a", ctx) {
		t.Error("feature_a should be enabled")
	}

	if evaluator.IsEnabled("feature_b", ctx) {
		t.Error("feature_b should be disabled")
	}

	if evaluator.IsEnabled("nonexistent", ctx) {
		t.Error("nonexistent flag should return false")
	}
}

func TestFlagEvaluator_IsEnabled_GrayscaleDisabled(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: false,
		Flags: []FeatureFlag{
			{
				Name:    "feature_a",
				Enabled: true,
				Percentage: 100,
			},
		},
	}

	evaluator := NewFlagEvaluator(config)
	ctx := &EvaluationContext{UserID: "user1"}

	if evaluator.IsEnabled("feature_a", ctx) {
		t.Error("feature_a should be disabled when grayscale is disabled")
	}
}

func TestFlagEvaluator_IsEnabled_UserTargeting(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: true,
		Flags: []FeatureFlag{
			{
				Name:    "beta_feature",
				Enabled: true,
				Users:   []string{"user1", "user2"},
			},
		},
	}

	evaluator := NewFlagEvaluator(config)

	// User in list
	ctx := &EvaluationContext{UserID: "user1"}
	if !evaluator.IsEnabled("beta_feature", ctx) {
		t.Error("beta_feature should be enabled for user1")
	}

	// User not in list
	ctx = &EvaluationContext{UserID: "user3"}
	if evaluator.IsEnabled("beta_feature", ctx) {
		t.Error("beta_feature should be disabled for user3")
	}
}

func TestFlagEvaluator_IsEnabled_GroupTargeting(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: true,
		Flags: []FeatureFlag{
			{
				Name:    "admin_feature",
				Enabled: true,
				Groups:  []string{"admin", "beta_testers"},
			},
		},
	}

	evaluator := NewFlagEvaluator(config)

	// User in admin group
	ctx := &EvaluationContext{
		UserID: "user1",
		Groups: []string{"admin"},
	}
	if !evaluator.IsEnabled("admin_feature", ctx) {
		t.Error("admin_feature should be enabled for admin group")
	}

	// User in beta_testers group
	ctx = &EvaluationContext{
		UserID: "user2",
		Groups: []string{"beta_testers"},
	}
	if !evaluator.IsEnabled("admin_feature", ctx) {
		t.Error("admin_feature should be enabled for beta_testers group")
	}

	// User not in any target group
	ctx = &EvaluationContext{
		UserID: "user3",
		Groups: []string{"regular"},
	}
	if evaluator.IsEnabled("admin_feature", ctx) {
		t.Error("admin_feature should be disabled for regular group")
	}
}

func TestFlagEvaluator_IsEnabled_PercentageRollout(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: true,
		Flags: []FeatureFlag{
			{
				Name:       "gradual_rollout",
				Enabled:    true,
				Percentage: 50,
			},
		},
	}

	evaluator := NewFlagEvaluator(config)

	// Test with many users to verify percentage distribution
	enabledCount := 0
	totalUsers := 1000

	for i := 0; i < totalUsers; i++ {
		ctx := &EvaluationContext{UserID: string(rune('a' + i%26)) + string(rune(i))}
		if evaluator.IsEnabled("gradual_rollout", ctx) {
			enabledCount++
		}
	}

	// Allow 10% tolerance
	expectedMin := int(float64(totalUsers) * 0.40)
	expectedMax := int(float64(totalUsers) * 0.60)

	if enabledCount < expectedMin || enabledCount > expectedMax {
		t.Errorf("Expected ~50%% enabled, got %d/%d (%.1f%%)", enabledCount, totalUsers, float64(enabledCount)/float64(totalUsers)*100)
	}
}

func TestFlagEvaluator_IsEnabled_PercentageConsistency(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: true,
		Flags: []FeatureFlag{
			{
				Name:       "consistent_flag",
				Enabled:    true,
				Percentage: 50,
			},
		},
	}

	evaluator := NewFlagEvaluator(config)
	ctx := &EvaluationContext{UserID: "consistent_user"}

	// Same user should always get same result
	firstResult := evaluator.IsEnabled("consistent_flag", ctx)
	for i := 0; i < 100; i++ {
		if evaluator.IsEnabled("consistent_flag", ctx) != firstResult {
			t.Error("Same user should always get consistent result")
		}
	}
}

func TestFlagEvaluator_IsEnabled_TargetingRules(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: true,
		Flags: []FeatureFlag{
			{
				Name:       "geo_feature",
				Enabled:    true,
				Percentage: 100,
				Rules: []TargetingRule{
					{
						Attribute: "country",
						Operator:  "in",
						Values:    []string{"US", "CA", "UK"},
					},
				},
			},
		},
	}

	evaluator := NewFlagEvaluator(config)

	// User in allowed country
	ctx := &EvaluationContext{
		UserID:     "user1",
		Attributes: map[string]string{"country": "US"},
	}
	if !evaluator.IsEnabled("geo_feature", ctx) {
		t.Error("geo_feature should be enabled for US users")
	}

	// User not in allowed country
	ctx = &EvaluationContext{
		UserID:     "user2",
		Attributes: map[string]string{"country": "FR"},
	}
	if evaluator.IsEnabled("geo_feature", ctx) {
		t.Error("geo_feature should be disabled for FR users")
	}
}

func TestFlagEvaluator_IsEnabled_RuleOperators(t *testing.T) {
	tests := []struct {
		name     string
		rule     TargetingRule
		attr     map[string]string
		expected bool
	}{
		{
			name:     "eq_match",
			rule:     TargetingRule{Attribute: "version", Operator: "eq", Values: []string{"2.0"}},
			attr:     map[string]string{"version": "2.0"},
			expected: true,
		},
		{
			name:     "eq_no_match",
			rule:     TargetingRule{Attribute: "version", Operator: "eq", Values: []string{"2.0"}},
			attr:     map[string]string{"version": "1.0"},
			expected: false,
		},
		{
			name:     "neq_match",
			rule:     TargetingRule{Attribute: "version", Operator: "neq", Values: []string{"1.0"}},
			attr:     map[string]string{"version": "2.0"},
			expected: true,
		},
		{
			name:     "neq_no_match",
			rule:     TargetingRule{Attribute: "version", Operator: "neq", Values: []string{"1.0"}},
			attr:     map[string]string{"version": "1.0"},
			expected: false,
		},
		{
			name:     "in_match",
			rule:     TargetingRule{Attribute: "platform", Operator: "in", Values: []string{"ios", "android"}},
			attr:     map[string]string{"platform": "ios"},
			expected: true,
		},
		{
			name:     "in_no_match",
			rule:     TargetingRule{Attribute: "platform", Operator: "in", Values: []string{"ios", "android"}},
			attr:     map[string]string{"platform": "web"},
			expected: false,
		},
		{
			name:     "not_in_match",
			rule:     TargetingRule{Attribute: "platform", Operator: "not_in", Values: []string{"ios", "android"}},
			attr:     map[string]string{"platform": "web"},
			expected: true,
		},
		{
			name:     "not_in_no_match",
			rule:     TargetingRule{Attribute: "platform", Operator: "not_in", Values: []string{"ios", "android"}},
			attr:     map[string]string{"platform": "ios"},
			expected: false,
		},
		{
			name:     "contains_match",
			rule:     TargetingRule{Attribute: "email", Operator: "contains", Values: []string{"@company.com"}},
			attr:     map[string]string{"email": "user@company.com"},
			expected: true,
		},
		{
			name:     "contains_no_match",
			rule:     TargetingRule{Attribute: "email", Operator: "contains", Values: []string{"@company.com"}},
			attr:     map[string]string{"email": "user@other.com"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &GrayscaleConfig{
				Enabled: true,
				Flags: []FeatureFlag{
					{
						Name:       "test_flag",
						Enabled:    true,
						Percentage: 100,
						Rules:      []TargetingRule{tt.rule},
					},
				},
			}

			evaluator := NewFlagEvaluator(config)
			ctx := &EvaluationContext{
				UserID:     "user1",
				Attributes: tt.attr,
			}

			result := evaluator.IsEnabled("test_flag", ctx)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFlagEvaluator_GetVariant(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: true,
		Flags: []FeatureFlag{
			{
				Name:         "button_color",
				Enabled:      true,
				Percentage:   100,
				DefaultValue: "blue",
				Variants: []FlagVariant{
					{Name: "red", Value: "red", Weight: 33.33},
					{Name: "green", Value: "green", Weight: 33.33},
					{Name: "blue", Value: "blue", Weight: 33.34},
				},
			},
		},
	}

	evaluator := NewFlagEvaluator(config)

	// Test variant distribution
	variantCounts := make(map[string]int)
	totalUsers := 1000

	for i := 0; i < totalUsers; i++ {
		ctx := &EvaluationContext{UserID: string(rune('a'+i%26)) + string(rune(i))}
		variant := evaluator.GetVariant("button_color", ctx)
		if v, ok := variant.(string); ok {
			variantCounts[v]++
		}
	}

	// Each variant should have roughly 33% of users
	for variant, count := range variantCounts {
		pct := float64(count) / float64(totalUsers) * 100
		if pct < 20 || pct > 46 {
			t.Errorf("Variant %s has unexpected distribution: %.1f%%", variant, pct)
		}
	}
}

func TestFlagEvaluator_GetVariant_Consistency(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: true,
		Flags: []FeatureFlag{
			{
				Name:       "experiment",
				Enabled:    true,
				Percentage: 100,
				Variants: []FlagVariant{
					{Name: "a", Value: "variant_a", Weight: 50},
					{Name: "b", Value: "variant_b", Weight: 50},
				},
			},
		},
	}

	evaluator := NewFlagEvaluator(config)
	ctx := &EvaluationContext{UserID: "consistent_user"}

	// Same user should always get same variant
	firstVariant := evaluator.GetVariant("experiment", ctx)
	for i := 0; i < 100; i++ {
		if evaluator.GetVariant("experiment", ctx) != firstVariant {
			t.Error("Same user should always get consistent variant")
		}
	}
}

func TestFlagEvaluator_GetABTestVariant(t *testing.T) {
	now := time.Now()
	config := &GrayscaleConfig{
		Enabled: true,
		ABTests: []ABTest{
			{
				Name:       "checkout_flow",
				Enabled:    true,
				StartTime:  now.Add(-time.Hour),
				EndTime:    now.Add(time.Hour),
				TrafficPct: 100,
				Variants: []ABVariant{
					{Name: "control", Value: "old_flow", Weight: 50, IsControl: true},
					{Name: "treatment", Value: "new_flow", Weight: 50, IsControl: false},
				},
			},
		},
	}

	evaluator := NewFlagEvaluator(config)

	// Test variant distribution
	variantCounts := make(map[string]int)
	totalUsers := 1000

	for i := 0; i < totalUsers; i++ {
		ctx := &EvaluationContext{UserID: string(rune('a'+i%26)) + string(rune(i))}
		variant := evaluator.GetABTestVariant("checkout_flow", ctx)
		if variant != nil {
			variantCounts[variant.Name]++
		}
	}

	// Each variant should have roughly 50% of users
	for variant, count := range variantCounts {
		pct := float64(count) / float64(totalUsers) * 100
		if pct < 40 || pct > 60 {
			t.Errorf("Variant %s has unexpected distribution: %.1f%%", variant, pct)
		}
	}
}

func TestFlagEvaluator_GetABTestVariant_TimeWindow(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
		expected  bool
	}{
		{
			name:      "active_test",
			startTime: now.Add(-time.Hour),
			endTime:   now.Add(time.Hour),
			expected:  true,
		},
		{
			name:      "not_started",
			startTime: now.Add(time.Hour),
			endTime:   now.Add(2 * time.Hour),
			expected:  false,
		},
		{
			name:      "ended",
			startTime: now.Add(-2 * time.Hour),
			endTime:   now.Add(-time.Hour),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &GrayscaleConfig{
				Enabled: true,
				ABTests: []ABTest{
					{
						Name:       "test",
						Enabled:    true,
						StartTime:  tt.startTime,
						EndTime:    tt.endTime,
						TrafficPct: 100,
						Variants: []ABVariant{
							{Name: "control", Value: "a", Weight: 100},
						},
					},
				},
			}

			evaluator := NewFlagEvaluator(config)
			ctx := &EvaluationContext{UserID: "user1"}
			variant := evaluator.GetABTestVariant("test", ctx)

			if tt.expected && variant == nil {
				t.Error("Expected variant, got nil")
			}
			if !tt.expected && variant != nil {
				t.Error("Expected nil, got variant")
			}
		})
	}
}

func TestFlagEvaluator_GetABTestVariant_TrafficPercentage(t *testing.T) {
	now := time.Now()
	config := &GrayscaleConfig{
		Enabled: true,
		ABTests: []ABTest{
			{
				Name:       "limited_test",
				Enabled:    true,
				StartTime:  now.Add(-time.Hour),
				EndTime:    now.Add(time.Hour),
				TrafficPct: 10, // Only 10% of traffic
				Variants: []ABVariant{
					{Name: "control", Value: "a", Weight: 100},
				},
			},
		},
	}

	evaluator := NewFlagEvaluator(config)

	inTestCount := 0
	totalUsers := 1000

	for i := 0; i < totalUsers; i++ {
		ctx := &EvaluationContext{UserID: string(rune('a'+i%26)) + string(rune(i))}
		if evaluator.GetABTestVariant("limited_test", ctx) != nil {
			inTestCount++
		}
	}

	// Should be roughly 10% in test
	pct := float64(inTestCount) / float64(totalUsers) * 100
	if pct < 5 || pct > 15 {
		t.Errorf("Expected ~10%% in test, got %.1f%%", pct)
	}
}

func TestFlagEvaluator_GetConfigVersion(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: true,
		Versions: []ConfigVersion{
			{
				Version:     "1.0.0",
				Description: "Initial version",
				Active:      false,
			},
			{
				Version:     "1.1.0",
				Description: "Current version",
				Active:      true,
				Values: map[string]interface{}{
					"max_connections": 100,
				},
			},
		},
	}

	evaluator := NewFlagEvaluator(config)
	version := evaluator.GetConfigVersion()

	if version == nil {
		t.Fatal("Expected active version, got nil")
	}

	if version.Version != "1.1.0" {
		t.Errorf("Expected version 1.1.0, got %s", version.Version)
	}

	if version.Values["max_connections"] != 100 {
		t.Error("Expected max_connections to be 100")
	}
}

func TestFlagEvaluator_UpdateConfig(t *testing.T) {
	config1 := &GrayscaleConfig{
		Enabled: true,
		Flags: []FeatureFlag{
			{Name: "feature_a", Enabled: true, Percentage: 100},
		},
	}

	config2 := &GrayscaleConfig{
		Enabled: true,
		Flags: []FeatureFlag{
			{Name: "feature_a", Enabled: false, Percentage: 100},
		},
	}

	evaluator := NewFlagEvaluator(config1)
	ctx := &EvaluationContext{UserID: "user1"}

	if !evaluator.IsEnabled("feature_a", ctx) {
		t.Error("feature_a should be enabled initially")
	}

	evaluator.UpdateConfig(config2)

	if evaluator.IsEnabled("feature_a", ctx) {
		t.Error("feature_a should be disabled after config update")
	}
}

func TestFlagEvaluator_ListFlags(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: true,
		Flags: []FeatureFlag{
			{Name: "feature_a", Enabled: true},
			{Name: "feature_b", Enabled: false},
		},
	}

	evaluator := NewFlagEvaluator(config)
	flags := evaluator.ListFlags()

	if len(flags) != 2 {
		t.Errorf("Expected 2 flags, got %d", len(flags))
	}
}

func TestFlagEvaluator_ListABTests(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: true,
		ABTests: []ABTest{
			{Name: "test_a", Enabled: true},
			{Name: "test_b", Enabled: false},
		},
	}

	evaluator := NewFlagEvaluator(config)
	tests := evaluator.ListABTests()

	if len(tests) != 2 {
		t.Errorf("Expected 2 tests, got %d", len(tests))
	}
}

func TestFlagEvaluator_NilContext(t *testing.T) {
	config := &GrayscaleConfig{
		Enabled: true,
		Flags: []FeatureFlag{
			{
				Name:       "percentage_flag",
				Enabled:    true,
				Percentage: 100,
			},
		},
	}

	evaluator := NewFlagEvaluator(config)

	// Should not panic with nil context
	result := evaluator.IsEnabled("percentage_flag", nil)
	if !result {
		t.Error("100% flag should be enabled even with nil context")
	}
}

func TestFlagEvaluator_NilConfig(t *testing.T) {
	evaluator := NewFlagEvaluator(nil)
	ctx := &EvaluationContext{UserID: "user1"}

	if evaluator.IsEnabled("any_flag", ctx) {
		t.Error("Should return false with nil config")
	}

	if evaluator.GetVariant("any_flag", ctx) != nil {
		t.Error("Should return nil with nil config")
	}

	if evaluator.GetABTestVariant("any_test", ctx) != nil {
		t.Error("Should return nil with nil config")
	}

	if evaluator.GetConfigVersion() != nil {
		t.Error("Should return nil with nil config")
	}
}
