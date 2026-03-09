// Package feishu provides a Feishu/Lark bot channel implementation using lightweight HTTP+WebSocket client.
package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

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

	mu            sync.RWMutex
	status        channel.Status
	connectedAt   *time.Time
	lastError     string
	lastErrorAt   *time.Time
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

	typingReactionMu      sync.Mutex
	typingReactionByMsgID map[string]*typingReactionState

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
		config:                cfg,
		logger:                logger.With(zap.String("channel", "feishu")),
		messages:              make(chan channel.Message, 100),
		status:                channel.StatusDisconnected,
		commandHandlers:       make(map[string]BotCommandHandler),
		typingReactionByMsgID: make(map[string]*typingReactionState),
	}
	c.RegisterCommand("/help", c.handleHelpCommand)
	c.RegisterCommand("/start", c.handleStartCommand)
	return c
}

func (c *Channel) Name() string { return "feishu" }
func (c *Channel) Type() string { return "feishu" }

func (c *Channel) SetMessageHandler(handler MessageHandler)             { c.messageHandler = handler }
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
			MessageID   string          `json:"message_id"`
			ChatID      string          `json:"chat_id"`
			ChatType    string          `json:"chat_type"`
			MessageType string          `json:"message_type"`
			Content     string          `json:"content"`
			ParentID    string          `json:"parent_id"`
			CreateTime  json.RawMessage `json:"create_time"`
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
		var tc struct {
			Text string `json:"text"`
		}
		json.Unmarshal([]byte(msg.Content), &tc)
		content = tc.Text
	case "image":
		var ic struct {
			ImageKey string `json:"image_key"`
		}
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

	c.addTypingReaction(ctx, messageID)
	c.startTypingReactionKeepalive(ctx, messageID, 5*time.Second)

	msgTimestamp := time.Now()
	if ts, ok := parseFeishuMessageTimestamp(msg.CreateTime); ok {
		msgTimestamp = ts
	}

	channelMsg := channel.Message{
		ID: messageID, ChannelName: "feishu", ChatID: chatID, UserID: userID,
		Type: c.convertMessageType(msgType), Content: content, Timestamp: msgTimestamp,
		IsGroup:   msg.ChatType == "group",
		ReplyToID: msg.ParentID,
		Metadata:  map[string]interface{}{"msg_type": msgType, "language": "zh-CN"},
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
				if sendErr := c.SendText(c.ctx, chatID, errMsg, messageID); sendErr != nil {
					c.logger.Error("failed to send error message to user", zap.Error(sendErr), zap.String("original_error", err.Error()))
				}
				return
			}
			if response == "" {
				return
			}
			if err := c.SendText(c.ctx, chatID, response, messageID); err != nil {
				c.logger.Error("failed to send response", zap.Error(err))
			} else if c.sessionManager != nil && sessionID != "" {
				c.sessionManager.EmitMessageSent(sessionID, userID, response)
			}
		}()
	} else {
		c.safeSendMessage(channelMsg, messageID)
	}
}

func parseFeishuMessageTimestamp(raw json.RawMessage) (time.Time, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}, false
	}

	var tsText string
	if err := json.Unmarshal(raw, &tsText); err == nil {
		tsText = strings.TrimSpace(tsText)
		if tsText != "" {
			if ts, err := strconv.ParseInt(tsText, 10, 64); err == nil {
				return normalizeFeishuUnixTimestamp(ts)
			}
		}
	}

	var tsInt int64
	if err := json.Unmarshal(raw, &tsInt); err == nil {
		return normalizeFeishuUnixTimestamp(tsInt)
	}

	var tsFloat float64
	if err := json.Unmarshal(raw, &tsFloat); err == nil {
		return normalizeFeishuUnixTimestamp(int64(tsFloat))
	}

	return time.Time{}, false
}

