package config

import (
	"time"
)

// ClaudeCodeCLIConfig holds Claude Code CLI configuration (v0.10.3).
// This is separated from LLM provider configuration.
// When disabled, CLI-backed features are unavailable (for example skills via
// `blue` subcommands), while native server capabilities may still work.
type ClaudeCodeCLIConfig struct {
	// Enabled is the master switch for CLI integration (strongly recommended to keep enabled)
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Install holds installation settings
	Install CLIInstallConfig `yaml:"install" json:"install"`

	// Download holds download settings
	Download CLIDownloadConfig `yaml:"download" json:"download"`

	// Features holds feature toggles (only effective when Enabled=true)
	Features CLIFeaturesConfig `yaml:"features" json:"features"`

	// Backend holds CLI backend settings (for advanced users)
	Backend CLIBackendConfig `yaml:"backend" json:"backend"`
}

// CLIInstallConfig holds CLI installation settings.
type CLIInstallConfig struct {
	// Path is the installation path (empty = auto-detect)
	Path string `yaml:"path" json:"path"`

	// AutoUpdate enables automatic updates
	AutoUpdate bool `yaml:"auto_update" json:"auto_update"`

	// VerifyChecksum enables checksum verification
	VerifyChecksum bool `yaml:"verify_checksum" json:"verify_checksum"`
}

// CLIDownloadConfig holds CLI download settings.
type CLIDownloadConfig struct {
	// BaseURL is the download base URL
	BaseURL string `yaml:"base_url" json:"base_url"`

	// CacheDir is the cache directory (empty = default)
	CacheDir string `yaml:"cache_dir" json:"cache_dir"`
}

// CLIFeaturesConfig holds CLI feature toggles.
type CLIFeaturesConfig struct {
	// Skills enables skills functionality
	Skills bool `yaml:"skills" json:"skills"`

	// ToolCalling enables tool calling functionality
	ToolCalling bool `yaml:"tool_calling" json:"tool_calling"`

	// FileOperations enables file operations
	FileOperations bool `yaml:"file_operations" json:"file_operations"`

	// TerminalCommands enables terminal command execution
	TerminalCommands bool `yaml:"terminal_commands" json:"terminal_commands"`

	// MCPIntegration enables MCP integration
	MCPIntegration bool `yaml:"mcp_integration" json:"mcp_integration"`

	// AgentMode enables agent mode
	AgentMode bool `yaml:"agent_mode" json:"agent_mode"`

	// ProjectContext enables project context
	ProjectContext bool `yaml:"project_context" json:"project_context"`
}

// CLIBackendConfig holds CLI backend configuration (for advanced users).
type CLIBackendConfig struct {
	// Command is the CLI command to execute
	Command string `yaml:"command" json:"command"`

	// WorkspaceDir is the workspace directory
	WorkspaceDir string `yaml:"workspace_dir" json:"workspace_dir"`

	// DefaultModel is the default model to use
	DefaultModel string `yaml:"default_model" json:"default_model"`

	// Timeout is the command timeout
	Timeout string `yaml:"timeout" json:"timeout"`

	// SessionTTL is the session time-to-live
	SessionTTL string `yaml:"session_ttl" json:"session_ttl"`

	// Args are the default command arguments
	Args []string `yaml:"args" json:"args"`

	// ResumeArgs are the arguments for resuming a session
	ResumeArgs []string `yaml:"resume_args" json:"resume_args"`

	// Output is the output format
	Output string `yaml:"output" json:"output"`

	// ResumeOutput is the output format for resume
	ResumeOutput string `yaml:"resume_output" json:"resume_output"`

	// Input is the input mode
	Input string `yaml:"input" json:"input"`

	// MaxPromptArgChars is the maximum prompt argument characters
	MaxPromptArgChars int `yaml:"max_prompt_arg_chars" json:"max_prompt_arg_chars"`

	// ModelArg is the model argument flag
	ModelArg string `yaml:"model_arg" json:"model_arg"`

	// ModelAliases maps model aliases to actual model names
	ModelAliases map[string]string `yaml:"model_aliases" json:"model_aliases"`

	// SessionArg is the session argument flag
	SessionArg string `yaml:"session_arg" json:"session_arg"`

	// SessionMode is the session mode
	SessionMode string `yaml:"session_mode" json:"session_mode"`

	// SystemPromptArg is the system prompt argument flag
	SystemPromptArg string `yaml:"system_prompt_arg" json:"system_prompt_arg"`

	// SystemPromptMode is the system prompt mode
	SystemPromptMode string `yaml:"system_prompt_mode" json:"system_prompt_mode"`

	// SystemPromptWhen specifies when to apply system prompt
	SystemPromptWhen string `yaml:"system_prompt_when" json:"system_prompt_when"`

	// Serialize enables request serialization
	Serialize bool `yaml:"serialize" json:"serialize"`
}

