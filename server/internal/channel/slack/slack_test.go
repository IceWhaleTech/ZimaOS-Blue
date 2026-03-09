package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Name(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Name() != "slack" {
		t.Errorf("expected name 'slack', got %s", ch.Name())
	}
}

func TestChannel_Type(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Type() != "slack" {
		t.Errorf("expected type 'slack', got %s", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	info := ch.Info()
	if info.Name != "slack" {
		t.Errorf("expected name 'slack', got %s", info.Name)
	}
	if info.Type != "slack" {
		t.Errorf("expected type 'slack', got %s", info.Type)
	}
	if info.Status != channel.StatusDisconnected {
		t.Errorf("expected status 'disconnected', got %s", info.Status)
	}
	if !info.Enabled {
		t.Error("expected enabled to be true")
	}
}

func TestChannel_IsConnected(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("expected channel to not be connected initially")
	}
}

func TestChannel_Messages(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	messages := ch.Messages()
	if messages == nil {
		t.Error("expected messages channel to not be nil")
	}
}

func TestChannel_isChannelAllowed(t *testing.T) {
	tests := []struct {
		name            string
		allowedChannels []string
		channelID       string
		expected        bool
	}{
		{
			name:            "empty allowed list allows all",
			allowedChannels: []string{},
			channelID:       "C180",
			expected:        true,
		},
		{
			name:            "channel ID in allowed list",
			allowedChannels: []string{"C180", "C789"},
			channelID:       "C180",
			expected:        true,
		},
		{
			name:            "channel ID not in allowed list",
			allowedChannels: []string{"C789", "C999"},
			channelID:       "C180",
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.SlackConfig{
				Enabled:         true,
				BotToken:        "xoxb-test-token",
				AppToken:        "xapp-test-token",
				AllowedChannels: tt.allowedChannels,
			}
			logger := zap.NewNop()
			ch := New(cfg, logger)

			result := ch.isChannelAllowed(tt.channelID)
			if result != tt.expected {
				t.Errorf("isChannelAllowed(%s) = %v, want %v", tt.channelID, result, tt.expected)
			}
		})
	}
}

func TestChannel_isUserAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedUsers []string
		userID       string
		expected     bool
	}{
		{
			name:         "empty allowed list allows all",
			allowedUsers: []string{},
			userID:       "U180",
			expected:     true,
		},
		{
			name:         "user ID in allowed list",
			allowedUsers: []string{"U180", "U789"},
			userID:       "U180",
			expected:     true,
		},
		{
			name:         "user ID not in allowed list",
			allowedUsers: []string{"U789", "U999"},
			userID:       "U180",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.SlackConfig{
				Enabled:      true,
				BotToken:     "xoxb-test-token",
				AppToken:     "xapp-test-token",
				AllowedUsers: tt.allowedUsers,
			}
			logger := zap.NewNop()
			ch := New(cfg, logger)

			result := ch.isUserAllowed(tt.userID)
			if result != tt.expected {
				t.Errorf("isUserAllowed(%s) = %v, want %v", tt.userID, result, tt.expected)
			}
		})
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stopping a channel that was never started should not error
	err := ch.Stop(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestChannel_Send_NotInitialized(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx := context.Background()
	msg := channel.OutgoingMessage{
		ChatID:  "C180",
		Content: "Hello",
	}

	err := ch.Send(ctx, msg)
	if err == nil {
		t.Error("expected error when sending without initialization")
	}
}

func TestChannel_SendStreaming_NotInitialized(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx := context.Background()
	content := make(chan string)
	done := make(chan struct{})

	close(content) // Close immediately

	err := ch.SendStreaming(ctx, "C180", "", content, done)
	if err == nil {
		t.Error("expected error when streaming without initialization")
	}
}

func TestParseSlackTimestamp(t *testing.T) {
	tests := []struct {
		input    string
		expected int64 // Unix timestamp
	}{
		{"1807890.180", 1807890},
		{"1609459200.000000", 1609459200},
		{"invalid", 0}, // Will return current time, so we just check it doesn't panic
	}

	for _, tt := range tests {
		result := parseSlackTimestamp(tt.input)
		if tt.expected != 0 && result.Unix() != tt.expected {
			t.Errorf("parseSlackTimestamp(%s) = %d, want %d", tt.input, result.Unix(), tt.expected)
		}
	}
}

func TestChannel_Send_URLAttachmentFallsBackToText(t *testing.T) {
	var requests []map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode slack payload: %v", err)
		}
		requests = append(requests, payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"ts":"123.456"}`))
	}))
	defer server.Close()

	ch := New(channel.SlackConfig{Enabled: true}, zap.NewNop())
	ch.client = &slackClient{botToken: "test", baseURL: server.URL, http: server.Client()}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "C123",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeImage,
			URL:  "https://example.com/image.png",
		}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if got := requests[0]["text"]; got != "caption\nhttps://example.com/image.png" {
		t.Fatalf("unexpected fallback text: %v", got)
	}
}

func TestChannel_Send_PassesBlocksMetadata(t *testing.T) {
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode slack payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"ts":"123.456"}`))
	}))
	defer server.Close()

	ch := New(channel.SlackConfig{Enabled: true}, zap.NewNop())
	ch.client = &slackClient{botToken: "test", baseURL: server.URL, http: server.Client()}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "C123",
		Content: "hello",
		Metadata: map[string]interface{}{
			"blocks": []map[string]interface{}{{
				"type": "section",
				"text": map[string]interface{}{"type": "mrkdwn", "text": "*hello*"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	blocks, ok := payload["blocks"].([]interface{})
	if !ok || len(blocks) != 1 {
		t.Fatalf("expected blocks payload, got %#v", payload["blocks"])
	}
}

func TestChannel_Send_BlocksWithoutTextStillSends(t *testing.T) {
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode slack payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"ts":"123.456"}`))
	}))
	defer server.Close()

	ch := New(channel.SlackConfig{Enabled: true}, zap.NewNop())
	ch.client = &slackClient{botToken: "test", baseURL: server.URL, http: server.Client()}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: "C123",
		Metadata: map[string]interface{}{
			"blocks": []map[string]interface{}{{
				"type": "section",
				"text": map[string]interface{}{"type": "mrkdwn", "text": "*hello*"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	blocks, ok := payload["blocks"].([]interface{})
	if !ok || len(blocks) != 1 {
		t.Fatalf("expected blocks payload, got %#v", payload["blocks"])
	}
}

func TestChannel_HandleMessageEvent_MessageHandlerRepliesUseThreadTarget(t *testing.T) {
	var payload map[string]interface{}
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		switch r.URL.Path {
		case "/users.info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"user":{"name":"alice"}}`))
		case "/chat.postMessage":
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("failed to decode slack payload: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"ts":"1710000000.003"}`))
			close(done)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	ch := New(channel.SlackConfig{Enabled: true}, zap.NewNop())
	ch.ctx = context.Background()
	ch.botUserID = "BOT"
	ch.client = &slackClient{botToken: "test", baseURL: server.URL, http: server.Client()}
	ch.SetMessageHandler(func(ctx context.Context, msg channel.Message) (string, error) {
		return "reply", nil
	})

	ch.handleMessageEvent(json.RawMessage(`{"user":"U123","channel":"C123","text":"hello","ts":"1710000000.002","thread_ts":"1710000000.001","channel_type":"channel"}`))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reply send")
	}

	if got := payload["thread_ts"]; got != "1710000000.001" {
		t.Fatalf("thread_ts = %v, want existing thread target", got)
	}
	if got := payload["text"]; got != "reply" {
		t.Fatalf("text = %v, want reply", got)
	}
}

