package config

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
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
	if len(cfg.DiscoveryPageURLs) != len(defaultSkillMarketDiscoveryPageURLs) {
		t.Fatalf(
			"DiscoveryPageURLs count = %d, want %d",
			len(cfg.DiscoveryPageURLs),
			len(defaultSkillMarketDiscoveryPageURLs),
		)
	}
	for i, expected := range defaultSkillMarketDiscoveryPageURLs {
		if cfg.DiscoveryPageURLs[i] != expected {
			t.Fatalf(
				"DiscoveryPageURLs[%d] = %q, want %q",
				i,
				cfg.DiscoveryPageURLs[i],
				expected,
			)
		}
	}
}

func TestSkillMarketConfigAcceptsLegacySeedURLsInYAML(t *testing.T) {
	cfg := DefaultSkillMarketConfig()
	raw := []byte("seed_urls:\n  - https://example.com/legacy\n")

	if err := yaml.Unmarshal(raw, cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if len(cfg.DiscoveryPageURLs) != 1 || cfg.DiscoveryPageURLs[0] != "https://example.com/legacy" {
		t.Fatalf("DiscoveryPageURLs = %v, want legacy seed_urls value", cfg.DiscoveryPageURLs)
	}
}

func TestSkillMarketConfigPrefersDiscoveryPageURLsOverLegacySeedURLsInYAML(t *testing.T) {
	cfg := DefaultSkillMarketConfig()
	raw := []byte(
		"discovery_page_urls:\n" +
			"  - https://example.com/current\n" +
			"seed_urls:\n" +
			"  - https://example.com/legacy\n",
	)

	if err := yaml.Unmarshal(raw, cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if len(cfg.DiscoveryPageURLs) != 1 || cfg.DiscoveryPageURLs[0] != "https://example.com/current" {
		t.Fatalf("DiscoveryPageURLs = %v, want canonical discovery_page_urls value", cfg.DiscoveryPageURLs)
	}
}

func TestSkillMarketConfigAcceptsLegacySeedURLsInJSON(t *testing.T) {
	cfg := DefaultSkillMarketConfig()
	raw := []byte(`{"SeedURLs":["https://example.com/legacy"]}`)

	if err := json.Unmarshal(raw, cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(cfg.DiscoveryPageURLs) != 1 || cfg.DiscoveryPageURLs[0] != "https://example.com/legacy" {
		t.Fatalf("DiscoveryPageURLs = %v, want legacy SeedURLs value", cfg.DiscoveryPageURLs)
	}
}

func TestSkillMarketConfigAcceptsDiscoveryPageURLsInJSONSnakeCase(t *testing.T) {
	cfg := DefaultSkillMarketConfig()
	raw := []byte(`{"discovery_page_urls":["https://example.com/current"]}`)

	if err := json.Unmarshal(raw, cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(cfg.DiscoveryPageURLs) != 1 || cfg.DiscoveryPageURLs[0] != "https://example.com/current" {
		t.Fatalf("DiscoveryPageURLs = %v, want discovery_page_urls value", cfg.DiscoveryPageURLs)
	}
}

func TestSkillMarketConfigPrefersDiscoveryPageURLsOverLegacySeedURLsInJSON(t *testing.T) {
	cfg := DefaultSkillMarketConfig()
	raw := []byte(
		`{"discovery_page_urls":["https://example.com/current"],"SeedURLs":["https://example.com/legacy"]}`,
	)

	if err := json.Unmarshal(raw, cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(cfg.DiscoveryPageURLs) != 1 || cfg.DiscoveryPageURLs[0] != "https://example.com/current" {
		t.Fatalf("DiscoveryPageURLs = %v, want canonical discovery_page_urls value", cfg.DiscoveryPageURLs)
	}
}
