package nextcloudtalk

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

type nextcloudRoundTripFunc func(*http.Request) (*http.Response, error)

func (f nextcloudRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestChannel_Send_ExternalAttachmentFallsBackToChatText(t *testing.T) {
	var paths []string
	var messages []string
	ch := New(Config{ServerURL: "https://nc.example", RoomToken: "room", Username: "u", Password: "p"}, zap.NewNop())
	ch.client = &http.Client{Transport: nextcloudRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		paths = append(paths, req.URL.Path)
		if strings.Contains(req.URL.Path, chatAPIPath) {
			body, _ := io.ReadAll(req.Body)
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}
			if msg, _ := payload["message"].(string); msg != "" {
				messages = append(messages, msg)
			}
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: "room",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			URL:  "https://example.com/file.pdf",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(paths) != 1 || !strings.Contains(paths[0], chatAPIPath) {
		t.Fatalf("expected single chat message request, got paths %#v", paths)
	}
	if len(messages) != 1 || messages[0] != "https://example.com/file.pdf" {
		t.Fatalf("messages = %#v, want URL fallback", messages)
	}
}

func TestChannel_Send_NoSendableContentErrors(t *testing.T) {
	ch := New(Config{ServerURL: "https://nc.example", RoomToken: "room", Username: "u", Password: "p"}, zap.NewNop())
	ch.client = &http.Client{Transport: nextcloudRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: "room",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
		}},
	})
	if err == nil {
		t.Fatal("expected no sendable content error")
	}
}

func TestChannel_Send_AttachmentFallbackPreservesReplyTarget(t *testing.T) {
	var payloads []map[string]interface{}
	ch := New(Config{ServerURL: "https://nc.example", RoomToken: "room", Username: "u", Password: "p"}, zap.NewNop())
	ch.client = &http.Client{Transport: nextcloudRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, chatAPIPath) {
			body, _ := io.ReadAll(req.Body)
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}
			payloads = append(payloads, payload)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:    "room",
		ReplyToID: "42",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			URL:  "https://example.com/file.pdf",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(payloads) != 1 {
		t.Fatalf("payloads = %#v, want 1 request", payloads)
	}
	if got := payloads[0]["replyTo"]; got != float64(42) {
		t.Fatalf("replyTo = %#v, want 42", got)
	}
	if got := payloads[0]["message"]; got != "https://example.com/file.pdf" {
		t.Fatalf("message = %#v", got)
	}
}
