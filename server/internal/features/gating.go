// Package features provides feature gating based on CLI installation status.
package features

import (
	"sync"
)

// Feature represents a feature that can be gated.
type Feature string

const (
	// Core features (always available)
	FeatureBasicChat       Feature = "basic_chat"
	FeatureProviderConfig  Feature = "provider_config"
	FeatureModelSelection  Feature = "model_selection"

	// CLI-dependent features
	FeatureToolCalling     Feature = "tool_calling"
	FeatureFileOperations  Feature = "file_operations"
	FeatureCodeExecution   Feature = "code_execution"
	FeatureAgentMode       Feature = "agent_mode"
	FeatureProjectContext  Feature = "project_context"
	FeatureMCPIntegration  Feature = "mcp_integration"

	// Statistics features
	FeatureUsageStats      Feature = "usage_stats"
	FeatureCostTracking    Feature = "cost_tracking"

	// Advanced features
	FeatureMultiProvider   Feature = "multi_provider"
	FeatureProviderPool    Feature = "provider_pool"
	FeatureAutoFallback    Feature = "auto_fallback"
)

// FeatureInfo describes a feature.
type FeatureInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RequiresCLI bool   `json:"requires_cli"`
	Enabled     bool   `json:"enabled"`
	Category    string `json:"category"`
}

// FeatureGate manages feature availability.
type FeatureGate struct {
	mu           sync.RWMutex
	cliInstalled bool
	overrides    map[Feature]bool
	features     map[Feature]*FeatureInfo
}

// NewFeatureGate creates a new feature gate.
func NewFeatureGate() *FeatureGate {
	fg := &FeatureGate{
		overrides: make(map[Feature]bool),
		features:  make(map[Feature]*FeatureInfo),
	}

	// Register all features
	fg.registerFeatures()

	return fg
}

// registerFeatures registers all known features.
func (fg *FeatureGate) registerFeatures() {
	// Core features (always available)
	fg.features[FeatureBasicChat] = &FeatureInfo{
		Name:        "Basic Chat",
		Description: "Basic chat functionality with LLM providers",
		RequiresCLI: false,
		Category:    "core",
	}
	fg.features[FeatureProviderConfig] = &FeatureInfo{
		Name:        "Provider Configuration",
		Description: "Configure and manage LLM providers",
		RequiresCLI: false,
		Category:    "core",
	}
	fg.features[FeatureModelSelection] = &FeatureInfo{
		Name:        "Model Selection",
		Description: "Select and switch between models",
		RequiresCLI: false,
		Category:    "core",
	}

	// CLI-dependent features
	fg.features[FeatureToolCalling] = &FeatureInfo{
		Name:        "Tool Calling",
		Description: "Execute tools and functions through the LLM",
		RequiresCLI: true,
		Category:    "cli",
	}
	fg.features[FeatureFileOperations] = &FeatureInfo{
		Name:        "File Operations",
		Description: "Read, write, and edit files",
		RequiresCLI: true,
		Category:    "cli",
	}
	fg.features[FeatureCodeExecution] = &FeatureInfo{
		Name:        "Code Execution",
		Description: "Execute code and shell commands",
		RequiresCLI: true,
		Category:    "cli",
	}
	fg.features[FeatureAgentMode] = &FeatureInfo{
		Name:        "Agent Mode",
		Description: "Autonomous agent capabilities",
		RequiresCLI: true,
		Category:    "cli",
	}
	fg.features[FeatureProjectContext] = &FeatureInfo{
		Name:        "Project Context",
		Description: "Understand and work with project structure",
		RequiresCLI: true,
		Category:    "cli",
	}
	fg.features[FeatureMCPIntegration] = &FeatureInfo{
		Name:        "MCP Integration",
		Description: "Model Context Protocol server integration",
		RequiresCLI: true,
		Category:    "cli",
	}

	// Statistics features
	fg.features[FeatureUsageStats] = &FeatureInfo{
		Name:        "Usage Statistics",
		Description: "Track API usage and performance metrics",
		RequiresCLI: false,
		Category:    "stats",
	}
	fg.features[FeatureCostTracking] = &FeatureInfo{
		Name:        "Cost Tracking",
		Description: "Estimate and track API costs",
		RequiresCLI: false,
		Category:    "stats",
	}

	// Advanced features
	fg.features[FeatureMultiProvider] = &FeatureInfo{
		Name:        "Multi-Provider",
		Description: "Use multiple LLM providers simultaneously",
		RequiresCLI: false,
		Category:    "advanced",
	}
	fg.features[FeatureProviderPool] = &FeatureInfo{
		Name:        "Provider Pool",
		Description: "Load balancing across provider instances",
		RequiresCLI: false,
		Category:    "advanced",
	}
	fg.features[FeatureAutoFallback] = &FeatureInfo{
		Name:        "Auto Fallback",
		Description: "Automatic fallback to alternative providers",
		RequiresCLI: false,
		Category:    "advanced",
	}
}

// SetCLIInstalled sets whether the CLI is installed.
func (fg *FeatureGate) SetCLIInstalled(installed bool) {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	fg.cliInstalled = installed
}

// IsCLIInstalled returns whether the CLI is installed.
func (fg *FeatureGate) IsCLIInstalled() bool {
	fg.mu.RLock()
	defer fg.mu.RUnlock()
	return fg.cliInstalled
}

