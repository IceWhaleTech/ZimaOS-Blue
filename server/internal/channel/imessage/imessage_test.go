package imessage

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_New(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	ch := New(cfg, logger)

	if ch == nil {
		t.Fatal("Expected non-nil channel")
	}

	if ch.Name() != "imessage" {
		t.Errorf("Name() = %s, want imessage", ch.Name())
	}

	if ch.Type() != "imessage" {
		t.Errorf("Type() = %s, want imessage", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true

	ch := New(cfg, logger)
	info := ch.Info()

	if info.Name != "imessage" {
		t.Errorf("Info().Name = %s, want imessage", info.Name)
	}

	if info.Type != "imessage" {
		t.Errorf("Info().Type = %s, want imessage", info.Type)
	}

	if info.Status != channel.StatusDisconnected {
		t.Errorf("Info().Status = %s, want disconnected", info.Status)
	}

	if !info.Enabled {
		t.Error("Info().Enabled = false, want true")
	}
}

func TestChannel_IsConnected(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("IsConnected() = true, want false before start")
	}
}

func TestChannel_Messages(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	ch := New(cfg, logger)
	messages := ch.Messages()

	if messages == nil {
		t.Error("Messages() returned nil channel")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Enabled {
		t.Error("DefaultConfig().Enabled = true, want false")
	}

	if cfg.PollInterval != 1000 {
		t.Errorf("DefaultConfig().PollInterval = %d, want 1000", cfg.PollInterval)
	}

	if cfg.DatabasePath == "" {
		t.Error("DefaultConfig().DatabasePath is empty")
	}
}

func TestConvertMacOSTimestamp(t *testing.T) {
	// Test with a known timestamp
	// 2024-01-01 00:00:00 UTC in macOS timestamp format
	// Days from 2001-01-01 to 2024-01-01 = 8400 days
	// 8400 * 24 * 60 * 60 * 1e9 nanoseconds
	timestamp := int64(8400 * 24 * 60 * 60 * 1e9)
	result := convertMacOSTimestamp(timestamp)

	expected := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("convertMacOSTimestamp() = %v, want %v", result, expected)
	}
}

func TestNormalizePhoneNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"+1 (555) 123-4567", "15551234567"},
		{"555-123-4567", "5551234567"},
		{"+86 138 0000 0000", "8613800000000"},
		{"1234567890", "1234567890"},
	}

	for _, tt := range tests {
		result := normalizePhoneNumber(tt.input)
		if result != tt.expected {
			t.Errorf("normalizePhoneNumber(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestEscapeAppleScript(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "Hello World"},
		{"Hello \"World\"", "Hello \\\"World\\\""},
		{"Line1\nLine2", "Line1\\nLine2"},
		{"Tab\there", "Tab\\there"},
		{"Back\\slash", "Back\\\\slash"},
	}

	for _, tt := range tests {
		result := escapeAppleScript(tt.input)
		if result != tt.expected {
			t.Errorf("escapeAppleScript(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestIsSenderAllowed(t *testing.T) {
	logger := zap.NewNop()

	// Test with no restrictions
	cfg := DefaultConfig()
	ch := New(cfg, logger)

	if !ch.isSenderAllowed("+1234567890") {
		t.Error("isSenderAllowed() = false with no restrictions, want true")
	}

	// Test with phone number restrictions
	cfg.AllowedNumbers = []string{"+1 (555) 123-4567"}
	ch = New(cfg, logger)

	if !ch.isSenderAllowed("+15551234567") {
		t.Error("isSenderAllowed() = false for allowed number, want true")
	}

	if ch.isSenderAllowed("+19999999999") {
		t.Error("isSenderAllowed() = true for non-allowed number, want false")
	}

	// Test with email restrictions
	cfg = DefaultConfig()
	cfg.AllowedEmails = []string{"test@example.com"}
	ch = New(cfg, logger)

	if !ch.isSenderAllowed("test@example.com") {
		t.Error("isSenderAllowed() = false for allowed email, want true")
	}

	if !ch.isSenderAllowed("TEST@EXAMPLE.COM") {
		t.Error("isSenderAllowed() = false for allowed email (case insensitive), want true")
	}

	if ch.isSenderAllowed("other@example.com") {
		t.Error("isSenderAllowed() = true for non-allowed email, want false")
	}
}

func TestChannel_StopWithoutStart(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	ch := New(cfg, logger)

	// Stop without start should not error
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := ch.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() without Start() error = %v, want nil", err)
	}
}
