package bluebubbles

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Send_AttachmentFallbackIncludedInMessage(t *testing.T) {
	var payload map[string]interface{}
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	cfg := channel.BlueBubblesConfig{Enabled: true, ServerURL: server.URL, Password: "pw"}
	ch := New(cfg, zap.NewNop())
	ch.status = channel.StatusConnected
	ch.httpClient = server.Client()

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "iMessage;+;chat123",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			URL:  "https://example.com/file.pdf",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if payload["message"] != "caption\nhttps://example.com/file.pdf" {
		t.Fatalf("message = %v", payload["message"])
	}
}

func TestChannel_Send_NoSendableContentErrors(t *testing.T) {
	cfg := channel.BlueBubblesConfig{Enabled: true, ServerURL: "http://127.0.0.1", Password: "pw"}
	ch := New(cfg, zap.NewNop())
	ch.status = channel.StatusConnected

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: "iMessage;+;chat123",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
		}},
	})
	if err == nil {
		t.Fatal("expected no sendable content error")
	}
}
