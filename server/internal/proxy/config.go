package proxy

import "time"

// ProxyConfig is the main proxy configuration
type ProxyConfig struct {
	Enabled      bool                `json:"enabled" yaml:"enabled"`
	Port         PortConfig          `json:"port" yaml:"port"`
	Route        *RouteConfig        `json:"route" yaml:"route"`
	Routing      RouteConfig         `json:"routing" yaml:"routing"` // Deprecated: use Route
	Connection   ConnectionConfig    `json:"connection" yaml:"connection"`
	HealthCheck  HealthCheckConfig   `json:"health_check" yaml:"health_check"`
	Mock         *MockConfig         `json:"mock" yaml:"mock"`
	ModelCompat  *ModelCompatConfig  `json:"model_compat" yaml:"model_compat"`
	Watcher      *WatcherConfig      `json:"watcher" yaml:"watcher"`
	Masking      *MaskingConfig      `json:"masking" yaml:"masking"`
	ModelRouter  *ModelRouterConfig  `json:"model_router" yaml:"model_router"`   // NEW: Model Router
	QuotaMonitor *QuotaMonitorConfig `json:"quota_monitor" yaml:"quota_monitor"` // NEW: Quota Monitor
	RuleRouting  *RoutingConfig      `json:"rule_routing" yaml:"rule_routing"`   // Condition-based routing rules
}

// PortConfig port configuration
type PortConfig struct {
	Value       int    `json:"value" yaml:"value"`               // 0 = dynamic allocation
	Range       string `json:"range" yaml:"range"`               // e.g., "9000-9100"
	BindAddress string `json:"bind_address" yaml:"bind_address"` // Default: "127.0.0.1"
	PortFile    string `json:"port_file" yaml:"port_file"`       // Write allocated port to file
}

// RouteConfig route configuration
type RouteConfig struct {
	DefaultProvider string            `json:"default_provider" yaml:"default_provider"`
	LoadBalancing   string            `json:"load_balancing" yaml:"load_balancing"` // priority, round-robin, weighted
	Providers       []*ProviderConfig `json:"providers" yaml:"providers"`
	Failover        FailoverConfig    `json:"failover" yaml:"failover"`
}

// ProviderConfig provider configuration
type ProviderConfig struct {
	Name          string `json:"name" yaml:"name"`
	Endpoint      string `json:"endpoint" yaml:"endpoint"`
	APIKey        string `json:"api_key" yaml:"api_key"`
	Priority      int    `json:"priority" yaml:"priority"`
	Weight        int    `json:"weight" yaml:"weight"`
	Enabled       bool   `json:"enabled" yaml:"enabled"`
	HealthCheck   string `json:"health_check" yaml:"health_check"`
	SkipTLSVerify bool   `json:"skip_tls_verify" yaml:"skip_tls_verify"`
}

// FailoverConfig failover configuration
type FailoverConfig struct {
	Enabled          bool          `json:"enabled" yaml:"enabled"`
	MaxRetries       int           `json:"max_retries" yaml:"max_retries"`
	RetryDelay       time.Duration `json:"retry_delay" yaml:"retry_delay"`
	CircuitBreaker   bool          `json:"circuit_breaker" yaml:"circuit_breaker"`
	FailureThreshold int           `json:"failure_threshold" yaml:"failure_threshold"`
	RecoveryTimeout  time.Duration `json:"recovery_timeout" yaml:"recovery_timeout"`

	// Transient error (502/503) circuit breaker overrides — shorter recovery for temporary blips
	TransientRecoveryTimeout time.Duration `json:"transient_recovery_timeout" yaml:"transient_recovery_timeout"` // 0 = use RecoveryTimeout

	// Smart failover settings
	ErrorClassification   ErrorClassificationConfig `json:"error_classification" yaml:"error_classification"`
	StreamingAnomaly      StreamingAnomalyConfig    `json:"streaming_anomaly" yaml:"streaming_anomaly"`
	ContextWindowCheck    bool                      `json:"context_window_check" yaml:"context_window_check"`       // Skip providers with insufficient context window
	QuotaCooldown         time.Duration             `json:"quota_cooldown" yaml:"quota_cooldown"`                   // Skip recently-errored providers for quota errors (0 = disabled)
	ContextWindowOverride map[string]int            `json:"context_window_override" yaml:"context_window_override"` // Provider name -> max context tokens override
	ProviderRace          ProviderRaceConfig        `json:"provider_race" yaml:"provider_race"`                     // Multi-provider concurrent race
}

