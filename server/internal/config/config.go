package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channelconfig"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
)

type Config struct {
	Server        ServerConfig         `yaml:"server"`
	Log           LogConfig            `yaml:"log"`
	Worker        WorkerConfig         `yaml:"worker"`
	Resources     ResourcesConfig      `yaml:"resources"`
	Cgroup        CgroupConfig         `yaml:"cgroup"`
	Channels      channelconfig.Config `yaml:"channels"`
	Performance   PerformanceConfig    `yaml:"performance"`
	Security      SecurityConfig       `yaml:"security"`
	LLM           LLMConfig            `yaml:"llm"`
	Session       SessionConfig        `yaml:"session"`
	Embedding     EmbeddingConfig      `yaml:"embedding"`
	Memory        MemoryConfig         `yaml:"memory"`
	Grayscale     GrayscaleConfig      `yaml:"grayscale"`
	Companion     CompanionConfig      `yaml:"companion"`
	ClaudeCodeCLI ClaudeCodeCLIConfig  `yaml:"claude_code_cli"` // v0.10.3: Separated from LLM
	FirstRun      FirstRunConfig       `yaml:"first_run"`       // v0.10.3
	CCSwitch      CCSwitchConfig       `yaml:"cc_switch"`       // v0.10.3
	Statistics    StatisticsConfig     `yaml:"statistics"`      // v0.10.3
	ToolCalling   ToolCallingConfig    `yaml:"tool_calling"`    // v0.10.3
	Media         MediaConfig          `yaml:"media"`
	SkillMarket   SkillMarketConfig    `yaml:"skill_market"`
	Browser       browser.Config       `yaml:"browser"`
	Agents        AgentsConfig         `yaml:"agents"`    // v0.11.0
	Research      ResearchConfig       `yaml:"research"`  // v0.11.x
	Harness       HarnessConfig        `yaml:"harness"`   // v0.11.x
	Proxy         *proxy.ProxyConfig   `yaml:"proxy"`     // v0.10.5.1: API Proxy
	Pruner        *pruner.Config       `yaml:"pruner"`    // v0.10.27: Context Pruner
	Update        UpdateConfig         `yaml:"update"`    // OTA Update
	Heartbeat     HeartbeatConfig      `yaml:"heartbeat"` // Heartbeat agent polling

	// Deprecated: Use ClaudeCodeCLI instead. Kept for backward compatibility.
	ClaudeCode ClaudeCodeConfig `yaml:"claudecode"`
}

// ClaudeCodeConfig holds Claude Code CLI integration configuration (v0.10).
// Deprecated: Use ClaudeCodeCLIConfig instead.
type ClaudeCodeConfig struct {
	Enabled      bool                    `yaml:"enabled"`
	Command      string                  `yaml:"command"`
	WorkspaceDir string                  `yaml:"workspace_dir"`
	DefaultModel string                  `yaml:"default_model"`
	Timeout      time.Duration           `yaml:"timeout"`
	SessionTTL   time.Duration           `yaml:"session_ttl"`
	APIKey       string                  `yaml:"api_key"`
	BaseURL      string                  `yaml:"base_url"`
	Backend      ClaudeCodeBackendConfig `yaml:"backend"`
}

// ClaudeCodeBackendConfig holds CLI backend configuration.
type ClaudeCodeBackendConfig struct {
	Args              []string          `yaml:"args"`
	ResumeArgs        []string          `yaml:"resume_args"`
	Output            string            `yaml:"output"`
	Input             string            `yaml:"input"`
	MaxPromptArgChars int               `yaml:"max_prompt_arg_chars"`
	Env               map[string]string `yaml:"env"`
	ClearEnv          []string          `yaml:"clear_env"`
	ModelArg          string            `yaml:"model_arg"`
	ModelAliases      map[string]string `yaml:"model_aliases"`
	SessionArg        string            `yaml:"session_arg"`
	SessionMode       string            `yaml:"session_mode"`
	SystemPromptArg   string            `yaml:"system_prompt_arg"`
	SystemPromptMode  string            `yaml:"system_prompt_mode"`
	SystemPromptWhen  string            `yaml:"system_prompt_when"`
	Serialize         bool              `yaml:"serialize"`
}

