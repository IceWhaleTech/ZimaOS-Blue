// Package qq provides a QQ Bot OpenAPI channel implementation.
package qq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Config contains QQ Bot channel configuration.
type Config struct {
	Enabled   bool   `yaml:"enabled"`
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
	Token     string `yaml:"token"`
	Sandbox   bool   `yaml:"sandbox"`
}

const (
	prodAPIBase    = "https://api.sgroup.qq.com"
	sandboxAPIBase = "https://sandbox.api.sgroup.qq.com"

	gatewayPath       = "/gateway"
	sendMessagePath   = "/channels/%s/messages"
	accessTokenPath   = "/app/getAppAccessToken"

	wsReconnectDelay  = 5 * time.Second
	heartbeatInterval = 30 * time.Second
	tokenRefreshEarly = 60 // seconds before expiry to refresh
)

// WebSocket opcodes from QQ Bot gateway protocol.
const (
	opDispatch        = 0
	opHeartbeat       = 1
	opIdentify        = 2
	opResume          = 6
	opReconnect       = 7
	opInvalidSession  = 9
	opHello           = 10
	opHeartbeatAck    = 11
)

// wsPayload represents a WebSocket gateway payload.
type wsPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  int64           `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

// helloData is the payload of the Hello event.
type helloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

// identifyData is sent to authenticate with the gateway.
type identifyData struct {
	Token   string           `json:"token"`
	Intents int              `json:"intents"`
	Shard   [2]int           `json:"shard"`
	Props   *identifyProps   `json:"properties,omitempty"`
}

type identifyProps struct {
	OS      string `json:"$os"`
	Browser string `json:"$browser"`
	Device  string `json:"$device"`
}

// resumeData is sent to resume a disconnected session.
type resumeData struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Seq       int64  `json:"seq"`
}

// accessTokenResponse is the response from /app/getAppAccessToken.
type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
}

// gatewayResponse is the response from GET /gateway.
type gatewayResponse struct {
	URL string `json:"url"`
}

// messageEvent represents an incoming message from the gateway.
type messageEvent struct {
	ID        string       `json:"id"`
	ChannelID string       `json:"channel_id"`
	GuildID   string       `json:"guild_id"`
	Content   string       `json:"content"`
	Author    messageAuthor `json:"author"`
	Timestamp string       `json:"timestamp"`
}

type messageAuthor struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

// Channel implements the channel.Channel interface for QQ Bot.
type Channel struct {
	config   Config
	logger   *zap.Logger
	client   *http.Client
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64
	msgsSent    atomic.Int64
	msgsRecv    atomic.Int64

	// Access token management
	tokenMu     sync.RWMutex
	accessToken string
	tokenExpiry time.Time

	// WebSocket state
	ws        *websocket.Conn
	sessionID string
	seq       int64

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new QQ Bot channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "qq")),
		client:   &http.Client{Timeout: 30 * time.Second},
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

func (c *Channel) Name() string                    { return "qq" }
func (c *Channel) Type() string                    { return "qq" }
func (c *Channel) Messages() <-chan channel.Message { return c.messages }

func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected
}

func (c *Channel) apiBase() string {
	if c.config.Sandbox {
		return sandboxAPIBase
	}
	return prodAPIBase
}

// refreshAccessToken obtains or refreshes the access token.
func (c *Channel) refreshAccessToken(ctx context.Context) error {
	c.tokenMu.RLock()
	if c.accessToken != "" && time.Until(c.tokenExpiry) > time.Duration(tokenRefreshEarly)*time.Second {
		c.tokenMu.RUnlock()
		return nil
	}
	c.tokenMu.RUnlock()

	body, _ := json.Marshal(map[string]string{
		"appId":     c.config.AppID,
		"clientSecret": c.config.AppSecret,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase()+accessTokenPath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request access token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("access token API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var tokenResp accessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	var expiresIn int
	fmt.Sscanf(tokenResp.ExpiresIn, "%d", &expiresIn)

	c.tokenMu.Lock()
	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = timeutil.NowTime().Add(time.Duration(expiresIn) * time.Second)
	c.tokenMu.Unlock()

	c.logger.Debug("access token refreshed", zap.Int("expires_in", expiresIn))
	return nil
}

// getAccessToken returns the current access token, refreshing if needed.
func (c *Channel) getAccessToken(ctx context.Context) (string, error) {
	if err := c.refreshAccessToken(ctx); err != nil {
		return "", err
	}
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.accessToken, nil
}

// getGatewayURL fetches the WebSocket gateway URL.
func (c *Channel) getGatewayURL(ctx context.Context) (string, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase()+gatewayPath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create gateway request: %w", err)
	}
	req.Header.Set("Authorization", "QQBot "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch gateway URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gateway API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var gw gatewayResponse
	if err := json.NewDecoder(resp.Body).Decode(&gw); err != nil {
		return "", fmt.Errorf("failed to parse gateway response: %w", err)
	}
	return gw.URL, nil
}

// Start initializes and starts the QQ Bot channel.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	if err := c.refreshAccessToken(c.ctx); err != nil {
		c.setError(fmt.Sprintf("failed to get access token: %v", err))
		return fmt.Errorf("failed to get access token: %w", err)
	}

	gwURL, err := c.getGatewayURL(c.ctx)
	if err != nil {
		c.setError(fmt.Sprintf("failed to get gateway URL: %v", err))
		return fmt.Errorf("failed to get gateway URL: %w", err)
	}

	if err := c.connectWS(gwURL); err != nil {
		c.setError(fmt.Sprintf("failed to connect WebSocket: %v", err))
		return fmt.Errorf("failed to connect WebSocket: %w", err)
	}

	now := timeutil.NowTime()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	go c.readLoop()
	go c.tokenRefreshLoop()

	c.logger.Info("QQ Bot channel started")
	return nil
}

// connectWS establishes the WebSocket connection and performs identification.
func (c *Channel) connectWS(url string) error {
	conn, _, err := websocket.DefaultDialer.DialContext(c.ctx, url, nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}
	c.ws = conn

	// Read Hello
	var hello wsPayload
	if err := conn.ReadJSON(&hello); err != nil {
		conn.Close()
		return fmt.Errorf("failed to read hello: %w", err)
	}
	if hello.Op != opHello {
		conn.Close()
		return fmt.Errorf("expected hello (op %d), got op %d", opHello, hello.Op)
	}

	// Send Identify
	c.tokenMu.RLock()
	token := c.accessToken
	c.tokenMu.RUnlock()

	identify := wsPayload{
		Op: opIdentify,
	}
	identData := identifyData{
		Token:   fmt.Sprintf("QQBot %s", token),
		Intents: 0 | (1 << 30) | (1 << 1) | (1 << 0), // PUBLIC_GUILD_MESSAGES | GUILDS | GUILD_MEMBERS
		Shard:   [2]int{0, 1},
	}
	raw, _ := json.Marshal(identData)
	identify.D = raw

	if err := conn.WriteJSON(identify); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send identify: %w", err)
	}

	// Read Ready
	var ready wsPayload
	if err := conn.ReadJSON(&ready); err != nil {
		conn.Close()
		return fmt.Errorf("failed to read ready: %w", err)
	}
	if ready.T == "READY" {
		var readyData struct {
			SessionID string `json:"session_id"`
		}
		json.Unmarshal(ready.D, &readyData)
		c.sessionID = readyData.SessionID
		c.seq = ready.S
		c.logger.Info("gateway session established", zap.String("session_id", c.sessionID))
	}

	return nil
}

// readLoop reads messages from the WebSocket connection.
func (c *Channel) readLoop() {
	defer func() {
		if c.ws != nil {
			c.ws.Close()
		}
	}()

	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-heartbeatTicker.C:
			c.sendHeartbeat()
		default:
		}

		if c.ws == nil {
			return
		}

		c.ws.SetReadDeadline(timeutil.NowTime().Add(heartbeatInterval + 10*time.Second))
		var payload wsPayload
		if err := c.ws.ReadJSON(&payload); err != nil {
			if c.ctx.Err() != nil {
				return
			}
			c.logger.Warn("websocket read error, attempting reconnect", zap.Error(err))
			c.reconnect()
			continue
		}

		if payload.S > 0 {
			atomic.StoreInt64(&c.seq, payload.S)
		}

		switch payload.Op {
		case opDispatch:
			c.handleDispatch(payload)
		case opHeartbeat:
			c.sendHeartbeat()
		case opReconnect:
			c.logger.Info("server requested reconnect")
			c.reconnect()
		case opInvalidSession:
			c.logger.Warn("invalid session, re-identifying")
			c.sessionID = ""
			c.reconnect()
		case opHeartbeatAck:
			// OK
		}
	}
}

// sendHeartbeat sends a heartbeat to the gateway.
func (c *Channel) sendHeartbeat() {
	if c.ws == nil {
		return
	}
	seq := atomic.LoadInt64(&c.seq)
	raw, _ := json.Marshal(seq)
	payload := wsPayload{Op: opHeartbeat, D: raw}
	if err := c.ws.WriteJSON(payload); err != nil {
		c.logger.Warn("failed to send heartbeat", zap.Error(err))
	}
}

// reconnect attempts to re-establish the WebSocket connection.
func (c *Channel) reconnect() {
	c.mu.Lock()
	c.status = channel.StatusReconnecting
	c.mu.Unlock()

	if c.ws != nil {
		c.ws.Close()
		c.ws = nil
	}

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(wsReconnectDelay):
		}

		gwURL, err := c.getGatewayURL(c.ctx)
		if err != nil {
			c.logger.Warn("reconnect: failed to get gateway URL", zap.Error(err))
			continue
		}

		conn, _, err := websocket.DefaultDialer.DialContext(c.ctx, gwURL, nil)
		if err != nil {
			c.logger.Warn("reconnect: websocket dial failed", zap.Error(err))
			continue
		}
		c.ws = conn

		// Read Hello
		var hello wsPayload
		if err := conn.ReadJSON(&hello); err != nil || hello.Op != opHello {
			c.logger.Warn("reconnect: failed to read hello", zap.Error(err))
			conn.Close()
			c.ws = nil
			continue
		}

		// Try Resume if we have a session
		if c.sessionID != "" {
			c.tokenMu.RLock()
			token := c.accessToken
			c.tokenMu.RUnlock()

			resumePayload := wsPayload{Op: opResume}
			rd := resumeData{
				Token:     fmt.Sprintf("QQBot %s", token),
				SessionID: c.sessionID,
				Seq:       atomic.LoadInt64(&c.seq),
			}
			raw, _ := json.Marshal(rd)
			resumePayload.D = raw

			if err := conn.WriteJSON(resumePayload); err != nil {
				c.logger.Warn("reconnect: failed to send resume", zap.Error(err))
				conn.Close()
				c.ws = nil
				continue
			}
		} else {
			// Fresh identify
			if err := c.identifyOnConn(conn); err != nil {
				c.logger.Warn("reconnect: identify failed", zap.Error(err))
				conn.Close()
				c.ws = nil
				continue
			}
		}

		now := timeutil.NowTime()
		c.mu.Lock()
		c.status = channel.StatusConnected
		c.connectedAt = &now
		c.lastError = ""
		c.lastErrorAt = nil
		c.mu.Unlock()

		c.logger.Info("reconnected to QQ Bot gateway")
		return
	}
}

// identifyOnConn sends an Identify payload on the given connection.
func (c *Channel) identifyOnConn(conn *websocket.Conn) error {
	c.tokenMu.RLock()
	token := c.accessToken
	c.tokenMu.RUnlock()

	identData := identifyData{
		Token:   fmt.Sprintf("QQBot %s", token),
		Intents: 0 | (1 << 30) | (1 << 1) | (1 << 0),
		Shard:   [2]int{0, 1},
	}
	raw, _ := json.Marshal(identData)
	payload := wsPayload{Op: opIdentify, D: raw}
	return conn.WriteJSON(payload)
}

// handleDispatch processes dispatched events from the gateway.
func (c *Channel) handleDispatch(payload wsPayload) {
	switch payload.T {
	case "AT_MESSAGE_CREATE", "MESSAGE_CREATE":
		var msg messageEvent
		if err := json.Unmarshal(payload.D, &msg); err != nil {
			c.logger.Warn("failed to parse message event", zap.Error(err))
			return
		}
		if msg.Author.Bot {
			return
		}
		c.processMessage(msg)
	case "READY":
		var readyData struct {
			SessionID string `json:"session_id"`
		}
		json.Unmarshal(payload.D, &readyData)
		c.sessionID = readyData.SessionID
		c.logger.Info("session ready", zap.String("session_id", c.sessionID))
	case "RESUMED":
		c.logger.Info("session resumed")
	}
}

// processMessage converts a QQ message event to a unified channel.Message.
func (c *Channel) processMessage(msg messageEvent) {
	ts, _ := time.Parse(time.RFC3339, msg.Timestamp)

	channelMsg := channel.Message{
		ID:          msg.ID,
		ChannelName: "qq",
		ChatID:      msg.ChannelID,
		UserID:      msg.Author.ID,
		Username:    msg.Author.Username,
		Type:        channel.MessageTypeText,
		Content:     msg.Content,
		Timestamp:   ts,
		IsGroup:     msg.GuildID != "",
		Metadata: map[string]interface{}{
			"guild_id":   msg.GuildID,
			"channel_id": msg.ChannelID,
		},
	}
	if channelMsg.IsGroup {
		channelMsg.GroupName = msg.GuildID
	}

	c.msgCount.Add(1)
	c.msgsRecv.Add(1)

	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full, dropping message", zap.String("id", msg.ID))
	}
}

// tokenRefreshLoop periodically refreshes the access token.
func (c *Channel) tokenRefreshLoop() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.refreshAccessToken(c.ctx); err != nil {
				c.logger.Warn("periodic token refresh failed", zap.Error(err))
			}
		}
	}
}

// Stop gracefully shuts down the channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	c.status = channel.StatusDisconnected
	c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}
	if c.ws != nil {
		c.ws.Close()
	}
	close(c.messages)
	c.logger.Info("QQ Bot channel stopped")
	return nil
}

// Send sends a message via QQ Bot HTTP API.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	payload := map[string]interface{}{
		"content": msg.Content,
	}
	if msg.ReplyToID != "" {
		payload["msg_id"] = msg.ReplyToID
	}

	// Handle image attachment via file_image field.
	for _, att := range msg.Attachments {
		if att.Type == channel.MessageTypeImage && att.URL != "" {
			payload["file_image"] = att.URL
			break
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	url := fmt.Sprintf("%s"+sendMessagePath, c.apiBase(), msg.ChatID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "QQBot "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("QQ API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	c.msgsSent.Add(1)
	return nil
}

// SendStreaming sends a message with streaming support via chunked sends.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	var fullContent strings.Builder
	chunkSize := 500
	lastSend := timeutil.NowTime()
	sendInterval := 800 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				// Channel closed, send remaining content.
				if fullContent.Len() > 0 {
					return c.Send(ctx, channel.OutgoingMessage{
						ChatID:    chatID,
						ReplyToID: replyToID,
						Content:   fullContent.String(),
					})
				}
				return nil
			}

			fullContent.WriteString(chunk)

			// Send intermediate chunks to provide streaming feel.
			if fullContent.Len() >= chunkSize && timeutil.SinceTime(lastSend) >= sendInterval {
				if err := c.Send(ctx, channel.OutgoingMessage{
					ChatID:    chatID,
					ReplyToID: replyToID,
					Content:   fullContent.String(),
				}); err != nil {
					c.logger.Warn("failed to send streaming chunk", zap.Error(err))
				}
				fullContent.Reset()
				lastSend = timeutil.NowTime()
			}
		}
	}
}

// Info returns current information about the channel.
func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name: "qq", Type: "qq", Status: c.status, Enabled: c.config.Enabled,
		ConnectedAt:      c.connectedAt,
		LastError:        c.lastError,
		LastErrorAt:      c.lastErrorAt,
		MessageCount:     c.msgCount.Load(),
		MessagesReceived: c.msgsRecv.Load(),
		MessagesSent:     c.msgsSent.Load(),
		Metadata: map[string]interface{}{
			"sandbox":    c.config.Sandbox,
			"session_id": c.sessionID,
		},
	}
}

func (c *Channel) setError(errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = errMsg
	now := timeutil.NowTime()
	c.lastErrorAt = &now
	c.status = channel.StatusError
}

// Validator for QQ Bot configuration.
type Validator struct{}

func NewValidator() *Validator { return &Validator{} }

func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	appID := config["app_id"]
	appSecret := config["app_secret"]
	if appID == "" || appSecret == "" {
		return validator.Result{Success: false, Error: "app_id and app_secret are required"}
	}

	sandbox := config["sandbox"] == "true"
	apiBase := prodAPIBase
	if sandbox {
		apiBase = sandboxAPIBase
	}

	// Get access token.
	body, _ := json.Marshal(map[string]string{
		"appId":        appID,
		"clientSecret": appSecret,
	})

	client := &http.Client{Timeout: 10 * time.Second}
	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+accessTokenPath, bytes.NewReader(body))
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to create request: %v", err)}
	}
	tokenReq.Header.Set("Content-Type", "application/json")

	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to connect to QQ API: %v", err)}
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(tokenResp.Body)
		return validator.Result{Success: false, Error: fmt.Sprintf("QQ token API error (status %d): %s", tokenResp.StatusCode, string(respBody))}
	}

	var tokenData accessTokenResponse
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to parse token response: %v", err)}
	}

	if tokenData.AccessToken == "" {
		return validator.Result{Success: false, Error: "received empty access token"}
	}

	// Validate by calling GET /gateway.
	gwReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+gatewayPath, nil)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to create gateway request: %v", err)}
	}
	gwReq.Header.Set("Authorization", "QQBot "+tokenData.AccessToken)

	gwResp, err := client.Do(gwReq)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to connect to QQ gateway: %v", err)}
	}
	defer gwResp.Body.Close()

	if gwResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(gwResp.Body)
		return validator.Result{Success: false, Error: fmt.Sprintf("QQ gateway API error (status %d): %s", gwResp.StatusCode, string(respBody))}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
		Data: map[string]interface{}{
			"app_id": appID,
		},
	}
}
