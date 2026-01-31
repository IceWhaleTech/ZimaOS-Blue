package metrics

import (
	"sort"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/timeutil"
)

// CallCollector collects and aggregates API call metrics.
type CallCollector struct {
	mu sync.RWMutex

	// Raw call records (kept for a limited time)
	records []CallRecord

	// Aggregated stats by model
	modelStats map[string]*modelStatsAccumulator

	// Global stats
	globalStats *statsAccumulator

	// Configuration
	maxRecords    int
	retentionTime time.Duration
}

// modelStatsAccumulator accumulates statistics for a model.
type modelStatsAccumulator struct {
	calls           int64
	successfulCalls int64
	failedCalls     int64
	timeoutCalls    int64
	rateLimitCalls  int64
	errorCalls      int64

	// Latency samples for percentile calculation
	latencies []float64

	// Token totals
	inputTokens      int64
	outputTokens     int64
	cacheReadTokens  int64
	cacheWriteTokens int64

	// Speed samples
	tokensPerSecond []float64
	ttftSamples     []float64

	// For average calculation
	totalLatency float64
	totalTPS     float64
	totalTTFT    float64
}

// statsAccumulator accumulates global statistics.
type statsAccumulator struct {
	totalCalls      int64
	successfulCalls int64
	failedCalls     int64
	timeoutCalls    int64
	rateLimitCalls  int64
	errorCalls      int64
}

// NewCallCollector creates a new CallCollector.
func NewCallCollector(maxRecords int, retentionTime time.Duration) *CallCollector {
	return &CallCollector{
		records:       make([]CallRecord, 0, maxRecords),
		modelStats:    make(map[string]*modelStatsAccumulator),
		globalStats:   &statsAccumulator{},
		maxRecords:    maxRecords,
		retentionTime: retentionTime,
	}
}

// RecordCall records a single API call.
func (c *CallCollector) RecordCall(record CallRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Set timestamp if not set
	if record.Timestamp.IsZero() {
		record.Timestamp = timeutil.NowTime()
	}

	// Add to records
	c.records = append(c.records, record)

	// Trim old records
	c.trimRecords()

	// Update global stats
	c.globalStats.totalCalls++
	if record.Success {
		c.globalStats.successfulCalls++
	} else {
		c.globalStats.failedCalls++
		switch record.ErrorType {
		case ErrorTypeTimeout:
			c.globalStats.timeoutCalls++
		case ErrorTypeRateLimit:
			c.globalStats.rateLimitCalls++
		default:
			c.globalStats.errorCalls++
		}
	}

	// Update model stats
	model := record.Model
	if model == "" {
		model = "unknown"
	}

	stats, ok := c.modelStats[model]
	if !ok {
		stats = &modelStatsAccumulator{
			latencies:       make([]float64, 0, 1000),
			tokensPerSecond: make([]float64, 0, 1000),
			ttftSamples:     make([]float64, 0, 1000),
		}
		c.modelStats[model] = stats
	}

	stats.calls++
	if record.Success {
		stats.successfulCalls++
	} else {
		stats.failedCalls++
		switch record.ErrorType {
		case ErrorTypeTimeout:
			stats.timeoutCalls++
		case ErrorTypeRateLimit:
			stats.rateLimitCalls++
		default:
			stats.errorCalls++
		}
	}

	// Record latency
	if record.LatencyMs > 0 {
		stats.latencies = append(stats.latencies, record.LatencyMs)
		stats.totalLatency += record.LatencyMs
	}

	// Record tokens
	stats.inputTokens += record.InputTokens
	stats.outputTokens += record.OutputTokens
	stats.cacheReadTokens += record.CacheReadTokens
	stats.cacheWriteTokens += record.CacheWriteTokens

	// Record speed
	if record.TokensPerSecond > 0 {
		stats.tokensPerSecond = append(stats.tokensPerSecond, record.TokensPerSecond)
		stats.totalTPS += record.TokensPerSecond
	}

	if record.TimeToFirstToken > 0 {
		stats.ttftSamples = append(stats.ttftSamples, record.TimeToFirstToken)
		stats.totalTTFT += record.TimeToFirstToken
	}
}

