// Package providerpool provides unified LLM provider management for ZimaOS-Blue.
// It supports multiple providers, intelligent routing, and usage tracking.
package providerpool

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ProviderType represents the type of provider
type ProviderType string

const (
	// ProviderTypeBuiltin represents built-in providers (OpenAI, Anthropic, etc.)
	ProviderTypeBuiltin ProviderType = "builtin"
	// ProviderTypePlatform represents platform/aggregator providers (OpenRouter, SiliconFlow, etc.)
	ProviderTypePlatform ProviderType = "platform"
	// ProviderTypeCustom represents user-defined OpenAI-compatible providers
	ProviderTypeCustom ProviderType = "custom"
	// ProviderTypeACP represents ACP (Agent Communication Protocol) providers
	ProviderTypeACP ProviderType = "acp"
	// ProviderTypeIDE represents providers discovered from local IDE tools
	ProviderTypeIDE ProviderType = "ide"
	// ProviderTypeTrial represents trial providers with limited quota
	ProviderTypeTrial ProviderType = "trial"
	// ProviderTypeMedia represents media generation providers (image/video)
	ProviderTypeMedia ProviderType = "media"
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

// APIFormat represents the API format used by a provider
type APIFormat string

const (
	// APIFormatOpenAI represents OpenAI-compatible API format (default)
	APIFormatOpenAI APIFormat = "openai"
	// APIFormatResponses represents Responses API format (e.g. /v1/responses, /backend-api/codex/responses)
	APIFormatResponses APIFormat = "responses"
	// APIFormatAnthropic represents Anthropic Claude API format
	APIFormatAnthropic APIFormat = "anthropic"
	// APIFormatOllama represents Ollama API format
	APIFormatOllama APIFormat = "ollama"
	// APIFormatGoogle represents Google AI API format
	APIFormatGoogle APIFormat = "google"
	// APIFormatCloudCode represents Google Cloud Code Assist API format (used by Antigravity/Gemini CLI OAuth)
	APIFormatCloudCode APIFormat = "cloudcode"
	// APIFormatCopilot represents GitHub Copilot API format (OpenAI-compatible with Copilot auth)
	APIFormatCopilot APIFormat = "copilot"
)

// APIFormatMode controls whether a provider uses a fixed format or runtime resolution.
type APIFormatMode string

const (
	// APIFormatModeAuto lets the runtime resolve the best format for the request.
	APIFormatModeAuto APIFormatMode = "auto"
	// APIFormatModePinned forces a single configured format.
	APIFormatModePinned APIFormatMode = "pinned"
)

// Provider represents an LLM provider configuration
type Provider struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Type          ProviderType     `json:"type"`
	Location      ProviderLocation `json:"location"` // cloud or local
	Enabled       bool             `json:"enabled"`
	Status        ProviderStatus   `json:"status"`
	BaseURL       string           `json:"base_url,omitempty"`
	APIVersion    string           `json:"api_version,omitempty"`     // e.g., "v1", "2024-01"
	APIFormat     APIFormat        `json:"api_format,omitempty"`      // openai, anthropic, ollama, google (auto-detected if empty)
	APIFormatMode APIFormatMode    `json:"api_format_mode,omitempty"` // auto or pinned
	SkipTLSVerify bool             `json:"skip_tls_verify,omitempty"` // skip TLS certificate verification for self-signed certs

	// Authentication
	APIKeys []APIKey     `json:"api_keys,omitempty"`
	OAuth   *OAuthConfig `json:"oauth,omitempty"`

	// Configuration
	Priority  int               `json:"priority"` // Higher = preferred
	RateLimit *RateLimitConfig  `json:"rate_limit,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"` // Custom headers

	// Model Parameters (defaults for this provider)
	ModelParams *ModelParams `json:"model_params,omitempty"`

	// Allowed Models holds the allowlist when AllowlistConfigured is true.
	// Nil/empty here are interpreted by discovery using AllowlistConfigured.
	AllowedModels []string `json:"allowed_models,omitempty"`
	// AllowlistConfigured tracks whether model allowlist mode is enabled.
	// This preserves "configured but empty" semantics across JSON persistence.
	AllowlistConfigured bool `json:"allowlist_configured,omitempty"`

	// Metadata
	Icon        string    `json:"icon,omitempty"`        // Built-in icon name (e.g., "openai", "anthropic")
	CustomIcon  string    `json:"custom_icon,omitempty"` // Custom icon: base64 data URL or relative file path
	Description string    `json:"description,omitempty"`
	Website     string    `json:"website,omitempty"`     // Official website URL for the provider
	APIKeyURL   string    `json:"api_key_url,omitempty"` // URL to obtain/manage API keys
	Beta        bool      `json:"beta,omitempty"`        // Beta providers are shown in "Other" with a beta badge
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Detected capabilities (persisted across restarts)
	DetectedFormat APIFormat `json:"detected_format,omitempty"` // Probed API format that works (persisted)
	DetectedAt     time.Time `json:"detected_at,omitempty"`     // When format was last probed

	// AlternateBaseURLs is a list of alternate base URLs to try when auth fails on the primary BaseURL.
	// The system will probe these in order when the primary endpoint returns 401/403.
	// When an alternate URL succeeds, it's persisted as the new BaseURL via DetectedEndpoint.
	AlternateBaseURLs []string `json:"alternate_base_urls,omitempty"`

	// DetectedEndpoint remembers which endpoint worked (full base URL without path).
	// This is persisted and survives restarts. When set, it overrides BaseURL.
	DetectedEndpoint string `json:"detected_endpoint,omitempty"`

	// Health check
	LastHealthCheck time.Time `json:"last_health_check,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
	LastErrorTime   time.Time `json:"last_error_time,omitempty"`

	// Cached parsed URL — lazily initialized, avoids url.Parse on every request
	parsedURL     *url.URL  `json:"-"`
	parsedURLOnce sync.Once `json:"-"`
}

// ParsedBaseURL returns the cached parsed URL for this provider.
// Thread-safe, parsed once on first call. Returns nil if BaseURL is invalid.
// Use EffectiveBaseURL() for the actual URL to use (considers DetectedEndpoint).
func (p *Provider) ParsedBaseURL() *url.URL {
	p.parsedURLOnce.Do(func() {
		p.parsedURL, _ = url.Parse(p.BaseURL)
	})
	return p.parsedURL
}

// EffectiveBaseURL returns the effective base URL to use, considering DetectedEndpoint.
// If DetectedEndpoint is set, it takes precedence over BaseURL.
func (p *Provider) EffectiveBaseURL() string {
	if p.DetectedEndpoint != "" {
		return p.DetectedEndpoint
	}
	return p.BaseURL
}

// ParsedEffectiveBaseURL returns the cached parsed effective base URL.
// Thread-safe, parsed once on first call.
func (p *Provider) ParsedEffectiveBaseURL() *url.URL {
	p.parsedURLOnce.Do(func() {
		p.parsedURL, _ = url.Parse(p.EffectiveBaseURL())
	})
	return p.parsedURL
}

// ResetParsedURL invalidates the cached parsed URL so it will be re-parsed on next access.
// Call this after changing BaseURL or DetectedEndpoint.
func (p *Provider) ResetParsedURL() {
	p.parsedURLOnce = sync.Once{}
}

// APIKey represents a single API key with metadata
type APIKey struct {
	ID         string    `json:"id"`
	Key        string    `json:"-"`        // Never expose in JSON
	KeyHash    string    `json:"key_hash"` // For identification (first 8 + last 4 chars)
	KeyDigest  string    `json:"-"`        // Internal full fingerprint for dedupe/storage only
	Label      string    `json:"label,omitempty"`
	UsageCount int64     `json:"usage_count"`
	LastUsed   time.Time `json:"last_used,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	Enabled    bool      `json:"enabled"`
}

