package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Profile represents a browser profile configuration.
type Profile struct {
	// ID is the unique profile identifier.
	ID string `json:"id"`
	// Name is the display name.
	Name string `json:"name"`
	// Description is an optional description.
	Description string `json:"description,omitempty"`
	// UserAgent is the custom user agent string.
	UserAgent string `json:"user_agent,omitempty"`
	// Viewport is the default viewport size.
	Viewport *Viewport `json:"viewport,omitempty"`
	// Proxy is the proxy configuration.
	Proxy *ProxyConfig `json:"proxy,omitempty"`
	// Cookies are pre-set cookies.
	Cookies []Cookie `json:"cookies,omitempty"`
	// Headers are custom HTTP headers.
	Headers map[string]string `json:"headers,omitempty"`
	// LocalStorage is pre-set local storage data.
	LocalStorage map[string]map[string]string `json:"local_storage,omitempty"`
	// Geolocation is the geolocation override.
	Geolocation *Geolocation `json:"geolocation,omitempty"`
	// Timezone is the timezone override.
	Timezone string `json:"timezone,omitempty"`
	// Locale is the locale override.
	Locale string `json:"locale,omitempty"`
	// ColorScheme is the preferred color scheme (light/dark).
	ColorScheme string `json:"color_scheme,omitempty"`
	// ReducedMotion enables reduced motion preference.
	ReducedMotion bool `json:"reduced_motion,omitempty"`
	// Permissions are granted permissions.
	Permissions []string `json:"permissions,omitempty"`
	// BlockedURLs are URLs to block.
	BlockedURLs []string `json:"blocked_urls,omitempty"`
	// CreatedAt is when the profile was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the profile was last updated.
	UpdatedAt time.Time `json:"updated_at"`
	// IsDefault indicates if this is the default profile.
	IsDefault bool `json:"is_default,omitempty"`
}

// ProxyConfig represents proxy configuration.
type ProxyConfig struct {
	// Server is the proxy server URL.
	Server string `json:"server"`
	// Username is the proxy username.
	Username string `json:"username,omitempty"`
	// Password is the proxy password.
	Password string `json:"password,omitempty"`
	// Bypass is a list of hosts to bypass the proxy.
	Bypass []string `json:"bypass,omitempty"`
}

// Cookie represents a browser cookie.
type Cookie struct {
	// Name is the cookie name.
	Name string `json:"name"`
	// Value is the cookie value.
	Value string `json:"value"`
	// Domain is the cookie domain.
	Domain string `json:"domain"`
	// Path is the cookie path.
	Path string `json:"path,omitempty"`
	// Expires is the expiration time.
	Expires *time.Time `json:"expires,omitempty"`
	// HTTPOnly marks the cookie as HTTP only.
	HTTPOnly bool `json:"http_only,omitempty"`
	// Secure marks the cookie as secure.
	Secure bool `json:"secure,omitempty"`
	// SameSite is the SameSite attribute.
	SameSite string `json:"same_site,omitempty"`
}

// Geolocation represents a geolocation override.
type Geolocation struct {
	// Latitude is the latitude.
	Latitude float64 `json:"latitude"`
	// Longitude is the longitude.
	Longitude float64 `json:"longitude"`
	// Accuracy is the accuracy in meters.
	Accuracy float64 `json:"accuracy,omitempty"`
}

// ProfileManager manages browser profiles.
type ProfileManager struct {
	profiles   map[string]*Profile
	profileDir string
	mu         sync.RWMutex
}

// NewProfileManager creates a new profile manager.
func NewProfileManager(profileDir string) (*ProfileManager, error) {
	if profileDir == "" {
		profileDir = filepath.Join(os.TempDir(), "zimaos-browser-profiles")
	}

	if err := os.MkdirAll(profileDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create profile dir: %w", err)
	}

	pm := &ProfileManager{
		profiles:   make(map[string]*Profile),
		profileDir: profileDir,
	}

	// Load existing profiles
	if err := pm.loadProfiles(); err != nil {
		return nil, fmt.Errorf("failed to load profiles: %w", err)
	}

	return pm, nil
}

// loadProfiles loads profiles from disk.
func (pm *ProfileManager) loadProfiles() error {
	files, err := os.ReadDir(pm.profileDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		profilePath := filepath.Join(pm.profileDir, file.Name())
		data, err := os.ReadFile(profilePath)
		if err != nil {
			continue
		}

		var profile Profile
		if err := json.Unmarshal(data, &profile); err != nil {
			continue
		}

		pm.profiles[profile.ID] = &profile
	}

	return nil
}

// saveProfile saves a profile to disk.
func (pm *ProfileManager) saveProfile(profile *Profile) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}

	profilePath := filepath.Join(pm.profileDir, profile.ID+".json")
	return os.WriteFile(profilePath, data, 0600)
}

// CreateProfile creates a new profile.
func (pm *ProfileManager) CreateProfile(ctx context.Context, profile *Profile) (*Profile, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if profile.ID == "" {
		return nil, fmt.Errorf("profile ID is required")
	}

	if _, exists := pm.profiles[profile.ID]; exists {
		return nil, fmt.Errorf("profile %s already exists", profile.ID)
	}

	now := time.Now()
	profile.CreatedAt = now
	profile.UpdatedAt = now

	// Set default viewport if not specified
	if profile.Viewport == nil {
		defaultVP := DefaultViewport()
		profile.Viewport = &defaultVP
	}

	if err := pm.saveProfile(profile); err != nil {
		return nil, fmt.Errorf("failed to save profile: %w", err)
	}

	pm.profiles[profile.ID] = profile
	return profile, nil
}