func TestChannel_HandleAppMentionEvent_MessageHandlerRepliesUseThreadTarget(t *testing.T) {
	var payload map[string]interface{}
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		switch r.URL.Path {
		case "/chat.postMessage":
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("failed to decode slack payload: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"ts":"1710000000.003"}`))
			close(done)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	ch := New(channel.SlackConfig{Enabled: true}, zap.NewNop())
	ch.ctx = context.Background()
	ch.client = &slackClient{botToken: "test", baseURL: server.URL, http: server.Client()}
	ch.SetMessageHandler(func(ctx context.Context, msg channel.Message) (string, error) {
		return "reply", nil
	})

	ch.handleAppMentionEvent(json.RawMessage(`{"user":"U123","channel":"C123","text":"<@BOT> hello","ts":"1710000000.002","thread_ts":"1710000000.001"}`))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reply send")
	}

	if got := payload["thread_ts"]; got != "1710000000.001" {
		t.Fatalf("thread_ts = %v, want existing thread target", got)
	}
	if got := payload["text"]; got != "reply" {
		t.Fatalf("text = %v, want reply", got)
	}
}

func TestChannel_HandleMessageEvent_NormalizesMentions(t *testing.T) {
	ch := New(channel.SlackConfig{Enabled: true}, zap.NewNop())
	ch.botUserID = "BOT"

	ch.handleMessageEvent(json.RawMessage(`{"user":"U123","channel":"C123","text":"hello <@U999> and <@U888>","ts":"1710000000.002","channel_type":"channel"}`))

	select {
	case msg := <-ch.Messages():
		mentionIDs, ok := msg.Metadata["mention_ids"].([]string)
		if !ok || len(mentionIDs) != 2 || mentionIDs[0] != "U999" || mentionIDs[1] != "U888" {
			t.Fatalf("mention_ids = %#v", msg.Metadata["mention_ids"])
		}
		mentions, ok := msg.Metadata["mentions"].([]map[string]interface{})
		if !ok || len(mentions) != 2 {
			t.Fatalf("mentions = %#v", msg.Metadata["mentions"])
		}
		if mentions[0]["id"] != "U999" || mentions[0]["type"] != "user" {
			t.Fatalf("first mention = %#v", mentions[0])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for slack message")
	}
}

func TestChannel_SendInteractionToChannel_PreservesActions(t *testing.T) {
	ch := New(channel.SlackConfig{Enabled: true}, zap.NewNop())
	ch.sendInteractionToChannel(&InteractionCallback{
		Type:        "block_actions",
		TriggerID:   "trigger-1",
		CallbackID:  "cb-1",
		ResponseURL: "https://example.com/resp",
		Actions: []InteractionAction{{
			ActionID: "approve",
			BlockID:  "block-1",
			Type:     "button",
			Value:    "yes",
		}},
	})

	select {
	case msg := <-ch.Messages():
		actionIDs, ok := msg.Metadata["action_ids"].([]string)
		if !ok || len(actionIDs) != 1 || actionIDs[0] != "approve" {
			t.Fatalf("action_ids = %#v", msg.Metadata["action_ids"])
		}
		actions, ok := msg.Metadata["actions"].([]map[string]interface{})
		if !ok || len(actions) != 1 {
			t.Fatalf("actions = %#v", msg.Metadata["actions"])
		}
		if actions[0]["action_id"] != "approve" || actions[0]["block_id"] != "block-1" {
			t.Fatalf("first action = %#v", actions[0])
		}
		selected, ok := actions[0]["selected_values"].([]string)
		if !ok || len(selected) != 1 || selected[0] != "yes" {
			t.Fatalf("selected_values = %#v", actions[0]["selected_values"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for slack interaction message")
	}
}