// OAuthConfig represents OAuth template configuration for providers that support it.
// The actual connected accounts (tokens) are stored in the oauth_tokens table.
// This struct on Provider holds the template (provider_type, scopes, endpoint, etc.).
type OAuthConfig struct {
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"-"` // Never expose
	AccessToken  string    `json:"-"` // Never expose
	RefreshToken string    `json:"-"` // Never expose
	TokenExpiry  time.Time `json:"token_expiry,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`

	// OAuth provider metadata
	ProviderType string `json:"provider_type,omitempty"` // "antigravity", "gemini-cli", "copilot"
	ProjectID    string `json:"project_id,omitempty"`    // Google Cloud Code project ID
	Email        string `json:"email,omitempty"`         // Authenticated user email
	Endpoint     string `json:"endpoint,omitempty"`      // API endpoint URL (e.g., cloudcode-pa.googleapis.com)
	Connected    bool   `json:"connected"`               // Whether OAuth is currently connected (legacy: true if any account connected)

	// Multi-account: number of connected OAuth accounts (populated at runtime from oauth_tokens table)
	AccountCount int `json:"account_count,omitempty"`
}

// OAuthAccount represents a single connected OAuth account (returned in API responses).
type OAuthAccount struct {
	ID           string    `json:"id"`
	ProviderType string    `json:"provider_type"`
	Email        string    `json:"email,omitempty"`
	ProjectID    string    `json:"project_id,omitempty"`
	Endpoint     string    `json:"endpoint,omitempty"`
	TokenExpiry  time.Time `json:"token_expiry,omitempty"`
	Connected    bool      `json:"connected"`
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
	DetectedMaxTokens *int   `json:"detected_max_tokens,omitempty"` // Actual max tokens supported by server
	DetectedAt        *int64 `json:"detected_at,omitempty"`         // Unix timestamp of last detection
}

