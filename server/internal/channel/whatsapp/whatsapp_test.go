package whatsapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.PhoneNumber = "+1807890"

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
	cfg.PhoneNumber = "+1807890"

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

	if info.Metadata["phone_number"] != "+1807890" {
		t.Errorf("Info().Metadata[phone_number] = %v, want +1807890", info.Metadata["phone_number"])
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
			sender:         "+1807890",
			want:           true,
		},
		{
			name:           "allowed number",
			allowedNumbers: []string{"+1807890"},
			sender:         "+1807890",
			want:           true,
		},
		{
			name:           "allowed number with different format",
			allowedNumbers: []string{"1807890"},
			sender:         "+1807890",
			want:           true,
		},
		{
			name:           "not allowed number",
			allowedNumbers: []string{"+1111111111"},
			sender:         "+1807890",
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
		{"1807890@s.whatsapp.net", "1807890"},
		{"1807890-1807890@g.us", "1807890"},
		{"1807890", "1807890"},
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
		{"1807890", "1807890@s.whatsapp.net"},
		{"+1807890", "1807890@s.whatsapp.net"},
		{"1807890@s.whatsapp.net", "1807890@s.whatsapp.net"},
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
		{"1807890", "1807890"},
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

func TestChannel_Send_UsesTransportAndUpdatesStats(t *testing.T) {
	logger := zap.NewNop()
	ch := New(DefaultConfig(), logger)
	ch.mu.Lock()
	ch.isLoggedIn = true
	ch.status = channel.StatusConnected
	ch.mu.Unlock()

	var gotJID, gotText, gotReply string
	ch.sendTextFunc = func(ctx context.Context, jid, text, replyToID string) error {
		gotJID, gotText, gotReply = jid, text, replyToID
		return nil
	}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:    "+1807890",
		ReplyToID: "msg-1",
		Content:   "hello whatsapp",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if gotJID != "1807890@s.whatsapp.net" {
		t.Fatalf("unexpected jid %q", gotJID)
	}
	if gotText != "hello whatsapp" {
		t.Fatalf("unexpected text %q", gotText)
	}
	if gotReply != "msg-1" {
		t.Fatalf("unexpected reply id %q", gotReply)
	}
	info := ch.Info()
	if info.MessagesSent != 1 {
		t.Fatalf("expected MessagesSent=1, got %d", info.MessagesSent)
	}
	if info.LastReplyAt == nil {
		t.Fatal("expected LastReplyAt to be set")
	}
}

func TestChannel_SendWithAttachment_URLFallbackToText(t *testing.T) {
	logger := zap.NewNop()
	ch := New(DefaultConfig(), logger)
	ch.mu.Lock()
	ch.isLoggedIn = true
	ch.status = channel.StatusConnected
	ch.mu.Unlock()

	var sent []string
	ch.sendTextFunc = func(ctx context.Context, jid, text, replyToID string) error {
		sent = append(sent, jid+"|"+text)
		return nil
	}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "1807890",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeImage,
			URL:  "https://example.com/image.png",
		}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("expected 1 fallback send, got %d", len(sent))
	}
	if sent[0] != "1807890@s.whatsapp.net|caption\nhttps://example.com/image.png" {
		t.Fatalf("unexpected fallback payload %q", sent[0])
	}
}

func TestChannel_Start_UsesWACLIDoctor(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.PhoneNumber = "+1807890"
	cfg.SessionPath = t.TempDir()
	cfg.CLIPath = "/bin/echo"

	ch := New(cfg, logger)
	var gotName string
	var gotArgs []string
	ch.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string{}, args...)
		return []byte("ok"), nil
	}

	err := ch.Start(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer ch.Stop(context.Background())

	if gotName != "/bin/echo" {
		t.Fatalf("expected CLI path /bin/echo, got %q", gotName)
	}
	joined := strings.Join(gotArgs, " ")
	if !strings.Contains(joined, "--store "+cfg.SessionPath) || !strings.Contains(joined, "doctor") {
		t.Fatalf("expected doctor call with store path, got %q", joined)
	}
	if !ch.IsConnected() {
		t.Fatal("expected channel to be connected after successful doctor")
	}
}

func TestChannel_Send_DefaultBackendUsesWACLIText(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.SessionPath = t.TempDir()
	cfg.CLIPath = "/bin/echo"

	ch := New(cfg, logger)
	ch.cliPath = cfg.CLIPath
	ch.mu.Lock()
	ch.isLoggedIn = true
	ch.status = channel.StatusConnected
	ch.mu.Unlock()

	var gotName string
	var gotArgs []string
	ch.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string{}, args...)
		return []byte("sent"), nil
	}

	err := ch.Send(context.Background(), channel.OutgoingMessage{ChatID: "+1807890", Content: "hello"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if gotName != cfg.CLIPath {
		t.Fatalf("expected CLI path %q, got %q", cfg.CLIPath, gotName)
	}
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"--store " + cfg.SessionPath, "send text", "--to +1807890", "--message hello"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in args %q", want, joined)
		}
	}
}

func TestChannel_Send_DefaultBackendUsesWACLIFile(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.SessionPath = t.TempDir()
	cfg.CLIPath = "/bin/echo"

	ch := New(cfg, logger)
	ch.cliPath = cfg.CLIPath
	ch.mu.Lock()
	ch.isLoggedIn = true
	ch.status = channel.StatusConnected
	ch.mu.Unlock()

	var fileArg string
	var fileBytes []byte
	ch.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--file" {
				fileArg = args[i+1]
				break
			}
		}
		if fileArg == "" {
			return nil, nil
		}
		data, err := os.ReadFile(fileArg)
		if err != nil {
			return nil, err
		}
		fileBytes = data
		return []byte("sent"), nil
	}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "+1807890",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type:     channel.MessageTypeFile,
			Name:     "agenda.pdf",
			MimeType: "application/pdf",
			Data:     []byte("pdf-bytes"),
		}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fileArg == "" {
		t.Fatal("expected --file argument to be passed")
	}
	if filepath.Ext(fileArg) != ".pdf" {
		t.Fatalf("expected temp file to preserve extension, got %q", fileArg)
	}
	if string(fileBytes) != "pdf-bytes" {
		t.Fatalf("unexpected temp file bytes %q", string(fileBytes))
	}
}