// CompanionConfig holds Echo Companion monitoring configuration (v0.9.1).
type CompanionConfig struct {
	Enabled     bool                       `yaml:"enabled"`
	Storage     CompanionStorageConfig     `yaml:"storage"`
	WebSocket   CompanionWebSocketConfig   `yaml:"websocket"`
	Retention   CompanionRetentionConfig   `yaml:"retention"`
	Alerts      CompanionAlertConfig       `yaml:"alerts"`
	Security    CompanionSecurityConfig    `yaml:"security"`
	Performance CompanionPerformanceConfig `yaml:"performance"`
}

// CompanionStorageConfig holds companion storage configuration.
type CompanionStorageConfig struct {
	BasePath string `yaml:"base_path"`
	Format   string `yaml:"format"`
}

// CompanionWebSocketConfig holds companion WebSocket configuration.
type CompanionWebSocketConfig struct {
	PingInterval    time.Duration `yaml:"ping_interval"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ReadBufferSize  int           `yaml:"read_buffer_size"`
	WriteBufferSize int           `yaml:"write_buffer_size"`
}

// CompanionRetentionConfig holds companion data retention configuration.
type CompanionRetentionConfig struct {
	EventsDays   int `yaml:"events_days"`
	SessionsDays int `yaml:"sessions_days"`
	AlertsDays   int `yaml:"alerts_days"`
}

// CompanionAlertConfig holds companion alert configuration.
type CompanionAlertConfig struct {
	Enabled         bool                    `yaml:"enabled"`
	ThreatThreshold string                  `yaml:"threat_threshold"`
	Channels        []CompanionAlertChannel `yaml:"channels"`
}

// CompanionAlertChannel represents an alert notification channel.
type CompanionAlertChannel struct {
	Type       string            `yaml:"type"`
	URL        string            `yaml:"url,omitempty"`
	Recipients []string          `yaml:"recipients,omitempty"`
	Headers    map[string]string `yaml:"headers,omitempty"`
}

// CompanionSecurityConfig holds companion security integration configuration.
type CompanionSecurityConfig struct {
	PromptGuardIntegration bool `yaml:"prompt_guard_integration"`
	AuditLogIntegration    bool `yaml:"audit_log_integration"`
	SandboxMonitor         bool `yaml:"sandbox_monitor"`
}

// CompanionPerformanceConfig holds companion performance configuration.
type CompanionPerformanceConfig struct {
	MaxConcurrentSessions int           `yaml:"max_concurrent_sessions"`
	EventBufferSize       int           `yaml:"event_buffer_size"`
	BatchWriteInterval    time.Duration `yaml:"batch_write_interval"`
}

// SecurityConfig holds security-related configuration (v0.7).
type SecurityConfig struct {
	JWT        JWTConfig        `yaml:"jwt"`
	OIDC       OIDCConfig       `yaml:"oidc"`
	Users      UsersConfig      `yaml:"users"`
	Password   PasswordConfig   `yaml:"password"`
	MFA        MFAConfig        `yaml:"mfa"`
	Audit      AuditConfig      `yaml:"audit"`
	Sandbox    SandboxConfig    `yaml:"sandbox"`
	Encryption EncryptionConfig `yaml:"encryption"`
}

// JWTConfig holds JWT authentication configuration.
type JWTConfig struct {
	Secret            string        `yaml:"secret"`
	Expiration        time.Duration `yaml:"expiration"`
	RefreshExpiration time.Duration `yaml:"refresh_expiration"`
	Issuer            string        `yaml:"issuer"`
}

// EncryptionConfig holds encryption configuration for sensitive data.
type EncryptionConfig struct {
	Enabled    bool   `yaml:"enabled"`
	KeyPath    string `yaml:"key_path"`
	Passphrase string `yaml:"passphrase"`
	Algorithm  string `yaml:"algorithm"` // "aes-256-gcm"
}

// OIDCConfig holds OIDC provider configuration.
type OIDCConfig struct {
	Enabled                bool          `yaml:"enabled"`
	Issuer                 string        `yaml:"issuer"`
	SigningKeyPath         string        `yaml:"signing_key_path"`
	SigningKeyRotationDays int           `yaml:"signing_key_rotation_days"`
	AccessTokenTTL         time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL        time.Duration `yaml:"refresh_token_ttl"`
	AuthorizationCodeTTL   time.Duration `yaml:"authorization_code_ttl"`
	Clients                []OIDCClient  `yaml:"clients"`
}

// OIDCClient holds OIDC client configuration.
type OIDCClient struct {
	ClientID          string   `yaml:"client_id"`
	ClientSecret      string   `yaml:"client_secret"`
	RedirectURIs      []string `yaml:"redirect_uris"`
	AllowedScopes     []string `yaml:"allowed_scopes"`
	AllowedGrantTypes []string `yaml:"allowed_grant_types"`
	Public            bool     `yaml:"public"`
}

// UsersConfig holds user management configuration.
type UsersConfig struct {
	AllowRegistration        bool   `yaml:"allow_registration"`
	RequireEmailVerification bool   `yaml:"require_email_verification"`
	DefaultRole              string `yaml:"default_role"`
}

// PasswordConfig holds password policy configuration.
type PasswordConfig struct {
	MinLength        int           `yaml:"min_length"`
	RequireUppercase bool          `yaml:"require_uppercase"`
	RequireLowercase bool          `yaml:"require_lowercase"`
	RequireNumber    bool          `yaml:"require_number"`
	RequireSpecial   bool          `yaml:"require_special"`
	HistoryCount     int           `yaml:"history_count"`
	ExpirationDays   int           `yaml:"expiration_days"`
	LockoutThreshold int           `yaml:"lockout_threshold"`
	LockoutDuration  time.Duration `yaml:"lockout_duration"`
}

// MFAConfig holds MFA configuration.
type MFAConfig struct {
	Enabled            bool   `yaml:"enabled"`
	Required           bool   `yaml:"required"`
	Issuer             string `yaml:"issuer"`
	RecoveryCodesCount int    `yaml:"recovery_codes_count"`
}

// AuditConfig holds audit logging configuration.
type AuditConfig struct {
	Enabled         bool          `yaml:"enabled"`
	RetentionDays   int           `yaml:"retention_days"`
	LogRequestBody  bool          `yaml:"log_request_body"`
	LogResponseBody bool          `yaml:"log_response_body"`
	ExcludedPaths   []string      `yaml:"excluded_paths"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

// SandboxConfig holds sandbox execution configuration.
type SandboxConfig struct {
	Enabled        bool          `yaml:"enabled"`
	DefaultTimeout time.Duration `yaml:"default_timeout"`
	MaxTimeout     time.Duration `yaml:"max_timeout"`
	MemoryLimit    string        `yaml:"memory_limit"`
	CPULimit       float64       `yaml:"cpu_limit"`
	ProcessLimit   int           `yaml:"process_limit"`
	NetworkEnabled bool          `yaml:"network_enabled"`
}

// ResourcesConfig holds resource limit configuration.
type ResourcesConfig struct {
	MaxMemoryMB   int64  `yaml:"max_memory_mb"`
	MaxCPUPercent int    `yaml:"max_cpu_percent"`
	MaxOpenFiles  uint64 `yaml:"max_open_files"`
	MaxGoroutines int    `yaml:"max_goroutines"`
	GCPercent     int    `yaml:"gc_percent"`
}

// CgroupConfig holds cgroup v2 configuration.
type CgroupConfig struct {
	Enabled    bool            `yaml:"enabled"`
	CgroupRoot string          `yaml:"cgroup_root"`
	CgroupName string          `yaml:"cgroup_name"`
	IO         CgroupIOConfig  `yaml:"io"`
	Memory     CgroupMemConfig `yaml:"memory"`
	CPU        CgroupCPUConfig `yaml:"cpu"`
}

// CgroupIOConfig holds IO bandwidth limit configuration.
type CgroupIOConfig struct {
	Enabled   bool     `yaml:"enabled"`
	ReadBPS   uint64   `yaml:"read_bps"`
	WriteBPS  uint64   `yaml:"write_bps"`
	ReadIOPS  uint64   `yaml:"read_iops"`
	WriteIOPS uint64   `yaml:"write_iops"`
	Devices   []string `yaml:"devices"`
}

// CgroupMemConfig holds cgroup memory limit configuration.
type CgroupMemConfig struct {
	Enabled      bool   `yaml:"enabled"`
	MaxBytes     uint64 `yaml:"max_bytes"`
	HighBytes    uint64 `yaml:"high_bytes"`
	SwapMaxBytes uint64 `yaml:"swap_max_bytes"`
}

// CgroupCPUConfig holds cgroup CPU limit configuration.
type CgroupCPUConfig struct {
	Enabled    bool `yaml:"enabled"`
	MaxPercent int  `yaml:"max_percent"`
	Weight     int  `yaml:"weight"`
}

// UpdateConfig holds OTA update configuration.
type UpdateConfig struct {
	Enabled        bool          `yaml:"enabled"`
	CheckInterval  time.Duration `yaml:"check_interval"`
	AutoDownload   bool          `yaml:"auto_download"`
	AutoApply      bool          `yaml:"auto_apply"`
	ReleaseChannel string        `yaml:"release_channel"`
	BackupCount    int           `yaml:"backup_count"`
	StoragePath    string        `yaml:"storage_path"`
}

// HeartbeatConfig holds heartbeat agent polling configuration.
type HeartbeatConfig struct {
	Enabled         bool                  `yaml:"enabled"`
	Interval        time.Duration         `yaml:"interval"`
	Prompt          string                `yaml:"prompt"`
	AckMaxChars     int                   `yaml:"ack_max_chars"`
	WorkspaceDir    string                `yaml:"workspace_dir"`
	LLMProvider     string                `yaml:"llm_provider"`
	LLMModel        string                `yaml:"llm_model"`
	ActiveHours     *HeartbeatActiveHours `yaml:"active_hours"`
	Visibility      HeartbeatVisibility   `yaml:"visibility"`
	DeliveryChannel string                `yaml:"delivery_channel"`
	DeliveryChatID  string                `yaml:"delivery_chat_id"`
}

// HeartbeatActiveHours defines the time window when heartbeat is allowed to run.
type HeartbeatActiveHours struct {
	Start    string `yaml:"start"`
	End      string `yaml:"end"`
	Timezone string `yaml:"timezone"`
}

// HeartbeatVisibility controls what heartbeat results are delivered.
type HeartbeatVisibility struct {
	ShowOk       bool `yaml:"show_ok"`
	ShowAlerts   bool `yaml:"show_alerts"`
	UseIndicator bool `yaml:"use_indicator"`
}

type ServerConfig struct {
	Host             string        `yaml:"host"`
	Port             int           `yaml:"port"`
	PortAutoFallback bool          `yaml:"port_auto_fallback"` // Auto fallback to random port if configured port is in use
	ReadTimeout      time.Duration `yaml:"read_timeout"`
	WriteTimeout     time.Duration `yaml:"write_timeout"`
	IdleTimeout      time.Duration `yaml:"idle_timeout"`
	TLS              TLSConfig     `yaml:"tls"`
}

// TLSConfig holds TLS/HTTPS configuration.
type TLSConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Port         int    `yaml:"port"`          // HTTPS port, default 443
	CertFile     string `yaml:"cert_file"`     // Path to certificate file
	KeyFile      string `yaml:"key_file"`      // Path to private key file
	AutoCert     bool   `yaml:"auto_cert"`     // Enable automatic certificate via ACME
	ACMEEmail    string `yaml:"acme_email"`    // Email for ACME registration
	ACMEDomains  string `yaml:"acme_domains"`  // Comma-separated domains for ACME
	ACMEProvider string `yaml:"acme_provider"` // letsencrypt, zerossl, or custom
	ACMEDir      string `yaml:"acme_dir"`      // Directory to store ACME certificates
	SelfSigned   bool   `yaml:"self_signed"`   // Generate self-signed certificate
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // json or console
	Output string `yaml:"output"` // stdout, stderr, or file path
}

