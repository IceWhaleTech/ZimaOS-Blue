package voice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
)

// Security fix: Use origin checker instead of allowing all origins
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 16,
	WriteBufferSize: 1024 * 16,
	CheckOrigin:     security.CheckOriginDefault,
}

// WSHandler handles WebSocket connections for voice streaming.
type WSHandler struct {
	service Service
}

// NewWSHandler creates a new WebSocket handler.
func NewWSHandler(service Service) *WSHandler {
	return &WSHandler{service: service}
}

// RegisterRoutes registers the WebSocket routes.
func (h *WSHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/stream", h.HandleStream)
}

// wsConnection represents a WebSocket connection.
type wsConnection struct {
	conn      *websocket.Conn
	sessionID string
	userID    string
	config    *VoiceConfig
	mu        sync.Mutex
	closed    bool
}

// HandleStream handles WebSocket connections for voice streaming.
func (h *WSHandler) HandleStream(c echo.Context) error {
	// Get user ID from context (set by auth middleware)
	userID := c.Get("user_id")
	userIDStr := "anonymous"
	if userID != nil {
		userIDStr = userID.(string)
	}

	// Get language from query param (default to "en")
	language := c.QueryParam("language")
	if language == "" {
		language = "en"
	}

	// Upgrade to WebSocket
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	// Create connection wrapper
	conn := &wsConnection{
		conn:   ws,
		userID: userIDStr,
		config: &VoiceConfig{
			Language:         language,
			Voice:            "alloy",
			AutoPlayResponse: true,
		},
	}

	// Create session
	session, err := h.service.CreateSession(c.Request().Context(), conn.userID, conn.config)
	if err != nil {
		conn.sendError("failed to create session: " + err.Error())
		return nil
	}
	conn.sessionID = session.ID

	// Send session info
	conn.sendMessage(&WebSocketMessage{
		Type: MsgTypeConfig,
		Data: map[string]interface{}{
			"session_id": session.ID,
			"config":     conn.config,
		},
	})

	// Handle messages
	h.handleConnection(c.Request().Context(), conn)

	// Cleanup
	h.service.CloseSession(context.Background(), conn.sessionID)

	return nil
}

// handleConnection handles the WebSocket connection lifecycle.
func (h *WSHandler) handleConnection(ctx context.Context, conn *wsConnection) {
	// Set up ping/pong
	conn.conn.SetPongHandler(func(string) error {
		conn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start ping ticker
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Message channel
	msgChan := make(chan []byte, 10)
	errChan := make(chan error, 1)

	// Read messages in goroutine
	go func() {
		for {
			_, message, err := conn.conn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			msgChan <- message
		}
	}()

	// Main loop
	for {
		select {
		case <-ctx.Done():
			return
		case <-pingTicker.C:
			conn.mu.Lock()
			if conn.closed {
				conn.mu.Unlock()
				return
			}
			err := conn.conn.WriteMessage(websocket.PingMessage, nil)
			conn.mu.Unlock()
			if err != nil {
				return
			}
		case err := <-errChan:
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log error
			}
			return
		case message := <-msgChan:
			h.handleMessage(ctx, conn, message)
		}
	}
}

// handleMessage handles a single WebSocket message.
func (h *WSHandler) handleMessage(ctx context.Context, conn *wsConnection, message []byte) {
	var msg WebSocketMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		conn.sendError("invalid message format")
		return
	}

	switch msg.Type {
	case MsgTypeAudio:
		h.handleAudioMessage(ctx, conn, msg.Data)
	case MsgTypeConfig:
		h.handleConfigMessage(conn, msg.Data)
	case MsgTypePing:
		conn.sendMessage(&WebSocketMessage{Type: MsgTypePong})
	default:
		conn.sendError("unknown message type: " + msg.Type)
	}
}

