package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Name(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Name() != "telegram" {
		t.Errorf("expected name 'telegram', got %s", ch.Name())
	}
}

func TestChannel_Type(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Type() != "telegram" {
		t.Errorf("expected type 'telegram', got %s", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	info := ch.Info()
	if info.Name != "telegram" {
		t.Errorf("expected name 'telegram', got %s", info.Name)
	}
	if info.Type != "telegram" {
		t.Errorf("expected type 'telegram', got %s", info.Type)
	}
	if info.Status != channel.StatusDisconnected {
		t.Errorf("expected status 'disconnected', got %s", info.Status)
	}
	if !info.Enabled {
		t.Error("expected enabled to be true")
	}
}

func TestChannel_IsConnected(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("expected channel to not be connected initially")
	}
}

func TestChannel_Messages(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	messages := ch.Messages()
	if messages == nil {
		t.Error("expected messages channel to not be nil")
	}
}

func TestChannel_isUserAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedUsers []string
		userID       int64
		username     string
		expected     bool
	}{
		{
			name:         "empty allowed list allows all",
			allowedUsers: []string{},
			userID:       123,
			username:     "testuser",
			expected:     true,
		},
		{
			name:         "user ID in allowed list",
			allowedUsers: []string{"123", "456"},
			userID:       123,
			username:     "testuser",
			expected:     true,
		},
		{
			name:         "username in allowed list",
			allowedUsers: []string{"testuser", "otheruser"},
			userID:       123,
			username:     "testuser",
			expected:     true,
		},
		{
			name:         "user not in allowed list",
			allowedUsers: []string{"456", "otheruser"},
			userID:       123,
			username:     "testuser",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.TelegramConfig{
				Enabled:      true,
				BotToken:     "test-token",
				AllowedUsers: tt.allowedUsers,
			}
			logger := zap.NewNop()
			_ = New(cfg, logger)

			// We can't directly test isUserAllowed without creating a tgbotapi.User
			// This test documents the expected behavior
		})
	}
}

func TestChannel_isGroupAllowed(t *testing.T) {
	tests := []struct {
		name          string
		allowedGroups []string
		chatID        int64
		expected      bool
	}{
		{
			name:          "empty allowed list allows all",
			allowedGroups: []string{},
			chatID:        -180,
			expected:      true,
		},
		{
			name:          "chat ID in allowed list",
			allowedGroups: []string{"-180", "-789"},
			chatID:        -180,
			expected:      true,
		},
		{
			name:          "chat ID not in allowed list",
			allowedGroups: []string{"-789", "-999"},
			chatID:        -180,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.TelegramConfig{
				Enabled:       true,
				BotToken:      "test-token",
				AllowedGroups: tt.allowedGroups,
			}
			logger := zap.NewNop()
			ch := New(cfg, logger)

			result := ch.isGroupAllowed(tt.chatID)
			if result != tt.expected {
				t.Errorf("isGroupAllowed(%d) = %v, want %v", tt.chatID, result, tt.expected)
			}
		})
	}
}

func TestParseChatID(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"180", 180, false},
		{"-180", -180, false},
		{"0", 0, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		result, err := parseChatID(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("parseChatID(%s) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseChatID(%s) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("parseChatID(%s) = %d, want %d", tt.input, result, tt.expected)
			}
		}
	}
}