// LLMProvidersConfig holds LLM provider configuration (v0.10.3).
// This is separated from CLI configuration.
type LLMProvidersConfig struct {
	// Providers holds individual provider configurations
	Providers ProvidersMap `yaml:"providers" json:"providers"`

	// DefaultProvider is the default provider to use
	DefaultProvider string `yaml:"default_provider" json:"default_provider"`

	// Fallback holds fallback configuration
	Fallback FallbackConfig `yaml:"fallback" json:"fallback"`
}

// ProvidersMap holds provider configurations.
type ProvidersMap struct {
	Anthropic AnthropicProviderConfig `yaml:"anthropic" json:"anthropic"`
	OpenAI    OpenAIProviderConfig    `yaml:"openai" json:"openai"`
	Ollama    OllamaProviderConfig    `yaml:"ollama" json:"ollama"`
}

// AnthropicProviderConfig holds Anthropic provider configuration.
type AnthropicProviderConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	APIKey  string `yaml:"api_key" json:"api_key,omitempty"`
	Model   string `yaml:"model" json:"model"`
}

// OpenAIProviderConfig holds OpenAI provider configuration.
type OpenAIProviderConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	APIKey  string `yaml:"api_key" json:"api_key,omitempty"`
	BaseURL string `yaml:"base_url" json:"base_url,omitempty"`
	Model   string `yaml:"model" json:"model"`
}

// OllamaProviderConfig holds Ollama provider configuration.
type OllamaProviderConfig struct {
	Enabled    bool     `yaml:"enabled" json:"enabled"`
	Endpoints  []string `yaml:"endpoints" json:"endpoints"`
	AutoDetect bool     `yaml:"auto_detect" json:"auto_detect"`
}

// FallbackConfig holds fallback configuration.
type FallbackConfig struct {
	Enabled bool     `yaml:"enabled" json:"enabled"`
	Chain   []string `yaml:"chain" json:"chain"`
}

// FirstRunConfig holds first-run wizard configuration (v0.10.3).
type FirstRunConfig struct {
	// Enabled enables the first-run wizard
	Enabled bool `yaml:"enabled" json:"enabled"`

	// ShowProviderDetection shows provider detection step
	ShowProviderDetection bool `yaml:"show_provider_detection" json:"show_provider_detection"`

	// ShowCLIDownload shows CLI download step
	ShowCLIDownload bool `yaml:"show_cli_download" json:"show_cli_download"`

	// AllowSkip allows skipping the wizard
	AllowSkip bool `yaml:"allow_skip" json:"allow_skip"`

	// RecommendCLI shows "strongly recommended" message for CLI
	RecommendCLI bool `yaml:"recommend_cli" json:"recommend_cli"`
}

// CCSwitchConfig holds cc-switch integration configuration (v0.10.3).
type CCSwitchConfig struct {
	// Enabled enables cc-switch integration
	Enabled bool `yaml:"enabled" json:"enabled"`

	// ConfigPath is the path to cc-switch config (empty = default)
	ConfigPath string `yaml:"config_path" json:"config_path"`

	// SyncProfiles enables profile synchronization
	SyncProfiles bool `yaml:"sync_profiles" json:"sync_profiles"`
}

// StatisticsConfig holds statistics collection configuration (v0.10.3).
type StatisticsConfig struct {
	// Enabled enables statistics collection
	Enabled bool `yaml:"enabled" json:"enabled"`

	// OptInRequired requires user opt-in
	OptInRequired bool `yaml:"opt_in_required" json:"opt_in_required"`

	// StoragePath is the storage path (empty = default)
	StoragePath string `yaml:"storage_path" json:"storage_path"`

	// RetentionDays is the data retention period
	RetentionDays int `yaml:"retention_days" json:"retention_days"`
}

