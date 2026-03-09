package tools

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestToolPolicyResolver_GlobalProfile(t *testing.T) {
	cfg := &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}
	cfg.ToolCalling.Profile = "coding"
	resolver := NewToolPolicyResolver(cfg)
	defs := []ToolDefinition{
		{Name: "read"},
		{Name: "exec"},
		{Name: "message"},
		{Name: "session_status"},
	}
	filtered := resolver.Filter(ToolPolicyRequest{}, defs)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 tools after coding profile, got %d (%#v)", len(filtered), filtered)
	}
	for _, def := range filtered {
		if def.Name == "message" {
			t.Fatalf("message should be filtered from coding profile")
		}
	}
}

func TestToolPolicyResolver_AgentOverride(t *testing.T) {
	cfg := &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}
	cfg.ToolCalling.Profile = "full"
	cfg.Agents.List = []config.AgentConfig{{
		ID:      "support",
		Enabled: true,
		ToolPolicy: config.ToolPolicyConfig{
			Profile: "messaging",
		},
	}}
	resolver := NewToolPolicyResolver(cfg)
	defs := []ToolDefinition{{Name: "message"}, {Name: "sessions_list"}, {Name: "exec"}}
	filtered := resolver.Filter(ToolPolicyRequest{AgentID: "support"}, defs)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 tools after messaging profile, got %d", len(filtered))
	}
	for _, def := range filtered {
		if def.Name == "exec" {
			t.Fatalf("exec should be filtered from support agent")
		}
	}
}

func TestToolPolicyResolver_ByProvider(t *testing.T) {
	cfg := &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}
	cfg.ToolCalling.ByProvider = map[string]config.ToolPolicyConfig{
		"gpt-5": {Deny: []string{"browser"}},
	}
	resolver := NewToolPolicyResolver(cfg)
	defs := []ToolDefinition{{Name: "browser"}, {Name: "web_search"}}
	filtered := resolver.Filter(ToolPolicyRequest{Model: "gpt-5"}, defs)
	if len(filtered) != 1 || filtered[0].Name != "web_search" {
		t.Fatalf("unexpected provider-filtered tools: %#v", filtered)
	}
}