func normalizeFeishuUnixTimestamp(ts int64) (time.Time, bool) {
	if ts <= 0 {
		return time.Time{}, false
	}
	switch {
	case ts >= 1_000_000_000_000_000_000:
		return time.Unix(0, ts), true // nanoseconds
	case ts >= 1_000_000_000_000_000:
		return time.Unix(0, ts*int64(time.Microsecond)), true // microseconds
	case ts >= 1_000_000_000_000:
		return time.Unix(0, ts*int64(time.Millisecond)), true // milliseconds
	default:
		return time.Unix(ts, 0), true // seconds
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
	c.removeTypingReaction(ctx, replyToID)
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
	if cardJSON, ok, err := feishuCardJSON(msg.Metadata); err != nil {
		return err
	} else if ok {
		return c.sendCardMessage(ctx, msg.ChatID, cardJSON, msg.ReplyToID)
	}

	sentAny := false
	for _, att := range msg.Attachments {
		if err := c.sendAttachment(ctx, msg.ChatID, msg.ReplyToID, att); err != nil {
			c.logger.Warn("failed to send attachment, falling back to text",
				zap.String("type", string(att.Type)),
				zap.String("name", att.Name),
				zap.Error(err))
			fallback := feishuAttachmentFallback(att)
			if fallback == "" {
				continue
			}
			if err := c.SendText(ctx, msg.ChatID, fallback, msg.ReplyToID); err != nil {
				return fmt.Errorf("failed to send attachment fallback: %w", err)
			}
			sentAny = true
			continue
		}
		sentAny = true
	}
	if msg.Content != "" {
		if strings.EqualFold(msg.Format, "markdown") {
			if err := c.SendMarkdown(ctx, msg.ChatID, msg.Content, msg.ReplyToID); err != nil {
				return err
			}
		} else {
			if err := c.SendText(ctx, msg.ChatID, msg.Content, msg.ReplyToID); err != nil {
				return err
			}
		}
		sentAny = true
	}
	if !sentAny {
		return fmt.Errorf("no sendable Feishu content")
	}
	return nil
}

// sendAttachment uploads and sends a single attachment via Feishu API.
func (c *Channel) sendAttachment(ctx context.Context, chatID, replyToID string, att channel.Attachment) error {
	if len(att.Data) == 0 {
		return fmt.Errorf("no binary data in attachment (URL: %s)", att.URL)
	}

	receiveIDType := "chat_id"
	if strings.HasPrefix(chatID, "ou_") {
		receiveIDType = "open_id"
	} else if strings.HasPrefix(chatID, "on_") {
		receiveIDType = "union_id"
	}

	switch att.Type {
	case channel.MessageTypeImage:
		imageKey, err := c.client.uploadImage(ctx, att.Data, att.MimeType)
		if err != nil {
			return fmt.Errorf("upload image: %w", err)
		}
		content, _ := json.Marshal(map[string]string{"image_key": imageKey})
		c.removeTypingReaction(ctx, replyToID)
		if err := c.client.sendMessage(ctx, receiveIDType, chatID, "image", string(content), replyToID); err != nil {
			return err
		}
		c.msgsSent.Add(1)
		return nil

	case channel.MessageTypeVideo:
		// Feishu requires "media" msg_type for video, with both file_key and image_key (cover).
		fileKey, err := c.client.uploadFile(ctx, att.Data, videoFileName(att.Name), "mp4")
		if err != nil {
			return fmt.Errorf("upload video: %w", err)
		}
		// Upload cover image — use provided thumbnail or a minimal placeholder
		coverData := att.Thumbnail
		if len(coverData) == 0 {
			coverData = minimalPNG()
		}
		imageKey, err := c.client.uploadImage(ctx, coverData, "image/png")
		if err != nil {
			return fmt.Errorf("upload video cover: %w", err)
		}
		content, _ := json.Marshal(map[string]string{"file_key": fileKey, "image_key": imageKey})
		c.removeTypingReaction(ctx, replyToID)
		if err := c.client.sendMessage(ctx, receiveIDType, chatID, "media", string(content), replyToID); err != nil {
			return err
		}
		c.msgsSent.Add(1)
		return nil

	case channel.MessageTypeAudio, channel.MessageTypeFile:
		fileType := "stream"
		if att.Type == channel.MessageTypeAudio {
			fileType = "opus"
		}
		fileName := att.Name
		if fileName == "" {
			if att.Type == channel.MessageTypeAudio {
				fileName = "audio.opus"
			} else {
				fileName = "file"
			}
		}
		fileKey, err := c.client.uploadFile(ctx, att.Data, fileName, fileType)
		if err != nil {
			return fmt.Errorf("upload file: %w", err)
		}
		content, _ := json.Marshal(map[string]string{"file_key": fileKey})
		c.removeTypingReaction(ctx, replyToID)
		if err := c.client.sendMessage(ctx, receiveIDType, chatID, "file", string(content), replyToID); err != nil {
			return err
		}
		c.msgsSent.Add(1)
		return nil

	default:
		return fmt.Errorf("unsupported attachment type: %s", att.Type)
	}
}

func (c *Channel) SendCard(ctx context.Context, chatID string, cardJSON string) error {
	return c.sendCardMessage(ctx, chatID, cardJSON, "")
}

func (c *Channel) sendCardMessage(ctx context.Context, chatID string, cardJSON string, replyToID string) error {
	receiveIDType := resolveReceiveIDType(chatID)
	c.removeTypingReaction(ctx, replyToID)
	if err := c.client.sendMessage(ctx, receiveIDType, chatID, "interactive", cardJSON, replyToID); err != nil {
		return err
	}
	c.msgsSent.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastReplyAt = &now
	c.mu.Unlock()
	return nil
}

// SendMarkdown sends markdown content using Feishu interactive cards.
// It preserves markdown structure (including headings/tables) instead of humanizing to plain text.
func (c *Channel) SendMarkdown(ctx context.Context, chatID, markdown, replyToID string) error {
	cardJSON, err := buildMarkdownCardJSON(markdown)
	if err != nil {
		return err
	}
	return c.sendCardMessage(ctx, chatID, cardJSON, replyToID)
}

func feishuCardJSON(metadata map[string]interface{}) (string, bool, error) {
	if metadata == nil {
		return "", false, nil
	}
	for _, key := range []string{"card_json", "card"} {
		raw, ok := metadata[key]
		if !ok {
			continue
		}
		switch value := raw.(type) {
		case string:
			if strings.TrimSpace(value) == "" {
				return "", false, nil
			}
			return value, true, nil
		default:
			body, err := json.Marshal(value)
			if err != nil {
				return "", false, fmt.Errorf("failed to marshal Feishu %s: %w", key, err)
			}
			return string(body), true, nil
		}
	}
	return "", false, nil
}

func feishuAttachmentFallback(att channel.Attachment) string {
	if strings.TrimSpace(att.URL) != "" {
		return strings.TrimSpace(att.URL)
	}
	if strings.TrimSpace(att.Name) != "" {
		return strings.TrimSpace(att.Name)
	}
	if len(att.Data) == 0 {
		return ""
	}
	switch att.Type {
	case channel.MessageTypeImage:
		return "Image attachment"
	case channel.MessageTypeVideo:
		return "Video attachment"
	case channel.MessageTypeAudio:
		return "Audio attachment"
	default:
		return "File attachment"
	}
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
		Metadata: map[string]interface{}{
			"app_id":                  c.config.AppID,
			"connection":              "websocket",
			"typing_reaction_enabled": !c.config.DisableTypingReaction,
		},
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

// videoFileName returns a sensible filename for a video attachment.
func videoFileName(name string) string {
	if name != "" {
		return name
	}
	return "video.mp4"
}

// minimalPNG returns a 1x1 black PNG image (67 bytes) for use as a placeholder cover.
func minimalPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, // 8-bit RGB
		0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54, // IDAT chunk
		0x08, 0xd7, 0x63, 0x60, 0x60, 0x60, 0x00, 0x00, // deflated black pixel
		0x00, 0x04, 0x00, 0x01, 0x27, 0x34, 0x27, 0x0a,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, // IEND chunk
		0xae, 0x42, 0x60, 0x82,
	}
}

