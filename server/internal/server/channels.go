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
		"name":              info.Name,
		"type":              info.Type,
		"status":            info.Status,
		"enabled":           info.Enabled,
		"connected_at":      info.ConnectedAt,
		"last_error":        info.LastError,
		"last_error_at":     info.LastErrorAt,
		"message_count":     info.MessageCount,
		"messages_received": info.MessagesReceived,
		"messages_sent":     info.MessagesSent,
		"last_message_at":   info.LastMessageAt,
		"last_reply_at":     info.LastReplyAt,
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
	ID        string            `json:"id"`
	Enabled   bool              `json:"enabled"`
	Status    string            `json:"status"`
	Config    map[string]string `json:"config"`
	LastError string            `json:"last_error,omitempty"`
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

// GetEnabled returns all enabled channel configurations.
func (s *ChannelConfigStore) GetEnabled() []*ChannelConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*ChannelConfig, 0)
	for _, cfg := range s.configs {
		if cfg.Enabled {
			result = append(result, cfg)
		}
	}
	return result
}

// ChannelConfigHandler handles channel configuration API endpoints.
type ChannelConfigHandler struct {
	store   *ChannelConfigStore
	manager *channel.Manager
	factory *ChannelFactory
}

// NewChannelConfigHandler creates a new channel config handler.
func NewChannelConfigHandler(store *ChannelConfigStore) *ChannelConfigHandler {
	return &ChannelConfigHandler{store: store}
}

// SetManager sets the channel manager for connection lifecycle management.
func (h *ChannelConfigHandler) SetManager(manager *channel.Manager) {
	h.manager = manager
}

// SetFactory sets the channel factory for creating channel instances.
func (h *ChannelConfigHandler) SetFactory(factory *ChannelFactory) {
	h.factory = factory
}

// ChannelConfigResponse represents a channel config with runtime stats.
type ChannelConfigResponse struct {
	ID               string            `json:"id"`
	Enabled          bool              `json:"enabled"`
	Status           string            `json:"status"`
	Config           map[string]string `json:"config"`
	LastError        string            `json:"last_error,omitempty"`
	MessagesReceived int64             `json:"messages_received,omitempty"`
	MessagesSent     int64             `json:"messages_sent,omitempty"`
	LastMessageAt    *string           `json:"last_message_at,omitempty"`
	LastReplyAt      *string           `json:"last_reply_at,omitempty"`
}

