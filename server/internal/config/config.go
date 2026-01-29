package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Log           LogConfig           `mapstructure:"log"`
	Worker        WorkerConfig        `mapstructure:"worker"`
	Resources     ResourcesConfig     `mapstructure:"resources"`
	Cgroup        CgroupConfig        `mapstructure:"cgroup"`
	Channels      channel.Config      `mapstructure:"channels"`
	Performance   PerformanceConfig   `mapstructure:"performance"`
	Security      SecurityConfig      `mapstructure:"security"`
	LLM           LLMConfig           `mapstructure:"llm"`
	Session       SessionConfig       `mapstructure:"session"`
	Embedding     EmbeddingConfig     `mapstructure:"embedding"`
	Memory        MemoryConfig        `mapstructure:"memory"`
	Grayscale     GrayscaleConfig     `mapstructure:"grayscale"`
	Companion     CompanionConfig     `mapstructure:"companion"`
	ClaudeCodeCLI ClaudeCodeCLIConfig `mapstructure:"claude_code_cli"` // v0.10.3: Separated from LLM
	FirstRun      FirstRunConfig      `mapstructure:"first_run"`       // v0.10.3
	CCSwitch      CCSwitchConfig      `mapstructure:"cc_switch"`       // v0.10.3
	Statistics    StatisticsConfig    `mapstructure:"statistics"`      // v0.10.3
	ToolCalling   ToolCallingConfig   `mapstructure:"tool_calling"`    // v0.10.3

	// Deprecated: Use ClaudeCodeCLI instead. Kept for backward compatibility.
	ClaudeCode ClaudeCodeConfig `mapstructure:"claudecode"`
}

// ClaudeCodeConfig holds Claude Code CLI integration configuration (v0.10).
// Deprecated: Use ClaudeCodeCLIConfig instead.
type ClaudeCodeConfig struct {
	Enabled      bool                      `mapstructure:"enabled"`
	Command      string                    `mapstructure:"command"`
	WorkspaceDir string                    `mapstructure:"workspace_dir"`
	DefaultModel string                    `mapstructure:"default_model"`
	Timeout      time.Duration             `mapstructure:"timeout"`
	SessionTTL   time.Duration             `mapstructure:"session_ttl"`
	APIKey       string                    `mapstructure:"api_key"`
	BaseURL      string                    `mapstructure:"base_url"`
	Backend      ClaudeCodeBackendConfig   `mapstructure:"backend"`
}

// ClaudeCodeBackendConfig holds CLI backend configuration.
type ClaudeCodeBackendConfig struct {
	Args             []string          `mapstructure:"args"`
	ResumeArgs       []string          `mapstructure:"resume_args"`
	Output           string            `mapstructure:"output"`
	Input            string            `mapstructure:"input"`
	MaxPromptArgChars int              `mapstructure:"max_prompt_arg_chars"`
	Env              map[string]string `mapstructure:"env"`
	ClearEnv         []string          `mapstructure:"clear_env"`
	ModelArg         string            `mapstructure:"model_arg"`
	ModelAliases     map[string]string `mapstructure:"model_aliases"`
	SessionArg       string            `mapstructure:"session_arg"`
	SessionMode      string            `mapstructure:"session_mode"`
	SystemPromptArg  string            `mapstructure:"system_prompt_arg"`
	SystemPromptMode string            `mapstructure:"system_prompt_mode"`
	SystemPromptWhen string            `mapstructure:"system_prompt_when"`
	Serialize        bool              `mapstructure:"serialize"`
}

// CompanionConfig holds Echo Companion monitoring configuration (v0.9.1).
type CompanionConfig struct {
	Enabled     bool                         `mapstructure:"enabled"`
	Storage     CompanionStorageConfig       `mapstructure:"storage"`
	WebSocket   CompanionWebSocketConfig     `mapstructure:"websocket"`
	Retention   CompanionRetentionConfig     `mapstructure:"retention"`
	Alerts      CompanionAlertConfig         `mapstructure:"alerts"`
	Security    CompanionSecurityConfig      `mapstructure:"security"`
	Performance CompanionPerformanceConfig   `mapstructure:"performance"`
}

// CompanionStorageConfig holds companion storage configuration.
type CompanionStorageConfig struct {
	BasePath string `mapstructure:"base_path"`
	Format   string `mapstructure:"format"`
}

