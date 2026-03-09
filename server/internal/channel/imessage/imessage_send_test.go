package imessage

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Send_AttachmentFallbackIncludedInText(t *testing.T) {
	ch := New(DefaultConfig(), zap.NewNop())

	var sentRecipient string
	var sentContent string
	ch.sendTextFunc = func(ctx context.Context, recipient, content string) error {
		sentRecipient = recipient
		sentContent = content
		return nil
	}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "+1807890",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			URL:  "https://example.com/file.pdf",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if sentRecipient != "+1807890" {
		t.Fatalf("recipient = %q", sentRecipient)
	}
	if sentContent != "caption\nhttps://example.com/file.pdf" {
		t.Fatalf("content = %q", sentContent)
	}
}

func TestChannel_Send_NoSendableContentErrors(t *testing.T) {
	ch := New(DefaultConfig(), zap.NewNop())
	ch.sendTextFunc = func(ctx context.Context, recipient, content string) error {
		t.Fatal("sendTextFunc should not be called")
		return nil
	}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: "+1807890",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
		}},
	})
	if err == nil {
		t.Fatal("expected no sendable content error")
	}
}
