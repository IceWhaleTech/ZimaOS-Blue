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

func TestParseFeishuTimestamp(t *testing.T) {
	tests := []struct {
		input    string
		expected int64 // Unix milliseconds
	}{
		{"1609459200000", 1609459200000},
		{"1234567890123", 1234567890123},
		{"0", 0},
	}

	for _, tt := range tests {
		result := parseFeishuTimestamp(tt.input)
		if result.UnixMilli() != tt.expected {
			t.Errorf("parseFeishuTimestamp(%s) = %d, want %d", tt.input, result.UnixMilli(), tt.expected)
		}
	}
}
