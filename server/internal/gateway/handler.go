package gateway

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for gateway management.
type Handler struct {
	gateway *Gateway
	logger  *zap.Logger
}

// NewHandler creates a new gateway handler.
func NewHandler(gateway *Gateway, logger *zap.Logger) *Handler {
	return &Handler{
		gateway: gateway,
		logger:  logger.With(zap.String("handler", "gateway")),
	}
}

// RegisterRoutes registers gateway routes.
func (h *Handler) RegisterRoutes(e *echo.Echo, g *echo.Group) {
	// WebSocket endpoint
	e.GET("/ws", h.WebSocket)

	// API endpoints
	gateway := g.Group("/gateway")
	gateway.GET("/status", h.Status)
	gateway.GET("/connections", h.ListConnections)
	gateway.GET("/connections/:id", h.GetConnection)
	gateway.DELETE("/connections/:id", h.CloseConnection)
}

// WebSocket handles WebSocket upgrade requests.
// @Summary WebSocket gateway
// @Tags gateway
// @Router /ws [get]
func (h *Handler) WebSocket(c echo.Context) error {
	h.gateway.HandleWebSocket(c.Response().Writer, c.Request())
	return nil
}

// Status returns gateway status.
// @Summary Get gateway status
// @Tags gateway
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/gateway/status [get]
func (h *Handler) Status(c echo.Context) error {
	stats := h.gateway.Stats()
	return c.JSON(http.StatusOK, stats)
}

// ListConnections returns all connections.
// @Summary List gateway connections
// @Tags gateway
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /api/v1/gateway/connections [get]
func (h *Handler) ListConnections(c echo.Context) error {
	connections := h.gateway.GetConnections()
	result := make([]map[string]interface{}, len(connections))
	for i, conn := range connections {
		result[i] = conn.Info()
	}
	return c.JSON(http.StatusOK, result)
}

// GetConnection returns a connection by ID.
// @Summary Get gateway connection
// @Tags gateway
// @Produce json
// @Param id path string true "Connection ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /api/v1/gateway/connections/{id} [get]
func (h *Handler) GetConnection(c echo.Context) error {
	id := c.Param("id")

	conn, exists := h.gateway.GetConnection(id)
	if !exists {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "connection not found"})
	}

	return c.JSON(http.StatusOK, conn.Info())
}

// CloseConnection closes a connection.
// @Summary Close gateway connection
// @Tags gateway
// @Param id path string true "Connection ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/gateway/connections/{id} [delete]
func (h *Handler) CloseConnection(c echo.Context) error {
	id := c.Param("id")

	conn, exists := h.gateway.GetConnection(id)
	if !exists {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "connection not found"})
	}

	conn.Close()
	return c.JSON(http.StatusOK, map[string]string{"status": "closed"})
}
