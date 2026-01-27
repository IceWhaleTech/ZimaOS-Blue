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

	// Bot commands
	commandHandlers map[string]BotCommandHandler
	commandMu       sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// BotCommandHandler handles bot commands.
type BotCommandHandler func(ctx context.Context, cmd string, args string, event *messageEvent) (string, *InteractiveCard, error)

// InteractiveCard represents a Feishu interactive card.
type InteractiveCard struct {
	Config   CardConfig    `json:"config,omitempty"`
	Header   *CardHeader   `json:"header,omitempty"`
	Elements []CardElement `json:"elements,omitempty"`
}

// CardConfig represents card configuration.
type CardConfig struct {
	WideScreenMode bool `json:"wide_screen_mode,omitempty"`
	EnableForward  bool `json:"enable_forward,omitempty"`
}

// CardHeader represents card header.
type CardHeader struct {
	Title    *CardText `json:"title,omitempty"`
	Template string    `json:"template,omitempty"` // blue, wathet, turquoise, green, yellow, orange, red, carmine, violet, purple, indigo, grey
}

// CardText represents text in a card.
type CardText struct {
	Tag     string `json:"tag"` // plain_text, lark_md
	Content string `json:"content"`
}

// CardElement represents an element in a card.
type CardElement interface {
	isCardElement()
}

// DivElement represents a div element.
type DivElement struct {
	Tag    string    `json:"tag"` // div
	Text   *CardText `json:"text,omitempty"`
	Fields []struct {
		IsShort bool      `json:"is_short"`
		Text    *CardText `json:"text"`
	} `json:"fields,omitempty"`
}

func (DivElement) isCardElement() {}

// ActionElement represents an action element with buttons.
type ActionElement struct {
	Tag     string         `json:"tag"` // action
	Actions []ActionButton `json:"actions"`
	Layout  string         `json:"layout,omitempty"` // bisected, trisection, flow
}

func (ActionElement) isCardElement() {}

// ActionButton represents a button in an action element.
type ActionButton struct {
	Tag   string    `json:"tag"` // button
	Text  *CardText `json:"text"`
	URL   string    `json:"url,omitempty"`
	Type  string    `json:"type,omitempty"` // default, primary, danger
	Value map[string]interface{} `json:"value,omitempty"`
}

// NoteElement represents a note element.
type NoteElement struct {
	Tag      string      `json:"tag"` // note
	Elements []CardText  `json:"elements"`
}

func (NoteElement) isCardElement() {}

// HrElement represents a horizontal rule element.
type HrElement struct {
	Tag string `json:"tag"` // hr
}

func (HrElement) isCardElement() {}