type WorkerConfig struct {
	PoolSize    int `yaml:"pool_size"`
	MaxQueueLen int `yaml:"max_queue_len"`
}

// PerformanceConfig holds performance optimization configuration.
type PerformanceConfig struct {
	Database        DatabasePerfConfig        `yaml:"database"`
	Memory          MemoryPerfConfig          `yaml:"memory"`
	Concurrency     ConcurrencyPerfConfig     `yaml:"concurrency"`
	Network         NetworkPerfConfig         `yaml:"network"`
	Cache           CachePerfConfig           `yaml:"cache"`
	Profiling       ProfilingPerfConfig       `yaml:"profiling"`
	ResourceReclaim ResourceReclaimPerfConfig `yaml:"resource_reclaim"`
}

// DatabasePerfConfig holds database performance configuration.
type DatabasePerfConfig struct {
	PoolSize           int           `yaml:"pool_size"`
	MaxIdleConns       int           `yaml:"max_idle_conns"`
	ConnMaxLifetime    time.Duration `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime    time.Duration `yaml:"conn_max_idle_time"`
	WALMode            bool          `yaml:"wal_mode"`
	CacheSize          int           `yaml:"cache_size"`
	PageSize           int           `yaml:"page_size"`
	CheckpointInterval time.Duration `yaml:"checkpoint_interval"`
	BatchSize          int           `yaml:"batch_size"`
	SlowQueryThreshold time.Duration `yaml:"slow_query_threshold"`
	EnableQueryCache   bool          `yaml:"enable_query_cache"`
}

// ResourceReclaimPerfConfig holds idle reclaim thresholds for heavy lazy resources.
type ResourceReclaimPerfConfig struct {
	Enabled                       bool          `yaml:"enabled"`
	BrowserIdleAfter              time.Duration `yaml:"browser_idle_after"`
	WorkflowIdleAfter             time.Duration `yaml:"workflow_idle_after"`
	CronIdleAfter                 time.Duration `yaml:"cron_idle_after"`
	STTIdleAfter                  time.Duration `yaml:"stt_idle_after"`
	SpeechStatusDoesNotPrewarmSTT bool          `yaml:"speech_status_does_not_prewarm_stt"`
}

// MemoryPerfConfig holds memory performance configuration.
type MemoryPerfConfig struct {
	GOGC           int    `yaml:"gogc"`
	GOMemLimit     string `yaml:"gomemlimit"`
	BufferPoolSize int    `yaml:"buffer_pool_size"`
	ObjectPoolSize int    `yaml:"object_pool_size"`
}

// ConcurrencyPerfConfig holds concurrency performance configuration.
type ConcurrencyPerfConfig struct {
	WorkerPoolSize    int `yaml:"worker_pool_size"`
	MaxGoroutines     int `yaml:"max_goroutines"`
	ChannelBufferSize int `yaml:"channel_buffer_size"`
}

// NetworkPerfConfig holds network performance configuration.
type NetworkPerfConfig struct {
	HTTP2Enabled       bool          `yaml:"http2_enabled"`
	KeepAliveTimeout   time.Duration `yaml:"keep_alive_timeout"`
	CompressionEnabled bool          `yaml:"compression_enabled"`
	CompressionLevel   int           `yaml:"compression_level"`
	RequestTimeout     time.Duration `yaml:"request_timeout"`
	RetryMaxAttempts   int           `yaml:"retry_max_attempts"`
	RetryBackoffBase   time.Duration `yaml:"retry_backoff_base"`
}

// CachePerfConfig holds cache performance configuration.
type CachePerfConfig struct {
	L1Enabled      bool          `yaml:"l1_enabled"`
	L1Size         int           `yaml:"l1_size"`
	L1TTL          time.Duration `yaml:"l1_ttl"`
	L2Enabled      bool          `yaml:"l2_enabled"`
	L2Path         string        `yaml:"l2_path"`
	L2Size         string        `yaml:"l2_size"`
	L2TTL          time.Duration `yaml:"l2_ttl"`
	EvictionPolicy string        `yaml:"eviction_policy"`
}

// ProfilingPerfConfig holds profiling configuration.
type ProfilingPerfConfig struct {
	PprofEnabled     bool   `yaml:"pprof_enabled"`
	PprofPath        string `yaml:"pprof_path"`
	MetricsEnabled   bool   `yaml:"metrics_enabled"`
	BenchmarkEnabled bool   `yaml:"benchmark_enabled"`
}

func Load(configPath string) (*Config, error) {
	cfg := defaults()

	// Resolve config file path
	path := configPath
	if path == "" {
		path = findConfigFile()
	}

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && configPath == "" {
				// Auto-discovered path doesn't exist — use defaults
				return &cfg, nil
			}
			return nil, err
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	}

	// Override from environment variables
	applyEnvOverrides(&cfg)

	return &cfg, nil
}

// applyEnvOverrides applies BLUE_* environment variable overrides to the config.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("BLUE_SERVER_HOST"); v != "" {
		cfg.Server.Host = strings.TrimSpace(v)
	}
	if v := os.Getenv("BLUE_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("BLUE_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("BLUE_WEB_FETCH_ALLOW_PRIVATE_HOSTS"); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			cfg.ToolCalling.WebFetch.AllowPrivateHosts = enabled
		}
	}
	if v := os.Getenv("BLUE_WEB_FETCH_TIMEOUT"); v != "" {
		if timeout, err := time.ParseDuration(v); err == nil {
			cfg.ToolCalling.WebFetch.Timeout = timeout
		}
	}
	if v := os.Getenv("BLUE_WEB_FETCH_FIRECRAWL_TIMEOUT"); v != "" {
		if timeout, err := time.ParseDuration(v); err == nil {
			cfg.ToolCalling.WebFetch.FirecrawlTimeout = timeout
		}
	}
}

func parseStringListEnv(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var parsed []string
	if strings.HasPrefix(raw, "[") {
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
			out := make([]string, 0, len(parsed))
			for _, item := range parsed {
				trimmed := strings.TrimSpace(item)
				if trimmed != "" {
					out = append(out, trimmed)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseStringMapEnv(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	out := make(map[string]string, len(parsed))
	for key, value := range parsed {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// findConfigFile searches standard locations for config.yaml.
func findConfigFile() string {
	candidates := []string{
		"config.yaml",
		"config.yml",
		"config/config.yaml",
		"config/config.yml",
		"/etc/zimaos-blue/config.yaml",
		"/etc/zimaos-blue/config.yml",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".zimaos-blue", "config.yaml"),
			filepath.Join(home, ".zimaos-blue", "config.yml"),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func defaults() Config {
	return Config{
		Server: ServerConfig{
			Host: "0.0.0.0", Port: 80, PortAutoFallback: true,
			ReadTimeout: 5 * time.Minute, WriteTimeout: 5 * time.Minute, IdleTimeout: 120 * time.Second,
		},
		Log:    LogConfig{Level: "info", Format: "console", Output: "stdout"},
		Worker: WorkerConfig{PoolSize: 10, MaxQueueLen: 100},
		Resources: ResourcesConfig{
			MaxMemoryMB: 512, MaxCPUPercent: 50, MaxOpenFiles: 65536, MaxGoroutines: 10000, GCPercent: 100,
		},
		Cgroup: CgroupConfig{
			CgroupRoot: "/sys/fs/cgroup", CgroupName: "zimaos-blue",
			CPU: CgroupCPUConfig{Weight: 100},
		},

		Channels: func() channelconfig.Config {
			cfg := channelconfig.DefaultConfig()
			cfg.DefaultTimeoutSeconds = 300
			return cfg
		}(),
		Performance: PerformanceConfig{
			Database: DatabasePerfConfig{
				PoolSize: 10, MaxIdleConns: 5, ConnMaxLifetime: time.Hour, ConnMaxIdleTime: 10 * time.Minute, WALMode: true,
				CacheSize: 2000, PageSize: 4096, CheckpointInterval: 5 * time.Minute,
				BatchSize: 1000, SlowQueryThreshold: 100 * time.Millisecond, EnableQueryCache: true,
			},
			Memory:      MemoryPerfConfig{GOGC: 100, GOMemLimit: "512MB", BufferPoolSize: 1000, ObjectPoolSize: 500},
			Concurrency: ConcurrencyPerfConfig{WorkerPoolSize: 100, MaxGoroutines: 10000, ChannelBufferSize: 100},
			Network: NetworkPerfConfig{
				HTTP2Enabled: true, KeepAliveTimeout: 30 * time.Second, CompressionEnabled: true,
				CompressionLevel: 6, RequestTimeout: 5 * time.Minute, RetryMaxAttempts: 3, RetryBackoffBase: 100 * time.Millisecond,
			},
			Cache: CachePerfConfig{
				L1Enabled: true, L1Size: 1000, L1TTL: 5 * time.Minute,
				L2Enabled: true, L2Path: "./cache", L2Size: "100MB", L2TTL: time.Hour, EvictionPolicy: "lru",
			},
			Profiling: ProfilingPerfConfig{PprofEnabled: true, PprofPath: "/debug/pprof", MetricsEnabled: true},
			ResourceReclaim: ResourceReclaimPerfConfig{
				Enabled:                       true,
				BrowserIdleAfter:              10 * time.Minute,
				WorkflowIdleAfter:             15 * time.Minute,
				CronIdleAfter:                 15 * time.Minute,
				STTIdleAfter:                  10 * time.Minute,
				SpeechStatusDoesNotPrewarmSTT: true,
			},
		},

		Security: SecurityConfig{
			JWT:        JWTConfig{Secret: defaultJWTSecretPlaceholder, Expiration: 24 * time.Hour, RefreshExpiration: 720 * time.Hour, Issuer: "zimaos-blue"},
			OIDC:       OIDCConfig{Enabled: true, Issuer: "http://localhost", SigningKeyPath: "./keys/oidc.key", SigningKeyRotationDays: 90, AccessTokenTTL: time.Hour, RefreshTokenTTL: 720 * time.Hour, AuthorizationCodeTTL: 10 * time.Minute},
			Users:      UsersConfig{DefaultRole: "user"},
			Password:   PasswordConfig{MinLength: 8, RequireUppercase: true, RequireLowercase: true, RequireNumber: true, RequireSpecial: true, HistoryCount: 5, LockoutThreshold: 5, LockoutDuration: 15 * time.Minute},
			MFA:        MFAConfig{Enabled: true, Issuer: "ZimaOS-Blue", RecoveryCodesCount: 8},
			Audit:      AuditConfig{Enabled: true, RetentionDays: 90, ExcludedPaths: []string{"/health", "/metrics"}, CleanupInterval: 24 * time.Hour},
			Sandbox:    SandboxConfig{Enabled: true, DefaultTimeout: 5 * time.Minute, MaxTimeout: 5 * time.Minute, MemoryLimit: "256MB", CPULimit: 1.0, ProcessLimit: 10},
			Encryption: EncryptionConfig{KeyPath: "./keys/encryption.key", Algorithm: "aes-256-gcm"},
		},

		LLM: LLMConfig{
			HealthCheck: LLMHealthCheckConfig{Enabled: true, Interval: 30 * time.Second, Timeout: 5 * time.Second, UnhealthyThreshold: 3, RecoveryThreshold: 2},
			Metrics:     LLMMetricsConfig{Enabled: true, IncludeLatencyHistogram: true, IncludeTokenCounts: true, IncludeErrorBreakdown: true},
		},
		Session: SessionConfig{
			MaxTokens: 0, MaxMessages: 100, IdleTimeout: 30 * time.Minute,
			Compaction:  SessionCompactionConfig{Enabled: true, Threshold: 0.8, Strategy: "summarize", SummaryMaxTokens: 500, PreserveRecent: 5, AutoCompact: true, AutoCompactInterval: 5 * time.Minute},
			Persistence: SessionPersistenceConfig{Enabled: true, Path: "./data/blue.db", Interval: time.Minute, OnMessage: true, OnCompact: true},
			Isolation:   SessionIsolationConfig{ByAgent: true, ByChannel: true, ByPeer: true},
			Cleanup:     SessionCleanupConfig{Enabled: true, ArchiveAfter: 168 * time.Hour, DeleteAfter: 720 * time.Hour, CleanupInterval: time.Hour},
			Audit: SessionAuditConfig{
				Enabled:          true,
				Path:             "",
				RetentionDays:    30,
				CleanupInterval:  6 * time.Hour,
				CleanupBatchSize: 500,
			},
		},
		Embedding: EmbeddingConfig{
			Enabled: false, Provider: "cybertron", Model: "BAAI/bge-small-zh-v1.5", Dimensions: 0, BatchSize: 100, Timeout: 5 * time.Minute,
			Cache:  EmbeddingCacheConfig{Enabled: true, MaxEntries: 10000, TTL: 24 * time.Hour},
			OpenAI: OpenAIEmbeddingConfig{BaseURL: "https://api.openai.com"},
			Ollama: OllamaEmbeddingConfig{BaseURL: "http://localhost:11434"},
		},
		Memory: MemoryConfig{
			VectorStore: VectorStoreConfig{Enabled: true, DBPath: "./data/memory.db", Dimensions: 0},
			Search:      MemorySearchConfig{VectorWeight: 0.7, KeywordWeight: 0.3, MinScore: 0.5, MaxResults: 10},
		},

		Companion: CompanionConfig{
			Enabled:     true,
			Storage:     CompanionStorageConfig{BasePath: "./data/companion", Format: "jsonl"},
			WebSocket:   CompanionWebSocketConfig{PingInterval: 30 * time.Second, WriteTimeout: 10 * time.Second, ReadBufferSize: 1024, WriteBufferSize: 1024},
			Retention:   CompanionRetentionConfig{EventsDays: 7, SessionsDays: 30, AlertsDays: 90},
			Alerts:      CompanionAlertConfig{Enabled: true, ThreatThreshold: "medium"},
			Security:    CompanionSecurityConfig{PromptGuardIntegration: true, AuditLogIntegration: true, SandboxMonitor: true},
			Performance: CompanionPerformanceConfig{MaxConcurrentSessions: 1000, EventBufferSize: 10000, BatchWriteInterval: time.Second},
		},
		ClaudeCode: ClaudeCodeConfig{
			Command: "claude", WorkspaceDir: ".", DefaultModel: "sonnet", Timeout: 5 * time.Minute, SessionTTL: 24 * time.Hour,
			Backend: ClaudeCodeBackendConfig{
				Args:       []string{"-p", "--output-format", "json", "--dangerously-skip-permissions"},
				ResumeArgs: []string{"-p", "--output-format", "json", "--dangerously-skip-permissions", "--resume", "{sessionId}"},
				Output:     "json", Input: "arg", MaxPromptArgChars: 100000, ModelArg: "--model",
				ModelAliases: map[string]string{"opus": "opus", "sonnet": "sonnet", "haiku": "haiku"},
				SessionArg:   "--session-id", SessionMode: "always",
				SystemPromptArg: "--append-system-prompt", SystemPromptMode: "append", SystemPromptWhen: "first",
				ClearEnv: []string{"ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY_OLD"}, Serialize: true,
			},
		},

		ClaudeCodeCLI: *DefaultClaudeCodeCLIConfig(),
		FirstRun:      *DefaultFirstRunConfig(),
		CCSwitch:      *DefaultCCSwitchConfig(),
		Statistics:    *DefaultStatisticsConfig(),
		ToolCalling:   *DefaultToolCallingConfig(),
		Media:         *DefaultMediaConfig(),
		SkillMarket:   *DefaultSkillMarketConfig(),
		Browser:       *browser.DefaultConfig(),
		Agents:        *DefaultAgentsConfig(),
		Research:      *DefaultResearchConfig(),
		Harness:       *DefaultHarnessConfig(),

		Proxy: &proxy.ProxyConfig{
			Enabled: true,
			Port:    proxy.PortConfig{Range: "9000-9100", BindAddress: "127.0.0.1"},
			Routing: proxy.RouteConfig{DefaultProvider: "anthropic", LoadBalancing: "priority",
				Failover: proxy.FailoverConfig{Enabled: true, MaxRetries: 3, RetryDelay: time.Second, CircuitBreaker: true, FailureThreshold: 5, RecoveryTimeout: 30 * time.Second}},
			Connection: proxy.ConnectionConfig{
				MaxIdleConns: 100, MaxIdleConnsPerHost: 10, MaxConnsPerHost: 100, IdleConnTimeout: 90 * time.Second,
				KeepAlive: true, KeepAliveInterval: 30 * time.Second, DialTimeout: 30 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 10 * time.Minute, ForceHTTP2: true,
			},
			HealthCheck:  proxy.HealthCheckConfig{Enabled: true, Interval: 30 * time.Second, Timeout: 10 * time.Second},
			ModelRouter:  &proxy.ModelRouterConfig{Enabled: false, DefaultFamily: "claude-3"},
			QuotaMonitor: &proxy.QuotaMonitorConfig{Enabled: true, SyncInterval: 5 * time.Minute, WarningThreshold: 20.0, CriticalThreshold: 5.0, TrackTokens: true, TrackRequests: true},
		},
		// Keep default trigger threshold moderate so pruning is observable in real chats.
		Pruner: &pruner.Config{Enabled: true, Backend: "local", Threshold: 0.5, MinLines: 80, TimeoutMs: 5000},
		Heartbeat: HeartbeatConfig{
			Interval: 30 * time.Minute, AckMaxChars: 300, WorkspaceDir: "./data", LLMProvider: "claude", LLMModel: "claude-sonnet-4-5-20250929",
			Prompt:     "Read HEARTBEAT.md if it exists (workspace context). Follow it strictly. Do not infer or repeat old tasks from prior chats. If nothing needs attention, reply HEARTBEAT_OK.",
			Visibility: HeartbeatVisibility{ShowAlerts: true, UseIndicator: true},
		},
		Update: UpdateConfig{
			Enabled: true, CheckInterval: 24 * time.Hour, ReleaseChannel: "stable", BackupCount: 3, StoragePath: "./data/updates",
		},
	}
}
