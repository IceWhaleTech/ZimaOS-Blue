package config

import "time"

// LLMConfig holds all LLM-related configuration.
type LLMConfig struct {
	Chains           []ModelChainConfig     `mapstructure:"chains"`
	AgentPreferences []AgentModelPreference `mapstructure:"agent_preferences"`
	HealthCheck      LLMHealthCheckConfig   `mapstructure:"health_check"`
	Metrics          LLMMetricsConfig       `mapstructure:"metrics"`
}

// ModelChainConfig defines a chain of models for failover.
type ModelChainConfig struct {
	Name        string        `mapstructure:"name"`
	Description string        `mapstructure:"description"`
	Models      []ModelConfig `mapstructure:"models"`
	Default     bool          `mapstructure:"default"`
}

// ModelConfig defines a single model in the chain.
type ModelConfig struct {
	Provider   string        `mapstructure:"provider"`
	Model      string        `mapstructure:"model"`
	Priority   int           `mapstructure:"priority"`
	Weight     int           `mapstructure:"weight"`
	MaxRetries int           `mapstructure:"max_retries"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

// AgentModelPreference defines per-agent model preferences.
type AgentModelPreference struct {
	AgentID   string        `mapstructure:"agent_id"`
	ChannelID string        `mapstructure:"channel_id"`
	ChainName string        `mapstructure:"chain_name"`
	Overrides []ModelConfig `mapstructure:"overrides"`
}

// LLMHealthCheckConfig holds health check configuration.
type LLMHealthCheckConfig struct {
	Enabled            bool          `mapstructure:"enabled"`
	Interval           time.Duration `mapstructure:"interval"`
	Timeout            time.Duration `mapstructure:"timeout"`
	UnhealthyThreshold int           `mapstructure:"unhealthy_threshold"`
	RecoveryThreshold  int           `mapstructure:"recovery_threshold"`
}

// LLMMetricsConfig holds metrics configuration.
type LLMMetricsConfig struct {
	Enabled                 bool `mapstructure:"enabled"`
	IncludeLatencyHistogram bool `mapstructure:"include_latency_histogram"`
	IncludeTokenCounts      bool `mapstructure:"include_token_counts"`
	IncludeErrorBreakdown   bool `mapstructure:"include_error_breakdown"`
}

// SessionConfig holds session management configuration.
type SessionConfig struct {
	MaxTokens   int                      `mapstructure:"max_tokens"`
	MaxMessages int                      `mapstructure:"max_messages"`
	IdleTimeout time.Duration            `mapstructure:"idle_timeout"`
	Compaction  SessionCompactionConfig  `mapstructure:"compaction"`
	Persistence SessionPersistenceConfig `mapstructure:"persistence"`
	Isolation   SessionIsolationConfig   `mapstructure:"isolation"`
	Cleanup     SessionCleanupConfig     `mapstructure:"cleanup"`
}

// SessionCompactionConfig holds compaction settings.
type SessionCompactionConfig struct {
	Enabled             bool          `mapstructure:"enabled"`
	Threshold           float64       `mapstructure:"threshold"`
	Strategy            string        `mapstructure:"strategy"`
	SummaryMaxTokens    int           `mapstructure:"summary_max_tokens"`
	PreserveRecent      int           `mapstructure:"preserve_recent"`
	AutoCompact         bool          `mapstructure:"auto_compact"`
	AutoCompactInterval time.Duration `mapstructure:"auto_compact_interval"`
}

// SessionPersistenceConfig holds persistence settings.
type SessionPersistenceConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	Path      string        `mapstructure:"path"`
	Interval  time.Duration `mapstructure:"interval"`
	OnMessage bool          `mapstructure:"on_message"`
	OnCompact bool          `mapstructure:"on_compact"`
}

// SessionIsolationConfig holds isolation settings.
type SessionIsolationConfig struct {
	ByAgent   bool `mapstructure:"by_agent"`
	ByChannel bool `mapstructure:"by_channel"`
	ByPeer    bool `mapstructure:"by_peer"`
	ByThread  bool `mapstructure:"by_thread"`
}

// SessionCleanupConfig holds cleanup settings.
type SessionCleanupConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	ArchiveAfter    time.Duration `mapstructure:"archive_after"`
	DeleteAfter     time.Duration `mapstructure:"delete_after"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

// EmbeddingConfig holds embedding provider configuration.
type EmbeddingConfig struct {
	Provider   string                   `mapstructure:"provider"`
	Model      string                   `mapstructure:"model"`
	Dimensions int                      `mapstructure:"dimensions"`
	BatchSize  int                      `mapstructure:"batch_size"`
	Timeout    time.Duration            `mapstructure:"timeout"`
	Cache      EmbeddingCacheConfig     `mapstructure:"cache"`
	OpenAI     OpenAIEmbeddingConfig    `mapstructure:"openai"`
	Ollama     OllamaEmbeddingConfig    `mapstructure:"ollama"`
}

// EmbeddingCacheConfig holds embedding cache configuration.
type EmbeddingCacheConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	MaxEntries int           `mapstructure:"max_entries"`
	TTL        time.Duration `mapstructure:"ttl"`
}

// OpenAIEmbeddingConfig holds OpenAI embedding configuration.
type OpenAIEmbeddingConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// OllamaEmbeddingConfig holds Ollama embedding configuration.
type OllamaEmbeddingConfig struct {
	BaseURL string `mapstructure:"base_url"`
}

// MemoryConfig holds memory system configuration.
type MemoryConfig struct {
	VectorStore VectorStoreConfig `mapstructure:"vector_store"`
	Search      MemorySearchConfig `mapstructure:"search"`
}

// VectorStoreConfig holds vector store configuration.
type VectorStoreConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	DBPath     string `mapstructure:"db_path"`
	Dimensions int    `mapstructure:"dimensions"`
}

// MemorySearchConfig holds memory search configuration.
type MemorySearchConfig struct {
	VectorWeight  float64 `mapstructure:"vector_weight"`
	KeywordWeight float64 `mapstructure:"keyword_weight"`
	MinScore      float64 `mapstructure:"min_score"`
	MaxResults    int     `mapstructure:"max_results"`
}
