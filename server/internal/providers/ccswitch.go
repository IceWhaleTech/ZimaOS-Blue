package providers

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CCSwitchConfig represents the cc-switch configuration file.
type CCSwitchConfig struct {
	ActiveProfile string    `json:"active_profile"`
	Profiles      []Profile `json:"profiles"`
}

// Profile represents a cc-switch profile.
type Profile struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
}

// ProfileDisplay is a safe version of Profile for display (with masked keys).
type ProfileDisplay struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
	IsActive bool   `json:"is_active"`
}

// CCSwitchIntegration provides integration with cc-switch configuration.
type CCSwitchIntegration struct {
	ConfigPath string
}

// NewCCSwitchIntegration creates a new cc-switch integration with default config path.
func NewCCSwitchIntegration() *CCSwitchIntegration {
	return &CCSwitchIntegration{
		ConfigPath: "", // Will use default path
	}
}

// NewCCSwitchIntegrationWithPath creates a new cc-switch integration with a custom config path.
func NewCCSwitchIntegrationWithPath(configPath string) *CCSwitchIntegration {
	return &CCSwitchIntegration{
		ConfigPath: configPath,
	}
}

// getConfigPath returns the config path, using default if not set.
func (c *CCSwitchIntegration) getConfigPath() string {
	if c.ConfigPath != "" {
		return c.ConfigPath
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".claude-code-switch", "config.json")
}

// IsAvailable checks if cc-switch configuration exists.
func (c *CCSwitchIntegration) IsAvailable() bool {
	configPath := c.getConfigPath()
	if configPath == "" {
		return false
	}

	_, err := os.Stat(configPath)
	return err == nil
}

// ReadConfig reads the cc-switch configuration file.
func (c *CCSwitchIntegration) ReadConfig() (*CCSwitchConfig, error) {
	configPath := c.getConfigPath()
	if configPath == "" {
		return nil, os.ErrNotExist
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config CCSwitchConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// GetActiveProfile returns the currently active profile.
func (c *CCSwitchIntegration) GetActiveProfile() (*Profile, error) {
	config, err := c.ReadConfig()
	if err != nil {
		return nil, err
	}

	for _, profile := range config.Profiles {
		if profile.Name == config.ActiveProfile {
			return &profile, nil
		}
	}

	// Return first profile if no active profile is set
	if len(config.Profiles) > 0 {
		return &config.Profiles[0], nil
	}

	return nil, os.ErrNotExist
}

// ListProfiles returns all profiles.
func (c *CCSwitchIntegration) ListProfiles() ([]Profile, error) {
	config, err := c.ReadConfig()
	if err != nil {
		return nil, err
	}

	return config.Profiles, nil
}

// ListProfilesDisplay returns all profiles in display-safe format.
func (c *CCSwitchIntegration) ListProfilesDisplay() ([]ProfileDisplay, error) {
	config, err := c.ReadConfig()
	if err != nil {
		return nil, err
	}

	displays := make([]ProfileDisplay, len(config.Profiles))
	for i, p := range config.Profiles {
		displays[i] = ProfileDisplay{
			Name:     p.Name,
			Provider: p.Provider,
			Model:    p.Model,
			APIKey:   MaskAPIKey(p.APIKey),
			BaseURL:  p.BaseURL,
			IsActive: p.Name == config.ActiveProfile,
		}
	}

	return displays, nil
}

// ActivateProfile activates a profile by name.
func (c *CCSwitchIntegration) ActivateProfile(name string) error {
	config, err := c.ReadConfig()
	if err != nil {
		return err
	}

	// Check if profile exists
	found := false
	for _, p := range config.Profiles {
		if p.Name == name {
			found = true
			break
		}
	}

	if !found {
		return os.ErrNotExist
	}

	config.ActiveProfile = name

	return c.writeConfig(config)
}

// writeConfig writes the configuration back to file.
func (c *CCSwitchIntegration) writeConfig(config *CCSwitchConfig) error {
	configPath := c.getConfigPath()
	if configPath == "" {
		return os.ErrNotExist
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

// GetActiveProfileName returns the name of the active profile.
func (c *CCSwitchIntegration) GetActiveProfileName() (string, error) {
	config, err := c.ReadConfig()
	if err != nil {
		return "", err
	}

	return config.ActiveProfile, nil
}