func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	// Keep streaming card updates in multi-second cadence to reduce edit noise/rate pressure.
	const updateInterval = 3 * time.Second
	var fullContent strings.Builder
	var messageID string
	var dirty bool
	var clearedReaction bool
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	publish := func(final bool) error {
		if !dirty && !final {
			return nil
		}
		text := fullContent.String()
		if text == "" && !final {
			return nil
		}
		cardJSON, err := buildStreamingCardJSON(text, final)
		if err != nil {
			return err
		}
		receiveIDType := resolveReceiveIDType(chatID)
		if messageID == "" {
			if !clearedReaction {
				c.removeTypingReaction(ctx, replyToID)
				clearedReaction = true
			}
			sentID, err := c.client.sendMessageWithID(ctx, receiveIDType, chatID, "interactive", cardJSON, replyToID)
			if err != nil {
				return err
			}
			messageID = sentID
		} else {
			if err := c.client.updateMessage(ctx, messageID, "interactive", cardJSON); err != nil {
				return err
			}
		}
		dirty = false
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := publish(false); err != nil {
				return err
			}
		case chunk, ok := <-content:
			if !ok {
				if err := publish(true); err != nil {
					return err
				}
				return nil
			}
			fullContent.WriteString(chunk)
			dirty = true
		}
	}
}

