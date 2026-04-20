package config

import (
	"testing"
	"time"
)

func TestDefaultAgentCoreConfig(t *testing.T) {
	cfg := DefaultAgentCoreConfig()

	if cfg.WorkspaceDir != "." {
		t.Errorf("expected WorkspaceDir '.', got %q", cfg.WorkspaceDir)
	}
}

func TestDefaultLLMProvidersConfig(t *testing.T) {
	cfg := DefaultLLMProvidersConfig()

	if cfg.DefaultProvider != "anthropic" {
		t.Errorf("expected DefaultProvider 'anthropic', got '%s'", cfg.DefaultProvider)
	}

	if !cfg.Providers.Anthropic.Enabled {
		t.Error("expected Anthropic to be enabled by default")
	}

	if !cfg.Providers.OpenAI.Enabled {
		t.Error("expected OpenAI to be enabled by default")
	}

	if !cfg.Providers.Ollama.Enabled {
		t.Error("expected Ollama to be enabled by default")
	}

	if !cfg.Providers.Ollama.AutoDetect {
		t.Error("expected Ollama AutoDetect to be true by default")
	}

	if !cfg.Fallback.Enabled {
		t.Error("expected Fallback to be enabled by default")
	}

	if len(cfg.Fallback.Chain) != 3 {
		t.Errorf("expected 3 providers in fallback chain, got %d", len(cfg.Fallback.Chain))
	}
}

func TestDefaultFirstRunConfig(t *testing.T) {
	cfg := DefaultFirstRunConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled to be true by default")
	}

	if !cfg.ShowProviderDetection {
		t.Error("expected ShowProviderDetection to be true by default")
	}

	if !cfg.ShowCLIDownload {
		t.Error("expected ShowCLIDownload to be true by default")
	}

	if !cfg.AllowSkip {
		t.Error("expected AllowSkip to be true by default")
	}

	if !cfg.RecommendCLI {
		t.Error("expected RecommendCLI to be true by default")
	}
}

func TestDefaultCCSwitchConfig(t *testing.T) {
	cfg := DefaultCCSwitchConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled to be true by default")
	}

	if !cfg.SyncProfiles {
		t.Error("expected SyncProfiles to be true by default")
	}
}

func TestDefaultStatisticsConfig(t *testing.T) {
	cfg := DefaultStatisticsConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled to be true by default")
	}

	if !cfg.OptInRequired {
		t.Error("expected OptInRequired to be true by default")
	}

	if cfg.RetentionDays != 90 {
		t.Errorf("expected RetentionDays 90, got %d", cfg.RetentionDays)
	}
}