// ToolCallingConfig holds tool calling configuration (v0.10.3).
type ToolCallingConfig struct {
	// AutoDetect enables automatic capability detection
	AutoDetect bool `yaml:"auto_detect" json:"auto_detect"`

	// DetectionTimeout is the timeout for capability detection
	DetectionTimeout time.Duration `yaml:"detection_timeout" json:"detection_timeout"`

	// SmartSelection enables IR-based tool selection to reduce token usage
	SmartSelection bool `yaml:"smart_selection" json:"smart_selection"`

	// SmartSelectionMaxTools limits the number of tools sent to the LLM
	SmartSelectionMaxTools int `yaml:"smart_selection_max_tools" json:"smart_selection_max_tools"`

	// SmartSkillSelection enables progressive skill selection.
	SmartSkillSelection bool `yaml:"smart_skill_selection" json:"smart_skill_selection"`

	// SkillSelectorMode controls selector strategy: hybrid|ir_only|llm_only.
	SkillSelectorMode string `yaml:"skill_selector_mode" json:"skill_selector_mode"`

	// SkillRerankEnabled toggles stage-2 reranking.
	SkillRerankEnabled bool `yaml:"skill_rerank_enabled" json:"skill_rerank_enabled"`

	// SkillRerankModel is the compact reranker model hint.
	SkillRerankModel string `yaml:"skill_rerank_model" json:"skill_rerank_model"`

	// SkillRerankONNXEnabled toggles ONNX model path for reranker.
	SkillRerankONNXEnabled bool `yaml:"skill_rerank_onnx_enabled" json:"skill_rerank_onnx_enabled"`

	// SkillRerankONNXAutoDownload controls whether ONNX model can be auto-downloaded.
	SkillRerankONNXAutoDownload bool `yaml:"skill_rerank_onnx_auto_download" json:"skill_rerank_onnx_auto_download"`

	// SkillSelectorConfidenceThreshold is the confidence threshold for auto-selection.
	SkillSelectorConfidenceThreshold float64 `yaml:"skill_selector_confidence_threshold" json:"skill_selector_confidence_threshold"`

	// ToolRouterDynamicExposure enables query-based tool exposure filtering.
	ToolRouterDynamicExposure bool `yaml:"tool_router_dynamic_exposure" json:"tool_router_dynamic_exposure"`

	// ToolRouterSchemaCompression removes non-essential schema fields before prompting.
	ToolRouterSchemaCompression bool `yaml:"tool_router_schema_compression" json:"tool_router_schema_compression"`

	// Profile selects a legacy named base tool policy preset.
	Profile string `yaml:"profile" json:"profile"`

	// Profiles defines legacy named tool allowlists kept for compatibility.
	Profiles map[string][]string `yaml:"profiles" json:"profiles,omitempty"`

	// Groups defines reusable tool groups for internal allow/deny expansion.
	Groups map[string][]string `yaml:"groups" json:"groups,omitempty"`

	// Allow further narrows the globally visible tool set.
	Allow []string `yaml:"allow" json:"allow,omitempty"`

	// Deny removes tools from the globally visible tool set.
	Deny []string `yaml:"deny" json:"deny,omitempty"`

	// ByProvider narrows tool policy for specific providers or models.
	ByProvider map[string]ToolPolicyConfig `yaml:"by_provider" json:"by_provider,omitempty"`

	// WebSearch controls the runtime web_search tool backend.
	WebSearch ToolCallingWebSearchConfig `yaml:"web_search" json:"web_search"`

	// WebFetch controls the runtime web_fetch tool backend.
	WebFetch ToolCallingWebFetchConfig `yaml:"web_fetch" json:"web_fetch"`

	// Ripgrep controls the runtime ripgrep backend for local search tools.
	Ripgrep ToolCallingRipgrepConfig `yaml:"ripgrep" json:"ripgrep"`

	// Adapters holds adapter configurations
	Adapters ToolCallingAdaptersConfig `yaml:"adapters" json:"adapters"`

	// ProviderOverrides holds provider-specific overrides
	ProviderOverrides map[string]ProviderOverrideConfig `yaml:"provider_overrides" json:"provider_overrides"`
}

// ToolCallingAdaptersConfig holds adapter configurations.
type ToolCallingAdaptersConfig struct {
	// CLIProxy holds CLIProxy adapter configuration
	CLIProxy CLIProxyAdapterConfig `yaml:"cli_proxy" json:"cli_proxy"`

	// CCNexus holds ccNexus adapter configuration
	CCNexus CCNexusAdapterConfig `yaml:"cc_nexus" json:"cc_nexus"`
}

// CLIProxyAdapterConfig holds CLIProxy adapter configuration.
type CLIProxyAdapterConfig struct {
	// Enabled enables the adapter (only works when CLI is enabled)
	Enabled bool `yaml:"enabled" json:"enabled"`

	// PromptTemplate is the prompt template to use
	PromptTemplate string `yaml:"prompt_template" json:"prompt_template"`
}

