package metrics

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
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
	mu   sync.Mutex // Protects Stop() from being called multiple times

	systemMonitorOnce     sync.Once
	systemMetricsLoopOnce sync.Once
	started               atomic.Bool
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

	// SQLite database path for persistence when using a dedicated metrics DB.
	SQLiteDBPath string

	// SharedSQLiteDB reuses an existing SQLite database instead of opening metrics.db.
	SharedSQLiteDB *sql.DB

	// SharedSQLiteReadDB optionally provides a separate reader when
	// SharedSQLiteDB points at a writer-only pool.
	SharedSQLiteReadDB *sql.DB

	// Persistence save interval
	PersistenceInterval time.Duration
}

// DefaultWriterConfig returns the default writer configuration.
func DefaultWriterConfig() *WriterConfig {
	return &WriterConfig{
		CollectionInterval:    10 * time.Second,
		MaxSamples:            50,
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

	w := &MetricsWriter{
		store:          store,
		callCollector:  NewCallCollector(config.MaxSamples, time.Hour),
		tokenTracker:   NewTokenTracker(),
		latencyTracker: NewLatencyTracker(config.MaxSamples),
		config:         config,
		done:           make(chan struct{}),
	}

	// Initialize SQLite store if configured.
	if config.SharedSQLiteDB != nil {
		sqliteStore, err := NewSQLiteStoreWithReadDB(config.SharedSQLiteDB, config.SharedSQLiteReadDB)
		if err == nil {
			w.sqliteStore = sqliteStore
			w.loadPersistedData()
		}
	} else if config.SQLiteDBPath != "" {
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

	// Load latency samples for percentile calculations
	latencySamples, err := w.sqliteStore.LoadLatencySamples(ctx)
	if err == nil && len(latencySamples) > 0 {
		exports := make([]LatencySampleExport, len(latencySamples))
		for i, sample := range latencySamples {
			exports[i] = LatencySampleExport{
				Model:       sample.Model,
				SampleType:  sample.SampleType,
				Samples:     sample.Samples,
				TotalValue:  sample.TotalValue,
				MinValue:    sample.MinValue,
				MaxValue:    sample.MaxValue,
				SampleCount: sample.SampleCount,
			}
		}
		w.latencyTracker.LoadLatencySamples(exports)
	}
}

// persistData saves metrics data to SQLite.
// On corruption, rotates the broken DB aside and recreates a fresh one.
func (w *MetricsWriter) persistData() {
	if w.sqliteStore == nil {
		return
	}

	ctx := context.Background()
	var writeErr error

	// Save global token usage
	usage := w.tokenTracker.GetTokenUsage()
	if usage != nil {
		if err := w.sqliteStore.SaveTokenUsage(ctx, usage); err != nil {
			writeErr = err
		}
	}

	// Save model token usage
	modelUsages := w.tokenTracker.GetAllModelUsage()
	if len(modelUsages) > 0 {
		if err := w.sqliteStore.SaveModelTokenUsage(ctx, modelUsages); err != nil {
			writeErr = err
		}
	}

	// Save model stats
	modelStats := w.GetModelStats()
	if len(modelStats) > 0 {
		if err := w.sqliteStore.SaveModelMetrics(ctx, modelStats); err != nil {
			writeErr = err
		}
	}

	// Save latency samples for percentile calculations
	latencyExports := w.latencyTracker.ExportLatencySamples()
	if len(latencyExports) > 0 {
		samples := make([]LatencySampleData, len(latencyExports))
		for i, exp := range latencyExports {
			samples[i] = LatencySampleData{
				Model:       exp.Model,
				SampleType:  exp.SampleType,
				Samples:     exp.Samples,
				TotalValue:  exp.TotalValue,
				MinValue:    exp.MinValue,
				MaxValue:    exp.MaxValue,
				SampleCount: exp.SampleCount,
			}
		}
		if err := w.sqliteStore.SaveLatencySamples(ctx, samples); err != nil {
			writeErr = err
		}
	}

	// Self-heal only on confirmed corruption; transient IO/lock failures
	// should not cause us to rotate the metrics database away.
	if writeErr != nil && dbutil.IsSQLiteCorruptionError(writeErr) {
		w.resetSQLiteStore()
	}
}

// Start starts background metrics collection.
func (w *MetricsWriter) Start() {
	// Load persisted data if not already loaded
	w.loadPersistedData()
	w.started.Store(true)

	// Start periodic flush
	w.wg.Add(1)
	go w.collectionLoop()

	// Persisted/system-backed writers still start system metrics collection eagerly.
	if w.config.EnableSystemMetrics && (w.store != nil || w.systemMonitor != nil) {
		w.startSystemMetricsLoopIfNeeded()
	}

	// Start persistence loop if SQLite store is available
	if w.sqliteStore != nil {
		w.wg.Add(1)
		go w.persistenceLoop()
	}
}

// Stop stops background metrics collection.
func (w *MetricsWriter) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if already stopped
	select {
	case <-w.done:
		// Already stopped
		return
	default:
		close(w.done)
	}

	w.wg.Wait()

	// Final persist before shutdown
	w.persistData()

	// Close SQLite store
	if w.sqliteStore != nil {
		w.sqliteStore.Close()
	}
}

// resetSQLiteStore closes the broken DB, first tries to salvage readable data
// via batch sqlite recovery, and falls back to rotating it to .bak.<timestamp>
// before opening a fresh database.
func (w *MetricsWriter) resetSQLiteStore() {
	if w.sqliteStore == nil {
		return
	}
	if !w.sqliteStore.ownsDB {
		// Shared blue.db should not be rotated or closed by the metrics layer.
		w.sqliteStore = nil
		return
	}

	dbPath := w.sqliteStore.dbPath
	_ = w.sqliteStore.Close()
	if repairResult, err := dbutil.RepairSQLiteDatabase(dbPath); err != nil {
		slog.Warn("metrics sqlite recovery failed; rotating corrupted database and recreating fresh",
			"db_path", dbPath,
			"error", err,
		)
		if _, rotateErr := dbutil.RotateCorruptSQLiteDatabase(dbPath); rotateErr != nil {
			slog.Warn("metrics sqlite rotation failed after recovery failure",
				"db_path", dbPath,
				"error", rotateErr,
			)
			w.sqliteStore = nil
			return
		}
	} else if repairResult != nil && repairResult.PartialImport {
		slog.Warn("metrics sqlite salvaged via partial batch recovery",
			"db_path", dbPath,
			"warning", repairResult.RecoverWarning,
		)
	} else {
		slog.Info("metrics sqlite repaired via batch recovery", "db_path", dbPath)
	}
	newStore, err := NewSQLiteStore(dbPath)
	if err != nil {
		// Give up on persistence — in-memory metrics still work
		w.sqliteStore = nil
		return
	}
	w.sqliteStore = newStore
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
	monitor := w.ensureSystemMonitor()
	if monitor == nil {
		return
	}

	// Collect system metrics
	if err := monitor.Collect(); err != nil {
		return
	}

	// Write to store if available
	if w.store != nil {
		ctx := context.Background()
		metrics := monitor.GetSystemMetrics()

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
	w.RecordAPICallForUser("", model, success, latencyMs, inputTokens, outputTokens, cacheRead, cacheWrite, errorType)
}

// RecordAPICallForUser records an API call with all metrics including user tracking.
func (w *MetricsWriter) RecordAPICallForUser(userID, model string, success bool, latencyMs float64, inputTokens, outputTokens, cacheRead, cacheWrite int64, errorType string) {
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

	// Record token usage with user tracking
	w.tokenTracker.RecordTokenUsageForUser(userID, model, inputTokens, outputTokens, cacheRead, cacheWrite)

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
		if userID != "" {
			point.AddTag("user_id", userID)
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

// RecordCounter records a lightweight runtime counter with optional tags.
func (w *MetricsWriter) RecordCounter(name string, value int64, tags map[string]string) {
	if w == nil || value == 0 || strings.TrimSpace(name) == "" {
		return
	}
	if w.store == nil {
		return
	}
	ctx := context.Background()
	point := NewPoint(MeasurementCounters).
		AddTag(TagMetric, strings.TrimSpace(name)).
		AddField(FieldCount, value)
	for key, raw := range tags {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			point.AddTag(key, trimmed)
		}
	}
	w.store.Write(ctx, point)
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

// GetUserTokenUsage returns token usage for a specific user.
func (w *MetricsWriter) GetUserTokenUsage(userID string) *UserTokenUsage {
	return w.tokenTracker.GetUserTokenUsage(userID)
}

// GetAllUserUsage returns token usage for all users.
func (w *MetricsWriter) GetAllUserUsage() []UserTokenUsage {
	return w.tokenTracker.GetAllUserUsage()
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
	monitor := w.ensureSystemMonitorForAccess()
	if monitor == nil {
		return nil
	}
	return monitor.GetSystemMetrics()
}

// GetResourceHistory returns resource history.
func (w *MetricsWriter) GetResourceHistory() []ResourceHistory {
	monitor := w.ensureSystemMonitorForAccess()
	if monitor == nil {
		return nil
	}
	return monitor.GetResourceHistory()
}

// GetDB returns the underlying *sql.DB for the metrics SQLite store.
// Returns nil if no SQLite store is configured.
func (w *MetricsWriter) GetDB() *sql.DB {
	if w.sqliteStore == nil {
		return nil
	}
	return w.sqliteStore.db
}

// GetReadDB returns the underlying read *sql.DB for the metrics SQLite store.
// Returns nil if no SQLite store is configured.
func (w *MetricsWriter) GetReadDB() *sql.DB {
	if w.sqliteStore == nil {
		return nil
	}
	return w.sqliteStore.readDB
}

// GetCurrentProcessMetrics returns metrics for the current process.
func (w *MetricsWriter) GetCurrentProcessMetrics() (*ProcessMetrics, error) {
	monitor := w.ensureSystemMonitorForAccess()
	if monitor == nil {
		return nil, nil
	}
	return monitor.GetCurrentProcessMetrics()
}

func (w *MetricsWriter) ensureSystemMonitor() *SystemMonitor {
	if w == nil || w.config == nil || !w.config.EnableSystemMetrics {
		return nil
	}
	w.systemMonitorOnce.Do(func() {
		w.systemMonitor = NewSystemMonitor(w.config.MaxSamples, w.config.DiskPath)
	})
	return w.systemMonitor
}

func (w *MetricsWriter) startSystemMetricsLoopIfNeeded() {
	if w == nil || !w.started.Load() || w.config == nil || !w.config.EnableSystemMetrics {
		return
	}
	if w.ensureSystemMonitor() == nil {
		return
	}
	w.systemMetricsLoopOnce.Do(func() {
		w.wg.Add(1)
		go w.systemMetricsLoop()
	})
}

func (w *MetricsWriter) ensureSystemMonitorForAccess() *SystemMonitor {
	monitor := w.ensureSystemMonitor()
	if monitor == nil {
		return nil
	}
	if !monitor.hasSamples() {
		_ = monitor.Collect()
	}
	w.startSystemMetricsLoopIfNeeded()
	return monitor
}
