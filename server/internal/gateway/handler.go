package gateway

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/auth"
)

// Handler provides HTTP handlers for gateway management.
type Handler struct {
	gateway    *Gateway
	logger     *zap.Logger
	jwtService *auth.JWTService
}

// NewHandler creates a new gateway handler.
func NewHandler(gateway *Gateway, logger *zap.Logger) *Handler {
	return &Handler{
		gateway: gateway,
		logger:  logger.With(zap.String("handler", "gateway")),
	}
}

// NewHandlerWithAuth creates a new gateway handler with JWT authentication.
func NewHandlerWithAuth(gateway *Gateway, logger *zap.Logger, jwtService *auth.JWTService) *Handler {
	return &Handler{
		gateway:    gateway,
		logger:     logger.With(zap.String("handler", "gateway")),
		jwtService: jwtService,
	}
}

// RegisterRoutes registers gateway routes.
func (h *Handler) RegisterRoutes(e *echo.Echo, g *echo.Group) {
	// WebSocket endpoint - with optional authentication
	// Security: Token can be passed via query param or Authorization header
	e.GET("/ws", h.WebSocket)

	// API endpoints
	gateway := g.Group("/gateway")
	gateway.GET("/status", h.Status)
	gateway.GET("/connections", h.ListConnections)
	gateway.GET("/connections/:id", h.GetConnection)
	gateway.DELETE("/connections/:id", h.CloseConnection)
}

// WebSocket handles WebSocket upgrade requests.
// Security: Validates JWT token from query parameter or Authorization header.
// @Summary WebSocket gateway
// @Tags gateway
// @Param token query string false "JWT token for authentication"
// @Router /ws [get]
func (h *Handler) WebSocket(c echo.Context) error {
	// Security fix: Validate authentication before WebSocket upgrade
	userID, err := h.authenticateWebSocket(c)
	if err != nil {
		h.logger.Warn("WebSocket authentication failed",
			zap.String("remote_addr", c.RealIP()),
			zap.Error(err))
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	// Store user ID in request context for the gateway to use
	if userID != "" {
		c.Set("user_id", userID)
	}

	h.gateway.HandleWebSocket(c.Response().Writer, c.Request())
	return nil
}

// authenticateWebSocket validates the WebSocket connection authentication.
// Returns the user ID if authenticated, or an error if authentication fails.
func (h *Handler) authenticateWebSocket(c echo.Context) (string, error) {
	// If no JWT service is configured, allow anonymous connections
	// This maintains backward compatibility for development
	if h.jwtService == nil {
		h.logger.Debug("WebSocket authentication skipped - no JWT service configured")
		return "", nil
	}

	// Try to get token from query parameter first (common for WebSocket)
	token := c.QueryParam("token")

	// If not in query param, try Authorization header
	if token == "" {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				token = parts[1]
			}
		}
	}

	// If no token found, reject the connection
	if token == "" {
		return "", auth.ErrInvalidToken
	}

	// Validate the token
	claims, err := h.jwtService.ValidateToken(token)
	if err != nil {
		return "", err
	}

	h.logger.Debug("WebSocket authenticated",
		zap.String("user_id", claims.UserID),
		zap.String("remote_addr", c.RealIP()))

	return claims.UserID, nil
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
