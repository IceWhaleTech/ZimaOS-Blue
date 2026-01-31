package metrics

import (
	"context"
	"sync"
	"time"
)

// MetricsWriter coordinates metrics collection and storage.
type MetricsWriter struct {
	store          MetricsStore
	sqliteStore    *SQLiteStore // SQLite store for persistence
	callCollector  *CallCollector
	tokenTracker   *TokenTracker
	latencyTracker *LatencyTracker
	systemMonitor  *SystemMonitor

	// Configuration
	config *WriterConfig

	// Background collection
	done chan struct{}
	wg   sync.WaitGroup
}

// WriterConfig contains configuration for the metrics writer.
type WriterConfig struct {
	// How often to collect and write metrics
	CollectionInterval time.Duration

	// Maximum samples to keep in memory
	MaxSamples int

	// Enable system metrics collection
	EnableSystemMetrics bool

	// System metrics collection interval
	SystemMetricsInterval time.Duration

	// Disk path for disk usage monitoring
	DiskPath string

	// SQLite database path for persistence
	SQLiteDBPath string

	// Persistence save interval
	PersistenceInterval time.Duration
}

// DefaultWriterConfig returns the default writer configuration.
func DefaultWriterConfig() *WriterConfig {
	return &WriterConfig{
		CollectionInterval:    10 * time.Second,
		MaxSamples:            1000,
		EnableSystemMetrics:   true,
		SystemMetricsInterval: 30 * time.Second,
		PersistenceInterval:   60 * time.Second, // Save every minute
	}
}

// NewMetricsWriter creates a new MetricsWriter.
func NewMetricsWriter(store MetricsStore, config *WriterConfig) *MetricsWriter {
	if config == nil {
		config = DefaultWriterConfig()
	}

	var systemMonitor *SystemMonitor
	if config.EnableSystemMetrics {
		systemMonitor = NewSystemMonitor(config.MaxSamples, config.DiskPath)
	}

	w := &MetricsWriter{
		store:          store,
		callCollector:  NewCallCollector(config.MaxSamples, time.Hour),
		tokenTracker:   NewTokenTracker(),
		latencyTracker: NewLatencyTracker(config.MaxSamples),
		systemMonitor:  systemMonitor,
		config:         config,
		done:           make(chan struct{}),
	}

	// Initialize SQLite store if path is configured
	if config.SQLiteDBPath != "" {
		sqliteStore, err := NewSQLiteStore(config.SQLiteDBPath)
		if err == nil {
			w.sqliteStore = sqliteStore
			// Load persisted data
			w.loadPersistedData()
		}
	}

	return w
}

// loadPersistedData loads metrics data from SQLite on startup.
func (w *MetricsWriter) loadPersistedData() {
	if w.sqliteStore == nil {
		return
	}

	ctx := context.Background()

	// Load global token usage
	usage, err := w.sqliteStore.LoadTokenUsage(ctx)
	if err == nil && usage != nil {
		w.tokenTracker.LoadUsage(usage)
	}

	// Load model token usage
	modelUsages, err := w.sqliteStore.LoadModelTokenUsage(ctx)
	if err == nil && len(modelUsages) > 0 {
		w.tokenTracker.LoadModelUsages(modelUsages)
	}

	// Load model stats into call collector
	modelStats, err := w.sqliteStore.LoadModelMetrics(ctx)
	if err == nil && len(modelStats) > 0 {
		w.callCollector.LoadModelStats(modelStats)
	}
}

// persistData saves metrics data to SQLite.
func (w *MetricsWriter) persistData() {
	if w.sqliteStore == nil {
		return
	}

	ctx := context.Background()

	// Save global token usage
	usage := w.tokenTracker.GetTokenUsage()
	if usage != nil {
		w.sqliteStore.SaveTokenUsage(ctx, usage)
	}

	// Save model token usage
	modelUsages := w.tokenTracker.GetAllModelUsage()
	if len(modelUsages) > 0 {
		w.sqliteStore.SaveModelTokenUsage(ctx, modelUsages)
	}

	// Save model stats
	modelStats := w.GetModelStats()
	if len(modelStats) > 0 {
		w.sqliteStore.SaveModelMetrics(ctx, modelStats)
	}
}

// Start starts background metrics collection.
func (w *MetricsWriter) Start() {
	// Load persisted data if not already loaded
	w.loadPersistedData()

	// Start periodic flush
	w.wg.Add(1)
	go w.collectionLoop()

	// Start system metrics collection if enabled
	if w.config.EnableSystemMetrics {
		w.wg.Add(1)
		go w.systemMetricsLoop()
	}

	// Start persistence loop if SQLite store is available
	if w.sqliteStore != nil {
		w.wg.Add(1)
		go w.persistenceLoop()
	}
}

