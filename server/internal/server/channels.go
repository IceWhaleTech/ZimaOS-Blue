package server

import (
	"net/http"

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
