package proxy

import (
	"database/sql"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// PipelineStatsCollector collects and batch-persists proxy pipeline statistics
// (routing, failover) asynchronously.
type PipelineStatsCollector struct {
	db *sql.DB

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
	c := &PipelineStatsCollector{
		db:           db,
		routingStats: routingStats,
		stopCh:       make(chan struct{}),
	}
	c.initSchema()
	c.loadStats()
	return c
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
	Routing  RoutingStatsSnapshot   `json:"routing"`
	Failover FailoverSnapshot       `json:"failover"`
}

// FailoverSnapshot contains failover statistics.
type FailoverSnapshot struct {
	Total    int64                        `json:"total"`
	Success  int64                        `json:"success"`
	Failure  int64                        `json:"failure"`
	ByReason map[string]int64             `json:"by_reason"`
	Recent   []*failoverLogEntry          `json:"recent"`
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

	var ft, fs, ff int64
	var fTimeout, fRateLimit, fAuthError, fModelNotFound, fAPIError, fCooldown, fUnknown int64
	var rr, tr, csm int64

	err := c.db.QueryRow(`
		SELECT COALESCE(failover_total,0), COALESCE(failover_success,0), COALESCE(failover_failure,0),
		       COALESCE(failover_timeout,0), COALESCE(failover_rate_limit,0), COALESCE(failover_auth_error,0),
		       COALESCE(failover_model_not_found,0), COALESCE(failover_api_error,0),
		       COALESCE(failover_cooldown,0), COALESCE(failover_unknown,0),
		       COALESCE(routed_requests,0), COALESCE(tokens_routed,0), COALESCE(cost_saved_micro,0)
		FROM pipeline_stats WHERE id = 1
	`).Scan(&ft, &fs, &ff, &fTimeout, &fRateLimit, &fAuthError, &fModelNotFound, &fAPIError, &fCooldown, &fUnknown, &rr, &tr, &csm)
	if err != nil {
		return // no data yet
	}

	atomic.StoreInt64(&c.failoverTotal, ft)
	atomic.StoreInt64(&c.failoverSuccess, fs)
	atomic.StoreInt64(&c.failoverFailure, ff)

	// Load reason counters
	reasonMap := map[providerpool.FailoverReason]int64{
		providerpool.FailoverReasonTimeout:       fTimeout,
		providerpool.FailoverReasonRateLimit:     fRateLimit,
		providerpool.FailoverReasonAuthError:     fAuthError,
		providerpool.FailoverReasonModelNotFound: fModelNotFound,
		providerpool.FailoverReasonAPIError:      fAPIError,
		providerpool.FailoverReasonCooldown:      fCooldown,
		providerpool.FailoverReasonUnknown:       fUnknown,
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
		c.routingStats.Load(rr, tr, csm)
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
	var ft, fs, ff, sa int64
	err := c.db.QueryRow(`
		SELECT COALESCE(failover_total,0), COALESCE(failover_success,0),
		       COALESCE(failover_failure,0), COALESCE(stream_anomalies,0)
		FROM smart_failover_metrics WHERE id = 1
	`).Scan(&ft, &fs, &ff, &sa)
	if err == nil {
		c.smartMetrics.mu.Lock()
		c.smartMetrics.FailoverTotal = ft
		c.smartMetrics.FailoverSuccess = fs
		c.smartMetrics.FailoverFailure = ff
		atomic.StoreInt64(&c.smartMetrics.StreamAnomalies, sa)
		c.smartMetrics.mu.Unlock()
	}

	// Load per-provider error/failover counts
	rows, err := c.db.Query(`SELECT provider, error_type, error_count, failover_count FROM smart_failover_provider_errors`)
	if err != nil {
		return
	}
	defer rows.Close()

	c.smartMetrics.mu.Lock()
	defer c.smartMetrics.mu.Unlock()
	for rows.Next() {
		var provider, errType string
		var errCount, foCount int64
		if err := rows.Scan(&provider, &errType, &errCount, &foCount); err != nil {
			continue
		}
		et := RetryableErrorType(errType)
		if errCount > 0 {
			c.smartMetrics.ErrorsByType[et] += errCount
			if _, ok := c.smartMetrics.ProviderErrors[provider]; !ok {
				c.smartMetrics.ProviderErrors[provider] = make(map[RetryableErrorType]int64)
			}
			c.smartMetrics.ProviderErrors[provider][et] += errCount
		}
		if foCount > 0 {
			c.smartMetrics.ProviderFailovers[provider] += foCount
		}
	}
}

// LoadBreakerState restores circuit breaker state from DB.
// Must be called after SetFailoverHandler.
func (c *PipelineStatsCollector) LoadBreakerState() {
	if c.db == nil || c.failoverHandler == nil {
		return
	}

	rows, err := c.db.Query(`SELECT name, state, failures, successes, last_failure_time, last_state_change FROM circuit_breaker_state`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var snap BreakerSnapshot
		var lastFailure, lastChange sql.NullString
		if err := rows.Scan(&snap.Name, &snap.State, &snap.Failures, &snap.Successes, &lastFailure, &lastChange); err != nil {
			continue
		}
		if lastFailure.Valid {
			snap.LastFailureTime, _ = time.Parse(time.RFC3339, lastFailure.String)
		}
		if lastChange.Valid {
			snap.LastStateChange, _ = time.Parse(time.RFC3339, lastChange.String)
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
	_, err = tx.Exec(`
		INSERT INTO pipeline_stats (id, failover_total, failover_success, failover_failure,
			failover_timeout, failover_rate_limit, failover_auth_error,
			failover_model_not_found, failover_api_error, failover_cooldown, failover_unknown,
			routed_requests, tokens_routed, cost_saved_micro, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			failover_total = excluded.failover_total,
			failover_success = excluded.failover_success,
			failover_failure = excluded.failover_failure,
			failover_timeout = excluded.failover_timeout,
			failover_rate_limit = excluded.failover_rate_limit,
			failover_auth_error = excluded.failover_auth_error,
			failover_model_not_found = excluded.failover_model_not_found,
			failover_api_error = excluded.failover_api_error,
			failover_cooldown = excluded.failover_cooldown,
			failover_unknown = excluded.failover_unknown,
			routed_requests = excluded.routed_requests,
			tokens_routed = excluded.tokens_routed,
			cost_saved_micro = excluded.cost_saved_micro,
			updated_at = excluded.updated_at
	`, ft, fs, ff,
		reasons[string(providerpool.FailoverReasonTimeout)],
		reasons[string(providerpool.FailoverReasonRateLimit)],
		reasons[string(providerpool.FailoverReasonAuthError)],
		reasons[string(providerpool.FailoverReasonModelNotFound)],
		reasons[string(providerpool.FailoverReasonAPIError)],
		reasons[string(providerpool.FailoverReasonCooldown)],
		reasons[string(providerpool.FailoverReasonUnknown)],
		rr, tr, csm, now.Format(time.RFC3339))

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
		stmt, err := tx.Prepare(`
			INSERT INTO failover_log (timestamp, request_id, total_attempts, success_provider,
				success_model, failed_providers, final_error, duration_ms)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err == nil {
			for _, e := range events {
				stmt.Exec(e.Timestamp.Format(time.RFC3339), e.RequestID, e.TotalAttempts,
					e.SuccessProvider, e.SuccessModel, e.FailedProviders, e.FinalError, e.DurationMs)
			}
			stmt.Close()
		}

		// Trim to last 1000 rows
		tx.Exec(`DELETE FROM failover_log WHERE id NOT IN (SELECT id FROM failover_log ORDER BY id DESC LIMIT 1000)`)
	}

	// Flush smart failover metrics
	c.flushSmartMetrics(tx, now)

	// Flush circuit breaker state
	c.flushBreakerState(tx, now)

	if err := tx.Commit(); err != nil {
		slog.Warn("[pipeline-stats] failed to commit", "error", err)
	}
}

func (c *PipelineStatsCollector) loadRecentFailovers(limit int) []*failoverLogEntry {
	if c.db == nil {
		return nil
	}

	rows, err := c.db.Query(`
		SELECT timestamp, request_id, total_attempts, success_provider,
		       success_model, failed_providers, final_error, duration_ms
		FROM failover_log ORDER BY id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []*failoverLogEntry
	for rows.Next() {
		var e failoverLogEntry
		var ts string
		if err := rows.Scan(&ts, &e.RequestID, &e.TotalAttempts, &e.SuccessProvider,
			&e.SuccessModel, &e.FailedProviders, &e.FinalError, &e.DurationMs); err != nil {
			continue
		}
		e.Timestamp, _ = time.Parse(time.RFC3339, ts)
		result = append(result, &e)
	}
	return result
}

// flushSmartMetrics persists SmartFailoverHandler.FailoverMetrics to DB.
func (c *PipelineStatsCollector) flushSmartMetrics(tx *sql.Tx, now time.Time) {
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
	tx.Exec(`
		INSERT INTO smart_failover_metrics (id, failover_total, failover_success, failover_failure, stream_anomalies, updated_at)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			failover_total = excluded.failover_total,
			failover_success = excluded.failover_success,
			failover_failure = excluded.failover_failure,
			stream_anomalies = excluded.stream_anomalies,
			updated_at = excluded.updated_at
	`, ft, fs, ff, sa, now.Format(time.RFC3339))

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
		stmt, err := tx.Prepare(`
			INSERT INTO smart_failover_provider_errors (provider, error_type, error_count, failover_count)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(provider, error_type) DO UPDATE SET
				error_count = excluded.error_count,
				failover_count = excluded.failover_count
		`)
		if err == nil {
			for _, pe := range provErrors {
				stmt.Exec(pe.provider, string(pe.errType), pe.count, foByProvider[pe.provider])
			}
			stmt.Close()
		}
	}
}

// flushBreakerState persists circuit breaker state to DB.
func (c *PipelineStatsCollector) flushBreakerState(tx *sql.Tx, now time.Time) {
	if c.failoverHandler == nil {
		return
	}

	snaps := c.failoverHandler.SnapshotBreakers()
	if len(snaps) == 0 {
		return
	}

	stmt, err := tx.Prepare(`
		INSERT INTO circuit_breaker_state (name, state, failures, successes, last_failure_time, last_state_change, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			state = excluded.state,
			failures = excluded.failures,
			successes = excluded.successes,
			last_failure_time = excluded.last_failure_time,
			last_state_change = excluded.last_state_change,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return
	}
	defer stmt.Close()

	for _, snap := range snaps {
		var lastFailure, lastChange *string
		if !snap.LastFailureTime.IsZero() {
			s := snap.LastFailureTime.Format(time.RFC3339)
			lastFailure = &s
		}
		if !snap.LastStateChange.IsZero() {
			s := snap.LastStateChange.Format(time.RFC3339)
			lastChange = &s
		}
		stmt.Exec(snap.Name, snap.State, snap.Failures, snap.Successes,
			lastFailure, lastChange, now.Format(time.RFC3339))
	}
}
