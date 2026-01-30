package proxy

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// QuotaInfo represents quota information for a provider/account
type QuotaInfo struct {
	Provider     string    `json:"provider"`
	AccountID    string    `json:"account_id,omitempty"`
	TotalQuota   int64     `json:"total_quota"`    // Total allowed requests/tokens
	UsedQuota    int64     `json:"used_quota"`     // Used requests/tokens
	RemainingPct float64   `json:"remaining_pct"`  // Remaining percentage (0-100)
	ResetTime    time.Time `json:"reset_time"`     // When quota resets
	LastSync     time.Time `json:"last_sync"`      // Last sync time
	Status       string    `json:"status"`         // "healthy", "warning", "critical", "banned", "rate_limited"
	RateLimited  bool      `json:"rate_limited"`   // Currently rate limited (429)
	BanDetected  bool      `json:"ban_detected"`   // 403 ban detected
}

// QuotaMonitorConfig configuration for quota monitoring
type QuotaMonitorConfig struct {
	Enabled           bool          `json:"enabled" yaml:"enabled"`
	SyncInterval      time.Duration `json:"sync_interval" yaml:"sync_interval"`           // How often to sync quotas
	WarningThreshold  float64       `json:"warning_threshold" yaml:"warning_threshold"`   // Warn when below this % (default: 20)
	CriticalThreshold float64       `json:"critical_threshold" yaml:"critical_threshold"` // Critical when below this % (default: 5)
	TrackTokens       bool          `json:"track_tokens" yaml:"track_tokens"`             // Track token usage
	TrackRequests     bool          `json:"track_requests" yaml:"track_requests"`         // Track request count
}

// DefaultQuotaMonitorConfig returns default configuration
func DefaultQuotaMonitorConfig() *QuotaMonitorConfig {
	return &QuotaMonitorConfig{
		Enabled:           true,
		SyncInterval:      5 * time.Minute,
		WarningThreshold:  20.0,
		CriticalThreshold: 5.0,
		TrackTokens:       true,
		TrackRequests:     true,
	}
}

// QuotaSummary provides dashboard-ready quota information
type QuotaSummary struct {
	Providers        []*QuotaInfo `json:"providers"`
	AverageRemaining float64      `json:"average_remaining_pct"`
	TotalProviders   int          `json:"total_providers"`
	HealthyCount     int          `json:"healthy_count"`
	WarningCount     int          `json:"warning_count"`
	CriticalCount    int          `json:"critical_count"`
	BannedCount      int          `json:"banned_count"`
	BestProvider     string       `json:"best_provider"`
}

