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
}

func newMockChannel(name, channelType string, enabled bool) *mockChannel {
	return &mockChannel{
		name:        name,
		channelType: channelType,
		enabled:     enabled,
		messages:    make(chan Message, 10),
	}
}

func (m *mockChannel) Name() string {
	return m.name
}

func (m *mockChannel) Type() string {
	return m.channelType
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