// Model represents an available model from a provider
type Model struct {
	ID          string `json:"id"`
	ProviderID  string `json:"provider_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Enabled     bool   `json:"enabled"`

	// Capabilities
	Capabilities ModelCapabilities `json:"capabilities"`

	// Pricing (per 1M tokens, in USD)
	InputPrice  float64 `json:"input_price,omitempty"`
	OutputPrice float64 `json:"output_price,omitempty"`
	CachePrice  float64 `json:"cache_price,omitempty"` // Cache read price

	// Per-request pricing (USD) — for media generation models (image/video/audio)
	PricePerRequest float64 `json:"price_per_request,omitempty"`

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
	Chat            bool `json:"chat"`
	Completion      bool `json:"completion"`
	Vision          bool `json:"vision"`
	FunctionCall    bool `json:"function_call"`
	Streaming       bool `json:"streaming"`
	Thinking        bool `json:"thinking"`         // Extended thinking mode (Claude)
	JSON            bool `json:"json"`             // JSON mode support
	SystemPrompt    bool `json:"system_prompt"`    // System prompt support
	ImageGeneration bool `json:"image_generation"` // Image generation (text-to-image)
	VideoGeneration bool `json:"video_generation"` // Video generation (text-to-video)
	AudioGeneration bool `json:"audio_generation"` // Audio generation (TTS, music)
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
	ModelID             string             `json:"model_id"`
	Strategy            RoutingStrategy    `json:"strategy,omitempty"`
	Mode                RoutingMode        `json:"mode,omitempty"`                  // Location preference: auto, cloud, local
	Exclude             []string           `json:"exclude,omitempty"`               // Provider IDs to exclude
	RequireCap          *ModelCapabilities `json:"require_cap,omitempty"`           // Required capabilities
	PreferredProviderID string             `json:"preferred_provider_id,omitempty"` // Sticky routing: try this provider first (tool rounds)
}

// RouteResult represents the routing decision
type RouteResult struct {
	Provider  *Provider         `json:"provider"`
	Model     *Model            `json:"model"`
	APIKey    *APIKey           `json:"api_key,omitempty"`
	OAuth     *OAuthConfig      `json:"oauth,omitempty"`
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
	KeyID      string        `json:"key_id,omitempty"`
	KeyHash    string        `json:"key_hash,omitempty"`
	Healthy    bool          `json:"healthy"`
	Latency    time.Duration `json:"latency"`
	Error      string        `json:"error,omitempty"`
	CheckedAt  time.Time     `json:"checked_at"`
}

// IDEProvider represents a provider discovered from a local IDE
type IDEProvider struct {
	IDEName      string    `json:"ide_name"` // e.g., "antigravity", "cursor"
	IDEVersion   string    `json:"ide_version,omitempty"`
	ProxyURL     string    `json:"proxy_url"`   // Local proxy endpoint
	ConfigPath   string    `json:"config_path"` // Path to IDE config
	Connected    bool      `json:"connected"`
	Models       []string  `json:"models,omitempty"`
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
	ModelID     string    `json:"model_id"`              // Model identifier (can be pattern like "gpt-*")
	ProviderID  string    `json:"provider_id,omitempty"` // Optional: specific provider
	InputPrice  float64   `json:"input_price"`           // Price per 1M input tokens (USD)
	OutputPrice float64   `json:"output_price"`          // Price per 1M output tokens (USD)
	CachePrice  float64   `json:"cache_price,omitempty"` // Price per 1M cache read tokens (USD)
	IsCustom    bool      `json:"is_custom"`             // True if user-defined, false if default
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
		UpdatedAt:          timeutil.NowTime(),
	}
}

// FailoverReason represents why a failover occurred
type FailoverReason string

const (
	// FailoverReasonAPIError indicates the provider returned an API error
	FailoverReasonAPIError FailoverReason = "api_error"
	// FailoverReasonTimeout indicates the request timed out
	FailoverReasonTimeout FailoverReason = "timeout"
	// FailoverReasonRateLimit indicates rate limiting was hit
	FailoverReasonRateLimit FailoverReason = "rate_limit"
	// FailoverReasonAuthError indicates authentication failed
	FailoverReasonAuthError FailoverReason = "auth_error"
	// FailoverReasonModelNotFound indicates the model was not available
	FailoverReasonModelNotFound FailoverReason = "model_not_found"
	// FailoverReasonCooldown indicates the provider is in cooldown
	FailoverReasonCooldown FailoverReason = "cooldown"
	// FailoverReasonUnknown indicates an unknown error
	FailoverReasonUnknown FailoverReason = "unknown"
)

// FailoverRecord tracks a single failover event
type FailoverRecord struct {
	Timestamp      time.Time      `json:"timestamp"`
	ProviderID     string         `json:"provider_id"`
	ProviderName   string         `json:"provider_name"`
	ModelID        string         `json:"model_id"`
	Reason         FailoverReason `json:"reason"`
	Error          string         `json:"error"`
	Latency        time.Duration  `json:"latency"`
	NextProviderID string         `json:"next_provider_id,omitempty"`
}

// FailoverResult contains the complete failover tracking for a request
type FailoverResult struct {
	RequestID       string            `json:"request_id"`
	StartTime       time.Time         `json:"start_time"`
	EndTime         time.Time         `json:"end_time"`
	TotalAttempts   int               `json:"total_attempts"`
	SuccessProvider string            `json:"success_provider,omitempty"`
	SuccessModel    string            `json:"success_model,omitempty"`
	FailedAttempts  []*FailoverRecord `json:"failed_attempts,omitempty"`
	FinalError      string            `json:"final_error,omitempty"`
}

// Summary returns a compact trace of provider attempts for logging.
func (r *FailoverResult) Summary() string {
	if r == nil {
		return ""
	}
	repeatedProviders := make(map[string]int)
	for _, attempt := range r.FailedAttempts {
		if attempt == nil {
			continue
		}
		providerID := strings.TrimSpace(attempt.ProviderID)
		if providerID == "" {
			providerID = strings.TrimSpace(attempt.ProviderName)
		}
		if providerID == "" {
			continue
		}
		repeatedProviders[providerID]++
	}
	if successProvider := strings.TrimSpace(r.SuccessProvider); successProvider != "" {
		repeatedProviders[successProvider]++
	}

	parts := make([]string, 0, len(r.FailedAttempts)+1)
	for i, attempt := range r.FailedAttempts {
		if attempt == nil {
			continue
		}
		providerID := strings.TrimSpace(attempt.ProviderID)
		if providerID == "" {
			providerID = strings.TrimSpace(attempt.ProviderName)
		}
		if providerID == "" {
			providerID = "unknown"
		}

		segmentProvider := providerID
		if repeatedProviders[providerID] > 1 {
			if modelID := strings.TrimSpace(attempt.ModelID); modelID != "" {
				segmentProvider += ":" + modelID
			}
		}

		segment := fmt.Sprintf("%s[%s,%dms]", segmentProvider, attempt.Reason, attempt.Latency.Milliseconds())
		if nextProviderID := strings.TrimSpace(attempt.NextProviderID); nextProviderID != "" {
			nextProviderLabel := nextProviderID
			if repeatedProviders[nextProviderID] > 1 {
				nextModelID := ""
				for j := i + 1; j < len(r.FailedAttempts); j++ {
					nextAttempt := r.FailedAttempts[j]
					if nextAttempt == nil {
						continue
					}
					candidateProviderID := strings.TrimSpace(nextAttempt.ProviderID)
					if candidateProviderID == "" {
						candidateProviderID = strings.TrimSpace(nextAttempt.ProviderName)
					}
					if candidateProviderID == nextProviderID {
						nextModelID = strings.TrimSpace(nextAttempt.ModelID)
						break
					}
				}
				if nextModelID == "" && strings.TrimSpace(r.SuccessProvider) == nextProviderID {
					nextModelID = strings.TrimSpace(r.SuccessModel)
				}
				if nextModelID != "" {
					nextProviderLabel += ":" + nextModelID
				}
			}
			segment += "->" + nextProviderLabel
		}
		parts = append(parts, segment)
	}
	if successProvider := strings.TrimSpace(r.SuccessProvider); successProvider != "" {
		successSegment := successProvider + "[success"
		if successModel := strings.TrimSpace(r.SuccessModel); successModel != "" {
			successSegment += ":" + successModel
		}
		successSegment += "]"
		parts = append(parts, successSegment)
	}
	return strings.Join(parts, " | ")
}

// CooldownEntry tracks cooldown state for a provider
type CooldownEntry struct {
	ProviderID            string    `json:"provider_id"`
	CooldownUntil         time.Time `json:"cooldown_until"`
	FailureCount          int       `json:"failure_count"`
	TransientFailureCount int       `json:"transient_failure_count"` // 502/503 failures tracked separately
	LastFailure           time.Time `json:"last_failure"`
	LastError             string    `json:"last_error"`
}

// CooldownConfig configures the cooldown behavior
type CooldownConfig struct {
	// FailureThreshold is the number of failures before cooldown
	FailureThreshold int `json:"failure_threshold"`
	// InitialCooldown is the initial cooldown duration
	InitialCooldown time.Duration `json:"initial_cooldown"`
	// MaxCooldown is the maximum cooldown duration
	MaxCooldown time.Duration `json:"max_cooldown"`
	// CooldownMultiplier increases cooldown on repeated failures
	CooldownMultiplier float64 `json:"cooldown_multiplier"`
	// ResetAfter resets failure count after this duration of success
	ResetAfter time.Duration `json:"reset_after"`

	// Transient error overrides (502/503) — shorter cooldown for temporary upstream issues
	TransientFailureThreshold int           `json:"transient_failure_threshold"` // 0 = use FailureThreshold
	TransientInitialCooldown  time.Duration `json:"transient_initial_cooldown"`  // 0 = use InitialCooldown
	TransientMaxCooldown      time.Duration `json:"transient_max_cooldown"`      // 0 = use MaxCooldown
}

// DefaultCooldownConfig returns sensible defaults for cooldown
func DefaultCooldownConfig() *CooldownConfig {
	return &CooldownConfig{
		FailureThreshold:   3,
		InitialCooldown:    30 * time.Second,
		MaxCooldown:        5 * time.Minute,
		CooldownMultiplier: 2.0,
		ResetAfter:         5 * time.Minute,

		// Transient 502/503: higher threshold, much shorter cooldown
		TransientFailureThreshold: 5,
		TransientInitialCooldown:  5 * time.Second,
		TransientMaxCooldown:      30 * time.Second,
	}
}
