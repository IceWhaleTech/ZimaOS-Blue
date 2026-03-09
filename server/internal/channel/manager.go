package channel

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/humanizer"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
)

const managerSendTimeoutFallback = 10 * time.Second

var (
	markdownHeadingLineRe      = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s+\S`)
	markdownTableDividerLineRe = regexp.MustCompile(`(?m)^\s*\|(?:\s*:?-{3,}:?\s*\|)+\s*$`)
	markdownFenceLineRe        = regexp.MustCompile("(?m)^```")
	markdownBulletLineRe       = regexp.MustCompile(`(?m)^\s{0,3}[-*+]\s+\S`)
	markdownOrderedLineRe      = regexp.MustCompile(`(?m)^\s{0,3}\d+\.\s+\S`)
)

// Manager manages all messaging channels.
type Manager struct {
	mu       sync.RWMutex
	channels map[string]Channel
	handler  MessageHandler
	logger   *zap.Logger
	config   Config

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// warmupFunc is called with a conversation ID to pre-compute context.
	// Set by the chat handler via SetWarmupFunc.
	warmupFunc func(convID string)

	// onStopHooks are called when the manager stops, to persist stats
	onStopHooks []func()
}

// NewManager creates a new channel manager.
func NewManager(cfg Config, logger *zap.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		channels:    make(map[string]Channel),
		logger:      logger,
		config:      cfg,
		ctx:         ctx,
		cancel:      cancel,
		onStopHooks: make([]func(), 0),
	}
}

// OnStop registers a callback to be called when the manager stops.
func (m *Manager) OnStop(hook func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStopHooks = append(m.onStopHooks, hook)
}

// SetHandler sets the message handler for incoming messages.
func (m *Manager) SetHandler(handler MessageHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handler = handler
}

// SetWarmupFunc sets the function called to pre-compute context when a channel message arrives.
func (m *Manager) SetWarmupFunc(fn func(convID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warmupFunc = fn
}

// Register registers a channel with the manager.
func (m *Manager) Register(ch Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := ch.Name()
	if _, exists := m.channels[name]; exists {
		return fmt.Errorf("channel %s already registered", name)
	}

	m.channels[name] = ch
	m.logger.Info("channel registered", zap.String("name", name), zap.String("type", ch.Type()))
	return nil
}

// Unregister removes a channel from the manager.
func (m *Manager) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, exists := m.channels[name]
	if !exists {
		return fmt.Errorf("channel %s not found", name)
	}

	// Stop the channel if it's running
	if ch.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := ch.Stop(ctx); err != nil {
			m.logger.Warn("error stopping channel during unregister",
				zap.String("name", name),
				zap.Error(err))
		}
	}

	delete(m.channels, name)
	m.logger.Info("channel unregistered", zap.String("name", name))
	return nil
}

// Get returns a channel by name.
func (m *Manager) Get(name string) (Channel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ch, exists := m.channels[name]
	return ch, exists
}

// List returns information about all registered channels.
func (m *Manager) List() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]Info, 0, len(m.channels))
	for _, ch := range m.channels {
		infos = append(infos, ch.Info())
	}
	return infos
}

// Start starts all enabled channels.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.Enabled {
		m.logger.Info("channels are disabled globally")
		return nil
	}

	for name, ch := range m.channels {
		info := ch.Info()
		if !info.Enabled {
			m.logger.Debug("skipping disabled channel", zap.String("name", name))
			continue
		}

		if err := m.startChannel(ctx, ch); err != nil {
			m.logger.Error("failed to start channel",
				zap.String("name", name),
				zap.Error(err))
			continue
		}

		// Start message handler goroutine for this channel
		m.wg.Add(1)
		go m.handleMessages(ch)
	}

	return nil
}

// startChannel starts a single channel.
func (m *Manager) startChannel(ctx context.Context, ch Channel) error {
	m.logger.Info("starting channel", zap.String("name", ch.Name()))

	if err := ch.Start(ctx); err != nil {
		return fmt.Errorf("failed to start channel %s: %w", ch.Name(), err)
	}

	m.logger.Info("channel started", zap.String("name", ch.Name()))
	return nil
}

