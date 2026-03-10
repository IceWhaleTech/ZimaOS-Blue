package proxy

import (
	"log/slog"
	"sync/atomic"
)

// PromptCacheStats tracks prompt caching effectiveness.
// All fields are updated atomically and safe for concurrent use.
type PromptCacheStats struct {
	// Requests is the total number of requests processed.
	Requests atomic.Int64
	// CacheHits is the number of requests where cache_read_input_tokens > 0.
	CacheHits atomic.Int64
	// CacheMisses is the number of requests where cache_read_input_tokens == 0.
	CacheMisses atomic.Int64
	// TotalCacheReadTokens is the cumulative cache_read_input_tokens.
	TotalCacheReadTokens atomic.Int64
	// TotalCacheCreationTokens is the cumulative cache_creation_input_tokens.
	TotalCacheCreationTokens atomic.Int64
	// TotalInputTokens is the cumulative input tokens (for computing reuse ratio).
	TotalInputTokens atomic.Int64
}

// Record records a single request's cache usage.
func (s *PromptCacheStats) Record(inputTokens, cacheReadTokens, cacheCreationTokens int) {
	s.Requests.Add(1)
	s.TotalInputTokens.Add(int64(inputTokens))
	s.TotalCacheReadTokens.Add(int64(cacheReadTokens))
	s.TotalCacheCreationTokens.Add(int64(cacheCreationTokens))
	if cacheReadTokens > 0 {
		s.CacheHits.Add(1)
	} else {
		s.CacheMisses.Add(1)
	}
}

// HitRate returns the cache hit rate as a float64 [0, 1].
func (s *PromptCacheStats) HitRate() float64 {
	total := s.Requests.Load()
	if total == 0 {
		return 0
	}
	return float64(s.CacheHits.Load()) / float64(total)
}

// ReuseRatio returns the fraction of input tokens served from cache.
func (s *PromptCacheStats) ReuseRatio() float64 {
	total := s.TotalInputTokens.Load()
	if total == 0 {
		return 0
	}
	return float64(s.TotalCacheReadTokens.Load()) / float64(total)
}

// Snapshot returns a copy of the current stats for reporting.
func (s *PromptCacheStats) Snapshot() PromptCacheSnapshot {
	return PromptCacheSnapshot{
		Requests:             s.Requests.Load(),
		CacheHits:            s.CacheHits.Load(),
		CacheMisses:          s.CacheMisses.Load(),
		TotalCacheReadTokens: s.TotalCacheReadTokens.Load(),
		TotalCacheCreation:   s.TotalCacheCreationTokens.Load(),
		TotalInputTokens:     s.TotalInputTokens.Load(),
		HitRate:              s.HitRate(),
		ReuseRatio:           s.ReuseRatio(),
	}
}

// PromptCacheSnapshot is a JSON-serializable snapshot of cache stats.
type PromptCacheSnapshot struct {
	Requests             int64   `json:"requests"`
	CacheHits            int64   `json:"cache_hits"`
	CacheMisses          int64   `json:"cache_misses"`
	TotalCacheReadTokens int64   `json:"total_cache_read_tokens"`
	TotalCacheCreation   int64   `json:"total_cache_creation_tokens"`
	TotalInputTokens     int64   `json:"total_input_tokens"`
	HitRate              float64 `json:"hit_rate"`
	ReuseRatio           float64 `json:"reuse_ratio"`
}

func promptCacheEvent(cacheRead, cacheCreation int) string {
	switch {
	case cacheRead > 0 && cacheCreation > 0:
		return "hit+create"
	case cacheRead > 0:
		return "hit"
	case cacheCreation > 0:
		return "create"
	default:
		return "miss"
	}
}

// LogTokenChurn logs a per-request summary of token reuse vs churn.
func LogTokenChurn(provider, model, promptCacheKey string, inputTokens, cacheRead, cacheCreation int) {
	if inputTokens == 0 {
		return
	}
	reusePct := 0.0
	if inputTokens > 0 {
		reusePct = float64(cacheRead) / float64(inputTokens) * 100
	}
	attrs := []any{
		"provider", provider,
		"model", model,
		"cache_event", promptCacheEvent(cacheRead, cacheCreation),
		"input_tokens", inputTokens,
		"cache_read", cacheRead,
		"cache_creation", cacheCreation,
		"reuse_pct", int(reusePct),
	}
	if promptCacheKey != "" {
		attrs = append(attrs, "prompt_cache_key", promptCacheKey)
	}
	slog.Info("[prompt-cache] token churn", attrs...)
}
