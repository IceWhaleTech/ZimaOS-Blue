package server

import (
	"testing"

	"go.uber.org/zap"
)

func TestChannelFactory_CreateWechat_UsesStableChannelID(t *testing.T) {
	f := NewChannelFactory(zap.NewNop())

	ch, err := f.CreateChannel(&ChannelConfig{
		ID:      "wechat",
		Enabled: true,
		Config: map[string]string{
			"corp_id":  "corp-id",
			"agent_id": "agent-id",
			"secret":   "secret",
		},
	})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	if ch == nil {
		t.Fatal("CreateChannel returned nil channel")
	}
	if ch.Name() != "wechat" {
		t.Fatalf("Name = %q, want %q", ch.Name(), "wechat")
	}
	if ch.Type() != "wechat_work" {
		t.Fatalf("Type = %q, want %q", ch.Type(), "wechat_work")
	}
	if got := ch.Info().Metadata["provider"]; got != "wechat_work" {
		t.Fatalf("provider = %v, want %q", got, "wechat_work")
	}
}

func TestChannelFactory_CreateWechatILink_UsesDedicatedChannelID(t *testing.T) {
	f := NewChannelFactory(zap.NewNop())

	ch, err := f.CreateChannel(&ChannelConfig{
		ID:      "wechat_ilink",
		Enabled: true,
		Config: map[string]string{
			"api_base_url": "https://ilink.example.com",
			"bot_token":    "bot-token",
		},
	})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	if ch == nil {
		t.Fatal("CreateChannel returned nil channel")
	}
	if ch.Name() != "wechat_ilink" {
		t.Fatalf("Name = %q, want %q", ch.Name(), "wechat_ilink")
	}
	if ch.Type() != "wechat_ilink" {
		t.Fatalf("Type = %q, want %q", ch.Type(), "wechat_ilink")
	}
	info := ch.Info()
	if got := info.Metadata["api_base_url"]; got != "https://ilink.example.com" {
		t.Fatalf("api_base_url = %v, want %q", got, "https://ilink.example.com")
	}
}