// trimRecords removes old records beyond retention time.
func (c *CallCollector) trimRecords() {
	if len(c.records) == 0 {
		return
	}

	cutoff := timeutil.NowTime().Add(-c.retentionTime)
	idx := 0
	for i, r := range c.records {
		if r.Timestamp.After(cutoff) {
			idx = i
			break
		}
	}

	if idx > 0 {
		c.records = c.records[idx:]
	}

	// Also trim if exceeds max
	if len(c.records) > c.maxRecords {
		c.records = c.records[len(c.records)-c.maxRecords:]
	}
}

// GetCallStats returns current call statistics.
func (c *CallCollector) GetCallStats() *CallStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &CallStats{
		TotalCalls:      c.globalStats.totalCalls,
		SuccessfulCalls: c.globalStats.successfulCalls,
		FailedCalls:     c.globalStats.failedCalls,
		TimeoutCalls:    c.globalStats.timeoutCalls,
		RateLimitCalls:  c.globalStats.rateLimitCalls,
		ErrorCalls:      c.globalStats.errorCalls,
	}

	if stats.TotalCalls > 0 {
		stats.SuccessRate = float64(stats.SuccessfulCalls) / float64(stats.TotalCalls) * 100
	}

	return stats
}

// GetCallStatsByPeriod returns call statistics for a specific period.
func (c *CallCollector) GetCallStatsByPeriod(period string) *CallStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	duration := periodToDuration(period)
	cutoff := timeutil.NowTime().Add(-duration)

	stats := &CallStats{}

	for _, r := range c.records {
		if r.Timestamp.After(cutoff) {
			stats.TotalCalls++
			if r.Success {
				stats.SuccessfulCalls++
			} else {
				stats.FailedCalls++
				switch r.ErrorType {
				case ErrorTypeTimeout:
					stats.TimeoutCalls++
				case ErrorTypeRateLimit:
					stats.RateLimitCalls++
				default:
					stats.ErrorCalls++
				}
			}
		}
	}

	if stats.TotalCalls > 0 {
		stats.SuccessRate = float64(stats.SuccessfulCalls) / float64(stats.TotalCalls) * 100
	}

	return stats
}

// GetModelStats returns statistics for all models.
func (c *CallCollector) GetModelStats() []ModelStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]ModelStats, 0, len(c.modelStats))

	for model, acc := range c.modelStats {
		stats := c.buildModelStats(model, acc)
		result = append(result, stats)
	}

	// Sort by calls descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Calls > result[j].Calls
	})

	return result
}

// GetModelStatsByName returns statistics for a specific model.
func (c *CallCollector) GetModelStatsByName(model string) *ModelStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	acc, ok := c.modelStats[model]
	if !ok {
		return nil
	}

	stats := c.buildModelStats(model, acc)
	return &stats
}

// buildModelStats builds ModelStats from accumulator.
func (c *CallCollector) buildModelStats(model string, acc *modelStatsAccumulator) ModelStats {
	stats := ModelStats{
		Model:            model,
		Calls:            acc.calls,
		SuccessfulCalls:  acc.successfulCalls,
		FailedCalls:      acc.failedCalls,
		InputTokens:      acc.inputTokens,
		OutputTokens:     acc.outputTokens,
		TotalTokens:      acc.inputTokens + acc.outputTokens,
		CacheReadTokens:  acc.cacheReadTokens,
		CacheWriteTokens: acc.cacheWriteTokens,
	}

	if stats.Calls > 0 {
		stats.SuccessRate = float64(stats.SuccessfulCalls) / float64(stats.Calls) * 100
	}

	// Calculate latency stats
	if len(acc.latencies) > 0 {
		stats.AvgLatency = acc.totalLatency / float64(len(acc.latencies))
		stats.P50Latency = percentile(acc.latencies, 50)
		stats.P95Latency = percentile(acc.latencies, 95)
		stats.P99Latency = percentile(acc.latencies, 99)
	}

	// Calculate speed stats
	if len(acc.tokensPerSecond) > 0 {
		stats.AvgTokensPerSecond = acc.totalTPS / float64(len(acc.tokensPerSecond))
	}

	if len(acc.ttftSamples) > 0 {
		stats.AvgTimeToFirstToken = acc.totalTTFT / float64(len(acc.ttftSamples))
	}

	return stats
}

