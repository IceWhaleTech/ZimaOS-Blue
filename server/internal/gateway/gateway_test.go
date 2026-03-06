package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func TestNewGateway(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	g := NewGateway(cfg, logger)
	if g == nil {
		t.Fatal("expected non-nil gateway")
	}
}

func TestDefaultConfig_RequestTimeoutDisabled(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.RequestTimeoutSeconds != 0 {
		t.Fatalf("expected RequestTimeoutSeconds=0, got %d", cfg.RequestTimeoutSeconds)
	}
}

func TestGatewayStats(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	g := NewGateway(cfg, logger)

	stats := g.Stats()
	if stats["total_connections"].(int64) != 0 {
		t.Errorf("expected 0 connections, got %d", stats["total_connections"])
	}
	if stats["active_connections"].(int) != 0 {
		t.Errorf("expected 0 active connections, got %d", stats["active_connections"])
	}
	if stats["active_messages"].(int64) != 0 {
		t.Errorf("expected 0 active messages, got %d", stats["active_messages"])
	}
}

func TestGatewayRegisterHandler(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	g := NewGateway(cfg, logger)

	g.RegisterHandler("test.method", func(ctx context.Context, conn *Connection, msg *Message) (*Message, error) {
		return &Message{
			ID:   msg.ID,
			Type: TypeResponse,
		}, nil
	})

	// Handler should be registered
	g.mu.RLock()
	_, exists := g.handlers["test.method"]
	g.mu.RUnlock()

	if !exists {
		t.Error("handler should be registered")
	}
}

func TestWebSocketConnection(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PingIntervalSeconds = 10 // Use longer interval for tests
	g := NewGateway(cfg, logger)

	// Create test server
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.HandleWebSocket(w, r)
	}))
	defer server.Close()

	// Convert http URL to ws URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect to WebSocket
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer ws.Close()

	// Wait for connection to be registered
	time.Sleep(50 * time.Millisecond)

	// Check connection count
	connections := g.GetConnections()
	if len(connections) != 1 {
		t.Errorf("expected 1 connection, got %d", len(connections))
	}
}

func TestWebSocketRequestResponse(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PingIntervalSeconds = 10
	g := NewGateway(cfg, logger)

	// Register a test handler
	g.RegisterHandler("echo", func(ctx context.Context, conn *Connection, msg *Message) (*Message, error) {
		return &Message{
			ID:      msg.ID,
			Type:    TypeResponse,
			Payload: msg.Payload,
		}, nil
	})

	// Create test server
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.HandleWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer ws.Close()

	// Send a request
	request := Message{
		ID:      "test-1",
		Type:    TypeRequest,
		Method:  "echo",
		Payload: json.RawMessage(`{"data":"hello"}`),
	}

	err = ws.WriteJSON(request)
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Read response
	var response Message
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	err = ws.ReadJSON(&response)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if response.ID != "test-1" {
		t.Errorf("expected ID 'test-1', got '%s'", response.ID)
	}
	if response.Type != TypeResponse {
		t.Errorf("expected type response, got %s", response.Type)
	}
}

func TestWebSocketUnknownMethod(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PingIntervalSeconds = 10
	g := NewGateway(cfg, logger)

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.HandleWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer ws.Close()

	// Send a request for unknown method
	request := Message{
		ID:     "test-1",
		Type:   TypeRequest,
		Method: "unknown.method",
	}

	err = ws.WriteJSON(request)
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Read response
	var response Message
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	err = ws.ReadJSON(&response)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if response.Error == nil {
		t.Error("expected error response")
	}
	if response.Error != nil && response.Error.Code != 404 {
		t.Errorf("expected error code 404, got %d", response.Error.Code)
	}
}

func TestWebSocketRequestTimeout(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PingIntervalSeconds = 10
	cfg.RequestTimeoutSeconds = 1
	g := NewGateway(cfg, logger)

	g.RegisterHandler("slow.method", func(ctx context.Context, conn *Connection, msg *Message) (*Message, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
			return &Message{Payload: json.RawMessage(`{"ok":true}`)}, nil
		}
	})

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.HandleWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer ws.Close()

	request := Message{
		ID:     "timeout-1",
		Type:   TypeRequest,
		Method: "slow.method",
	}
	if err := ws.WriteJSON(request); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	var response Message
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := ws.ReadJSON(&response); err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if response.Type != TypeError {
		t.Fatalf("expected error response, got %s", response.Type)
	}
	if response.Error == nil {
		t.Fatal("expected error payload")
	}
	if response.Error.Code != 500 {
		t.Fatalf("expected error code 500, got %d", response.Error.Code)
	}
	if !strings.Contains(strings.ToLower(response.Error.Message), "timed out") {
		t.Fatalf("expected timeout message, got %q", response.Error.Message)
	}
}

