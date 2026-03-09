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
	}, nil, 0)

	tool := GetWebFetchTool(registry)
	if tool == nil {
		t.Fatal("expected web_fetch tool to be registered")
	}
	if !tool.config.AllowPrivateHosts {
		t.Fatal("expected AllowPrivateHosts to be applied")
	}
	if tool.config.Timeout != 7*time.Second {
		t.Fatalf("timeout = %v, want %v", tool.config.Timeout, 7*time.Second)
	}
}
