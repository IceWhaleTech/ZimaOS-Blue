package main

import (
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestBuildBuiltinToolConfigs_UsesWebFetchOverrides(t *testing.T) {
	cfg := &config.Config{}
	cfg.ToolCalling.WebSearch.Provider = "duckduckgo"
	cfg.ToolCalling.WebSearch.MaxResults = 8
	cfg.ToolCalling.WebFetch.AllowPrivateHosts = true
	cfg.ToolCalling.WebFetch.Timeout = 17 * time.Second

	webSearchCfg, webFetchCfg := buildBuiltinToolConfigs(cfg)
	if webSearchCfg.Provider != "duckduckgo" {
		t.Fatalf("provider = %q, want duckduckgo", webSearchCfg.Provider)
	}
	if webSearchCfg.MaxResults != 8 {
		t.Fatalf("max_results = %d, want 8", webSearchCfg.MaxResults)
	}
	if !webFetchCfg.AllowPrivateHosts {
		t.Fatal("expected AllowPrivateHosts to be true")
	}
	if webFetchCfg.Timeout != 17*time.Second {
		t.Fatalf("timeout = %v, want %v", webFetchCfg.Timeout, 17*time.Second)
	}
}