// ListChannelConfigs lists all channel configurations with runtime stats.
func (h *ChannelConfigHandler) ListChannelConfigs(c echo.Context) error {
	configs := h.store.List()
	responses := make([]*ChannelConfigResponse, 0, len(configs))

	for _, cfg := range configs {
		resp := &ChannelConfigResponse{
			ID:        cfg.ID,
			Enabled:   cfg.Enabled,
			Status:    cfg.Status,
			Config:    cfg.Config,
			LastError: cfg.LastError,
		}

		// Merge runtime stats from channel manager if available
		if h.manager != nil {
			if ch, exists := h.manager.Get(cfg.ID); exists {
				info := ch.Info()
				resp.Status = string(info.Status)
				resp.MessagesReceived = info.MessagesReceived
				resp.MessagesSent = info.MessagesSent
				if info.LastMessageAt != nil {
					t := info.LastMessageAt.Format("2006-01-02T15:04:05Z07:00")
					resp.LastMessageAt = &t
				}
				if info.LastReplyAt != nil {
					t := info.LastReplyAt.Format("2006-01-02T15:04:05Z07:00")
					resp.LastReplyAt = &t
				}
				if info.LastError != "" {
					resp.LastError = info.LastError
				}
			}
		}

		responses = append(responses, resp)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"channels": responses,
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

	// Get existing config to check previous enabled state
	oldCfg, _ := h.store.Get(id)
	wasEnabled := oldCfg != nil && oldCfg.Enabled

	cfg := &ChannelConfig{
		ID:      id,
		Enabled: req.Enabled,
		Status:  "disconnected",
		Config:  req.Config,
	}

	// Save configuration first
	if err := h.store.Set(id, cfg); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save configuration")
	}

	// Handle connection lifecycle if manager is available
	if h.manager != nil {
		ctx := c.Request().Context()

		// If was enabled, stop and unregister the old connection first
		if wasEnabled {
			if ch, exists := h.manager.Get(id); exists {
				if ch.IsConnected() {
					_ = h.manager.StopChannel(ctx, id)
				}
				_ = h.manager.Unregister(id)
			}
		}

		// If now enabled, create, register and start the connection
		if req.Enabled {
			cfg.Status = "connecting"

			// Create and register channel if factory is available
			if h.factory != nil {
				ch, err := h.factory.CreateChannel(cfg)
				if err != nil {
					cfg.Status = "error"
					cfg.LastError = err.Error()
					_ = h.store.Set(id, cfg)
					return c.JSON(http.StatusOK, map[string]interface{}{
						"success": true,
						"message": "Configuration saved but failed to create channel",
						"channel": cfg,
					})
				}
				if ch == nil {
					// Unknown channel type
					cfg.Status = "error"
					cfg.LastError = "unsupported channel type: " + id
					_ = h.store.Set(id, cfg)
					return c.JSON(http.StatusOK, map[string]interface{}{
						"success": true,
						"message": "Configuration saved but channel type not supported",
						"channel": cfg,
					})
				}
				if err := h.manager.Register(ch); err != nil {
					cfg.Status = "error"
					cfg.LastError = err.Error()
					_ = h.store.Set(id, cfg)
					return c.JSON(http.StatusOK, map[string]interface{}{
						"success": true,
						"message": "Configuration saved but failed to register channel",
						"channel": cfg,
					})
				}
			}

			// Start the channel
			if err := h.manager.StartChannel(ctx, id); err != nil {
				cfg.Status = "error"
				cfg.LastError = err.Error()
			} else {
				// Get actual status from manager
				if ch, exists := h.manager.Get(id); exists {
					info := ch.Info()
					cfg.Status = string(info.Status)
				}
			}
			// Update stored config with new status
			_ = h.store.Set(id, cfg)
		}
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

	wasEnabled := cfg.Enabled
	cfg.Enabled = req.Enabled
	cfg.LastError = "" // Clear previous error

	// Handle connection lifecycle if manager is available
	if h.manager != nil {
		ctx := c.Request().Context()

		if req.Enabled && !wasEnabled {
			// Enabling: create, register and start the connection
			cfg.Status = "connecting"

			// Check if channel is already registered
			_, exists := h.manager.Get(id)
			if !exists && h.factory != nil {
				// Create and register channel
				ch, err := h.factory.CreateChannel(cfg)
				if err != nil {
					cfg.Status = "error"
					cfg.LastError = err.Error()
					_ = h.store.Set(id, cfg)
					return c.JSON(http.StatusOK, map[string]interface{}{
						"success": false,
						"enabled": cfg.Enabled,
						"status":  cfg.Status,
						"channel": cfg,
					})
				}
				if ch == nil {
					// Unknown channel type
					cfg.Status = "error"
					cfg.LastError = "unsupported channel type: " + id
					_ = h.store.Set(id, cfg)
					return c.JSON(http.StatusOK, map[string]interface{}{
						"success": false,
						"enabled": cfg.Enabled,
						"status":  cfg.Status,
						"channel": cfg,
					})
				}
				if err := h.manager.Register(ch); err != nil {
					cfg.Status = "error"
					cfg.LastError = err.Error()
					_ = h.store.Set(id, cfg)
					return c.JSON(http.StatusOK, map[string]interface{}{
						"success": false,
						"enabled": cfg.Enabled,
						"status":  cfg.Status,
						"channel": cfg,
					})
				}
			}

			// Start the channel
			if err := h.manager.StartChannel(ctx, id); err != nil {
				cfg.Status = "error"
				cfg.LastError = err.Error()
			} else {
				// Get actual status from manager
				if ch, exists := h.manager.Get(id); exists {
					info := ch.Info()
					cfg.Status = string(info.Status)
				}
			}
		} else if !req.Enabled && wasEnabled {
			// Disabling: stop the connection
			if ch, exists := h.manager.Get(id); exists && ch.IsConnected() {
				if err := h.manager.StopChannel(ctx, id); err != nil {
					cfg.LastError = err.Error()
				}
			}
			cfg.Status = "disconnected"
		}
	} else {
		// No manager, just update status based on enabled state
		if req.Enabled {
			cfg.Status = "connecting"
		} else {
			cfg.Status = "disconnected"
		}
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
	g.POST("/setup/test-connection", h.TestConnection)
}

// TestConnectionRequest represents a request to test a channel connection.
type TestConnectionRequest struct {
	Type   string            `json:"type"`
	Config map[string]string `json:"config"`
}

// TestConnection tests a channel connection without saving configuration.
func (h *ChannelConfigHandler) TestConnection(c echo.Context) error {
	var req TestConnectionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Type == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Channel type is required",
		})
	}

	// Use the channel factory to validate the connection
	if h.factory == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"message": "Channel factory not initialized",
		})
	}

	// Test the connection using the factory's validator
	result := h.factory.ValidateConnection(c.Request().Context(), req.Type, req.Config)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": result.Success,
		"message": result.Message,
		"details": result.Details,
	})
}
