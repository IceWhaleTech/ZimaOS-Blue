package proxy

import (
	"sync"
	"sync/atomic"
	"time"
)

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled         bool          `json:"enabled"`
	RetentionPeriod time.Duration `json:"retention_period"`
	BucketSize      time.Duration `json:"bucket_size"`
}

// DefaultMetricsConfig returns default metrics configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Enabled:         true,
		RetentionPeriod: 24 * time.Hour,
		BucketSize:      1 * time.Minute,
	}
}

// RequestMetrics holds metrics for a single request
type RequestMetrics struct {
	Timestamp       time.Time     `json:"timestamp"`
	Provider        string        `json:"provider"`
	Model           string        `json:"model"`
	StatusCode      int           `json:"status_code"`
	Latency         time.Duration `json:"latency"`          // Total response time
	TTFT            time.Duration `json:"ttft"`             // Time to First Token
	TokensIn        int64         `json:"tokens_in"`
	TokensOut       int64         `json:"tokens_out"`
	RequestSize     int64         `json:"request_size"`
	ResponseSize    int64         `json:"response_size"`
	Success         bool          `json:"success"`
	Cached          bool          `json:"cached"`
	Streaming       bool          `json:"streaming"`
	ProxyOverhead   time.Duration `json:"proxy_overhead"`   // Time spent in proxy processing
}

// MetricsBucket holds aggregated metrics for a time period
type MetricsBucket struct {
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	RequestCount   int64     `json:"request_count"`
	SuccessCount   int64     `json:"success_count"`
	ErrorCount     int64     `json:"error_count"`
	TotalLatency   int64     `json:"total_latency_ms"`
	TotalTTFT      int64     `json:"total_ttft_ms"`
	TotalTokensIn  int64     `json:"total_tokens_in"`
	TotalTokensOut int64     `json:"total_tokens_out"`
	TotalBytes     int64     `json:"total_bytes"`
}

// ProviderMetrics holds metrics for a specific provider
type ProviderMetrics struct {
	Name           string  `json:"name"`
	RequestCount   int64   `json:"request_count"`
	SuccessCount   int64   `json:"success_count"`
	ErrorCount     int64   `json:"error_count"`
	TotalLatency   int64   `json:"total_latency_ms"`
	AvgLatency     float64 `json:"avg_latency_ms"`
	TotalTTFT      int64   `json:"total_ttft_ms"`
	AvgTTFT        float64 `json:"avg_ttft_ms"`
	TotalTokensIn  int64   `json:"total_tokens_in"`
	TotalTokensOut int64   `json:"total_tokens_out"`
	TokensPerSec   float64 `json:"tokens_per_sec"` // Output tokens per second
}

