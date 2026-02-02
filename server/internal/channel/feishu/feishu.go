// Package feishu provides a Feishu/Lark bot channel implementation using official SDK.
package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

// Channel implements the channel.Channel interface for Feishu/Lark using official SDK.
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

	// Official SDK client
	client   *lark.Client
	wsClient *larkws.Client

	// Bot commands
	commandHandlers map[string]BotCommandHandler
	commandMu       sync.RWMutex

	// Message handler for AI processing
	messageHandler MessageHandler

	// Ensure messages channel is only closed once
	closeOnce sync.Once

	ctx    context.Context
	cancel context.CancelFunc
}

// BotCommandHandler handles bot commands.
type BotCommandHandler func(ctx context.Context, cmd string, args string, chatID string, userID string) (string, error)

// MessageHandler handles incoming messages and returns AI response.
type MessageHandler func(ctx context.Context, msg channel.Message) (string, error)

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

// SetMessageHandler sets the handler for processing messages.
func (c *Channel) SetMessageHandler(handler MessageHandler) {
	c.messageHandler = handler
}

// Start initializes and starts the Feishu bot with WebSocket long connection.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Create Lark client
	c.client = lark.NewClient(c.config.AppID, c.config.AppSecret)

	// Create event dispatcher with message handler
	eventDispatcher := dispatcher.NewEventDispatcher(
		c.config.VerificationToken,
		c.config.EncryptKey,
	).OnP2MessageReceiveV1(c.onMessageReceive)

	// Create WebSocket client for long connection
	c.wsClient = larkws.NewClient(
		c.config.AppID,
		c.config.AppSecret,
		larkws.WithEventHandler(eventDispatcher),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	// Start WebSocket connection in background
	go func() {
		c.logger.Info("starting feishu websocket connection")
		if err := c.wsClient.Start(c.ctx); err != nil {
			c.logger.Error("feishu websocket error", zap.Error(err))
			c.setError(fmt.Sprintf("websocket error: %v", err))
		}
	}()

	// Wait a moment for connection to establish
	time.Sleep(500 * time.Millisecond)

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("feishu channel started with websocket long connection")
	return nil
}

// onMessageReceive handles incoming messages via OnP2MessageReceiveV1.
func (c *Channel) onMessageReceive(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}

	msg := event.Event.Message
	sender := event.Event.Sender

	// Parse message content
	var content string
	msgType := ""
	if msg.MessageType != nil {
		msgType = *msg.MessageType
	}

	switch msgType {
	case "text":
		var textContent struct {
			Text string `json:"text"`
		}
		if msg.Content != nil {
			if err := json.Unmarshal([]byte(*msg.Content), &textContent); err == nil {
				content = textContent.Text
			}
		}
	default:
		if msg.Content != nil {
			content = *msg.Content
		}
	}

	// Get IDs safely
	chatID := ""
	if msg.ChatId != nil {
		chatID = *msg.ChatId
	}
	userID := ""
	if sender != nil && sender.SenderId != nil && sender.SenderId.OpenId != nil {
		userID = *sender.SenderId.OpenId
	}
	messageID := ""
	if msg.MessageId != nil {
		messageID = *msg.MessageId
	}

	c.logger.Info("received message",
		zap.String("chat_id", chatID),
		zap.String("user_id", userID),
		zap.String("content", content),
		zap.String("msg_type", msgType))

	// Check if this is a command
	if c.processCommand(ctx, content, chatID, userID) {
		return nil
	}

	// Convert to unified message format
	channelMsg := channel.Message{
		ID:          messageID,
		ChannelName: "feishu",
		ChatID:      chatID,
		UserID:      userID,
		Type:        c.convertMessageType(msgType),
		Content:     content,
		Timestamp:   time.Now(),
		IsGroup:     msg.ChatType != nil && *msg.ChatType == "group",
		Metadata: map[string]interface{}{
			"msg_type": msgType,
		},
	}

	c.msgCount.Add(1)

	// If message handler is set, process and reply
	if c.messageHandler != nil {
		go func() {
			response, err := c.messageHandler(c.ctx, channelMsg)
			if err != nil {
				c.logger.Error("message handler error", zap.Error(err))
				response = "处理消息时发生错误，请稍后重试。"
			}
			if response != "" {
				if err := c.SendText(c.ctx, chatID, response); err != nil {
					c.logger.Error("failed to send response", zap.Error(err))
				}
			}
		}()
	} else {
		// Send to message channel for external processing
		// Check if channel is still connected before sending
		c.mu.RLock()
		isConnected := c.status == channel.StatusConnected
		c.mu.RUnlock()

		if !isConnected {
			c.logger.Warn("channel not connected, dropping message",
				zap.String("message_id", messageID))
			return nil
		}

		select {
		case c.messages <- channelMsg:
		default:
			c.logger.Warn("message channel full, dropping message",
				zap.String("message_id", messageID))
		}
	}

	return nil
}