// GetProfile returns a profile by ID.
func (pm *ProfileManager) GetProfile(profileID string) (*Profile, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	profile, exists := pm.profiles[profileID]
	if !exists {
		return nil, fmt.Errorf("profile %s not found", profileID)
	}

	return profile, nil
}

// UpdateProfile updates an existing profile.
func (pm *ProfileManager) UpdateProfile(ctx context.Context, profile *Profile) (*Profile, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	existing, exists := pm.profiles[profile.ID]
	if !exists {
		return nil, fmt.Errorf("profile %s not found", profile.ID)
	}

	// Preserve creation time
	profile.CreatedAt = existing.CreatedAt
	profile.UpdatedAt = time.Now()

	if err := pm.saveProfile(profile); err != nil {
		return nil, fmt.Errorf("failed to save profile: %w", err)
	}

	pm.profiles[profile.ID] = profile
	return profile, nil
}

// DeleteProfile deletes a profile.
func (pm *ProfileManager) DeleteProfile(ctx context.Context, profileID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.profiles[profileID]; !exists {
		return fmt.Errorf("profile %s not found", profileID)
	}

	// Delete profile file
	profilePath := filepath.Join(pm.profileDir, profileID+".json")
	if err := os.Remove(profilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete profile file: %w", err)
	}

	delete(pm.profiles, profileID)
	return nil
}

// ListProfiles returns all profiles.
func (pm *ProfileManager) ListProfiles() []*Profile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	profiles := make([]*Profile, 0, len(pm.profiles))
	for _, profile := range pm.profiles {
		profiles = append(profiles, profile)
	}
	return profiles
}

// GetDefaultProfile returns the default profile.
func (pm *ProfileManager) GetDefaultProfile() *Profile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, profile := range pm.profiles {
		if profile.IsDefault {
			return profile
		}
	}

	// Return first profile if no default is set
	for _, profile := range pm.profiles {
		return profile
	}

	return nil
}

// SetDefaultProfile sets a profile as the default.
func (pm *ProfileManager) SetDefaultProfile(ctx context.Context, profileID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	profile, exists := pm.profiles[profileID]
	if !exists {
		return fmt.Errorf("profile %s not found", profileID)
	}

	// Clear default flag from all profiles
	for _, p := range pm.profiles {
		if p.IsDefault {
			p.IsDefault = false
			p.UpdatedAt = time.Now()
			pm.saveProfile(p)
		}
	}

	// Set new default
	profile.IsDefault = true
	profile.UpdatedAt = time.Now()
	return pm.saveProfile(profile)
}

// CloneProfile creates a copy of an existing profile.
func (pm *ProfileManager) CloneProfile(ctx context.Context, sourceID, newID, newName string) (*Profile, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	source, exists := pm.profiles[sourceID]
	if !exists {
		return nil, fmt.Errorf("source profile %s not found", sourceID)
	}

	if _, exists := pm.profiles[newID]; exists {
		return nil, fmt.Errorf("profile %s already exists", newID)
	}

	// Deep copy the profile
	data, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}

	var newProfile Profile
	if err := json.Unmarshal(data, &newProfile); err != nil {
		return nil, err
	}

	now := time.Now()
	newProfile.ID = newID
	newProfile.Name = newName
	newProfile.CreatedAt = now
	newProfile.UpdatedAt = now
	newProfile.IsDefault = false

	if err := pm.saveProfile(&newProfile); err != nil {
		return nil, fmt.Errorf("failed to save profile: %w", err)
	}

	pm.profiles[newID] = &newProfile
	return &newProfile, nil
}

// ExportProfile exports a profile to JSON.
func (pm *ProfileManager) ExportProfile(profileID string) ([]byte, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	profile, exists := pm.profiles[profileID]
	if !exists {
		return nil, fmt.Errorf("profile %s not found", profileID)
	}

	return json.MarshalIndent(profile, "", "  ")
}

// ImportProfile imports a profile from JSON.
func (pm *ProfileManager) ImportProfile(ctx context.Context, data []byte) (*Profile, error) {
	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("invalid profile data: %w", err)
	}

	return pm.CreateProfile(ctx, &profile)
}

// GetProfileDataDir returns the data directory for a profile.
func (pm *ProfileManager) GetProfileDataDir(profileID string) string {
	return filepath.Join(pm.profileDir, profileID, "data")
}

// CreateDefaultProfile creates a default profile if none exists.
func (pm *ProfileManager) CreateDefaultProfile(ctx context.Context) (*Profile, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if default profile already exists
	for _, p := range pm.profiles {
		if p.IsDefault {
			return p, nil
		}
	}

	defaultVP := DefaultViewport()
	profile := &Profile{
		ID:          "default",
		Name:        "Default Profile",
		Description: "Default browser profile",
		Viewport:    &defaultVP,
		IsDefault:   true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := pm.saveProfile(profile); err != nil {
		return nil, fmt.Errorf("failed to save default profile: %w", err)
	}

	pm.profiles[profile.ID] = profile
	return profile, nil
}