func resolveReceiveIDType(chatID string) string {
	if strings.HasPrefix(chatID, "ou_") {
		return "open_id"
	}
	if strings.HasPrefix(chatID, "on_") {
		return "union_id"
	}
	return "chat_id"
}

func buildStreamingCardJSON(text string, final bool) (string, error) {
	parts := splitMarkdownForCard(text, 1400)
	elements := make([]map[string]string, 0, len(parts)+1)
	if len(parts) == 0 {
		parts = []string{" "}
	}
	for _, p := range parts {
		elements = append(elements, map[string]string{
			"tag":     "markdown",
			"content": p,
		})
	}
	if !final {
		elements = append(elements, map[string]string{
			"tag":     "markdown",
			"content": "\n`...`",
		})
	}
	card := map[string]interface{}{
		"config": map[string]bool{
			"wide_screen_mode": true,
		},
		"elements": elements,
	}
	raw, err := json.Marshal(card)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func splitMarkdownForCard(s string, limit int) []string {
	if limit <= 0 {
		limit = 1400
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if strings.TrimSpace(s) == "" {
		return nil
	}

	lines := strings.SplitAfter(s, "\n")
	chunks := make([]string, 0, len(lines)/6+1)

	var cur strings.Builder
	curRunes := 0
	inFence := false
	fenceHeader := "```"

	flush := func() {
		if curRunes == 0 {
			return
		}
		out := cur.String()
		if inFence {
			out += "\n```"
		}
		out = strings.TrimSuffix(out, "\n")
		if out != "" {
			chunks = append(chunks, out)
		}
		cur.Reset()
		curRunes = 0
		if inFence {
			cur.WriteString(fenceHeader)
			cur.WriteString("\n")
			curRunes = utf8.RuneCountInString(fenceHeader) + 1
		}
	}

	var appendPiece func(string)
	appendPiece = func(piece string) {
		if piece == "" {
			return
		}
		pieceRunes := utf8.RuneCountInString(piece)
		if pieceRunes > limit {
			runes := []rune(piece)
			for len(runes) > 0 {
				n := limit
				if n > len(runes) {
					n = len(runes)
				}
				appendPiece(string(runes[:n]))
				runes = runes[n:]
			}
			return
		}
		if curRunes > 0 && curRunes+pieceRunes > limit {
			flush()
		}
		cur.WriteString(piece)
		curRunes += pieceRunes

		trimmed := strings.TrimSpace(piece)
		if strings.HasPrefix(trimmed, "```") {
			if !inFence {
				inFence = true
				fenceHeader = trimmed
			} else {
				inFence = false
				fenceHeader = "```"
			}
		}
	}

	for _, line := range lines {
		appendPiece(line)
	}
	flush()
	return chunks
}

func buildMarkdownCardJSON(markdown string) (string, error) {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	title, body := extractMarkdownCardTitle(markdown)
	body = normalizeMarkdownTablesForCard(body)

	parts := splitMarkdownForCard(body, 1400)
	if len(parts) == 0 {
		parts = []string{" "}
	}

	elements := make([]map[string]string, 0, len(parts))
	for _, p := range parts {
		elements = append(elements, map[string]string{
			"tag":     "markdown",
			"content": p,
		})
	}

	card := map[string]interface{}{
		"config": map[string]bool{
			"wide_screen_mode": true,
		},
		"elements": elements,
	}
	if title != "" {
		card["header"] = map[string]interface{}{
			"title": map[string]string{
				"tag":     "plain_text",
				"content": title,
			},
		}
	}

	raw, err := json.Marshal(card)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func extractMarkdownCardTitle(markdown string) (title, body string) {
	lines := strings.Split(markdown, "\n")
	headingIdx := -1
	headingContent := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if h, ok := parseATXHeading(trimmed); ok {
			headingIdx = i
			headingContent = h
		}
		break
	}

	if headingIdx < 0 {
		return "", markdown
	}

	rest := make([]string, 0, len(lines)-1)
	rest = append(rest, lines[:headingIdx]...)
	next := headingIdx + 1
	for next < len(lines) && strings.TrimSpace(lines[next]) == "" {
		next++
	}
	rest = append(rest, lines[next:]...)
	return headingContent, strings.Join(rest, "\n")
}

func parseATXHeading(line string) (string, bool) {
	if line == "" || line[0] != '#' {
		return "", false
	}
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i == 0 || i > 6 {
		return "", false
	}
	if i >= len(line) || line[i] != ' ' {
		return "", false
	}
	content := strings.TrimSpace(line[i+1:])
	if content == "" {
		return "", false
	}
	return content, true
}

func normalizeMarkdownTablesForCard(markdown string) string {
	lines := strings.Split(markdown, "\n")
	if len(lines) == 0 {
		return markdown
	}

	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		if !isPipeTableLine(lines[i]) || i+1 >= len(lines) || !isPipeTableLine(lines[i+1]) {
			out = append(out, lines[i])
			i++
			continue
		}

		header := parseMarkdownTableCells(lines[i])
		divider := parseMarkdownTableCells(lines[i+1])
		if !isMarkdownTableDivider(divider) {
			out = append(out, lines[i])
			i++
			continue
		}

		rows := [][]string{header}
		i += 2
		for i < len(lines) && isPipeTableLine(lines[i]) {
			rows = append(rows, parseMarkdownTableCells(lines[i]))
			i++
		}
		out = append(out, renderMarkdownTableAsCodeFence(rows))
	}

	return strings.Join(out, "\n")
}

func isPipeTableLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return len(trimmed) >= 2 && trimmed[0] == '|' && trimmed[len(trimmed)-1] == '|'
}

func parseMarkdownTableCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) > 0 && trimmed[0] == '|' {
		trimmed = trimmed[1:]
	}
	if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '|' {
		trimmed = trimmed[:len(trimmed)-1]
	}
	parts := strings.Split(trimmed, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

func isMarkdownTableDivider(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		normalized := strings.TrimSpace(c)
		normalized = strings.Trim(normalized, ":-")
		if normalized != "" {
			return false
		}
	}
	return true
}

func renderMarkdownTableAsCodeFence(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}

	colCount := 0
	for _, row := range rows {
		if len(row) > colCount {
			colCount = len(row)
		}
	}
	widths := make([]int, colCount)
	for _, row := range rows {
		for i, cell := range row {
			w := utf8.RuneCountInString(cell)
			if w > widths[i] {
				widths[i] = w
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("```text\n")
	writeMarkdownTableCodeRow(&sb, rows[0], widths)
	sb.WriteString("|")
	for _, w := range widths {
		if w < 3 {
			w = 3
		}
		sb.WriteString(" ")
		sb.WriteString(strings.Repeat("-", w))
		sb.WriteString(" |")
	}
	sb.WriteByte('\n')
	for _, row := range rows[1:] {
		writeMarkdownTableCodeRow(&sb, row, widths)
	}
	sb.WriteString("```")
	return sb.String()
}

func writeMarkdownTableCodeRow(sb *strings.Builder, row []string, widths []int) {
	sb.WriteString("|")
	for i, w := range widths {
		cell := ""
		if i < len(row) {
			cell = row[i]
		}
		sb.WriteString(" ")
		sb.WriteString(cell)
		pad := w - utf8.RuneCountInString(cell)
		for j := 0; j < pad; j++ {
			sb.WriteByte(' ')
		}
		sb.WriteString(" |")
	}
	sb.WriteByte('\n')
}

const typingReactionEmojiType = "Typing"

type typingReactionState struct {
	reactionID      string
	ready           chan struct{}
	removeScheduled bool
	resendInFlight  bool
}

func (c *Channel) addTypingReaction(ctx context.Context, messageID string) {
	if c.client == nil || messageID == "" || c.config.DisableTypingReaction {
		return
	}
	c.typingReactionMu.Lock()
	if _, exists := c.typingReactionByMsgID[messageID]; exists {
		c.typingReactionMu.Unlock()
		return
	}
	state := &typingReactionState{ready: make(chan struct{})}
	c.typingReactionByMsgID[messageID] = state
	c.typingReactionMu.Unlock()

	go func() {
		reactionID, err := c.client.addMessageReaction(ctx, messageID, typingReactionEmojiType)
		if err != nil {
			c.logger.Debug("failed to add typing reaction", zap.String("message_id", messageID), zap.Error(err))
		}

		c.typingReactionMu.Lock()
		current, ok := c.typingReactionByMsgID[messageID]
		if ok && current == state {
			if err == nil {
				state.reactionID = reactionID
			}
			close(state.ready)
		}
		c.typingReactionMu.Unlock()
	}()
}

func (c *Channel) startTypingReactionKeepalive(ctx context.Context, messageID string, interval time.Duration) {
	if c.client == nil || messageID == "" || c.config.DisableTypingReaction {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.typingReactionMu.Lock()
				state := c.typingReactionByMsgID[messageID]
				stop := state == nil || state.removeScheduled
				c.typingReactionMu.Unlock()
				if stop {
					return
				}
				c.resendTypingReaction(ctx, messageID)
			}
		}
	}()
}

