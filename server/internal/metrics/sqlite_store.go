package metrics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore implements MetricsStore using SQLite for persistence.
type SQLiteStore struct {
	db     *sql.DB
	dbPath string
	mu     sync.RWMutex
}

// NewSQLiteStore creates a new SQLite-based metrics store.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	store, err := openMetricsDB(dbPath)
	if err != nil {
		// DB is corrupt or locked — rename and retry with a fresh one
		rotateCorruptDB(dbPath)
		store, err = openMetricsDB(dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open database after rotation: %w", err)
		}
	}
	return store, nil
}

// openMetricsDB opens (or creates) the metrics SQLite database.
func openMetricsDB(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Connection pool limits
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)

	// DELETE journal — no -shm/-wal files; metrics are expendable
	db.Exec("PRAGMA journal_mode=DELETE")
	db.Exec("PRAGMA busy_timeout=5000")
	db.Exec("PRAGMA cache_size=-500") // ~512KB page cache for lower idle memory

	store := &SQLiteStore{
		db:     db,
		dbPath: dbPath,
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Release unused memory after schema init
	db.Exec("PRAGMA shrink_memory")

	return store, nil
}

// rotateCorruptDB renames a corrupt/locked DB (and its WAL/SHM) out of the way.
func rotateCorruptDB(dbPath string) {
	suffix := fmt.Sprintf(".bad.%d", time.Now().Unix())
	os.Rename(dbPath, dbPath+suffix)
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")
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

	tagsJSON, err := json.Marshal(point.Tags)
	if err != nil {
		return err
	}

	fieldsJSON, err := json.Marshal(point.Fields)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO metrics_points (measurement, tags, fields, timestamp) VALUES (?, ?, ?, ?)`,
		point.Measurement, string(tagsJSON), string(fieldsJSON), point.Timestamp,
	)

	return err
}

// WriteBatch writes multiple points to the store.
func (s *SQLiteStore) WriteBatch(ctx context.Context, points []*Point) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO metrics_points (measurement, tags, fields, timestamp) VALUES (?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, point := range points {
		tagsJSON, err := json.Marshal(point.Tags)
		if err != nil {
			return err
		}

		fieldsJSON, err := json.Marshal(point.Fields)
		if err != nil {
			return err
		}

		_, err = stmt.ExecContext(ctx, point.Measurement, string(tagsJSON), string(fieldsJSON), point.Timestamp)
		if err != nil {
			return err
		}
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

	rows, err := s.db.QueryContext(ctx,
		`SELECT tags, fields, timestamp FROM metrics_points
		 WHERE measurement = ? AND timestamp BETWEEN ? AND ?
		 ORDER BY timestamp DESC`,
		measurement, start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &QueryResult{
		Series: []Series{{
			Name:    measurement,
			Columns: []string{"time", "tags", "fields"},
			Values:  [][]interface{}{},
		}},
	}

	for rows.Next() {
		var tagsJSON, fieldsJSON string
		var timestamp time.Time
		if err := rows.Scan(&tagsJSON, &fieldsJSON, &timestamp); err != nil {
			return nil, err
		}
		result.Series[0].Values = append(result.Series[0].Values, []interface{}{timestamp, tagsJSON, fieldsJSON})
	}

	return result, nil
}

// SaveModelMetrics saves or updates model metrics.
func (s *SQLiteStore) SaveModelMetrics(ctx context.Context, stats []ModelStats) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO model_metrics (model, calls, successful_calls, failed_calls,
			input_tokens, output_tokens, total_tokens, cache_read_tokens, cache_write_tokens,
			estimated_cost, total_latency_ms, latency_count, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(model) DO UPDATE SET
			calls = excluded.calls,
			successful_calls = excluded.successful_calls,
			failed_calls = excluded.failed_calls,
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			total_tokens = excluded.total_tokens,
			cache_read_tokens = excluded.cache_read_tokens,
			cache_write_tokens = excluded.cache_write_tokens,
			estimated_cost = excluded.estimated_cost,
			total_latency_ms = excluded.total_latency_ms,
			latency_count = excluded.latency_count,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, stat := range stats {
		latencyCount := int64(1)
		if stat.AvgLatency > 0 {
			latencyCount = stat.Calls
		}
		_, err = stmt.ExecContext(ctx,
			stat.Model, stat.Calls, stat.SuccessfulCalls, stat.FailedCalls,
			stat.InputTokens, stat.OutputTokens, stat.TotalTokens,
			stat.CacheReadTokens, stat.CacheWriteTokens,
			stat.EstimatedCost, stat.AvgLatency*float64(latencyCount), latencyCount,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LoadModelMetrics loads model metrics from the database.
func (s *SQLiteStore) LoadModelMetrics(ctx context.Context) ([]ModelStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT model, calls, successful_calls, failed_calls,
			input_tokens, output_tokens, total_tokens, cache_read_tokens, cache_write_tokens,
			estimated_cost, total_latency_ms, latency_count
		FROM model_metrics
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []ModelStats
	for rows.Next() {
		var stat ModelStats
		var totalLatency float64
		var latencyCount int64
		err := rows.Scan(
			&stat.Model, &stat.Calls, &stat.SuccessfulCalls, &stat.FailedCalls,
			&stat.InputTokens, &stat.OutputTokens, &stat.TotalTokens,
			&stat.CacheReadTokens, &stat.CacheWriteTokens,
			&stat.EstimatedCost, &totalLatency, &latencyCount,
		)
		if err != nil {
			return nil, err
		}

		if latencyCount > 0 {
			stat.AvgLatency = totalLatency / float64(latencyCount)
		}
		if stat.Calls > 0 {
			stat.SuccessRate = float64(stat.SuccessfulCalls) / float64(stat.Calls) * 100
		}

		stats = append(stats, stat)
	}

	return stats, nil
}

// SaveTokenUsage saves global token usage.
func (s *SQLiteStore) SaveTokenUsage(ctx context.Context, usage *TokenUsage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		UPDATE token_usage SET
			input_tokens = ?,
			output_tokens = ?,
			total_tokens = ?,
			cache_read_tokens = ?,
			cache_write_tokens = ?,
			estimated_cost = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, usage.InputTokens, usage.OutputTokens, usage.TotalTokens,
		usage.CacheReadTokens, usage.CacheWriteTokens, usage.EstimatedCost)

	return err
}

// LoadTokenUsage loads global token usage.
func (s *SQLiteStore) LoadTokenUsage(ctx context.Context) (*TokenUsage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var usage TokenUsage
	err := s.db.QueryRowContext(ctx, `
		SELECT input_tokens, output_tokens, total_tokens,
			cache_read_tokens, cache_write_tokens, estimated_cost
		FROM token_usage WHERE id = 1
	`).Scan(&usage.InputTokens, &usage.OutputTokens, &usage.TotalTokens,
		&usage.CacheReadTokens, &usage.CacheWriteTokens, &usage.EstimatedCost)

	if err == sql.ErrNoRows {
		return &TokenUsage{}, nil
	}
	if err != nil {
		return nil, err
	}

	return &usage, nil
}

// SaveModelTokenUsage saves token usage for all models.
func (s *SQLiteStore) SaveModelTokenUsage(ctx context.Context, usages []ModelTokenUsage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Use model_metrics table for token usage as well
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO model_metrics (model, input_tokens, output_tokens, total_tokens,
			cache_read_tokens, cache_write_tokens, estimated_cost, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(model) DO UPDATE SET
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			total_tokens = excluded.total_tokens,
			cache_read_tokens = excluded.cache_read_tokens,
			cache_write_tokens = excluded.cache_write_tokens,
			estimated_cost = excluded.estimated_cost,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, usage := range usages {
		_, err = stmt.ExecContext(ctx,
			usage.Model, usage.InputTokens, usage.OutputTokens, usage.TotalTokens,
			usage.CacheReadTokens, usage.CacheWriteTokens, usage.EstimatedCost,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LoadModelTokenUsage loads token usage for all models.
func (s *SQLiteStore) LoadModelTokenUsage(ctx context.Context) ([]ModelTokenUsage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT model, input_tokens, output_tokens, total_tokens,
			cache_read_tokens, cache_write_tokens, estimated_cost
		FROM model_metrics
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usages []ModelTokenUsage
	for rows.Next() {
		var usage ModelTokenUsage
		err := rows.Scan(
			&usage.Model, &usage.InputTokens, &usage.OutputTokens, &usage.TotalTokens,
			&usage.CacheReadTokens, &usage.CacheWriteTokens, &usage.EstimatedCost,
		)
		if err != nil {
			return nil, err
		}
		usages = append(usages, usage)
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO latency_samples (model, sample_type, samples, total_value, min_value, max_value, sample_count, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(model, sample_type) DO UPDATE SET
			samples = excluded.samples,
			total_value = excluded.total_value,
			min_value = excluded.min_value,
			max_value = excluded.max_value,
			sample_count = excluded.sample_count,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, sample := range samples {
		samplesJSON, err := json.Marshal(sample.Samples)
		if err != nil {
			return err
		}

		_, err = stmt.ExecContext(ctx,
			sample.Model, sample.SampleType, string(samplesJSON),
			sample.TotalValue, sample.MinValue, sample.MaxValue, sample.SampleCount,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LoadLatencySamples loads latency samples from the database.
func (s *SQLiteStore) LoadLatencySamples(ctx context.Context) ([]LatencySampleData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT model, sample_type, samples, total_value, min_value, max_value, sample_count
		FROM latency_samples
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []LatencySampleData
	for rows.Next() {
		var sample LatencySampleData
		var samplesJSON string
		err := rows.Scan(
			&sample.Model, &sample.SampleType, &samplesJSON,
			&sample.TotalValue, &sample.MinValue, &sample.MaxValue, &sample.SampleCount,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(samplesJSON), &sample.Samples); err != nil {
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

	cutoff := time.Now().Add(-retention)
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM metrics_points WHERE timestamp < ?`,
		cutoff,
	)
	return err
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
