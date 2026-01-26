// Package feishu provides a Feishu/Lark bot channel implementation.
package feishu

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

// Channel implements the channel.Channel interface for Feishu/Lark.
type Channel struct {
	config   channel.FeishuConfig
	logger   *zap.Logger
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64

	tenantAccessToken string
	tokenExpireTime   time.Time
	tokenMu           sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new Feishu channel.
func New(cfg channel.FeishuConfig, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "feishu")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "feishu"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "feishu"
}

// Start initializes and starts the Feishu bot.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Get initial tenant access token
	if err := c.refreshTenantAccessToken(); err != nil {
		c.setError(fmt.Sprintf("failed to get tenant access token: %v", err))
		return fmt.Errorf("failed to get Feishu tenant access token: %w", err)
	}

	c.logger.Info("feishu tenant access token obtained")

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("feishu channel started")
	return nil
}

// refreshTenantAccessToken refreshes the tenant access token.
func (c *Channel) refreshTenantAccessToken() error {
	payload := map[string]string{
		"app_id":     c.config.AppID,
		"app_secret": c.config.AppSecret,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := http.Post(
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		"application/json",
		bytes.NewReader(jsonPayload),
	)
	if err != nil {
		return fmt.Errorf("failed to request tenant access token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("Feishu API error: %d - %s", result.Code, result.Msg)
	}

	c.tokenMu.Lock()
	c.tenantAccessToken = result.TenantAccessToken
	c.tokenExpireTime = time.Now().Add(time.Duration(result.Expire-300) * time.Second) // Refresh 5 minutes early
	c.tokenMu.Unlock()

	return nil
}

// getTenantAccessToken returns the current tenant access token, refreshing if necessary.
func (c *Channel) getTenantAccessToken() (string, error) {
	c.tokenMu.RLock()
	if time.Now().Before(c.tokenExpireTime) {
		token := c.tenantAccessToken
		c.tokenMu.RUnlock()
		return token, nil
	}
	c.tokenMu.RUnlock()

	if err := c.refreshTenantAccessToken(); err != nil {
		return "", err
	}

	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.tenantAccessToken, nil
}

// HandleCallback handles incoming Feishu callback requests.
// This should be called by an HTTP handler.
func (c *Channel) HandleCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		c.logger.Error("failed to read request body", zap.Error(err))
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Check if message is encrypted
	var encryptedBody struct {
		Encrypt string `json:"encrypt"`
	}
	if err := json.Unmarshal(body, &encryptedBody); err == nil && encryptedBody.Encrypt != "" {
		// Decrypt the message
		decrypted, err := c.decryptMessage(encryptedBody.Encrypt)
		if err != nil {
			c.logger.Error("failed to decrypt message", zap.Error(err))
			http.Error(w, "Decryption failed", http.StatusInternalServerError)
			return
		}
		body = []byte(decrypted)
	}

	// Parse the event
	var event feishuEvent
	if err := json.Unmarshal(body, &event); err != nil {
		c.logger.Error("failed to parse event", zap.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Handle URL verification
	if event.Type == "url_verification" {
		c.handleURLVerification(w, &event)
		return
	}

	// Verify token
	if event.Token != c.config.VerificationToken {
		c.logger.Warn("invalid verification token")
		http.Error(w, "Invalid token", http.StatusForbidden)
		return
	}

	// Handle event callback
	c.handleEventCallback(w, &event)
}

// feishuEvent represents a Feishu event.
type feishuEvent struct {
	Schema    string `json:"schema"`
	Type      string `json:"type"`
	Token     string `json:"token"`
	Challenge string `json:"challenge"`
	Header    struct {
		EventID    string `json:"event_id"`
		EventType  string `json:"event_type"`
		CreateTime string `json:"create_time"`
		Token      string `json:"token"`
		AppID      string `json:"app_id"`
		TenantKey  string `json:"tenant_key"`
	} `json:"header"`
	Event json.RawMessage `json:"event"`
}

// handleURLVerification handles URL verification requests.
func (c *Channel) handleURLVerification(w http.ResponseWriter, event *feishuEvent) {
	response := map[string]string{
		"challenge": event.Challenge,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleEventCallback handles event callbacks.
func (c *Channel) handleEventCallback(w http.ResponseWriter, event *feishuEvent) {
	switch event.Header.EventType {
	case "im.message.receive_v1":
		c.handleMessageEvent(event)
	}

	w.WriteHeader(http.StatusOK)
}

// messageEvent represents a message event.
type messageEvent struct {
	Sender struct {
		SenderID struct {
			UnionID string `json:"union_id"`
			UserID  string `json:"user_id"`
			OpenID  string `json:"open_id"`
		} `json:"sender_id"`
		SenderType string `json:"sender_type"`
		TenantKey  string `json:"tenant_key"`
	} `json:"sender"`
	Message struct {
		MessageID   string `json:"message_id"`
		RootID      string `json:"root_id"`
		ParentID    string `json:"parent_id"`
		CreateTime  string `json:"create_time"`
		ChatID      string `json:"chat_id"`
		ChatType    string `json:"chat_type"`
		MessageType string `json:"message_type"`
		Content     string `json:"content"`
		Mentions    []struct {
			Key       string `json:"key"`
			ID        struct {
				UnionID string `json:"union_id"`
				UserID  string `json:"user_id"`
				OpenID  string `json:"open_id"`
			} `json:"id"`
			Name      string `json:"name"`
			TenantKey string `json:"tenant_key"`
		} `json:"mentions"`
	} `json:"message"`
}

// handleMessageEvent handles message events.
func (c *Channel) handleMessageEvent(event *feishuEvent) {
	var msgEvent messageEvent
	if err := json.Unmarshal(event.Event, &msgEvent); err != nil {
		c.logger.Error("failed to parse message event", zap.Error(err))
		return
	}

	// Parse message content
	var content string
	switch msgEvent.Message.MessageType {
	case "text":
		var textContent struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(msgEvent.Message.Content), &textContent); err == nil {
			content = textContent.Text
		}
	default:
		content = msgEvent.Message.Content
	}

	// Convert to unified message format
	channelMsg := channel.Message{
		ID:          msgEvent.Message.MessageID,
		ChannelName: "feishu",
		ChatID:      msgEvent.Message.ChatID,
		UserID:      msgEvent.Sender.SenderID.OpenID,
		Type:        c.convertMessageType(msgEvent.Message.MessageType),
		Content:     content,
		Timestamp:   parseFeishuTimestamp(msgEvent.Message.CreateTime),
		IsGroup:     msgEvent.Message.ChatType == "group",
		Metadata: map[string]interface{}{
			"chat_type":    msgEvent.Message.ChatType,
			"message_type": msgEvent.Message.MessageType,
			"tenant_key":   msgEvent.Sender.TenantKey,
		},
	}

	// Handle reply
	if msgEvent.Message.ParentID != "" {
		channelMsg.ReplyToID = msgEvent.Message.ParentID
	}

	c.msgCount.Add(1)

	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", channelMsg.ID))
	}
}

// convertMessageType converts Feishu message type to unified type.
func (c *Channel) convertMessageType(msgType string) channel.MessageType {
	switch msgType {
	case "text":
		return channel.MessageTypeText
	case "image":
		return channel.MessageTypeImage
	case "audio":
		return channel.MessageTypeAudio
	case "media", "video":
		return channel.MessageTypeVideo
	case "file":
		return channel.MessageTypeFile
	case "interactive":
		return channel.MessageTypeCard
	default:
		return channel.MessageTypeText
	}
}

// decryptMessage decrypts an encrypted message.
func (c *Channel) decryptMessage(encrypted string) (string, error) {
	if c.config.EncryptKey == "" {
		return "", fmt.Errorf("encrypt key not configured")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("invalid base64: %w", err)
	}

	// Generate AES key from encrypt key
	hash := sha256.Sum256([]byte(c.config.EncryptKey))
	key := hash[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	// Remove PKCS7 padding
	padding := int(ciphertext[len(ciphertext)-1])
	if padding > aes.BlockSize || padding == 0 {
		return "", fmt.Errorf("invalid padding")
	}
	ciphertext = ciphertext[:len(ciphertext)-padding]

	return string(ciphertext), nil
}

// Stop gracefully shuts down the channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusDisconnected {
		c.mu.Unlock()
		return nil
	}
	c.status = channel.StatusDisconnected
	c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	close(c.messages)
	c.logger.Info("feishu channel stopped")
	return nil
}

// Send sends a message through Feishu.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	token, err := c.getTenantAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get tenant access token: %w", err)
	}

	// Determine message type and content
	msgType := "text"
	var content interface{}

	if msg.Format == "interactive" || msg.Format == "card" {
		msgType = "interactive"
		content = json.RawMessage(msg.Content)
	} else {
		content = map[string]string{
			"text": msg.Content,
		}
	}

	contentJSON, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	// Build request payload
	payload := map[string]interface{}{
		"receive_id": msg.ChatID,
		"msg_type":   msgType,
		"content":    string(contentJSON),
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Determine receive_id_type based on chat ID format
	receiveIDType := "chat_id"
	if strings.HasPrefix(msg.ChatID, "ou_") {
		receiveIDType = "open_id"
	} else if strings.HasPrefix(msg.ChatID, "on_") {
		receiveIDType = "union_id"
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=%s", receiveIDType)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("Feishu API error: %d - %s", result.Code, result.Msg)
	}

	return nil
}

// SendStreaming sends a message with streaming support.
// Feishu supports card updates for streaming-like behavior.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	var fullContent strings.Builder
	var sentMsgID string
	lastUpdate := time.Now()
	updateInterval := 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				// Channel closed, send final message
				if fullContent.Len() > 0 {
					if sentMsgID != "" {
						return c.updateMessage(ctx, sentMsgID, fullContent.String())
					}
					return c.Send(ctx, channel.OutgoingMessage{
						ChatID:  chatID,
						Content: fullContent.String(),
					})
				}
				return nil
			}

			fullContent.WriteString(chunk)

			// Update message periodically
			if time.Since(lastUpdate) >= updateInterval {
				if sentMsgID == "" {
					// Send initial message
					msgID, err := c.sendAndGetID(ctx, chatID, fullContent.String())
					if err != nil {
						c.logger.Error("failed to send streaming message", zap.Error(err))
						continue
					}
					sentMsgID = msgID
				} else {
					// Update existing message
					if err := c.updateMessage(ctx, sentMsgID, fullContent.String()); err != nil {
						c.logger.Warn("failed to update streaming message", zap.Error(err))
					}
				}
				lastUpdate = time.Now()
			}
		}
	}
}