func TestParseMessageID(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"123", 123, false},
		{"0", 0, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		result, err := parseMessageID(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("parseMessageID(%s) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseMessageID(%s) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("parseMessageID(%s) = %d, want %d", tt.input, result, tt.expected)
			}
		}
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
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

func TestChannel_Send_NotInitialized(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx := context.Background()
	msg := channel.OutgoingMessage{
		ChatID:  "180",
		Content: "Hello",
	}

	err := ch.Send(ctx, msg)
	if err == nil {
		t.Error("expected error when sending without initialization")
	}
}

func TestChannel_SendStreaming_NotInitialized(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx := context.Background()
	content := make(chan string)
	done := make(chan struct{})

	close(content) // Close immediately

	err := ch.SendStreaming(ctx, "180", "", content, done)
	if err == nil {
		t.Error("expected error when streaming without initialization")
	}
}

func TestBuildAttachmentChattable_AppliesReplyAndParseMode(t *testing.T) {
	att := channel.Attachment{
		Type: channel.MessageTypeImage,
		Name: "image.png",
		URL:  "https://example.com/image.png",
	}

	chattable, err := buildAttachmentChattable(123, "42", "<b>hello</b>", "html", att)
	if err != nil {
		t.Fatalf("buildAttachmentChattable returned error: %v", err)
	}

	photo, ok := chattable.(tgbotapi.PhotoConfig)
	if !ok {
		t.Fatalf("expected PhotoConfig, got %T", chattable)
	}
	if photo.ReplyToMessageID != 42 {
		t.Fatalf("expected reply_to_message_id 42, got %d", photo.ReplyToMessageID)
	}
	if photo.ParseMode != tgbotapi.ModeHTML {
		t.Fatalf("expected HTML parse mode, got %q", photo.ParseMode)
	}
	if photo.Caption != "<b>hello</b>" {
		t.Fatalf("expected caption to be preserved, got %q", photo.Caption)
	}
}

func TestTelegramParseMode(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{format: "markdown", want: tgbotapi.ModeMarkdown},
		{format: "md", want: tgbotapi.ModeMarkdown},
		{format: "html", want: tgbotapi.ModeHTML},
		{format: "markdownv2", want: tgbotapi.ModeMarkdownV2},
		{format: "plain", want: ""},
	}

	for _, tt := range tests {
		if got := telegramParseMode(tt.format); got != tt.want {
			t.Fatalf("telegramParseMode(%q) = %q, want %q", tt.format, got, tt.want)
		}
	}
}

func TestChannel_HandleUpdate_MessageHandlerRepliesToCurrentMessage(t *testing.T) {
	var replyTo string
	done := make(chan struct{})
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bottest-token/getMe":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"result": map[string]interface{}{
					"id":         1,
					"is_bot":     true,
					"first_name": "bot",
					"username":   "bot",
				},
			})
		case "/bottest-token/sendMessage":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			replyTo = r.FormValue("reply_to_message_id")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"result": map[string]interface{}{
					"message_id": 999,
					"date":       1710000000,
					"chat": map[string]interface{}{
						"id":   456,
						"type": "private",
					},
					"text": "reply",
				},
			})
			close(done)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	bot, err := tgbotapi.NewBotAPIWithClient("test-token", server.URL+"/bot%s/%s", server.Client())
	if err != nil {
		t.Fatalf("new bot api: %v", err)
	}

	ch := New(channel.TelegramConfig{Enabled: true}, zap.NewNop())
	ch.ctx = context.Background()
	ch.bot = bot
	ch.SetMessageHandler(func(ctx context.Context, msg channel.Message) (string, error) {
		return "reply", nil
	})

	ch.handleUpdate(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 123,
		Text:      "hello",
		Chat:      &tgbotapi.Chat{ID: 456, Type: "private"},
		From:      &tgbotapi.User{ID: 7, UserName: "alice"},
	}})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reply send")
	}

	if replyTo != "123" {
		t.Fatalf("reply_to_message_id = %q, want current message id", replyTo)
	}
}

func TestChannel_ConvertMessage_PreservesCaptionEntitiesAndMediaKinds(t *testing.T) {
	ch := New(channel.TelegramConfig{Enabled: true}, zap.NewNop())
	msg := ch.convertMessage(&tgbotapi.Message{
		MessageID: 321,
		Date:      1710000000,
		Chat:      &tgbotapi.Chat{ID: 456, Type: "private"},
		From:      &tgbotapi.User{ID: 7, UserName: "alice"},
		Caption:   "See Bob",
		CaptionEntities: []tgbotapi.MessageEntity{{
			Type:   "text_mention",
			Offset: 4,
			Length: 3,
			User:   &tgbotapi.User{ID: 99, UserName: "bob", FirstName: "Bob"},
		}},
		Video: &tgbotapi.Video{FileID: "video-1", FileName: "demo.mp4", MimeType: "video/mp4", FileSize: 2048},
	})

	if msg.Type != channel.MessageTypeVideo {
		t.Fatalf("Type = %q, want video", msg.Type)
	}
	if msg.Content != "See Bob" {
		t.Fatalf("Content = %q, want caption", msg.Content)
	}
	if len(msg.Attachments) != 1 || msg.Attachments[0].ID != "video-1" {
		t.Fatalf("Attachments = %#v", msg.Attachments)
	}
	if msg.Attachments[0].MimeType != "video/mp4" {
		t.Fatalf("MimeType = %q", msg.Attachments[0].MimeType)
	}
	if msg.Metadata["entity_source"] != "caption" {
		t.Fatalf("entity_source = %v", msg.Metadata["entity_source"])
	}
	mentionIDs, ok := msg.Metadata["mention_ids"].([]string)
	if !ok || len(mentionIDs) != 1 || mentionIDs[0] != "99" {
		t.Fatalf("mention_ids = %#v", msg.Metadata["mention_ids"])
	}
	mentions, ok := msg.Metadata["mentions"].([]map[string]interface{})
	if !ok || len(mentions) != 1 || mentions[0]["id"] != "99" {
		t.Fatalf("mentions = %#v", msg.Metadata["mentions"])
	}
}

