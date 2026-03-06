// Package gateway provides a WebSocket-based gateway for real-time communication.
package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// MessageType represents the type of gateway message.
type MessageType string

const (
	// TypeRequest is a request message.
	TypeRequest MessageType = "request"
	// TypeResponse is a response message.
	TypeResponse MessageType = "response"
	// TypeEvent is an event message.
	TypeEvent MessageType = "event"
	// TypeError is an error message.
	TypeError MessageType = "error"
)

// Message represents a gateway message.
type Message struct {
	ID        string          `json:"id"`
	Type      MessageType     `json:"type"`
	Method    string          `json:"method,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     *ErrorPayload   `json:"error,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// ErrorPayload represents an error in a message.
type ErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// RequestHandler handles gateway requests.
type RequestHandler func(ctx context.Context, conn *Connection, msg *Message) (*Message, error)

// Config contains gateway configuration.
type Config struct {
	// Enabled indicates if the gateway is enabled.
	Enabled bool `yaml:"enabled"`
	// ReadBufferSize is the WebSocket read buffer size.
	ReadBufferSize int `yaml:"read_buffer_size"`
	// WriteBufferSize is the WebSocket write buffer size.
	WriteBufferSize int `yaml:"write_buffer_size"`
	// MaxMessageSize is the maximum message size in bytes.
	MaxMessageSize int64 `yaml:"max_message_size"`
	// PingInterval is the interval for ping messages.
	PingIntervalSeconds int `yaml:"ping_interval_seconds"`
	// PongTimeout is the timeout for pong responses.
	PongTimeoutSeconds int `yaml:"pong_timeout_seconds"`
	// WriteTimeout is the timeout for write operations.
	WriteTimeoutSeconds int `yaml:"write_timeout_seconds"`
	// RequestTimeout is the timeout for a single gateway request.
	// 0 disables hard request timeout and relies on connection cancellation.
	RequestTimeoutSeconds int `yaml:"request_timeout_seconds"`
	// MaxConnections is the maximum number of connections.
	MaxConnections int `yaml:"max_connections"`
}

// DefaultConfig returns the default gateway configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:               true,
		ReadBufferSize:        1024,
		WriteBufferSize:       1024,
		MaxMessageSize:        512 * 1024, // 512KB
		PingIntervalSeconds:   30,
		PongTimeoutSeconds:    60,
		WriteTimeoutSeconds:   10,
		RequestTimeoutSeconds: 0,
		MaxConnections:        1000,
	}
}

// Gateway manages WebSocket connections.
type Gateway struct {
	config   Config
	logger   *zap.Logger
	upgrader websocket.Upgrader

	connections map[string]*Connection
	handlers    map[string]RequestHandler
	mu          sync.RWMutex

	// Statistics
	totalConnections atomic.Int64
	activeMessages   atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
}

// NewGateway creates a new gateway.
func NewGateway(cfg Config, logger *zap.Logger) *Gateway {
	ctx, cancel := context.WithCancel(context.Background())

	return &Gateway{
		config: cfg,
		logger: logger.With(zap.String("component", "gateway")),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  cfg.ReadBufferSize,
			WriteBufferSize: cfg.WriteBufferSize,
			// Security fix: Use origin checker instead of allowing all origins
			CheckOrigin: security.CheckOriginDefault,
		},
		connections: make(map[string]*Connection),
		handlers:    make(map[string]RequestHandler),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// RegisterHandler registers a request handler.
func (g *Gateway) RegisterHandler(method string, handler RequestHandler) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.handlers[method] = handler
	g.logger.Debug("handler registered", zap.String("method", method))
}

// HandleWebSocket handles WebSocket upgrade requests.
func (g *Gateway) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check connection limit
	g.mu.RLock()
	if len(g.connections) >= g.config.MaxConnections {
		g.mu.RUnlock()
		http.Error(w, "too many connections", http.StatusServiceUnavailable)
		return
	}
	g.mu.RUnlock()

	// Upgrade connection
	ws, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		g.logger.Error("failed to upgrade connection", zap.Error(err))
		return
	}

	// Create connection
	conn := NewConnection(ws, g.config, g.logger)
	if raw := r.Context().Value("user_id"); raw != nil {
		if userID, ok := raw.(string); ok {
			if userID = strings.TrimSpace(userID); userID != "" {
				conn.SetUserID(userID)
			}
		}
	}

	// Register connection
	g.mu.Lock()
	g.connections[conn.ID] = conn
	g.mu.Unlock()

	g.totalConnections.Add(1)

	g.logger.Info("client connected",
		zap.String("conn_id", conn.ID),
		zap.String("remote_addr", r.RemoteAddr))

	// Handle connection
	go g.handleConnection(conn)
}

