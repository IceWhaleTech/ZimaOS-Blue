package mcp

import (
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler provides HTTP endpoints for the MCP server.
type Handler struct {
	server *Server
}

// NewHandler creates a new MCP HTTP handler.
func NewHandler(server *Server) *Handler {
	return &Handler{server: server}
}

// RegisterRoutes registers MCP routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/sse", h.SSE)
	g.POST("/message", h.Message)
}

// SSE handles GET /api/v1/mcp/sse — SSE stream for server→client messages.
func (h *Handler) SSE(c echo.Context) error {
	sess := h.server.CreateSession()
	defer h.server.RemoveSession(sess.ID)

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	flusher, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "streaming not supported")
	}

	// Send endpoint event so client knows where to POST messages
	endpoint := fmt.Sprintf("/api/v1/mcp/message?session_id=%s", sess.ID)
	fmt.Fprintf(c.Response(), "event: endpoint\ndata: %s\n\n", endpoint)
	flusher.Flush()

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-sess.done:
			return nil
		case msg, ok := <-sess.Messages:
			if !ok {
				return nil
			}
			fmt.Fprintf(c.Response(), "event: message\ndata: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// Message handles POST /api/v1/mcp/message — JSON-RPC from client→server.
func (h *Handler) Message(c echo.Context) error {
	sessionID := c.QueryParam("session_id")
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session_id required"})
	}

	body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxRequestBodySize+1))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read body"})
	}
	if len(body) > maxRequestBodySize {
		return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "request body too large"})
	}

	resp, err := h.server.HandleMessage(c.Request().Context(), sessionID, body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// For notifications (no response needed), return 202
	if resp == nil {
		return c.NoContent(http.StatusAccepted)
	}

	// Send response via SSE to the session
	h.server.SendToSession(sessionID, resp)

	// Also return the response directly (some clients expect this)
	c.Response().Header().Set("Content-Type", "application/json")
	return c.JSONBlob(http.StatusOK, resp)
}
