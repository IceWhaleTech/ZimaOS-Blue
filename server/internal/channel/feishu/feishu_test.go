package feishu

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

func TestChannel_Name(t *testing.T) {
	cfg := channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Name() != "feishu" {
		t.Errorf("expected name 'feishu', got %s", ch.Name())
	}
}

func TestChannel_Type(t *testing.T) {
	cfg := channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Type() != "feishu" {
		t.Errorf("expected type 'feishu', got %s", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	cfg := channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	info := ch.Info()
	if info.Name != "feishu" {
		t.Errorf("expected name 'feishu', got %s", info.Name)
	}
	if info.Type != "feishu" {
		t.Errorf("expected type 'feishu', got %s", info.Type)
	}
	if info.Status != channel.StatusDisconnected {
		t.Errorf("expected status 'disconnected', got %s", info.Status)
	}
	if !info.Enabled {
		t.Error("expected enabled to be true")
	}
	if info.Metadata["app_id"] != "test-app-id" {
		t.Errorf("expected app_id 'test-app-id', got %v", info.Metadata["app_id"])
	}
}

func TestChannel_IsConnected(t *testing.T) {
	cfg := channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("expected channel to not be connected initially")
	}
}

func TestChannel_Messages(t *testing.T) {
	cfg := channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	messages := ch.Messages()
	if messages == nil {
		t.Error("expected messages channel to not be nil")
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	cfg := channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stopping a channel that was never started should not error
	err := ch.Stop(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestChannel_convertMessageType(t *testing.T) {
	cfg := channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	tests := []struct {
		input    string
		expected channel.MessageType
	}{
		{"text", channel.MessageTypeText},
		{"image", channel.MessageTypeImage},
		{"audio", channel.MessageTypeAudio},
		{"video", channel.MessageTypeVideo},
		{"media", channel.MessageTypeVideo},
		{"file", channel.MessageTypeFile},
		{"interactive", channel.MessageTypeCard},
		{"unknown", channel.MessageTypeText},
	}

	for _, tt := range tests {
		result := ch.convertMessageType(tt.input)
		if result != tt.expected {
			t.Errorf("convertMessageType(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestChannel_SafeSendMessage_AfterStop(t *testing.T) {
	cfg := channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	// Manually set status to connected to simulate a running channel
	ch.mu.Lock()
	ch.status = channel.StatusConnected
	ch.mu.Unlock()

	// Stop the channel (this closes the messages channel)
	ctx := context.Background()
	err := ch.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// This should NOT panic - safeSendMessage should handle closed channel gracefully
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("safeSendMessage panicked after Stop: %v", r)
		}
	}()

	msg := channel.Message{
		ID:          "test-msg-id",
		ChannelName: "feishu",
		Content:     "test message",
	}
	ch.safeSendMessage(msg, "test-msg-id")
}

func TestChannel_SafeSendMessage_WhenDisconnected(t *testing.T) {
	cfg := channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	// Channel is disconnected by default, safeSendMessage should drop the message
	msg := channel.Message{
		ID:          "test-msg-id",
		ChannelName: "feishu",
		Content:     "test message",
	}

	// This should not panic and should not block
	ch.safeSendMessage(msg, "test-msg-id")

	// Verify no message was sent (channel should be empty)
	select {
	case <-ch.Messages():
		t.Error("expected no message to be sent when disconnected")
	default:
		// Expected - no message sent
	}
}

func TestChannel_SafeSendMessage_ChannelFull(t *testing.T) {
	cfg := channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	// Set status to connected
	ch.mu.Lock()
	ch.status = channel.StatusConnected
	ch.mu.Unlock()

	// Fill the channel buffer (capacity is 100)
	for i := 0; i < 100; i++ {
		ch.messages <- channel.Message{ID: "fill-msg"}
	}

	// This should not block - safeSendMessage uses non-blocking send
	done := make(chan struct{})
	go func() {
		msg := channel.Message{
			ID:          "overflow-msg",
			ChannelName: "feishu",
			Content:     "overflow message",
		}
		ch.safeSendMessage(msg, "overflow-msg")
		close(done)
	}()

	select {
	case <-done:
		// Expected - safeSendMessage returned without blocking
	case <-time.After(1 * time.Second):
		t.Error("safeSendMessage blocked when channel was full")
	}
}

func TestChannel_Stop_DoubleStop(t *testing.T) {
	cfg := channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	// Manually set status to connected
	ch.mu.Lock()
	ch.status = channel.StatusConnected
	ch.mu.Unlock()

	ctx := context.Background()

	// First stop
	err := ch.Stop(ctx)
	if err != nil {
		t.Fatalf("First Stop() error = %v", err)
	}

	// Second stop should not panic (sync.Once protects close)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Double Stop() panicked: %v", r)
		}
	}()

	err = ch.Stop(ctx)
	if err != nil {
		t.Errorf("Second Stop() error = %v", err)
	}
}

func TestChannel_ConcurrentSendAndStop(t *testing.T) {
	cfg := channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	// Set status to connected
	ch.mu.Lock()
	ch.status = channel.StatusConnected
	ch.mu.Unlock()

	ctx := context.Background()

	// Start multiple goroutines sending messages
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				msg := channel.Message{
					ID:          "concurrent-msg",
					ChannelName: "feishu",
					Content:     "concurrent message",
				}
				ch.safeSendMessage(msg, "concurrent-msg")
			}
		}(i)
	}

	// Stop the channel while messages are being sent
	go func() {
		time.Sleep(10 * time.Millisecond)
		ch.Stop(ctx)
		close(done)
	}()

	// Wait for completion - should not panic
	select {
	case <-done:
		// Success - no panic occurred
	case <-time.After(5 * time.Second):
		t.Error("Test timed out")
	}
}
