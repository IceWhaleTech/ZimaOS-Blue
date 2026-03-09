package discord

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Name(t *testing.T) {
	cfg := channel.DiscordConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Name() != "discord" {
		t.Errorf("expected name 'discord', got %s", ch.Name())
	}
}

func TestChannel_Type(t *testing.T) {
	cfg := channel.DiscordConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Type() != "discord" {
		t.Errorf("expected type 'discord', got %s", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	cfg := channel.DiscordConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	info := ch.Info()
	if info.Name != "discord" {
		t.Errorf("expected name 'discord', got %s", info.Name)
	}
	if info.Type != "discord" {
		t.Errorf("expected type 'discord', got %s", info.Type)
	}
	if info.Status != channel.StatusDisconnected {
		t.Errorf("expected status 'disconnected', got %s", info.Status)
	}
	if !info.Enabled {
		t.Error("expected enabled to be true")
	}
}

func TestChannel_IsConnected(t *testing.T) {
	cfg := channel.DiscordConfig{
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
	cfg := channel.DiscordConfig{
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

func TestChannel_isGuildAllowed(t *testing.T) {
	tests := []struct {
		name          string
		allowedGuilds []string
		guildID       string
		expected      bool
	}{
		{
			name:          "empty allowed list allows all",
			allowedGuilds: []string{},
			guildID:       "180",
			expected:      true,
		},
		{
			name:          "guild ID in allowed list",
			allowedGuilds: []string{"180", "789"},
			guildID:       "180",
			expected:      true,
		},
		{
			name:          "guild ID not in allowed list",
			allowedGuilds: []string{"789", "999"},
			guildID:       "180",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.DiscordConfig{
				Enabled:       true,
				BotToken:      "test-token",
				AllowedGuilds: tt.allowedGuilds,
			}
			logger := zap.NewNop()
			ch := New(cfg, logger)

			result := ch.isGuildAllowed(tt.guildID)
			if result != tt.expected {
				t.Errorf("isGuildAllowed(%s) = %v, want %v", tt.guildID, result, tt.expected)
			}
		})
	}
}

func TestChannel_isUserAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedUsers []string
		userID       string
		expected     bool
	}{
		{
			name:         "empty allowed list allows all",
			allowedUsers: []string{},
			userID:       "180",
			expected:     true,
		},
		{
			name:         "user ID in allowed list",
			allowedUsers: []string{"180", "789"},
			userID:       "180",
			expected:     true,
		},
		{
			name:         "user ID not in allowed list",
			allowedUsers: []string{"789", "999"},
			userID:       "180",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.DiscordConfig{
				Enabled:      true,
				BotToken:     "test-token",
				AllowedUsers: tt.allowedUsers,
			}
			logger := zap.NewNop()
			ch := New(cfg, logger)

			result := ch.isUserAllowed(tt.userID)
			if result != tt.expected {
				t.Errorf("isUserAllowed(%s) = %v, want %v", tt.userID, result, tt.expected)
			}
		})
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	cfg := channel.DiscordConfig{
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
	cfg := channel.DiscordConfig{
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
	cfg := channel.DiscordConfig{
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

func TestChannel_HandleMessage_MessageHandlerRepliesToCurrentMessage(t *testing.T) {
	ch := New(channel.DiscordConfig{Enabled: true, BotToken: "test-token"}, zap.NewNop())
	ch.ctx = context.Background()
	ch.session = &discordgo.Session{}

	done := make(chan struct{})
	var sent *discordgo.MessageSend
	ch.sendMessageFunc = func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
		if channelID != "ch-1" {
			t.Fatalf("channelID = %q, want ch-1", channelID)
		}
		sent = data
		close(done)
		return &discordgo.Message{ID: "resp-1"}, nil
	}
	ch.SetMessageHandler(func(ctx context.Context, msg channel.Message) (string, error) {
		return "reply", nil
	})

	state := discordgo.NewState()
	state.Ready.User = &discordgo.User{ID: "bot-1"}
	session := &discordgo.Session{State: state}
	ch.handleMessage(session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        "msg-1",
		ChannelID: "ch-1",
		Content:   "hello",
		Author:    &discordgo.User{ID: "user-1", Username: "alice"},
	}})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reply send")
	}

	if sent == nil || sent.Reference == nil {
		t.Fatal("expected reply reference to be set")
	}
	if sent.Reference.MessageID != "msg-1" {
		t.Fatalf("reference message_id = %q, want current message id", sent.Reference.MessageID)
	}
	if sent.Reference.ChannelID != "ch-1" {
		t.Fatalf("reference channel_id = %q, want ch-1", sent.Reference.ChannelID)
	}
}

func TestChannel_ConvertMessage_NormalizesMentionsEmbedsComponentsAndReply(t *testing.T) {
	ch := New(channel.DiscordConfig{Enabled: true, BotToken: "test-token"}, zap.NewNop())
	msg := ch.convertMessage(&discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        "msg-1",
		ChannelID: "ch-1",
		GuildID:   "guild-1",
		Author:    &discordgo.User{ID: "user-1", Username: "alice"},
		Mentions: []*discordgo.User{{
			ID:         "user-2",
			Username:   "bob",
			GlobalName: "Bob",
		}},
		Embeds: []*discordgo.MessageEmbed{{
			Type:        discordgo.EmbedTypeRich,
			Title:       "Card title",
			Description: "Card body",
		}},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{Label: "Approve", CustomID: "approve", Style: discordgo.PrimaryButton},
			}},
		},
		MessageReference: &discordgo.MessageReference{MessageID: "parent-1", ChannelID: "ch-1"},
	}})

	if msg.Type != channel.MessageTypeCard {
		t.Fatalf("Type = %q, want card", msg.Type)
	}
	if msg.ReplyToID != "parent-1" {
		t.Fatalf("ReplyToID = %q, want parent-1", msg.ReplyToID)
	}
	mentionIDs, ok := msg.Metadata["mention_ids"].([]string)
	if !ok || len(mentionIDs) != 1 || mentionIDs[0] != "user-2" {
		t.Fatalf("mention_ids = %#v", msg.Metadata["mention_ids"])
	}
	embeds, ok := msg.Metadata["embeds"].([]map[string]interface{})
	if !ok || len(embeds) != 1 || embeds[0]["title"] != "Card title" {
		t.Fatalf("embeds = %#v", msg.Metadata["embeds"])
	}
	components, ok := msg.Metadata["components"].([]map[string]interface{})
	if !ok || len(components) != 1 {
		t.Fatalf("components = %#v", msg.Metadata["components"])
	}
	rowChildren, ok := components[0]["components"].([]map[string]interface{})
	if !ok || len(rowChildren) != 1 {
		t.Fatalf("row children = %#v", components[0]["components"])
	}
	if rowChildren[0]["custom_id"] != "approve" {
		t.Fatalf("first component = %#v", rowChildren[0])
	}
	if msg.Metadata["reference_message_id"] != "parent-1" {
		t.Fatalf("reference_message_id = %v", msg.Metadata["reference_message_id"])
	}
}
