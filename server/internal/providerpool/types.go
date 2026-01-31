// Package providerpool provides unified LLM provider management for ZimaOS-Echo.
// It supports multiple providers, intelligent routing, and usage tracking.
package providerpool

import (
	"time"
)

// ProviderType represents the type of provider
type ProviderType string

const (
	// ProviderTypeBuiltin represents built-in providers (OpenAI, Anthropic, etc.)
	ProviderTypeBuiltin ProviderType = "builtin"
	// ProviderTypeCustom represents user-defined OpenAI-compatible providers
	ProviderTypeCustom ProviderType = "custom"
	// ProviderTypeACP represents ACP (Agent Communication Protocol) providers
	ProviderTypeACP ProviderType = "acp"
	// ProviderTypeIDE represents providers discovered from local IDE tools
	ProviderTypeIDE ProviderType = "ide"
)

// ProviderLocation represents where the provider runs
type ProviderLocation string

const (
	// ProviderLocationCloud represents cloud-based providers (OpenAI, Anthropic, etc.)
	ProviderLocationCloud ProviderLocation = "cloud"
	// ProviderLocationLocal represents locally running providers (Ollama, LM Studio, etc.)
	ProviderLocationLocal ProviderLocation = "local"
)

// ProviderStatus represents the current status of a provider
type ProviderStatus string

const (
	// ProviderStatusActive indicates the provider is healthy and available
	ProviderStatusActive ProviderStatus = "active"
	// ProviderStatusInactive indicates the provider is disabled
	ProviderStatusInactive ProviderStatus = "inactive"
	// ProviderStatusError indicates the provider has connectivity issues
	ProviderStatusError ProviderStatus = "error"
)

// Provider represents an LLM provider configuration
type Provider struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Type        ProviderType     `json:"type"`
	Location    ProviderLocation `json:"location"`  // cloud or local
	Enabled     bool             `json:"enabled"`
	Status      ProviderStatus   `json:"status"`
	BaseURL     string           `json:"base_url,omitempty"`
	APIVersion  string           `json:"api_version,omitempty"` // e.g., "v1", "2024-01"

	// Authentication
	APIKeys []APIKey     `json:"api_keys,omitempty"`
	OAuth   *OAuthConfig `json:"oauth,omitempty"`

	// Configuration
	Priority    int              `json:"priority"`               // Higher = preferred
	RateLimit   *RateLimitConfig `json:"rate_limit,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`      // Custom headers

	// Model Parameters (defaults for this provider)
	ModelParams *ModelParams `json:"model_params,omitempty"`

	// Allowed Models (if set, only these models are available; if empty, all models from API are available)
	AllowedModels []string `json:"allowed_models,omitempty"`

	// Metadata
	Icon        string    `json:"icon,omitempty"`        // Built-in icon name (e.g., "openai", "anthropic")
	CustomIcon  string    `json:"custom_icon,omitempty"` // Custom icon: base64 data URL or relative file path
	Description string    `json:"description,omitempty"`
	Website     string    `json:"website,omitempty"`     // Official website URL for the provider
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Health check
	LastHealthCheck time.Time `json:"last_health_check,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
	LastErrorTime   time.Time `json:"last_error_time,omitempty"`
}

// APIKey represents a single API key with metadata
type APIKey struct {
	ID         string    `json:"id"`
	Key        string    `json:"-"`         // Never expose in JSON
	KeyHash    string    `json:"key_hash"`  // For identification (first 8 + last 4 chars)
	Label      string    `json:"label,omitempty"`
	UsageCount int64     `json:"usage_count"`
	LastUsed   time.Time `json:"last_used,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	Enabled    bool      `json:"enabled"`
}

