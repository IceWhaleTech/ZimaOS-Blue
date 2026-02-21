package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/labstack/echo/v4"
)

// SettingsHandler handles user settings API endpoints
type SettingsHandler struct {
	mu           sync.RWMutex
	settingsPath string
	settings     *Settings
}

// Settings represents user preferences stored on backend
type Settings struct {
	Locale   string `json:"locale,omitempty"`   // User's preferred locale (e.g., "zh-CN", "en-US")
	Timezone string `json:"timezone,omitempty"` // User's timezone
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(dataDir string) *SettingsHandler {
	settingsPath := filepath.Join(dataDir, "settings.json")
	h := &SettingsHandler{
		settingsPath: settingsPath,
		settings:     &Settings{},
	}
	h.load()
	return h
}

// RegisterRoutes registers settings routes
func (h *SettingsHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/settings", h.Get)
	g.PUT("/settings", h.Update)
	g.PATCH("/settings", h.Patch)
}

// Get handles GET /api/settings
func (h *SettingsHandler) Get(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return c.JSON(http.StatusOK, h.settings)
}

// Update handles PUT /api/settings (full update)
func (h *SettingsHandler) Update(c echo.Context) error {
	var newSettings Settings
	if err := c.Bind(&newSettings); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	h.mu.Lock()
	h.settings = &newSettings
	h.mu.Unlock()

	if err := h.save(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save settings"})
	}

	return c.JSON(http.StatusOK, h.settings)
}

// Patch handles PATCH /api/settings (partial update)
func (h *SettingsHandler) Patch(c echo.Context) error {
	var updates map[string]interface{}
	if err := c.Bind(&updates); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Apply partial updates
	if locale, ok := updates["locale"].(string); ok {
		h.settings.Locale = locale
	}
	if timezone, ok := updates["timezone"].(string); ok {
		h.settings.Timezone = timezone
	}

	if err := h.save(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save settings"})
	}

	return c.JSON(http.StatusOK, h.settings)
}

// GetLocale returns the current locale setting
func (h *SettingsHandler) GetLocale() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.settings.Locale
}

// load reads settings from disk
func (h *SettingsHandler) load() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := os.ReadFile(h.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, use defaults
			h.settings = &Settings{}
			return nil
		}
		return err
	}

	return json.Unmarshal(data, h.settings)
}

// save writes settings to disk
func (h *SettingsHandler) save() error {
	// Ensure directory exists
	dir := filepath.Dir(h.settingsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(h.settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(h.settingsPath, data, 0644)
}

