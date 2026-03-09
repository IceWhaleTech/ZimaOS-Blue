package viber

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

type viberRoundTripFunc func(*http.Request) (*http.Response, error)

func (f viberRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestChannel_Send_ImageFailureFallsBackToText(t *testing.T) {
	var payloads []sendPayload
	ch := New(Config{AuthToken: "token", BotName: "bot"}, zap.NewNop())
	ch.client = &http.Client{Transport: viberRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload sendPayload
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		payloads = append(payloads, payload)
		respBody := `{"status":0}`
		if payload.Type == "picture" {
			respBody = `{"status":1,"status_message":"boom"}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(respBody)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "user-1",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeImage,
			URL:  "https://example.com/image.png",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(payloads) != 2 {
		t.Fatalf("expected 2 payloads, got %d", len(payloads))
	}
	if payloads[1].Type != "text" || payloads[1].Text != "caption\nhttps://example.com/image.png" {
		t.Fatalf("unexpected fallback payload: %#v", payloads[1])
	}
}

func TestChannel_Send_AttachmentWithoutURLFallsBackToName(t *testing.T) {
	var payloads []sendPayload
	ch := New(Config{AuthToken: "token", BotName: "bot"}, zap.NewNop())
	ch.client = &http.Client{Transport: viberRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload sendPayload
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		payloads = append(payloads, payload)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":0}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "user-1",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			Name: "agenda.pdf",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if payloads[0].Type != "text" || payloads[0].Text != "caption\nagenda.pdf" {
		t.Fatalf("unexpected payload: %#v", payloads[0])
	}
}

func TestChannel_ProcessMessage_FilePreservesMetadata(t *testing.T) {
	ch := New(Config{AuthToken: "token", BotName: "bot"}, zap.NewNop())
	ch.processMessage(callbackEvent{
		Event:        "message",
		Timestamp:    time.Now().UnixMilli(),
		MessageToken: 1,
		Sender:       &callbackSender{ID: "user-1", Name: "Alice"},
		Message: &callbackMessage{
			Type:     "file",
			Media:    "https://example.com/file.pdf",
			FileName: "file.pdf",
			FileSize: 42,
		},
	})

	select {
	case got := <-ch.Messages():
		if len(got.Attachments) != 1 {
			t.Fatalf("Attachments len = %d, want 1", len(got.Attachments))
		}
		att := got.Attachments[0]
		if att.Name != "file.pdf" {
			t.Fatalf("Name = %q, want file.pdf", att.Name)
		}
		if att.Size != 42 {
			t.Fatalf("Size = %d, want 42", att.Size)
		}
		if att.MimeType != "application/pdf" {
			t.Fatalf("MimeType = %q", att.MimeType)
		}
		if got.Metadata["attachment_count"] != 1 {
			t.Fatalf("attachment_count = %#v", got.Metadata["attachment_count"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for viber message")
	}
}

func TestChannel_ProcessMessage_RichMediaBecomesCard(t *testing.T) {
	ch := New(Config{AuthToken: "token", BotName: "bot"}, zap.NewNop())
	ch.processMessage(callbackEvent{
		Event:        "message",
		Timestamp:    time.Now().UnixMilli(),
		MessageToken: 2,
		Sender:       &callbackSender{ID: "user-2", Name: "Bob", Avatar: "https://example.com/avatar.png"},
		Message: &callbackMessage{
			Type: "rich_media",
			RichMedia: map[string]interface{}{
				"Type": "rich_media",
				"Buttons": []map[string]interface{}{{
					"Text": "Approve",
				}},
			},
			TrackingData: "trace-1",
		},
	})

	select {
	case got := <-ch.Messages():
		if got.Type != channel.MessageTypeCard {
			t.Fatalf("Type = %q, want card", got.Type)
		}
		if got.Content != "[rich media]" {
			t.Fatalf("Content = %q, want [rich media]", got.Content)
		}
		richMedia, ok := got.Metadata["rich_media"].(map[string]interface{})
		if !ok || richMedia["Type"] != "rich_media" {
			t.Fatalf("rich_media = %#v", got.Metadata["rich_media"])
		}
		if got.Metadata["tracking_data"] != "trace-1" {
			t.Fatalf("tracking_data = %v", got.Metadata["tracking_data"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for viber rich media message")
	}
}

func TestChannel_ProcessMessage_URLFallsBackToMedia(t *testing.T) {
	ch := New(Config{AuthToken: "token", BotName: "bot"}, zap.NewNop())
	ch.processMessage(callbackEvent{
		Event:        "message",
		Timestamp:    time.Now().UnixMilli(),
		MessageToken: 3,
		Sender:       &callbackSender{ID: "user-3", Name: "Carol"},
		Message: &callbackMessage{
			Type:  "url",
			Media: "https://example.com/page",
		},
	})

	select {
	case got := <-ch.Messages():
		if got.Type != channel.MessageTypeText {
			t.Fatalf("Type = %q, want text", got.Type)
		}
		if got.Content != "https://example.com/page" {
			t.Fatalf("Content = %q", got.Content)
		}
		if got.Metadata["media"] != "https://example.com/page" {
			t.Fatalf("media = %v", got.Metadata["media"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for viber url message")
	}
}
