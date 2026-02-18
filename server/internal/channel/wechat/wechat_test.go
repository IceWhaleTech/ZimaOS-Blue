package wechat

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Name(t *testing.T) {
	cfg := channel.WeChatWorkConfig{
		Enabled: true,
		CorpID:  "test-corp-id",
		AgentID: "test-agent-id",
		Secret:  "test-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Name() != "wechat_work" {
		t.Errorf("expected name 'wechat_work', got %s", ch.Name())
	}
}

func TestChannel_Type(t *testing.T) {
	cfg := channel.WeChatWorkConfig{
		Enabled: true,
		CorpID:  "test-corp-id",
		AgentID: "test-agent-id",
		Secret:  "test-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Type() != "wechat_work" {
		t.Errorf("expected type 'wechat_work', got %s", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	cfg := channel.WeChatWorkConfig{
		Enabled: true,
		CorpID:  "test-corp-id",
		AgentID: "test-agent-id",
		Secret:  "test-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	info := ch.Info()
	if info.Name != "wechat_work" {
		t.Errorf("expected name 'wechat_work', got %s", info.Name)
	}
	if info.Type != "wechat_work" {
		t.Errorf("expected type 'wechat_work', got %s", info.Type)
	}
	if info.Status != channel.StatusDisconnected {
		t.Errorf("expected status 'disconnected', got %s", info.Status)
	}
	if !info.Enabled {
		t.Error("expected enabled to be true")
	}
	if info.Metadata["corp_id"] != "test-corp-id" {
		t.Errorf("expected corp_id 'test-corp-id', got %v", info.Metadata["corp_id"])
	}
	if info.Metadata["agent_id"] != "test-agent-id" {
		t.Errorf("expected agent_id 'test-agent-id', got %v", info.Metadata["agent_id"])
	}
}

func TestChannel_IsConnected(t *testing.T) {
	cfg := channel.WeChatWorkConfig{
		Enabled: true,
		CorpID:  "test-corp-id",
		AgentID: "test-agent-id",
		Secret:  "test-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("expected channel to not be connected initially")
	}
}

func TestChannel_Messages(t *testing.T) {
	cfg := channel.WeChatWorkConfig{
		Enabled: true,
		CorpID:  "test-corp-id",
		AgentID: "test-agent-id",
		Secret:  "test-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	messages := ch.Messages()
	if messages == nil {
		t.Error("expected messages channel to not be nil")
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	cfg := channel.WeChatWorkConfig{
		Enabled: true,
		CorpID:  "test-corp-id",
		AgentID: "test-agent-id",
		Secret:  "test-secret",
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

func TestChannel_verifySignature(t *testing.T) {
	cfg := channel.WeChatWorkConfig{
		Enabled: true,
		CorpID:  "test-corp-id",
		AgentID: "test-agent-id",
		Secret:  "test-secret",
		Token:   "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	// Test with known values
	// Note: This is a simplified test. In production, you'd use actual WeChat signatures.
	timestamp := "1807890"
	nonce := "test-nonce"
	encrypt := "test-encrypt"

	// The signature should be SHA1 of sorted(token, timestamp, nonce, encrypt)
	// This test just verifies the function doesn't panic
	_ = ch.verifySignature("invalid-signature", timestamp, nonce, encrypt)
}

func TestChannel_convertMessage(t *testing.T) {
	cfg := channel.WeChatWorkConfig{
		Enabled: true,
		CorpID:  "test-corp-id",
		AgentID: "test-agent-id",
		Secret:  "test-secret",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	tests := []struct {
		name     string
		msg      *wechatMessage
		expected channel.MessageType
	}{
		{
			name: "text message",
			msg: &wechatMessage{
				MsgType:      "text",
				Content:      "Hello",
				FromUserName: "user123",
				MsgId:        "msg123",
				CreateTime:   1807890,
			},
			expected: channel.MessageTypeText,
		},
		{
			name: "image message",
			msg: &wechatMessage{
				MsgType:      "image",
				FromUserName: "user123",
				MsgId:        "msg123",
				CreateTime:   1807890,
				MediaId:      "media123",
			},
			expected: channel.MessageTypeImage,
		},
		{
			name: "voice message",
			msg: &wechatMessage{
				MsgType:      "voice",
				FromUserName: "user123",
				MsgId:        "msg123",
				CreateTime:   1807890,
				MediaId:      "media123",
			},
			expected: channel.MessageTypeAudio,
		},
		{
			name: "video message",
			msg: &wechatMessage{
				MsgType:      "video",
				FromUserName: "user123",
				MsgId:        "msg123",
				CreateTime:   1807890,
				MediaId:      "media123",
			},
			expected: channel.MessageTypeVideo,
		},
		{
			name: "file message",
			msg: &wechatMessage{
				MsgType:      "file",
				FromUserName: "user123",
				MsgId:        "msg123",
				CreateTime:   1807890,
				MediaId:      "media123",
			},
			expected: channel.MessageTypeFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ch.convertMessage(tt.msg)
			if result.Type != tt.expected {
				t.Errorf("expected type %s, got %s", tt.expected, result.Type)
			}
			if result.ChannelName != "wechat_work" {
				t.Errorf("expected channel name 'wechat_work', got %s", result.ChannelName)
			}
			if result.ID != tt.msg.MsgId {
				t.Errorf("expected ID %s, got %s", tt.msg.MsgId, result.ID)
			}
		})
	}
}