// SendText sends a text message using official SDK.
func (c *Channel) SendText(ctx context.Context, chatID string, text string) error {
	content, _ := json.Marshal(map[string]string{"text": text})

	// Determine receive_id_type
	receiveIDType := "chat_id"
	if strings.HasPrefix(chatID, "ou_") {
		receiveIDType = "open_id"
	} else if strings.HasPrefix(chatID, "on_") {
		receiveIDType = "union_id"
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType("text").
			Content(string(content)).
			Build()).
		Build()

	resp, err := c.client.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error: %d - %s", resp.Code, resp.Msg)
	}

	c.logger.Debug("message sent", zap.String("chat_id", chatID))
	return nil
}

// Send sends a message through Feishu (implements channel.Channel interface).
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	return c.SendText(ctx, msg.ChatID, msg.Content)
}

// SendCard sends an interactive card message.
func (c *Channel) SendCard(ctx context.Context, chatID string, cardJSON string) error {
	receiveIDType := "chat_id"
	if strings.HasPrefix(chatID, "ou_") {
		receiveIDType = "open_id"
	} else if strings.HasPrefix(chatID, "on_") {
		receiveIDType = "union_id"
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType("interactive").
			Content(cardJSON).
			Build()).
		Build()

	resp, err := c.client.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send card: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error: %d - %s", resp.Code, resp.Msg)
	}

	return nil
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

	// Use sync.Once to ensure channel is only closed once
	c.closeOnce.Do(func() {
		close(c.messages)
	})
	c.logger.Info("feishu channel stopped")
	return nil
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

// Info returns current information about the channel.
func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return channel.Info{
		Name:         "feishu",
		Type:         "feishu",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
		Metadata: map[string]interface{}{
			"app_id":     c.config.AppID,
			"connection": "websocket",
		},
	}
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

// RegisterCommand registers a bot command handler.
func (c *Channel) RegisterCommand(cmd string, handler BotCommandHandler) {
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	c.commandHandlers[cmd] = handler
}

// processCommand checks if a message is a command and processes it.
func (c *Channel) processCommand(ctx context.Context, content string, chatID string, userID string) bool {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "/") {
		return false
	}

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

	c.logger.Debug("processing command", zap.String("command", cmd), zap.String("args", args))

	go func() {
		response, err := handler(ctx, cmd, args, chatID, userID)
		if err != nil {
			c.logger.Error("command handler error", zap.String("command", cmd), zap.Error(err))
			response = "处理命令时发生错误，请稍后重试。"
		}
		if response != "" {
			if err := c.SendText(ctx, chatID, response); err != nil {
				c.logger.Error("failed to send command response", zap.Error(err))
			}
		}
	}()

	return true
}

// handleHelpCommand handles the /help command.
func (c *Channel) handleHelpCommand(ctx context.Context, cmd string, args string, chatID string, userID string) (string, error) {
	return "📚 帮助信息\n\n可用命令：\n/start - 开始使用机器人\n/help - 显示帮助信息\n\n使用方法：\n直接发送消息即可与 AI 对话。", nil
}

// handleStartCommand handles the /start command.
func (c *Channel) handleStartCommand(ctx context.Context, cmd string, args string, chatID string, userID string) (string, error) {
	return "👋 欢迎使用 ZimaOS Echo\n\n我是您的 AI 助手，可以帮助您：\n• 回答问题\n• 处理任务\n• 提供建议\n\n直接发送消息开始对话吧！", nil
}

// GetClient returns the Lark client for advanced usage.
func (c *Channel) GetClient() *lark.Client {
	return c.client
}

// SendStreaming sends a message with streaming support.
// For Feishu, we accumulate the content and send as a single message.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	var fullContent strings.Builder

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				// Channel closed, send final message
				if fullContent.Len() > 0 {
					return c.SendText(ctx, chatID, fullContent.String())
				}
				return nil
			}
			fullContent.WriteString(chunk)
		}
	}
}