// sendAndGetID sends a message and returns the message ID.
func (c *Channel) sendAndGetID(ctx context.Context, chatID, content string) (string, error) {
	token, err := c.getTenantAccessToken()
	if err != nil {
		return "", fmt.Errorf("failed to get tenant access token: %w", err)
	}

	contentJSON, _ := json.Marshal(map[string]string{"text": content})
	payload := map[string]interface{}{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    string(contentJSON),
	}

	jsonPayload, _ := json.Marshal(payload)

	receiveIDType := "chat_id"
	if strings.HasPrefix(chatID, "ou_") {
		receiveIDType = "open_id"
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=%s", receiveIDType)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Code != 0 {
		return "", fmt.Errorf("Feishu API error: %d - %s", result.Code, result.Msg)
	}

	return result.Data.MessageID, nil
}

// updateMessage updates an existing message.
func (c *Channel) updateMessage(ctx context.Context, messageID, content string) error {
	token, err := c.getTenantAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get tenant access token: %w", err)
	}

	contentJSON, _ := json.Marshal(map[string]string{"text": content})
	payload := map[string]interface{}{
		"msg_type": "text",
		"content":  string(contentJSON),
	}

	jsonPayload, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages/%s", messageID)

	req, _ := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Code != 0 {
		return fmt.Errorf("Feishu API error: %d - %s", result.Code, result.Msg)
	}

	return nil
}

// Info returns current information about the channel.
func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info := channel.Info{
		Name:         "feishu",
		Type:         "feishu",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
		Metadata: map[string]interface{}{
			"app_id": c.config.AppID,
		},
	}

	return info
}

// IsConnected returns true if the channel is connected.
func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected
}

// Messages returns the channel for receiving incoming messages.
func (c *Channel) Messages() <-chan channel.Message {
	return c.messages
}

// setError sets the last error.
func (c *Channel) setError(err string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err
	now := time.Now()
	c.lastErrorAt = &now
	c.status = channel.StatusError
}

// parseFeishuTimestamp parses a Feishu timestamp to time.Time.
func parseFeishuTimestamp(ts string) time.Time {
	var msec int64
	fmt.Sscanf(ts, "%d", &msec)
	return time.UnixMilli(msec)
}
