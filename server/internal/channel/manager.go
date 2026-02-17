package channel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/humanizer"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
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
	m.mu.RUnlock()

	if handler == nil {
		m.logger.Warn("no message handler set, dropping message",
			zap.String("channel", ch.Name()),
			zap.String("message_id", msg.ID))
		return
	}

	ctx, cancel := context.WithTimeout(m.ctx, time.Duration(m.config.DefaultTimeoutSeconds)*time.Second)
	defer cancel()

	m.logger.Debug("processing message",
		zap.String("channel", ch.Name()),
		zap.String("message_id", msg.ID),
		zap.String("user_id", msg.UserID),
		zap.String("content", truncateString(msg.Content, 100)))

	response, err := handler(ctx, msg)
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
			ReplyToID: msg.ID,
			Content:   i18n.T(lang, i18n.MsgProcessingError, err),
		}
		if sendErr := ch.Send(ctx, errorResponse); sendErr != nil {
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
		// Set reply to the original message
		if response.ReplyToID == "" {
			response.ReplyToID = msg.ID
		}

		// Prepare response: strip AI tags and split if too long
		parts := PrepareResponse(response.Content, m.config.MaxMessageLength)
		if len(parts) == 0 {
			m.logger.Warn("response content is empty after processing",
				zap.String("channel", ch.Name()),
				zap.String("chat_id", response.ChatID))
			return
		}

		// Send each part
		for i, part := range parts {
			outMsg := OutgoingMessage{
				ChatID:      response.ChatID,
				Content:     part,
				Format:      response.Format,
				Attachments: response.Attachments,
				Metadata:    response.Metadata,
			}
			// Only set ReplyToID for the first message
			if i == 0 {
				outMsg.ReplyToID = response.ReplyToID
			}
			// Only include attachments in the last message
			if i < len(parts)-1 {
				outMsg.Attachments = nil
			}

			if err := ch.Send(ctx, outMsg); err != nil {
				m.logger.Error("error sending response",
					zap.String("channel", ch.Name()),
					zap.String("chat_id", outMsg.ChatID),
					zap.Int("part", i+1),
					zap.Int("total_parts", len(parts)),
					zap.Error(err))
				break
			}
		}
	}
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

	// Humanize content for IM channels (skip if explicitly markdown)
	if msg.Format != "markdown" && msg.Content != "" {
		msg.Content = humanizer.Humanize(msg.Content, humanizer.ModeIM)
	}

	return ch.Send(ctx, msg)
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
		if err := ch.Send(ctx, msg); err != nil {
			errors[ch.Name()] = err
		}
	}

	return errors
}

// truncateString truncates a string to the specified length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