// MetricsCollector collects and aggregates usage metrics
type MetricsCollector struct {
	config *MetricsConfig
	mu     sync.RWMutex

	// Global counters
	totalRequests  int64
	totalSuccess   int64
	totalErrors    int64
	totalTokensIn  int64
	totalTokensOut int64
	totalBytes     int64

	// Latency tracking for percentiles
	latencies      []int64 // Store latencies in ms for percentile calculation
	ttfts          []int64 // Store TTFT in ms
	proxyOverheads []int64 // Store proxy overhead in ms
	maxLatencies   int     // Max number of latencies to keep

	// Per-provider metrics
	providerMetrics map[string]*ProviderMetrics

	// Time-series buckets
	buckets []*MetricsBucket

	// Recent requests for detailed analysis
	recentRequests []RequestMetrics
	maxRecent      int
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(config *MetricsConfig) *MetricsCollector {
	if config == nil {
		config = DefaultMetricsConfig()
	}

	return &MetricsCollector{
		config:          config,
		providerMetrics: make(map[string]*ProviderMetrics),
		buckets:         make([]*MetricsBucket, 0),
		recentRequests:  make([]RequestMetrics, 0),
		latencies:       make([]int64, 0),
		ttfts:           make([]int64, 0),
		proxyOverheads:  make([]int64, 0),
		maxRecent:       1000,
		maxLatencies:    10000,
	}
}

// Record records a request metric
func (mc *MetricsCollector) Record(m RequestMetrics) {
	if !mc.config.Enabled {
		return
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Update global counters
	atomic.AddInt64(&mc.totalRequests, 1)
	atomic.AddInt64(&mc.totalTokensIn, m.TokensIn)
	atomic.AddInt64(&mc.totalTokensOut, m.TokensOut)
	atomic.AddInt64(&mc.totalBytes, m.RequestSize+m.ResponseSize)

	if m.Success {
		atomic.AddInt64(&mc.totalSuccess, 1)
	} else {
		atomic.AddInt64(&mc.totalErrors, 1)
	}

	// Track latencies for percentile calculation
	mc.latencies = append(mc.latencies, m.Latency.Milliseconds())
	if len(mc.latencies) > mc.maxLatencies {
		mc.latencies = mc.latencies[1:]
	}

	// Track TTFT
	if m.TTFT > 0 {
		mc.ttfts = append(mc.ttfts, m.TTFT.Milliseconds())
		if len(mc.ttfts) > mc.maxLatencies {
			mc.ttfts = mc.ttfts[1:]
		}
	}

	// Track proxy overhead
	if m.ProxyOverhead > 0 {
		mc.proxyOverheads = append(mc.proxyOverheads, m.ProxyOverhead.Milliseconds())
		if len(mc.proxyOverheads) > mc.maxLatencies {
			mc.proxyOverheads = mc.proxyOverheads[1:]
		}
	}

	// Update provider metrics
	pm, ok := mc.providerMetrics[m.Provider]
	if !ok {
		pm = &ProviderMetrics{Name: m.Provider}
		mc.providerMetrics[m.Provider] = pm
	}

	pm.RequestCount++
	pm.TotalLatency += m.Latency.Milliseconds()
	pm.TotalTokensIn += m.TokensIn
	pm.TotalTokensOut += m.TokensOut
	if m.TTFT > 0 {
		pm.TotalTTFT += m.TTFT.Milliseconds()
	}
	if m.Success {
		pm.SuccessCount++
	} else {
		pm.ErrorCount++
	}
	if pm.RequestCount > 0 {
		pm.AvgLatency = float64(pm.TotalLatency) / float64(pm.RequestCount)
		pm.AvgTTFT = float64(pm.TotalTTFT) / float64(pm.RequestCount)
		// Calculate tokens per second (output tokens / total latency in seconds)
		if pm.TotalLatency > 0 {
			pm.TokensPerSec = float64(pm.TotalTokensOut) / (float64(pm.TotalLatency) / 1000.0)
		}
	}

	// Update time-series bucket
	mc.updateBucket(m)

	// Store recent request
	mc.recentRequests = append(mc.recentRequests, m)
	if len(mc.recentRequests) > mc.maxRecent {
		mc.recentRequests = mc.recentRequests[1:]
	}
}

// updateBucket updates or creates a time-series bucket
func (mc *MetricsCollector) updateBucket(m RequestMetrics) {
	bucketStart := m.Timestamp.Truncate(mc.config.BucketSize)
	bucketEnd := bucketStart.Add(mc.config.BucketSize)

	// Find or create bucket
	var bucket *MetricsBucket
	for _, b := range mc.buckets {
		if b.StartTime.Equal(bucketStart) {
			bucket = b
			break
		}
	}

	if bucket == nil {
		bucket = &MetricsBucket{
			StartTime: bucketStart,
			EndTime:   bucketEnd,
		}
		mc.buckets = append(mc.buckets, bucket)
	}

	// Update bucket
	bucket.RequestCount++
	bucket.TotalLatency += m.Latency.Milliseconds()
	bucket.TotalTokensIn += m.TokensIn
	bucket.TotalTokensOut += m.TokensOut
	bucket.TotalBytes += m.RequestSize + m.ResponseSize
	if m.TTFT > 0 {
		bucket.TotalTTFT += m.TTFT.Milliseconds()
	}

	if m.Success {
		bucket.SuccessCount++
	} else {
		bucket.ErrorCount++
	}

	// Cleanup old buckets
	mc.cleanupBuckets()
}

// cleanupBuckets removes buckets older than retention period
func (mc *MetricsCollector) cleanupBuckets() {
	cutoff := time.Now().Add(-mc.config.RetentionPeriod)
	newBuckets := make([]*MetricsBucket, 0)

	for _, b := range mc.buckets {
		if b.StartTime.After(cutoff) {
			newBuckets = append(newBuckets, b)
		}
	}

	mc.buckets = newBuckets
}

// GetProviderMetrics returns metrics for a specific provider
func (mc *MetricsCollector) GetProviderMetrics(name string) (*ProviderMetrics, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	pm, ok := mc.providerMetrics[name]
	if !ok {
		return nil, false
	}

	// Return a copy
	copy := *pm
	return &copy, true
}

// GetAllProviderMetrics returns metrics for all providers
func (mc *MetricsCollector) GetAllProviderMetrics() map[string]*ProviderMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*ProviderMetrics)
	for k, v := range mc.providerMetrics {
		copy := *v
		result[k] = &copy
	}
	return result
}

