package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

// InfluxDBStore implements MetricsStore using InfluxDB.
type InfluxDBStore struct {
	client   influxdb2.Client
	writeAPI api.WriteAPI
	queryAPI api.QueryAPI

	config *StoreConfig

	// Buffer for batch writes
	buffer     []*Point
	bufferMu   sync.Mutex
	flushTimer *time.Timer

	// Shutdown
	done chan struct{}
	wg   sync.WaitGroup
}

// NewInfluxDBStore creates a new InfluxDB store.
func NewInfluxDBStore(config *StoreConfig) (*InfluxDBStore, error) {
	if config == nil {
		config = DefaultStoreConfig()
	}

	// Create InfluxDB client
	client := influxdb2.NewClient(config.URL, "")

	store := &InfluxDBStore{
		client:   client,
		writeAPI: client.WriteAPI("", config.Database),
		queryAPI: client.QueryAPI(""),
		config:   config,
		buffer:   make([]*Point, 0, config.BatchSize),
		done:     make(chan struct{}),
	}

	// Start background flush goroutine
	store.wg.Add(1)
	go store.flushLoop()

	return store, nil
}

// Write writes a single point to the store.
func (s *InfluxDBStore) Write(ctx context.Context, point *Point) error {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()

	s.buffer = append(s.buffer, point)

	if len(s.buffer) >= s.config.BatchSize {
		return s.flushLocked(ctx)
	}

	return nil
}

// WriteBatch writes multiple points to the store.
func (s *InfluxDBStore) WriteBatch(ctx context.Context, points []*Point) error {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()

	s.buffer = append(s.buffer, points...)

	if len(s.buffer) >= s.config.BatchSize {
		return s.flushLocked(ctx)
	}

	return nil
}

// Flush forces a flush of buffered points.
func (s *InfluxDBStore) Flush(ctx context.Context) error {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()

	return s.flushLocked(ctx)
}

// flushLocked flushes buffered points (must hold bufferMu).
func (s *InfluxDBStore) flushLocked(ctx context.Context) error {
	if len(s.buffer) == 0 {
		return nil
	}

	for _, point := range s.buffer {
		p := influxdb2.NewPoint(
			point.Measurement,
			point.Tags,
			point.Fields,
			point.Timestamp,
		)
		s.writeAPI.WritePoint(p)
	}

	// Clear buffer
	s.buffer = s.buffer[:0]

	return nil
}

// flushLoop periodically flushes buffered points.
func (s *InfluxDBStore) flushLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.Flush(context.Background())
		case <-s.done:
			// Final flush
			s.Flush(context.Background())
			return
		}
	}
}

// Query executes a Flux query and returns results.
func (s *InfluxDBStore) Query(ctx context.Context, query string) (*QueryResult, error) {
	result, err := s.queryAPI.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer result.Close()

	qr := &QueryResult{
		Series: make([]Series, 0),
	}

	currentSeries := &Series{
		Columns: make([]string, 0),
		Values:  make([][]interface{}, 0),
	}

	for result.Next() {
		record := result.Record()

		// Build row
		row := make([]interface{}, 0)
		for _, col := range currentSeries.Columns {
			row = append(row, record.ValueByKey(col))
		}

		if len(row) == 0 {
			// First record, extract columns
			for key := range record.Values() {
				currentSeries.Columns = append(currentSeries.Columns, key)
			}
			for _, col := range currentSeries.Columns {
				row = append(row, record.ValueByKey(col))
			}
		}

		currentSeries.Values = append(currentSeries.Values, row)
	}

	if result.Err() != nil {
		return nil, fmt.Errorf("query iteration error: %w", result.Err())
	}

	if len(currentSeries.Values) > 0 {
		qr.Series = append(qr.Series, *currentSeries)
	}

	return qr, nil
}

// QueryRange queries metrics within a time range.
func (s *InfluxDBStore) QueryRange(ctx context.Context, measurement string, start, end time.Time, aggregation string) (*QueryResult, error) {
	// Build Flux query
	query := fmt.Sprintf(`
		from(bucket: "%s")
			|> range(start: %s, stop: %s)
			|> filter(fn: (r) => r._measurement == "%s")
	`, s.config.Database, start.Format(time.RFC3339), end.Format(time.RFC3339), measurement)

	// Add aggregation if specified
	if aggregation != "" {
		query += fmt.Sprintf(`
			|> aggregateWindow(every: %s, fn: mean, createEmpty: false)
		`, aggregation)
	}

	return s.Query(ctx, query)
}

// Close closes the store connection.
func (s *InfluxDBStore) Close() error {
	close(s.done)
	s.wg.Wait()

	s.writeAPI.Flush()
	s.client.Close()

	return nil
}

// WriteAPICall writes an API call metric.
func (s *InfluxDBStore) WriteAPICall(ctx context.Context, model string, success bool, latencyMs float64, errorType string) error {
	point := NewPoint(MeasurementAPICalls).
		AddTag(TagModel, model).
		AddTag(TagStatus, boolToStatus(success)).
		AddField(FieldCount, 1).
		AddField(FieldSuccess, boolToInt(success)).
		AddField(FieldLatencyMs, latencyMs)

	if errorType != "" {
		point.AddTag(TagErrorType, errorType)
	}

	return s.Write(ctx, point)
}

// WriteTokenUsage writes token usage metrics.
func (s *InfluxDBStore) WriteTokenUsage(ctx context.Context, model string, inputTokens, outputTokens, cacheRead, cacheWrite int64, cost float64) error {
	point := NewPoint(MeasurementTokenUsage).
		AddTag(TagModel, model).
		AddField(FieldInputTokens, inputTokens).
		AddField(FieldOutputTokens, outputTokens).
		AddField(FieldTotalTokens, inputTokens+outputTokens).
		AddField(FieldCacheReadTokens, cacheRead).
		AddField(FieldCacheWriteTokens, cacheWrite).
		AddField(FieldCost, cost)

	return s.Write(ctx, point)
}

// WriteLatency writes latency metrics.
func (s *InfluxDBStore) WriteLatency(ctx context.Context, model string, latencyMs float64) error {
	point := NewPoint(MeasurementLatency).
		AddTag(TagModel, model).
		AddField(FieldLatencyMs, latencyMs)

	return s.Write(ctx, point)
}

// WriteSpeed writes speed metrics.
func (s *InfluxDBStore) WriteSpeed(ctx context.Context, model string, tokensPerSecond, ttftMs float64) error {
	point := NewPoint(MeasurementSpeed).
		AddTag(TagModel, model).
		AddField(FieldTokensPerSecond, tokensPerSecond).
		AddField(FieldTTFT, ttftMs)

	return s.Write(ctx, point)
}

// WriteSystemMetrics writes system resource metrics.
func (s *InfluxDBStore) WriteSystemMetrics(ctx context.Context, cpuPercent, memoryBytes, memoryPercent, diskPercent float64) error {
	point := NewPoint(MeasurementSystem).
		AddField(FieldCPUPercent, cpuPercent).
		AddField(FieldMemoryBytes, memoryBytes).
		AddField(FieldMemoryPercent, memoryPercent).
		AddField(FieldDiskPercent, diskPercent)

	return s.Write(ctx, point)
}

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
