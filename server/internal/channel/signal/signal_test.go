package signal

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PhoneNumber = "+1234567890"

	ch := New(cfg, logger)

	if ch == nil {
		t.Fatal("Expected non-nil channel")
	}

	if ch.Name() != "signal" {
		t.Errorf("Name() = %s, want signal", ch.Name())
	}

	if ch.Type() != "signal" {
		t.Errorf("Type() = %s, want signal", ch.Type())
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Enabled {
		t.Error("Default config should have Enabled = false")
	}

	if cfg.ConfigPath != "./data/signal" {
		t.Errorf("ConfigPath = %s, want ./data/signal", cfg.ConfigPath)
	}

	if cfg.SignalCLIPath != "signal-cli" {
		t.Errorf("SignalCLIPath = %s, want signal-cli", cfg.SignalCLIPath)
	}

	if !cfg.UseJsonRpc {
		t.Error("UseJsonRpc should be true by default")
	}
}

func TestChannel_Info(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.PhoneNumber = "+1234567890"

	ch := New(cfg, logger)
	info := ch.Info()

	if info.Name != "signal" {
		t.Errorf("Info().Name = %s, want signal", info.Name)
	}

	if info.Type != "signal" {
		t.Errorf("Info().Type = %s, want signal", info.Type)
	}

	if !info.Enabled {
		t.Error("Info().Enabled should be true")
	}

	if info.Metadata["phone_number"] != "+1234567890" {
		t.Errorf("Info().Metadata[phone_number] = %v, want +1234567890", info.Metadata["phone_number"])
	}
}

func TestChannel_IsConnected(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("IsConnected() should be false before Start()")
	}
}

func TestChannel_Messages(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	ch := New(cfg, logger)
	messages := ch.Messages()

	if messages == nil {
		t.Error("Messages() should return non-nil channel")
	}
}

func TestChannel_isSenderAllowed(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		allowedNumbers []string
		sender         string
		want           bool
	}{
		{
			name:           "no restrictions allows all",
			allowedNumbers: []string{},
			sender:         "+1234567890",
			want:           true,
		},
		{
			name:           "allowed number",
			allowedNumbers: []string{"+1234567890"},
			sender:         "+1234567890",
			want:           true,
		},
		{
			name:           "allowed number with different format",
			allowedNumbers: []string{"+1 234 567 890"},
			sender:         "+1234567890",
			want:           true,
		},
		{
			name:           "not allowed number",
			allowedNumbers: []string{"+1111111111"},
			sender:         "+1234567890",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.AllowedNumbers = tt.allowedNumbers
			ch := New(cfg, logger)

			got := ch.isSenderAllowed(tt.sender)
			if got != tt.want {
				t.Errorf("isSenderAllowed(%s) = %v, want %v", tt.sender, got, tt.want)
			}
		})
	}
}

func TestNormalizePhoneNumber(t *testing.T) {
	tests := []struct {
		phone string
		want  string
	}{
		{"+1 234 567 890", "+1234567890"},
		{"(123) 456-7890", "(123)4567890"}, // Note: keeps + but removes spaces and dashes
		{"+1-234-567-890", "+1234567890"},
		{"+1234567890", "+1234567890"},
	}

	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			got := normalizePhoneNumber(tt.phone)
			if got != tt.want {
				t.Errorf("normalizePhoneNumber(%s) = %s, want %s", tt.phone, got, tt.want)
			}
		})
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Stop should not error when not started
	err := ch.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
}

func TestChannel_GetConfigPath(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.ConfigPath = "/custom/path"

	ch := New(cfg, logger)

	path := ch.GetConfigPath()
	if path != "/custom/path/data" {
		t.Errorf("GetConfigPath() = %s, want /custom/path/data", path)
	}
}

func TestSignalMessage_Parsing(t *testing.T) {
	// Test that the SignalMessage struct can be used
	msg := SignalMessage{}
	msg.Envelope.Source = "+1234567890"
	msg.Envelope.SourceNumber = "+1234567890"
	msg.Envelope.Timestamp = time.Now().UnixMilli()

	if msg.Envelope.Source != "+1234567890" {
		t.Errorf("Source = %s, want +1234567890", msg.Envelope.Source)
	}
}

func TestDataMessage_Struct(t *testing.T) {
	dm := DataMessage{
		Timestamp:        time.Now().UnixMilli(),
		Message:          "Hello, World!",
		ExpiresInSeconds: 3600,
	}

	if dm.Message != "Hello, World!" {
		t.Errorf("Message = %s, want Hello, World!", dm.Message)
	}
}

func TestGroupInfo_Struct(t *testing.T) {
	gi := GroupInfo{
		GroupID: "group123",
		Type:    "group",
		Name:    "Test Group",
	}

	if gi.GroupID != "group123" {
		t.Errorf("GroupID = %s, want group123", gi.GroupID)
	}

	if gi.Name != "Test Group" {
		t.Errorf("Name = %s, want Test Group", gi.Name)
	}
}