// handleMessages handles incoming messages from a channel.
func (m *Manager) handleMessages(ch Channel) {
	defer m.wg.Done()

	name := ch.Name()
	messages := ch.Messages()

	for {
		select {
		case <-m.ctx.Done():
			return
		case msg, ok := <-messages:
			if !ok {
				m.logger.Debug("message channel closed", zap.String("channel", name))
				return
			}

			m.processMessage(ch, msg)
		}
	}
}

// processMessage processes a single incoming message.
func (m *Manager) processMessage(ch Channel, msg Message) {
	m.mu.RLock()
	handler := m.handler
	warmupFn := m.warmupFunc
	m.mu.RUnlock()

	if handler == nil {
		m.logger.Warn("no message handler set, dropping message",
			zap.String("channel", ch.Name()),
			zap.String("message_id", msg.ID))
		return
	}

	processCtx, cancel := context.WithTimeout(m.ctx, time.Duration(m.config.DefaultTimeoutSeconds)*time.Second)
	defer cancel()

	m.logger.Debug("processing message",
		zap.String("channel", ch.Name()),
		zap.String("message_id", msg.ID),
		zap.String("user_id", msg.UserID),
		zap.String("content", truncateString(msg.Content, 100)))

	// Send typing indicator immediately so the user sees the bot is working
	if ti, ok := ch.(TypingIndicator); ok {
		if err := ti.SendTyping(processCtx, msg.ChatID); err != nil {
			m.logger.Debug("failed to send typing indicator",
				zap.String("channel", ch.Name()),
				zap.String("chat_id", msg.ChatID),
				zap.Error(err))
		}
	}

	// Fire warmup in background to pre-compute system prompt + history
	if warmupFn != nil {
		convID := channelConversationID(ch.Name(), msg.ChatID)
		go warmupFn(convID)
	}

	// Periodically re-send typing indicator while the handler is running
	typingDone := make(chan struct{})
	if ti, ok := ch.(TypingIndicator); ok {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-typingDone:
					return
				case <-processCtx.Done():
					return
				case <-ticker.C:
					_ = ti.SendTyping(processCtx, msg.ChatID)
				}
			}
		}()
	}

	// Send periodic heartbeat messages so the user knows the bot is still alive
	heartbeatDone := make(chan struct{})
	go m.sendHeartbeats(processCtx, ch, msg.ChatID, msg.ID, heartbeatDone)

	response, err := handler(processCtx, msg)
	close(typingDone)
	close(heartbeatDone)

	if err != nil {
		m.logger.Error("error handling message",
			zap.String("channel", ch.Name()),
			zap.String("message_id", msg.ID),
			zap.Error(err))

		// Send error message back to the channel user
		// Get language from message metadata, default to English
		lang := i18n.DefaultLanguage
		if msg.Metadata != nil {
			if langStr, ok := msg.Metadata["language"].(string); ok {
				lang = i18n.ParseLanguage(langStr)
			}
		}
		errorResponse := OutgoingMessage{
			ChatID:    msg.ChatID,
			ReplyToID: defaultReplyTarget(ch.Type(), msg),
			Content:   i18n.T(lang, i18n.MsgProcessingError, err),
		}
		if sendErr := m.sendWithTimeout(ch, errorResponse); sendErr != nil {
			m.logger.Error("error sending error response",
				zap.String("channel", ch.Name()),
				zap.String("chat_id", msg.ChatID),
				zap.Error(sendErr))
		}
		return
	}

	if response != nil {
		// Set the chat ID if not specified
		if response.ChatID == "" {
			response.ChatID = msg.ChatID
		}
		// Set reply to the original message or channel-specific thread target.
		if response.ReplyToID == "" {
			response.ReplyToID = defaultReplyTarget(ch.Type(), msg)
		}
		parts, prepared := prepareOutgoingTextParts(ch.Type(), *response, m.config.MaxMessageLength, true)
		if len(parts) == 0 && len(prepared.Attachments) == 0 {
			m.logger.Warn("response content is empty after processing",
				zap.String("channel", ch.Name()),
				zap.String("chat_id", prepared.ChatID))
			return
		}

		outbound := buildOutgoingMessages(ch.Type(), prepared, parts)
		if len(outbound) == 0 {
			m.logger.Warn("response content is empty after preparation",
				zap.String("channel", ch.Name()),
				zap.String("chat_id", prepared.ChatID))
			return
		}

		for i, outMsg := range outbound {
			if err := m.sendWithTimeout(ch, outMsg); err != nil {
				m.logger.Error("error sending response",
					zap.String("channel", ch.Name()),
					zap.String("chat_id", outMsg.ChatID),
					zap.Int("part", i+1),
					zap.Int("total_parts", len(outbound)),
					zap.Error(err))
				break
			}
		}
	}
}

