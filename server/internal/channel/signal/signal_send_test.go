package signal

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Send_AttachmentDataUsesAttachmentPath(t *testing.T) {
	ch := newRegisteredTestSignalChannel()

	var textCalls []string
	var gotCaption string
	var gotFilename string
	ch.sendTextFunc = func(ctx context.Context, chatID, content string) error {
		textCalls = append(textCalls, content)
		return nil
	}
	ch.sendImageDataFunc = func(ctx context.Context, chatID string, imageData []byte, filename string, caption string) error {
		gotFilename = filename
		gotCaption = caption
		return nil
	}
	ch.sendFileDataFunc = func(ctx context.Context, chatID string, fileData []byte, filename string, caption string) error {
		t.Fatal("sendFileDataFunc should not be called")
		return nil
	}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "+1807890",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeImage,
			Name: "photo.png",
			Data: []byte("img"),
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if gotFilename != "photo.png" {
		t.Fatalf("filename = %q", gotFilename)
	}
	if gotCaption != "caption" {
		t.Fatalf("caption = %q", gotCaption)
	}
	if len(textCalls) != 0 {
		t.Fatalf("unexpected text calls: %#v", textCalls)
	}
}

func TestChannel_Send_URLAttachmentFallsBackToText(t *testing.T) {
	ch := newRegisteredTestSignalChannel()

	var textCalls []string
	ch.sendTextFunc = func(ctx context.Context, chatID, content string) error {
		textCalls = append(textCalls, content)
		return nil
	}
	ch.sendImageDataFunc = func(ctx context.Context, chatID string, imageData []byte, filename string, caption string) error {
		return nil
	}
	ch.sendFileDataFunc = func(ctx context.Context, chatID string, fileData []byte, filename string, caption string) error {
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
	if len(textCalls) != 1 {
		t.Fatalf("text call count = %d, want 1", len(textCalls))
	}
	if textCalls[0] != "caption\nhttps://example.com/file.pdf" {
		t.Fatalf("text = %q", textCalls[0])
	}
}

func TestChannel_Send_NoSendableContentErrors(t *testing.T) {
	ch := newRegisteredTestSignalChannel()
	ch.sendTextFunc = func(ctx context.Context, chatID, content string) error {
		t.Fatal("sendTextFunc should not be called")
		return nil
	}
	ch.sendImageDataFunc = func(ctx context.Context, chatID string, imageData []byte, filename string, caption string) error {
		t.Fatal("sendImageDataFunc should not be called")
		return nil
	}
	ch.sendFileDataFunc = func(ctx context.Context, chatID string, fileData []byte, filename string, caption string) error {
		t.Fatal("sendFileDataFunc should not be called")
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

func newRegisteredTestSignalChannel() *Channel {
	cfg := DefaultConfig()
	cfg.PhoneNumber = "+1807890"
	ch := New(cfg, zap.NewNop())
	ch.isRegistered = true
	return ch
}

func TestChannel_HandleDataMessage_AttachmentOnlyMessagePreserved(t *testing.T) {
	ch := newRegisteredTestSignalChannel()
	jsonLine := `{"envelope":{"source":"+1234567890","sourceName":"Alice","sourceUuid":"uuid-1","sourceDevice":1,"dataMessage":{"timestamp":1710000000000,"message":"","attachments":[{"contentType":"image/png","filename":"photo.png","id":"att-1","size":12}]}}}`

	ch.processMessage(jsonLine)

	select {
	case got := <-ch.Messages():
		if got.Type != channel.MessageTypeImage {
			t.Fatalf("Type = %q, want image", got.Type)
		}
		if got.Content != "" {
			t.Fatalf("Content = %q, want empty", got.Content)
		}
		if len(got.Attachments) != 1 {
			t.Fatalf("Attachments len = %d, want 1", len(got.Attachments))
		}
		if got.Attachments[0].Name != "photo.png" {
			t.Fatalf("attachment name = %q", got.Attachments[0].Name)
		}
		if got.Metadata["attachment_count"] != 1 {
			t.Fatalf("attachment_count = %v, want 1", got.Metadata["attachment_count"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for signal message")
	}
}
