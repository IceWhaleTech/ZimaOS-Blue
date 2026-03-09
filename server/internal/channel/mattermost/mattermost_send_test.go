package mattermost

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Send_URLAttachmentFallsBackToMessageText(t *testing.T) {
	var post map[string]interface{}
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4/posts":
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
				t.Fatalf("decode post: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := channel.MattermostConfig{Enabled: true, ServerURL: server.URL, BotToken: "token"}
	ch := New(cfg, zap.NewNop())
	ch.status = channel.StatusConnected
	ch.httpClient = server.Client()

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "channel-1",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			URL:  "https://example.com/file.pdf",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if post["message"] != "caption\nhttps://example.com/file.pdf" {
		t.Fatalf("message = %v", post["message"])
	}
}

func TestChannel_Send_UploadFailureFallsBackToName(t *testing.T) {
	var post map[string]interface{}
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4/files":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`upload failed`))
		case "/api/v4/posts":
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
				t.Fatalf("decode post: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := channel.MattermostConfig{Enabled: true, ServerURL: server.URL, BotToken: "token"}
	ch := New(cfg, zap.NewNop())
	ch.status = channel.StatusConnected
	ch.httpClient = server.Client()

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: "channel-1",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			Name: "report.pdf",
			Data: []byte("file-data"),
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if post["message"] != "report.pdf" {
		t.Fatalf("message = %v", post["message"])
	}
}

func TestChannel_Send_NoSendableContentErrors(t *testing.T) {
	cfg := channel.MattermostConfig{Enabled: true, ServerURL: "http://127.0.0.1", BotToken: "token"}
	ch := New(cfg, zap.NewNop())
	ch.status = channel.StatusConnected

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: "channel-1",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
		}},
	})
	if err == nil {
		t.Fatal("expected no sendable content error")
	}
}

func TestChannel_Send_PassesPropsAndAttachmentsMetadata(t *testing.T) {
	var post map[string]interface{}
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4/posts":
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
				t.Fatalf("decode post: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := channel.MattermostConfig{Enabled: true, ServerURL: server.URL, BotToken: "token"}
	ch := New(cfg, zap.NewNop())
	ch.status = channel.StatusConnected
	ch.httpClient = server.Client()

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "channel-1",
		Content: "hello",
		Metadata: map[string]interface{}{
			"props": map[string]interface{}{"from_bot": true},
			"attachments": []map[string]interface{}{{
				"fallback": "legacy",
				"text":     "rich",
			}},
		},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	props, ok := post["props"].(map[string]interface{})
	if !ok {
		t.Fatalf("props = %#v", post["props"])
	}
	if props["from_bot"] != true {
		t.Fatalf("from_bot = %v", props["from_bot"])
	}
	attachments, ok := props["attachments"].([]interface{})
	if !ok || len(attachments) != 1 {
		t.Fatalf("attachments = %#v", props["attachments"])
	}
}

func TestChannel_Send_MetadataAttachmentsWithoutTextStillSend(t *testing.T) {
	var post map[string]interface{}
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4/posts":
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
				t.Fatalf("decode post: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := channel.MattermostConfig{Enabled: true, ServerURL: server.URL, BotToken: "token"}
	ch := New(cfg, zap.NewNop())
	ch.status = channel.StatusConnected
	ch.httpClient = server.Client()

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: "channel-1",
		Metadata: map[string]interface{}{
			"attachments": []map[string]interface{}{{
				"fallback": "legacy",
				"text":     "rich",
			}},
		},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	props, ok := post["props"].(map[string]interface{})
	if !ok {
		t.Fatalf("props = %#v", post["props"])
	}
	attachments, ok := props["attachments"].([]interface{})
	if !ok || len(attachments) != 1 {
		t.Fatalf("attachments = %#v", props["attachments"])
	}
}