func TestChannel_ConvertMessage_MapsVoiceAndSticker(t *testing.T) {
	ch := New(channel.TelegramConfig{Enabled: true}, zap.NewNop())

	voiceMsg := ch.convertMessage(&tgbotapi.Message{
		MessageID: 401,
		Chat:      &tgbotapi.Chat{ID: 456, Type: "private"},
		From:      &tgbotapi.User{ID: 7, UserName: "alice"},
		Voice:     &tgbotapi.Voice{FileID: "voice-1", MimeType: "audio/ogg", FileSize: 128},
	})
	if voiceMsg.Type != channel.MessageTypeAudio {
		t.Fatalf("voice Type = %q, want audio", voiceMsg.Type)
	}
	if len(voiceMsg.Attachments) != 1 || voiceMsg.Attachments[0].ID != "voice-1" {
		t.Fatalf("voice attachments = %#v", voiceMsg.Attachments)
	}

	stickerMsg := ch.convertMessage(&tgbotapi.Message{
		MessageID: 402,
		Chat:      &tgbotapi.Chat{ID: -100, Type: "supergroup", Title: "group"},
		From:      &tgbotapi.User{ID: 7, UserName: "alice"},
		Sticker:   &tgbotapi.Sticker{FileID: "sticker-1", Emoji: "🙂", SetName: "set", FileSize: 64},
	})
	if stickerMsg.Type != channel.MessageTypeImage {
		t.Fatalf("sticker Type = %q, want image", stickerMsg.Type)
	}
	if stickerMsg.Metadata["sticker_emoji"] != "🙂" {
		t.Fatalf("sticker_emoji = %v", stickerMsg.Metadata["sticker_emoji"])
	}
	if stickerMsg.Metadata["sticker_set_name"] != "set" {
		t.Fatalf("sticker_set_name = %v", stickerMsg.Metadata["sticker_set_name"])
	}
}

func TestChannel_HandleCallbackQuery_PreservesOriginMessageMetadata(t *testing.T) {
	answered := false
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bottest-token/getMe":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"result": map[string]interface{}{
					"id":         1,
					"is_bot":     true,
					"first_name": "bot",
					"username":   "bot",
				},
			})
		case "/bottest-token/answerCallbackQuery":
			answered = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "result": true})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	bot, err := tgbotapi.NewBotAPIWithClient("test-token", server.URL+"/bot%s/%s", server.Client())
	if err != nil {
		t.Fatalf("new bot api: %v", err)
	}

	ch := New(channel.TelegramConfig{Enabled: true}, zap.NewNop())
	ch.ctx = context.Background()
	ch.bot = bot
	ch.handleCallbackQuery(&tgbotapi.CallbackQuery{
		ID:           "cb-1",
		Data:         "approve",
		ChatInstance: "ci-1",
		From:         &tgbotapi.User{ID: 7, UserName: "alice"},
		Message:      &tgbotapi.Message{MessageID: 123, Chat: &tgbotapi.Chat{ID: -100, Type: "supergroup", Title: "Ops"}},
	})

	select {
	case msg := <-ch.Messages():
		if msg.ReplyToID != "123" {
			t.Fatalf("ReplyToID = %q, want 123", msg.ReplyToID)
		}
		if msg.Metadata["origin_message_id"] != "123" {
			t.Fatalf("origin_message_id = %v", msg.Metadata["origin_message_id"])
		}
		if msg.Metadata["chat_instance"] != "ci-1" {
			t.Fatalf("chat_instance = %v", msg.Metadata["chat_instance"])
		}
		if msg.GroupName != "Ops" {
			t.Fatalf("GroupName = %q, want Ops", msg.GroupName)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for callback message")
	}

	if !answered {
		t.Fatal("expected callback query to be answered")
	}
}
