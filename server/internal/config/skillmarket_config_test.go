package config

import (
	"testing"
	"time"
)

func TestDefaultSkillMarketConfigUsesDailyRefreshCadence(t *testing.T) {
	cfg := DefaultSkillMarketConfig()
	if cfg == nil {
		t.Fatal("DefaultSkillMarketConfig() returned nil")
	}
	if cfg.CrawlIncrementalInterval != 24*time.Hour {
		t.Fatalf("CrawlIncrementalInterval = %s, want 24h", cfg.CrawlIncrementalInterval)
	}
	if cfg.CrawlFullInterval != 24*time.Hour {
		t.Fatalf("CrawlFullInterval = %s, want 24h", cfg.CrawlFullInterval)
	}
	if cfg.UpdateCheckInterval != 24*time.Hour {
		t.Fatalf("UpdateCheckInterval = %s, want 24h", cfg.UpdateCheckInterval)
	}
	if len(cfg.ClawHubMirrorBaseURLs) != 0 {
		t.Fatalf("ClawHubMirrorBaseURLs = %v, want empty by default", cfg.ClawHubMirrorBaseURLs)
	}
}