// Stop stops background metrics collection.
func (w *MetricsWriter) Stop() {
	close(w.done)
	w.wg.Wait()

	// Final persist before shutdown
	w.persistData()

	// Close SQLite store
	if w.sqliteStore != nil {
		w.sqliteStore.Close()
	}
}

// persistenceLoop periodically saves metrics to SQLite.
func (w *MetricsWriter) persistenceLoop() {
	defer w.wg.Done()

	interval := w.config.PersistenceInterval
	if interval <= 0 {
		interval = 60 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.persistData()
		case <-w.done:
			return
		}
	}
}

// collectionLoop periodically writes collected metrics to storage.
func (w *MetricsWriter) collectionLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.CollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.writeCollectedMetrics()
		case <-w.done:
			// Final write
			w.writeCollectedMetrics()
			return
		}
	}
}

// systemMetricsLoop periodically collects system metrics.
func (w *MetricsWriter) systemMetricsLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.SystemMetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.collectSystemMetrics()
		case <-w.done:
			return
		}
	}
}

// writeCollectedMetrics writes all collected metrics to storage.
func (w *MetricsWriter) writeCollectedMetrics() {
	// Skip if store is nil
	if w.store == nil {
		return
	}

	ctx := context.Background()

	// Write call statistics
	stats := w.callCollector.GetCallStats()
	if stats.TotalCalls > 0 {
		point := NewPoint(MeasurementAPICalls).
			AddField("total_calls", stats.TotalCalls).
			AddField("successful_calls", stats.SuccessfulCalls).
			AddField("failed_calls", stats.FailedCalls).
			AddField("success_rate", stats.SuccessRate)
		w.store.Write(ctx, point)
	}

	// Write model-specific stats
	modelStats := w.callCollector.GetModelStats()
	for _, ms := range modelStats {
		point := NewPoint(MeasurementAPICalls).
			AddTag(TagModel, ms.Model).
			AddField("calls", ms.Calls).
			AddField("successful_calls", ms.SuccessfulCalls).
			AddField("avg_latency_ms", ms.AvgLatency).
			AddField("total_tokens", ms.TotalTokens).
			AddField("estimated_cost", ms.EstimatedCost)
		w.store.Write(ctx, point)
	}

	// Write token usage
	tokenUsage := w.tokenTracker.GetTokenUsage()
	if tokenUsage.TotalTokens > 0 {
		point := NewPoint(MeasurementTokenUsage).
			AddField(FieldInputTokens, tokenUsage.InputTokens).
			AddField(FieldOutputTokens, tokenUsage.OutputTokens).
			AddField(FieldTotalTokens, tokenUsage.TotalTokens).
			AddField(FieldCacheReadTokens, tokenUsage.CacheReadTokens).
			AddField(FieldCacheWriteTokens, tokenUsage.CacheWriteTokens).
			AddField(FieldCost, tokenUsage.EstimatedCost)
		w.store.Write(ctx, point)
	}

	// Write latency stats
	latencyStats := w.latencyTracker.GetLatencyStats()
	if latencyStats.Samples > 0 {
		point := NewPoint(MeasurementLatency).
			AddField("min_ms", latencyStats.Min).
			AddField("max_ms", latencyStats.Max).
			AddField("avg_ms", latencyStats.Avg).
			AddField("p50_ms", latencyStats.P50).
			AddField("p95_ms", latencyStats.P95).
			AddField("p99_ms", latencyStats.P99).
			AddField("samples", latencyStats.Samples)
		w.store.Write(ctx, point)
	}

	// Write speed stats
	speedStats := w.latencyTracker.GetSpeedStats()
	if speedStats.TokensPerSecond > 0 {
		point := NewPoint(MeasurementSpeed).
			AddField(FieldTokensPerSecond, speedStats.TokensPerSecond).
			AddField("avg_tokens_per_second", speedStats.AvgTokensPerSecond).
			AddField("max_tokens_per_second", speedStats.MaxTokensPerSecond).
			AddField(FieldTTFT, speedStats.TimeToFirstToken)
		w.store.Write(ctx, point)
	}
}

// collectSystemMetrics collects and writes system metrics.
func (w *MetricsWriter) collectSystemMetrics() {
	if w.systemMonitor == nil {
		return
	}

	// Collect system metrics
	if err := w.systemMonitor.Collect(); err != nil {
		return
	}

	// Write to store if available
	if w.store != nil {
		ctx := context.Background()
		metrics := w.systemMonitor.GetSystemMetrics()

		point := NewPoint(MeasurementSystem).
			AddField(FieldCPUPercent, metrics.CPUUsagePercent).
			AddField(FieldMemoryBytes, metrics.MemoryUsed).
			AddField(FieldMemoryPercent, metrics.MemoryPercent).
			AddField(FieldDiskPercent, metrics.DiskPercent)

		w.store.Write(ctx, point)
	}
}