func (m *Manager) sendWithTimeout(ch Channel, msg OutgoingMessage) error {
	sendCtx, cancel := context.WithTimeout(m.ctx, m.responseSendTimeout())
	defer cancel()
	return ch.Send(sendCtx, msg)
}

func (m *Manager) responseSendTimeout() time.Duration {
	if m.config.DefaultTimeoutSeconds <= 0 {
		return managerSendTimeoutFallback
	}
	return time.Duration(m.config.DefaultTimeoutSeconds) * time.Second
}

// Stop stops all channels.
func (m *Manager) Stop(ctx context.Context) error {
	m.cancel()

	m.mu.RLock()
	channels := make([]Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		channels = append(channels, ch)
	}
	m.mu.RUnlock()

	var lastErr error
	for _, ch := range channels {
		if ch.IsConnected() {
			m.logger.Info("stopping channel", zap.String("name", ch.Name()))
			if err := ch.Stop(ctx); err != nil {
				m.logger.Error("error stopping channel",
					zap.String("name", ch.Name()),
					zap.Error(err))
				lastErr = err
			}
		}
	}

	// Wait for all message handlers to finish
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("all channel handlers stopped")
	case <-ctx.Done():
		m.logger.Warn("timeout waiting for channel handlers to stop")
		return ctx.Err()
	}

	// Run onStop hooks to persist stats
	m.mu.RLock()
	hooks := make([]func(), len(m.onStopHooks))
	copy(hooks, m.onStopHooks)
	m.mu.RUnlock()
	for _, hook := range hooks {
		hook()
	}

	return lastErr
}

// StartChannel starts a specific channel by name.
func (m *Manager) StartChannel(_ context.Context, name string) error {
	m.mu.RLock()
	ch, exists := m.channels[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("channel %s not found", name)
	}

	if ch.IsConnected() {
		return fmt.Errorf("channel %s is already running", name)
	}

	// Use the manager's long-lived context, not the HTTP request context,
	// so the channel stays alive after the API call returns.
	if err := m.startChannel(m.ctx, ch); err != nil {
		return err
	}

	// Start message handler goroutine
	m.wg.Add(1)
	go m.handleMessages(ch)

	return nil
}

// StopChannel stops a specific channel by name.
func (m *Manager) StopChannel(ctx context.Context, name string) error {
	m.mu.RLock()
	ch, exists := m.channels[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("channel %s not found", name)
	}

	if !ch.IsConnected() {
		return fmt.Errorf("channel %s is not running", name)
	}

	m.logger.Info("stopping channel", zap.String("name", name))
	return ch.Stop(ctx)
}

// Send sends a message through a specific channel.
// Content is automatically humanized for IM readability unless Format is "markdown".
func (m *Manager) Send(ctx context.Context, channelName string, msg OutgoingMessage) error {
	m.mu.RLock()
	ch, exists := m.channels[channelName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("channel %s not found", channelName)
	}

	if !ch.IsConnected() {
		return fmt.Errorf("channel %s is not connected", channelName)
	}

	parts, prepared := prepareOutgoingTextParts(ch.Type(), msg, m.config.MaxMessageLength, false)
	return sendPreparedMessages(ctx, ch, buildOutgoingMessages(ch.Type(), prepared, parts))
}

// Broadcast sends a message to all connected channels.
func (m *Manager) Broadcast(ctx context.Context, msg OutgoingMessage) map[string]error {
	m.mu.RLock()
	channels := make([]Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		if ch.IsConnected() {
			channels = append(channels, ch)
		}
	}
	m.mu.RUnlock()

	errors := make(map[string]error)
	for _, ch := range channels {
		parts, prepared := prepareOutgoingTextParts(ch.Type(), msg, m.config.MaxMessageLength, false)
		if err := sendPreparedMessages(ctx, ch, buildOutgoingMessages(ch.Type(), prepared, parts)); err != nil {
			errors[ch.Name()] = err
		}
	}

	return errors
}

