// Package feishu provides a Feishu/Lark bot channel implementation using lightweight HTTP+WebSocket client.
package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/humanizer"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
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
	msgCount      atomic.Int64
	msgsReceived  atomic.Int64
	msgsSent      atomic.Int64
	lastMessageAt *time.Time
	lastReplyAt   *time.Time

	// Lightweight clients (no larksuite SDK)
	client   *larkClient
	wsClient *larkWSClient

	// Bot commands
	commandHandlers map[string]BotCommandHandler
	commandMu       sync.RWMutex

	// Message handler for AI processing
	messageHandler MessageHandler

	// Bot session manager for monitoring
	sessionManager *channel.BotSessionManager

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
	c.RegisterCommand("/help", c.handleHelpCommand)
	c.RegisterCommand("/start", c.handleStartCommand)
	return c
}

func (c *Channel) Name() string { return "feishu" }
func (c *Channel) Type() string { return "feishu" }

func (c *Channel) SetMessageHandler(handler MessageHandler) { c.messageHandler = handler }
func (c *Channel) SetSessionManager(manager *channel.BotSessionManager) { c.sessionManager = manager }

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

	c.client = newLarkClient(c.config.AppID, c.config.AppSecret)
	c.wsClient = newLarkWSClient(c.config.AppID, c.config.AppSecret, c.logger, c.onWSEvent)

	go func() {
		c.logger.Info("starting feishu websocket connection")
		if err := c.wsClient.start(c.ctx); err != nil {
			c.logger.Error("feishu websocket error", zap.Error(err))
			c.setError(fmt.Sprintf("websocket error: %v", err))
		}
	}()

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

// onWSEvent handles raw event payloads from the WebSocket client.
func (c *Channel) onWSEvent(ctx context.Context, payload []byte) {
	var envelope struct {
		Schema string `json:"schema"`
		Header struct {
			EventType string `json:"event_type"`
		} `json:"header"`
		Event json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		c.logger.Error("failed to parse event envelope", zap.Error(err))
		return
	}

	switch envelope.Header.EventType {
	case "im.message.receive_v1":
		c.onMessageReceive(ctx, envelope.Event)
	case "p2.chat_access_event.bot_p2p_chat_entered_v1":
		c.onBotP2pChatEntered(ctx, envelope.Event)
	case "im.message.message_read_v1":
		c.logger.Debug("message read event")
	default:
		c.logger.Debug("unhandled event type", zap.String("type", envelope.Header.EventType))
	}
}

