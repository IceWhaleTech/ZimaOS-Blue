package metrics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore implements MetricsStore using SQLite for persistence.
type SQLiteStore struct {
	db     *sql.DB
	readDB *sql.DB
	dbPath string
	ownsDB bool
	mu     sync.RWMutex
}

func newStoreWithReadDB(writeDB, readDB *sql.DB, dbPath string, ownsDB bool) (*SQLiteStore, error) {
	if readDB == nil {
		readDB = writeDB
	}
	store := &SQLiteStore{
		db:     writeDB,
		readDB: readDB,
		dbPath: dbPath,
		ownsDB: ownsDB,
	}

	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Release unused memory after schema init
	writeDB.Exec("PRAGMA shrink_memory")
	return store, nil
}

// NewSQLiteStore creates a new SQLite-based metrics store.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(dbPath, dbPath, func(db *sql.DB) error {
		// Connection pool limits
		db.SetMaxOpenConns(2)
		db.SetMaxIdleConns(1)

		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("set metrics journal mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("set metrics busy timeout: %w", err)
		}
		if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
			return fmt.Errorf("set metrics synchronous mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA wal_autocheckpoint=1000"); err != nil {
			return fmt.Errorf("set metrics wal autocheckpoint: %w", err)
		}
		if runtime.GOOS == "darwin" {
			if _, err := db.Exec("PRAGMA fullfsync=ON"); err != nil {
				return fmt.Errorf("set metrics fullfsync: %w", err)
			}
			if _, err := db.Exec("PRAGMA checkpoint_fullfsync=ON"); err != nil {
				return fmt.Errorf("set metrics checkpoint_fullfsync: %w", err)
			}
		}
		db.Exec("PRAGMA cache_size=-500") // ~512KB page cache for lower idle memory
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open metrics database: %w", err)
	}

	readDB, readErr := openMetricsReaderDB(dbPath)
	if readErr != nil || readDB == nil {
		readDB = db
	}

	store, err := newStoreWithReadDB(db, readDB, dbPath, true)
	if err != nil {
		if readDB != nil && readDB != db {
			_ = readDB.Close()
		}
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// NewSQLiteStoreWithDB reuses an existing SQLite database for metrics persistence.
func NewSQLiteStoreWithDB(db *sql.DB) (*SQLiteStore, error) {
	return NewSQLiteStoreWithReadDB(db, db)
}

// NewSQLiteStoreWithReadDB reuses existing SQLite write/read handles for
// metrics persistence.
func NewSQLiteStoreWithReadDB(writeDB, readDB *sql.DB) (*SQLiteStore, error) {
	if writeDB == nil {
		return nil, fmt.Errorf("metrics db is nil")
	}
	return newStoreWithReadDB(writeDB, readDB, "", false)
}

func (s *SQLiteStore) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *SQLiteStore) table(ctx context.Context, name string) *z.ZormTable {
	return z.TableContext(ctx, s.db, name)
}

func (s *SQLiteStore) readTable(ctx context.Context, name string) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), name)
}

