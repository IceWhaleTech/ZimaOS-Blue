package server

import (
	"sync"
	"sync/atomic"
	"time"
)

// UsageStats tracks usage statistics per provider and model
type UsageStats struct {
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	TotalRequests int64     `json:"total_requests"`
	SuccessCount  int64     `json:"success_count"`
	ErrorCount    int64     `json:"error_count"`
	TotalTokens   int64     `json:"total_tokens"`
	InputTokens   int64     `json:"input_tokens"`
	OutputTokens  int64     `json:"output_tokens"`
	TotalLatency  int64     `json:"total_latency_ms"`
	MinLatency    int64     `json:"min_latency_ms"`
	MaxLatency    int64     `json:"max_latency_ms"`
	LastUsed      time.Time `json:"last_used"`
	CreatedAt     time.Time `json:"created_at"`
}

// UsageTracker tracks usage statistics per provider-model combination
type UsageTracker struct {
	stats map[string]*UsageStats
	mu    sync.RWMutex
}

// NewUsageTracker creates a new usage tracker
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		stats: make(map[string]*UsageStats),
	}
}

// getKey generates a unique key for provider-model combination
func (ut *UsageTracker) getKey(provider, model string) string {
	return provider + ":" + model
}

// RecordRequest records a request for a provider-model combination
func (ut *UsageTracker) RecordRequest(provider, model string, success bool, latency int64, inputTokens, outputTokens int64) {
	key := ut.getKey(provider, model)

	ut.mu.Lock()
	defer ut.mu.Unlock()

	stats, exists := ut.stats[key]
	if !exists {
		stats = &UsageStats{
			Provider:  provider,
			Model:     model,
			CreatedAt: time.Now(),
			MinLatency: 1<<63 - 1,
		}
		ut.stats[key] = stats
	}

	// Update counters
	atomic.AddInt64(&stats.TotalRequests, 1)
	if success {
		atomic.AddInt64(&stats.SuccessCount, 1)
	} else {
		atomic.AddInt64(&stats.ErrorCount, 1)
	}

	// Update token counts
	totalTokens := inputTokens + outputTokens
	atomic.AddInt64(&stats.TotalTokens, totalTokens)
	atomic.AddInt64(&stats.InputTokens, inputTokens)
	atomic.AddInt64(&stats.OutputTokens, outputTokens)

	// Update latency
	atomic.AddInt64(&stats.TotalLatency, latency)

	// Update min/max latency
	for {
		currentMin := atomic.LoadInt64(&stats.MinLatency)
		if latency >= currentMin || atomic.CompareAndSwapInt64(&stats.MinLatency, currentMin, latency) {
			break
		}
	}

	for {
		currentMax := atomic.LoadInt64(&stats.MaxLatency)
		if latency <= currentMax || atomic.CompareAndSwapInt64(&stats.MaxLatency, currentMax, latency) {
			break
		}
	}

	stats.LastUsed = time.Now()
}

// GetStats returns usage statistics for a provider-model combination
func (ut *UsageTracker) GetStats(provider, model string) *UsageStats {
	key := ut.getKey(provider, model)

	ut.mu.RLock()
	defer ut.mu.RUnlock()

	if stats, exists := ut.stats[key]; exists {
		return stats
	}
	return nil
}

// GetAllStats returns all usage statistics
func (ut *UsageTracker) GetAllStats() []*UsageStats {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	result := make([]*UsageStats, 0, len(ut.stats))
	for _, stats := range ut.stats {
		result = append(result, stats)
	}
	return result
}

// GetProviderStats returns aggregated stats for a provider
func (ut *UsageTracker) GetProviderStats(provider string) map[string]interface{} {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	var totalRequests, successCount, errorCount, totalTokens int64
	var totalLatency int64
	modelCount := 0

	for key, stats := range ut.stats {
		if len(key) > len(provider) && key[:len(provider)] == provider {
			totalRequests += atomic.LoadInt64(&stats.TotalRequests)
			successCount += atomic.LoadInt64(&stats.SuccessCount)
			errorCount += atomic.LoadInt64(&stats.ErrorCount)
			totalTokens += atomic.LoadInt64(&stats.TotalTokens)
			totalLatency += atomic.LoadInt64(&stats.TotalLatency)
			modelCount++
		}
	}

	avgLatency := int64(0)
	if totalRequests > 0 {
		avgLatency = totalLatency / totalRequests
	}

	successRate := float64(0)
	if totalRequests > 0 {
		successRate = float64(successCount) / float64(totalRequests) * 100
	}

	return map[string]interface{}{
		"provider":        provider,
		"total_requests":  totalRequests,
		"success_count":   successCount,
		"error_count":     errorCount,
		"success_rate":    successRate,
		"total_tokens":    totalTokens,
		"avg_latency_ms":  avgLatency,
		"model_count":     modelCount,
	}
}

// Reset resets all statistics
func (ut *UsageTracker) Reset() {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.stats = make(map[string]*UsageStats)
}
