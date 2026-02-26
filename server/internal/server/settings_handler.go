package server

import (
	"context"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

const settingsKVKey = "config:settings"

// SettingsHandler handles user settings API endpoints
type SettingsHandler struct {
	mu       sync.RWMutex
	kv       kvstore.Store
	settings *Settings
}

// Settings represents user preferences stored on backend
type Settings struct {
	Locale             string `json:"locale,omitempty"`               // User's preferred locale (e.g., "zh-CN", "en-US")
	Timezone           string `json:"timezone,omitempty"`             // User's timezone
	SmartToolSelection *bool  `json:"smart_tool_selection,omitempty"` // IR-based tool filtering (nil = default true)
	AgentMode          *bool  `json:"agent_mode,omitempty"`           // Autonomous agent mode (nil = default false)
	AgentAutoConfirm   *bool  `json:"agent_auto_confirm,omitempty"`   // Skip confirmation in agent mode (nil = default false)
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(kv kvstore.Store) *SettingsHandler {
	h := &SettingsHandler{
		kv:       kv,
		settings: &Settings{},
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
	if v, ok := updates["smart_tool_selection"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmartToolSelection = &b
		}
	}
	if v, ok := updates["agent_mode"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.AgentMode = &b
		}
	}
	if v, ok := updates["agent_auto_confirm"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.AgentAutoConfirm = &b
		}
	}

	if err := h.save(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save settings"})
	}

	return c.JSON(http.StatusOK, h.settings)
}

// GetLocale returns the current locale setting, falling back to OS-detected locale.
func (h *SettingsHandler) GetLocale() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.Locale != "" {
		return h.settings.Locale
	}
	return workspace.DetectLocale()
}

// GetSmartToolSelection returns whether smart tool selection is enabled (default false).
func (h *SettingsHandler) GetSmartToolSelection() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmartToolSelection == nil {
		return false
	}
	return *h.settings.SmartToolSelection
}

// GetAgentMode returns whether agent mode is enabled (default true).
func (h *SettingsHandler) GetAgentMode() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentMode == nil {
		return true
	}
	return *h.settings.AgentMode
}

// GetAgentAutoConfirm returns whether agent mode skips confirmation (default false).
func (h *SettingsHandler) GetAgentAutoConfirm() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentAutoConfirm == nil {
		return false
	}
	return *h.settings.AgentAutoConfirm
}

// load reads settings from kvstore
func (h *SettingsHandler) load() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.kv.GetJSON(context.Background(), settingsKVKey, h.settings); err != nil {
		// Key not found or error — use defaults
		h.settings = &Settings{}
	}
}

// save writes settings to kvstore
func (h *SettingsHandler) save() error {
	return h.kv.SetJSON(context.Background(), settingsKVKey, h.settings, 0)
}