// CCNexusAdapterConfig holds ccNexus adapter configuration.
type CCNexusAdapterConfig struct {
	// Enabled enables the adapter
	Enabled bool `yaml:"enabled" json:"enabled"`

	// SchemaMapping is the schema mapping mode
	SchemaMapping string `yaml:"schema_mapping" json:"schema_mapping"`
}

// ProviderOverrideConfig holds provider-specific override configuration.
type ProviderOverrideConfig struct {
	// ToolCalling is the tool calling mode (native, adapter, disabled)
	ToolCalling string `yaml:"tool_calling" json:"tool_calling"`
}

// ToolCallingWebSearchConfig holds runtime web_search provider configuration.
type ToolCallingWebSearchConfig struct {
	Provider   string        `yaml:"provider" json:"provider"`
	Providers  []string      `yaml:"providers" json:"providers,omitempty"`
	APIKey     string        `yaml:"api_key" json:"api_key,omitempty"`
	BaseURL    string        `yaml:"base_url" json:"base_url,omitempty"`
	MaxResults int           `yaml:"max_results" json:"max_results"`
	Timeout    time.Duration `yaml:"timeout" json:"timeout"`
	SafeSearch bool          `yaml:"safe_search" json:"safe_search"`
	Region     string        `yaml:"region" json:"region"`
}

// ToolCallingWebFetchConfig holds runtime web_fetch configuration.
type ToolCallingWebFetchConfig struct {
	AllowPrivateHosts bool          `yaml:"allow_private_hosts" json:"allow_private_hosts"`
	Timeout           time.Duration `yaml:"timeout" json:"timeout"`
	FirecrawlTimeout  time.Duration `yaml:"firecrawl_timeout" json:"firecrawl_timeout"`
}

// ToolCallingRipgrepConfig holds runtime ripgrep configuration.
type ToolCallingRipgrepConfig struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	AutoDownload      bool     `yaml:"auto_download" json:"auto_download"`
	AllowSystemBinary bool     `yaml:"allow_system_binary" json:"allow_system_binary"`
	CacheDir          string   `yaml:"cache_dir" json:"cache_dir"`
	MirrorBaseURL     string   `yaml:"mirror_base_url" json:"mirror_base_url"`
	MirrorBaseURLs    []string `yaml:"mirror_base_urls" json:"mirror_base_urls,omitempty"`
}

// DefaultClaudeCodeCLIConfig returns the default CLI configuration.
func DefaultClaudeCodeCLIConfig() *ClaudeCodeCLIConfig {
	return &ClaudeCodeCLIConfig{
		Enabled: true, // Strongly recommended
		Install: CLIInstallConfig{
			Path:           "",
			AutoUpdate:     false,
			VerifyChecksum: true,
		},
		Download: CLIDownloadConfig{
			BaseURL:  "https://storage.googleapis.com/anthropic-public/claude-code",
			CacheDir: "",
		},
		Features: CLIFeaturesConfig{
			Skills:           true,
			ToolCalling:      true,
			FileOperations:   true,
			TerminalCommands: true,
			MCPIntegration:   true,
			AgentMode:        true,
			ProjectContext:   true,
		},
		Backend: CLIBackendConfig{
			Command:           "claude",
			WorkspaceDir:      ".",
			DefaultModel:      "sonnet",
			Timeout:           "5m",
			SessionTTL:        "24h",
			Args:              []string{"-p", "--output-format", "text", "--dangerously-skip-permissions"},
			ResumeArgs:        []string{"-p", "--output-format", "text", "--dangerously-skip-permissions", "--resume", "{sessionId}"},
			Output:            "text",
			ResumeOutput:      "text",
			Input:             "arg",
			MaxPromptArgChars: 100000,
			ModelArg:          "--model",
			ModelAliases: map[string]string{
				"opus":   "opus",
				"sonnet": "sonnet",
				"haiku":  "haiku",
			},
			SessionArg:       "--session-id",
			SessionMode:      "always",
			SystemPromptArg:  "--append-system-prompt",
			SystemPromptMode: "append",
			SystemPromptWhen: "first",
			Serialize:        true,
		},
	}
}