func (c *Channel) onMessageReceive(ctx context.Context, eventData json.RawMessage) {
	var event struct {
		Message struct {
			MessageID   string `json:"message_id"`
			ChatID      string `json:"chat_id"`
			ChatType    string `json:"chat_type"`
			MessageType string `json:"message_type"`
			Content     string `json:"content"`
			ParentID    string `json:"parent_id"`
		} `json:"message"`
		Sender struct {
			SenderID struct {
				OpenID string `json:"open_id"`
			} `json:"sender_id"`
		} `json:"sender"`
	}
	if err := json.Unmarshal(eventData, &event); err != nil {
		c.logger.Error("failed to parse message event", zap.Error(err))
		return
	}

	msg := event.Message
	sender := event.Sender

	var content string
	var attachment *channel.Attachment
	msgType := msg.MessageType

	switch msgType {
	case "text":
		var tc struct{ Text string `json:"text"` }
		json.Unmarshal([]byte(msg.Content), &tc)
		content = tc.Text
	case "image":
		var ic struct{ ImageKey string `json:"image_key"` }
		json.Unmarshal([]byte(msg.Content), &ic)
		content = ic.ImageKey
		if ic.ImageKey != "" && msg.MessageID != "" {
			if data, err := c.client.getMessageResource(ctx, msg.MessageID, ic.ImageKey, "image"); err == nil {
				attachment = &channel.Attachment{ID: ic.ImageKey, Type: channel.MessageTypeImage, Name: ic.ImageKey + ".png", Data: data, Size: int64(len(data)), MimeType: "image/png"}
			}
		}
	case "audio":
		var ac struct {
			FileKey  string `json:"file_key"`
			Duration int    `json:"duration"`
		}
		json.Unmarshal([]byte(msg.Content), &ac)
		content = ac.FileKey
		if ac.FileKey != "" && msg.MessageID != "" {
			if data, err := c.client.getMessageResource(ctx, msg.MessageID, ac.FileKey, "file"); err == nil {
				attachment = &channel.Attachment{ID: ac.FileKey, Type: channel.MessageTypeAudio, Name: ac.FileKey + ".opus", Data: data, Size: int64(len(data)), MimeType: "audio/opus"}
			}
		}
	case "file":
		var fc struct {
			FileKey  string `json:"file_key"`
			FileName string `json:"file_name"`
		}
		json.Unmarshal([]byte(msg.Content), &fc)
		content = fc.FileName
		if fc.FileKey != "" && msg.MessageID != "" {
			if data, err := c.client.getMessageResource(ctx, msg.MessageID, fc.FileKey, "file"); err == nil {
				attachment = &channel.Attachment{ID: fc.FileKey, Type: channel.MessageTypeFile, Name: fc.FileName, Data: data, Size: int64(len(data)), MimeType: "application/octet-stream"}
			}
		}
	default:
		content = msg.Content
	}

	chatID := msg.ChatID
	userID := sender.SenderID.OpenID
	messageID := msg.MessageID

	c.logger.Info("received message", zap.String("chat_id", chatID), zap.String("user_id", userID), zap.String("content", content), zap.String("msg_type", msgType))

	if c.processCommand(ctx, content, chatID, userID) {
		return
	}

	channelMsg := channel.Message{
		ID: messageID, ChannelName: "feishu", ChatID: chatID, UserID: userID,
		Type: c.convertMessageType(msgType), Content: content, Timestamp: time.Now(),
		IsGroup:  msg.ChatType == "group",
		ReplyToID: msg.ParentID,
		Metadata: map[string]interface{}{"msg_type": msgType, "language": "zh-CN"},
	}
	if attachment != nil {
		channelMsg.Attachments = []channel.Attachment{*attachment}
	}

	c.msgCount.Add(1)
	c.msgsReceived.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastMessageAt = &now
	c.mu.Unlock()

	var sessionID string
	if c.sessionManager != nil {
		sessionID, _ = c.sessionManager.GetOrCreateSession(ctx, chatID, userID)
		c.sessionManager.EmitMessageReceived(sessionID, userID, content)
	}

	if c.messageHandler != nil {
		go func() {
			response, err := c.messageHandler(c.ctx, channelMsg)
			if err != nil {
				c.logger.Error("message handler error", zap.Error(err))
				if c.sessionManager != nil && sessionID != "" {
					c.sessionManager.EmitError(sessionID, userID, err.Error())
				}
				// Extract language from message metadata for i18n
				lang := i18n.DefaultLanguage
				if channelMsg.Metadata != nil {
					if langStr, ok := channelMsg.Metadata["language"].(string); ok {
						lang = i18n.ParseLanguage(langStr)
					}
				}
				// Send user-friendly error message
				errMsg := i18n.T(lang, i18n.MsgProcessingError, err)
				if sendErr := c.SendText(c.ctx, chatID, errMsg, ""); sendErr != nil {
					c.logger.Error("failed to send error message to user", zap.Error(sendErr), zap.String("original_error", err.Error()))
				}
				return
			}
			if response == "" {
				return
			}
			if err := c.SendText(c.ctx, chatID, response, ""); err != nil {
				c.logger.Error("failed to send response", zap.Error(err))
			} else if c.sessionManager != nil && sessionID != "" {
				c.sessionManager.EmitMessageSent(sessionID, userID, response)
			}
		}()
	} else {
		c.safeSendMessage(channelMsg, messageID)
	}
}

func (c *Channel) onBotP2pChatEntered(ctx context.Context, eventData json.RawMessage) {
	var event struct {
		ChatID     *string `json:"chat_id"`
		OperatorID *struct {
			OpenID *string `json:"open_id"`
		} `json:"operator_id"`
	}
	json.Unmarshal(eventData, &event)
	if c.sessionManager != nil && event.ChatID != nil {
		userID := ""
		if event.OperatorID != nil && event.OperatorID.OpenID != nil {
			userID = *event.OperatorID.OpenID
		}
		c.sessionManager.GetOrCreateSession(ctx, *event.ChatID, userID)
	}
}