// GetHourlyStats returns hourly statistics for the last 24 hours.
func (c *CallCollector) GetHourlyStats() []HourlyStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := timeutil.NowTime()
	hourlyMap := make(map[time.Time]*HourlyStats)

	// Initialize last 24 hours
	for i := 0; i < 24; i++ {
		hour := now.Add(-time.Duration(i) * time.Hour).Truncate(time.Hour)
		hourlyMap[hour] = &HourlyStats{Hour: hour}
	}

	// Aggregate records
	for _, r := range c.records {
		hour := r.Timestamp.Truncate(time.Hour)
		if stats, ok := hourlyMap[hour]; ok {
			stats.Calls++
			if r.Success {
				// Will calculate success rate later
			}
		}
	}

	// Calculate success rates and convert to slice
	result := make([]HourlyStats, 0, 24)
	for _, stats := range hourlyMap {
		if stats.Calls > 0 {
			// Count successes
			successCount := int64(0)
			for _, r := range c.records {
				if r.Timestamp.Truncate(time.Hour).Equal(stats.Hour) && r.Success {
					successCount++
				}
			}
			stats.SuccessRate = float64(successCount) / float64(stats.Calls) * 100
		}
		result = append(result, *stats)
	}

	// Sort by hour
	sort.Slice(result, func(i, j int) bool {
		return result[i].Hour.Before(result[j].Hour)
	})

	return result
}

// Reset resets all statistics.
func (c *CallCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.records = make([]CallRecord, 0, c.maxRecords)
	c.modelStats = make(map[string]*modelStatsAccumulator)
	c.globalStats = &statsAccumulator{}
}

// LoadModelStats loads model statistics from persisted data.
func (c *CallCollector) LoadModelStats(stats []ModelStats) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, stat := range stats {
		acc := &modelStatsAccumulator{
			calls:            stat.Calls,
			successfulCalls:  stat.SuccessfulCalls,
			failedCalls:      stat.FailedCalls,
			inputTokens:      stat.InputTokens,
			outputTokens:     stat.OutputTokens,
			cacheReadTokens:  stat.CacheReadTokens,
			cacheWriteTokens: stat.CacheWriteTokens,
			latencies:        make([]float64, 0, 1000),
			tokensPerSecond:  make([]float64, 0, 1000),
			ttftSamples:      make([]float64, 0, 1000),
		}

		// Restore average latency as a single sample
		if stat.AvgLatency > 0 {
			acc.latencies = append(acc.latencies, stat.AvgLatency)
			acc.totalLatency = stat.AvgLatency
		}

		c.modelStats[stat.Model] = acc

		// Update global stats
		c.globalStats.totalCalls += stat.Calls
		c.globalStats.successfulCalls += stat.SuccessfulCalls
		c.globalStats.failedCalls += stat.FailedCalls
	}
}

// periodToDuration converts a period string to duration.
func periodToDuration(period string) time.Duration {
	switch period {
	case PeriodRealtime:
		return time.Minute
	case PeriodHourly:
		return time.Hour
	case PeriodDaily:
		return 24 * time.Hour
	case PeriodWeekly:
		return 7 * 24 * time.Hour
	case PeriodMonthly:
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// percentile calculates the p-th percentile of a slice.
func percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}

	// Make a copy and sort
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	// Calculate index
	idx := (p / 100) * float64(len(sorted)-1)
	lower := int(idx)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	// Linear interpolation
	weight := idx - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}