func openMetricsReaderDB(dbPath string) (*sql.DB, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" || dbPath == ":memory:" {
		return nil, nil
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := dbutil.OpenSQLiteWithRecovery(dsn, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(2)
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("set metrics reader busy timeout: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

type metricsPointRow struct {
	Measurement string `json:"measurement" zorm:"measurement"`
	Tags        string `json:"tags" zorm:"tags"`
	Fields      string `json:"fields" zorm:"fields"`
	Timestamp   string `json:"timestamp" zorm:"timestamp"`
}

type modelMetricRow struct {
	Model            string    `json:"model" zorm:"model"`
	Calls            int64     `json:"calls" zorm:"calls"`
	SuccessfulCalls  int64     `json:"successful_calls" zorm:"successful_calls"`
	FailedCalls      int64     `json:"failed_calls" zorm:"failed_calls"`
	InputTokens      int64     `json:"input_tokens" zorm:"input_tokens"`
	OutputTokens     int64     `json:"output_tokens" zorm:"output_tokens"`
	TotalTokens      int64     `json:"total_tokens" zorm:"total_tokens"`
	CacheReadTokens  int64     `json:"cache_read_tokens" zorm:"cache_read_tokens"`
	CacheWriteTokens int64     `json:"cache_write_tokens" zorm:"cache_write_tokens"`
	EstimatedCost    float64   `json:"estimated_cost" zorm:"estimated_cost"`
	TotalLatencyMS   float64   `json:"total_latency_ms" zorm:"total_latency_ms"`
	LatencyCount     int64     `json:"latency_count" zorm:"latency_count"`
	UpdatedAt        time.Time `json:"updated_at" zorm:"updated_at"`
}

type tokenUsageRow struct {
	ID               int64     `json:"id" zorm:"id"`
	InputTokens      int64     `json:"input_tokens" zorm:"input_tokens"`
	OutputTokens     int64     `json:"output_tokens" zorm:"output_tokens"`
	TotalTokens      int64     `json:"total_tokens" zorm:"total_tokens"`
	CacheReadTokens  int64     `json:"cache_read_tokens" zorm:"cache_read_tokens"`
	CacheWriteTokens int64     `json:"cache_write_tokens" zorm:"cache_write_tokens"`
	EstimatedCost    float64   `json:"estimated_cost" zorm:"estimated_cost"`
	UpdatedAt        time.Time `json:"updated_at" zorm:"updated_at"`
}

type modelTokenUsageRow struct {
	Model            string    `json:"model" zorm:"model"`
	InputTokens      int64     `json:"input_tokens" zorm:"input_tokens"`
	OutputTokens     int64     `json:"output_tokens" zorm:"output_tokens"`
	TotalTokens      int64     `json:"total_tokens" zorm:"total_tokens"`
	CacheReadTokens  int64     `json:"cache_read_tokens" zorm:"cache_read_tokens"`
	CacheWriteTokens int64     `json:"cache_write_tokens" zorm:"cache_write_tokens"`
	EstimatedCost    float64   `json:"estimated_cost" zorm:"estimated_cost"`
	UpdatedAt        time.Time `json:"updated_at" zorm:"updated_at"`
}

type latencySampleRow struct {
	Model       string    `json:"model" zorm:"model"`
	SampleType  string    `json:"sample_type" zorm:"sample_type"`
	Samples     string    `json:"samples" zorm:"samples"`
	TotalValue  float64   `json:"total_value" zorm:"total_value"`
	MinValue    float64   `json:"min_value" zorm:"min_value"`
	MaxValue    float64   `json:"max_value" zorm:"max_value"`
	SampleCount int64     `json:"sample_count" zorm:"sample_count"`
	UpdatedAt   time.Time `json:"updated_at" zorm:"updated_at"`
}

func pointToMetricsPointRow(point *Point) (metricsPointRow, error) {
	tagsJSON, err := json.Marshal(point.Tags)
	if err != nil {
		return metricsPointRow{}, err
	}
	fieldsJSON, err := json.Marshal(point.Fields)
	if err != nil {
		return metricsPointRow{}, err
	}
	return metricsPointRow{
		Measurement: point.Measurement,
		Tags:        string(tagsJSON),
		Fields:      string(fieldsJSON),
		Timestamp:   point.Timestamp.UTC().Format(time.RFC3339Nano),
	}, nil
}

func parseMetricsTime(raw string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err == nil {
		return parsed
	}
	parsed, err = time.Parse(time.RFC3339, raw)
	if err == nil {
		return parsed
	}
	parsed, err = time.Parse("2006-01-02 15:04:05", raw)
	if err == nil {
		return parsed
	}
	parsed, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", raw)
	return parsed
}

func modelStatsToRows(stats []ModelStats) []modelMetricRow {
	now := timeutil.NowTime().UTC()
	rows := make([]modelMetricRow, 0, len(stats))
	for _, stat := range stats {
		latencyCount := int64(1)
		if stat.AvgLatency > 0 {
			latencyCount = stat.Calls
		}
		rows = append(rows, modelMetricRow{
			Model:            stat.Model,
			Calls:            stat.Calls,
			SuccessfulCalls:  stat.SuccessfulCalls,
			FailedCalls:      stat.FailedCalls,
			InputTokens:      stat.InputTokens,
			OutputTokens:     stat.OutputTokens,
			TotalTokens:      stat.TotalTokens,
			CacheReadTokens:  stat.CacheReadTokens,
			CacheWriteTokens: stat.CacheWriteTokens,
			EstimatedCost:    stat.EstimatedCost,
			TotalLatencyMS:   stat.AvgLatency * float64(latencyCount),
			LatencyCount:     latencyCount,
			UpdatedAt:        now,
		})
	}
	return rows
}

func rowToModelStats(row modelMetricRow) ModelStats {
	stat := ModelStats{
		Model:            row.Model,
		Calls:            row.Calls,
		SuccessfulCalls:  row.SuccessfulCalls,
		FailedCalls:      row.FailedCalls,
		InputTokens:      row.InputTokens,
		OutputTokens:     row.OutputTokens,
		TotalTokens:      row.TotalTokens,
		CacheReadTokens:  row.CacheReadTokens,
		CacheWriteTokens: row.CacheWriteTokens,
		EstimatedCost:    row.EstimatedCost,
	}
	if row.LatencyCount > 0 {
		stat.AvgLatency = row.TotalLatencyMS / float64(row.LatencyCount)
	}
	if stat.Calls > 0 {
		stat.SuccessRate = float64(stat.SuccessfulCalls) / float64(stat.Calls) * 100
	}
	return stat
}

func modelTokenUsageToRows(usages []ModelTokenUsage) []modelTokenUsageRow {
	now := timeutil.NowTime().UTC()
	rows := make([]modelTokenUsageRow, 0, len(usages))
	for _, usage := range usages {
		rows = append(rows, modelTokenUsageRow{
			Model:            usage.Model,
			InputTokens:      usage.InputTokens,
			OutputTokens:     usage.OutputTokens,
			TotalTokens:      usage.TotalTokens,
			CacheReadTokens:  usage.CacheReadTokens,
			CacheWriteTokens: usage.CacheWriteTokens,
			EstimatedCost:    usage.EstimatedCost,
			UpdatedAt:        now,
		})
	}
	return rows
}

func rowToModelTokenUsage(row modelTokenUsageRow) ModelTokenUsage {
	return ModelTokenUsage{
		Model:            row.Model,
		InputTokens:      row.InputTokens,
		OutputTokens:     row.OutputTokens,
		TotalTokens:      row.TotalTokens,
		CacheReadTokens:  row.CacheReadTokens,
		CacheWriteTokens: row.CacheWriteTokens,
		EstimatedCost:    row.EstimatedCost,
	}
}

func latencySamplesToRows(samples []LatencySampleData) ([]latencySampleRow, error) {
	now := timeutil.NowTime().UTC()
	rows := make([]latencySampleRow, 0, len(samples))
	for _, sample := range samples {
		samplesJSON, err := json.Marshal(sample.Samples)
		if err != nil {
			return nil, err
		}
		rows = append(rows, latencySampleRow{
			Model:       sample.Model,
			SampleType:  sample.SampleType,
			Samples:     string(samplesJSON),
			TotalValue:  sample.TotalValue,
			MinValue:    sample.MinValue,
			MaxValue:    sample.MaxValue,
			SampleCount: sample.SampleCount,
			UpdatedAt:   now,
		})
	}
	return rows, nil
}

func rowToLatencySample(row latencySampleRow) (LatencySampleData, error) {
	sample := LatencySampleData{
		Model:       row.Model,
		SampleType:  row.SampleType,
		TotalValue:  row.TotalValue,
		MinValue:    row.MinValue,
		MaxValue:    row.MaxValue,
		SampleCount: row.SampleCount,
	}
	if err := json.Unmarshal([]byte(row.Samples), &sample.Samples); err != nil {
		return LatencySampleData{}, err
	}
	return sample, nil
}

// initSchema creates the necessary tables.
func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS metrics_points (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		measurement TEXT NOT NULL,
		tags TEXT,
		fields TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_metrics_measurement ON metrics_points(measurement);
	CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON metrics_points(timestamp);

	-- Aggregated metrics for quick access
	CREATE TABLE IF NOT EXISTS metrics_summary (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		metric_type TEXT NOT NULL UNIQUE,
		data TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Model-specific metrics
	CREATE TABLE IF NOT EXISTS model_metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		model TEXT NOT NULL UNIQUE,
		calls INTEGER DEFAULT 0,
		successful_calls INTEGER DEFAULT 0,
		failed_calls INTEGER DEFAULT 0,
		input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		cache_read_tokens INTEGER DEFAULT 0,
		cache_write_tokens INTEGER DEFAULT 0,
		estimated_cost REAL DEFAULT 0,
		total_latency_ms REAL DEFAULT 0,
		latency_count INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_model_metrics_model ON model_metrics(model);

	-- Global token usage
	CREATE TABLE IF NOT EXISTS token_usage (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		cache_read_tokens INTEGER DEFAULT 0,
		cache_write_tokens INTEGER DEFAULT 0,
		estimated_cost REAL DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Initialize global token usage row
	INSERT OR IGNORE INTO token_usage (id) VALUES (1);

	-- Latency samples for percentile calculations
	CREATE TABLE IF NOT EXISTS latency_samples (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		model TEXT NOT NULL,
		sample_type TEXT NOT NULL,
		samples TEXT NOT NULL,
		total_value REAL DEFAULT 0,
		min_value REAL DEFAULT 0,
		max_value REAL DEFAULT 0,
		sample_count INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_latency_samples_model_type ON latency_samples(model, sample_type);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Write writes a single point to the store.
func (s *SQLiteStore) Write(ctx context.Context, point *Point) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	row, err := pointToMetricsPointRow(point)
	if err != nil {
		return err
	}
	_, err = s.table(ctx, "metrics_points").Insert(row)
	return err
}

// WriteBatch writes multiple points to the store.
func (s *SQLiteStore) WriteBatch(ctx context.Context, points []*Point) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(points) == 0 {
		return nil
	}

	rows := make([]metricsPointRow, 0, len(points))
	for _, point := range points {
		row, err := pointToMetricsPointRow(point)
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := z.TableContext(ctx, tx, "metrics_points").Insert(&rows); err != nil {
		return err
	}
	return tx.Commit()
}

// Query executes a query (not fully implemented for SQLite).
func (s *SQLiteStore) Query(ctx context.Context, query string) (*QueryResult, error) {
	// SQLite doesn't support InfluxQL, return empty result
	return &QueryResult{}, nil
}

// QueryRange queries metrics within a time range.
func (s *SQLiteStore) QueryRange(ctx context.Context, measurement string, start, end time.Time, aggregation string) (*QueryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows []metricsPointRow
	if _, err := s.readTable(ctx, "metrics_points").Select(&rows,
		z.Fields("measurement", "tags", "fields", "timestamp"),
		z.Where(
			z.Eq("measurement", measurement),
			z.Between("timestamp", start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano)),
		),
		z.OrderBy("timestamp DESC"),
	); err != nil {
		return nil, err
	}

	result := &QueryResult{
		Series: []Series{{
			Name:    measurement,
			Columns: []string{"time", "tags", "fields"},
			Values:  [][]interface{}{},
		}},
	}

	for i := range rows {
		result.Series[0].Values = append(result.Series[0].Values, []interface{}{
			parseMetricsTime(rows[i].Timestamp),
			rows[i].Tags,
			rows[i].Fields,
		})
	}

	return result, nil
}

// SaveModelMetrics saves or updates model metrics.
func (s *SQLiteStore) SaveModelMetrics(ctx context.Context, stats []ModelStats) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(stats) == 0 {
		return nil
	}

	rows := modelStatsToRows(stats)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := z.TableContext(ctx, tx, "model_metrics").Insert(&rows,
		z.OnConflictDoUpdateSet(
			[]string{"model"},
			[]string{
				"calls",
				"successful_calls",
				"failed_calls",
				"input_tokens",
				"output_tokens",
				"total_tokens",
				"cache_read_tokens",
				"cache_write_tokens",
				"estimated_cost",
				"total_latency_ms",
				"latency_count",
				"updated_at",
			},
		),
	); err != nil {
		return err
	}

	return tx.Commit()
}

// LoadModelMetrics loads model metrics from the database.
func (s *SQLiteStore) LoadModelMetrics(ctx context.Context) ([]ModelStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows []modelMetricRow
	if _, err := s.readTable(ctx, "model_metrics").Select(&rows,
		z.Fields(
			"model",
			"calls",
			"successful_calls",
			"failed_calls",
			"input_tokens",
			"output_tokens",
			"total_tokens",
			"cache_read_tokens",
			"cache_write_tokens",
			"estimated_cost",
			"total_latency_ms",
			"latency_count",
		),
	); err != nil {
		return nil, err
	}

	stats := make([]ModelStats, 0, len(rows))
	for i := range rows {
		stats = append(stats, rowToModelStats(rows[i]))
	}

	return stats, nil
}

// SaveTokenUsage saves global token usage.
func (s *SQLiteStore) SaveTokenUsage(ctx context.Context, usage *TokenUsage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.table(ctx, "token_usage").Update(
		z.V{
			"input_tokens":       usage.InputTokens,
			"output_tokens":      usage.OutputTokens,
			"total_tokens":       usage.TotalTokens,
			"cache_read_tokens":  usage.CacheReadTokens,
			"cache_write_tokens": usage.CacheWriteTokens,
			"estimated_cost":     usage.EstimatedCost,
			"updated_at":         timeutil.NowTime().UTC(),
		},
		z.Fields(
			"input_tokens",
			"output_tokens",
			"total_tokens",
			"cache_read_tokens",
			"cache_write_tokens",
			"estimated_cost",
			"updated_at",
		),
		z.Where(z.Eq("id", 1)),
	)
	return err
}

// LoadTokenUsage loads global token usage.
func (s *SQLiteStore) LoadTokenUsage(ctx context.Context) (*TokenUsage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows []tokenUsageRow
	if _, err := s.readTable(ctx, "token_usage").Select(&rows,
		z.Where(z.Eq("id", 1)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return &TokenUsage{}, nil
	}
	return &TokenUsage{
		InputTokens:      rows[0].InputTokens,
		OutputTokens:     rows[0].OutputTokens,
		TotalTokens:      rows[0].TotalTokens,
		CacheReadTokens:  rows[0].CacheReadTokens,
		CacheWriteTokens: rows[0].CacheWriteTokens,
		EstimatedCost:    rows[0].EstimatedCost,
	}, nil
}

// SaveModelTokenUsage saves token usage for all models.
func (s *SQLiteStore) SaveModelTokenUsage(ctx context.Context, usages []ModelTokenUsage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(usages) == 0 {
		return nil
	}

	rows := modelTokenUsageToRows(usages)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := z.TableContext(ctx, tx, "model_metrics").Insert(&rows,
		z.OnConflictDoUpdateSet(
			[]string{"model"},
			[]string{
				"input_tokens",
				"output_tokens",
				"total_tokens",
				"cache_read_tokens",
				"cache_write_tokens",
				"estimated_cost",
				"updated_at",
			},
		),
	); err != nil {
		return err
	}

	return tx.Commit()
}

// LoadModelTokenUsage loads token usage for all models.
func (s *SQLiteStore) LoadModelTokenUsage(ctx context.Context) ([]ModelTokenUsage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows []modelTokenUsageRow
	if _, err := s.readTable(ctx, "model_metrics").Select(&rows,
		z.Fields(
			"model",
			"input_tokens",
			"output_tokens",
			"total_tokens",
			"cache_read_tokens",
			"cache_write_tokens",
			"estimated_cost",
		),
	); err != nil {
		return nil, err
	}

	usages := make([]ModelTokenUsage, 0, len(rows))
	for i := range rows {
		usages = append(usages, rowToModelTokenUsage(rows[i]))
	}

	return usages, nil
}

// LatencySampleData represents latency sample data for persistence.
type LatencySampleData struct {
	Model       string    `json:"model"`
	SampleType  string    `json:"sample_type"` // "latency", "tps", "ttft", "decode"
	Samples     []float64 `json:"samples"`
	TotalValue  float64   `json:"total_value"`
	MinValue    float64   `json:"min_value"`
	MaxValue    float64   `json:"max_value"`
	SampleCount int64     `json:"sample_count"`
}

// SaveLatencySamples saves latency samples to the database.
func (s *SQLiteStore) SaveLatencySamples(ctx context.Context, samples []LatencySampleData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(samples) == 0 {
		return nil
	}

	rows, err := latencySamplesToRows(samples)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := z.TableContext(ctx, tx, "latency_samples").Insert(&rows,
		z.OnConflictDoUpdateSet(
			[]string{"model", "sample_type"},
			[]string{
				"samples",
				"total_value",
				"min_value",
				"max_value",
				"sample_count",
				"updated_at",
			},
		),
	); err != nil {
		return err
	}

	return tx.Commit()
}

// LoadLatencySamples loads latency samples from the database.
func (s *SQLiteStore) LoadLatencySamples(ctx context.Context) ([]LatencySampleData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows []latencySampleRow
	if _, err := s.readTable(ctx, "latency_samples").Select(&rows,
		z.Fields(
			"model",
			"sample_type",
			"samples",
			"total_value",
			"min_value",
			"max_value",
			"sample_count",
		),
	); err != nil {
		return nil, err
	}

	samples := make([]LatencySampleData, 0, len(rows))
	for i := range rows {
		sample, err := rowToLatencySample(rows[i])
		if err != nil {
			return nil, err
		}
		samples = append(samples, sample)
	}

	return samples, nil
}

// Cleanup removes old metrics data.
func (s *SQLiteStore) Cleanup(ctx context.Context, retention time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := timeutil.NowTime().Add(-retention)
	_, err := s.table(ctx, "metrics_points").Delete(
		z.Where(z.Lt("timestamp", cutoff.UTC().Format(time.RFC3339Nano))),
	)
	return err
}

// Close closes the database connection when this store owns it.
func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil || !s.ownsDB {
		return nil
	}
	if s.readDB != nil && s.readDB != s.db {
		_ = s.readDB.Close()
	}
	return s.db.Close()
}