// ProviderRaceConfig controls concurrent provider racing behavior.
type ProviderRaceConfig struct {
	Enabled                    bool          `json:"enabled" yaml:"enabled"`
	MaxParallel                int           `json:"max_parallel" yaml:"max_parallel"`                                   // Max providers to race concurrently
	MinProviders               int           `json:"min_providers" yaml:"min_providers"`                                 // Require at least N candidates to start race
	EmptyRateMinSamples        int           `json:"empty_rate_min_samples" yaml:"empty_rate_min_samples"`               // Min attempts before applying empty-rate policy
	EmptyRateCooldownThreshold float64       `json:"empty_rate_cooldown_threshold" yaml:"empty_rate_cooldown_threshold"` // >= threshold enters temporary cooldown
	EmptyRateSinkThreshold     float64       `json:"empty_rate_sink_threshold" yaml:"empty_rate_sink_threshold"`         // >= threshold sinks to tail
	EmptyRateExcludeThreshold  float64       `json:"empty_rate_exclude_threshold" yaml:"empty_rate_exclude_threshold"`   // >= threshold excluded from race
	EmptyRateCooldown          time.Duration `json:"empty_rate_cooldown" yaml:"empty_rate_cooldown"`                     // Cooldown duration after threshold hit
}

// ErrorClassificationConfig configuration for error classification
type ErrorClassificationConfig struct {
	Enabled         bool     `json:"enabled" yaml:"enabled"`
	FailoverErrors  []string `json:"failover_errors" yaml:"failover_errors"`   // Error types that trigger failover
	RetryableErrors []string `json:"retryable_errors" yaml:"retryable_errors"` // Error types that trigger retry
}

// ConnectionConfig connection pool configuration
type ConnectionConfig struct {
	MaxIdleConns          int           `json:"max_idle_conns" yaml:"max_idle_conns"`
	MaxIdleConnsPerHost   int           `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`
	MaxConnsPerHost       int           `json:"max_conns_per_host" yaml:"max_conns_per_host"`
	IdleConnTimeout       time.Duration `json:"idle_conn_timeout" yaml:"idle_conn_timeout"`
	KeepAlive             bool          `json:"keep_alive" yaml:"keep_alive"`
	KeepAliveInterval     time.Duration `json:"keep_alive_interval" yaml:"keep_alive_interval"`
	DialTimeout           time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	TLSHandshakeTimeout   time.Duration `json:"tls_handshake_timeout" yaml:"tls_handshake_timeout"`
	ResponseHeaderTimeout time.Duration `json:"response_header_timeout" yaml:"response_header_timeout"`
	ForceHTTP2            bool          `json:"force_http2" yaml:"force_http2"`
}

// HealthCheckConfig health check configuration
type HealthCheckConfig struct {
	Enabled  bool          `json:"enabled" yaml:"enabled"`
	Interval time.Duration `json:"interval" yaml:"interval"`
	Timeout  time.Duration `json:"timeout" yaml:"timeout"`
}

// DefaultProxyConfig returns default proxy configuration
func DefaultProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		Enabled: true,
		Port: PortConfig{
			Value:       0,
			Range:       "9000-9100",
			BindAddress: "127.0.0.1",
			PortFile:    "",
		},
		Routing: RouteConfig{
			DefaultProvider: "anthropic",
			LoadBalancing:   "priority",
			Providers:       []*ProviderConfig{},
			Failover: FailoverConfig{
				Enabled:                  true,
				MaxRetries:               3,
				RetryDelay:               time.Second,
				CircuitBreaker:           true,
				FailureThreshold:         5,
				RecoveryTimeout:          30 * time.Second,
				TransientRecoveryTimeout: 5 * time.Second, // 502/503 recover quickly
				ContextWindowCheck:       true,
				ProviderRace:             DefaultProviderRaceConfig(),
				ErrorClassification: ErrorClassificationConfig{
					Enabled: true,
					FailoverErrors: []string{
						"context_too_long",
						"quota_exceeded",
						"rate_limited",
						"model_overloaded",
						"service_unavailable",
					},
					RetryableErrors: []string{
						"timeout",
					},
				},
				StreamingAnomaly: DefaultStreamingAnomalyConfig(),
			},
		},
		Connection: ConnectionConfig{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   20,
			MaxConnsPerHost:       100,
			IdleConnTimeout:       120 * time.Second,
			KeepAlive:             true,
			KeepAliveInterval:     30 * time.Second,
			DialTimeout:           10 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Minute,
			ForceHTTP2:            true,
		},
		HealthCheck: HealthCheckConfig{
			Enabled:  true,
			Interval: 30 * time.Second,
			Timeout:  10 * time.Second,
		},
	}
}

// DefaultProviderRaceConfig returns default provider race configuration.
func DefaultProviderRaceConfig() ProviderRaceConfig {
	return ProviderRaceConfig{
		Enabled:                    false,
		MaxParallel:                2,
		MinProviders:               2,
		EmptyRateMinSamples:        10,
		EmptyRateCooldownThreshold: 0.30,
		EmptyRateSinkThreshold:     0.50,
		EmptyRateExcludeThreshold:  0.80,
		EmptyRateCooldown:          2 * time.Minute,
	}
}
