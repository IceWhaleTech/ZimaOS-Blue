package providerpool

import (
	"sync"
	"time"
)

// UsageTracker tracks usage across providers and models
type UsageTracker struct {
	storage        Storage
	pricingManager *PricingManager
	records        chan *UsageRecord
	done           chan struct{}

	// In-memory aggregation
	current   map[string]*usageAggregation
	currentMu sync.RWMutex

	// Configuration
	flushInterval time.Duration
	bufferSize    int
}

// usageAggregation holds in-memory usage aggregation
type usageAggregation struct {
	ProviderID        string
	ModelID           string
	InputTokens       int64
	OutputTokens      int64
	CacheReadTokens   int64
	CacheWriteTokens  int64
	RequestCount      int64
	SuccessCount      int64
	FailureCount      int64
	TotalLatencyMs    int64
	MinLatencyMs      int64
	MaxLatencyMs      int64
	EstimatedCost     float64
	LastUpdated       time.Time
}

// UsageTrackerOption configures the UsageTracker
type UsageTrackerOption func(*UsageTracker)

// WithUsageFlushInterval sets the flush interval
func WithUsageFlushInterval(interval time.Duration) UsageTrackerOption {
	return func(t *UsageTracker) {
		t.flushInterval = interval
	}
}

// WithUsageBufferSize sets the buffer size
func WithUsageBufferSize(size int) UsageTrackerOption {
	return func(t *UsageTracker) {
		t.bufferSize = size
	}
}

// NewUsageTracker creates a new usage tracker
func NewUsageTracker(storage Storage, opts ...UsageTrackerOption) *UsageTracker {
	t := &UsageTracker{
		storage:       storage,
		current:       make(map[string]*usageAggregation),
		flushInterval: 30 * time.Second,
		bufferSize:    1000,
	}

	for _, opt := range opts {
		opt(t)
	}

	t.records = make(chan *UsageRecord, t.bufferSize)
	t.done = make(chan struct{})

	return t
}

// SetPricingManager sets the pricing manager for cost calculation
func (t *UsageTracker) SetPricingManager(pm *PricingManager) {
	t.pricingManager = pm
}

// Start starts the background processing
func (t *UsageTracker) Start() {
	go t.processRecords()
	go t.periodicFlush()
}

// Stop stops the tracker and flushes remaining data
func (t *UsageTracker) Stop() {
	close(t.done)
	t.flush()
}

// Record records a usage event
func (t *UsageTracker) Record(record *UsageRecord) {
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}
	if record.ID == "" {
		record.ID = GenerateID("usage")
	}

	// Calculate cost using pricing manager if available and cost not already set
	if record.EstimatedCost == 0 && t.pricingManager != nil {
		record.EstimatedCost = t.pricingManager.CalculateCost(
			record.ModelID,
			record.ProviderID,
			record.InputTokens,
			record.OutputTokens,
			record.CacheReadTokens,
		)
	}

	select {
	case t.records <- record:
	default:
		t.processRecord(record)
	}
}

// RecordRequest is a convenience method to record a request
func (t *UsageTracker) RecordRequest(providerID, modelID string, inputTokens, outputTokens int64, latencyMs int64, success bool) {
	// Cost will be calculated automatically in Record() if pricingManager is set
	t.Record(&UsageRecord{
		ProviderID:   providerID,
		ModelID:      modelID,
		Timestamp:    time.Now(),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		RequestCount: 1,
		LatencyMs:    latencyMs,
		Success:      success,
	})
}

func (t *UsageTracker) processRecords() {
	for {
		select {
		case <-t.done:
			for {
				select {
				case record := <-t.records:
					t.processRecord(record)
				default:
					return
				}
			}
		case record := <-t.records:
			t.processRecord(record)
		}
	}
}

func (t *UsageTracker) processRecord(record *UsageRecord) {
	t.storage.AppendUsage(record)
	t.updateAggregation(record)
}

func (t *UsageTracker) updateAggregation(record *UsageRecord) {
	key := record.ProviderID + ":" + record.ModelID

	t.currentMu.Lock()
	defer t.currentMu.Unlock()

	agg, exists := t.current[key]
	if !exists {
		agg = &usageAggregation{
			ProviderID:   record.ProviderID,
			ModelID:      record.ModelID,
			MinLatencyMs: record.LatencyMs,
			MaxLatencyMs: record.LatencyMs,
		}
		t.current[key] = agg
	}

	agg.InputTokens += record.InputTokens
	agg.OutputTokens += record.OutputTokens
	agg.CacheReadTokens += record.CacheReadTokens
	agg.CacheWriteTokens += record.CacheWriteTokens
	agg.RequestCount += record.RequestCount
	agg.TotalLatencyMs += record.LatencyMs
	agg.EstimatedCost += record.EstimatedCost
	agg.LastUpdated = time.Now()

	if record.Success {
		agg.SuccessCount++
	} else {
		agg.FailureCount++
	}

	if record.LatencyMs > 0 {
		if record.LatencyMs < agg.MinLatencyMs || agg.MinLatencyMs == 0 {
			agg.MinLatencyMs = record.LatencyMs
		}
		if record.LatencyMs > agg.MaxLatencyMs {
			agg.MaxLatencyMs = record.LatencyMs
		}
	}
}

func (t *UsageTracker) periodicFlush() {
	ticker := time.NewTicker(t.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			t.flush()
		}
	}
}

