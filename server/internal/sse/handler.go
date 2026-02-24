package sse

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
)

// Handler serves the SSE event stream endpoint.
type Handler struct {
	broker    *Broker
	keepalive time.Duration
}

// NewHandler creates a new SSE handler.
func NewHandler(broker *Broker) *Handler {
	return &Handler{
		broker:    broker,
		keepalive: 30 * time.Second,
	}
}

// RegisterRoutes registers the SSE endpoint on the given echo group.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/events", h.Stream)
}

// Stream handles GET /api/v1/events — authenticated SSE endpoint.
func (h *Handler) Stream(c echo.Context) error {
	// Extract user ID from JWT claims
	userID := ""
	if claims, ok := c.Get("user").(*auth.Claims); ok && claims != nil {
		userID = claims.UserID
	}
	if userID == "" {
		userID = "default"
	}

	// Set SSE headers
	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.Writer.(http.Flusher)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "streaming not supported")
	}

	// Disable the server's WriteTimeout for this long-lived SSE connection.
	// Without this, net/http closes the connection after WriteTimeout (default 30s).
	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{}) // zero = no deadline

	// Subscribe to events
	ch := h.broker.Subscribe(userID)
	defer h.broker.Unsubscribe(userID, ch)

	ctx := c.Request().Context()
	ticker := time.NewTicker(h.keepalive)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case evt := <-ch:
			data := evt.MarshalData()
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, data); err != nil {
				return nil // client disconnected
			}
			flusher.Flush()

		case <-ticker.C:
			// Keepalive comment to prevent proxy/browser timeout
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return nil
			}
			flusher.Flush()
		}
	}
}
