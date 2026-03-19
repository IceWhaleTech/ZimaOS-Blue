package companion

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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
	initFn   func() Streamer
	initMu   sync.Mutex
}

// NewWebSocketHandler creates a new WebSocket handler.
func NewWebSocketHandler(streamer Streamer, config *Config) *WebSocketHandler {
	return &WebSocketHandler{
		streamer: streamer,
		config:   config,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  config.WebSocket.ReadBufferSize,
			WriteBufferSize: config.WebSocket.WriteBufferSize,
			// Security fix: Use origin checker instead of allowing all origins
			CheckOrigin: security.CheckOriginDefault,
		},
		conns: make(map[*websocket.Conn]context.CancelFunc),
	}
}

// NewLazyWebSocketHandler creates a WebSocket handler that defers streamer
// creation until the first incoming companion stream request.
func NewLazyWebSocketHandler(initFn func() Streamer, config *Config) *WebSocketHandler {
	return &WebSocketHandler{
		config: config,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  config.WebSocket.ReadBufferSize,
			WriteBufferSize: config.WebSocket.WriteBufferSize,
			CheckOrigin:     security.CheckOriginDefault,
		},
		conns:  make(map[*websocket.Conn]context.CancelFunc),
		initFn: initFn,
	}
}

// RegisterRoutes registers the WebSocket routes.
func (h *WebSocketHandler) RegisterRoutes(e *echo.Echo) {
	h.RegisterGroupRoutes(e.Group("/api/v1"))
	h.RegisterCompatGroupRoutes(e.Group("/api"))
}

// RegisterGroupRoutes registers WebSocket routes on an existing API group.
func (h *WebSocketHandler) RegisterGroupRoutes(g *echo.Group) {
	g.GET("/companion/stream", h.HandleStream)
	g.GET("/companion/session/:id", h.HandleSessionStream)
}

// RegisterCompatGroupRoutes registers frontend-compatible WebSocket routes.
func (h *WebSocketHandler) RegisterCompatGroupRoutes(g *echo.Group) {
	g.GET("/companion/stream", h.HandleStream)
	g.GET("/companion/session/:id", h.HandleSessionStream)
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
	streamer := h.ensureStreamer()
	if streamer == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "companion service not available"})
	}

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
	eventCh, cleanup := streamer.Subscribe(sessionID)
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
			conn.SetWriteDeadline(timeutil.NowTime().Add(h.config.WebSocket.WriteTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return nil
			}
		case event, ok := <-eventCh:
			if !ok {
				return nil
			}
			conn.SetWriteDeadline(timeutil.NowTime().Add(h.config.WebSocket.WriteTimeout))
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

func (h *WebSocketHandler) ensureStreamer() Streamer {
	if h == nil {
		return nil
	}
	if h.streamer != nil || h.initFn == nil {
		return h.streamer
	}

	h.initMu.Lock()
	defer h.initMu.Unlock()

	if h.streamer == nil && h.initFn != nil {
		h.streamer = h.initFn()
	}
	return h.streamer
}

// readPump reads messages from the WebSocket connection.
func (h *WebSocketHandler) readPump(ctx context.Context, conn *websocket.Conn) {
	defer conn.Close()

	conn.SetReadLimit(512)
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(timeutil.NowTime().Add(60 * time.Second))
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

// Close closes all WebSocket connections gracefully.
// It sends a close message to clients before closing the connection,
// allowing them to reconnect automatically.
func (h *WebSocketHandler) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Send close message to all clients before closing
	// Use CloseGoingAway (1001) to indicate server restart/shutdown
	closeMsg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "Server is shutting down. Please reconnect in a moment.")

	for conn, cancel := range h.conns {
		// Send close message with a short deadline
		conn.SetWriteDeadline(timeutil.NowTime().Add(1 * time.Second))
		conn.WriteMessage(websocket.CloseMessage, closeMsg)

		// Cancel context and close connection
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