// DefaultLLMProvidersConfig returns the default LLM providers configuration.
func DefaultLLMProvidersConfig() *LLMProvidersConfig {
	return &LLMProvidersConfig{
		Providers: ProvidersMap{
			Anthropic: AnthropicProviderConfig{
				Enabled: true,
				Model:   "claude-3-5-sonnet-20241022",
			},
			OpenAI: OpenAIProviderConfig{
				Enabled: true,
				Model:   "gpt-4o",
			},
			Ollama: OllamaProviderConfig{
				Enabled: true,
				Endpoints: []string{
					"http://localhost:11434",
					"http://127.0.0.1:11434",
				},
				AutoDetect: true,
			},
		},
		DefaultProvider: "anthropic",
		Fallback: FallbackConfig{
			Enabled: true,
			Chain:   []string{"anthropic", "openai", "ollama"},
		},
	}
}

// DefaultFirstRunConfig returns the default first-run configuration.
func DefaultFirstRunConfig() *FirstRunConfig {
	return &FirstRunConfig{
		Enabled:               true,
		ShowProviderDetection: true,
		ShowCLIDownload:       true,
		AllowSkip:             true,
		RecommendCLI:          true,
	}
}

// DefaultCCSwitchConfig returns the default cc-switch configuration.
func DefaultCCSwitchConfig() *CCSwitchConfig {
	return &CCSwitchConfig{
		Enabled:      true,
		ConfigPath:   "",
		SyncProfiles: true,
	}
}

// DefaultStatisticsConfig returns the default statistics configuration.
func DefaultStatisticsConfig() *StatisticsConfig {
	return &StatisticsConfig{
		Enabled:       true,
		OptInRequired: true,
		StoragePath:   "",
		RetentionDays: 90,
	}
}

// DefaultToolCallingConfig returns the default tool calling configuration.
func DefaultToolCallingConfig() *ToolCallingConfig {
	return &ToolCallingConfig{
		AutoDetect:                       true,
		DetectionTimeout:                 5 * time.Minute,
		SmartSelection:                   false,
		SmartSelectionMaxTools:           10,
		SmartSkillSelection:              false,
		SkillSelectorMode:                "hybrid",
		SkillRerankEnabled:               false,
		SkillRerankModel:                 "cross-encoder/ms-marco-MiniLM-L6-v2",
		SkillRerankONNXEnabled:           false,
		SkillRerankONNXAutoDownload:      false,
		SkillSelectorConfidenceThreshold: 0.78,
		ToolRouterDynamicExposure:        false,
		ToolRouterSchemaCompression:      false,
		Profile:                          "",
		Profiles: map[string][]string{
			"minimal":   {"sessions"},
			"coding":    {"group:fs", "group:runtime", "group:sessions", "group:memory", "group:research", "group:web"},
			"messaging": {"message", "sessions"},
			"full":      {},
		},
		Groups: map[string][]string{
			"group:runtime":    {"exec", "process"},
			"group:fs":         {"file_read", "file_write", "edit", "grep", "rg", "find", "ls"},
			"group:sessions":   {"sessions"},
			"group:memory":     {"memory"},
			"group:research":   {"research_run", "research_status"},
			"group:web":        {"web"},
			"group:ui":         {"browser", "canvas"},
			"group:automation": {"cron", "gateway", "nodes"},
			"group:messaging":  {"message"},
		},
		WebSearch: ToolCallingWebSearchConfig{
			Provider:   "duckduckgo",
			Providers:  []string{"duckduckgo", "bing"},
			MaxResults: 5,
			Timeout:    5 * time.Minute,
			SafeSearch: false,
			Region:     "wt-wt",
		},
		WebFetch: ToolCallingWebFetchConfig{
			AllowPrivateHosts: false,
			Timeout:           5 * time.Minute,
			FirecrawlTimeout:  5 * time.Minute,
		},
		Ripgrep: ToolCallingRipgrepConfig{
			Enabled:           true,
			AutoDownload:      true,
			AllowSystemBinary: true,
			CacheDir:          "",
			MirrorBaseURL:     "",
			MirrorBaseURLs: []string{
				"https://mirror.ghproxy.com",
				"https://gh-proxy.com",
				"https://downloads.sourceforge.net/project/ripgrep.mirror",
			},
		},
		Adapters: ToolCallingAdaptersConfig{
			CLIProxy: CLIProxyAdapterConfig{
				Enabled:        true,
				PromptTemplate: "default",
			},
			CCNexus: CCNexusAdapterConfig{
				Enabled:       true,
				SchemaMapping: "auto",
			},
		},
		ProviderOverrides: map[string]ProviderOverrideConfig{
			"ollama": {ToolCalling: "adapter"},
			"custom": {ToolCalling: "auto"},
		},
	}
}