// New creates a new Feishu channel.
func New(cfg channel.FeishuConfig, logger *zap.Logger) *Channel {
	c := &Channel{
		config:          cfg,
		logger:          logger.With(zap.String("channel", "feishu")),
		messages:        make(chan channel.Message, 100),
		status:          channel.StatusDisconnected,
		commandHandlers: make(map[string]BotCommandHandler),
	}

	// Register default command handlers
	c.RegisterCommand("/help", c.handleHelpCommand)
	c.RegisterCommand("/start", c.handleStartCommand)

	return c
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

	// Check if this is a command
	if c.processCommand(content, &msgEvent) {
		return
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

// RegisterCommand registers a bot command handler.
func (c *Channel) RegisterCommand(cmd string, handler BotCommandHandler) {
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	c.commandHandlers[cmd] = handler
}

// handleHelpCommand handles the /help command.
func (c *Channel) handleHelpCommand(ctx context.Context, cmd string, args string, event *messageEvent) (string, *InteractiveCard, error) {
	card := &InteractiveCard{
		Config: CardConfig{
			WideScreenMode: true,
			EnableForward:  true,
		},
		Header: &CardHeader{
			Title:    &CardText{Tag: "plain_text", Content: "📚 帮助信息"},
			Template: "blue",
		},
		Elements: []CardElement{
			DivElement{
				Tag: "div",
				Text: &CardText{
					Tag:     "lark_md",
					Content: "**可用命令：**\n\n/start - 开始使用机器人\n/help - 显示帮助信息\n\n**使用方法：**\n直接发送消息即可与 AI 对话。",
				},
			},
		},
	}

	return "", card, nil
}

// handleStartCommand handles the /start command.
func (c *Channel) handleStartCommand(ctx context.Context, cmd string, args string, event *messageEvent) (string, *InteractiveCard, error) {
	card := &InteractiveCard{
		Config: CardConfig{
			WideScreenMode: true,
			EnableForward:  true,
		},
		Header: &CardHeader{
			Title:    &CardText{Tag: "plain_text", Content: "👋 欢迎使用 ZimaOS Echo"},
			Template: "green",
		},
		Elements: []CardElement{
			DivElement{
				Tag: "div",
				Text: &CardText{
					Tag:     "lark_md",
					Content: "我是您的 AI 助手，可以帮助您：\n\n• 回答问题\n• 处理任务\n• 提供建议\n\n直接发送消息开始对话吧！",
				},
			},
			ActionElement{
				Tag:    "action",
				Layout: "bisected",
				Actions: []ActionButton{
					{
						Tag:  "button",
						Text: &CardText{Tag: "plain_text", Content: "📚 查看帮助"},
						Type: "default",
						Value: map[string]interface{}{
							"action": "help",
						},
					},
				},
			},
		},
	}

	return "", card, nil
}

// processCommand checks if a message is a command and processes it.
func (c *Channel) processCommand(content string, event *messageEvent) bool {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "/") {
		return false
	}

	// Parse command and arguments
	parts := strings.SplitN(content, " ", 2)
	cmd := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	c.commandMu.RLock()
	handler, exists := c.commandHandlers[cmd]
	c.commandMu.RUnlock()

	if !exists {
		return false
	}

	c.logger.Debug("processing command",
		zap.String("command", cmd),
		zap.String("args", args))

	text, card, err := handler(c.ctx, cmd, args, event)
	if err != nil {
		c.logger.Error("command handler error",
			zap.String("command", cmd),
			zap.Error(err))
		text = "处理命令时发生错误，请稍后重试。"
	}

	// Send response
	if card != nil {
		if err := c.SendCard(c.ctx, event.Message.ChatID, card); err != nil {
			c.logger.Error("failed to send card response",
				zap.String("command", cmd),
				zap.Error(err))
		}
	} else if text != "" {
		if err := c.Send(c.ctx, channel.OutgoingMessage{
			ChatID:  event.Message.ChatID,
			Content: text,
		}); err != nil {
			c.logger.Error("failed to send text response",
				zap.String("command", cmd),
				zap.Error(err))
		}
	}

	return true
}

// SendCard sends an interactive card message.
func (c *Channel) SendCard(ctx context.Context, chatID string, card *InteractiveCard) error {
	token, err := c.getTenantAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get tenant access token: %w", err)
	}

	cardJSON, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("failed to marshal card: %w", err)
	}

	payload := map[string]interface{}{
		"receive_id": chatID,
		"msg_type":   "interactive",
		"content":    string(cardJSON),
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	receiveIDType := "chat_id"
	if strings.HasPrefix(chatID, "ou_") {
		receiveIDType = "open_id"
	} else if strings.HasPrefix(chatID, "on_") {
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
		return fmt.Errorf("failed to send card: %w", err)
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

// HandleCardAction handles card action callbacks.
func (c *Channel) HandleCardAction(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		c.logger.Error("failed to read request body", zap.Error(err))
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var action struct {
		OpenID        string `json:"open_id"`
		UserID        string `json:"user_id"`
		OpenMessageID string `json:"open_message_id"`
		TenantKey     string `json:"tenant_key"`
		Token         string `json:"token"`
		Action        struct {
			Value map[string]interface{} `json:"value"`
			Tag   string                 `json:"tag"`
		} `json:"action"`
	}

	if err := json.Unmarshal(body, &action); err != nil {
		c.logger.Error("failed to parse card action", zap.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Verify token
	if action.Token != c.config.VerificationToken {
		c.logger.Warn("invalid verification token in card action")
		http.Error(w, "Invalid token", http.StatusForbidden)
		return
	}

	c.logger.Debug("received card action",
		zap.String("user_id", action.UserID),
		zap.Any("value", action.Action.Value))

	// Handle built-in actions
	if actionName, ok := action.Action.Value["action"].(string); ok {
		switch actionName {
		case "help":
			// Trigger help command
			event := &messageEvent{}
			event.Message.ChatID = action.OpenID
			event.Sender.SenderID.OpenID = action.OpenID
			c.processCommand("/help", event)
		}
	}

	// Send to message channel for custom handling
	channelMsg := channel.Message{
		ID:          action.OpenMessageID,
		ChannelName: "feishu",
		ChatID:      action.OpenID,
		UserID:      action.OpenID,
		Type:        channel.MessageTypeText,
		Content:     fmt.Sprintf("%v", action.Action.Value),
		Timestamp:   time.Now(),
		IsGroup:     false,
		Metadata: map[string]interface{}{
			"is_card_action": true,
			"action_value":   action.Action.Value,
			"action_tag":     action.Action.Tag,
			"message_id":     action.OpenMessageID,
		},
	}

	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full, dropping card action")
	}

	w.WriteHeader(http.StatusOK)
}