// handleConnection handles a single connection.
func (g *Gateway) handleConnection(conn *Connection) {
	defer func() {
		conn.Close()
		g.mu.Lock()
		delete(g.connections, conn.ID)
		g.mu.Unlock()

		g.logger.Info("client disconnected", zap.String("conn_id", conn.ID))
	}()

	// Set read limit
	conn.ws.SetReadLimit(g.config.MaxMessageSize)

	// Set pong handler
	conn.ws.SetPongHandler(func(string) error {
		conn.ws.SetReadDeadline(timeutil.NowTime().Add(time.Duration(g.config.PongTimeoutSeconds) * time.Second))
		return nil
	})

	// Start ping goroutine
	go g.pingLoop(conn)

	// Read messages
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-conn.done:
			return
		default:
		}

		// Set read deadline
		conn.ws.SetReadDeadline(timeutil.NowTime().Add(time.Duration(g.config.PongTimeoutSeconds) * time.Second))

		// Read message
		_, data, err := conn.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				g.logger.Error("read error", zap.String("conn_id", conn.ID), zap.Error(err))
			}
			return
		}

		// Parse message
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			g.logger.Error("failed to parse message",
				zap.String("conn_id", conn.ID),
				zap.Error(err))
			g.sendError(conn, "", 400, "invalid message format")
			continue
		}

		// Handle message
		go g.handleMessage(conn, &msg)
	}
}

// handleMessage handles a single message.
func (g *Gateway) handleMessage(conn *Connection, msg *Message) {
	g.activeMessages.Add(1)
	defer g.activeMessages.Add(-1)
	conn.MessagesReceived.Add(1)

	if msg.Type != TypeRequest {
		// Frontend heartbeat may send {"type":"ping"}.
		if strings.EqualFold(string(msg.Type), "ping") {
			_ = conn.Send(&Message{
				Type:      TypeResponse,
				Timestamp: timeutil.NowMilli(),
			})
		}
		return
	}

	g.logger.Debug("received message",
		zap.String("conn_id", conn.ID),
		zap.String("msg_id", msg.ID),
		zap.String("method", msg.Method))

	if strings.TrimSpace(msg.Method) == "" {
		g.sendError(conn, msg.ID, 400, "method is required")
		return
	}

	// Get handler
	g.mu.RLock()
	handler, exists := g.handlers[msg.Method]
	g.mu.RUnlock()

	if !exists {
		g.sendError(conn, msg.ID, 404, fmt.Sprintf("method not found: %s", msg.Method))
		return
	}

	// Create request context bound to gateway/connection lifecycle, with optional timeout.
	ctx, cancel := g.newRequestContext(conn)
	defer cancel()

	// Call handler
	response, err := handler(ctx, conn, msg)
	if err != nil {
		g.logger.Error("handler error",
			zap.String("conn_id", conn.ID),
			zap.String("method", msg.Method),
			zap.Error(err))
		if errors.Is(err, context.Canceled) && ctx.Err() == context.Canceled {
			// Connection closed/cancelled — no user-facing error needed.
			return
		}
		msgText := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			if g.config.RequestTimeoutSeconds > 0 {
				msgText = fmt.Sprintf("request timed out after %ds: %v", g.config.RequestTimeoutSeconds, err)
			} else {
				msgText = fmt.Sprintf("request timed out: %v", err)
			}
		}
		g.sendError(conn, msg.ID, 500, msgText)
		return
	}

	// Send response
	if response != nil {
		response.ID = msg.ID
		response.Type = TypeResponse
		response.Timestamp = timeutil.NowMilli()
		conn.Send(response)
	}
}

func (g *Gateway) newRequestContext(conn *Connection) (context.Context, context.CancelFunc) {
	baseCtx, baseCancel := context.WithCancel(g.ctx)
	go func() {
		select {
		case <-conn.done:
			baseCancel()
		case <-baseCtx.Done():
		}
	}()
	if g.config.RequestTimeoutSeconds <= 0 {
		return baseCtx, baseCancel
	}
	timeoutCtx, timeoutCancel := context.WithTimeout(baseCtx, time.Duration(g.config.RequestTimeoutSeconds)*time.Second)
	return timeoutCtx, func() {
		timeoutCancel()
		baseCancel()
	}
}