// sendHeartbeats sends a single emoji message to the channel while the handler
// is processing, so the user knows the bot is still alive. Stops when done is closed.
func (m *Manager) sendHeartbeats(ctx context.Context, ch Channel, chatID, replyToID string, done <-chan struct{}) {
	cfg := m.config.Heartbeat
	if !cfg.Enabled || len(cfg.Emojis) == 0 {
		return
	}
	// Feishu now uses reaction-based pending indicator on inbound message,
	// so we suppress extra heartbeat text like "💬..." to avoid noisy placeholders.
	if ch.Type() == "feishu" {
		return
	}

	// Wait initial delay before sending heartbeat
	select {
	case <-done:
		return
	case <-ctx.Done():
		return
	case <-time.After(cfg.InitialDelay):
	}

	// Send a single heartbeat message with ellipsis
	emoji := cfg.Emojis[0] + "..."
	if err := ch.Send(ctx, OutgoingMessage{
		ChatID:    chatID,
		ReplyToID: replyToID,
		Content:   emoji,
	}); err != nil {
		m.logger.Debug("failed to send heartbeat message",
			zap.String("channel", ch.Name()),
			zap.String("chat_id", chatID),
			zap.Error(err))
	}
}

// truncateString truncates a string to the specified length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// channelConversationID builds a stable conversation ID for IM channels.
// Must match the logic in server/chat.go channelConversationID.
func channelConversationID(channelName, chatID string) string {
	if chatID != "" {
		return "ch:" + channelName + ":" + chatID
	}
	return "ch:" + channelName
}

func maybePromoteMarkdownReport(channelType string, msg *OutgoingMessage) {
	if msg == nil {
		return
	}
	if !shouldPromoteMarkdownReport(channelType, *msg) {
		return
	}
	msg.Format = "markdown"
}

func shouldPromoteMarkdownReport(channelType string, msg OutgoingMessage) bool {
	// Feishu supports interactive cards; preserve report markdown instead of compacting to plain text.
	if channelType != "feishu" {
		return false
	}
	format := strings.ToLower(strings.TrimSpace(msg.Format))
	switch format {
	case "", "text", "plain", "plain_text":
	default:
		return false
	}
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return false
	}
	runeCount := utf8.RuneCountInString(content)
	if runeCount < 240 || !strings.Contains(content, "\n") {
		return false
	}

	// Strong markdown signals: fenced code or table divider rows.
	if markdownFenceLineRe.MatchString(content) || markdownTableDividerLineRe.MatchString(content) {
		return true
	}

	// Report-like long structured markdown: multi-section headings.
	headingCount := len(markdownHeadingLineRe.FindAllString(content, -1))
	if headingCount >= 2 && runeCount >= 360 {
		return true
	}

	// Fallback for long structured lists (common in research summaries).
	bulletCount := len(markdownBulletLineRe.FindAllString(content, -1))
	orderedCount := len(markdownOrderedLineRe.FindAllString(content, -1))
	return runeCount >= 800 && (bulletCount >= 4 || orderedCount >= 4 || bulletCount+orderedCount >= 5)
}

func shouldShowDetails(msg OutgoingMessage) bool {
	if msg.Metadata == nil {
		return false
	}
	v, ok := msg.Metadata["show_details"]
	if !ok {
		return false
	}
	show, ok := v.(bool)
	return ok && show
}

func defaultReplyTarget(channelType string, msg Message) string {
	switch channelType {
	case "googlechat":
		if msg.Metadata != nil {
			if threadName, ok := msg.Metadata["thread_name"].(string); ok && strings.TrimSpace(threadName) != "" {
				return threadName
			}
		}
		return ""
	case "line":
		if msg.Metadata != nil {
			if replyToken, ok := msg.Metadata["reply_token"].(string); ok && strings.TrimSpace(replyToken) != "" {
				return replyToken
			}
		}
		return ""
	case "slack":
		if msg.Metadata != nil {
			if threadTS, ok := msg.Metadata["thread_ts"].(string); ok && strings.TrimSpace(threadTS) != "" {
				return threadTS
			}
		}
		if strings.TrimSpace(msg.ReplyToID) != "" {
			return msg.ReplyToID
		}
		return msg.ID
	case "telegram":
		if msg.Metadata != nil {
			if originMessageID, ok := msg.Metadata["origin_message_id"].(string); ok && strings.TrimSpace(originMessageID) != "" {
				return originMessageID
			}
		}
		return msg.ID
	case "mattermost":
		if strings.TrimSpace(msg.ReplyToID) != "" {
			return msg.ReplyToID
		}
		return msg.ID
	case "nextcloudtalk":
		if msg.Metadata != nil {
			if replyable, ok := msg.Metadata["is_replyable"].(bool); ok && !replyable {
				return ""
			}
		}
		return msg.ID
	case "messenger", "instagram", "twitter", "signal", "viber", "zalo", "wechat_work":
		return ""
	default:
		return msg.ID
	}
}