func TestDefaultToolCallingConfig(t *testing.T) {
	cfg := DefaultToolCallingConfig()

	if !cfg.AutoDetect {
		t.Error("expected AutoDetect to be true by default")
	}

	if cfg.DetectionTimeout != 5*time.Minute {
		t.Errorf("expected DetectionTimeout 5m, got %v", cfg.DetectionTimeout)
	}
	if cfg.SmartSelection {
		t.Error("expected SmartSelection to be false by default")
	}
	if cfg.SkillRerankEnabled {
		t.Error("expected SkillRerankEnabled to be false by default")
	}
	if cfg.SkillRerankModel != "cross-encoder/ms-marco-MiniLM-L6-v2" {
		t.Errorf("expected SkillRerankModel cross-encoder/ms-marco-MiniLM-L6-v2, got %q", cfg.SkillRerankModel)
	}
	if cfg.SkillRerankONNXEnabled {
		t.Error("expected SkillRerankONNXEnabled to be false by default")
	}
	if cfg.SkillRerankONNXAutoDownload {
		t.Error("expected SkillRerankONNXAutoDownload to be false by default")
	}
	if cfg.ToolRouterDynamicExposure {
		t.Error("expected ToolRouterDynamicExposure to be false by default")
	}
	if cfg.ToolRouterSchemaCompression {
		t.Error("expected ToolRouterSchemaCompression to be false by default")
	}

	if !cfg.Adapters.CLIProxy.Enabled {
		t.Error("expected CLIProxy to be enabled by default")
	}

	if !cfg.Adapters.CCNexus.Enabled {
		t.Error("expected CCNexus to be enabled by default")
	}

	if cfg.Adapters.CCNexus.SchemaMapping != "auto" {
		t.Errorf("expected SchemaMapping 'auto', got '%s'", cfg.Adapters.CCNexus.SchemaMapping)
	}

	if cfg.Profile != "" {
		t.Errorf("expected Profile to be empty by default, got %q", cfg.Profile)
	}
	if len(cfg.Profiles["coding"]) == 0 {
		t.Fatalf("expected coding profile entries to be populated")
	}
	if !containsString(cfg.Profiles["minimal"], "sessions") {
		t.Fatalf("expected minimal profile to include sessions, got %#v", cfg.Profiles["minimal"])
	}
	if !containsString(cfg.Profiles["coding"], "group:research") {
		t.Fatalf("expected coding profile to include group:research, got %#v", cfg.Profiles["coding"])
	}
	if !containsString(cfg.Profiles["coding"], "group:web") {
		t.Fatalf("expected coding profile to include group:web, got %#v", cfg.Profiles["coding"])
	}
	if len(cfg.Groups["group:runtime"]) == 0 {
		t.Fatalf("expected group:runtime entries to be populated")
	}
	if !containsString(cfg.Groups["group:fs"], "read") || !containsString(cfg.Groups["group:fs"], "write") {
		t.Fatalf("expected group:fs to include read/write, got %#v", cfg.Groups["group:fs"])
	}
	if containsString(cfg.Groups["group:fs"], "rg") {
		t.Fatalf("did not expect group:fs to include rg, got %#v", cfg.Groups["group:fs"])
	}
	if containsString(cfg.Groups["group:fs"], "apply_patch") {
		t.Fatalf("did not expect group:fs to include apply_patch, got %#v", cfg.Groups["group:fs"])
	}
	if got := cfg.Groups["group:sessions"]; len(got) != 1 || got[0] != "sessions" {
		t.Fatalf("expected group:sessions to expose only sessions, got %#v", got)
	}
	if got := cfg.Groups["group:memory"]; len(got) != 1 || got[0] != "memory" {
		t.Fatalf("expected group:memory to expose only memory, got %#v", got)
	}
	if got := cfg.Groups["group:web"]; len(got) != 1 || got[0] != "web" {
		t.Fatalf("expected group:web to expose only web, got %#v", got)
	}
	if len(cfg.Groups["group:research"]) == 0 {
		t.Fatalf("expected group:research entries to be populated")
	}
	if got := cfg.Groups["group:research"]; len(got) != 1 || got[0] != "research" {
		t.Fatalf("expected group:research to expose only research, got %#v", got)
	}

	if cfg.WebSearch.Provider != "duckduckgo" {
		t.Errorf("expected WebSearch.Provider 'duckduckgo', got %q", cfg.WebSearch.Provider)
	}
	if len(cfg.WebSearch.Providers) != 2 || cfg.WebSearch.Providers[0] != "duckduckgo" || cfg.WebSearch.Providers[1] != "bing" {
		t.Errorf("expected WebSearch.Providers ['duckduckgo', 'bing'], got %#v", cfg.WebSearch.Providers)
	}
	if cfg.WebSearch.MaxResults != 5 {
		t.Errorf("expected WebSearch.MaxResults 5, got %d", cfg.WebSearch.MaxResults)
	}
	if cfg.WebSearch.Timeout != 5*time.Minute {
		t.Errorf("expected WebSearch.Timeout 5m, got %v", cfg.WebSearch.Timeout)
	}
	if cfg.WebSearch.Region != "wt-wt" {
		t.Errorf("expected WebSearch.Region 'wt-wt', got %q", cfg.WebSearch.Region)
	}
	if cfg.WebSearch.BrowserFallback.Enabled == nil || !*cfg.WebSearch.BrowserFallback.Enabled {
		t.Fatalf("expected WebSearch.BrowserFallback.Enabled true by default")
	}
	if cfg.WebSearch.BrowserFallback.Engine != "bing" {
		t.Errorf("expected WebSearch.BrowserFallback.Engine 'bing', got %q", cfg.WebSearch.BrowserFallback.Engine)
	}
	if cfg.WebSearch.BrowserFallback.TriggerMode != "quality_or_failure" {
		t.Errorf("expected WebSearch.BrowserFallback.TriggerMode 'quality_or_failure', got %q", cfg.WebSearch.BrowserFallback.TriggerMode)
	}
	if cfg.WebSearch.BrowserFallback.MaxBrowserRetries != 1 {
		t.Errorf("expected WebSearch.BrowserFallback.MaxBrowserRetries 1, got %d", cfg.WebSearch.BrowserFallback.MaxBrowserRetries)
	}
	if cfg.WebFetch.Timeout != 5*time.Minute {
		t.Errorf("expected WebFetch.Timeout 5m, got %v", cfg.WebFetch.Timeout)
	}
	if !cfg.WebFetch.HTTPNativeEnabled {
		t.Error("expected WebFetch.HTTPNativeEnabled true by default")
	}
	if cfg.WebFetch.HTTPNativeLibrary != "" {
		t.Errorf("expected WebFetch.HTTPNativeLibrary empty by default, got %q", cfg.WebFetch.HTTPNativeLibrary)
	}
	if len(cfg.WebFetch.HTTPNativePreferHosts) != 0 {
		t.Errorf("expected WebFetch.HTTPNativePreferHosts empty by default, got %#v", cfg.WebFetch.HTTPNativePreferHosts)
	}
	if cfg.WebFetch.FirecrawlTimeout != 5*time.Minute {
		t.Errorf("expected WebFetch.FirecrawlTimeout 5m, got %v", cfg.WebFetch.FirecrawlTimeout)
	}
	if cfg.WebFetch.JinaReaderEnabled {
		t.Error("expected WebFetch.JinaReaderEnabled false by default")
	}
	if cfg.WebFetch.JinaReaderTimeout != 5*time.Minute {
		t.Errorf("expected WebFetch.JinaReaderTimeout 5m, got %v", cfg.WebFetch.JinaReaderTimeout)
	}
	if len(cfg.WebFetch.ProxyFetcherProviders) != 2 {
		t.Errorf("expected WebFetch.ProxyFetcherProviders to contain firecrawl and jina_reader, got %v", cfg.WebFetch.ProxyFetcherProviders)
	}
	if !cfg.Ripgrep.Enabled {
		t.Error("expected Ripgrep.Enabled to be true by default")
	}
	if !cfg.Ripgrep.AutoDownload {
		t.Error("expected Ripgrep.AutoDownload to be true by default")
	}
	if !cfg.Ripgrep.AllowSystemBinary {
		t.Error("expected Ripgrep.AllowSystemBinary to be true by default")
	}
	if cfg.Ripgrep.CacheDir != "" {
		t.Errorf("expected Ripgrep.CacheDir empty by default, got %q", cfg.Ripgrep.CacheDir)
	}
	if len(cfg.Ripgrep.MirrorBaseURLs) != 3 {
		t.Fatalf("expected 3 ripgrep mirror base urls, got %#v", cfg.Ripgrep.MirrorBaseURLs)
	}
}

func TestProviderOverrideConfig(t *testing.T) {
	cfg := DefaultToolCallingConfig()

	ollamaOverride, ok := cfg.ProviderOverrides["ollama"]
	if !ok {
		t.Fatal("expected ollama override to exist")
	}

	if ollamaOverride.ToolCalling != "adapter" {
		t.Errorf("expected ollama ToolCalling 'adapter', got '%s'", ollamaOverride.ToolCalling)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
