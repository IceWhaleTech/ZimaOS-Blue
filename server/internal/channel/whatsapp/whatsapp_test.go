package whatsapp

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

	if ch.Name() != "whatsapp" {
		t.Errorf("Name() = %s, want whatsapp", ch.Name())
	}

	if ch.Type() != "whatsapp" {
		t.Errorf("Type() = %s, want whatsapp", ch.Type())
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Enabled {
		t.Error("Default config should have Enabled = false")
	}

	if cfg.SessionPath != "./data/whatsapp" {
		t.Errorf("SessionPath = %s, want ./data/whatsapp", cfg.SessionPath)
	}

	if cfg.QRTimeout != 60 {
		t.Errorf("QRTimeout = %d, want 60", cfg.QRTimeout)
	}

	if cfg.ReconnectDelay != 5 {
		t.Errorf("ReconnectDelay = %d, want 5", cfg.ReconnectDelay)
	}
}

func TestChannel_Info(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.PhoneNumber = "+1234567890"

	ch := New(cfg, logger)
	info := ch.Info()

	if info.Name != "whatsapp" {
		t.Errorf("Info().Name = %s, want whatsapp", info.Name)
	}

	if info.Type != "whatsapp" {
		t.Errorf("Info().Type = %s, want whatsapp", info.Type)
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
			allowedNumbers: []string{"1234567890"},
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

func TestExtractPhoneFromJID(t *testing.T) {
	tests := []struct {
		jid  string
		want string
	}{
		{"1234567890@s.whatsapp.net", "1234567890"},
		{"1234567890-1234567890@g.us", "1234567890"},
		{"1234567890", "1234567890"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.jid, func(t *testing.T) {
			got := extractPhoneFromJID(tt.jid)
			if got != tt.want {
				t.Errorf("extractPhoneFromJID(%s) = %s, want %s", tt.jid, got, tt.want)
			}
		})
	}
}

func TestToJID(t *testing.T) {
	tests := []struct {
		phone string
		want  string
	}{
		{"1234567890", "1234567890@s.whatsapp.net"},
		{"+1234567890", "1234567890@s.whatsapp.net"},
		{"1234567890@s.whatsapp.net", "1234567890@s.whatsapp.net"},
	}

	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			got := toJID(tt.phone)
			if got != tt.want {
				t.Errorf("toJID(%s) = %s, want %s", tt.phone, got, tt.want)
			}
		})
	}
}

func TestNormalizePhoneNumber(t *testing.T) {
	tests := []struct {
		phone string
		want  string
	}{
		{"+1 234 567 890", "1234567890"},
		{"(123) 456-7890", "1234567890"},
		{"+1-234-567-890", "1234567890"},
		{"1234567890", "1234567890"},
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

func TestChannel_GetQRCode(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	ch := New(cfg, logger)

	qr := ch.GetQRCode()
	if qr != "" {
		t.Errorf("GetQRCode() = %s, want empty string before login", qr)
	}
}
