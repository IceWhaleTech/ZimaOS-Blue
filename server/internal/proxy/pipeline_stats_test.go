package proxy

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	_ "github.com/mattn/go-sqlite3"
)

func openPipelineStatsTestDB(t *testing.T, dbPath string, readOnly bool) *sql.DB {
	t.Helper()

	dsn := dbPath
	if readOnly {
		dsn = "file:" + dbPath + "?mode=ro"
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("sql.Open(%q): %v", dsn, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if !readOnly {
		if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
			db.Close()
			t.Fatalf("set WAL: %v", err)
		}
		if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
			db.Close()
			t.Fatalf("set busy_timeout: %v", err)
		}
	}

	return db
}

func TestPipelineStatsCollector_ReaderLoadsPersistedState(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pipeline_stats.db")
	writeDB := openPipelineStatsTestDB(t, dbPath, false)
	defer writeDB.Close()

	routingStats := NewRoutingStats()
	routingStats.Load(11, 22, 33)

	collector := NewPipelineStatsCollector(writeDB, routingStats)
	start := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
	collector.OnFailover(&providerpool.FailoverResult{
		RequestID:       "req-1",
		StartTime:       start,
		EndTime:         start.Add(2 * time.Second),
		TotalAttempts:   2,
		SuccessProvider: "provider-b",
		SuccessModel:    "model-b",
		FailedAttempts: []*providerpool.FailoverRecord{
			{
				ProviderID: "provider-a",
				Reason:     providerpool.FailoverReasonTimeout,
				Latency:    1500 * time.Millisecond,
			},
		},
	})

	smartMetrics := NewFailoverMetrics()
	smartMetrics.RecordError("provider-a", &ErrorClassification{Type: ErrorTypeRateLimited})
	smartMetrics.RecordFailover("provider-a", "provider-b", true)
	smartMetrics.RecordStreamAnomaly()
	collector.SetSmartFailoverMetrics(smartMetrics)

	failoverCfg := DefaultProxyConfig().Routing.Failover
	breakerHandler := NewFailoverHandler(&failoverCfg, nil)
	breakerHandler.LoadBreakerState(BreakerSnapshot{
		Name:            "provider-a",
		State:           "open",
		Failures:        3,
		Successes:       1,
		LastFailureTime: start.Add(-time.Minute),
		LastStateChange: start.Add(-30 * time.Second),
	})
	collector.SetFailoverHandler(breakerHandler)

	collector.flush()

	readerDB := openPipelineStatsTestDB(t, dbPath, true)
	defer readerDB.Close()

	writeDB2 := openPipelineStatsTestDB(t, dbPath, false)
	loadedRoutingStats := NewRoutingStats()
	loaded := NewPipelineStatsCollectorWithReadDB(writeDB2, readerDB, loadedRoutingStats)
	if loaded.readDB == nil {
		t.Fatal("expected read db to be initialized")
	}
	if loaded.readDB == loaded.db {
		t.Fatal("expected collector to use a separate read db")
	}
	if err := writeDB2.Close(); err != nil {
		t.Fatalf("close write db: %v", err)
	}

	loadedSmartMetrics := NewFailoverMetrics()
	loaded.SetSmartFailoverMetrics(loadedSmartMetrics)
	loaded.LoadSmartMetrics()

	loadedBreakerHandler := NewFailoverHandler(&failoverCfg, nil)
	loaded.SetFailoverHandler(loadedBreakerHandler)
	loaded.LoadBreakerState()

	snap := loaded.Snapshot()
	if snap.Routing.RoutedRequests != 11 || snap.Routing.TokensRouted != 22 {
		t.Fatalf("unexpected routing snapshot: %+v", snap.Routing)
	}
	if snap.Failover.Total != 1 || snap.Failover.Success != 1 || snap.Failover.Failure != 0 {
		t.Fatalf("unexpected failover snapshot: %+v", snap.Failover)
	}
	if snap.Failover.ByReason[string(providerpool.FailoverReasonTimeout)] != 1 {
		t.Fatalf("unexpected failover reasons: %+v", snap.Failover.ByReason)
	}
	if len(snap.Failover.Recent) != 1 {
		t.Fatalf("recent failovers len = %d, want 1", len(snap.Failover.Recent))
	}
	if snap.Failover.Recent[0].RequestID != "req-1" || !snap.Failover.Recent[0].Timestamp.Equal(start) {
		t.Fatalf("unexpected recent failover: %+v", snap.Failover.Recent[0])
	}

	if loadedSmartMetrics.FailoverTotal != 1 || loadedSmartMetrics.FailoverSuccess != 1 || loadedSmartMetrics.FailoverFailure != 0 {
		t.Fatalf("unexpected smart metrics totals: %+v", loadedSmartMetrics)
	}
	if got := atomic.LoadInt64(&loadedSmartMetrics.StreamAnomalies); got != 1 {
		t.Fatalf("stream anomalies = %d, want 1", got)
	}
	if loadedSmartMetrics.ProviderFailovers["provider-a"] != 1 {
		t.Fatalf("provider failovers = %+v, want provider-a=1", loadedSmartMetrics.ProviderFailovers)
	}
	if loadedSmartMetrics.ProviderErrors["provider-a"][ErrorTypeRateLimited] != 1 {
		t.Fatalf("provider errors = %+v, want provider-a/rate_limited=1", loadedSmartMetrics.ProviderErrors)
	}

	breakers := loadedBreakerHandler.SnapshotBreakers()
	if len(breakers) != 1 {
		t.Fatalf("breaker snapshots len = %d, want 1", len(breakers))
	}
	if breakers[0].Name != "provider-a" || breakers[0].State != "half-open" || breakers[0].Failures != 3 {
		t.Fatalf("unexpected breaker snapshot: %+v", breakers[0])
	}
}

func TestPipelineStatsCollector_TrimFailoverLogKeepsLatest1000(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "trim.db")
	writeDB := openPipelineStatsTestDB(t, dbPath, false)
	defer writeDB.Close()

	collector := NewPipelineStatsCollector(writeDB, NewRoutingStats())
	base := time.Date(2026, 3, 28, 13, 0, 0, 0, time.UTC)

	collector.eventMu.Lock()
	for i := 0; i < 1005; i++ {
		collector.eventBuf = append(collector.eventBuf, &failoverLogEntry{
			Timestamp:     base.Add(time.Duration(i) * time.Second),
			RequestID:     fmt.Sprintf("req-%04d", i),
			TotalAttempts: 1,
		})
	}
	collector.eventMu.Unlock()

	collector.flush()

	var count int
	if err := writeDB.QueryRow(`SELECT COUNT(1) FROM failover_log`).Scan(&count); err != nil {
		t.Fatalf("count failover_log: %v", err)
	}
	if count != 1000 {
		t.Fatalf("failover_log count = %d, want 1000", count)
	}

	var oldestRequestID string
	if err := writeDB.QueryRow(`SELECT request_id FROM failover_log ORDER BY id ASC LIMIT 1`).Scan(&oldestRequestID); err != nil {
		t.Fatalf("select oldest request_id: %v", err)
	}
	if oldestRequestID != "req-0005" {
		t.Fatalf("oldest request_id = %q, want %q", oldestRequestID, "req-0005")
	}

	recent := collector.loadRecentFailovers(1)
	if len(recent) != 1 || recent[0].RequestID != "req-1004" {
		t.Fatalf("unexpected recent failover after trim: %+v", recent)
	}
}
