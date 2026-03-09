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

func TestChannel_Send_MetadataCardsV2Preserved(t *testing.T) {
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
		Metadata: map[string]interface{}{
			"cardsV2": []map[string]interface{}{{
				"cardId": "custom",
				"card":   map[string]interface{}{"sections": []map[string]interface{}{{"widgets": []map[string]interface{}{{"textParagraph": map[string]interface{}{"text": "rich"}}}}}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if payload["text"] != "caption" {
		t.Fatalf("text payload = %v, want caption", payload["text"])
	}
	cards, _ := payload["cardsV2"].([]interface{})
	if len(cards) != 1 {
		t.Fatalf("expected 1 metadata card, got %d", len(cards))
	}
}

func TestChannel_Send_AttachmentCardAppendsToMetadataCardsV2(t *testing.T) {
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
		ChatID: "space-1",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeImage,
			URL:  "https://example.com/image.png",
		}},
		Metadata: map[string]interface{}{
			"cardsV2": []map[string]interface{}{{
				"cardId": "custom",
				"card":   map[string]interface{}{"sections": []map[string]interface{}{{"widgets": []map[string]interface{}{{"textParagraph": map[string]interface{}{"text": "rich"}}}}}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	cards, _ := payload["cardsV2"].([]interface{})
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards after append, got %d", len(cards))
	}
}

func TestChannel_Send_ReplyToIDUsesThreadName(t *testing.T) {
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
		ChatID:    "space-1",
		ReplyToID: "spaces/AAA/threads/thread-1",
		Content:   "caption",
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	thread, ok := payload["thread"].(map[string]interface{})
	if !ok {
		t.Fatalf("thread payload = %#v", payload["thread"])
	}
	if thread["name"] != "spaces/AAA/threads/thread-1" {
		t.Fatalf("thread name = %v", thread["name"])
	}
}

func TestChannel_SendStreaming_PreservesReplyToID(t *testing.T) {
	var payload map[string]interface{}
	ch := New(Config{WebhookURL: "https://chat.googleapis.test/webhook"}, zap.NewNop())
	ch.client = &http.Client{Transport: googleChatRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	content := make(chan string, 1)
	content <- "stream"
	close(content)
	done := make(chan struct{})
	if err := ch.SendStreaming(context.Background(), "space-1", "spaces/AAA/threads/thread-1", content, done); err != nil {
		t.Fatalf("SendStreaming error = %v", err)
	}
	thread, ok := payload["thread"].(map[string]interface{})
	if !ok {
		t.Fatalf("thread payload = %#v", payload["thread"])
	}
	if thread["name"] != "spaces/AAA/threads/thread-1" {
		t.Fatalf("thread name = %v", thread["name"])
	}
}