// OAuthConfig represents OAuth configuration for providers that support it
type OAuthConfig struct {
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"-"` // Never expose
	AccessToken  string    `json:"-"` // Never expose
	RefreshToken string    `json:"-"` // Never expose
	TokenExpiry  time.Time `json:"token_expiry,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute,omitempty"`
	TokensPerMinute   int `json:"tokens_per_minute,omitempty"`
	TokensPerDay      int `json:"tokens_per_day,omitempty"`
}

// ModelParams represents default model parameters for a provider
type ModelParams struct {
	Temperature      *float64 `json:"temperature,omitempty"`       // 0.0 - 2.0, nil means use model default
	MaxTokens        *int     `json:"max_tokens,omitempty"`        // Max output tokens, nil means use model default
	TopP             *float64 `json:"top_p,omitempty"`             // 0.0 - 1.0
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"` // -2.0 - 2.0
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`  // -2.0 - 2.0

	// Server-detected capabilities (read-only, populated by capability detection)
	DetectedMaxTokens *int  `json:"detected_max_tokens,omitempty"` // Actual max tokens supported by server
	DetectedAt        *int64 `json:"detected_at,omitempty"`         // Unix timestamp of last detection
}

// Model represents an available model from a provider
type Model struct {
	ID          string            `json:"id"`
	ProviderID  string            `json:"provider_id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Enabled     bool              `json:"enabled"`

	// Capabilities
	Capabilities ModelCapabilities `json:"capabilities"`

	// Pricing (per 1M tokens, in USD)
	InputPrice  float64 `json:"input_price,omitempty"`
	OutputPrice float64 `json:"output_price,omitempty"`
	CachePrice  float64 `json:"cache_price,omitempty"` // Cache read price

	// Limits
	ContextWindow int `json:"context_window,omitempty"`
	MaxOutput     int `json:"max_output,omitempty"`

	// Metadata
	Description string    `json:"description,omitempty"`
	Deprecated  bool      `json:"deprecated,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// ModelCapabilities describes what a model can do
type ModelCapabilities struct {
	Chat         bool `json:"chat"`
	Completion   bool `json:"completion"`
	Vision       bool `json:"vision"`
	FunctionCall bool `json:"function_call"`
	Streaming    bool `json:"streaming"`
	Thinking     bool `json:"thinking"`      // Extended thinking mode (Claude)
	JSON         bool `json:"json"`          // JSON mode support
	SystemPrompt bool `json:"system_prompt"` // System prompt support
}

// UsageRecord tracks usage per provider/model
type UsageRecord struct {
	ID         string    `json:"id"`
	ProviderID string    `json:"provider_id"`
	ModelID    string    `json:"model_id"`
	APIKeyID   string    `json:"api_key_id,omitempty"`
	Timestamp  time.Time `json:"timestamp"`

	// Token counts
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int64 `json:"cache_write_tokens,omitempty"`

	// Request metrics
	RequestCount int64 `json:"request_count"`
	LatencyMs    int64 `json:"latency_ms"`
	Success      bool  `json:"success"`

	// Cost estimation
	EstimatedCost float64 `json:"estimated_cost,omitempty"`

	// Context
	SessionID string `json:"session_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

// UsageSummary aggregates usage statistics
type UsageSummary struct {
	ProviderID string `json:"provider_id,omitempty"`
	ModelID    string `json:"model_id,omitempty"`
	Period     string `json:"period"` // "day", "week", "month"

	// Aggregated metrics
	TotalInputTokens      int64   `json:"total_input_tokens"`
	TotalOutputTokens     int64   `json:"total_output_tokens"`
	TotalCacheReadTokens  int64   `json:"total_cache_read_tokens"`
	TotalCacheWriteTokens int64   `json:"total_cache_write_tokens"`
	TotalRequests         int64   `json:"total_requests"`
	SuccessfulRequests    int64   `json:"successful_requests"`
	FailedRequests        int64   `json:"failed_requests"`
	TotalEstimatedCost    float64 `json:"total_estimated_cost"`

	// Latency stats
	AvgLatencyMs int64 `json:"avg_latency_ms"`
	MinLatencyMs int64 `json:"min_latency_ms"`
	MaxLatencyMs int64 `json:"max_latency_ms"`
	P95LatencyMs int64 `json:"p95_latency_ms"`

	// Time range
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// RoutingStrategy defines how to select providers
type RoutingStrategy string

const (
	// RoutingStrategyPriority selects by provider priority
	RoutingStrategyPriority RoutingStrategy = "priority"
	// RoutingStrategyCost selects the cheapest provider
	RoutingStrategyCost RoutingStrategy = "cost"
	// RoutingStrategyLatency selects the fastest provider
	RoutingStrategyLatency RoutingStrategy = "latency"
	// RoutingStrategyRoundRobin rotates through providers
	RoutingStrategyRoundRobin RoutingStrategy = "round_robin"
)

// RoutingMode defines the location preference for routing
type RoutingMode string

const (
	// RoutingModeAuto uses intelligent routing across all providers
	RoutingModeAuto RoutingMode = "auto"
	// RoutingModeCloud only uses cloud providers
	RoutingModeCloud RoutingMode = "cloud"
	// RoutingModeLocal only uses local providers
	RoutingModeLocal RoutingMode = "local"
)

// RouteRequest represents a routing request
type RouteRequest struct {
	ModelID    string             `json:"model_id"`
	Strategy   RoutingStrategy    `json:"strategy,omitempty"`
	Mode       RoutingMode        `json:"mode,omitempty"`        // Location preference: auto, cloud, local
	Exclude    []string           `json:"exclude,omitempty"`     // Provider IDs to exclude
	RequireCap *ModelCapabilities `json:"require_cap,omitempty"` // Required capabilities
}

// RouteResult represents the routing decision
type RouteResult struct {
	Provider  *Provider `json:"provider"`
	Model     *Model    `json:"model"`
	APIKey    *APIKey   `json:"api_key,omitempty"`
	Fallbacks []*RouteCandidate `json:"fallbacks,omitempty"`
}

// RouteCandidate represents a potential routing target
type RouteCandidate struct {
	Provider *Provider `json:"provider"`
	Model    *Model    `json:"model"`
	Score    float64   `json:"score"` // Routing score (higher = better)
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	ProviderID string        `json:"provider_id"`
	Healthy    bool          `json:"healthy"`
	Latency    time.Duration `json:"latency"`
	Error      string        `json:"error,omitempty"`
	CheckedAt  time.Time     `json:"checked_at"`
}

// IDEProvider represents a provider discovered from a local IDE
type IDEProvider struct {
	IDEName     string    `json:"ide_name"`     // e.g., "antigravity", "cursor"
	IDEVersion  string    `json:"ide_version,omitempty"`
	ProxyURL    string    `json:"proxy_url"`    // Local proxy endpoint
	ConfigPath  string    `json:"config_path"`  // Path to IDE config
	Connected   bool      `json:"connected"`
	Models      []string  `json:"models,omitempty"`
	DiscoveredAt time.Time `json:"discovered_at"`
}

// PoolConfig represents the provider pool configuration
type PoolConfig struct {
	// Routing
	DefaultStrategy    RoutingStrategy `json:"default_strategy"`
	DefaultRoutingMode RoutingMode     `json:"default_routing_mode"` // auto, cloud, local

	// Health check
	HealthCheckEnabled  bool          `json:"health_check_enabled"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`

	// IDE discovery
	IDEDiscoveryEnabled      bool          `json:"ide_discovery_enabled"`
	IDEDiscoveryScanInterval time.Duration `json:"ide_discovery_scan_interval"`

	// Usage tracking
	UsageTrackingEnabled bool `json:"usage_tracking_enabled"`
	UsageRetentionDays   int  `json:"usage_retention_days"`

	// Encryption
	EncryptionKey string `json:"-"` // Never expose
}

// ModelPricing represents custom pricing configuration for a model
type ModelPricing struct {
	ModelID     string    `json:"model_id"`               // Model identifier (can be pattern like "gpt-*")
	ProviderID  string    `json:"provider_id,omitempty"`  // Optional: specific provider
	InputPrice  float64   `json:"input_price"`            // Price per 1M input tokens (USD)
	OutputPrice float64   `json:"output_price"`           // Price per 1M output tokens (USD)
	CachePrice  float64   `json:"cache_price,omitempty"`  // Price per 1M cache read tokens (USD)
	IsCustom    bool      `json:"is_custom"`              // True if user-defined, false if default
	UpdatedAt   time.Time `json:"updated_at"`
}

// PricingConfig holds all pricing configurations
type PricingConfig struct {
	// Default price for unknown models (per 1M tokens)
	DefaultInputPrice  float64 `json:"default_input_price"`
	DefaultOutputPrice float64 `json:"default_output_price"`
	DefaultCachePrice  float64 `json:"default_cache_price"`

	// Custom pricing overrides (model_id -> pricing)
	CustomPricing map[string]*ModelPricing `json:"custom_pricing"`

	// Last update timestamp
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultPricingConfig returns the default pricing configuration
func DefaultPricingConfig() *PricingConfig {
	return &PricingConfig{
		// Default prices for unknown models (conservative estimate)
		DefaultInputPrice:  5.0,  // $5 per 1M input tokens
		DefaultOutputPrice: 15.0, // $15 per 1M output tokens
		DefaultCachePrice:  0.5,  // $0.5 per 1M cache tokens
		CustomPricing:      make(map[string]*ModelPricing),
		UpdatedAt:          time.Now(),
	}
}