// GetTimeSeries returns time-series data for a time range
func (mc *MetricsCollector) GetTimeSeries(start, end time.Time) []*MetricsBucket {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make([]*MetricsBucket, 0)
	for _, b := range mc.buckets {
		if (b.StartTime.Equal(start) || b.StartTime.After(start)) &&
			b.StartTime.Before(end) {
			copy := *b
			result = append(result, &copy)
		}
	}
	return result
}

// GetRecentRequests returns recent request metrics
func (mc *MetricsCollector) GetRecentRequests(limit int) []RequestMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if limit <= 0 || limit > len(mc.recentRequests) {
		limit = len(mc.recentRequests)
	}

	start := len(mc.recentRequests) - limit
	result := make([]RequestMetrics, limit)
	copy(result, mc.recentRequests[start:])
	return result
}

// Summary returns a summary of all metrics
func (mc *MetricsCollector) Summary() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	avgLatency := float64(0)
	avgTTFT := float64(0)
	avgProxyOverhead := float64(0)

	if mc.totalRequests > 0 {
		totalLatency := int64(0)
		totalTTFT := int64(0)
		for _, pm := range mc.providerMetrics {
			totalLatency += pm.TotalLatency
			totalTTFT += pm.TotalTTFT
		}
		avgLatency = float64(totalLatency) / float64(mc.totalRequests)
		if totalTTFT > 0 {
			avgTTFT = float64(totalTTFT) / float64(mc.totalRequests)
		}
	}

	// Calculate proxy overhead average
	if len(mc.proxyOverheads) > 0 {
		total := int64(0)
		for _, v := range mc.proxyOverheads {
			total += v
		}
		avgProxyOverhead = float64(total) / float64(len(mc.proxyOverheads))
	}

	successRate := float64(0)
	if mc.totalRequests > 0 {
		successRate = float64(mc.totalSuccess) / float64(mc.totalRequests) * 100
	}

	// Calculate tokens per second (throughput)
	tokensPerSec := float64(0)
	if avgLatency > 0 && mc.totalRequests > 0 {
		totalLatencySeconds := (avgLatency * float64(mc.totalRequests)) / 1000.0
		if totalLatencySeconds > 0 {
			tokensPerSec = float64(mc.totalTokensOut) / totalLatencySeconds
		}
	}

	return map[string]interface{}{
		"enabled":            mc.config.Enabled,
		"total_requests":     mc.totalRequests,
		"total_success":      mc.totalSuccess,
		"total_errors":       mc.totalErrors,
		"success_rate":       successRate,
		"avg_latency_ms":     avgLatency,
		"avg_ttft_ms":        avgTTFT,
		"avg_proxy_overhead": avgProxyOverhead,
		"p95_latency_ms":     mc.calculatePercentile(mc.latencies, 95),
		"p99_latency_ms":     mc.calculatePercentile(mc.latencies, 99),
		"p95_ttft_ms":        mc.calculatePercentile(mc.ttfts, 95),
		"p99_ttft_ms":        mc.calculatePercentile(mc.ttfts, 99),
		"tokens_per_sec":     tokensPerSec,
		"total_tokens_in":    mc.totalTokensIn,
		"total_tokens_out":   mc.totalTokensOut,
		"total_bytes":        mc.totalBytes,
		"provider_count":     len(mc.providerMetrics),
		"bucket_count":       len(mc.buckets),
	}
}