// CompanionWebSocketConfig holds companion WebSocket configuration.
type CompanionWebSocketConfig struct {
	PingInterval    time.Duration `mapstructure:"ping_interval"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ReadBufferSize  int           `mapstructure:"read_buffer_size"`
	WriteBufferSize int           `mapstructure:"write_buffer_size"`
}

// CompanionRetentionConfig holds companion data retention configuration.
type CompanionRetentionConfig struct {
	EventsDays   int `mapstructure:"events_days"`
	SessionsDays int `mapstructure:"sessions_days"`
	AlertsDays   int `mapstructure:"alerts_days"`
}

// CompanionAlertConfig holds companion alert configuration.
type CompanionAlertConfig struct {
	Enabled         bool                    `mapstructure:"enabled"`
	ThreatThreshold string                  `mapstructure:"threat_threshold"`
	Channels        []CompanionAlertChannel `mapstructure:"channels"`
}

// CompanionAlertChannel represents an alert notification channel.
type CompanionAlertChannel struct {
	Type       string            `mapstructure:"type"`
	URL        string            `mapstructure:"url,omitempty"`
	Recipients []string          `mapstructure:"recipients,omitempty"`
	Headers    map[string]string `mapstructure:"headers,omitempty"`
}

// CompanionSecurityConfig holds companion security integration configuration.
type CompanionSecurityConfig struct {
	PromptGuardIntegration bool `mapstructure:"prompt_guard_integration"`
	AuditLogIntegration    bool `mapstructure:"audit_log_integration"`
	SandboxMonitor         bool `mapstructure:"sandbox_monitor"`
}

// CompanionPerformanceConfig holds companion performance configuration.
type CompanionPerformanceConfig struct {
	MaxConcurrentSessions int           `mapstructure:"max_concurrent_sessions"`
	EventBufferSize       int           `mapstructure:"event_buffer_size"`
	BatchWriteInterval    time.Duration `mapstructure:"batch_write_interval"`
}

// SecurityConfig holds security-related configuration (v0.7).
type SecurityConfig struct {
	JWT        JWTConfig        `mapstructure:"jwt"`
	OIDC       OIDCConfig       `mapstructure:"oidc"`
	Users      UsersConfig      `mapstructure:"users"`
	Password   PasswordConfig   `mapstructure:"password"`
	MFA        MFAConfig        `mapstructure:"mfa"`
	Audit      AuditConfig      `mapstructure:"audit"`
	Sandbox    SandboxConfig    `mapstructure:"sandbox"`
	Encryption EncryptionConfig `mapstructure:"encryption"`
}

// JWTConfig holds JWT authentication configuration.
type JWTConfig struct {
	Secret            string        `mapstructure:"secret"`
	Expiration        time.Duration `mapstructure:"expiration"`
	RefreshExpiration time.Duration `mapstructure:"refresh_expiration"`
	Issuer            string        `mapstructure:"issuer"`
}

// EncryptionConfig holds encryption configuration for sensitive data.
type EncryptionConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	KeyPath    string `mapstructure:"key_path"`
	Passphrase string `mapstructure:"passphrase"`
}

// OIDCConfig holds OIDC provider configuration.
type OIDCConfig struct {
	Enabled                bool          `mapstructure:"enabled"`
	Issuer                 string        `mapstructure:"issuer"`
	SigningKeyPath         string        `mapstructure:"signing_key_path"`
	SigningKeyRotationDays int           `mapstructure:"signing_key_rotation_days"`
	AccessTokenTTL         time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL        time.Duration `mapstructure:"refresh_token_ttl"`
	AuthorizationCodeTTL   time.Duration `mapstructure:"authorization_code_ttl"`
	Clients                []OIDCClient  `mapstructure:"clients"`
}

// OIDCClient holds OIDC client configuration.
type OIDCClient struct {
	ClientID          string   `mapstructure:"client_id"`
	ClientSecret      string   `mapstructure:"client_secret"`
	RedirectURIs      []string `mapstructure:"redirect_uris"`
	AllowedScopes     []string `mapstructure:"allowed_scopes"`
	AllowedGrantTypes []string `mapstructure:"allowed_grant_types"`
	Public            bool     `mapstructure:"public"`
}

// UsersConfig holds user management configuration.
type UsersConfig struct {
	AllowRegistration        bool   `mapstructure:"allow_registration"`
	RequireEmailVerification bool   `mapstructure:"require_email_verification"`
	DefaultRole              string `mapstructure:"default_role"`
}

// PasswordConfig holds password policy configuration.
type PasswordConfig struct {
	MinLength        int           `mapstructure:"min_length"`
	RequireUppercase bool          `mapstructure:"require_uppercase"`
	RequireLowercase bool          `mapstructure:"require_lowercase"`
	RequireNumber    bool          `mapstructure:"require_number"`
	RequireSpecial   bool          `mapstructure:"require_special"`
	HistoryCount     int           `mapstructure:"history_count"`
	ExpirationDays   int           `mapstructure:"expiration_days"`
	LockoutThreshold int           `mapstructure:"lockout_threshold"`
	LockoutDuration  time.Duration `mapstructure:"lockout_duration"`
}

// MFAConfig holds MFA configuration.
type MFAConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	Required           bool   `mapstructure:"required"`
	Issuer             string `mapstructure:"issuer"`
	RecoveryCodesCount int    `mapstructure:"recovery_codes_count"`
}

// AuditConfig holds audit logging configuration.
type AuditConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	RetentionDays   int           `mapstructure:"retention_days"`
	LogRequestBody  bool          `mapstructure:"log_request_body"`
	LogResponseBody bool          `mapstructure:"log_response_body"`
	ExcludedPaths   []string      `mapstructure:"excluded_paths"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

// SandboxConfig holds sandbox execution configuration.
type SandboxConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	DefaultTimeout time.Duration `mapstructure:"default_timeout"`
	MaxTimeout     time.Duration `mapstructure:"max_timeout"`
	MemoryLimit    string        `mapstructure:"memory_limit"`
	CPULimit       float64       `mapstructure:"cpu_limit"`
	ProcessLimit   int           `mapstructure:"process_limit"`
	NetworkEnabled bool          `mapstructure:"network_enabled"`
}