// pingLoop sends periodic ping messages.
func (g *Gateway) pingLoop(conn *Connection) {
	ticker := time.NewTicker(time.Duration(g.config.PingIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-conn.done:
			return
		case <-ticker.C:
			conn.ws.SetWriteDeadline(timeutil.NowTime().Add(time.Duration(g.config.WriteTimeoutSeconds) * time.Second))
			if err := conn.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// sendError sends an error message.
func (g *Gateway) sendError(conn *Connection, msgID string, code int, message string) {
	errMsg := &Message{
		ID:   msgID,
		Type: TypeError,
		Error: &ErrorPayload{
			Code:    code,
			Message: message,
		},
		Timestamp: timeutil.NowMilli(),
	}
	conn.Send(errMsg)
}

// Broadcast sends a message to all connections.
func (g *Gateway) Broadcast(msg *Message) {
	g.mu.RLock()
	connections := make([]*Connection, 0, len(g.connections))
	for _, conn := range g.connections {
		connections = append(connections, conn)
	}
	g.mu.RUnlock()

	for _, conn := range connections {
		conn.Send(msg)
	}
}

// BroadcastToUser sends a message to all connections for a user.
func (g *Gateway) BroadcastToUser(userID string, msg *Message) {
	g.mu.RLock()
	connections := make([]*Connection, 0)
	for _, conn := range g.connections {
		if conn.UserID == userID {
			connections = append(connections, conn)
		}
	}
	g.mu.RUnlock()

	for _, conn := range connections {
		conn.Send(msg)
	}
}

// GetConnection returns a connection by ID.
func (g *Gateway) GetConnection(id string) (*Connection, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	conn, exists := g.connections[id]
	return conn, exists
}

// GetConnections returns all connections.
func (g *Gateway) GetConnections() []*Connection {
	g.mu.RLock()
	defer g.mu.RUnlock()

	connections := make([]*Connection, 0, len(g.connections))
	for _, conn := range g.connections {
		connections = append(connections, conn)
	}
	return connections
}

// Stats returns gateway statistics.
func (g *Gateway) Stats() map[string]interface{} {
	g.mu.RLock()
	activeConnections := len(g.connections)
	g.mu.RUnlock()

	return map[string]interface{}{
		"total_connections":  g.totalConnections.Load(),
		"active_connections": activeConnections,
		"active_messages":    g.activeMessages.Load(),
	}
}

// Stop stops the gateway.
func (g *Gateway) Stop() {
	g.cancel()

	g.mu.Lock()
	for _, conn := range g.connections {
		conn.Close()
	}
	g.connections = make(map[string]*Connection)
	g.mu.Unlock()

	g.logger.Info("gateway stopped")
}

// Connection represents a WebSocket connection.
type Connection struct {
	ID        string
	UserID    string
	ws        *websocket.Conn
	config    Config
	logger    *zap.Logger
	sendChan  chan *Message
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex

	// Metadata
	Metadata map[string]interface{}

	// Statistics
	MessagesSent     atomic.Int64
	MessagesReceived atomic.Int64
	ConnectedAt      time.Time
}

// NewConnection creates a new connection.
func NewConnection(ws *websocket.Conn, cfg Config, logger *zap.Logger) *Connection {
	conn := &Connection{
		ID:          generateSecureConnectionID(),
		ws:          ws,
		config:      cfg,
		logger:      logger,
		sendChan:    make(chan *Message, 256),
		done:        make(chan struct{}),
		Metadata:    make(map[string]interface{}),
		ConnectedAt: timeutil.NowTime(),
	}

	// Start send goroutine
	go conn.sendLoop()

	return conn
}

// generateSecureConnectionID generates a cryptographically secure connection ID.
// Security fix: Use crypto/rand instead of time-based IDs to prevent enumeration attacks.
func generateSecureConnectionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based ID if crypto/rand fails (should never happen)
		return fmt.Sprintf("conn_%d", timeutil.NowNano())
	}
	return fmt.Sprintf("conn_%s", hex.EncodeToString(b))
}

// Send sends a message to the connection.
func (c *Connection) Send(msg *Message) error {
	select {
	case c.sendChan <- msg:
		return nil
	case <-c.done:
		return fmt.Errorf("connection closed")
	default:
		return fmt.Errorf("send buffer full")
	}
}

// sendLoop handles sending messages.
func (c *Connection) sendLoop() {
	for {
		select {
		case <-c.done:
			return
		case msg := <-c.sendChan:
			c.mu.Lock()
			c.ws.SetWriteDeadline(timeutil.NowTime().Add(time.Duration(c.config.WriteTimeoutSeconds) * time.Second))
			err := c.ws.WriteJSON(msg)
			c.mu.Unlock()

			if err != nil {
				c.logger.Error("failed to send message",
					zap.String("conn_id", c.ID),
					zap.Error(err))
				return
			}

			c.MessagesSent.Add(1)
		}
	}
}

// Close closes the connection.
func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.ws.Close()
	})
}

// SetUserID sets the user ID for the connection.
func (c *Connection) SetUserID(userID string) {
	c.UserID = userID
}

// Info returns connection information.
func (c *Connection) Info() map[string]interface{} {
	return map[string]interface{}{
		"id":                c.ID,
		"user_id":           c.UserID,
		"connected_at":      c.ConnectedAt,
		"messages_sent":     c.MessagesSent.Load(),
		"messages_received": c.MessagesReceived.Load(),
		"metadata":          c.Metadata,
	}
}
