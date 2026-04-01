package proxy

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
)

// PipelineStatsCollector collects and batch-persists proxy pipeline statistics
// (routing, failover) asynchronously.
type PipelineStatsCollector struct {
	db     *sql.DB
	readDB *sql.DB

	// Failover aggregates (atomic)
	failoverTotal   int64
	failoverSuccess int64
	failoverFailure int64
	// Per-reason counters (pre-allocated, indexed by reason string)
	reasonCounters sync.Map // FailoverReason → *int64

	// References to existing stat holders
	routingStats *RoutingStats

	// Smart failover metrics (per-provider error classification)
	smartMetrics    *FailoverMetrics
	failoverHandler *FailoverHandler

	// Failover event buffer for log persistence
	eventMu  sync.Mutex
	eventBuf []*failoverLogEntry

	// Lifecycle
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

type failoverLogEntry struct {
	Timestamp       time.Time `json:"timestamp"`
	RequestID       string    `json:"request_id"`
	TotalAttempts   int       `json:"total_attempts"`
	SuccessProvider string    `json:"success_provider"`
	SuccessModel    string    `json:"success_model"`
	FailedProviders string    `json:"failed_providers"` // JSON array
	FinalError      string    `json:"final_error"`
	DurationMs      int64     `json:"duration_ms"`
}

// NewPipelineStatsCollector creates a new collector. Call Start() to begin background persistence.
func NewPipelineStatsCollector(db *sql.DB, routingStats *RoutingStats) *PipelineStatsCollector {
	return NewPipelineStatsCollectorWithReadDB(db, db, routingStats)
}

// NewPipelineStatsCollectorWithReadDB creates a new collector with separate
// write and read database handles.
func NewPipelineStatsCollectorWithReadDB(writeDB, readDB *sql.DB, routingStats *RoutingStats) *PipelineStatsCollector {
	if readDB == nil {
		readDB = writeDB
	}
	c := &PipelineStatsCollector{
		db:           writeDB,
		readDB:       readDB,
		routingStats: routingStats,
		stopCh:       make(chan struct{}),
	}
	c.initSchema()
	c.loadStats()
	return c
}

func (c *PipelineStatsCollector) reader() *sql.DB {
	if c != nil && c.readDB != nil {
		return c.readDB
	}
	if c == nil {
		return nil
	}
	return c.db
}

func (c *PipelineStatsCollector) table(ctx context.Context, name string) *z.ZormTable {
	return z.TableContext(ctx, c.db, name)
}

func (c *PipelineStatsCollector) readTable(ctx context.Context, name string) *z.ZormTable {
	return z.TableContext(ctx, c.reader(), name)
}

type pipelineStatsRow struct {
	FailoverTotal         int64 `json:"failover_total" zorm:"failover_total"`
	FailoverSuccess       int64 `json:"failover_success" zorm:"failover_success"`
	FailoverFailure       int64 `json:"failover_failure" zorm:"failover_failure"`
	FailoverTimeout       int64 `json:"failover_timeout" zorm:"failover_timeout"`
	FailoverRateLimit     int64 `json:"failover_rate_limit" zorm:"failover_rate_limit"`
	FailoverAuthError     int64 `json:"failover_auth_error" zorm:"failover_auth_error"`
	FailoverModelNotFound int64 `json:"failover_model_not_found" zorm:"failover_model_not_found"`
	FailoverAPIError      int64 `json:"failover_api_error" zorm:"failover_api_error"`
	FailoverCooldown      int64 `json:"failover_cooldown" zorm:"failover_cooldown"`
	FailoverUnknown       int64 `json:"failover_unknown" zorm:"failover_unknown"`
	RoutedRequests        int64 `json:"routed_requests" zorm:"routed_requests"`
	TokensRouted          int64 `json:"tokens_routed" zorm:"tokens_routed"`
	CostSavedMicro        int64 `json:"cost_saved_micro" zorm:"cost_saved_micro"`
}

type smartFailoverMetricsRow struct {
	FailoverTotal   int64 `json:"failover_total" zorm:"failover_total"`
	FailoverSuccess int64 `json:"failover_success" zorm:"failover_success"`
	FailoverFailure int64 `json:"failover_failure" zorm:"failover_failure"`
	StreamAnomalies int64 `json:"stream_anomalies" zorm:"stream_anomalies"`
}

type smartFailoverProviderErrorRow struct {
	Provider      string `json:"provider" zorm:"provider"`
	ErrorType     string `json:"error_type" zorm:"error_type"`
	ErrorCount    int64  `json:"error_count" zorm:"error_count"`
	FailoverCount int64  `json:"failover_count" zorm:"failover_count"`
}

type circuitBreakerStateRow struct {
	Name            string  `json:"name" zorm:"name"`
	State           string  `json:"state" zorm:"state"`
	Failures        int     `json:"failures" zorm:"failures"`
	Successes       int     `json:"successes" zorm:"successes"`
	LastFailureTime *string `json:"last_failure_time" zorm:"last_failure_time"`
	LastStateChange *string `json:"last_state_change" zorm:"last_state_change"`
}

type failoverLogRow struct {
	ID              int64  `json:"id" zorm:"id,auto_incr"`
	Timestamp       string `json:"timestamp" zorm:"timestamp"`
	RequestID       string `json:"request_id" zorm:"request_id"`
	TotalAttempts   int    `json:"total_attempts" zorm:"total_attempts"`
	SuccessProvider string `json:"success_provider" zorm:"success_provider"`
	SuccessModel    string `json:"success_model" zorm:"success_model"`
	FailedProviders string `json:"failed_providers" zorm:"failed_providers"`
	FinalError      string `json:"final_error" zorm:"final_error"`
	DurationMs      int64  `json:"duration_ms" zorm:"duration_ms"`
}

type failoverLogIDRow struct {
	ID int64 `json:"id" zorm:"id"`
}

func formatPipelineStatsTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func parsePipelineStatsTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func nullablePipelineStatsTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return formatPipelineStatsTime(t)
}

