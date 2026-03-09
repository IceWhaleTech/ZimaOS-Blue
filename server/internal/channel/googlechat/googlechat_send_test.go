package googlechat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

type googleChatRoundTripFunc func(*http.Request) (*http.Response, error)

func (f googleChatRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestChannel_Send_AttachmentWithoutURLFallsBackToText(t *testing.T) {
	var payload map[string]interface{}
	ch := New(Config{WebhookURL: "https://chat.googleapis.test/webhook"}, zap.NewNop())
	ch.client = &http.Client{Transport: googleChatRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "space-1",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			Name: "agenda.pdf",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if payload["text"] != "caption\nagenda.pdf" {
		t.Fatalf("text payload = %v, want caption+name", payload["text"])
	}
}

func TestChannel_Send_ImageWidgetKeepsText(t *testing.T) {
	var payload map[string]interface{}
	ch := New(Config{WebhookURL: "https://chat.googleapis.test/webhook"}, zap.NewNop())
	ch.client = &http.Client{Transport: googleChatRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "space-1",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeImage,
			URL:  "https://example.com/image.png",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if payload["text"] != "caption" {
		t.Fatalf("text payload = %v, want caption", payload["text"])
	}
	cards, _ := payload["cardsV2"].([]interface{})
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
}