func (c *Channel) resendTypingReaction(ctx context.Context, messageID string) {
	if c.client == nil || messageID == "" || c.config.DisableTypingReaction {
		return
	}

	c.typingReactionMu.Lock()
	state := c.typingReactionByMsgID[messageID]
	if state == nil || state.removeScheduled || state.resendInFlight {
		c.typingReactionMu.Unlock()
		return
	}
	state.resendInFlight = true
	c.typingReactionMu.Unlock()
	defer func() {
		c.typingReactionMu.Lock()
		current, ok := c.typingReactionByMsgID[messageID]
		if ok && current == state {
			state.resendInFlight = false
		}
		c.typingReactionMu.Unlock()
	}()

	<-state.ready

	c.typingReactionMu.Lock()
	current, ok := c.typingReactionByMsgID[messageID]
	if !ok || current != state || state.removeScheduled {
		c.typingReactionMu.Unlock()
		return
	}
	oldReactionID := state.reactionID
	c.typingReactionMu.Unlock()

	if oldReactionID != "" {
		if err := c.client.deleteMessageReaction(ctx, messageID, oldReactionID); err != nil {
			c.logger.Debug("failed to delete typing reaction before resend",
				zap.String("message_id", messageID),
				zap.String("reaction_id", oldReactionID),
				zap.Error(err))
		}
	}

	newReactionID, err := c.client.addMessageReaction(ctx, messageID, typingReactionEmojiType)
	if err != nil {
		c.logger.Debug("failed to resend typing reaction", zap.String("message_id", messageID), zap.Error(err))
		return
	}

	c.typingReactionMu.Lock()
	current, ok = c.typingReactionByMsgID[messageID]
	if ok && current == state && !state.removeScheduled {
		state.reactionID = newReactionID
		c.typingReactionMu.Unlock()
		return
	}
	c.typingReactionMu.Unlock()

	// If message already completed and state was removed, avoid leaking the newly re-sent reaction.
	if newReactionID != "" {
		if err := c.client.deleteMessageReaction(ctx, messageID, newReactionID); err != nil {
			c.logger.Debug("failed to cleanup resent typing reaction",
				zap.String("message_id", messageID),
				zap.String("reaction_id", newReactionID),
				zap.Error(err))
		}
	}
}

func (c *Channel) removeTypingReaction(ctx context.Context, messageID string) {
	if c.client == nil || messageID == "" {
		return
	}
	c.typingReactionMu.Lock()
	state := c.typingReactionByMsgID[messageID]
	if state == nil {
		c.typingReactionMu.Unlock()
		return
	}
	if state.removeScheduled {
		c.typingReactionMu.Unlock()
		return
	}
	state.removeScheduled = true
	c.typingReactionMu.Unlock()

	select {
	case <-state.ready:
	case <-ctx.Done():
		return
	}

	// Wait for any in-flight resend operation to complete so we delete the latest reaction id.
	for {
		c.typingReactionMu.Lock()
		current, ok := c.typingReactionByMsgID[messageID]
		if !ok || current != state {
			c.typingReactionMu.Unlock()
			return
		}
		if !state.resendInFlight {
			reactionID := state.reactionID
			delete(c.typingReactionByMsgID, messageID)
			c.typingReactionMu.Unlock()

			if reactionID == "" {
				return
			}
			if err := c.client.deleteMessageReaction(ctx, messageID, reactionID); err != nil {
				c.logger.Debug("failed to remove typing reaction",
					zap.String("message_id", messageID),
					zap.String("reaction_id", reactionID),
					zap.Error(err))
			}
			return
		}
		c.typingReactionMu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-time.After(20 * time.Millisecond):
		}
	}
}