// Start begins the background flush goroutine.
func (c *PipelineStatsCollector) Start() {
	c.wg.Add(1)
	go c.flushLoop()
}

// Stop flushes remaining data and stops the background goroutine.
func (c *PipelineStatsCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	c.wg.Wait()
}

// Close implements io.Closer for lifecycle shutdown hooks.
func (c *PipelineStatsCollector) Close() error {
	c.Stop()
	return nil
}

// SetSmartFailoverMetrics sets the smart failover metrics reference for persistence.
func (c *PipelineStatsCollector) SetSmartFailoverMetrics(m *FailoverMetrics) {
	c.smartMetrics = m
}

// SetFailoverHandler sets the failover handler reference for circuit breaker persistence.
func (c *PipelineStatsCollector) SetFailoverHandler(fh *FailoverHandler) {
	c.failoverHandler = fh
}

// --- Failover callback (wired to Router.SetFailoverCallback) ---

func (c *PipelineStatsCollector) OnFailover(result *providerpool.FailoverResult) {
	if result == nil {
		return
	}

	// Update atomic counters
	atomic.AddInt64(&c.failoverTotal, 1)
	if result.SuccessProvider != "" {
		atomic.AddInt64(&c.failoverSuccess, 1)
	} else {
		atomic.AddInt64(&c.failoverFailure, 1)
	}

	// Count reasons from failed attempts
	for _, attempt := range result.FailedAttempts {
		c.incrementReason(attempt.Reason)
	}

	// Buffer the event for log persistence
	entry := &failoverLogEntry{
		Timestamp:       result.StartTime,
		RequestID:       result.RequestID,
		TotalAttempts:   result.TotalAttempts,
		SuccessProvider: result.SuccessProvider,
		SuccessModel:    result.SuccessModel,
		FinalError:      result.FinalError,
		DurationMs:      result.EndTime.Sub(result.StartTime).Milliseconds(),
	}

	// Serialize failed providers
	if len(result.FailedAttempts) > 0 {
		type failedEntry struct {
			ProviderID string `json:"provider_id"`
			Reason     string `json:"reason"`
			LatencyMs  int64  `json:"latency_ms"`
		}
		entries := make([]failedEntry, len(result.FailedAttempts))
		for i, a := range result.FailedAttempts {
			entries[i] = failedEntry{
				ProviderID: a.ProviderID,
				Reason:     string(a.Reason),
				LatencyMs:  a.Latency.Milliseconds(),
			}
		}
		if b, err := json.Marshal(entries); err == nil {
			entry.FailedProviders = string(b)
		}
	}

	c.eventMu.Lock()
	c.eventBuf = append(c.eventBuf, entry)
	c.eventMu.Unlock()
}