// QuotaMonitor tracks and manages quota information
type QuotaMonitor struct {
	config *QuotaMonitorConfig
	quotas map[string]*QuotaInfo // key: provider or provider:account_id
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewQuotaMonitor creates a new quota monitor
func NewQuotaMonitor(config *QuotaMonitorConfig) *QuotaMonitor {
	if config == nil {
		config = DefaultQuotaMonitorConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &QuotaMonitor{
		config: config,
		quotas: make(map[string]*QuotaInfo),
		ctx:    ctx,
		cancel: cancel,
	}
}

// RecordUsage records usage from a response
func (qm *QuotaMonitor) RecordUsage(provider string, resp *http.Response, tokensUsed int64) {
	if !qm.config.Enabled {
		return
	}

	qm.mu.Lock()
	defer qm.mu.Unlock()

	key := provider
	quota, exists := qm.quotas[key]
	if !exists {
		quota = &QuotaInfo{
			Provider:     provider,
			Status:       "healthy",
			RemainingPct: 100.0,
		}
		qm.quotas[key] = quota
	}

	// Update usage
	if qm.config.TrackTokens && tokensUsed > 0 {
		quota.UsedQuota += tokensUsed
	}
	quota.LastSync = time.Now()

	// Parse rate limit headers if present
	qm.parseRateLimitHeaders(quota, resp)

	// Update status based on remaining quota
	qm.updateStatus(quota)
}

// RecordRequest records a request (without response details)
func (qm *QuotaMonitor) RecordRequest(provider string) {
	if !qm.config.Enabled || !qm.config.TrackRequests {
		return
	}

	qm.mu.Lock()
	defer qm.mu.Unlock()

	key := provider
	quota, exists := qm.quotas[key]
	if !exists {
		quota = &QuotaInfo{
			Provider:     provider,
			Status:       "healthy",
			RemainingPct: 100.0,
		}
		qm.quotas[key] = quota
	}

	quota.UsedQuota++
	quota.LastSync = time.Now()
}

// RecordError records an error response
func (qm *QuotaMonitor) RecordError(provider string, statusCode int) {
	if !qm.config.Enabled {
		return
	}

	qm.mu.Lock()
	defer qm.mu.Unlock()

	key := provider
	quota, exists := qm.quotas[key]
	if !exists {
		quota = &QuotaInfo{
			Provider:     provider,
			Status:       "healthy",
			RemainingPct: 100.0,
		}
		qm.quotas[key] = quota
	}

	quota.LastSync = time.Now()

	// Detect rate limiting (429) or ban (403)
	if statusCode == 429 {
		quota.RateLimited = true
		quota.Status = "rate_limited"
	} else if statusCode == 403 {
		quota.BanDetected = true
		quota.Status = "banned"
	}
}

// ClearRateLimit clears rate limit status for a provider
func (qm *QuotaMonitor) ClearRateLimit(provider string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if quota, exists := qm.quotas[provider]; exists {
		quota.RateLimited = false
		qm.updateStatus(quota)
	}
}

// ClearBan clears ban status for a provider
func (qm *QuotaMonitor) ClearBan(provider string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if quota, exists := qm.quotas[provider]; exists {
		quota.BanDetected = false
		qm.updateStatus(quota)
	}
}

// parseRateLimitHeaders extracts quota info from response headers
func (qm *QuotaMonitor) parseRateLimitHeaders(quota *QuotaInfo, resp *http.Response) {
	if resp == nil {
		return
	}

	// Common rate limit headers
	// X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
	// anthropic-ratelimit-requests-limit, anthropic-ratelimit-requests-remaining
	// x-ratelimit-limit-requests, x-ratelimit-remaining-requests (OpenAI)

	// Try standard headers first
	if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "" {
		if v, err := strconv.ParseInt(limit, 10, 64); err == nil {
			quota.TotalQuota = v
		}
	}

	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if v, err := strconv.ParseInt(remaining, 10, 64); err == nil {
			if quota.TotalQuota > 0 {
				quota.UsedQuota = quota.TotalQuota - v
				quota.RemainingPct = float64(v) / float64(quota.TotalQuota) * 100
			}
		}
	}

	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if v, err := strconv.ParseInt(reset, 10, 64); err == nil {
			quota.ResetTime = time.Unix(v, 0)
		}
	}

	// Try Anthropic-specific headers
	if limit := resp.Header.Get("anthropic-ratelimit-requests-limit"); limit != "" {
		if v, err := strconv.ParseInt(limit, 10, 64); err == nil {
			quota.TotalQuota = v
		}
	}

	if remaining := resp.Header.Get("anthropic-ratelimit-requests-remaining"); remaining != "" {
		if v, err := strconv.ParseInt(remaining, 10, 64); err == nil {
			if quota.TotalQuota > 0 {
				quota.UsedQuota = quota.TotalQuota - v
				quota.RemainingPct = float64(v) / float64(quota.TotalQuota) * 100
			}
		}
	}

	// Try OpenAI-specific headers
	if limit := resp.Header.Get("x-ratelimit-limit-requests"); limit != "" {
		if v, err := strconv.ParseInt(limit, 10, 64); err == nil {
			quota.TotalQuota = v
		}
	}

	if remaining := resp.Header.Get("x-ratelimit-remaining-requests"); remaining != "" {
		if v, err := strconv.ParseInt(remaining, 10, 64); err == nil {
			if quota.TotalQuota > 0 {
				quota.UsedQuota = quota.TotalQuota - v
				quota.RemainingPct = float64(v) / float64(quota.TotalQuota) * 100
			}
		}
	}

	// Detect rate limiting (429) or ban (403)
	quota.RateLimited = resp.StatusCode == 429
	quota.BanDetected = resp.StatusCode == 403
}

// updateStatus updates quota status based on remaining percentage
func (qm *QuotaMonitor) updateStatus(quota *QuotaInfo) {
	if quota.BanDetected {
		quota.Status = "banned"
	} else if quota.RateLimited {
		quota.Status = "rate_limited"
	} else if quota.RemainingPct <= qm.config.CriticalThreshold {
		quota.Status = "critical"
	} else if quota.RemainingPct <= qm.config.WarningThreshold {
		quota.Status = "warning"
	} else {
		quota.Status = "healthy"
	}
}

