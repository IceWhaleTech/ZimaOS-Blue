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
		{Name: "file_read"},
		{Name: "exec"},
		{Name: "message"},
		{Name: "sessions"},
		{Name: "research_run"},
	}
	filtered := resolver.Filter(ToolPolicyRequest{}, defs)
	if len(filtered) != 4 {
		t.Fatalf("expected 4 tools after coding profile, got %d (%#v)", len(filtered), filtered)
	}
	hasResearch := false
	for _, def := range filtered {
		if def.Name == "message" {
			t.Fatalf("message should be filtered from coding profile")
		}
		if def.Name == "research_run" {
			hasResearch = true
		}
	}
	if !hasResearch {
		t.Fatalf("research_run should be allowed by coding profile: %#v", filtered)
	}
}

func TestToolPolicyResolver_DefaultChatDirectAllowlist(t *testing.T) {
	cfg := &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}
	resolver := NewToolPolicyResolver(cfg)
	defs := []ToolDefinition{
		{Name: "calendar"},
		{Name: "email"},
		{Name: "file_read"},
		{Name: "file_write"},
		{Name: "image_generation"},
		{Name: "memory"},
		{Name: "pdf"},
		{Name: "browser"},
		{Name: "research_run"},
		{Name: "sessions"},
		{Name: "web"},
		{Name: "apply_patch"},
		{Name: "write_begin"},
	}
	filtered := resolver.Filter(ToolPolicyRequest{RouteKind: ToolRouteKindChat}, defs)
	if len(filtered) != 11 {
		t.Fatalf("expected 11 tools after expanded default chat allowlist, got %d (%#v)", len(filtered), filtered)
	}
	allowed := map[string]bool{
		"browser": true, "calendar": true, "email": true, "file_read": true, "file_write": true,
		"image_generation": true, "memory": true, "pdf": true, "research_run": true, "sessions": true, "web": true,
	}
	for _, def := range filtered {
		if !allowed[def.Name] {
			t.Fatalf("unexpected tool %q after default chat allowlist", def.Name)
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
	defs := []ToolDefinition{{Name: "message"}, {Name: "sessions"}, {Name: "exec"}}
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
	defs := []ToolDefinition{{Name: "browser"}, {Name: "web"}}
	filtered := resolver.Filter(ToolPolicyRequest{Model: "gpt-5"}, defs)
	if len(filtered) != 1 || filtered[0].Name != "web" {
		t.Fatalf("unexpected provider-filtered tools: %#v", filtered)
	}
}

func TestToolPolicyResolver_ByProviderID(t *testing.T) {
	cfg := &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}
	cfg.ToolCalling.ByProvider = map[string]config.ToolPolicyConfig{
		"provider-123/gpt-5": {Allow: []string{"browser"}},
	}
	resolver := NewToolPolicyResolver(cfg)
	defs := []ToolDefinition{{Name: "browser"}, {Name: "web"}}
	filtered := resolver.Filter(ToolPolicyRequest{ProviderID: "provider-123", Model: "gpt-5"}, defs)
	if len(filtered) != 1 || filtered[0].Name != "browser" {
		t.Fatalf("unexpected provider-id filtered tools: %#v", filtered)
	}
}