func (t *UsageTracker) flush() {
	t.currentMu.Lock()
	t.current = make(map[string]*usageAggregation)
	t.currentMu.Unlock()
}

// GetCurrentStats returns current in-memory statistics
func (t *UsageTracker) GetCurrentStats() map[string]*UsageSummary {
	t.currentMu.RLock()
	defer t.currentMu.RUnlock()

	stats := make(map[string]*UsageSummary)

	for key, agg := range t.current {
		avgLatency := int64(0)
		if agg.RequestCount > 0 {
			avgLatency = agg.TotalLatencyMs / agg.RequestCount
		}

		stats[key] = &UsageSummary{
			ProviderID:            agg.ProviderID,
			ModelID:               agg.ModelID,
			Period:                "current",
			TotalInputTokens:      agg.InputTokens,
			TotalOutputTokens:     agg.OutputTokens,
			TotalCacheReadTokens:  agg.CacheReadTokens,
			TotalCacheWriteTokens: agg.CacheWriteTokens,
			TotalRequests:         agg.RequestCount,
			SuccessfulRequests:    agg.SuccessCount,
			FailedRequests:        agg.FailureCount,
			TotalEstimatedCost:    agg.EstimatedCost,
			AvgLatencyMs:          avgLatency,
			MinLatencyMs:          agg.MinLatencyMs,
			MaxLatencyMs:          agg.MaxLatencyMs,
		}
	}

	return stats
}

// GetUsage retrieves usage records for a time range
func (t *UsageTracker) GetUsage(providerID string, start, end time.Time) ([]*UsageRecord, error) {
	return t.storage.LoadUsage(providerID, start, end)
}

// GetSummary calculates a usage summary for a time range
func (t *UsageTracker) GetSummary(providerID string, start, end time.Time) (*UsageSummary, error) {
	records, err := t.storage.LoadUsage(providerID, start, end)
	if err != nil {
		return nil, err
	}

	summary := &UsageSummary{
		ProviderID: providerID,
		Period:     "custom",
		StartTime:  start,
		EndTime:    end,
	}

	if len(records) == 0 {
		return summary, nil
	}

	var totalLatency int64
	var latencyCount int64

	for _, record := range records {
		summary.TotalInputTokens += record.InputTokens
		summary.TotalOutputTokens += record.OutputTokens
		summary.TotalCacheReadTokens += record.CacheReadTokens
		summary.TotalCacheWriteTokens += record.CacheWriteTokens
		summary.TotalRequests += record.RequestCount
		summary.TotalEstimatedCost += record.EstimatedCost

		if record.Success {
			summary.SuccessfulRequests++
		} else {
			summary.FailedRequests++
		}

		if record.LatencyMs > 0 {
			totalLatency += record.LatencyMs
			latencyCount++

			if summary.MinLatencyMs == 0 || record.LatencyMs < summary.MinLatencyMs {
				summary.MinLatencyMs = record.LatencyMs
			}
			if record.LatencyMs > summary.MaxLatencyMs {
				summary.MaxLatencyMs = record.LatencyMs
			}
		}
	}

	if latencyCount > 0 {
		summary.AvgLatencyMs = totalLatency / latencyCount
	}

	return summary, nil
}

// GetWeeklySummary returns usage summary for the past 7 days
func (t *UsageTracker) GetWeeklySummary(providerID string) (*UsageSummary, error) {
	end := time.Now()
	start := end.AddDate(0, 0, -7)
	summary, err := t.GetSummary(providerID, start, end)
	if err != nil {
		return nil, err
	}
	summary.Period = "week"
	return summary, nil
}

// GetProviderStats returns aggregated stats per provider
func (t *UsageTracker) GetProviderStats(start, end time.Time) (map[string]*UsageSummary, error) {
	records, err := t.storage.LoadUsage("", start, end)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]*UsageSummary)

	for _, record := range records {
		summary, exists := stats[record.ProviderID]
		if !exists {
			summary = &UsageSummary{
				ProviderID: record.ProviderID,
				Period:     "custom",
				StartTime:  start,
				EndTime:    end,
			}
			stats[record.ProviderID] = summary
		}

		summary.TotalInputTokens += record.InputTokens
		summary.TotalOutputTokens += record.OutputTokens
		summary.TotalRequests += record.RequestCount
		summary.TotalEstimatedCost += record.EstimatedCost

		if record.Success {
			summary.SuccessfulRequests++
		} else {
			summary.FailedRequests++
		}
	}

	return stats, nil
}

// GetModelStats returns aggregated stats per model
func (t *UsageTracker) GetModelStats(providerID string, start, end time.Time) (map[string]*UsageSummary, error) {
	records, err := t.storage.LoadUsage(providerID, start, end)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]*UsageSummary)

	for _, record := range records {
		summary, exists := stats[record.ModelID]
		if !exists {
			summary = &UsageSummary{
				ProviderID: record.ProviderID,
				ModelID:    record.ModelID,
				Period:     "custom",
				StartTime:  start,
				EndTime:    end,
			}
			stats[record.ModelID] = summary
		}

		summary.TotalInputTokens += record.InputTokens
		summary.TotalOutputTokens += record.OutputTokens
		summary.TotalRequests += record.RequestCount
		summary.TotalEstimatedCost += record.EstimatedCost

		if record.Success {
			summary.SuccessfulRequests++
		} else {
			summary.FailedRequests++
		}
	}

	return stats, nil
}
