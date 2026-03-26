package features

import (
	"testing"
)

func TestFeatureGate_IsEnabled(t *testing.T) {
	fg := NewFeatureGate()

	// Core features should be enabled without CLI
	if !fg.IsEnabled(FeatureBasicChat) {
		t.Error("FeatureBasicChat should be enabled without CLI")
	}

	if !fg.IsEnabled(FeatureProviderConfig) {
		t.Error("FeatureProviderConfig should be enabled without CLI")
	}

	// Runtime features should be enabled without any external CLI dependency.
	if !fg.IsEnabled(FeatureToolCalling) {
		t.Error("FeatureToolCalling should be enabled without CLI")
	}

	// Agent mode is now native and should be enabled without CLI
	if !fg.IsEnabled(FeatureAgentMode) {
		t.Error("FeatureAgentMode should be enabled without CLI")
	}

	fg.SetCLIInstalled(false)
	if !fg.IsEnabled(FeatureToolCalling) {
		t.Error("FeatureToolCalling should remain enabled even when legacy CLI flag is false")
	}
}

func TestFeatureGate_Override(t *testing.T) {
	fg := NewFeatureGate()

	// ToolCalling should be enabled by default.
	if !fg.IsEnabled(FeatureToolCalling) {
		t.Error("FeatureToolCalling should be enabled without CLI")
	}

	// Override to enable
	fg.SetOverride(FeatureToolCalling, true)
	if !fg.IsEnabled(FeatureToolCalling) {
		t.Error("FeatureToolCalling should be enabled with override")
	}

	// Clear override
	fg.ClearOverride(FeatureToolCalling)
	if !fg.IsEnabled(FeatureToolCalling) {
		t.Error("FeatureToolCalling should revert to enabled after clearing override")
	}

	// Override to disable a core feature
	fg.SetOverride(FeatureBasicChat, false)
	if fg.IsEnabled(FeatureBasicChat) {
		t.Error("FeatureBasicChat should be disabled with override")
	}
}

func TestFeatureGate_GetFeatureInfo(t *testing.T) {
	fg := NewFeatureGate()

	info := fg.GetFeatureInfo(FeatureToolCalling)
	if info == nil {
		t.Fatal("GetFeatureInfo returned nil")
	}

	if info.Name != "Tool Calling" {
		t.Errorf("Name = %q, want %q", info.Name, "Tool Calling")
	}

	if info.RequiresCLI {
		t.Error("RequiresCLI should be false")
	}

	if !info.Enabled {
		t.Error("Enabled should be true without CLI")
	}
}

func TestFeatureGate_GetAllFeatures(t *testing.T) {
	fg := NewFeatureGate()

	features := fg.GetAllFeatures()
	if len(features) == 0 {
		t.Error("GetAllFeatures returned empty map")
	}

	// Check that all expected features are present
	expectedFeatures := []Feature{
		FeatureBasicChat,
		FeatureToolCalling,
		FeatureAgentMode,
		FeatureUsageStats,
	}

	for _, f := range expectedFeatures {
		if _, ok := features[f]; !ok {
			t.Errorf("Feature %q not found in GetAllFeatures", f)
		}
	}
}

func TestFeatureGate_GetEnabledDisabledFeatures(t *testing.T) {
	fg := NewFeatureGate()

	enabled := fg.GetEnabledFeatures()
	disabled := fg.GetDisabledFeatures()

	// All runtime features should be enabled by default.
	if len(disabled) != 0 {
		t.Errorf("GetDisabledFeatures should be empty by default, got %d", len(disabled))
	}

	// Toggling the legacy compatibility flag should not disable runtime features.
	fg.SetCLIInstalled(false)

	enabled = fg.GetEnabledFeatures()
	disabled = fg.GetDisabledFeatures()

	if len(disabled) != 0 {
		t.Errorf("GetDisabledFeatures should remain empty, got %d", len(disabled))
	}

	if len(enabled) != len(fg.GetAllFeatures()) {
		t.Error("All features should be enabled with CLI")
	}
}

func TestFeatureGate_GetFeaturesByCategory(t *testing.T) {
	fg := NewFeatureGate()

	byCategory := fg.GetFeaturesByCategory()

	// Check expected categories
	expectedCategories := []string{"core", "cli", "stats", "advanced"}
	for _, cat := range expectedCategories {
		if _, ok := byCategory[cat]; !ok {
			t.Errorf("Category %q not found", cat)
		}
	}

	// Core category should have features
	if len(byCategory["core"]) == 0 {
		t.Error("Core category should have features")
	}
}

func TestFeatureGate_GetCLIDependentFeatures(t *testing.T) {
	fg := NewFeatureGate()

	cliFeatures := fg.GetCLIDependentFeatures()
	if len(cliFeatures) != 0 {
		t.Errorf("GetCLIDependentFeatures should be empty, got %d entries", len(cliFeatures))
	}
}

func TestFeatureGate_GetStatus(t *testing.T) {
	fg := NewFeatureGate()

	status := fg.GetStatus()
	if status == nil {
		t.Fatal("GetStatus returned nil")
	}

	if !status.CLIInstalled {
		t.Error("CLIInstalled should default to true for legacy compatibility")
	}

	if status.TotalFeatures == 0 {
		t.Error("TotalFeatures should not be 0")
	}

	if status.EnabledCount+status.DisabledCount != status.TotalFeatures {
		t.Error("EnabledCount + DisabledCount should equal TotalFeatures")
	}

	if len(status.DisabledReasons) != 0 {
		t.Errorf("DisabledReasons should be empty, got %d", len(status.DisabledReasons))
	}

	fg.SetCLIInstalled(false)
	status = fg.GetStatus()

	if status.CLIInstalled {
		t.Error("CLIInstalled should reflect the legacy flag when explicitly disabled")
	}

	if status.DisabledCount != 0 {
		t.Errorf("DisabledCount should stay 0, got %d", status.DisabledCount)
	}
}

func TestFeatureGate_RequireFeature(t *testing.T) {
	fg := NewFeatureGate()

	// Core feature should not return error
	err := fg.RequireFeature(FeatureBasicChat)
	if err != nil {
		t.Errorf("RequireFeature(BasicChat) should not return error, got %v", err)
	}

	// Runtime features should not require CLI anymore.
	err = fg.RequireFeature(FeatureToolCalling)
	if err != nil {
		t.Errorf("RequireFeature(ToolCalling) should not return error, got %v", err)
	}
}

func TestFeatureGate_UnknownFeature(t *testing.T) {
	fg := NewFeatureGate()

	// Unknown feature should return false
	if fg.IsEnabled("unknown_feature") {
		t.Error("Unknown feature should not be enabled")
	}

	// GetFeatureInfo should return nil for unknown feature
	info := fg.GetFeatureInfo("unknown_feature")
	if info != nil {
		t.Error("GetFeatureInfo should return nil for unknown feature")
	}
}
