package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

// ChannelHandler handles channel-related API endpoints.
type ChannelHandler struct {
	manager *channel.Manager
}

// NewChannelHandler creates a new channel handler.
func NewChannelHandler(manager *channel.Manager) *ChannelHandler {
	return &ChannelHandler{
		manager: manager,
	}
}

// ListChannels lists all registered channels.
func (h *ChannelHandler) ListChannels(c echo.Context) error {
	infos := h.manager.List()
	return c.JSON(http.StatusOK, infos)
}

// GetChannel returns information about a specific channel.
func (h *ChannelHandler) GetChannel(c echo.Context) error {
	name := c.Param("name")

	ch, exists := h.manager.Get(name)
	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, "channel not found")
	}

	return c.JSON(http.StatusOK, ch.Info())
}

// StartChannel starts a specific channel.
func (h *ChannelHandler) StartChannel(c echo.Context) error {
	name := c.Param("name")

	if err := h.manager.StartChannel(c.Request().Context(), name); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	ch, _ := h.manager.Get(name)
	return c.JSON(http.StatusOK, ch.Info())
}

// StopChannel stops a specific channel.
func (h *ChannelHandler) StopChannel(c echo.Context) error {
	name := c.Param("name")

	if err := h.manager.StopChannel(c.Request().Context(), name); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	ch, _ := h.manager.Get(name)
	return c.JSON(http.StatusOK, ch.Info())
}

// GetChannelStatus returns the status of a specific channel.
func (h *ChannelHandler) GetChannelStatus(c echo.Context) error {
	name := c.Param("name")

	ch, exists := h.manager.Get(name)
	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, "channel not found")
	}

	info := ch.Info()
	status := map[string]interface{}{
		"name":          info.Name,
		"type":          info.Type,
		"status":        info.Status,
		"enabled":       info.Enabled,
		"connected_at":  info.ConnectedAt,
		"last_error":    info.LastError,
		"last_error_at": info.LastErrorAt,
		"message_count": info.MessageCount,
	}

	return c.JSON(http.StatusOK, status)
}

// TestChannelRequest represents a request to test a channel.
type TestChannelRequest struct {
	ChatID  string `json:"chat_id"`
	Message string `json:"message"`
}

// TestChannel tests a channel by sending a test message.
func (h *ChannelHandler) TestChannel(c echo.Context) error {
	name := c.Param("name")

	var req TestChannelRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ch, exists := h.manager.Get(name)
	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, "channel not found")
	}

	if !ch.IsConnected() {
		return echo.NewHTTPError(http.StatusBadRequest, "channel is not connected")
	}

	if req.ChatID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "chat_id is required")
	}

	if req.Message == "" {
		req.Message = "Test message from ZimaOS-Echo"
	}

	msg := channel.OutgoingMessage{
		ChatID:  req.ChatID,
		Content: req.Message,
	}

	if err := ch.Send(c.Request().Context(), msg); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Test message sent",
	})
}

// RegisterRoutes registers channel-related routes.
func (h *ChannelHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/channels", h.ListChannels)
	g.GET("/channels/:name", h.GetChannel)
	g.POST("/channels/:name/start", h.StartChannel)
	g.POST("/channels/:name/stop", h.StopChannel)
	g.GET("/channels/:name/status", h.GetChannelStatus)
	g.POST("/channels/:name/test", h.TestChannel)
}

// ChannelConfigStore stores channel configurations persistently.
type ChannelConfigStore struct {
	dataDir string
	mu      sync.RWMutex
	configs map[string]*ChannelConfig
}

// ChannelConfig represents a channel's configuration.
type ChannelConfig struct {
	ID      string            `json:"id"`
	Enabled bool              `json:"enabled"`
	Status  string            `json:"status"`
	Config  map[string]string `json:"config"`
}

// NewChannelConfigStore creates a new channel config store.
func NewChannelConfigStore(dataDir string) *ChannelConfigStore {
	store := &ChannelConfigStore{
		dataDir: dataDir,
		configs: make(map[string]*ChannelConfig),
	}
	store.load()
	return store
}

