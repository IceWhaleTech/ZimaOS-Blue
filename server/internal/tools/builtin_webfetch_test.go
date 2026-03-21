package tools

import (
	"testing"
	"time"
)

func TestRegisterBuiltinToolsWithConfig_AppliesWebFetchConfig(t *testing.T) {
	registry := NewRegistry()
	RegisterBuiltinToolsWithConfig(registry, WebSearchConfig{}, WebFetchConfig{
		AllowPrivateHosts: true,
		Timeout:           7 * time.Second,
		FirecrawlTimeout:  11 * time.Second,
	}, nil, 0)

	tool := GetWebFetchTool(registry)
	if tool == nil {
		t.Fatal("expected web_fetch tool to be registered")
	}
	if !registry.IsDisabled("web_fetch") {
		t.Fatal("expected legacy web_fetch tool to be hidden")
	}
	if GetWebTool(registry) == nil {
		t.Fatal("expected unified web tool to be registered")
	}
	if !tool.config.AllowPrivateHosts {
		t.Fatal("expected AllowPrivateHosts to be applied")
	}
	if tool.config.Timeout != 7*time.Second {
		t.Fatalf("timeout = %v, want %v", tool.config.Timeout, 7*time.Second)
	}
	if tool.config.FirecrawlTimeout != 11*time.Second {
		t.Fatalf("firecrawl timeout = %v, want %v", tool.config.FirecrawlTimeout, 11*time.Second)
	}
}
