package server

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
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
		req.Message = "Test message from ZimaOS-Blue"
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

const channelsKVKey = "config:channels"
const channelSettingsKVKey = "config:channels:settings"

// ChannelSettings contains global channel settings that apply across integrations.
type ChannelSettings struct {
	GroupAccess channel.GroupAccessConfig `json:"group_access"`
}

// DefaultChannelSettings returns the default global channel settings.
func DefaultChannelSettings() ChannelSettings {
	return ChannelSettings{
		GroupAccess: channel.DefaultGroupAccessConfig(),
	}
}

// ApplyChannelSettings overlays persisted global channel settings onto a base config.
func ApplyChannelSettings(cfg channel.Config, settings ChannelSettings) channel.Config {
	cfg.GroupAccess = normalizeChannelSettings(settings).GroupAccess
	return cfg
}

// ChannelConfigStore stores channel configurations persistently.
type ChannelConfigStore struct {
	kv          kvstore.Store
	mu          sync.RWMutex
	configs     map[string]*ChannelConfig
	settings    ChannelSettings
	hasSettings bool
}

// ChannelConfig represents a channel's configuration.
type ChannelConfig struct {
	ID           string            `json:"id"`
	Enabled      bool              `json:"enabled"`
	Status       string            `json:"status"`
	Config       map[string]string `json:"config"`
	LastError    string            `json:"last_error,omitempty"`
	LastErrorKey string            `json:"last_error_key,omitempty"`
	// Persistent statistics
	MessagesReceived int64   `json:"messages_received,omitempty"`
	MessagesSent     int64   `json:"messages_sent,omitempty"`
	LastMessageAt    *string `json:"last_message_at,omitempty"`
	LastReplyAt      *string `json:"last_reply_at,omitempty"`
}

// NewChannelConfigStore creates a new channel config store.
func NewChannelConfigStore(kv kvstore.Store) *ChannelConfigStore {
	store := &ChannelConfigStore{
		kv:       kv,
		configs:  make(map[string]*ChannelConfig),
		settings: DefaultChannelSettings(),
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

// GetSettings returns stored global channel settings and whether they were explicitly persisted.
func (s *ChannelConfigStore) GetSettings() (ChannelSettings, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return normalizeChannelSettings(s.settings), s.hasSettings
}

// SetSettings saves global channel settings.
func (s *ChannelConfigStore) SetSettings(settings ChannelSettings) error {
	normalized, err := validateAndNormalizeChannelSettings(settings)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = normalized
	s.hasSettings = true
	return s.saveSettings()
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
	var configs map[string]*ChannelConfig
	if err := s.kv.GetJSON(context.Background(), channelsKVKey, &configs); err != nil {
		configs = nil
	}
	if configs != nil {
		s.configs = configs
	}

	var settings ChannelSettings
	if err := s.kv.GetJSON(context.Background(), channelSettingsKVKey, &settings); err == nil {
		s.settings = normalizeChannelSettings(settings)
		s.hasSettings = true
	}
}

func (s *ChannelConfigStore) save() error {
	return s.kv.SetJSON(context.Background(), channelsKVKey, s.configs, 0)
}

func (s *ChannelConfigStore) saveSettings() error {
	return s.kv.SetJSON(context.Background(), channelSettingsKVKey, s.settings, 0)
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

// UpdateStats updates the statistics for a channel and persists them.
func (s *ChannelConfigStore) UpdateStats(id string, messagesReceived, messagesSent int64, lastMessageAt, lastReplyAt *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, ok := s.configs[id]
	if !ok {
		return nil // Channel not configured, skip
	}
	cfg.MessagesReceived = messagesReceived
	cfg.MessagesSent = messagesSent
	cfg.LastMessageAt = lastMessageAt
	cfg.LastReplyAt = lastReplyAt
	return s.save()
}

// ChannelConfigHandler handles channel configuration API endpoints.
type ChannelConfigHandler struct {
	store   *ChannelConfigStore
	manager *channel.Manager
	factory *ChannelFactory

	// Periodic stats persistence
	stopPersist chan struct{}
	persistOnce sync.Once
}

// NewChannelConfigHandler creates a new channel config handler.
func NewChannelConfigHandler(store *ChannelConfigStore) *ChannelConfigHandler {
	return &ChannelConfigHandler{store: store}
}

// SetManager sets the channel manager for connection lifecycle management.
func (h *ChannelConfigHandler) SetManager(manager *channel.Manager) {
	h.manager = manager
	// Register hook to persist stats when manager stops
	if manager != nil {
		manager.OnStop(h.persistAllStats)
		// Start periodic stats persistence
		h.startPeriodicPersist()
	}
}

// persistAllStats saves all channel statistics to persistent storage.
func (h *ChannelConfigHandler) persistAllStats() {
	if h.manager == nil {
		return
	}
	// Stop periodic persistence
	if h.stopPersist != nil {
		close(h.stopPersist)
	}
	h.doPeristStats()
}

// doPeristStats performs the actual stats persistence.
func (h *ChannelConfigHandler) doPeristStats() {
	if h.manager == nil {
		return
	}
	configs := h.store.List()
	for _, cfg := range configs {
		if ch, exists := h.manager.Get(cfg.ID); exists {
			info := ch.Info()
			// Accumulate stats: persisted + runtime
			totalReceived := cfg.MessagesReceived + info.MessagesReceived
			totalSent := cfg.MessagesSent + info.MessagesSent
			var lastMsgAt, lastReplyAt *string
			if info.LastMessageAt != nil {
				t := info.LastMessageAt.Format("2006-01-02T15:04:05Z07:00")
				lastMsgAt = &t
			} else {
				lastMsgAt = cfg.LastMessageAt
			}
			if info.LastReplyAt != nil {
				t := info.LastReplyAt.Format("2006-01-02T15:04:05Z07:00")
				lastReplyAt = &t
			} else {
				lastReplyAt = cfg.LastReplyAt
			}
			h.store.UpdateStats(cfg.ID, totalReceived, totalSent, lastMsgAt, lastReplyAt)
		}
	}
}

// startPeriodicPersist starts a goroutine to periodically persist stats.
func (h *ChannelConfigHandler) startPeriodicPersist() {
	h.persistOnce.Do(func() {
		h.stopPersist = make(chan struct{})
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					h.doPeristStats()
				case <-h.stopPersist:
					return
				}
			}
		}()
	})
}