// IsEnabled checks if a feature is enabled.
func (fg *FeatureGate) IsEnabled(feature Feature) bool {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	// Check for override
	if override, ok := fg.overrides[feature]; ok {
		return override
	}

	// Check feature definition
	info, ok := fg.features[feature]
	if !ok {
		return false
	}

	// If feature requires CLI, check if CLI is installed
	if info.RequiresCLI && !fg.cliInstalled {
		return false
	}

	return true
}

// SetOverride sets an override for a feature.
func (fg *FeatureGate) SetOverride(feature Feature, enabled bool) {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	fg.overrides[feature] = enabled
}

// ClearOverride clears an override for a feature.
func (fg *FeatureGate) ClearOverride(feature Feature) {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	delete(fg.overrides, feature)
}

// GetFeatureInfo returns information about a feature.
func (fg *FeatureGate) GetFeatureInfo(feature Feature) *FeatureInfo {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	info, ok := fg.features[feature]
	if !ok {
		return nil
	}

	// Create a copy with current enabled status
	result := *info
	result.Enabled = fg.isEnabledLocked(feature)
	return &result
}

// isEnabledLocked checks if a feature is enabled (must hold lock).
func (fg *FeatureGate) isEnabledLocked(feature Feature) bool {
	if override, ok := fg.overrides[feature]; ok {
		return override
	}

	info, ok := fg.features[feature]
	if !ok {
		return false
	}

	if info.RequiresCLI && !fg.cliInstalled {
		return false
	}

	return true
}

// GetAllFeatures returns information about all features.
func (fg *FeatureGate) GetAllFeatures() map[Feature]*FeatureInfo {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	result := make(map[Feature]*FeatureInfo)
	for f, info := range fg.features {
		copy := *info
		copy.Enabled = fg.isEnabledLocked(f)
		result[f] = &copy
	}
	return result
}

// GetEnabledFeatures returns all enabled features.
func (fg *FeatureGate) GetEnabledFeatures() []Feature {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	var enabled []Feature
	for f := range fg.features {
		if fg.isEnabledLocked(f) {
			enabled = append(enabled, f)
		}
	}
	return enabled
}

// GetDisabledFeatures returns all disabled features.
func (fg *FeatureGate) GetDisabledFeatures() []Feature {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	var disabled []Feature
	for f := range fg.features {
		if !fg.isEnabledLocked(f) {
			disabled = append(disabled, f)
		}
	}
	return disabled
}

// GetFeaturesByCategory returns features grouped by category.
func (fg *FeatureGate) GetFeaturesByCategory() map[string][]FeatureInfo {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	result := make(map[string][]FeatureInfo)
	for f, info := range fg.features {
		copy := *info
		copy.Enabled = fg.isEnabledLocked(f)
		result[info.Category] = append(result[info.Category], copy)
	}
	return result
}

// GetCLIDependentFeatures returns features that require CLI.
func (fg *FeatureGate) GetCLIDependentFeatures() []FeatureInfo {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	var cliFeatures []FeatureInfo
	for f, info := range fg.features {
		if info.RequiresCLI {
			copy := *info
			copy.Enabled = fg.isEnabledLocked(f)
			cliFeatures = append(cliFeatures, copy)
		}
	}
	return cliFeatures
}

// FeatureStatus represents the overall feature status.
type FeatureStatus struct {
	CLIInstalled     bool                    `json:"cli_installed"`
	TotalFeatures    int                     `json:"total_features"`
	EnabledCount     int                     `json:"enabled_count"`
	DisabledCount    int                     `json:"disabled_count"`
	Features         map[Feature]*FeatureInfo `json:"features"`
	DisabledReasons  map[Feature]string      `json:"disabled_reasons,omitempty"`
}

// GetStatus returns the overall feature status.
func (fg *FeatureGate) GetStatus() *FeatureStatus {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	status := &FeatureStatus{
		CLIInstalled:    fg.cliInstalled,
		TotalFeatures:   len(fg.features),
		Features:        make(map[Feature]*FeatureInfo),
		DisabledReasons: make(map[Feature]string),
	}

	for f, info := range fg.features {
		copy := *info
		copy.Enabled = fg.isEnabledLocked(f)
		status.Features[f] = &copy

		if copy.Enabled {
			status.EnabledCount++
		} else {
			status.DisabledCount++
			if info.RequiresCLI && !fg.cliInstalled {
				status.DisabledReasons[f] = "Requires Claude Code CLI"
			}
		}
	}

	return status
}

// RequireFeature returns an error if the feature is not enabled.
func (fg *FeatureGate) RequireFeature(feature Feature) error {
	if !fg.IsEnabled(feature) {
		info := fg.GetFeatureInfo(feature)
		if info != nil && info.RequiresCLI {
			return &FeatureDisabledError{
				Feature: feature,
				Reason:  "This feature requires Claude Code CLI to be installed",
			}
		}
		return &FeatureDisabledError{
			Feature: feature,
			Reason:  "Feature is disabled",
		}
	}
	return nil
}

// FeatureDisabledError is returned when a feature is not enabled.
type FeatureDisabledError struct {
	Feature Feature
	Reason  string
}

func (e *FeatureDisabledError) Error() string {
	return e.Reason
}