// GetQuotaSummary returns a summary of all quotas for dashboard display
func (qm *QuotaMonitor) GetQuotaSummary() *QuotaSummary {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	summary := &QuotaSummary{
		Providers:        make([]*QuotaInfo, 0, len(qm.quotas)),
		AverageRemaining: 0,
		TotalProviders:   len(qm.quotas),
		HealthyCount:     0,
		WarningCount:     0,
		CriticalCount:    0,
		BannedCount:      0,
	}

	var totalPct float64
	for _, quota := range qm.quotas {
		// Create a copy to avoid race conditions
		quotaCopy := &QuotaInfo{
			Provider:     quota.Provider,
			AccountID:    quota.AccountID,
			TotalQuota:   quota.TotalQuota,
			UsedQuota:    quota.UsedQuota,
			RemainingPct: quota.RemainingPct,
			ResetTime:    quota.ResetTime,
			LastSync:     quota.LastSync,
			Status:       quota.Status,
			RateLimited:  quota.RateLimited,
			BanDetected:  quota.BanDetected,
		}
		summary.Providers = append(summary.Providers, quotaCopy)
		totalPct += quota.RemainingPct

		switch quota.Status {
		case "healthy":
			summary.HealthyCount++
		case "warning":
			summary.WarningCount++
		case "critical", "rate_limited":
			summary.CriticalCount++
		case "banned":
			summary.BannedCount++
		}
	}

	if len(qm.quotas) > 0 {
		summary.AverageRemaining = totalPct / float64(len(qm.quotas))
	}

	// Find best provider (highest remaining quota, not banned)
	summary.BestProvider = qm.findBestProvider()

	return summary
}

// GetQuota returns quota info for a specific provider
func (qm *QuotaMonitor) GetQuota(provider string) *QuotaInfo {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	if quota, exists := qm.quotas[provider]; exists {
		// Return a copy
		return &QuotaInfo{
			Provider:     quota.Provider,
			AccountID:    quota.AccountID,
			TotalQuota:   quota.TotalQuota,
			UsedQuota:    quota.UsedQuota,
			RemainingPct: quota.RemainingPct,
			ResetTime:    quota.ResetTime,
			LastSync:     quota.LastSync,
			Status:       quota.Status,
			RateLimited:  quota.RateLimited,
			BanDetected:  quota.BanDetected,
		}
	}
	return nil
}

// GetBestProvider returns the provider with highest remaining quota
func (qm *QuotaMonitor) GetBestProvider() string {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	return qm.findBestProvider()
}

// findBestProvider returns the provider with highest remaining quota (internal, no lock)
func (qm *QuotaMonitor) findBestProvider() string {
	var best string
	var bestPct float64 = -1

	for _, quota := range qm.quotas {
		if quota.Status != "banned" && quota.Status != "rate_limited" && quota.RemainingPct > bestPct {
			bestPct = quota.RemainingPct
			best = quota.Provider
		}
	}

	return best
}

// IsProviderAvailable checks if a provider is available (not banned or rate limited)
func (qm *QuotaMonitor) IsProviderAvailable(provider string) bool {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	quota, exists := qm.quotas[provider]
	if !exists {
		return true // Unknown provider is assumed available
	}

	return quota.Status != "banned" && quota.Status != "rate_limited"
}

// Reset resets all quota tracking
func (qm *QuotaMonitor) Reset() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.quotas = make(map[string]*QuotaInfo)
}

// ResetProvider resets quota tracking for a specific provider
func (qm *QuotaMonitor) ResetProvider(provider string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	delete(qm.quotas, provider)
}

// Stop stops the quota monitor
func (qm *QuotaMonitor) Stop() {
	qm.cancel()
}

// Stats returns quota monitor statistics
func (qm *QuotaMonitor) Stats() map[string]interface{} {
	summary := qm.GetQuotaSummary()

	return map[string]interface{}{
		"enabled":              qm.config.Enabled,
		"total_providers":      summary.TotalProviders,
		"average_remaining":    summary.AverageRemaining,
		"healthy_count":        summary.HealthyCount,
		"warning_count":        summary.WarningCount,
		"critical_count":       summary.CriticalCount,
		"banned_count":         summary.BannedCount,
		"best_provider":        summary.BestProvider,
		"warning_threshold":    qm.config.WarningThreshold,
		"critical_threshold":   qm.config.CriticalThreshold,
	}
}
