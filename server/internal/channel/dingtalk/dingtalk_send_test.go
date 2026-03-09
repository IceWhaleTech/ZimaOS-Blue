package dingtalk

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Send_FileWithoutURLFallsBackToName(t *testing.T) {
	ch := New(Config{}, zap.NewNop())
	var texts []string
	ch.sendTextFunc = func(ctx context.Context, chatID, content string) error {
		texts = append(texts, content)
		return nil
	}
	ch.sendAttachmentFunc = func(ctx context.Context, chatID, caption string, att channel.Attachment) error {
		return context.DeadlineExceeded
	}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "u1",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			Name: "agenda.pdf",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(texts) != 1 || texts[0] != "caption\nagenda.pdf" {
		t.Fatalf("texts = %#v, want caption+name", texts)
	}
}

func TestChannel_Send_AttachmentSuccessConsumesCaption(t *testing.T) {
	ch := New(Config{}, zap.NewNop())
	var texts []string
	ch.sendTextFunc = func(ctx context.Context, chatID, content string) error {
		texts = append(texts, content)
		return nil
	}
	ch.sendAttachmentFunc = func(ctx context.Context, chatID, caption string, att channel.Attachment) error {
		if caption != "caption" {
			t.Fatalf("caption = %q, want caption", caption)
		}
		return nil
	}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:      "u1",
		Content:     "caption",
		Attachments: []channel.Attachment{{Type: channel.MessageTypeFile, Data: []byte("x")}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(texts) != 0 {
		t.Fatalf("expected no trailing text send, got %#v", texts)
	}
}

func TestMarshalJSONString_InvalidValueReturnsError(t *testing.T) {
	if _, err := marshalJSONString(map[string]interface{}{"bad": func() {}}); err == nil {
		t.Fatal("expected marshalJSONString to return error")
	}
}
