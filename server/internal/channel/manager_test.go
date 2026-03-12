package channel

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

// mockChannel is a mock implementation of the Channel interface for testing.
type mockChannel struct {
	name        string
	channelType string
	outbound    OutboundCapabilities
	enabled     bool
	connected   bool
	messages    chan Message
	mu          sync.RWMutex
	startErr    error
	stopErr     error
	sendErr     error
	respectCtx  bool
	msgCount    int64
	sent        []OutgoingMessage
	cleared     []string
}

func newMockChannel(name, channelType string, enabled bool) *mockChannel {
	return &mockChannel{
		name:        name,
		channelType: channelType,
		outbound:    defaultMockOutboundCapabilities(channelType),
		enabled:     enabled,
		messages:    make(chan Message, 10),
	}
}

func defaultMockOutboundCapabilities(channelType string) OutboundCapabilities {
	switch channelType {
	case "feishu":
		return OutboundCapabilities{
			MarkdownMode:              OutboundMarkdownModePreserveWhole,
			SupportsMarkdownFormat:    true,
			AutoPromoteMarkdownReport: true,
			SuppressHeartbeatText:     true,
		}
	case "telegram":
		return OutboundCapabilities{
			MarkdownMode:           OutboundMarkdownModeChunked,
			HumanizerPreset:        "telegram",
			SupportsMarkdownFormat: true,
		}
	case "matrix":
		return OutboundCapabilities{
			MarkdownMode:           OutboundMarkdownModeChunked,
			HumanizerPreset:        "matrix",
			SupportsMarkdownFormat: true,
		}
	case "teams", "wechat_work":
		return OutboundCapabilities{
			MarkdownMode:           OutboundMarkdownModeChunked,
			SupportsMarkdownFormat: true,
		}
	case "discord":
		return OutboundCapabilities{
			MarkdownMode:    OutboundMarkdownModeChunked,
			HumanizerPreset: "discord",
		}
	case "slack":
		return OutboundCapabilities{
			MarkdownMode:    OutboundMarkdownModeChunked,
			HumanizerPreset: "slack",
		}
	default:
		return DefaultOutboundCapabilities()
	}
}

func (m *mockChannel) Name() string {
	return m.name
}

func (m *mockChannel) Type() string {
	return m.channelType
}

func (m *mockChannel) OutboundCapabilities() OutboundCapabilities {
	return m.outbound
}

func (m *mockChannel) Start(ctx context.Context) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.mu.Lock()
	m.connected = true
	m.mu.Unlock()
	return nil
}

func (m *mockChannel) Stop(ctx context.Context) error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.mu.Lock()
	m.connected = false
	close(m.messages)
	m.mu.Unlock()
	return nil
}

func (m *mockChannel) Send(ctx context.Context, msg OutgoingMessage) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	if m.respectCtx {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	m.mu.Lock()
	m.msgCount++
	m.sent = append(m.sent, msg)
	m.mu.Unlock()
	return nil
}

func (m *mockChannel) ClearTypingReaction(_ context.Context, messageID string) {
	m.mu.Lock()
	m.cleared = append(m.cleared, messageID)
	m.mu.Unlock()
}

func (m *mockChannel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	go func() {
		for range content {
			// Consume content
		}
		close(done)
	}()
	return nil
}

func (m *mockChannel) Info() Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status := StatusDisconnected
	if m.connected {
		status = StatusConnected
	}
	return Info{
		Name:         m.name,
		Type:         m.channelType,
		Status:       status,
		Enabled:      m.enabled,
		MessageCount: m.msgCount,
	}
}

func (m *mockChannel) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

func (m *mockChannel) Messages() <-chan Message {
	return m.messages
}

func (m *mockChannel) simulateMessage(msg Message) {
	m.messages <- msg
}

func (m *mockChannel) lastSent() (OutgoingMessage, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.sent) == 0 {
		return OutgoingMessage{}, false
	}
	return m.sent[len(m.sent)-1], true
}

func (m *mockChannel) clearedReactions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, len(m.cleared))
	copy(out, m.cleared)
	return out
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func TestManager_Register(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)

	ch := newMockChannel("test", "mock", true)
	err := mgr.Register(ch)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Try to register the same channel again
	err = mgr.Register(ch)
	if err == nil {
		t.Fatal("expected error when registering duplicate channel")
	}
}

func TestManager_Unregister(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)

	ch := newMockChannel("test", "mock", true)
	mgr.Register(ch)

	err := mgr.Unregister("test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Try to unregister non-existent channel
	err = mgr.Unregister("nonexistent")
	if err == nil {
		t.Fatal("expected error when unregistering non-existent channel")
	}
}