// Get returns a channel's configuration.
func (s *ChannelConfigStore) Get(id string) (*ChannelConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.configs[id]
	return cfg, ok
}

// Set saves a channel's configuration.
func (s *ChannelConfigStore) Set(id string, cfg *ChannelConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg.ID = id
	s.configs[id] = cfg
	return s.save()
}

// List returns all channel configurations.
func (s *ChannelConfigStore) List() []*ChannelConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*ChannelConfig, 0, len(s.configs))
	for _, cfg := range s.configs {
		result = append(result, cfg)
	}
	return result
}

func (s *ChannelConfigStore) load() {
	configPath := filepath.Join(s.dataDir, "channels.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var configs map[string]*ChannelConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return
	}
	s.configs = configs
}

func (s *ChannelConfigStore) save() error {
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.configs, "", "  ")
	if err != nil {
		return err
	}
	configPath := filepath.Join(s.dataDir, "channels.json")
	return os.WriteFile(configPath, data, 0644)
}

// ChannelConfigHandler handles channel configuration API endpoints.
type ChannelConfigHandler struct {
	store *ChannelConfigStore
}

// NewChannelConfigHandler creates a new channel config handler.
func NewChannelConfigHandler(store *ChannelConfigStore) *ChannelConfigHandler {
	return &ChannelConfigHandler{store: store}
}

// ListChannelConfigs lists all channel configurations.
func (h *ChannelConfigHandler) ListChannelConfigs(c echo.Context) error {
	configs := h.store.List()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"channels": configs,
	})
}

// GetChannelConfig returns a channel's configuration.
func (h *ChannelConfigHandler) GetChannelConfig(c echo.Context) error {
	id := c.Param("id")
	cfg, ok := h.store.Get(id)
	if !ok {
		// Return empty config for unconfigured channels
		return c.JSON(http.StatusOK, &ChannelConfig{
			ID:      id,
			Enabled: false,
			Status:  "disconnected",
			Config:  make(map[string]string),
		})
	}
	return c.JSON(http.StatusOK, cfg)
}

// UpdateChannelConfigRequest represents a request to update channel config.
type UpdateChannelConfigRequest struct {
	Enabled bool              `json:"enabled"`
	Config  map[string]string `json:"config"`
}

// UpdateChannelConfig updates a channel's configuration.
func (h *ChannelConfigHandler) UpdateChannelConfig(c echo.Context) error {
	id := c.Param("id")

	var req UpdateChannelConfigRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	cfg := &ChannelConfig{
		ID:      id,
		Enabled: req.Enabled,
		Status:  "disconnected",
		Config:  req.Config,
	}

	if err := h.store.Set(id, cfg); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save configuration")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Configuration saved",
		"channel": cfg,
	})
}

// ToggleChannelRequest represents a request to toggle channel enabled state.
type ToggleChannelRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleChannel toggles a channel's enabled state.
func (h *ChannelConfigHandler) ToggleChannel(c echo.Context) error {
	id := c.Param("id")

	var req ToggleChannelRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Get existing config or create new one
	cfg, ok := h.store.Get(id)
	if !ok {
		cfg = &ChannelConfig{
			ID:     id,
			Status: "disconnected",
			Config: make(map[string]string),
		}
	}

	cfg.Enabled = req.Enabled
	// Update status based on enabled state
	if req.Enabled {
		cfg.Status = "connecting"
	} else {
		cfg.Status = "disconnected"
	}

	if err := h.store.Set(id, cfg); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save configuration")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"enabled": cfg.Enabled,
		"status":  cfg.Status,
		"channel": cfg,
	})
}

// RegisterRoutes registers channel config routes.
func (h *ChannelConfigHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/channels", h.ListChannelConfigs)
	g.GET("/channels/:id", h.GetChannelConfig)
	g.PUT("/channels/:id", h.UpdateChannelConfig)
	g.POST("/channels/:id/toggle", h.ToggleChannel)
}
