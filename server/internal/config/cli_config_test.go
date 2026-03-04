package config

import (
	"testing"
	"time"
)

func TestDefaultClaudeCodeCLIConfig(t *testing.T) {
	cfg := DefaultClaudeCodeCLIConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled to be true by default")
	}

	if !cfg.Install.VerifyChecksum {
		t.Error("expected VerifyChecksum to be true by default")
	}

	if !cfg.Features.Skills {
		t.Error("expected Skills to be true by default")
	}

	if !cfg.Features.ToolCalling {
		t.Error("expected ToolCalling to be true by default")
	}

	if !cfg.Features.FileOperations {
		t.Error("expected FileOperations to be true by default")
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

	if cfg.DetectionTimeout != 5*time.Second {
		t.Errorf("expected DetectionTimeout 5s, got %v", cfg.DetectionTimeout)
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

	if !cfg.Adapters.CLIProxy.Enabled {
		t.Error("expected CLIProxy to be enabled by default")
	}

	if !cfg.Adapters.CCNexus.Enabled {
		t.Error("expected CCNexus to be enabled by default")
	}

	if cfg.Adapters.CCNexus.SchemaMapping != "auto" {
		t.Errorf("expected SchemaMapping 'auto', got '%s'", cfg.Adapters.CCNexus.SchemaMapping)
	}

	if cfg.WebSearch.Provider != "duckduckgo" {
		t.Errorf("expected WebSearch.Provider 'duckduckgo', got %q", cfg.WebSearch.Provider)
	}
	if len(cfg.WebSearch.Providers) != 1 || cfg.WebSearch.Providers[0] != "duckduckgo" {
		t.Errorf("expected WebSearch.Providers ['duckduckgo'], got %#v", cfg.WebSearch.Providers)
	}
	if cfg.WebSearch.MaxResults != 5 {
		t.Errorf("expected WebSearch.MaxResults 5, got %d", cfg.WebSearch.MaxResults)
	}
	if cfg.WebSearch.Timeout != 30*time.Second {
		t.Errorf("expected WebSearch.Timeout 30s, got %v", cfg.WebSearch.Timeout)
	}
	if cfg.WebSearch.Region != "wt-wt" {
		t.Errorf("expected WebSearch.Region 'wt-wt', got %q", cfg.WebSearch.Region)
	}
}

func TestCLIFeaturesConfig(t *testing.T) {
	cfg := CLIFeaturesConfig{
		Skills:           true,
		ToolCalling:      true,
		FileOperations:   true,
		TerminalCommands: true,
		MCPIntegration:   true,
		AgentMode:        true,
		ProjectContext:   true,
	}

	// All features should be enabled
	if !cfg.Skills {
		t.Error("expected Skills to be true")
	}
	if !cfg.ToolCalling {
		t.Error("expected ToolCalling to be true")
	}
	if !cfg.FileOperations {
		t.Error("expected FileOperations to be true")
	}
	if !cfg.TerminalCommands {
		t.Error("expected TerminalCommands to be true")
	}
	if !cfg.MCPIntegration {
		t.Error("expected MCPIntegration to be true")
	}
	if !cfg.AgentMode {
		t.Error("expected AgentMode to be true")
	}
	if !cfg.ProjectContext {
		t.Error("expected ProjectContext to be true")
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
