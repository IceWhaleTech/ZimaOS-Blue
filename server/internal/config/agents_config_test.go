package config

import (
	"testing"
	"time"
)

func TestDefaultAgentsConfig(t *testing.T) {
	cfg := DefaultAgentsConfig()
	if cfg.Defaults.ToolPolicy.Profile != "coding" {
		t.Fatalf("expected defaults profile coding, got %q", cfg.Defaults.ToolPolicy.Profile)
	}
	if cfg.Defaults.Subagents.MaxParallel != 3 {
		t.Fatalf("expected defaults subagent max_parallel 3, got %d", cfg.Defaults.Subagents.MaxParallel)
	}
	if cfg.Defaults.Subagents.Timeout != 5*time.Minute {
		t.Fatalf("expected defaults subagent timeout 5m, got %v", cfg.Defaults.Subagents.Timeout)
	}
	if len(cfg.List) == 0 || cfg.List[0].ID != "main" {
		t.Fatalf("expected main agent in defaults, got %#v", cfg.List)
	}
}