func (c *Channel) SendText(ctx context.Context, chatID string, text string, replyToID string) error {
	// Humanize: strip markdown formatting for IM readability
	text, _ = humanizer.HumanizeForChannel(text, "feishu")
	content, _ := json.Marshal(map[string]string{"text": text})
	receiveIDType := "chat_id"
	if strings.HasPrefix(chatID, "ou_") {
		receiveIDType = "open_id"
	} else if strings.HasPrefix(chatID, "on_") {
		receiveIDType = "union_id"
	}
	if err := c.client.sendMessage(ctx, receiveIDType, chatID, "text", string(content), replyToID); err != nil {
		return err
	}
	c.msgsSent.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastReplyAt = &now
	c.mu.Unlock()
	return nil
}

func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	return c.SendText(ctx, msg.ChatID, msg.Content, msg.ReplyToID)
}

func (c *Channel) SendCard(ctx context.Context, chatID string, cardJSON string) error {
	receiveIDType := "chat_id"
	if strings.HasPrefix(chatID, "ou_") {
		receiveIDType = "open_id"
	} else if strings.HasPrefix(chatID, "on_") {
		receiveIDType = "union_id"
	}
	return c.client.sendMessage(ctx, receiveIDType, chatID, "interactive", cardJSON, "")
}

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
	c.closeOnce.Do(func() { close(c.messages) })
	c.logger.Info("feishu channel stopped")
	return nil
}

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

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name: "feishu", Type: "feishu", Status: c.status, Enabled: c.config.Enabled,
		ConnectedAt: c.connectedAt, LastError: c.lastError, LastErrorAt: c.lastErrorAt,
		MessageCount: c.msgCount.Load(), MessagesReceived: c.msgsReceived.Load(), MessagesSent: c.msgsSent.Load(),
		LastMessageAt: c.lastMessageAt, LastReplyAt: c.lastReplyAt,
		Metadata: map[string]interface{}{"app_id": c.config.AppID, "connection": "websocket"},
	}
}

func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected
}

func (c *Channel) Messages() <-chan channel.Message { return c.messages }

func (c *Channel) setError(err string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err
	now := time.Now()
	c.lastErrorAt = &now
	c.status = channel.StatusError
}

func (c *Channel) safeSendMessage(msg channel.Message, messageID string) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Warn("channel closed, dropping message", zap.String("message_id", messageID))
		}
	}()
	c.mu.RLock()
	isConnected := c.status == channel.StatusConnected
	c.mu.RUnlock()
	if !isConnected {
		return
	}
	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message", zap.String("message_id", messageID))
	}
}

func (c *Channel) RegisterCommand(cmd string, handler BotCommandHandler) {
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	c.commandHandlers[cmd] = handler
}

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
	go func() {
		response, err := handler(ctx, cmd, args, chatID, userID)
		if err != nil {
			response = "处理命令时发生错误，请稍后重试。"
		}
		if response != "" {
			c.SendText(ctx, chatID, response, "")
		}
	}()
	return true
}

func (c *Channel) handleHelpCommand(ctx context.Context, cmd string, args string, chatID string, userID string) (string, error) {
	return "📚 帮助信息\n\n可用命令：\n/start - 开始使用机器人\n/help - 显示帮助信息\n\n使用方法：\n直接发送消息即可与 AI 对话。", nil
}

func (c *Channel) handleStartCommand(ctx context.Context, cmd string, args string, chatID string, userID string) (string, error) {
	return "👋 欢迎使用 ZimaOS Blue\n\n我是您的 AI 助手，可以帮助您：\n• 回答问题\n• 处理任务\n• 提供建议\n\n直接发送消息开始对话吧！", nil
}

func (c *Channel) GetClient() *larkClient { return c.client }

func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)
	var fullContent strings.Builder
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				if fullContent.Len() > 0 {
					return c.SendText(ctx, chatID, fullContent.String(), "")
				}
				return nil
			}
			fullContent.WriteString(chunk)
		}
	}
}
