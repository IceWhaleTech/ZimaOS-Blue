package metrics

import (
	"context"
	"testing"
	"time"
)

func TestNewPoint(t *testing.T) {
	point := NewPoint("test_measurement")

	if point.Measurement != "test_measurement" {
		t.Errorf("Expected measurement 'test_measurement', got '%s'", point.Measurement)
	}

	if point.Tags == nil {
		t.Error("Expected Tags to be initialized")
	}

	if point.Fields == nil {
		t.Error("Expected Fields to be initialized")
	}

	if point.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}
}

func TestPoint_AddTag(t *testing.T) {
	point := NewPoint("test").
		AddTag("key1", "value1").
		AddTag("key2", "value2")

	if point.Tags["key1"] != "value1" {
		t.Errorf("Expected tag key1='value1', got '%s'", point.Tags["key1"])
	}

	if point.Tags["key2"] != "value2" {
		t.Errorf("Expected tag key2='value2', got '%s'", point.Tags["key2"])
	}
}

func TestPoint_AddField(t *testing.T) {
	point := NewPoint("test").
		AddField("count", 100).
		AddField("rate", 0.95).
		AddField("name", "test")

	if point.Fields["count"] != 100 {
		t.Errorf("Expected field count=100, got %v", point.Fields["count"])
	}

	if point.Fields["rate"] != 0.95 {
		t.Errorf("Expected field rate=0.95, got %v", point.Fields["rate"])
	}

	if point.Fields["name"] != "test" {
		t.Errorf("Expected field name='test', got %v", point.Fields["name"])
	}
}

func TestPoint_SetTimestamp(t *testing.T) {
	ts := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	point := NewPoint("test").SetTimestamp(ts)

	if !point.Timestamp.Equal(ts) {
		t.Errorf("Expected timestamp %v, got %v", ts, point.Timestamp)
	}
}

func TestDefaultStoreConfig(t *testing.T) {
	config := DefaultStoreConfig()

	if config.URL != "http://localhost:8086" {
		t.Errorf("Expected URL 'http://localhost:8086', got '%s'", config.URL)
	}

	if config.Database != "echo_metrics" {
		t.Errorf("Expected Database 'echo_metrics', got '%s'", config.Database)
	}

	if len(config.RetentionPolicies) != 5 {
		t.Errorf("Expected 5 retention policies, got %d", len(config.RetentionPolicies))
	}

	// Check retention policy names
	expectedPolicies := []string{"raw", "short", "medium", "long", "archive"}
	for i, expected := range expectedPolicies {
		if config.RetentionPolicies[i].Name != expected {
			t.Errorf("Expected policy %d to be '%s', got '%s'", i, expected, config.RetentionPolicies[i].Name)
		}
	}

	if config.BatchSize != 100 {
		t.Errorf("Expected BatchSize 100, got %d", config.BatchSize)
	}

	if config.FlushInterval != 10*time.Second {
		t.Errorf("Expected FlushInterval 10s, got %v", config.FlushInterval)
	}
}

func TestBoolToStatus(t *testing.T) {
	if boolToStatus(true) != "success" {
		t.Error("Expected 'success' for true")
	}

	if boolToStatus(false) != "error" {
		t.Error("Expected 'error' for false")
	}
}

func TestBoolToInt(t *testing.T) {
	if boolToInt(true) != 1 {
		t.Error("Expected 1 for true")
	}

	if boolToInt(false) != 0 {
		t.Error("Expected 0 for false")
	}
}

// MockStore implements MetricsStore for testing.
type MockStore struct {
	points []*Point
}

func NewMockStore() *MockStore {
	return &MockStore{
		points: make([]*Point, 0),
	}
}

func (m *MockStore) Write(ctx context.Context, point *Point) error {
	m.points = append(m.points, point)
	return nil
}

func (m *MockStore) WriteBatch(ctx context.Context, points []*Point) error {
	m.points = append(m.points, points...)
	return nil
}

func (m *MockStore) Query(ctx context.Context, query string) (*QueryResult, error) {
	return &QueryResult{}, nil
}

func (m *MockStore) QueryRange(ctx context.Context, measurement string, start, end time.Time, aggregation string) (*QueryResult, error) {
	return &QueryResult{}, nil
}

func (m *MockStore) Close() error {
	return nil
}

func (m *MockStore) GetPoints() []*Point {
	return m.points
}

func (m *MockStore) Clear() {
	m.points = m.points[:0]
}