// calculatePercentile calculates the percentile value from a slice of values
func (mc *MetricsCollector) calculatePercentile(values []int64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Make a copy and sort
	sorted := make([]int64, len(values))
	copy(sorted, values)
	mc.sortInt64(sorted)

	// Calculate index
	index := int(float64(len(sorted)-1) * percentile / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return float64(sorted[index])
}

// sortInt64 sorts a slice of int64 in ascending order
func (mc *MetricsCollector) sortInt64(values []int64) {
	for i := 0; i < len(values)-1; i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

// Reset resets all metrics
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.totalRequests = 0
	mc.totalSuccess = 0
	mc.totalErrors = 0
	mc.totalTokensIn = 0
	mc.totalTokensOut = 0
	mc.totalBytes = 0
	mc.providerMetrics = make(map[string]*ProviderMetrics)
	mc.buckets = make([]*MetricsBucket, 0)
	mc.recentRequests = make([]RequestMetrics, 0)
	mc.latencies = make([]int64, 0)
	mc.ttfts = make([]int64, 0)
	mc.proxyOverheads = make([]int64, 0)
}

// LatencyStats returns detailed latency statistics
func (mc *MetricsCollector) LatencyStats() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return map[string]interface{}{
		"latency": map[string]interface{}{
			"count": len(mc.latencies),
			"avg":   mc.calculateAverage(mc.latencies),
			"min":   mc.calculateMin(mc.latencies),
			"max":   mc.calculateMax(mc.latencies),
			"p50":   mc.calculatePercentile(mc.latencies, 50),
			"p75":   mc.calculatePercentile(mc.latencies, 75),
			"p90":   mc.calculatePercentile(mc.latencies, 90),
			"p95":   mc.calculatePercentile(mc.latencies, 95),
			"p99":   mc.calculatePercentile(mc.latencies, 99),
		},
		"ttft": map[string]interface{}{
			"count": len(mc.ttfts),
			"avg":   mc.calculateAverage(mc.ttfts),
			"min":   mc.calculateMin(mc.ttfts),
			"max":   mc.calculateMax(mc.ttfts),
			"p50":   mc.calculatePercentile(mc.ttfts, 50),
			"p75":   mc.calculatePercentile(mc.ttfts, 75),
			"p90":   mc.calculatePercentile(mc.ttfts, 90),
			"p95":   mc.calculatePercentile(mc.ttfts, 95),
			"p99":   mc.calculatePercentile(mc.ttfts, 99),
		},
		"proxy_overhead": map[string]interface{}{
			"count": len(mc.proxyOverheads),
			"avg":   mc.calculateAverage(mc.proxyOverheads),
			"min":   mc.calculateMin(mc.proxyOverheads),
			"max":   mc.calculateMax(mc.proxyOverheads),
			"p50":   mc.calculatePercentile(mc.proxyOverheads, 50),
			"p95":   mc.calculatePercentile(mc.proxyOverheads, 95),
		},
	}
}

// calculateAverage calculates the average of a slice
func (mc *MetricsCollector) calculateAverage(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := int64(0)
	for _, v := range values {
		total += v
	}
	return float64(total) / float64(len(values))
}

// calculateMin returns the minimum value
func (mc *MetricsCollector) calculateMin(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

// calculateMax returns the maximum value
func (mc *MetricsCollector) calculateMax(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}
