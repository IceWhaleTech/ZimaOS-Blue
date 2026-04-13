// Package slack provides a Slack bot channel implementation.
package slack

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

// InteractionSelectedOption represents a selected option in a Slack interaction.
type InteractionSelectedOption struct {
	Value string `json:"value"`
}

// InteractionAction represents a single action in a Slack interaction payload.
type InteractionAction struct {
	ActionID        string                      `json:"action_id"`
	BlockID         string                      `json:"block_id"`
	Type            string                      `json:"type"`
	Value           string                      `json:"value"`
	SelectedOption  InteractionSelectedOption   `json:"selected_option"`
	SelectedOptions []InteractionSelectedOption `json:"selected_options"`
}

// InteractionCallback represents a Slack interaction payload.
type InteractionCallback struct {
	Type        string `json:"type"`
	TriggerID   string `json:"trigger_id"`
	CallbackID  string `json:"callback_id"`
	ResponseURL string `json:"response_url"`
	Channel     struct {
		ID string `json:"id"`
	} `json:"channel"`
	User struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"user"`
	View struct {
		CallbackID string `json:"callback_id"`
	} `json:"view"`
	Actions []InteractionAction `json:"actions"`
}

type InteractionHandler func(ctx context.Context, callback *InteractionCallback) error
type MessageHandler func(ctx context.Context, msg channel.Message) (string, error)

type Channel struct {
	config       channel.SlackConfig
	logger       *zap.Logger
	client       *slackClient
	socketClient *slackSocketClient
	messages     chan channel.Message

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
	botUserID     string

	interactionHandlers map[string]InteractionHandler
	blockActionHandlers map[string]InteractionHandler
	handlerMu           sync.RWMutex

	messageHandler MessageHandler
	sessionManager *channel.BotSessionManager

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(cfg channel.SlackConfig, logger *zap.Logger) *Channel {
	return &Channel{
		config:              cfg,
		logger:              logger.With(zap.String("channel", "slack")),
		messages:            make(chan channel.Message, 100),
		status:              channel.StatusDisconnected,
		interactionHandlers: make(map[string]InteractionHandler),
		blockActionHandlers: make(map[string]InteractionHandler),
	}
}

func (c *Channel) Name() string { return "slack" }
func (c *Channel) Type() string { return "slack" }
func (c *Channel) OutboundCapabilities() channel.OutboundCapabilities {
	return channel.OutboundCapabilities{
		MarkdownMode:    channel.OutboundMarkdownModeChunked,
		HumanizerPreset: "slack",
	}
}
func (c *Channel) SetMessageHandler(handler MessageHandler)             { c.messageHandler = handler }
func (c *Channel) SetSessionManager(manager *channel.BotSessionManager) { c.sessionManager = manager }

func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.client = newSlackClient(c.config.BotToken, c.config.AppToken)

	auth, err := c.client.authTest(ctx)
	if err != nil {
		c.setError(fmt.Sprintf("auth test failed: %v", err))
		return fmt.Errorf("Slack auth test failed: %w", err)
	}
	c.botUserID = auth.UserID
	c.logger.Info("slack bot authenticated", zap.String("user_id", auth.UserID), zap.String("team", auth.Team))

	c.socketClient = newSlackSocketClient(c.config.AppToken, c.processSocketEvent)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if err := c.socketClient.run(c.ctx); err != nil {
			if c.ctx.Err() == nil {
				c.logger.Error("socket mode error", zap.Error(err))
				c.setError(fmt.Sprintf("socket mode error: %v", err))
			}
		}
	}()

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()
	c.logger.Info("slack channel started")
	return nil
}

func (c *Channel) processSocketEvent(env socketEnvelope) {
	switch env.Type {
	case "events_api":
		var wrapper struct {
			Type      string          `json:"type"`
			Event     json.RawMessage `json:"event"`
			EventType string          `json:"-"`
		}
		json.Unmarshal(env.Payload, &wrapper)
		var inner struct {
			Type string `json:"type"`
		}
		json.Unmarshal(wrapper.Event, &inner)
		switch inner.Type {
		case "message":
			c.handleMessageEvent(wrapper.Event)
		case "app_mention":
			c.handleAppMentionEvent(wrapper.Event)
		}
	case "slash_commands":
		c.handleSlashCommand(env.Payload)
	case "interactive":
		var cb InteractionCallback
		if json.Unmarshal(env.Payload, &cb) == nil {
			c.handleInteractionCallback(&cb)
		}
	}
}

type slackMessageEvent struct {
	User        string `json:"user"`
	BotID       string `json:"bot_id"`
	Channel     string `json:"channel"`
	ChannelType string `json:"channel_type"`
	Text        string `json:"text"`
	TS          string `json:"ts"`
	ThreadTS    string `json:"thread_ts"`
}

func (c *Channel) handleMessageEvent(data json.RawMessage) {
	var ev slackMessageEvent
	if json.Unmarshal(data, &ev) != nil {
		return
	}
	if ev.BotID != "" || ev.User == c.botUserID {
		return
	}
	if !c.isChannelAllowed(ev.Channel) || !c.isUserAllowed(ev.User) {
		return
	}

	username := ""
	if c.client != nil {
		username, _ = c.client.getUserInfo(c.ctx, ev.User)
	}
	channelMsg := channel.Message{
		ID: ev.TS, ChannelName: "slack", ChatID: ev.Channel, UserID: ev.User, Username: username,
		Type: channel.MessageTypeText, Content: ev.Text, Timestamp: parseSlackTimestamp(ev.TS),
		IsGroup:  strings.HasPrefix(ev.Channel, "C") || strings.HasPrefix(ev.Channel, "G"),
		Metadata: slackMessageMetadata(ev.ThreadTS, ev.ChannelType, ev.Text),
	}
	if ev.ThreadTS != "" {
		channelMsg.ReplyToID = ev.ThreadTS
	}

	c.msgCount.Add(1)
	c.msgsReceived.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastMessageAt = &now
	c.mu.Unlock()

	chatID, userID := ev.Channel, ev.User
	var sessionID string
	if c.sessionManager != nil {
		sessionID, _ = c.sessionManager.GetOrCreateSession(c.ctx, chatID, userID)
		c.sessionManager.EmitMessageReceived(sessionID, userID, channelMsg.Content)
	}

	if c.messageHandler != nil {
		replyToID := slackReplyTarget(channelMsg)
		go func() {
			response, err := c.messageHandler(c.ctx, channelMsg)
			if err != nil {
				c.logger.Error("message handler error", zap.Error(err))
				if c.sessionManager != nil && sessionID != "" {
					c.sessionManager.EmitError(sessionID, userID, err.Error())
				}
				c.Send(c.ctx, channel.OutgoingMessage{ChatID: chatID, ReplyToID: replyToID, Content: "处理消息时发生错误，请稍后重试。"})
				return
			}
			if response == "" {
				return
			}
			if err := c.Send(c.ctx, channel.OutgoingMessage{ChatID: chatID, ReplyToID: replyToID, Content: response}); err != nil {
				c.logger.Error("failed to send response", zap.Error(err))
			} else if c.sessionManager != nil && sessionID != "" {
				c.sessionManager.EmitMessageSent(sessionID, userID, response)
			}
		}()
		return
	}
	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full", zap.String("id", channelMsg.ID))
	}
}

func (c *Channel) handleAppMentionEvent(data json.RawMessage) {
	var ev struct {
		User     string `json:"user"`
		Channel  string `json:"channel"`
		Text     string `json:"text"`
		TS       string `json:"ts"`
		ThreadTS string `json:"thread_ts"`
	}
	if json.Unmarshal(data, &ev) != nil {
		return
	}
	if !c.isChannelAllowed(ev.Channel) || !c.isUserAllowed(ev.User) {
		return
	}

	channelMsg := channel.Message{
		ID: ev.TS, ChannelName: "slack", ChatID: ev.Channel, UserID: ev.User,
		Type: channel.MessageTypeText, Content: ev.Text, Timestamp: parseSlackTimestamp(ev.TS),
		IsGroup: true, Metadata: slackAppMentionMetadata(ev.ThreadTS, ev.Text),
	}
	if ev.ThreadTS != "" {
		channelMsg.ReplyToID = ev.ThreadTS
	}
	c.msgCount.Add(1)
	c.msgsReceived.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastMessageAt = &now
	c.mu.Unlock()

	chatID, userID := ev.Channel, ev.User
	var sessionID string
	if c.sessionManager != nil {
		sessionID, _ = c.sessionManager.GetOrCreateSession(c.ctx, chatID, userID)
		c.sessionManager.EmitMessageReceived(sessionID, userID, channelMsg.Content)
	}

	if c.messageHandler != nil {
		replyToID := slackReplyTarget(channelMsg)
		go func() {
			response, err := c.messageHandler(c.ctx, channelMsg)
			if err != nil {
				c.logger.Error("message handler error", zap.Error(err))
				if c.sessionManager != nil && sessionID != "" {
					c.sessionManager.EmitError(sessionID, userID, err.Error())
				}
				c.Send(c.ctx, channel.OutgoingMessage{ChatID: chatID, ReplyToID: replyToID, Content: "处理消息时发生错误，请稍后重试。"})
				return
			}
			if response == "" {
				return
			}
			if err := c.Send(c.ctx, channel.OutgoingMessage{ChatID: chatID, ReplyToID: replyToID, Content: response}); err != nil {
				c.logger.Error("failed to send response", zap.Error(err))
			} else if c.sessionManager != nil && sessionID != "" {
				c.sessionManager.EmitMessageSent(sessionID, userID, response)
			}
		}()
		return
	}

	select {
	case c.messages <- channelMsg:
	default:
	}
}

var (
	slackUserMentionPattern     *regexp.Regexp
	slackUserMentionPatternOnce sync.Once
)

func ensureSlackUserMentionPattern() {
	slackUserMentionPatternOnce.Do(func() {
		slackUserMentionPattern = regexp.MustCompile(`<@([A-Z0-9]+)(?:\|[^>]+)?>`)
	})
}

func slackReplyTarget(msg channel.Message) string {
	if strings.TrimSpace(msg.ReplyToID) != "" {
		return msg.ReplyToID
	}
	return msg.ID
}

func slackMessageMetadata(threadTS string, channelType string, text string) map[string]interface{} {
	metadata := map[string]interface{}{"thread_ts": threadTS, "channel_type": channelType}
	slackApplyMentions(metadata, text)
	return metadata
}

func slackAppMentionMetadata(threadTS string, text string) map[string]interface{} {
	metadata := map[string]interface{}{"event_type": "app_mention", "thread_ts": threadTS}
	slackApplyMentions(metadata, text)
	return metadata
}

func slackSlashCommandMetadata(command string, responseURL string, text string) map[string]interface{} {
	metadata := map[string]interface{}{"event_type": "slash_command", "command": command, "response_url": responseURL}
	slackApplyMentions(metadata, text)
	return metadata
}

func slackInteractionMetadata(cb *InteractionCallback) map[string]interface{} {
	metadata := map[string]interface{}{
		"is_interaction":   true,
		"interaction_type": cb.Type,
		"callback_id":      cb.CallbackID,
		"response_url":     cb.ResponseURL,
		"trigger_id":       cb.TriggerID,
	}
	actions := slackInteractionActionMetadata(cb.Actions)
	if len(actions) > 0 {
		metadata["actions"] = actions
		actionIDs := make([]string, 0, len(actions))
		for _, action := range actions {
			if actionID, ok := action["action_id"].(string); ok && strings.TrimSpace(actionID) != "" {
				actionIDs = append(actionIDs, actionID)
			}
		}
		if len(actionIDs) > 0 {
			metadata["action_ids"] = actionIDs
		}
	}
	return metadata
}

func slackInteractionActionMetadata(actions []InteractionAction) []map[string]interface{} {
	if len(actions) == 0 {
		return nil
	}
	metadata := make([]map[string]interface{}, 0, len(actions))
	for _, action := range actions {
		item := map[string]interface{}{}
		if strings.TrimSpace(action.ActionID) != "" {
			item["action_id"] = strings.TrimSpace(action.ActionID)
		}
		if strings.TrimSpace(action.BlockID) != "" {
			item["block_id"] = strings.TrimSpace(action.BlockID)
		}
		if strings.TrimSpace(action.Type) != "" {
			item["type"] = strings.TrimSpace(action.Type)
		}
		selectedValues := make([]string, 0, 1+len(action.SelectedOptions))
		if strings.TrimSpace(action.Value) != "" {
			item["value"] = strings.TrimSpace(action.Value)
			selectedValues = append(selectedValues, strings.TrimSpace(action.Value))
		}
		if strings.TrimSpace(action.SelectedOption.Value) != "" {
			selectedValues = append(selectedValues, strings.TrimSpace(action.SelectedOption.Value))
		}
		for _, option := range action.SelectedOptions {
			if strings.TrimSpace(option.Value) != "" {
				selectedValues = append(selectedValues, strings.TrimSpace(option.Value))
			}
		}
		if len(selectedValues) > 0 {
			item["selected_values"] = selectedValues
		}
		if len(item) > 0 {
			metadata = append(metadata, item)
		}
	}
	return metadata
}

func slackApplyMentions(metadata map[string]interface{}, text string) {
	mentions, ids := slackExtractMentions(text)
	if len(mentions) == 0 {
		return
	}
	metadata["mentions"] = mentions
	metadata["mention_ids"] = ids
}

func slackExtractMentions(text string) ([]map[string]interface{}, []string) {
	ensureSlackUserMentionPattern()
	matches := slackUserMentionPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(matches))
	mentions := make([]map[string]interface{}, 0, len(matches))
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		mentionID := strings.TrimSpace(match[1])
		if mentionID == "" {
			continue
		}
		if _, exists := seen[mentionID]; exists {
			continue
		}
		seen[mentionID] = struct{}{}
		mentions = append(mentions, map[string]interface{}{"type": "user", "id": mentionID})
		ids = append(ids, mentionID)
	}
	return mentions, ids
}

func (c *Channel) handleSlashCommand(data json.RawMessage) {
	var cmd struct {
		Command     string `json:"command"`
		Text        string `json:"text"`
		ChannelID   string `json:"channel_id"`
		UserID      string `json:"user_id"`
		UserName    string `json:"user_name"`
		TriggerID   string `json:"trigger_id"`
		ResponseURL string `json:"response_url"`
	}
	if json.Unmarshal(data, &cmd) != nil {
		return
	}
	if !c.isChannelAllowed(cmd.ChannelID) || !c.isUserAllowed(cmd.UserID) {
		return
	}

	channelMsg := channel.Message{
		ID: cmd.TriggerID, ChannelName: "slack", ChatID: cmd.ChannelID, UserID: cmd.UserID, Username: cmd.UserName,
		Type: channel.MessageTypeText, Content: cmd.Command + " " + cmd.Text, Timestamp: time.Now(), IsGroup: true,
		Metadata: slackSlashCommandMetadata(cmd.Command, cmd.ResponseURL, cmd.Text),
	}
	c.msgCount.Add(1)
	c.msgsReceived.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastMessageAt = &now
	c.mu.Unlock()
	select {
	case c.messages <- channelMsg:
	default:
	}
}

func (c *Channel) handleInteractionCallback(cb *InteractionCallback) {
	switch cb.Type {
	case "block_actions":
		for _, action := range cb.Actions {
			c.handlerMu.RLock()
			handler, exists := c.blockActionHandlers[action.ActionID]
			if !exists {
				for prefix, h := range c.blockActionHandlers {
					if strings.HasPrefix(action.ActionID, prefix) {
						handler = h
						exists = true
						break
					}
				}
			}
			c.handlerMu.RUnlock()
			if exists {
				if err := handler(c.ctx, cb); err != nil {
					c.logger.Error("block action error", zap.Error(err))
				}
				return
			}
		}
	case "shortcut", "view_submission", "message_action":
		callbackID := cb.CallbackID
		if cb.Type == "view_submission" {
			callbackID = cb.View.CallbackID
		}
		c.handlerMu.RLock()
		handler, exists := c.interactionHandlers[callbackID]
		c.handlerMu.RUnlock()
		if exists {
			if err := handler(c.ctx, cb); err != nil {
				c.logger.Error("interaction error", zap.Error(err))
			}
			return
		}
	}
	c.sendInteractionToChannel(cb)
}

func (c *Channel) sendInteractionToChannel(cb *InteractionCallback) {
	var vals []string
	for _, a := range cb.Actions {
		if a.Value != "" {
			vals = append(vals, a.Value)
		}
		if a.SelectedOption.Value != "" {
			vals = append(vals, a.SelectedOption.Value)
		}
		for _, o := range a.SelectedOptions {
			vals = append(vals, o.Value)
		}
	}
	content := cb.CallbackID
	if len(vals) > 0 {
		content = strings.Join(vals, ", ")
	}

	channelMsg := channel.Message{
		ID: cb.TriggerID, ChannelName: "slack", ChatID: cb.Channel.ID, UserID: cb.User.ID, Username: cb.User.Name,
		Type: channel.MessageTypeText, Content: content, Timestamp: time.Now(), IsGroup: true,
		Metadata: slackInteractionMetadata(cb),
	}
	select {
	case c.messages <- channelMsg:
	default:
	}
}

func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	if c.client == nil {
		return fmt.Errorf("client not initialized")
	}

	captionConsumed := false
	sentSomething := false
	hasBlocks := slackHasRenderableMetadata(msg.Metadata)

	for _, att := range msg.Attachments {
		caption := ""
		if !captionConsumed {
			caption = msg.Content
		}
		if len(att.Data) > 0 {
			name := att.Name
			if name == "" {
				name = "file"
			}
			if err := c.client.uploadFile(ctx, msg.ChatID, msg.ReplyToID, name, att.Data, caption); err != nil {
				c.logger.Warn("failed to upload file to slack, falling back to text",
					zap.String("type", string(att.Type)), zap.Error(err))
				fallback := buildAttachmentFallback(caption, att)
				if fallback == "" {
					return fmt.Errorf("failed to upload attachment %q: %w", name, err)
				}
				if _, err := c.sendMessage(ctx, msg.ChatID, fallback, msg.ReplyToID, nil); err != nil {
					return fmt.Errorf("failed to send attachment fallback: %w", err)
				}
				sentSomething = true
				if caption != "" {
					captionConsumed = true
				}
				continue
			}
			sentSomething = true
			if caption != "" {
				captionConsumed = true
			}
			continue
		}

		fallback := buildAttachmentFallback(caption, att)
		if fallback == "" {
			continue
		}
		if _, err := c.sendMessage(ctx, msg.ChatID, fallback, msg.ReplyToID, nil); err != nil {
			return fmt.Errorf("failed to send attachment link fallback: %w", err)
		}
		sentSomething = true
		if caption != "" {
			captionConsumed = true
		}
	}

	if !captionConsumed && (msg.Content != "" || hasBlocks) {
		_, err := c.sendMessage(ctx, msg.ChatID, msg.Content, msg.ReplyToID, msg.Metadata)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
		sentSomething = true
	}

	if !sentSomething {
		if len(msg.Attachments) > 0 {
			return fmt.Errorf("no sendable Slack attachments")
		}
		return fmt.Errorf("no sendable Slack content")
	}
	c.msgsSent.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastReplyAt = &now
	c.mu.Unlock()
	return nil
}

func (c *Channel) sendMessage(ctx context.Context, channelID, text, threadTS string, metadata map[string]interface{}) (string, error) {
	body := map[string]interface{}{"channel": channelID, "text": text}
	if threadTS != "" {
		body["thread_ts"] = threadTS
	}
	if blocks, ok := slackBlocks(metadata); ok {
		body["blocks"] = blocks
	}
	return c.client.postMessageWithBody(ctx, body)
}

func slackHasRenderableMetadata(metadata map[string]interface{}) bool {
	_, ok := slackBlocks(metadata)
	return ok
}

func slackBlocks(metadata map[string]interface{}) (interface{}, bool) {
	if metadata == nil {
		return nil, false
	}
	raw, ok := metadata["blocks"]
	if !ok || raw == nil {
		return nil, false
	}
	switch value := raw.(type) {
	case []interface{}:
		return value, true
	case []map[string]interface{}:
		return value, true
	case json.RawMessage:
		var decoded interface{}
		if err := json.Unmarshal(value, &decoded); err == nil {
			return decoded, true
		}
	case string:
		var decoded interface{}
		if err := json.Unmarshal([]byte(value), &decoded); err == nil {
			return decoded, true
		}
	}
	return nil, false
}

func buildAttachmentFallback(caption string, att channel.Attachment) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(caption) != "" {
		parts = append(parts, strings.TrimSpace(caption))
	}
	if strings.TrimSpace(att.URL) != "" {
		parts = append(parts, strings.TrimSpace(att.URL))
	} else if strings.TrimSpace(att.Name) != "" && len(parts) == 0 {
		parts = append(parts, strings.TrimSpace(att.Name))
	}
	return strings.Join(parts, "\n")
}

func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)
	if c.client == nil {
		return fmt.Errorf("client not initialized")
	}
	var full strings.Builder
	var sentTS string
	lastUpdate := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				if sentTS != "" && full.Len() > 0 {
					c.client.updateMessage(ctx, chatID, sentTS, full.String())
				}
				return nil
			}
			full.WriteString(chunk)
			if time.Since(lastUpdate) >= 500*time.Millisecond {
				if sentTS == "" {
					ts, err := c.client.postMessage(ctx, chatID, full.String(), replyToID)
					if err != nil {
						c.logger.Error("send streaming error", zap.Error(err))
						continue
					}
					sentTS = ts
				} else {
					c.client.updateMessage(ctx, chatID, sentTS, full.String())
				}
				lastUpdate = time.Now()
			}
		}
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
	done := make(chan struct{})
	go func() { c.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	close(c.messages)
	c.logger.Info("slack channel stopped")
	return nil
}

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	info := channel.Info{
		Name: "slack", Type: "slack", Status: c.status, Enabled: c.config.Enabled,
		ConnectedAt: c.connectedAt, LastError: c.lastError, LastErrorAt: c.lastErrorAt,
		MessageCount: c.msgCount.Load(), MessagesReceived: c.msgsReceived.Load(), MessagesSent: c.msgsSent.Load(),
		LastMessageAt: c.lastMessageAt, LastReplyAt: c.lastReplyAt, Metadata: make(map[string]interface{}),
	}
	if c.botUserID != "" {
		info.Metadata["bot_user_id"] = c.botUserID
	}
	return info
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

func (c *Channel) isChannelAllowed(ch string) bool {
	if len(c.config.AllowedChannels) == 0 {
		return true
	}
	for _, a := range c.config.AllowedChannels {
		if a == ch {
			return true
		}
	}
	return false
}

func (c *Channel) isUserAllowed(u string) bool {
	if len(c.config.AllowedUsers) == 0 {
		return true
	}
	for _, a := range c.config.AllowedUsers {
		if a == u {
			return true
		}
	}
	return false
}

func parseSlackTimestamp(ts string) time.Time {
	parts := strings.Split(ts, ".")
	if len(parts) != 2 {
		return time.Now()
	}
	sec, _ := strconv.ParseInt(parts[0], 10, 64)
	return time.Unix(sec, 0)
}

func (c *Channel) RegisterInteractionHandler(callbackID string, handler InteractionHandler) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.interactionHandlers[callbackID] = handler
}

func (c *Channel) RegisterBlockActionHandler(actionID string, handler InteractionHandler) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.blockActionHandlers[actionID] = handler
}

// RespondToInteraction sends a response via response URL.
func (c *Channel) RespondToInteraction(ctx context.Context, responseURL string, msg map[string]interface{}) error {
	data, _ := json.Marshal(msg)
	req, err := http.NewRequestWithContext(ctx, "POST", responseURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack respond: HTTP %d: %s", resp.StatusCode, b)
	}
	return nil
}

// WebhookHandler returns an HTTP handler for webhook events.
func (c *Channel) WebhookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		defer r.Body.Close()

		if c.config.SigningSecret != "" {
			ts := r.Header.Get("X-Slack-Request-Timestamp")
			sig := r.Header.Get("X-Slack-Signature")
			base := "v0:" + ts + ":" + string(body)
			mac := hmac.New(sha256.New, []byte(c.config.SigningSecret))
			mac.Write([]byte(base))
			expected := "v0=" + hex.EncodeToString(mac.Sum(nil))
			if !hmac.Equal([]byte(sig), []byte(expected)) {
				http.Error(w, "unauthorized", 401)
				return
			}
		}

		ct := r.Header.Get("Content-Type")
		if strings.Contains(ct, "application/x-www-form-urlencoded") {
			if err := r.ParseForm(); err == nil {
				if payload := r.FormValue("payload"); payload != "" {
					var cb InteractionCallback
					if json.Unmarshal([]byte(payload), &cb) == nil {
						c.handleInteractionCallback(&cb)
						w.WriteHeader(200)
						return
					}
				}
			}
		}

		var evt struct {
			Type      string          `json:"type"`
			Challenge string          `json:"challenge"`
			Event     json.RawMessage `json:"event"`
		}
		json.Unmarshal(body, &evt)
		if evt.Type == "url_verification" {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(evt.Challenge))
			return
		}
		if evt.Type == "event_callback" && len(evt.Event) > 0 {
			var inner struct {
				Type string `json:"type"`
			}
			json.Unmarshal(evt.Event, &inner)
			switch inner.Type {
			case "message":
				c.handleMessageEvent(evt.Event)
			case "app_mention":
				c.handleAppMentionEvent(evt.Event)
			}
		}
		w.WriteHeader(200)
	}
}