// ResourcesConfig holds resource limit configuration.
type ResourcesConfig struct {
	MaxMemoryMB   int64  `mapstructure:"max_memory_mb"`
	MaxCPUPercent int    `mapstructure:"max_cpu_percent"`
	MaxOpenFiles  uint64 `mapstructure:"max_open_files"`
	MaxGoroutines int    `mapstructure:"max_goroutines"`
	GCPercent     int    `mapstructure:"gc_percent"`
}

// CgroupConfig holds cgroup v2 configuration.
type CgroupConfig struct {
	Enabled    bool              `mapstructure:"enabled"`
	CgroupRoot string            `mapstructure:"cgroup_root"`
	CgroupName string            `mapstructure:"cgroup_name"`
	IO         CgroupIOConfig    `mapstructure:"io"`
	Memory     CgroupMemConfig   `mapstructure:"memory"`
	CPU        CgroupCPUConfig   `mapstructure:"cpu"`
}

// CgroupIOConfig holds IO bandwidth limit configuration.
type CgroupIOConfig struct {
	Enabled   bool     `mapstructure:"enabled"`
	ReadBPS   uint64   `mapstructure:"read_bps"`
	WriteBPS  uint64   `mapstructure:"write_bps"`
	ReadIOPS  uint64   `mapstructure:"read_iops"`
	WriteIOPS uint64   `mapstructure:"write_iops"`
	Devices   []string `mapstructure:"devices"`
}

// CgroupMemConfig holds cgroup memory limit configuration.
type CgroupMemConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	MaxBytes     uint64 `mapstructure:"max_bytes"`
	HighBytes    uint64 `mapstructure:"high_bytes"`
	SwapMaxBytes uint64 `mapstructure:"swap_max_bytes"`
}

// CgroupCPUConfig holds cgroup CPU limit configuration.
type CgroupCPUConfig struct {
	Enabled    bool `mapstructure:"enabled"`
	MaxPercent int  `mapstructure:"max_percent"`
	Weight     int  `mapstructure:"weight"`
}

