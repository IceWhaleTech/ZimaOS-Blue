package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Log         LogConfig         `mapstructure:"log"`
	Worker      WorkerConfig      `mapstructure:"worker"`
	Resources   ResourcesConfig   `mapstructure:"resources"`
	Cgroup      CgroupConfig      `mapstructure:"cgroup"`
	Channels    channel.Config    `mapstructure:"channels"`
	Performance PerformanceConfig `mapstructure:"performance"`
	Security    SecurityConfig    `mapstructure:"security"`
}

// SecurityConfig holds security-related configuration (v0.7).
type SecurityConfig struct {
	OIDC     OIDCConfig     `mapstructure:"oidc"`
	Users    UsersConfig    `mapstructure:"users"`
	Password PasswordConfig `mapstructure:"password"`
	MFA      MFAConfig      `mapstructure:"mfa"`
	Audit    AuditConfig    `mapstructure:"audit"`
	Sandbox  SandboxConfig  `mapstructure:"sandbox"`
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
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
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
}
