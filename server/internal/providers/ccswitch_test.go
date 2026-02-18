package providers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCCSwitchIntegration_IsAvailable(t *testing.T) {
	// Test with non-existent path
	ccSwitch := NewCCSwitchIntegrationWithPath("/non/existent/path/config.json")
	if ccSwitch.IsAvailable() {
		t.Error("IsAvailable() should return false for non-existent path")
	}
}

func TestCCSwitchIntegration_ReadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	config := CCSwitchConfig{
		ActiveProfile: "default",
		Profiles: []Profile{
			{
				Name:     "default",
				Provider: "anthropic",
				Model:    "claude-3-sonnet",
				APIKey:   "sk-ant-test-key",
				BaseURL:  "https://api.anthropic.com",
			},
			{
				Name:     "openai",
				Provider: "openai",
				Model:    "gpt-4o",
				APIKey:   "sk-openai-test-key",
				BaseURL:  "https://api.openai.com",
			},
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	ccSwitch := NewCCSwitchIntegrationWithPath(configPath)

	// Test IsAvailable
	if !ccSwitch.IsAvailable() {
		t.Error("IsAvailable() should return true for existing config")
	}

	// Test ReadConfig
	readConfig, err := ccSwitch.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig() error = %v", err)
	}

	if readConfig.ActiveProfile != "default" {
		t.Errorf("ActiveProfile = %q, want %q", readConfig.ActiveProfile, "default")
	}

	if len(readConfig.Profiles) != 2 {
		t.Errorf("len(Profiles) = %d, want 2", len(readConfig.Profiles))
	}
}

func TestCCSwitchIntegration_GetActiveProfile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	config := CCSwitchConfig{
		ActiveProfile: "openai",
		Profiles: []Profile{
			{
				Name:     "default",
				Provider: "anthropic",
				Model:    "claude-3-sonnet",
			},
			{
				Name:     "openai",
				Provider: "openai",
				Model:    "gpt-4o",
			},
		},
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, data, 0600)

	ccSwitch := NewCCSwitchIntegrationWithPath(configPath)

	profile, err := ccSwitch.GetActiveProfile()
	if err != nil {
		t.Fatalf("GetActiveProfile() error = %v", err)
	}

	if profile.Name != "openai" {
		t.Errorf("profile.Name = %q, want %q", profile.Name, "openai")
	}

	if profile.Provider != "openai" {
		t.Errorf("profile.Provider = %q, want %q", profile.Provider, "openai")
	}
}

func TestCCSwitchIntegration_ListProfiles(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	config := CCSwitchConfig{
		ActiveProfile: "default",
		Profiles: []Profile{
			{Name: "default", Provider: "anthropic"},
			{Name: "openai", Provider: "openai"},
			{Name: "ollama", Provider: "ollama"},
		},
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, data, 0600)

	ccSwitch := NewCCSwitchIntegrationWithPath(configPath)

	profiles, err := ccSwitch.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}

	if len(profiles) != 3 {
		t.Errorf("len(profiles) = %d, want 3", len(profiles))
	}
}

func TestCCSwitchIntegration_ListProfilesDisplay(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	config := CCSwitchConfig{
		ActiveProfile: "default",
		Profiles: []Profile{
			{
				Name:     "default",
				Provider: "anthropic",
				APIKey:   "sk-ant-api03-1807890abcdef",
			},
		},
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, data, 0600)

	ccSwitch := NewCCSwitchIntegrationWithPath(configPath)

	displays, err := ccSwitch.ListProfilesDisplay()
	if err != nil {
		t.Fatalf("ListProfilesDisplay() error = %v", err)
	}

	if len(displays) != 1 {
		t.Fatalf("len(displays) = %d, want 1", len(displays))
	}

	// Check that API key is masked
	if displays[0].APIKey == "sk-ant-api03-1807890abcdef" {
		t.Error("API key should be masked in display")
	}

	// Check IsActive flag
	if !displays[0].IsActive {
		t.Error("IsActive should be true for active profile")
	}
}

func TestCCSwitchIntegration_ActivateProfile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	config := CCSwitchConfig{
		ActiveProfile: "default",
		Profiles: []Profile{
			{Name: "default", Provider: "anthropic"},
			{Name: "openai", Provider: "openai"},
		},
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, data, 0600)

	ccSwitch := NewCCSwitchIntegrationWithPath(configPath)

	// Activate openai profile
	err := ccSwitch.ActivateProfile("openai")
	if err != nil {
		t.Fatalf("ActivateProfile() error = %v", err)
	}

	// Verify the change
	name, err := ccSwitch.GetActiveProfileName()
	if err != nil {
		t.Fatalf("GetActiveProfileName() error = %v", err)
	}

	if name != "openai" {
		t.Errorf("ActiveProfile = %q, want %q", name, "openai")
	}

	// Try to activate non-existent profile
	err = ccSwitch.ActivateProfile("non-existent")
	if err == nil {
		t.Error("ActivateProfile() should return error for non-existent profile")
	}
}

func TestCCSwitchIntegration_GetActiveProfileName(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	config := CCSwitchConfig{
		ActiveProfile: "my-profile",
		Profiles: []Profile{
			{Name: "my-profile", Provider: "anthropic"},
		},
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, data, 0600)

	ccSwitch := NewCCSwitchIntegrationWithPath(configPath)

	name, err := ccSwitch.GetActiveProfileName()
	if err != nil {
		t.Fatalf("GetActiveProfileName() error = %v", err)
	}

	if name != "my-profile" {
		t.Errorf("name = %q, want %q", name, "my-profile")
	}
}
