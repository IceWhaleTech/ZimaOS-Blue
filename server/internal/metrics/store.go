package metrics

import (
	"context"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// MetricsStore defines the interface for metrics storage.
type MetricsStore interface {
	// Write writes a single point to the store.
	Write(ctx context.Context, point *Point) error

	// WriteBatch writes multiple points to the store.
	WriteBatch(ctx context.Context, points []*Point) error

	// Query executes a query and returns results.
	Query(ctx context.Context, query string) (*QueryResult, error)

	// QueryRange queries metrics within a time range.
	QueryRange(ctx context.Context, measurement string, start, end time.Time, aggregation string) (*QueryResult, error)

	// Close closes the store connection.
	Close() error
}

// Point represents a single metric data point.
type Point struct {
	// Measurement is the metric name (e.g., "api_calls", "token_usage")
	Measurement string

	// Tags are indexed metadata (e.g., model, status)
	Tags map[string]string

	// Fields are the actual metric values
	Fields map[string]interface{}

	// Timestamp is when the metric was recorded
	Timestamp time.Time
}

// NewPoint creates a new Point with the given measurement.
func NewPoint(measurement string) *Point {
	return &Point{
		Measurement: measurement,
		Tags:        make(map[string]string),
		Fields:      make(map[string]interface{}),
		Timestamp:   timeutil.NowTime(),
	}
}

// AddTag adds a tag to the point.
func (p *Point) AddTag(key, value string) *Point {
	p.Tags[key] = value
	return p
}

// AddField adds a field to the point.
func (p *Point) AddField(key string, value interface{}) *Point {
	p.Fields[key] = value
	return p
}

// SetTimestamp sets the timestamp for the point.
func (p *Point) SetTimestamp(t time.Time) *Point {
	p.Timestamp = t
	return p
}

// QueryResult represents the result of a query.
type QueryResult struct {
	// Series contains the result series
	Series []Series
}

// Series represents a single series in the query result.
type Series struct {
	// Name is the measurement name
	Name string

	// Tags are the series tags
	Tags map[string]string

	// Columns are the column names
	Columns []string

	// Values are the data rows
	Values [][]interface{}
}

// RetentionPolicy defines a retention policy configuration.
type RetentionPolicy struct {
	Name     string
	Duration time.Duration
	Default  bool
}

// ContinuousQuery defines a continuous query configuration.
type ContinuousQuery struct {
	Name          string
	Database      string
	Query         string
	ResampleEvery time.Duration
	ResampleFor   time.Duration
}

// StoreConfig contains configuration for the metrics store.
type StoreConfig struct {
	// Connection
	URL      string
	Database string
	Username string
	Password string

	// Retention policies
	RetentionPolicies []RetentionPolicy

	// Collection settings
	CollectionInterval time.Duration
	BatchSize          int
	FlushInterval      time.Duration
}

// DefaultStoreConfig returns the default store configuration.
func DefaultStoreConfig() *StoreConfig {
	return &StoreConfig{
		URL:      "http://localhost:8086",
		Database: "echo_metrics",

		RetentionPolicies: []RetentionPolicy{
			{Name: "raw", Duration: time.Hour, Default: true},
			{Name: "short", Duration: 24 * time.Hour},
			{Name: "medium", Duration: 7 * 24 * time.Hour},
			{Name: "long", Duration: 30 * 24 * time.Hour},
			{Name: "archive", Duration: 365 * 24 * time.Hour},
		},

		CollectionInterval: 10 * time.Second,
		BatchSize:          100,
		FlushInterval:      10 * time.Second,
	}
}

// Measurement names for metrics.
const (
	MeasurementAPICalls   = "api_calls"
	MeasurementTokenUsage = "token_usage"
	MeasurementLatency    = "latency"
	MeasurementSpeed      = "speed"
	MeasurementSystem     = "system"
	MeasurementProcess    = "process"
	MeasurementCounters   = "runtime_counters"
)

// Tag keys for metrics.
const (
	TagModel     = "model"
	TagStatus    = "status"
	TagErrorType = "error_type"
	TagMetric    = "metric"
)

// Field keys for metrics.
const (
	FieldCount            = "count"
	FieldSuccess          = "success"
	FieldLatencyMs        = "latency_ms"
	FieldInputTokens      = "input_tokens"
	FieldOutputTokens     = "output_tokens"
	FieldTotalTokens      = "total_tokens"
	FieldCacheReadTokens  = "cache_read_tokens"
	FieldCacheWriteTokens = "cache_write_tokens"
	FieldCost             = "cost"
	FieldTokensPerSecond  = "tokens_per_second"
	FieldTTFT             = "ttft_ms"
	FieldCPUPercent       = "cpu_percent"
	FieldMemoryBytes      = "memory_bytes"
	FieldMemoryPercent    = "memory_percent"
	FieldDiskPercent      = "disk_percent"
)

// Helper functions

func boolToStatus(b bool) string {
	if b {
		return "success"
	}
	return "error"
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