// handleAudioMessage handles incoming audio data.
func (h *WSHandler) handleAudioMessage(ctx context.Context, conn *wsConnection, data interface{}) {
	// Update state to listening
	h.service.UpdateSessionState(ctx, conn.sessionID, StateListening)
	conn.sendMessage(&WebSocketMessage{
		Type: MsgTypeStateChange,
		Data: map[string]string{"state": string(StateListening)},
	})

	// Parse audio data
	audioData, ok := data.(map[string]interface{})
	if !ok {
		conn.sendError("invalid audio data format")
		return
	}

	// Get base64 audio
	audioBase64, ok := audioData["audio"].(string)
	if !ok {
		conn.sendError("audio field is required")
		return
	}

	// Decode audio
	audioBytes, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		conn.sendError("invalid base64 audio: " + err.Error())
		return
	}

	// Get format
	format, _ := audioData["format"].(string)
	if format == "" {
		format = "wav"
	}

	// Update state to processing
	h.service.UpdateSessionState(ctx, conn.sessionID, StateProcessing)
	conn.sendMessage(&WebSocketMessage{
		Type: MsgTypeStateChange,
		Data: map[string]string{"state": string(StateProcessing)},
	})

	// Transcribe audio
	transcript, err := h.service.Transcribe(ctx, &TranscribeRequest{
		SessionID: conn.sessionID,
		Format:    format,
		Language:  conn.config.Language,
	}, audioBytes)
	if err != nil {
		conn.sendError("transcription failed: " + err.Error())
		h.service.UpdateSessionState(ctx, conn.sessionID, StateIdle)
		return
	}

	// Send transcript
	conn.sendMessage(&WebSocketMessage{
		Type: MsgTypeTranscript,
		Data: transcript,
	})

	// Process with LLM (placeholder)
	response, err := h.service.ProcessVoiceInput(ctx, conn.sessionID, transcript.Text)
	if err != nil {
		conn.sendError("processing failed: " + err.Error())
		h.service.UpdateSessionState(ctx, conn.sessionID, StateIdle)
		return
	}

	// Send text response
	conn.sendMessage(&WebSocketMessage{
		Type: MsgTypeResponse,
		Data: map[string]string{"text": response},
	})

	// Synthesize response if auto-play is enabled
	if conn.config.AutoPlayResponse {
		h.service.UpdateSessionState(ctx, conn.sessionID, StateSpeaking)
		conn.sendMessage(&WebSocketMessage{
			Type: MsgTypeStateChange,
			Data: map[string]string{"state": string(StateSpeaking)},
		})

		audioResponse, contentType, err := h.service.Synthesize(ctx, &SynthesizeRequest{
			Text: response,
		})
		if err != nil {
			conn.sendError("synthesis failed: " + err.Error())
		} else {
			conn.sendMessage(&WebSocketMessage{
				Type: MsgTypeAudioResponse,
				Data: map[string]interface{}{
					"audio":        base64.StdEncoding.EncodeToString(audioResponse),
					"content_type": contentType,
				},
			})
		}
	}

	// Return to idle
	h.service.UpdateSessionState(ctx, conn.sessionID, StateIdle)
	conn.sendMessage(&WebSocketMessage{
		Type: MsgTypeStateChange,
		Data: map[string]string{"state": string(StateIdle)},
	})
}

// handleConfigMessage handles configuration updates.
func (h *WSHandler) handleConfigMessage(conn *wsConnection, data interface{}) {
	configData, ok := data.(map[string]interface{})
	if !ok {
		conn.sendError("invalid config data format")
		return
	}

	// Update config
	if lang, ok := configData["language"].(string); ok {
		conn.config.Language = lang
	}
	if voice, ok := configData["voice"].(string); ok {
		conn.config.Voice = voice
	}
	if wakeWord, ok := configData["wake_word"].(string); ok {
		conn.config.WakeWord = wakeWord
	}
	if enabled, ok := configData["wake_word_enabled"].(bool); ok {
		conn.config.WakeWordEnabled = enabled
	}
	if continuous, ok := configData["continuous_listening"].(bool); ok {
		conn.config.ContinuousListening = continuous
	}
	if autoPlay, ok := configData["auto_play_response"].(bool); ok {
		conn.config.AutoPlayResponse = autoPlay
	}

	// Send updated config
	conn.sendMessage(&WebSocketMessage{
		Type: MsgTypeConfig,
		Data: map[string]interface{}{
			"session_id": conn.sessionID,
			"config":     conn.config,
		},
	})
}

// sendMessage sends a message to the WebSocket connection.
func (conn *wsConnection) sendMessage(msg *WebSocketMessage) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.closed {
		return fmt.Errorf("connection closed")
	}

	return conn.conn.WriteJSON(msg)
}

// sendError sends an error message to the WebSocket connection.
func (conn *wsConnection) sendError(errMsg string) {
	conn.sendMessage(&WebSocketMessage{
		Type:  MsgTypeError,
		Error: errMsg,
	})
}