func prepareOutgoingTextParts(channelType string, msg OutgoingMessage, maxLength int, stripAITags bool) ([]string, OutgoingMessage) {
	maybePromoteMarkdownReport(channelType, &msg)
	if msg.Content == "" {
		return nil, msg
	}

	content := msg.Content
	if stripAITags {
		content = StripAITags(content)
	} else {
		content = strings.TrimSpace(content)
	}
	if content == "" {
		msg.Content = ""
		return nil, msg
	}

	format := strings.ToLower(strings.TrimSpace(msg.Format))
	switch format {
	case "markdown", "md", "markdownv2":
		msg.Content = content
		msg.Format = "markdown"
		if channelType == "feishu" {
			return []string{content}, msg
		}
		return splitMarkdownAware(content, maxLength), msg
	case "html":
		msg.Content = content
		return SplitMessage(content, maxLength), msg
	}

	if !shouldShowDetails(msg) {
		content = humanizer.CompactForIM(content)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		msg.Content = ""
		return nil, msg
	}

	sourceParts := splitSourceContent(content, maxLength)
	if len(sourceParts) == 0 {
		msg.Content = ""
		return nil, msg
	}

	renderedParts := make([]string, 0, len(sourceParts))
	resolvedFormat := ""
	for _, part := range sourceParts {
		rendered, recommendedFormat := humanizer.HumanizeForChannel(part, channelType)
		rendered = strings.TrimSpace(rendered)
		if rendered == "" {
			continue
		}
		renderedParts = append(renderedParts, rendered)
		if resolvedFormat == "" {
			resolvedFormat = recommendedFormat
		}
	}

	msg.Content = content
	if isPlainLikeFormat(msg.Format) {
		msg.Format = resolvedFormat
	}
	return renderedParts, msg
}

func buildOutgoingMessages(channelType string, msg OutgoingMessage, parts []string) []OutgoingMessage {
	if len(parts) == 0 {
		if msg.Content == "" && len(msg.Attachments) == 0 {
			return nil
		}
		return []OutgoingMessage{msg}
	}

	outbound := make([]OutgoingMessage, 0, len(parts))
	for i, part := range parts {
		outMsg := msg
		outMsg.Content = part
		if i > 0 && replyTargetSingleUse(channelType) {
			outMsg.ReplyToID = ""
		}
		if i < len(parts)-1 {
			outMsg.Attachments = nil
		}
		outbound = append(outbound, outMsg)
	}
	return outbound
}

func replyTargetSingleUse(channelType string) bool {
	switch channelType {
	case "line":
		return true
	default:
		return false
	}
}

func sendPreparedMessages(ctx context.Context, ch Channel, msgs []OutgoingMessage) error {
	for _, msg := range msgs {
		if err := ch.Send(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

func splitSourceContent(content string, maxLength int) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	if looksLikeMarkdown(content) {
		return splitMarkdownAware(content, maxLength)
	}
	return SplitMessage(content, maxLength)
}

func splitMarkdownAware(content string, maxLength int) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	parts := humanizer.ChunkMarkdownTextWithMode(content, markdownByteLimit(content, maxLength), humanizer.ChunkNewline)
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return cleaned
}

func looksLikeMarkdown(content string) bool {
	return markdownFenceLineRe.MatchString(content) ||
		markdownTableDividerLineRe.MatchString(content) ||
		markdownHeadingLineRe.MatchString(content) ||
		markdownBulletLineRe.MatchString(content) ||
		markdownOrderedLineRe.MatchString(content)
}

func markdownByteLimit(content string, maxLength int) int {
	if maxLength <= 0 {
		return 4096
	}
	runeCount := 0
	for idx := range content {
		if runeCount == maxLength {
			return idx
		}
		runeCount++
	}
	return len(content)
}

func isPlainLikeFormat(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text", "plain", "plain_text":
		return true
	default:
		return false
	}
}
