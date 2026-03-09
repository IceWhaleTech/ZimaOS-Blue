package teams

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Send_MarkdownAndAttachmentFallback(t *testing.T) {
	var activity map[string]interface{}
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&activity); err != nil {
			t.Fatalf("decode activity: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	cfg := channel.TeamsConfig{Enabled: true, AppID: "app", AppPassword: "pw"}
	ch := New(cfg, zap.NewNop())
	ch.accessToken = "token"
	ch.httpClient = server.Client()

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  server.URL + "|conv123",
		Content: "**hello**",
		Format:  "markdown",
		Attachments: []channel.Attachment{
			{Type: channel.MessageTypeFile, Name: "report.pdf"},
			{Type: channel.MessageTypeImage, Name: "image.png", URL: "https://example.com/image.png", MimeType: "image/png"},
		},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if activity["text"] != "**hello**\nreport.pdf" {
		t.Fatalf("text = %v", activity["text"])
	}
	if activity["textFormat"] != "markdown" {
		t.Fatalf("textFormat = %v", activity["textFormat"])
	}
	attachments, ok := activity["attachments"].([]interface{})
	if !ok || len(attachments) != 1 {
		t.Fatalf("attachments = %#v, want 1 item", activity["attachments"])
	}
	first, ok := attachments[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first attachment type = %T", attachments[0])
	}
	if first["contentUrl"] != "https://example.com/image.png" {
		t.Fatalf("contentUrl = %v", first["contentUrl"])
	}
}

func TestChannel_Send_NoSendableContentErrors(t *testing.T) {
	cfg := channel.TeamsConfig{Enabled: true, AppID: "app", AppPassword: "pw"}
	ch := New(cfg, zap.NewNop())
	ch.accessToken = "token"

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: serverURLPlaceholder("conv123"),
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
		}},
	})
	if err == nil {
		t.Fatal("expected no sendable content error")
	}
}

func serverURLPlaceholder(conversationID string) string {
	return "http://127.0.0.1|" + conversationID
}

func TestChannel_Send_MetadataAttachmentsWithoutTextStillSend(t *testing.T) {
	var activity map[string]interface{}
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&activity); err != nil {
			t.Fatalf("decode activity: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	cfg := channel.TeamsConfig{Enabled: true, AppID: "app", AppPassword: "pw"}
	ch := New(cfg, zap.NewNop())
	ch.accessToken = "token"
	ch.httpClient = server.Client()

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: server.URL + "|conv123",
		Metadata: map[string]interface{}{
			"summary":     "adaptive card",
			"channelData": map[string]interface{}{"tenant": map[string]interface{}{"id": "tenant-1"}},
			"attachments": []map[string]interface{}{{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content": map[string]interface{}{
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"type":    "AdaptiveCard",
					"version": "1.4",
					"body":    []map[string]interface{}{{"type": "TextBlock", "text": "hello"}},
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if activity["summary"] != "adaptive card" {
		t.Fatalf("summary = %v", activity["summary"])
	}
	attachments, ok := activity["attachments"].([]interface{})
	if !ok || len(attachments) != 1 {
		t.Fatalf("attachments = %#v, want 1 item", activity["attachments"])
	}
	channelData, ok := activity["channelData"].(map[string]interface{})
	if !ok {
		t.Fatalf("channelData = %#v", activity["channelData"])
	}
	if _, ok := channelData["tenant"].(map[string]interface{}); !ok {
		t.Fatalf("tenant = %#v", channelData["tenant"])
	}
}