func TestManager_Get(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)

	ch := newMockChannel("test", "mock", true)
	mgr.Register(ch)

	got, exists := mgr.Get("test")
	if !exists {
		t.Fatal("expected channel to exist")
	}
	if got.Name() != "test" {
		t.Fatalf("expected name 'test', got %s", got.Name())
	}

	_, exists = mgr.Get("nonexistent")
	if exists {
		t.Fatal("expected channel to not exist")
	}
}

func TestManager_List(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)

	ch1 := newMockChannel("test1", "mock", true)
	ch2 := newMockChannel("test2", "mock", false)
	mgr.Register(ch1)
	mgr.Register(ch2)

	infos := mgr.List()
	if len(infos) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(infos))
	}
}

func TestManager_StartStop(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)

	ch := newMockChannel("test", "mock", true)
	mgr.Register(ch)

	ctx := context.Background()
	err := mgr.Start(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Give time for goroutines to start
	time.Sleep(10 * time.Millisecond)

	if !ch.IsConnected() {
		t.Fatal("expected channel to be connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = mgr.Stop(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestManager_StartChannel(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)

	ch := newMockChannel("test", "mock", true)
	mgr.Register(ch)

	ctx := context.Background()
	err := mgr.StartChannel(ctx, "test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !ch.IsConnected() {
		t.Fatal("expected channel to be connected")
	}

	// Try to start already running channel
	err = mgr.StartChannel(ctx, "test")
	if err == nil {
		t.Fatal("expected error when starting already running channel")
	}

	// Try to start non-existent channel
	err = mgr.StartChannel(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error when starting non-existent channel")
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mgr.Stop(ctx)
}

func TestManager_StopChannel(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)

	ch := newMockChannel("test", "mock", true)
	mgr.Register(ch)

	ctx := context.Background()
	mgr.StartChannel(ctx, "test")

	err := mgr.StopChannel(ctx, "test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Try to stop already stopped channel
	ch2 := newMockChannel("test2", "mock", true)
	mgr.Register(ch2)
	err = mgr.StopChannel(ctx, "test2")
	if err == nil {
		t.Fatal("expected error when stopping non-running channel")
	}

	// Try to stop non-existent channel
	err = mgr.StopChannel(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error when stopping non-existent channel")
	}
}

func TestManager_Send(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)

	ch := newMockChannel("test", "mock", true)
	mgr.Register(ch)

	ctx := context.Background()
	mgr.StartChannel(ctx, "test")

	msg := OutgoingMessage{
		ChatID:  "123",
		Content: "Hello",
	}

	err := mgr.Send(ctx, "test", msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Try to send to non-existent channel
	err = mgr.Send(ctx, "nonexistent", msg)
	if err == nil {
		t.Fatal("expected error when sending to non-existent channel")
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mgr.Stop(ctx)
}

func TestManager_Send_FeishuReportAutoMarkdown(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("feishu", "feishu", true)
	mgr.Register(ch)

	ctx := context.Background()
	mgr.StartChannel(ctx, "feishu")

	report := "# 调研报告\n\n## 背景\n" +
		strings.Repeat("- 这是背景信息，包含上下文与范围说明。\n", 6) +
		"\n## 关键发现\n| 维度 | 结论 |\n| --- | --- |\n| 可靠性 | 高 |\n| 风险 | 中 |\n" +
		strings.Repeat("\n详细分析：该结论由多条证据共同支持。", 18)
	msg := OutgoingMessage{
		ChatID:  "oc_chat_1",
		Content: report,
	}

	if err := mgr.Send(ctx, "feishu", msg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	sent, ok := ch.lastSent()
	if !ok {
		t.Fatal("expected a sent message")
	}
	if sent.Format != "markdown" {
		t.Fatalf("expected format markdown, got %q", sent.Format)
	}
	if sent.Content != report {
		t.Fatalf("expected report markdown to stay unchanged, got %q", sent.Content)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mgr.Stop(stopCtx)
}

func TestManager_Send_FeishuShortTextDoesNotForceMarkdown(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("feishu", "feishu", true)
	mgr.Register(ch)

	ctx := context.Background()
	mgr.StartChannel(ctx, "feishu")

	msg := OutgoingMessage{
		ChatID:  "oc_chat_1",
		Content: "你好，帮我看下今天上海天气。",
	}
	if err := mgr.Send(ctx, "feishu", msg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	sent, ok := ch.lastSent()
	if !ok {
		t.Fatal("expected a sent message")
	}
	if sent.Format == "markdown" {
		t.Fatalf("expected short plain text to avoid markdown promotion, got %q", sent.Format)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mgr.Stop(stopCtx)
}

func TestPrepareOutgoingTextParts_FeishuMarkdownUsesSpecialPath(t *testing.T) {
	markdown := "# 周报\n\n- 第一项进展说明\n- 第二项进展说明\n- 第三项进展说明\n"
	expected := strings.TrimSpace(markdown)
	ch := newMockChannel("feishu", "feishu", true)
	parts, prepared := prepareOutgoingTextParts(resolveOutboundCapabilities(ch), OutgoingMessage{
		ChatID:  "oc_chat_1",
		Format:  "markdown",
		Content: markdown,
	}, 16, false)

	if prepared.Format != "markdown" {
		t.Fatalf("expected markdown format, got %q", prepared.Format)
	}
	if len(parts) != 1 {
		t.Fatalf("expected a single preserved markdown part, got %d", len(parts))
	}
	if parts[0] != expected {
		t.Fatalf("expected markdown to stay unchanged, got %q", parts[0])
	}
}

func TestChannelOutboundCapabilities_Feishu(t *testing.T) {
	capabilities := resolveOutboundCapabilities(newMockChannel("feishu", "feishu", true))
	if !capabilities.SupportsMarkdownFormat {
		t.Fatal("expected feishu to support markdown formatting")
	}
	if !capabilities.AutoPromoteMarkdownReport {
		t.Fatal("expected feishu to auto-promote markdown reports")
	}
	if !capabilities.SuppressHeartbeatText {
		t.Fatal("expected feishu to suppress heartbeat placeholder text")
	}
	if capabilities.MarkdownMode != OutboundMarkdownModePreserveWhole {
		t.Fatalf("expected feishu markdown mode preserve whole, got %v", capabilities.MarkdownMode)
	}
}

func TestChannelOutboundCapabilities_GenericMarkdownChannels(t *testing.T) {
	for _, channelType := range []string{"matrix", "teams", "telegram", "wechat_work"} {
		capabilities := resolveOutboundCapabilities(newMockChannel(channelType, channelType, true))
		if !capabilities.SupportsMarkdownFormat {
			t.Fatalf("expected %s to support markdown formatting", channelType)
		}
		if capabilities.AutoPromoteMarkdownReport {
			t.Fatalf("expected %s to keep report promotion disabled", channelType)
		}
		if capabilities.SuppressHeartbeatText {
			t.Fatalf("expected %s to keep heartbeat placeholder enabled", channelType)
		}
		if capabilities.MarkdownMode != OutboundMarkdownModeChunked {
			t.Fatalf("expected %s markdown mode chunked, got %v", channelType, capabilities.MarkdownMode)
		}
	}
}

func TestPrepareOutgoingTextParts_NonFeishuMarkdownUsesGenericSplit(t *testing.T) {
	markdown := "# 周报\n\n- 第一项进展说明\n- 第二项进展说明\n- 第三项进展说明\n"
	ch := newMockChannel("discord", "discord", true)
	parts, prepared := prepareOutgoingTextParts(resolveOutboundCapabilities(ch), OutgoingMessage{
		ChatID:  "oc_chat_1",
		Format:  "markdown",
		Content: markdown,
	}, 16, false)

	if prepared.Format != "markdown" {
		t.Fatalf("expected markdown format, got %q", prepared.Format)
	}
	if len(parts) <= 1 {
		t.Fatalf("expected generic markdown path to split content, got %d part(s)", len(parts))
	}
	if strings.Join(parts, "\n") == markdown {
		t.Fatalf("expected generic markdown path to chunk content, got %q", parts)
	}
}

func TestPrepareOutgoingTextParts_UsesCapabilityHumanizerPreset(t *testing.T) {
	ch := newMockChannel("custom", "custom", true)
	ch.outbound = OutboundCapabilities{
		MarkdownMode:    OutboundMarkdownModeChunked,
		HumanizerPreset: "telegram",
	}

	parts, prepared := prepareOutgoingTextParts(resolveOutboundCapabilities(ch), OutgoingMessage{
		ChatID:  "oc_chat_1",
		Content: "Hello **world**",
	}, 256, false)

	if prepared.Format != "html" {
		t.Fatalf("expected format html from capability preset, got %q", prepared.Format)
	}
	if len(parts) != 1 {
		t.Fatalf("expected one rendered part, got %d", len(parts))
	}
	if parts[0] != "Hello <b>world</b>" {
		t.Fatalf("expected telegram html rendering, got %q", parts[0])
	}
}

func TestManager_Send_DefaultHidesDetailedProcess(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("test", "mock", true)
	mgr.Register(ch)

	ctx := context.Background()
	mgr.StartChannel(ctx, "test")

	msg := OutgoingMessage{
		ChatID: "123",
		Content: "结论如下\n\n<!-- process-start -->\n```process\n[{\"tool\":\"exec\",\"cmd\":\"ls\"}]\n```\n" +
			"<!-- process-end -->",
	}
	if err := mgr.Send(ctx, "test", msg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	sent, ok := ch.lastSent()
	if !ok {
		t.Fatal("expected a sent message")
	}
	if sent.Content != "结论如下" {
		t.Fatalf("sent content = %q, want %q", sent.Content, "结论如下")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mgr.Stop(stopCtx)
}

func TestManager_Send_ShowDetailsKeepsProcessContent(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("test", "mock", true)
	mgr.Register(ch)

	ctx := context.Background()
	mgr.StartChannel(ctx, "test")

	msg := OutgoingMessage{
		ChatID: "123",
		Content: "结论如下\n\n<!-- process-start -->\n```process\n[{\"tool\":\"exec\",\"cmd\":\"ls\"}]\n```\n" +
			"<!-- process-end -->",
		Metadata: map[string]interface{}{"show_details": true},
	}
	if err := mgr.Send(ctx, "test", msg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	sent, ok := ch.lastSent()
	if !ok {
		t.Fatal("expected a sent message")
	}
	if !strings.Contains(sent.Content, `"tool":"exec"`) {
		t.Fatalf("expected detailed process content, got %q", sent.Content)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mgr.Stop(stopCtx)
}

func TestManager_Send_SplitsLongMessages(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MaxMessageLength = 12

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("test", "mock", true)
	mgr.Register(ch)

	ctx := context.Background()
	mgr.StartChannel(ctx, "test")

	msg := OutgoingMessage{
		ChatID:  "123",
		Content: "第一段内容。\n\n第二段内容。\n\n第三段内容。",
	}
	if err := mgr.Send(ctx, "test", msg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	ch.mu.RLock()
	defer ch.mu.RUnlock()
	if len(ch.sent) < 2 {
		t.Fatalf("expected split sends, got %d", len(ch.sent))
	}
}

func TestManager_ProcessMessage_AttachmentsOnlyStillSend(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("test", "mock", true)
	if err := mgr.Register(ch); err != nil {
		t.Fatalf("register channel: %v", err)
	}
	ctx := context.Background()
	if err := mgr.StartChannel(ctx, "test"); err != nil {
		t.Fatalf("start channel: %v", err)
	}

	mgr.SetHandler(func(ctx context.Context, msg Message) (*OutgoingMessage, error) {
		return &OutgoingMessage{
			ChatID: msg.ChatID,
			Attachments: []Attachment{{
				Type: MessageTypeImage,
				Name: "img.png",
				URL:  "https://example.com/img.png",
			}},
		}, nil
	})

	mgr.processMessage(ch, Message{ID: "m1", ChatID: "chat-1"})

	sent, ok := ch.lastSent()
	if !ok {
		t.Fatal("expected attachment-only response to be sent")
	}
	if len(sent.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(sent.Attachments))
	}
	if sent.Content != "" {
		t.Fatalf("expected empty content, got %q", sent.Content)
	}
}

func TestManager_Broadcast(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)

	ch1 := newMockChannel("test1", "mock", true)
	ch2 := newMockChannel("test2", "mock", true)
	mgr.Register(ch1)
	mgr.Register(ch2)

	ctx := context.Background()
	mgr.StartChannel(ctx, "test1")
	mgr.StartChannel(ctx, "test2")

	msg := OutgoingMessage{
		ChatID:  "123",
		Content: "Hello",
	}

	errors := mgr.Broadcast(ctx, msg)
	if len(errors) != 0 {
		t.Fatalf("expected no errors, got %v", errors)
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mgr.Stop(ctx)
}

func TestManager_MessageHandler(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.DefaultTimeoutSeconds = 5

	mgr := NewManager(cfg, logger)

	ch := newMockChannel("test", "mock", true)
	mgr.Register(ch)

	handlerCalled := make(chan struct{})
	mgr.SetHandler(func(ctx context.Context, msg Message) (*OutgoingMessage, error) {
		close(handlerCalled)
		return &OutgoingMessage{
			Content: "Response",
		}, nil
	})

	ctx := context.Background()
	mgr.StartChannel(ctx, "test")

	// Simulate incoming message
	ch.simulateMessage(Message{
		ID:      "msg1",
		ChatID:  "chat1",
		UserID:  "user1",
		Content: "Hello",
	})

	select {
	case <-handlerCalled:
		// Handler was called
	case <-time.After(2 * time.Second):
		t.Fatal("handler was not called")
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mgr.Stop(ctx)
}

func TestManager_MessageHandler_SupersededMessagesOnlyReplyLatest(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.DefaultTimeoutSeconds = 5

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("test", "mock", true)
	if err := mgr.Register(ch); err != nil {
		t.Fatalf("register channel: %v", err)
	}

	firstStarted := make(chan struct{})
	var firstOnce sync.Once
	handlerCalls := make(chan string, 4)
	mgr.SetHandler(func(ctx context.Context, msg Message) (*OutgoingMessage, error) {
		if msg.Content == "first" {
			firstOnce.Do(func() { close(firstStarted) })
			<-ctx.Done()
			handlerCalls <- "first"
			return nil, ctx.Err()
		}
		handlerCalls <- msg.Content
		return &OutgoingMessage{
			ChatID:  msg.ChatID,
			Content: "reply:" + msg.Content,
		}, nil
	})

	if err := mgr.StartChannel(context.Background(), "test"); err != nil {
		t.Fatalf("start channel: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mgr.Stop(stopCtx)
	}()

	ch.simulateMessage(Message{
		ID:      "msg-1",
		ChatID:  "chat-1",
		UserID:  "user-1",
		Content: "first",
	})

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first message handler did not start")
	}

	ch.simulateMessage(Message{
		ID:      "msg-2",
		ChatID:  "chat-1",
		UserID:  "user-1",
		Content: "second",
	})
	ch.simulateMessage(Message{
		ID:      "msg-3",
		ChatID:  "chat-1",
		UserID:  "user-1",
		Content: "third",
	})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ch.mu.RLock()
		sentCount := len(ch.sent)
		var last OutgoingMessage
		if sentCount > 0 {
			last = ch.sent[sentCount-1]
		}
		ch.mu.RUnlock()

		if sentCount == 1 {
			if last.Content != "reply:third" {
				t.Fatalf("latest sent content = %q, want %q", last.Content, "reply:third")
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	ch.mu.RLock()
	if len(ch.sent) != 1 {
		ch.mu.RUnlock()
		t.Fatalf("expected exactly one outbound reply, got %d", len(ch.sent))
	}
	ch.mu.RUnlock()

	collected := make([]string, 0, 3)
	timeout := time.After(2 * time.Second)
	for len(collected) < 2 {
		select {
		case call := <-handlerCalls:
			collected = append(collected, call)
		case <-timeout:
			t.Fatalf("timed out waiting for handler calls: %v", collected)
		}
	}
	if !containsString(collected, "first") {
		t.Fatalf("expected first message to be processed then canceled, got %v", collected)
	}
	if !containsString(collected, "third") {
		t.Fatalf("expected latest message to be processed, got %v", collected)
	}
	if containsString(collected, "second") {
		t.Fatalf("did not expect middle superseded message to be processed, got %v", collected)
	}
}

func TestManager_MessageHandler_SupersededMessagesClearPreviousReactions(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.DefaultTimeoutSeconds = 5

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("feishu", "feishu", true)
	if err := mgr.Register(ch); err != nil {
		t.Fatalf("register channel: %v", err)
	}

	firstStarted := make(chan struct{})
	var firstOnce sync.Once
	mgr.SetHandler(func(ctx context.Context, msg Message) (*OutgoingMessage, error) {
		if msg.Content == "first" {
			firstOnce.Do(func() { close(firstStarted) })
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return &OutgoingMessage{ChatID: msg.ChatID, Content: "reply:" + msg.Content}, nil
	})

	if err := mgr.StartChannel(context.Background(), "feishu"); err != nil {
		t.Fatalf("start channel: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mgr.Stop(stopCtx)
	}()

	ch.simulateMessage(Message{ID: "msg-1", ChatID: "chat-1", UserID: "user-1", Content: "first"})
	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first message handler did not start")
	}
	ch.simulateMessage(Message{ID: "msg-2", ChatID: "chat-1", UserID: "user-1", Content: "second"})
	ch.simulateMessage(Message{ID: "msg-3", ChatID: "chat-1", UserID: "user-1", Content: "third"})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ch.mu.RLock()
		sentCount := len(ch.sent)
		ch.mu.RUnlock()
		if sentCount == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	clearedDeadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(clearedDeadline) {
		cleared := ch.clearedReactions()
		if containsString(cleared, "msg-1") && containsString(cleared, "msg-2") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected cleared reactions for superseded messages, got %v", ch.clearedReactions())
}

func TestManager_MessageHandler_FeishuReportKeepsMarkdown(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.DefaultTimeoutSeconds = 5

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("feishu", "feishu", true)
	mgr.Register(ch)

	report := "# 调研结论\n\n## 摘要\n" +
		strings.Repeat("- 结论点。\n", 6) +
		"\n## 证据\n| 来源 | 说明 |\n| --- | --- |\n| A | 有效 |\n| B | 有效 |\n" +
		strings.Repeat("\n补充说明：这是完整调研报告正文。", 16)
	handlerCalled := make(chan struct{})
	mgr.SetHandler(func(ctx context.Context, msg Message) (*OutgoingMessage, error) {
		close(handlerCalled)
		return &OutgoingMessage{Content: report}, nil
	})

	ctx := context.Background()
	mgr.StartChannel(ctx, "feishu")

	ch.simulateMessage(Message{
		ID:      "msg_report_1",
		ChatID:  "oc_chat_report_1",
		UserID:  "ou_report_user_1",
		Content: "请给我完整调研报告",
	})

	select {
	case <-handlerCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("handler was not called")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		sent, ok := ch.lastSent()
		if ok {
			if sent.Format != "markdown" {
				t.Fatalf("expected markdown format, got %q", sent.Format)
			}
			if !strings.Contains(sent.Content, "## 摘要") {
				t.Fatalf("expected markdown heading to be preserved, got %q", sent.Content)
			}
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			mgr.Stop(stopCtx)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected report message to be sent")
}

func TestManager_DisabledChannels(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = false // Globally disabled

	mgr := NewManager(cfg, logger)

	ch := newMockChannel("test", "mock", true)
	mgr.Register(ch)

	ctx := context.Background()
	err := mgr.Start(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Channel should not be started when globally disabled
	if ch.IsConnected() {
		t.Fatal("expected channel to not be connected when globally disabled")
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc..."},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestManager_SendHeartbeats_SkipsFeishuPlaceholder(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Heartbeat.Enabled = true
	cfg.Heartbeat.InitialDelay = 1 * time.Millisecond
	cfg.Heartbeat.Emojis = []string{"💬"}

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("feishu", "feishu", true)

	done := make(chan struct{})
	mgr.sendHeartbeats(context.Background(), ch, "chat1", "msg1", done)

	if _, ok := ch.lastSent(); ok {
		t.Fatal("expected no heartbeat placeholder for feishu channel")
	}
}

func TestManager_ProcessTimeoutStillAllowsResponseSend(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.DefaultTimeoutSeconds = 1

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("test", "mock", true)
	ch.respectCtx = true
	if err := mgr.Register(ch); err != nil {
		t.Fatalf("register channel: %v", err)
	}

	mgr.SetHandler(func(ctx context.Context, msg Message) (*OutgoingMessage, error) {
		// Simulate a handler that runs longer than the processing deadline but still returns a response.
		time.Sleep(1100 * time.Millisecond)
		return &OutgoingMessage{Content: "late response"}, nil
	})

	if err := mgr.StartChannel(context.Background(), "test"); err != nil {
		t.Fatalf("start channel: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mgr.Stop(stopCtx)
	}()

	ch.simulateMessage(Message{
		ID:      "msg-timeout-send",
		ChatID:  "chat-timeout-send",
		UserID:  "user-timeout-send",
		Content: "hello",
	})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if sent, ok := ch.lastSent(); ok {
			if sent.Content != "late response" {
				t.Fatalf("unexpected sent content: %q", sent.Content)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("expected response to be sent with fresh send context after processing timeout")
}

func TestManager_ProcessMessage_GoogleChatRepliesUseThreadName(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("googlechat", "googlechat", true)
	if err := mgr.Register(ch); err != nil {
		t.Fatalf("register channel: %v", err)
	}
	ctx := context.Background()
	if err := mgr.StartChannel(ctx, "googlechat"); err != nil {
		t.Fatalf("start channel: %v", err)
	}

	mgr.SetHandler(func(ctx context.Context, msg Message) (*OutgoingMessage, error) {
		return &OutgoingMessage{ChatID: msg.ChatID, Content: "reply"}, nil
	})

	mgr.processMessage(ch, Message{
		ID:     "spaces/AAA/messages/1",
		ChatID: "spaces/AAA",
		Metadata: map[string]interface{}{
			"thread_name": "spaces/AAA/threads/thread-1",
		},
	})

	sent, ok := ch.lastSent()
	if !ok {
		t.Fatal("expected response to be sent")
	}
	if sent.ReplyToID != "spaces/AAA/threads/thread-1" {
		t.Fatalf("ReplyToID = %q, want thread name", sent.ReplyToID)
	}
}

func TestManager_ProcessMessage_LineRepliesUseReplyToken(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("line", "line", true)
	if err := mgr.Register(ch); err != nil {
		t.Fatalf("register channel: %v", err)
	}
	ctx := context.Background()
	if err := mgr.StartChannel(ctx, "line"); err != nil {
		t.Fatalf("start channel: %v", err)
	}

	mgr.SetHandler(func(ctx context.Context, msg Message) (*OutgoingMessage, error) {
		return &OutgoingMessage{ChatID: msg.ChatID, Content: "reply"}, nil
	})

	mgr.processMessage(ch, Message{
		ID:     "line-msg-1",
		ChatID: "user-1",
		Metadata: map[string]interface{}{
			"reply_token": "reply-token-1",
		},
	})

	sent, ok := ch.lastSent()
	if !ok {
		t.Fatal("expected response to be sent")
	}
	if sent.ReplyToID != "reply-token-1" {
		t.Fatalf("ReplyToID = %q, want reply token", sent.ReplyToID)
	}
}

func TestManager_Send_SplitKeepsThreadReplyForGoogleChat(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MaxMessageLength = 12

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("googlechat", "googlechat", true)
	mgr.Register(ch)

	ctx := context.Background()
	mgr.StartChannel(ctx, "googlechat")

	msg := OutgoingMessage{
		ChatID:    "spaces/AAA",
		ReplyToID: "spaces/AAA/threads/thread-1",
		Content:   "part one\n\npart two\n\npart three",
	}
	if err := mgr.Send(ctx, "googlechat", msg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	ch.mu.RLock()
	defer ch.mu.RUnlock()
	if len(ch.sent) < 2 {
		t.Fatalf("expected split sends, got %d", len(ch.sent))
	}
	for i, sent := range ch.sent {
		if sent.ReplyToID != "spaces/AAA/threads/thread-1" {
			t.Fatalf("part %d ReplyToID = %q, want thread preserved", i, sent.ReplyToID)
		}
	}
}

func TestManager_Send_SplitClearsSingleUseReplyForLine(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.MaxMessageLength = 12

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("line", "line", true)
	mgr.Register(ch)

	ctx := context.Background()
	mgr.StartChannel(ctx, "line")

	msg := OutgoingMessage{
		ChatID:    "user-1",
		ReplyToID: "reply-token-1",
		Content:   "part one\n\npart two\n\npart three",
	}
	if err := mgr.Send(ctx, "line", msg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	ch.mu.RLock()
	defer ch.mu.RUnlock()
	if len(ch.sent) < 2 {
		t.Fatalf("expected split sends, got %d", len(ch.sent))
	}
	if ch.sent[0].ReplyToID != "reply-token-1" {
		t.Fatalf("first ReplyToID = %q, want reply token", ch.sent[0].ReplyToID)
	}
	for i := 1; i < len(ch.sent); i++ {
		if ch.sent[i].ReplyToID != "" {
			t.Fatalf("part %d ReplyToID = %q, want cleared", i, ch.sent[i].ReplyToID)
		}
	}
}

func TestManager_ProcessMessage_SlackRepliesUseThreadTS(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("slack", "slack", true)
	if err := mgr.Register(ch); err != nil {
		t.Fatalf("register channel: %v", err)
	}
	ctx := context.Background()
	if err := mgr.StartChannel(ctx, "slack"); err != nil {
		t.Fatalf("start channel: %v", err)
	}

	mgr.SetHandler(func(ctx context.Context, msg Message) (*OutgoingMessage, error) {
		return &OutgoingMessage{ChatID: msg.ChatID, Content: "reply"}, nil
	})

	mgr.processMessage(ch, Message{
		ID:     "1710000000.002",
		ChatID: "C123",
		Metadata: map[string]interface{}{
			"thread_ts": "1710000000.001",
		},
	})

	sent, ok := ch.lastSent()
	if !ok {
		t.Fatal("expected response to be sent")
	}
	if sent.ReplyToID != "1710000000.001" {
		t.Fatalf("ReplyToID = %q, want thread ts", sent.ReplyToID)
	}
}

func TestManager_ProcessMessage_MattermostRepliesUseRootID(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("mattermost", "mattermost", true)
	if err := mgr.Register(ch); err != nil {
		t.Fatalf("register channel: %v", err)
	}
	ctx := context.Background()
	if err := mgr.StartChannel(ctx, "mattermost"); err != nil {
		t.Fatalf("start channel: %v", err)
	}

	mgr.SetHandler(func(ctx context.Context, msg Message) (*OutgoingMessage, error) {
		return &OutgoingMessage{ChatID: msg.ChatID, Content: "reply"}, nil
	})

	mgr.processMessage(ch, Message{
		ID:        "post-reply-1",
		ChatID:    "channel-1",
		ReplyToID: "root-post-1",
	})

	sent, ok := ch.lastSent()
	if !ok {
		t.Fatal("expected response to be sent")
	}
	if sent.ReplyToID != "root-post-1" {
		t.Fatalf("ReplyToID = %q, want root id", sent.ReplyToID)
	}
}

func TestManager_ProcessMessage_NextcloudTalkNonReplyableSkipsReplyTarget(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	mgr := NewManager(cfg, logger)
	ch := newMockChannel("nextcloudtalk", "nextcloudtalk", true)
	if err := mgr.Register(ch); err != nil {
		t.Fatalf("register channel: %v", err)
	}
	ctx := context.Background()
	if err := mgr.StartChannel(ctx, "nextcloudtalk"); err != nil {
		t.Fatalf("start channel: %v", err)
	}

	mgr.SetHandler(func(ctx context.Context, msg Message) (*OutgoingMessage, error) {
		return &OutgoingMessage{ChatID: msg.ChatID, Content: "reply"}, nil
	})

	mgr.processMessage(ch, Message{
		ID:     "123",
		ChatID: "room-1",
		Metadata: map[string]interface{}{
			"is_replyable": false,
		},
	})

	sent, ok := ch.lastSent()
	if !ok {
		t.Fatal("expected response to be sent")
	}
	if sent.ReplyToID != "" {
		t.Fatalf("ReplyToID = %q, want empty for non-replyable message", sent.ReplyToID)
	}
}

func TestManager_ProcessMessage_UnsupportedReplyChannelsSkipReplyTarget(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	channelTypes := []string{"messenger", "instagram", "twitter", "signal", "viber", "zalo", "wechat_work"}
	for _, channelType := range channelTypes {
		t.Run(channelType, func(t *testing.T) {
			mgr := NewManager(cfg, logger)
			ch := newMockChannel(channelType, channelType, true)
			if err := mgr.Register(ch); err != nil {
				t.Fatalf("register channel: %v", err)
			}
			ctx := context.Background()
			if err := mgr.StartChannel(ctx, channelType); err != nil {
				t.Fatalf("start channel: %v", err)
			}

			mgr.SetHandler(func(ctx context.Context, msg Message) (*OutgoingMessage, error) {
				return &OutgoingMessage{ChatID: msg.ChatID, Content: "reply"}, nil
			})

			mgr.processMessage(ch, Message{ID: "msg-1", ChatID: "chat-1"})

			sent, ok := ch.lastSent()
			if !ok {
				t.Fatal("expected response to be sent")
			}
			if sent.ReplyToID != "" {
				t.Fatalf("ReplyToID = %q, want empty", sent.ReplyToID)
			}
		})
	}
}

func TestDefaultReplyTarget_Matrix(t *testing.T) {
	tests := []struct {
		name        string
		channelType string
		msg         Message
		want        string
	}{
		{name: "googlechat thread", channelType: "googlechat", msg: Message{ID: "msg-1", Metadata: map[string]interface{}{"thread_name": "spaces/1/threads/2"}}, want: "spaces/1/threads/2"},
		{name: "line reply token", channelType: "line", msg: Message{ID: "msg-1", Metadata: map[string]interface{}{"reply_token": "reply-token-1"}}, want: "reply-token-1"},
		{name: "slack thread", channelType: "slack", msg: Message{ID: "msg-1", ReplyToID: "reply-1", Metadata: map[string]interface{}{"thread_ts": "1710000000.001"}}, want: "1710000000.001"},
		{name: "slack reply fallback", channelType: "slack", msg: Message{ID: "msg-1", ReplyToID: "reply-1"}, want: "reply-1"},
		{name: "mattermost reply", channelType: "mattermost", msg: Message{ID: "msg-1", ReplyToID: "post-1"}, want: "post-1"},
		{name: "nextcloudtalk non replyable", channelType: "nextcloudtalk", msg: Message{ID: "msg-1", Metadata: map[string]interface{}{"is_replyable": false}}, want: ""},
		{name: "nextcloudtalk replyable", channelType: "nextcloudtalk", msg: Message{ID: "msg-1"}, want: "msg-1"},
		{name: "unsupported messenger", channelType: "messenger", msg: Message{ID: "msg-1"}, want: ""},
		{name: "telegram callback origin", channelType: "telegram", msg: Message{ID: "cb-1", Metadata: map[string]interface{}{"origin_message_id": "123"}}, want: "123"},
		{name: "default fallback", channelType: "telegram", msg: Message{ID: "msg-1"}, want: "msg-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultReplyTarget(tt.channelType, tt.msg); got != tt.want {
				t.Fatalf("defaultReplyTarget(%q) = %q, want %q", tt.channelType, got, tt.want)
			}
		})
	}
}

func TestReplyTargetSingleUse_Matrix(t *testing.T) {
	tests := []struct {
		channelType string
		want        bool
	}{
		{channelType: "line", want: true},
		{channelType: "slack", want: false},
		{channelType: "googlechat", want: false},
		{channelType: "telegram", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.channelType, func(t *testing.T) {
			if got := replyTargetSingleUse(tt.channelType); got != tt.want {
				t.Fatalf("replyTargetSingleUse(%q) = %v, want %v", tt.channelType, got, tt.want)
			}
		})
	}
}