// --- Snapshot for API ---

// PipelineSnapshot is the combined stats snapshot returned by the API.
type PipelineSnapshot struct {
	Routing  RoutingStatsSnapshot `json:"routing"`
	Failover FailoverSnapshot     `json:"failover"`
}

// FailoverSnapshot contains failover statistics.
type FailoverSnapshot struct {
	Total    int64               `json:"total"`
	Success  int64               `json:"success"`
	Failure  int64               `json:"failure"`
	ByReason map[string]int64    `json:"by_reason"`
	Recent   []*failoverLogEntry `json:"recent"`
}

// Snapshot returns a combined pipeline stats snapshot.
func (c *PipelineStatsCollector) Snapshot() PipelineSnapshot {
	snap := PipelineSnapshot{}

	// Routing stats
	if c.routingStats != nil {
		snap.Routing = c.routingStats.Snapshot()
	}

	// Failover stats
	snap.Failover = FailoverSnapshot{
		Total:    atomic.LoadInt64(&c.failoverTotal),
		Success:  atomic.LoadInt64(&c.failoverSuccess),
		Failure:  atomic.LoadInt64(&c.failoverFailure),
		ByReason: c.reasonSnapshot(),
	}

	// Recent failover events from DB
	snap.Failover.Recent = c.loadRecentFailovers(20)

	return snap
}

// --- Internal ---

func (c *PipelineStatsCollector) incrementReason(reason providerpool.FailoverReason) {
	val, _ := c.reasonCounters.LoadOrStore(reason, new(int64))
	atomic.AddInt64(val.(*int64), 1)
}

func (c *PipelineStatsCollector) reasonSnapshot() map[string]int64 {
	result := make(map[string]int64)
	c.reasonCounters.Range(func(key, value interface{}) bool {
		reason := key.(providerpool.FailoverReason)
		count := atomic.LoadInt64(value.(*int64))
		if count > 0 {
			result[string(reason)] = count
		}
		return true
	})
	return result
}

