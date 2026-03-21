// Package database provides database optimization utilities.
package database

import (
	"context"
	"database/sql"
	"time"
)

// QueryStats holds statistics for a query execution.
type QueryStats struct {
	Query        string        `json:"query"`
	Duration     time.Duration `json:"duration"`
	RowsScanned  int64         `json:"rows_scanned"`
	RowsReturned int64         `json:"rows_returned"`
	IndexUsed    string        `json:"index_used,omitempty"`
	FullScan     bool          `json:"full_scan"`
}

// OptimizerConfig holds configuration for the query optimizer.
type OptimizerConfig struct {
	// SlowQueryThreshold is the duration above which a query is considered slow.
	SlowQueryThreshold time.Duration `yaml:"slow_query_threshold"`
	// EnableQueryCache enables query result caching.
	EnableQueryCache bool `yaml:"enable_query_cache"`
	// QueryCacheTTL is the TTL for cached query results.
	QueryCacheTTL time.Duration `yaml:"query_cache_ttl"`
	// MaxCachedQueries is the maximum number of queries to cache.
	MaxCachedQueries int `yaml:"max_cached_queries"`
	// LogSlowQueries enables logging of slow queries.
	LogSlowQueries bool `yaml:"log_slow_queries"`
}

// DefaultOptimizerConfig returns the default optimizer configuration.
func DefaultOptimizerConfig() OptimizerConfig {
	return OptimizerConfig{
		SlowQueryThreshold: 100 * time.Millisecond,
		EnableQueryCache:   true,
		QueryCacheTTL:      5 * time.Minute,
		MaxCachedQueries:   1000,
		LogSlowQueries:     true,
	}
}

// BatchConfig holds configuration for batch operations.
type BatchConfig struct {
	// BatchSize is the number of items per batch.
	BatchSize int `yaml:"batch_size"`
	// MaxRetries is the maximum number of retries for failed batches.
	MaxRetries int `yaml:"max_retries"`
	// RetryDelay is the delay between retries.
	RetryDelay time.Duration `yaml:"retry_delay"`
}

// DefaultBatchConfig returns the default batch configuration.
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		BatchSize:  1000,
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}
}

// WALConfig holds configuration for SQLite WAL mode.
type WALConfig struct {
	// Enabled enables WAL mode.
	Enabled bool `yaml:"enabled"`
	// CheckpointInterval is the interval between automatic checkpoints.
	CheckpointInterval time.Duration `yaml:"checkpoint_interval"`
	// CheckpointThreshold is the number of pages before triggering a checkpoint.
	CheckpointThreshold int `yaml:"checkpoint_threshold"`
	// CacheSize is the number of pages to cache (negative = KB).
	CacheSize int `yaml:"cache_size"`
	// PageSize is the database page size in bytes.
	PageSize int `yaml:"page_size"`
	// BusyTimeout is the timeout for busy connections in milliseconds.
	BusyTimeout int `yaml:"busy_timeout"`
	// SynchronousMode is the synchronous mode (OFF, NORMAL, FULL, EXTRA).
	SynchronousMode string `yaml:"synchronous_mode"`
}

// DefaultWALConfig returns the default WAL configuration.
func DefaultWALConfig() WALConfig {
	return WALConfig{
		Enabled:             true,
		CheckpointInterval:  5 * time.Minute,
		CheckpointThreshold: 1000,
		CacheSize:           -10000, // 10MB
		PageSize:            4096,
		BusyTimeout:         5000,
		SynchronousMode:     "FULL",
	}
}

// PoolConfig holds configuration for connection pooling.
type PoolConfig struct {
	// MaxOpenConns is the maximum number of open connections.
	MaxOpenConns int `yaml:"max_open_conns"`
	// MaxIdleConns is the maximum number of idle connections.
	MaxIdleConns int `yaml:"max_idle_conns"`
	// ConnMaxLifetime is the maximum lifetime of a connection.
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	// ConnMaxIdleTime is the maximum idle time of a connection.
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"`
	// HealthCheckInterval is the interval between health checks.
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`
}

// DefaultPoolConfig returns the default pool configuration.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:        10,
		MaxIdleConns:        5,
		ConnMaxLifetime:     time.Hour,
		ConnMaxIdleTime:     30 * time.Minute,
		HealthCheckInterval: time.Minute,
	}
}

// QueryPlan represents the result of EXPLAIN QUERY PLAN.
type QueryPlan struct {
	ID       int    `json:"id"`
	Parent   int    `json:"parent"`
	NotUsed  int    `json:"not_used"`
	Detail   string `json:"detail"`
	SelectID int    `json:"select_id,omitempty"`
	Order    int    `json:"order,omitempty"`
	From     int    `json:"from,omitempty"`
}

// IndexInfo represents information about a database index.
type IndexInfo struct {
	Name      string   `json:"name"`
	TableName string   `json:"table_name"`
	Columns   []string `json:"columns"`
	Unique    bool     `json:"unique"`
	Partial   bool     `json:"partial"`
}

// TableStats represents statistics about a database table.
type TableStats struct {
	Name       string `json:"name"`
	RowCount   int64  `json:"row_count"`
	PageCount  int64  `json:"page_count"`
	IndexCount int    `json:"index_count"`
}

// DBExecutor is an interface for database operations.
type DBExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}