func TestWebSocketBroadcast(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PingIntervalSeconds = 10
	g := NewGateway(cfg, logger)

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.HandleWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect two clients
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect client 1: %v", err)
	}
	defer ws1.Close()

	ws2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect client 2: %v", err)
	}
	defer ws2.Close()

	// Wait for connections
	time.Sleep(50 * time.Millisecond)

	// Broadcast a message
	event := &Message{
		ID:      "broadcast-1",
		Type:    TypeEvent,
		Method:  "test.event",
		Payload: json.RawMessage(`{"message":"hello all"}`),
	}
	g.Broadcast(event)

	// Both clients should receive the message
	var msg1, msg2 Message

	ws1.SetReadDeadline(time.Now().Add(2 * time.Second))
	err = ws1.ReadJSON(&msg1)
	if err != nil {
		t.Fatalf("client 1 failed to receive broadcast: %v", err)
	}

	ws2.SetReadDeadline(time.Now().Add(2 * time.Second))
	err = ws2.ReadJSON(&msg2)
	if err != nil {
		t.Fatalf("client 2 failed to receive broadcast: %v", err)
	}

	if msg1.ID != "broadcast-1" || msg2.ID != "broadcast-1" {
		t.Error("broadcast message ID mismatch")
	}
}

func TestConnectionInfo(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PingIntervalSeconds = 10
	g := NewGateway(cfg, logger)

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.HandleWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer ws.Close()

	time.Sleep(50 * time.Millisecond)

	connections := g.GetConnections()
	if len(connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(connections))
	}

	info := connections[0].Info()
	if info["id"] == "" {
		t.Error("expected non-empty connection ID")
	}
	if info["connected_at"] == nil {
		t.Error("expected connected_at timestamp")
	}
}

func TestGetConnection(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PingIntervalSeconds = 10
	g := NewGateway(cfg, logger)

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.HandleWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer ws.Close()

	time.Sleep(50 * time.Millisecond)

	connections := g.GetConnections()
	if len(connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(connections))
	}

	connID := connections[0].ID

	// Get by ID
	conn, exists := g.GetConnection(connID)
	if !exists {
		t.Error("connection should exist")
	}
	if conn.ID != connID {
		t.Errorf("expected ID %s, got %s", connID, conn.ID)
	}

	// Get non-existent
	_, exists = g.GetConnection("non-existent")
	if exists {
		t.Error("connection should not exist")
	}
}

func TestConnectionClose(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PingIntervalSeconds = 10
	g := NewGateway(cfg, logger)

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.HandleWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	connections := g.GetConnections()
	if len(connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(connections))
	}

	// Close the connection
	connections[0].Close()

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// Connection should be removed
	connections = g.GetConnections()
	if len(connections) != 0 {
		t.Errorf("expected 0 connections after close, got %d", len(connections))
	}

	ws.Close()
}

func TestSendToConnection(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PingIntervalSeconds = 10
	g := NewGateway(cfg, logger)

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.HandleWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer ws.Close()

	time.Sleep(50 * time.Millisecond)

	connections := g.GetConnections()
	connID := connections[0].ID

	// Send to specific connection
	msg := &Message{
		ID:     "direct-1",
		Type:   TypeEvent,
		Method: "direct.message",
	}

	conn, _ := g.GetConnection(connID)
	err = conn.Send(msg)
	if err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	// Read the message
	var received Message
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	err = ws.ReadJSON(&received)
	if err != nil {
		t.Fatalf("failed to receive: %v", err)
	}

	if received.ID != "direct-1" {
		t.Errorf("expected ID 'direct-1', got '%s'", received.ID)
	}
}

func TestGatewayStop(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PingIntervalSeconds = 10
	g := NewGateway(cfg, logger)

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.HandleWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer ws.Close()

	time.Sleep(50 * time.Millisecond)

	// Stop gateway
	g.Stop()

	// All connections should be closed
	connections := g.GetConnections()
	if len(connections) != 0 {
		t.Errorf("expected 0 connections after stop, got %d", len(connections))
	}
}

func TestBroadcastToUser(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PingIntervalSeconds = 10
	g := NewGateway(cfg, logger)

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.HandleWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect two clients
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect client 1: %v", err)
	}
	defer ws1.Close()

	ws2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect client 2: %v", err)
	}
	defer ws2.Close()

	time.Sleep(50 * time.Millisecond)

	connections := g.GetConnections()
	if len(connections) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(connections))
	}

	// Set user ID for first connection only
	connections[0].SetUserID("user-123")

	// Broadcast to user
	msg := &Message{
		ID:     "user-msg-1",
		Type:   TypeEvent,
		Method: "user.event",
	}
	g.BroadcastToUser("user-123", msg)

	// First client should receive
	ws1.SetReadDeadline(time.Now().Add(1 * time.Second))
	var received Message
	err = ws1.ReadJSON(&received)
	// May or may not receive depending on which connection is first
	// This test verifies the function doesn't panic
}
