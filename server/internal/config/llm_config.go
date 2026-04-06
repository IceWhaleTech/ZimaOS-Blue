package config

import "time"

// LLMConfig holds all LLM-related configuration.
type LLMConfig struct {
	Chains           []ModelChainConfig     `yaml:"chains"`
	AgentPreferences []AgentModelPreference `yaml:"agent_preferences"`
	HealthCheck      LLMHealthCheckConfig   `yaml:"health_check"`
	Metrics          LLMMetricsConfig       `yaml:"metrics"`
}

// ModelChainConfig defines a chain of models for failover.
type ModelChainConfig struct {
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Models      []ModelConfig `yaml:"models"`
	Default     bool          `yaml:"default"`
}

// ModelConfig defines a single model in the chain.
type ModelConfig struct {
	Provider   string        `yaml:"provider"`
	Model      string        `yaml:"model"`
	Priority   int           `yaml:"priority"`
	Weight     int           `yaml:"weight"`
	MaxRetries int           `yaml:"max_retries"`
	Timeout    time.Duration `yaml:"timeout"`
}

// AgentModelPreference defines per-agent model preferences.
type AgentModelPreference struct {
	AgentID   string        `yaml:"agent_id"`
	ChannelID string        `yaml:"channel_id"`
	ChainName string        `yaml:"chain_name"`
	Overrides []ModelConfig `yaml:"overrides"`
}

// LLMHealthCheckConfig holds health check configuration.
type LLMHealthCheckConfig struct {
	Enabled            bool          `yaml:"enabled"`
	Interval           time.Duration `yaml:"interval"`
	Timeout            time.Duration `yaml:"timeout"`
	UnhealthyThreshold int           `yaml:"unhealthy_threshold"`
	RecoveryThreshold  int           `yaml:"recovery_threshold"`
}

// LLMMetricsConfig holds metrics configuration.
type LLMMetricsConfig struct {
	Enabled                 bool `yaml:"enabled"`
	IncludeLatencyHistogram bool `yaml:"include_latency_histogram"`
	IncludeTokenCounts      bool `yaml:"include_token_counts"`
	IncludeErrorBreakdown   bool `yaml:"include_error_breakdown"`
}

// SessionConfig holds session management configuration.
type SessionConfig struct {
	// MaxTokens limits retained session history for compaction/truncation.
	// Values <= 0 enable auto budgeting from model context when available, with
	// a modern large-context fallback for model-agnostic session paths.
	MaxTokens                   int                      `yaml:"max_tokens"`
	MaxMessages                 int                      `yaml:"max_messages"`
	IdleTimeout                 time.Duration            `yaml:"idle_timeout"`
	ChatDBDurability            string                   `yaml:"chat_db_durability"`
	ChatPersistAsync            bool                     `yaml:"chat_persist_async"`
	ChatPersistFlushOnResponse  bool                     `yaml:"chat_persist_flush_on_response"`
	ChatReadLite                bool                     `yaml:"chat_read_lite"`
	ChatAttachmentExternalStore bool                     `yaml:"chat_attachment_external_store"`
	Compaction                  SessionCompactionConfig  `yaml:"compaction"`
	Persistence                 SessionPersistenceConfig `yaml:"persistence"`
	Isolation                   SessionIsolationConfig   `yaml:"isolation"`
	Cleanup                     SessionCleanupConfig     `yaml:"cleanup"`
	Audit                       SessionAuditConfig       `yaml:"audit"`
}

// SessionCompactionConfig holds compaction settings.
type SessionCompactionConfig struct {
	Enabled             bool          `yaml:"enabled"`
	Threshold           float64       `yaml:"threshold"`
	Strategy            string        `yaml:"strategy"`
	SummaryMaxTokens    int           `yaml:"summary_max_tokens"`
	PreserveRecent      int           `yaml:"preserve_recent"`
	AutoCompact         bool          `yaml:"auto_compact"`
	AutoCompactInterval time.Duration `yaml:"auto_compact_interval"`
}

// SessionPersistenceConfig holds persistence settings.
type SessionPersistenceConfig struct {
	Enabled   bool          `yaml:"enabled"`
	Path      string        `yaml:"path"`
	Interval  time.Duration `yaml:"interval"`
	OnMessage bool          `yaml:"on_message"`
	OnCompact bool          `yaml:"on_compact"`
}

// SessionIsolationConfig holds isolation settings.
type SessionIsolationConfig struct {
	ByAgent   bool `yaml:"by_agent"`
	ByChannel bool `yaml:"by_channel"`
	ByPeer    bool `yaml:"by_peer"`
	ByThread  bool `yaml:"by_thread"`
}

// SessionCleanupConfig holds cleanup settings.
type SessionCleanupConfig struct {
	Enabled         bool          `yaml:"enabled"`
	ArchiveAfter    time.Duration `yaml:"archive_after"`
	DeleteAfter     time.Duration `yaml:"delete_after"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

// SessionAuditConfig controls dedicated chat tool-payload audit logging.
type SessionAuditConfig struct {
	Enabled          bool          `yaml:"enabled"`
	Path             string        `yaml:"path"`
	IPCEnabled       bool          `yaml:"ipc_enabled"`
	RetentionDays    int           `yaml:"retention_days"`
	CleanupInterval  time.Duration `yaml:"cleanup_interval"`
	CleanupBatchSize int           `yaml:"cleanup_batch_size"`
}

// EmbeddingConfig holds embedding provider configuration.
type EmbeddingConfig struct {
	Enabled    bool                  `yaml:"enabled"`
	Provider   string                `yaml:"provider"`
	Model      string                `yaml:"model"`
	Dimensions int                   `yaml:"dimensions"`
	BatchSize  int                   `yaml:"batch_size"`
	Timeout    time.Duration         `yaml:"timeout"`
	Cache      EmbeddingCacheConfig  `yaml:"cache"`
	OpenAI     OpenAIEmbeddingConfig `yaml:"openai"`
	Ollama     OllamaEmbeddingConfig `yaml:"ollama"`
}

// EmbeddingCacheConfig holds embedding cache configuration.
type EmbeddingCacheConfig struct {
	Enabled    bool          `yaml:"enabled"`
	MaxEntries int           `yaml:"max_entries"`
	TTL        time.Duration `yaml:"ttl"`
}

// OpenAIEmbeddingConfig holds OpenAI embedding configuration.
type OpenAIEmbeddingConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
}

// OllamaEmbeddingConfig holds Ollama embedding configuration.
type OllamaEmbeddingConfig struct {
	BaseURL string `yaml:"base_url"`
}

// MemoryConfig holds memory system configuration.
type MemoryConfig struct {
	VectorStore VectorStoreConfig  `yaml:"vector_store"`
	Search      MemorySearchConfig `yaml:"search"`
	Backend     string             `yaml:"backend"`      // "local", "markdown", "mixed" (default: "markdown")
	MarkdownDir string             `yaml:"markdown_dir"` // Base directory for markdown files
}

// VectorStoreConfig holds vector store configuration.
type VectorStoreConfig struct {
	Enabled    bool   `yaml:"enabled"`
	DBPath     string `yaml:"db_path"`
	Dimensions int    `yaml:"dimensions"`
}

// MemorySearchConfig holds memory search configuration.
type MemorySearchConfig struct {
	VectorWeight  float64 `yaml:"vector_weight"`
	KeywordWeight float64 `yaml:"keyword_weight"`
	MinScore      float64 `yaml:"min_score"`
	MaxResults    int     `yaml:"max_results"`
}