type ServerConfig struct {
	Host           string        `mapstructure:"host"`
	Port           int           `mapstructure:"port"`
	PortAutoFallback bool        `mapstructure:"port_auto_fallback"` // Auto fallback to random port if configured port is in use
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	IdleTimeout    time.Duration `mapstructure:"idle_timeout"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // json or console
	Output string `mapstructure:"output"` // stdout, stderr, or file path
}

type WorkerConfig struct {
	PoolSize    int `mapstructure:"pool_size"`
	MaxQueueLen int `mapstructure:"max_queue_len"`
}

// PerformanceConfig holds performance optimization configuration.
type PerformanceConfig struct {
	Database    DatabasePerfConfig    `mapstructure:"database"`
	Memory      MemoryPerfConfig      `mapstructure:"memory"`
	Concurrency ConcurrencyPerfConfig `mapstructure:"concurrency"`
	Network     NetworkPerfConfig     `mapstructure:"network"`
	Cache       CachePerfConfig       `mapstructure:"cache"`
	Profiling   ProfilingPerfConfig   `mapstructure:"profiling"`
}

// DatabasePerfConfig holds database performance configuration.
type DatabasePerfConfig struct {
	PoolSize            int           `mapstructure:"pool_size"`
	MaxIdleConns        int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime     time.Duration `mapstructure:"conn_max_lifetime"`
	WALMode             bool          `mapstructure:"wal_mode"`
	CacheSize           int           `mapstructure:"cache_size"`
	PageSize            int           `mapstructure:"page_size"`
	CheckpointInterval  time.Duration `mapstructure:"checkpoint_interval"`
	BatchSize           int           `mapstructure:"batch_size"`
	SlowQueryThreshold  time.Duration `mapstructure:"slow_query_threshold"`
	EnableQueryCache    bool          `mapstructure:"enable_query_cache"`
}

// MemoryPerfConfig holds memory performance configuration.
type MemoryPerfConfig struct {
	GOGC           int    `mapstructure:"gogc"`
	GOMemLimit     string `mapstructure:"gomemlimit"`
	BufferPoolSize int    `mapstructure:"buffer_pool_size"`
	ObjectPoolSize int    `mapstructure:"object_pool_size"`
}

// ConcurrencyPerfConfig holds concurrency performance configuration.
type ConcurrencyPerfConfig struct {
	WorkerPoolSize    int `mapstructure:"worker_pool_size"`
	MaxGoroutines     int `mapstructure:"max_goroutines"`
	ChannelBufferSize int `mapstructure:"channel_buffer_size"`
}

// NetworkPerfConfig holds network performance configuration.
type NetworkPerfConfig struct {
	HTTP2Enabled       bool          `mapstructure:"http2_enabled"`
	KeepAliveTimeout   time.Duration `mapstructure:"keep_alive_timeout"`
	CompressionEnabled bool          `mapstructure:"compression_enabled"`
	CompressionLevel   int           `mapstructure:"compression_level"`
	RequestTimeout     time.Duration `mapstructure:"request_timeout"`
	RetryMaxAttempts   int           `mapstructure:"retry_max_attempts"`
	RetryBackoffBase   time.Duration `mapstructure:"retry_backoff_base"`
}

// CachePerfConfig holds cache performance configuration.
type CachePerfConfig struct {
	L1Enabled      bool          `mapstructure:"l1_enabled"`
	L1Size         int           `mapstructure:"l1_size"`
	L1TTL          time.Duration `mapstructure:"l1_ttl"`
	L2Enabled      bool          `mapstructure:"l2_enabled"`
	L2Path         string        `mapstructure:"l2_path"`
	L2Size         string        `mapstructure:"l2_size"`
	L2TTL          time.Duration `mapstructure:"l2_ttl"`
	EvictionPolicy string        `mapstructure:"eviction_policy"`
}

// ProfilingPerfConfig holds profiling configuration.
type ProfilingPerfConfig struct {
	PprofEnabled     bool   `mapstructure:"pprof_enabled"`
	PprofPath        string `mapstructure:"pprof_path"`
	MetricsEnabled   bool   `mapstructure:"metrics_enabled"`
	BenchmarkEnabled bool   `mapstructure:"benchmark_enabled"`
}

func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/zimaos-echo")
	}

	// Environment variables
	v.SetEnvPrefix("ECHO")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file (ignore if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.port_auto_fallback", true)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.idle_timeout", "120s")

	// Log defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("log.output", "stdout")

	// Worker defaults
	v.SetDefault("worker.pool_size", 10)
	v.SetDefault("worker.max_queue_len", 100)

	// Resources defaults
	v.SetDefault("resources.max_memory_mb", 512)
	v.SetDefault("resources.max_cpu_percent", 50)
	v.SetDefault("resources.max_open_files", 65536)
	v.SetDefault("resources.max_goroutines", 10000)
	v.SetDefault("resources.gc_percent", 100)

	// Cgroup defaults (disabled by default)
	v.SetDefault("cgroup.enabled", false)
	v.SetDefault("cgroup.cgroup_root", "/sys/fs/cgroup")
	v.SetDefault("cgroup.cgroup_name", "zimaos-echo")
	v.SetDefault("cgroup.io.enabled", false)
	v.SetDefault("cgroup.io.read_bps", 0)
	v.SetDefault("cgroup.io.write_bps", 0)
	v.SetDefault("cgroup.io.read_iops", 0)
	v.SetDefault("cgroup.io.write_iops", 0)
	v.SetDefault("cgroup.memory.enabled", false)
	v.SetDefault("cgroup.cpu.enabled", false)
	v.SetDefault("cgroup.cpu.weight", 100)

	// Channel defaults (disabled by default)
	v.SetDefault("channels.enabled", false)
	v.SetDefault("channels.default_timeout_seconds", 30)
	v.SetDefault("channels.max_message_length", 4096)
	v.SetDefault("channels.telegram.enabled", false)
	v.SetDefault("channels.discord.enabled", false)
	v.SetDefault("channels.slack.enabled", false)
	v.SetDefault("channels.wechat_work.enabled", false)
	v.SetDefault("channels.feishu.enabled", false)
	v.SetDefault("channels.matrix.enabled", false)

	// Performance defaults
	// Database
	v.SetDefault("performance.database.pool_size", 10)
	v.SetDefault("performance.database.max_idle_conns", 5)
	v.SetDefault("performance.database.conn_max_lifetime", "1h")
	v.SetDefault("performance.database.wal_mode", true)
	v.SetDefault("performance.database.cache_size", 10000)
	v.SetDefault("performance.database.page_size", 4096)
	v.SetDefault("performance.database.checkpoint_interval", "5m")
	v.SetDefault("performance.database.batch_size", 1000)
	v.SetDefault("performance.database.slow_query_threshold", "100ms")
	v.SetDefault("performance.database.enable_query_cache", true)

	// Memory
	v.SetDefault("performance.memory.gogc", 100)
	v.SetDefault("performance.memory.gomemlimit", "512MB")
	v.SetDefault("performance.memory.buffer_pool_size", 1000)
	v.SetDefault("performance.memory.object_pool_size", 500)

	// Concurrency
	v.SetDefault("performance.concurrency.worker_pool_size", 100)
	v.SetDefault("performance.concurrency.max_goroutines", 10000)
	v.SetDefault("performance.concurrency.channel_buffer_size", 100)

	// Network
	v.SetDefault("performance.network.http2_enabled", true)
	v.SetDefault("performance.network.keep_alive_timeout", "30s")
	v.SetDefault("performance.network.compression_enabled", true)
	v.SetDefault("performance.network.compression_level", 6)
	v.SetDefault("performance.network.request_timeout", "30s")
	v.SetDefault("performance.network.retry_max_attempts", 3)
	v.SetDefault("performance.network.retry_backoff_base", "100ms")

	// Cache
	v.SetDefault("performance.cache.l1_enabled", true)
	v.SetDefault("performance.cache.l1_size", 1000)
	v.SetDefault("performance.cache.l1_ttl", "5m")
	v.SetDefault("performance.cache.l2_enabled", true)
	v.SetDefault("performance.cache.l2_path", "./cache")
	v.SetDefault("performance.cache.l2_size", "100MB")
	v.SetDefault("performance.cache.l2_ttl", "1h")
	v.SetDefault("performance.cache.eviction_policy", "lru")

	// Profiling
	v.SetDefault("performance.profiling.pprof_enabled", true)
	v.SetDefault("performance.profiling.pprof_path", "/debug/pprof")
	v.SetDefault("performance.profiling.metrics_enabled", true)
	v.SetDefault("performance.profiling.benchmark_enabled", false)

	// Security defaults (v0.7)
	// JWT
	v.SetDefault("security.jwt.secret", "change-me-in-production-use-a-strong-secret-key")
	v.SetDefault("security.jwt.expiration", "24h")
	v.SetDefault("security.jwt.refresh_expiration", "720h")
	v.SetDefault("security.jwt.issuer", "zimaos-echo")

	// OIDC
	v.SetDefault("security.oidc.enabled", true)
	v.SetDefault("security.oidc.issuer", "http://localhost:8080")
	v.SetDefault("security.oidc.signing_key_path", "./keys/oidc.key")
	v.SetDefault("security.oidc.signing_key_rotation_days", 90)
	v.SetDefault("security.oidc.access_token_ttl", "1h")
	v.SetDefault("security.oidc.refresh_token_ttl", "720h")
	v.SetDefault("security.oidc.authorization_code_ttl", "10m")

	// Users
	v.SetDefault("security.users.allow_registration", false)
	v.SetDefault("security.users.require_email_verification", false)
	v.SetDefault("security.users.default_role", "user")

	// Password
	v.SetDefault("security.password.min_length", 12)
	v.SetDefault("security.password.require_uppercase", true)
	v.SetDefault("security.password.require_lowercase", true)
	v.SetDefault("security.password.require_number", true)
	v.SetDefault("security.password.require_special", true)
	v.SetDefault("security.password.history_count", 5)
	v.SetDefault("security.password.expiration_days", 0)
	v.SetDefault("security.password.lockout_threshold", 5)
	v.SetDefault("security.password.lockout_duration", "15m")

	// MFA
	v.SetDefault("security.mfa.enabled", true)
	v.SetDefault("security.mfa.required", false)
	v.SetDefault("security.mfa.issuer", "ZimaOS-Echo")
	v.SetDefault("security.mfa.recovery_codes_count", 8)

	// Audit
	v.SetDefault("security.audit.enabled", true)
	v.SetDefault("security.audit.retention_days", 90)
	v.SetDefault("security.audit.log_request_body", false)
	v.SetDefault("security.audit.log_response_body", false)
	v.SetDefault("security.audit.excluded_paths", []string{"/health", "/metrics"})
	v.SetDefault("security.audit.cleanup_interval", "24h")

	// Sandbox
	v.SetDefault("security.sandbox.enabled", true)
	v.SetDefault("security.sandbox.default_timeout", "30s")
	v.SetDefault("security.sandbox.max_timeout", "5m")
	v.SetDefault("security.sandbox.memory_limit", "256MB")
	v.SetDefault("security.sandbox.cpu_limit", 1.0)
	v.SetDefault("security.sandbox.process_limit", 10)
	v.SetDefault("security.sandbox.network_enabled", false)

	// Encryption
	v.SetDefault("security.encryption.enabled", false)
	v.SetDefault("security.encryption.key_path", "./keys/encryption.key")
	v.SetDefault("security.encryption.passphrase", "")

	// LLM defaults
	v.SetDefault("llm.health_check.enabled", true)
	v.SetDefault("llm.health_check.interval", "30s")
	v.SetDefault("llm.health_check.timeout", "5s")
	v.SetDefault("llm.health_check.unhealthy_threshold", 3)
	v.SetDefault("llm.health_check.recovery_threshold", 2)
	v.SetDefault("llm.metrics.enabled", true)
	v.SetDefault("llm.metrics.include_latency_histogram", true)
	v.SetDefault("llm.metrics.include_token_counts", true)
	v.SetDefault("llm.metrics.include_error_breakdown", true)

	// Session defaults
	v.SetDefault("session.max_tokens", 8000)
	v.SetDefault("session.max_messages", 100)
	v.SetDefault("session.idle_timeout", "30m")
	v.SetDefault("session.compaction.enabled", true)
	v.SetDefault("session.compaction.threshold", 0.8)
	v.SetDefault("session.compaction.strategy", "summarize")
	v.SetDefault("session.compaction.summary_max_tokens", 500)
	v.SetDefault("session.compaction.preserve_recent", 5)
	v.SetDefault("session.compaction.auto_compact", true)
	v.SetDefault("session.compaction.auto_compact_interval", "5m")
	v.SetDefault("session.persistence.enabled", true)
	v.SetDefault("session.persistence.path", "./data/sessions.db")
	v.SetDefault("session.persistence.interval", "1m")
	v.SetDefault("session.persistence.on_message", true)
	v.SetDefault("session.persistence.on_compact", true)
	v.SetDefault("session.isolation.by_agent", true)
	v.SetDefault("session.isolation.by_channel", true)
	v.SetDefault("session.isolation.by_peer", true)
	v.SetDefault("session.isolation.by_thread", false)
	v.SetDefault("session.cleanup.enabled", true)
	v.SetDefault("session.cleanup.archive_after", "168h")
	v.SetDefault("session.cleanup.delete_after", "720h")
	v.SetDefault("session.cleanup.cleanup_interval", "1h")

	// Embedding defaults
	v.SetDefault("embedding.provider", "openai")
	v.SetDefault("embedding.model", "text-embedding-3-small")
	v.SetDefault("embedding.dimensions", 1536)
	v.SetDefault("embedding.batch_size", 100)
	v.SetDefault("embedding.timeout", "30s")
	v.SetDefault("embedding.cache.enabled", true)
	v.SetDefault("embedding.cache.max_entries", 10000)
	v.SetDefault("embedding.cache.ttl", "24h")
	v.SetDefault("embedding.openai.base_url", "https://api.openai.com")
	v.SetDefault("embedding.ollama.base_url", "http://localhost:11434")

	// Memory defaults
	v.SetDefault("memory.vector_store.enabled", true)
	v.SetDefault("memory.vector_store.db_path", "./data/memory.db")
	v.SetDefault("memory.vector_store.dimensions", 1536)
	v.SetDefault("memory.search.vector_weight", 0.7)
	v.SetDefault("memory.search.keyword_weight", 0.3)
	v.SetDefault("memory.search.min_score", 0.5)
	v.SetDefault("memory.search.max_results", 10)

	// Grayscale/Feature flags defaults
	v.SetDefault("grayscale.enabled", false)

	// Companion defaults (v0.9.1)
	v.SetDefault("companion.enabled", true)
	v.SetDefault("companion.storage.base_path", "./data/companion")
	v.SetDefault("companion.storage.format", "jsonl")
	v.SetDefault("companion.websocket.ping_interval", "30s")
	v.SetDefault("companion.websocket.write_timeout", "10s")
	v.SetDefault("companion.websocket.read_buffer_size", 1024)
	v.SetDefault("companion.websocket.write_buffer_size", 1024)
	v.SetDefault("companion.retention.events_days", 7)
	v.SetDefault("companion.retention.sessions_days", 30)
	v.SetDefault("companion.retention.alerts_days", 90)
	v.SetDefault("companion.alerts.enabled", true)
	v.SetDefault("companion.alerts.threat_threshold", "medium")
	v.SetDefault("companion.security.prompt_guard_integration", true)
	v.SetDefault("companion.security.audit_log_integration", true)
	v.SetDefault("companion.security.sandbox_monitor", true)
	v.SetDefault("companion.performance.max_concurrent_sessions", 1000)
	v.SetDefault("companion.performance.event_buffer_size", 10000)
	v.SetDefault("companion.performance.batch_write_interval", "1s")

	// Claude Code CLI defaults (v0.10) - Deprecated
	v.SetDefault("claudecode.enabled", false)
	v.SetDefault("claudecode.command", "claude")
	v.SetDefault("claudecode.workspace_dir", ".")
	v.SetDefault("claudecode.default_model", "sonnet")
	v.SetDefault("claudecode.timeout", "5m")
	v.SetDefault("claudecode.session_ttl", "24h")
	v.SetDefault("claudecode.backend.args", []string{"-p", "--output-format", "json", "--dangerously-skip-permissions"})
	v.SetDefault("claudecode.backend.resume_args", []string{"-p", "--output-format", "json", "--dangerously-skip-permissions", "--resume", "{sessionId}"})
	v.SetDefault("claudecode.backend.output", "json")
	v.SetDefault("claudecode.backend.input", "arg")
	v.SetDefault("claudecode.backend.max_prompt_arg_chars", 100000)
	v.SetDefault("claudecode.backend.model_arg", "--model")
	v.SetDefault("claudecode.backend.model_aliases", map[string]string{
		"opus":   "opus",
		"sonnet": "sonnet",
		"haiku":  "haiku",
	})
	v.SetDefault("claudecode.backend.session_arg", "--session-id")
	v.SetDefault("claudecode.backend.session_mode", "always")
	v.SetDefault("claudecode.backend.system_prompt_arg", "--append-system-prompt")
	v.SetDefault("claudecode.backend.system_prompt_mode", "append")
	v.SetDefault("claudecode.backend.system_prompt_when", "first")
	v.SetDefault("claudecode.backend.clear_env", []string{"ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY_OLD"})
	v.SetDefault("claudecode.backend.serialize", true)

	// Claude Code CLI defaults (v0.10.3) - New separated configuration
	// Master switch - strongly recommended to keep enabled
	v.SetDefault("claude_code_cli.enabled", true)
	// Installation settings
	v.SetDefault("claude_code_cli.install.path", "")
	v.SetDefault("claude_code_cli.install.auto_update", false)
	v.SetDefault("claude_code_cli.install.verify_checksum", true)
	// Download settings
	v.SetDefault("claude_code_cli.download.base_url", "https://storage.googleapis.com/anthropic-public/claude-code")
	v.SetDefault("claude_code_cli.download.cache_dir", "")
	// Feature toggles (only effective when enabled=true)
	v.SetDefault("claude_code_cli.features.skills", true)
	v.SetDefault("claude_code_cli.features.tool_calling", true)
	v.SetDefault("claude_code_cli.features.file_operations", true)
	v.SetDefault("claude_code_cli.features.terminal_commands", true)
	v.SetDefault("claude_code_cli.features.mcp_integration", true)
	v.SetDefault("claude_code_cli.features.agent_mode", true)
	v.SetDefault("claude_code_cli.features.project_context", true)
	// Backend settings (for advanced users)
	v.SetDefault("claude_code_cli.backend.command", "claude")
	v.SetDefault("claude_code_cli.backend.workspace_dir", ".")
	v.SetDefault("claude_code_cli.backend.default_model", "sonnet")
	v.SetDefault("claude_code_cli.backend.timeout", "5m")
	v.SetDefault("claude_code_cli.backend.session_ttl", "24h")
	v.SetDefault("claude_code_cli.backend.args", []string{"-p", "--output-format", "text", "--dangerously-skip-permissions"})
	v.SetDefault("claude_code_cli.backend.resume_args", []string{"-p", "--output-format", "text", "--dangerously-skip-permissions", "--resume", "{sessionId}"})
	v.SetDefault("claude_code_cli.backend.output", "text")
	v.SetDefault("claude_code_cli.backend.resume_output", "text")
	v.SetDefault("claude_code_cli.backend.input", "arg")
	v.SetDefault("claude_code_cli.backend.max_prompt_arg_chars", 100000)
	v.SetDefault("claude_code_cli.backend.model_arg", "--model")
	v.SetDefault("claude_code_cli.backend.model_aliases", map[string]string{
		"opus":   "opus",
		"sonnet": "sonnet",
		"haiku":  "haiku",
	})
	v.SetDefault("claude_code_cli.backend.session_arg", "--session-id")
	v.SetDefault("claude_code_cli.backend.session_mode", "always")
	v.SetDefault("claude_code_cli.backend.system_prompt_arg", "--append-system-prompt")
	v.SetDefault("claude_code_cli.backend.system_prompt_mode", "append")
	v.SetDefault("claude_code_cli.backend.system_prompt_when", "first")
	v.SetDefault("claude_code_cli.backend.serialize", true)

	// First-run wizard defaults (v0.10.3)
	v.SetDefault("first_run.enabled", true)
	v.SetDefault("first_run.show_provider_detection", true)
	v.SetDefault("first_run.show_cli_download", true)
	v.SetDefault("first_run.allow_skip", true)
	v.SetDefault("first_run.recommend_cli", true)

	// cc-switch integration defaults (v0.10.3)
	v.SetDefault("cc_switch.enabled", true)
	v.SetDefault("cc_switch.config_path", "")
	v.SetDefault("cc_switch.sync_profiles", true)

	// Statistics collection defaults (v0.10.3)
	v.SetDefault("statistics.enabled", true)
	v.SetDefault("statistics.opt_in_required", true)
	v.SetDefault("statistics.storage_path", "")
	v.SetDefault("statistics.retention_days", 90)

	// Tool calling adapter defaults (v0.10.3)
	v.SetDefault("tool_calling.auto_detect", true)
	v.SetDefault("tool_calling.detection_timeout", "5s")
	v.SetDefault("tool_calling.adapters.cli_proxy.enabled", true)
	v.SetDefault("tool_calling.adapters.cli_proxy.prompt_template", "default")
	v.SetDefault("tool_calling.adapters.cc_nexus.enabled", true)
	v.SetDefault("tool_calling.adapters.cc_nexus.schema_mapping", "auto")
	v.SetDefault("tool_calling.provider_overrides.ollama.tool_calling", "adapter")
	v.SetDefault("tool_calling.provider_overrides.custom.tool_calling", "auto")
}