// SetFactory sets the channel factory for creating channel instances.
func (h *ChannelConfigHandler) SetFactory(factory *ChannelFactory) {
	h.factory = factory
}

// GetSettings returns global channel settings.
func (h *ChannelConfigHandler) GetSettings(c echo.Context) error {
	settings := DefaultChannelSettings()
	if h.manager != nil {
		settings.GroupAccess = h.manager.GetGroupAccess()
	} else if stored, ok := h.store.GetSettings(); ok {
		settings = stored
	}
	return c.JSON(http.StatusOK, settings)
}

// UpdateSettings updates global channel settings.
func (h *ChannelConfigHandler) UpdateSettings(c echo.Context) error {
	var req ChannelSettings
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	settings, err := validateAndNormalizeChannelSettings(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.store.SetSettings(settings); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save channel settings")
	}
	if h.manager != nil {
		h.manager.SetGroupAccess(settings.GroupAccess)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "Channel settings saved",
		"settings": settings,
	})
}

// ChannelConfigResponse represents a channel config with runtime stats.
type ChannelConfigResponse struct {
	ID               string            `json:"id"`
	Enabled          bool              `json:"enabled"`
	Available        bool              `json:"available"`
	Status           string            `json:"status"`
	Config           map[string]string `json:"config"`
	LastError        string            `json:"last_error,omitempty"`
	LastErrorKey     string            `json:"last_error_key,omitempty"`
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
			ID:               cfg.ID,
			Enabled:          cfg.Enabled,
			Available:        isChannelAvailable(cfg.ID),
			Status:           cfg.Status,
			Config:           cfg.Config,
			LastError:        cfg.LastError,
			LastErrorKey:     cfg.LastErrorKey,
			MessagesReceived: cfg.MessagesReceived,
			MessagesSent:     cfg.MessagesSent,
			LastMessageAt:    cfg.LastMessageAt,
			LastReplyAt:      cfg.LastReplyAt,
		}

		// Merge runtime stats from channel manager if available
		if h.manager != nil {
			if ch, exists := h.manager.Get(cfg.ID); exists {
				info := ch.Info()
				resp.Status = string(info.Status)
				// Use runtime stats if they are greater (accumulated during this session)
				if info.MessagesReceived > 0 || info.MessagesSent > 0 {
					// Add persisted stats to runtime stats for total count
					resp.MessagesReceived = cfg.MessagesReceived + info.MessagesReceived
					resp.MessagesSent = cfg.MessagesSent + info.MessagesSent
				}
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
				// Extract last_error_key from channel metadata (for frontend i18n)
				if info.Metadata != nil {
					if key, ok := info.Metadata["last_error_key"].(string); ok && key != "" {
						resp.LastErrorKey = key
					}
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

	// Block enabling channels that are not available on this platform
	if req.Enabled && !isChannelAvailable(id) {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   id + " is not available on this platform",
		})
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

	// Block enabling channels that are not available on this platform
	if req.Enabled && !isChannelAvailable(id) {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"enabled": false,
			"status":  "unavailable",
			"error":   id + " is not available on this platform",
		})
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
	g.GET("/channels/settings", h.GetSettings)
	g.PUT("/channels/settings", h.UpdateSettings)
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
		"success":     result.Success,
		"message":     result.Message,
		"message_key": result.MessageKey,
		"details":     result.Details,
	})
}

// isChannelAvailable returns whether a channel type is available on the current platform.
func isChannelAvailable(channelID string) bool {
	switch channelID {
	case "imessage":
		return runtime.GOOS == "darwin"
	default:
		return true
	}
}

func validateAndNormalizeChannelSettings(settings ChannelSettings) (ChannelSettings, error) {
	settings = normalizeChannelSettings(settings)
	switch settings.GroupAccess.Policy {
	case channel.GroupPolicyOpen, channel.GroupPolicyAllowlist, channel.GroupPolicyDisabled:
		return settings, nil
	default:
		return ChannelSettings{}, fmt.Errorf("invalid group access policy")
	}
}

func normalizeChannelSettings(settings ChannelSettings) ChannelSettings {
	normalized := DefaultChannelSettings()
	policy := channel.GroupPolicy(strings.ToLower(strings.TrimSpace(string(settings.GroupAccess.Policy))))
	if policy != "" {
		normalized.GroupAccess.Policy = policy
	}
	if len(settings.GroupAccess.AllowedChatIDs) == 0 {
		return normalized
	}

	allowed := make(map[string][]string)
	for channelName, chatIDs := range settings.GroupAccess.AllowedChatIDs {
		channelName = strings.TrimSpace(channelName)
		if channelName == "" {
			continue
		}
		seen := make(map[string]struct{})
		for _, chatID := range chatIDs {
			chatID = strings.TrimSpace(chatID)
			if chatID == "" {
				continue
			}
			if _, exists := seen[chatID]; exists {
				continue
			}
			seen[chatID] = struct{}{}
			allowed[channelName] = append(allowed[channelName], chatID)
		}
	}
	if len(allowed) > 0 {
		normalized.GroupAccess.AllowedChatIDs = allowed
	}
	return normalized
}