func (c *PipelineStatsCollector) initSchema() {
	if c.db == nil {
		return
	}

	// Pipeline stats (singleton)
	c.db.Exec(`
		CREATE TABLE IF NOT EXISTS pipeline_stats (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			failover_total INTEGER DEFAULT 0,
			failover_success INTEGER DEFAULT 0,
			failover_failure INTEGER DEFAULT 0,
			failover_timeout INTEGER DEFAULT 0,
			failover_rate_limit INTEGER DEFAULT 0,
			failover_auth_error INTEGER DEFAULT 0,
			failover_model_not_found INTEGER DEFAULT 0,
			failover_api_error INTEGER DEFAULT 0,
			failover_cooldown INTEGER DEFAULT 0,
			failover_unknown INTEGER DEFAULT 0,
			routed_requests INTEGER DEFAULT 0,
			tokens_routed INTEGER DEFAULT 0,
			cost_saved_micro INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)

	// Failover event log (ring buffer)
	c.db.Exec(`
		CREATE TABLE IF NOT EXISTS failover_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			request_id TEXT,
			total_attempts INTEGER,
			success_provider TEXT,
			success_model TEXT,
			failed_providers TEXT,
			final_error TEXT,
			duration_ms INTEGER
		)
	`)

	// Smart failover metrics: per-provider error classification
	c.db.Exec(`
		CREATE TABLE IF NOT EXISTS smart_failover_metrics (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			failover_total INTEGER DEFAULT 0,
			failover_success INTEGER DEFAULT 0,
			failover_failure INTEGER DEFAULT 0,
			stream_anomalies INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	c.db.Exec(`
		CREATE TABLE IF NOT EXISTS smart_failover_provider_errors (
			provider TEXT NOT NULL,
			error_type TEXT NOT NULL,
			error_count INTEGER DEFAULT 0,
			failover_count INTEGER DEFAULT 0,
			PRIMARY KEY (provider, error_type)
		)
	`)

	// Circuit breaker state
	c.db.Exec(`
		CREATE TABLE IF NOT EXISTS circuit_breaker_state (
			name TEXT PRIMARY KEY,
			state TEXT NOT NULL DEFAULT 'closed',
			failures INTEGER DEFAULT 0,
			successes INTEGER DEFAULT 0,
			last_failure_time DATETIME,
			last_state_change DATETIME,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
}

func (c *PipelineStatsCollector) loadStats() {
	if c.db == nil {
		return
	}

	ctx := context.Background()
	var rows []pipelineStatsRow
	if _, err := c.readTable(ctx, "pipeline_stats").Select(&rows,
		z.Fields(
			"failover_total", "failover_success", "failover_failure",
			"failover_timeout", "failover_rate_limit", "failover_auth_error",
			"failover_model_not_found", "failover_api_error",
			"failover_cooldown", "failover_unknown",
			"routed_requests", "tokens_routed", "cost_saved_micro",
		),
		z.Where(z.Eq("id", 1)),
		z.Limit(1),
	); err != nil || len(rows) == 0 {
		return
	}
	row := rows[0]

	atomic.StoreInt64(&c.failoverTotal, row.FailoverTotal)
	atomic.StoreInt64(&c.failoverSuccess, row.FailoverSuccess)
	atomic.StoreInt64(&c.failoverFailure, row.FailoverFailure)

	// Load reason counters
	reasonMap := map[providerpool.FailoverReason]int64{
		providerpool.FailoverReasonTimeout:       row.FailoverTimeout,
		providerpool.FailoverReasonRateLimit:     row.FailoverRateLimit,
		providerpool.FailoverReasonAuthError:     row.FailoverAuthError,
		providerpool.FailoverReasonModelNotFound: row.FailoverModelNotFound,
		providerpool.FailoverReasonAPIError:      row.FailoverAPIError,
		providerpool.FailoverReasonCooldown:      row.FailoverCooldown,
		providerpool.FailoverReasonUnknown:       row.FailoverUnknown,
	}
	for reason, count := range reasonMap {
		if count > 0 {
			p := new(int64)
			*p = count
			c.reasonCounters.Store(reason, p)
		}
	}

	// Load routing stats into the existing RoutingStats
	if c.routingStats != nil {
		c.routingStats.Load(row.RoutedRequests, row.TokensRouted, row.CostSavedMicro)
	}

	// Load smart failover metrics (deferred — smartMetrics may not be set yet at init time)
	// Actual loading happens in loadSmartMetrics(), called after SetSmartFailoverMetrics.
}

// LoadSmartMetrics restores smart failover metrics from DB.
// Must be called after SetSmartFailoverMetrics.
func (c *PipelineStatsCollector) LoadSmartMetrics() {
	if c.db == nil || c.smartMetrics == nil {
		return
	}

	// Load global counters
	ctx := context.Background()
	var metricRows []smartFailoverMetricsRow
	if _, err := c.readTable(ctx, "smart_failover_metrics").Select(&metricRows,
		z.Fields("failover_total", "failover_success", "failover_failure", "stream_anomalies"),
		z.Where(z.Eq("id", 1)),
		z.Limit(1),
	); err == nil && len(metricRows) > 0 {
		row := metricRows[0]
		c.smartMetrics.mu.Lock()
		c.smartMetrics.FailoverTotal = row.FailoverTotal
		c.smartMetrics.FailoverSuccess = row.FailoverSuccess
		c.smartMetrics.FailoverFailure = row.FailoverFailure
		atomic.StoreInt64(&c.smartMetrics.StreamAnomalies, row.StreamAnomalies)
		c.smartMetrics.mu.Unlock()
	}

	// Load per-provider error/failover counts
	var rows []smartFailoverProviderErrorRow
	if _, err := c.readTable(ctx, "smart_failover_provider_errors").Select(&rows,
		z.Fields("provider", "error_type", "error_count", "failover_count"),
	); err != nil {
		return
	}

	c.smartMetrics.mu.Lock()
	defer c.smartMetrics.mu.Unlock()
	for i := range rows {
		et := RetryableErrorType(rows[i].ErrorType)
		if rows[i].ErrorCount > 0 {
			c.smartMetrics.ErrorsByType[et] += rows[i].ErrorCount
			if _, ok := c.smartMetrics.ProviderErrors[rows[i].Provider]; !ok {
				c.smartMetrics.ProviderErrors[rows[i].Provider] = make(map[RetryableErrorType]int64)
			}
			c.smartMetrics.ProviderErrors[rows[i].Provider][et] += rows[i].ErrorCount
		}
		if rows[i].FailoverCount > 0 {
			c.smartMetrics.ProviderFailovers[rows[i].Provider] += rows[i].FailoverCount
		}
	}
}

// LoadBreakerState restores circuit breaker state from DB.
// Must be called after SetFailoverHandler.
func (c *PipelineStatsCollector) LoadBreakerState() {
	if c.db == nil || c.failoverHandler == nil {
		return
	}

	ctx := context.Background()
	var rows []circuitBreakerStateRow
	if _, err := c.readTable(ctx, "circuit_breaker_state").Select(&rows,
		z.Fields("name", "state", "failures", "successes", "last_failure_time", "last_state_change"),
	); err != nil {
		return
	}

	for i := range rows {
		var snap BreakerSnapshot
		snap.Name = rows[i].Name
		snap.State = rows[i].State
		snap.Failures = rows[i].Failures
		snap.Successes = rows[i].Successes
		if rows[i].LastFailureTime != nil {
			snap.LastFailureTime = parsePipelineStatsTime(*rows[i].LastFailureTime)
		}
		if rows[i].LastStateChange != nil {
			snap.LastStateChange = parsePipelineStatsTime(*rows[i].LastStateChange)
		}
		c.failoverHandler.LoadBreakerState(snap)
		slog.Info("[pipeline-stats] restored circuit breaker", "name", snap.Name, "state", snap.State, "failures", snap.Failures)
	}
}

func (c *PipelineStatsCollector) flushLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.flush()
		case <-c.stopCh:
			c.flush() // final flush
			return
		}
	}
}

func (c *PipelineStatsCollector) flush() {
	if c.db == nil {
		return
	}

	ctx := context.Background()
	now := timeutil.NowTime()

	// Snapshot failover counters
	ft := atomic.LoadInt64(&c.failoverTotal)
	fs := atomic.LoadInt64(&c.failoverSuccess)
	ff := atomic.LoadInt64(&c.failoverFailure)
	reasons := c.reasonSnapshot()

	// Snapshot routing stats
	var rr, tr, csm int64
	if c.routingStats != nil {
		snap := c.routingStats.Snapshot()
		rr = snap.RoutedRequests
		tr = snap.TokensRouted
		csm = int64(snap.CostSavedUSD * 1_000_000)
	}

	// Single transaction for all writes
	tx, err := c.db.Begin()
	if err != nil {
		slog.Warn("[pipeline-stats] failed to begin tx", "error", err)
		return
	}
	defer tx.Rollback()

	// UPSERT pipeline_stats
	_, err = z.TableContext(ctx, tx, "pipeline_stats").Insert(
		z.V{
			"id":                       1,
			"failover_total":           ft,
			"failover_success":         fs,
			"failover_failure":         ff,
			"failover_timeout":         reasons[string(providerpool.FailoverReasonTimeout)],
			"failover_rate_limit":      reasons[string(providerpool.FailoverReasonRateLimit)],
			"failover_auth_error":      reasons[string(providerpool.FailoverReasonAuthError)],
			"failover_model_not_found": reasons[string(providerpool.FailoverReasonModelNotFound)],
			"failover_api_error":       reasons[string(providerpool.FailoverReasonAPIError)],
			"failover_cooldown":        reasons[string(providerpool.FailoverReasonCooldown)],
			"failover_unknown":         reasons[string(providerpool.FailoverReasonUnknown)],
			"routed_requests":          rr,
			"tokens_routed":            tr,
			"cost_saved_micro":         csm,
			"updated_at":               formatPipelineStatsTime(now),
		},
		z.OnConflictDoUpdateSet(
			[]string{"id"},
			[]string{
				"failover_total", "failover_success", "failover_failure",
				"failover_timeout", "failover_rate_limit", "failover_auth_error",
				"failover_model_not_found", "failover_api_error", "failover_cooldown", "failover_unknown",
				"routed_requests", "tokens_routed", "cost_saved_micro", "updated_at",
			},
		),
	)
	if err != nil {
		slog.Warn("[pipeline-stats] failed to upsert pipeline_stats", "error", err)
		return
	}

	// Flush buffered failover events
	c.eventMu.Lock()
	events := c.eventBuf
	c.eventBuf = nil
	c.eventMu.Unlock()

	if len(events) > 0 {
		logTable := z.TableContext(ctx, tx, "failover_log")
		for _, e := range events {
			if _, err := logTable.Insert(z.V{
				"timestamp":        formatPipelineStatsTime(e.Timestamp),
				"request_id":       e.RequestID,
				"total_attempts":   e.TotalAttempts,
				"success_provider": e.SuccessProvider,
				"success_model":    e.SuccessModel,
				"failed_providers": e.FailedProviders,
				"final_error":      e.FinalError,
				"duration_ms":      e.DurationMs,
			}); err != nil {
				slog.Warn("[pipeline-stats] failed to insert failover_log", "error", err, "request_id", e.RequestID)
			}
		}

		// Trim to last 1000 rows
		var total int64
		if _, err := logTable.Select(&total, z.Fields("count(1)")); err == nil && total > 1000 {
			var staleRows []failoverLogIDRow
			if _, err := logTable.Select(&staleRows,
				z.Fields("id"),
				z.OrderBy("id ASC"),
				z.Limit(int(total-1000)),
			); err == nil && len(staleRows) > 0 {
				ids := make([]int64, 0, len(staleRows))
				for i := range staleRows {
					ids = append(ids, staleRows[i].ID)
				}
				if _, err := logTable.Delete(z.Where(z.In("id", ids))); err != nil {
					slog.Warn("[pipeline-stats] failed to trim failover_log", "error", err)
				}
			}
		}
	}

	// Flush smart failover metrics
	c.flushSmartMetrics(ctx, tx, now)

	// Flush circuit breaker state
	c.flushBreakerState(ctx, tx, now)

	if err := tx.Commit(); err != nil {
		slog.Warn("[pipeline-stats] failed to commit", "error", err)
	}
}

func (c *PipelineStatsCollector) loadRecentFailovers(limit int) []*failoverLogEntry {
	if c.db == nil {
		return nil
	}

	ctx := context.Background()
	var rows []failoverLogRow
	if _, err := c.readTable(ctx, "failover_log").Select(&rows,
		z.Fields("id", "timestamp", "request_id", "total_attempts", "success_provider", "success_model", "failed_providers", "final_error", "duration_ms"),
		z.OrderBy("id DESC"),
		z.Limit(limit),
	); err != nil {
		return nil
	}

	result := make([]*failoverLogEntry, 0, len(rows))
	for i := range rows {
		e := &failoverLogEntry{
			Timestamp:       parsePipelineStatsTime(rows[i].Timestamp),
			RequestID:       rows[i].RequestID,
			TotalAttempts:   rows[i].TotalAttempts,
			SuccessProvider: rows[i].SuccessProvider,
			SuccessModel:    rows[i].SuccessModel,
			FailedProviders: rows[i].FailedProviders,
			FinalError:      rows[i].FinalError,
			DurationMs:      rows[i].DurationMs,
		}
		result = append(result, e)
	}
	return result
}

// flushSmartMetrics persists SmartFailoverHandler.FailoverMetrics to DB.
func (c *PipelineStatsCollector) flushSmartMetrics(ctx context.Context, tx *sql.Tx, now time.Time) {
	if c.smartMetrics == nil {
		return
	}

	c.smartMetrics.mu.RLock()
	ft := c.smartMetrics.FailoverTotal
	fs := c.smartMetrics.FailoverSuccess
	ff := c.smartMetrics.FailoverFailure
	sa := atomic.LoadInt64(&c.smartMetrics.StreamAnomalies)

	// Copy provider-level data under lock
	type provErr struct {
		provider string
		errType  RetryableErrorType
		count    int64
	}
	var provErrors []provErr
	for provider, errMap := range c.smartMetrics.ProviderErrors {
		for et, count := range errMap {
			provErrors = append(provErrors, provErr{provider, et, count})
		}
	}
	provFailovers := make(map[string]int64, len(c.smartMetrics.ProviderFailovers))
	for k, v := range c.smartMetrics.ProviderFailovers {
		provFailovers[k] = v
	}
	c.smartMetrics.mu.RUnlock()

	// Upsert global counters
	_, _ = z.TableContext(ctx, tx, "smart_failover_metrics").Insert(
		z.V{
			"id":               1,
			"failover_total":   ft,
			"failover_success": fs,
			"failover_failure": ff,
			"stream_anomalies": sa,
			"updated_at":       formatPipelineStatsTime(now),
		},
		z.OnConflictDoUpdateSet(
			[]string{"id"},
			[]string{"failover_total", "failover_success", "failover_failure", "stream_anomalies", "updated_at"},
		),
	)

	// Upsert per-provider error counts
	// Merge failover counts into the same rows
	foByProvider := make(map[string]int64)
	for _, pe := range provErrors {
		foByProvider[pe.provider] = provFailovers[pe.provider]
	}
	// Also add providers that have failovers but no errors
	for provider, count := range provFailovers {
		if _, exists := foByProvider[provider]; !exists {
			provErrors = append(provErrors, provErr{provider, "", 0})
			foByProvider[provider] = count
		}
	}

	if len(provErrors) > 0 {
		table := z.TableContext(ctx, tx, "smart_failover_provider_errors")
		for _, pe := range provErrors {
			_, _ = table.Insert(
				z.V{
					"provider":       pe.provider,
					"error_type":     string(pe.errType),
					"error_count":    pe.count,
					"failover_count": foByProvider[pe.provider],
				},
				z.OnConflictDoUpdateSet(
					[]string{"provider", "error_type"},
					[]string{"error_count", "failover_count"},
				),
			)
		}
	}
}

// flushBreakerState persists circuit breaker state to DB.
func (c *PipelineStatsCollector) flushBreakerState(ctx context.Context, tx *sql.Tx, now time.Time) {
	if c.failoverHandler == nil {
		return
	}

	snaps := c.failoverHandler.SnapshotBreakers()
	if len(snaps) == 0 {
		return
	}

	table := z.TableContext(ctx, tx, "circuit_breaker_state")
	for _, snap := range snaps {
		_, _ = table.Insert(
			z.V{
				"name":              snap.Name,
				"state":             snap.State,
				"failures":          snap.Failures,
				"successes":         snap.Successes,
				"last_failure_time": nullablePipelineStatsTime(snap.LastFailureTime),
				"last_state_change": nullablePipelineStatsTime(snap.LastStateChange),
				"updated_at":        formatPipelineStatsTime(now),
			},
			z.OnConflictDoUpdateSet(
				[]string{"name"},
				[]string{"state", "failures", "successes", "last_failure_time", "last_state_change", "updated_at"},
			),
		)
	}
}
