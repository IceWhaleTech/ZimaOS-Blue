package discord

import (
	"context"
	"testing"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Send_AttachmentFallbackAndComponents(t *testing.T) {
	cfg := channel.DiscordConfig{Enabled: true, BotToken: "token"}
	ch := New(cfg, zap.NewNop())
	ch.session = &discordgo.Session{}

	var got *discordgo.MessageSend
	ch.sendMessageFunc = func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
		if channelID != "180" {
			t.Fatalf("channelID = %q, want 180", channelID)
		}
		got = data
		return &discordgo.Message{ID: "msg1"}, nil
	}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "180",
		Content: "hello",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			URL:  "https://example.com/file.pdf",
		}},
		Metadata: map[string]interface{}{
			"embeds": []*discordgo.MessageEmbed{{Title: "embed"}},
			"components": []ActionRow{{
				Components: []MessageComponent{{
					Type:     discordgo.ButtonComponent,
					CustomID: "open",
					Label:    "Open",
					Style:    discordgo.PrimaryButton,
				}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if got == nil {
		t.Fatal("expected message payload")
	}
	if got.Content != "hello\nhttps://example.com/file.pdf" {
		t.Fatalf("content = %q", got.Content)
	}
	if len(got.Embeds) != 1 {
		t.Fatalf("embeds len = %d, want 1", len(got.Embeds))
	}
	if len(got.Components) != 1 {
		t.Fatalf("components len = %d, want 1", len(got.Components))
	}
}

func TestChannel_Send_NoSendableContentErrors(t *testing.T) {
	cfg := channel.DiscordConfig{Enabled: true, BotToken: "token"}
	ch := New(cfg, zap.NewNop())
	ch.session = &discordgo.Session{}
	ch.sendMessageFunc = func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
		t.Fatal("sendMessageFunc should not be called")
		return nil, nil
	}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: "180",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
		}},
	})
	if err == nil {
		t.Fatal("expected no sendable content error")
	}
}
