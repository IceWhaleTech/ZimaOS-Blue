package companion

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// WebSocketHandler handles WebSocket connections for real-time event streaming.
type WebSocketHandler struct {
	streamer Streamer
	config   *Config
	upgrader websocket.Upgrader
	mu       sync.RWMutex
	conns    map[*websocket.Conn]context.CancelFunc
}

// NewWebSocketHandler creates a new WebSocket handler.
func NewWebSocketHandler(streamer Streamer, config *Config) *WebSocketHandler {
	return &WebSocketHandler{
		streamer: streamer,
		config:   config,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  config.WebSocket.ReadBufferSize,
			WriteBufferSize: config.WebSocket.WriteBufferSize,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
		conns: make(map[*websocket.Conn]context.CancelFunc),
	}
}

// RegisterRoutes registers the WebSocket routes.
func (h *WebSocketHandler) RegisterRoutes(e *echo.Echo) {
	// Register under /api/v1/companion
	e.GET("/api/v1/companion/stream", h.HandleStream)
	e.GET("/api/v1/companion/session/:id", h.HandleSessionStream)

	// Also register under /api/companion for frontend compatibility
	e.GET("/api/companion/stream", h.HandleStream)
	e.GET("/api/companion/session/:id", h.HandleSessionStream)
}

// HandleStream handles the main event stream WebSocket endpoint.
// GET /api/v1/companion/stream
func (h *WebSocketHandler) HandleStream(c echo.Context) error {
	return h.handleWebSocket(c, "")
}

// HandleSessionStream handles a single session event stream.
// GET /api/v1/companion/session/:id
func (h *WebSocketHandler) HandleSessionStream(c echo.Context) error {
	sessionID := c.Param("id")
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session ID required"})
	}
	return h.handleWebSocket(c, sessionID)
}

// handleWebSocket handles WebSocket connection setup and event streaming.
func (h *WebSocketHandler) handleWebSocket(c echo.Context, sessionID string) error {
	conn, err := h.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(c.Request().Context())

	h.mu.Lock()
	h.conns[conn] = cancel
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.conns, conn)
		h.mu.Unlock()
		cancel()
		conn.Close()
	}()

	// Subscribe to events
	eventCh, cleanup := h.streamer.Subscribe(sessionID)
	defer cleanup()

	// Start ping ticker
	pingTicker := time.NewTicker(h.config.WebSocket.PingInterval)
	defer pingTicker.Stop()

	// Start read goroutine to handle pongs and close messages
	go h.readPump(ctx, conn)

	// Write loop
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(h.config.WebSocket.WriteTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return nil
			}
		case event, ok := <-eventCh:
			if !ok {
				return nil
			}
			conn.SetWriteDeadline(time.Now().Add(h.config.WebSocket.WriteTimeout))
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return nil
			}
		}
	}
}

// readPump reads messages from the WebSocket connection.
func (h *WebSocketHandler) readPump(ctx context.Context, conn *websocket.Conn) {
	defer conn.Close()

	conn.SetReadLimit(512)
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}
}

// Close closes all WebSocket connections.
func (h *WebSocketHandler) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for conn, cancel := range h.conns {
		cancel()
		conn.Close()
	}
	h.conns = make(map[*websocket.Conn]context.CancelFunc)
}

// ConnectionCount returns the number of active WebSocket connections.
func (h *WebSocketHandler) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}