// RecordAPICall records an API call with all metrics.
func (w *MetricsWriter) RecordAPICall(model string, success bool, latencyMs float64, inputTokens, outputTokens, cacheRead, cacheWrite int64, errorType string) {
	// Record to call collector
	record := CallRecord{
		Model:            model,
		Success:          success,
		LatencyMs:        latencyMs,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		CacheReadTokens:  cacheRead,
		CacheWriteTokens: cacheWrite,
		ErrorType:        errorType,
	}
	w.callCollector.RecordCall(record)

	// Record token usage
	w.tokenTracker.RecordTokenUsage(model, inputTokens, outputTokens, cacheRead, cacheWrite)

	// Record latency
	w.latencyTracker.RecordLatency(model, latencyMs)

	// Write to store immediately for real-time data
	if w.store != nil {
		ctx := context.Background()

		// Write API call point
		point := NewPoint(MeasurementAPICalls).
			AddTag(TagModel, model).
			AddTag(TagStatus, boolToStatus(success)).
			AddField(FieldCount, 1).
			AddField(FieldLatencyMs, latencyMs).
			AddField(FieldInputTokens, inputTokens).
			AddField(FieldOutputTokens, outputTokens).
			AddField(FieldTotalTokens, inputTokens+outputTokens).
			AddField(FieldCost, w.tokenTracker.CalculateCost(model, inputTokens, outputTokens, cacheRead, cacheWrite))

		if errorType != "" {
			point.AddTag(TagErrorType, errorType)
		}

		w.store.Write(ctx, point)
	}
}

// RecordSpeed records speed metrics.
func (w *MetricsWriter) RecordSpeed(model string, tokensPerSecond, ttftMs, decodeSpeed float64) {
	w.latencyTracker.RecordSpeed(model, tokensPerSecond, ttftMs, decodeSpeed)

	if w.store != nil {
		ctx := context.Background()
		point := NewPoint(MeasurementSpeed).
			AddTag(TagModel, model).
			AddField(FieldTokensPerSecond, tokensPerSecond).
			AddField(FieldTTFT, ttftMs).
			AddField("decode_speed", decodeSpeed)
		w.store.Write(ctx, point)
	}
}

// GetCallStats returns current call statistics.
func (w *MetricsWriter) GetCallStats() *CallStats {
	return w.callCollector.GetCallStats()
}

// GetModelStats returns statistics for all models with cost data.
func (w *MetricsWriter) GetModelStats() []ModelStats {
	stats := w.callCollector.GetModelStats()

	// Merge cost data from token tracker
	for i := range stats {
		usage := w.tokenTracker.GetTokenUsageByModel(stats[i].Model)
		if usage != nil {
			stats[i].EstimatedCost = usage.EstimatedCost
		}
	}

	return stats
}

// GetTokenUsage returns current token usage.
func (w *MetricsWriter) GetTokenUsage() *TokenUsage {
	return w.tokenTracker.GetTokenUsage()
}

// GetLatencyStats returns current latency statistics.
func (w *MetricsWriter) GetLatencyStats() *LatencyStats {
	return w.latencyTracker.GetLatencyStats()
}

// GetSpeedStats returns current speed statistics.
func (w *MetricsWriter) GetSpeedStats() *GenerationSpeed {
	return w.latencyTracker.GetSpeedStats()
}

// GetAllModelUsage returns token usage for all models.
func (w *MetricsWriter) GetAllModelUsage() []ModelTokenUsage {
	return w.tokenTracker.GetAllModelUsage()
}

// Reset resets all metrics.
func (w *MetricsWriter) Reset() {
	w.callCollector.Reset()
	w.tokenTracker.Reset()
	w.latencyTracker.Reset()
	if w.systemMonitor != nil {
		w.systemMonitor.Reset()
	}
}

// GetSystemMetrics returns current system metrics.
func (w *MetricsWriter) GetSystemMetrics() *SystemResourceMetrics {
	if w.systemMonitor == nil {
		return nil
	}
	return w.systemMonitor.GetSystemMetrics()
}

// GetResourceHistory returns resource history.
func (w *MetricsWriter) GetResourceHistory() []ResourceHistory {
	if w.systemMonitor == nil {
		return nil
	}
	return w.systemMonitor.GetResourceHistory()
}

// GetCurrentProcessMetrics returns metrics for the current process.
func (w *MetricsWriter) GetCurrentProcessMetrics() (*ProcessMetrics, error) {
	if w.systemMonitor == nil {
		return nil, nil
	}
	return w.systemMonitor.GetCurrentProcessMetrics()
}
