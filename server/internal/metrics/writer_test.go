package metrics

import (
	"testing"
	"time"
)

func TestNewMetricsWriter(t *testing.T) {
	store := NewMockStore()
	writer := NewMetricsWriter(store, nil)

	if writer == nil {
		t.Fatal("Expected writer to be created")
	}

	if writer.callCollector == nil {
		t.Error("Expected callCollector to be initialized")
	}

	if writer.tokenTracker == nil {
		t.Error("Expected tokenTracker to be initialized")
	}

	if writer.latencyTracker == nil {
		t.Error("Expected latencyTracker to be initialized")
	}
}

func TestMetricsWriter_RecordAPICall(t *testing.T) {
	store := NewMockStore()
	config := &WriterConfig{
		CollectionInterval:    time.Hour, // Long interval to prevent auto-flush
		MaxSamples:            1000,
		EnableSystemMetrics:   false,
		SystemMetricsInterval: time.Hour,
	}
	writer := NewMetricsWriter(store, config)

	// Record a successful API call
	writer.RecordAPICall("sonnet", true, 1500.0, 1000, 500, 100, 50, "")

	// Check call stats
	stats := writer.GetCallStats()
	if stats.TotalCalls != 1 {
		t.Errorf("Expected 1 total call, got %d", stats.TotalCalls)
	}
	if stats.SuccessfulCalls != 1 {
		t.Errorf("Expected 1 successful call, got %d", stats.SuccessfulCalls)
	}

	// Check token usage
	tokenUsage := writer.GetTokenUsage()
	if tokenUsage.InputTokens != 1000 {
		t.Errorf("Expected 1000 input tokens, got %d", tokenUsage.InputTokens)
	}
	if tokenUsage.OutputTokens != 500 {
		t.Errorf("Expected 500 output tokens, got %d", tokenUsage.OutputTokens)
	}

	// Check latency stats
	latencyStats := writer.GetLatencyStats()
	if latencyStats.Samples != 1 {
		t.Errorf("Expected 1 latency sample, got %d", latencyStats.Samples)
	}
	if latencyStats.Avg != 1500.0 {
		t.Errorf("Expected avg latency 1500.0, got %.2f", latencyStats.Avg)
	}

	// Check that point was written to store
	points := store.GetPoints()
	if len(points) != 1 {
		t.Errorf("Expected 1 point written to store, got %d", len(points))
	}
}

func TestMetricsWriter_RecordAPICall_WithError(t *testing.T) {
	store := NewMockStore()
	writer := NewMetricsWriter(store, nil)

	// Record a failed API call
	writer.RecordAPICall("sonnet", false, 500.0, 100, 0, 0, 0, "rate_limit")

	stats := writer.GetCallStats()
	if stats.FailedCalls != 1 {
		t.Errorf("Expected 1 failed call, got %d", stats.FailedCalls)
	}

	// Check that error type tag was added
	points := store.GetPoints()
	if len(points) != 1 {
		t.Fatalf("Expected 1 point, got %d", len(points))
	}

	if points[0].Tags[TagErrorType] != "rate_limit" {
		t.Errorf("Expected error_type='rate_limit', got '%s'", points[0].Tags[TagErrorType])
	}
}

func TestMetricsWriter_RecordSpeed(t *testing.T) {
	store := NewMockStore()
	writer := NewMetricsWriter(store, nil)

	writer.RecordSpeed("sonnet", 50.0, 200.0, 55.0)

	speedStats := writer.GetSpeedStats()
	if speedStats.TokensPerSecond != 50.0 {
		t.Errorf("Expected TPS 50.0, got %.2f", speedStats.TokensPerSecond)
	}

	// Check that point was written
	points := store.GetPoints()
	if len(points) != 1 {
		t.Errorf("Expected 1 point, got %d", len(points))
	}

	if points[0].Measurement != MeasurementSpeed {
		t.Errorf("Expected measurement '%s', got '%s'", MeasurementSpeed, points[0].Measurement)
	}
}

func TestMetricsWriter_GetModelStats(t *testing.T) {
	store := NewMockStore()
	writer := NewMetricsWriter(store, nil)

	writer.RecordAPICall("sonnet", true, 1500.0, 1000, 500, 0, 0, "")
	writer.RecordAPICall("opus", true, 3000.0, 2000, 1000, 0, 0, "")
	writer.RecordAPICall("sonnet", true, 1000.0, 500, 250, 0, 0, "")

	modelStats := writer.GetModelStats()
	if len(modelStats) != 2 {
		t.Errorf("Expected 2 models, got %d", len(modelStats))
	}

	// Find sonnet stats
	var sonnetStats *ModelStats
	for i := range modelStats {
		if modelStats[i].Model == "sonnet" {
			sonnetStats = &modelStats[i]
			break
		}
	}

	if sonnetStats == nil {
		t.Fatal("Expected to find sonnet stats")
	}

	if sonnetStats.Calls != 2 {
		t.Errorf("Expected 2 calls for sonnet, got %d", sonnetStats.Calls)
	}
}

func TestMetricsWriter_GetAllModelUsage(t *testing.T) {
	store := NewMockStore()
	writer := NewMetricsWriter(store, nil)

	writer.RecordAPICall("sonnet", true, 1500.0, 1000, 500, 0, 0, "")
	writer.RecordAPICall("opus", true, 3000.0, 2000, 1000, 0, 0, "")

	allUsage := writer.GetAllModelUsage()
	if len(allUsage) != 2 {
		t.Errorf("Expected 2 models, got %d", len(allUsage))
	}
}

func TestMetricsWriter_Reset(t *testing.T) {
	store := NewMockStore()
	writer := NewMetricsWriter(store, nil)

	writer.RecordAPICall("sonnet", true, 1500.0, 1000, 500, 0, 0, "")

	stats := writer.GetCallStats()
	if stats.TotalCalls != 1 {
		t.Errorf("Expected 1 call before reset, got %d", stats.TotalCalls)
	}

	writer.Reset()

	stats = writer.GetCallStats()
	if stats.TotalCalls != 0 {
		t.Errorf("Expected 0 calls after reset, got %d", stats.TotalCalls)
	}
}

func TestDefaultWriterConfig(t *testing.T) {
	config := DefaultWriterConfig()

	if config.CollectionInterval != 10*time.Second {
		t.Errorf("Expected CollectionInterval 10s, got %v", config.CollectionInterval)
	}

	if config.MaxSamples != 50 {
		t.Errorf("Expected MaxSamples 50, got %d", config.MaxSamples)
	}

	if !config.EnableSystemMetrics {
		t.Error("Expected EnableSystemMetrics to be true")
	}

	if config.SystemMetricsInterval != 30*time.Second {
		t.Errorf("Expected SystemMetricsInterval 30s, got %v", config.SystemMetricsInterval)
	}
}

func TestMetricsWriter_MultipleModels(t *testing.T) {
	store := NewMockStore()
	writer := NewMetricsWriter(store, nil)

	// Record calls for multiple models
	models := []string{"sonnet", "opus", "haiku", "gpt-4o"}
	for _, model := range models {
		writer.RecordAPICall(model, true, 1000.0, 500, 250, 0, 0, "")
	}

	stats := writer.GetCallStats()
	if stats.TotalCalls != 4 {
		t.Errorf("Expected 4 total calls, got %d", stats.TotalCalls)
	}

	modelStats := writer.GetModelStats()
	if len(modelStats) != 4 {
		t.Errorf("Expected 4 models, got %d", len(modelStats))
	}
}

func TestMetricsWriter_RecordCounter(t *testing.T) {
	store := NewMockStore()
	writer := NewMetricsWriter(store, nil)

	writer.RecordCounter("grounding_fallback_total", 2, map[string]string{
		"route_kind": "chat",
		"status":     "fallback",
	})

	points := store.GetPoints()
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	if points[0].Measurement != MeasurementCounters {
		t.Fatalf("measurement = %q, want %q", points[0].Measurement, MeasurementCounters)
	}
	if points[0].Tags[TagMetric] != "grounding_fallback_total" {
		t.Fatalf("metric tag = %q, want grounding_fallback_total", points[0].Tags[TagMetric])
	}
	if points[0].Fields[FieldCount] != int64(2) {
		t.Fatalf("count field = %v, want 2", points[0].Fields[FieldCount])
	}
	if points[0].Tags["route_kind"] != "chat" {
		t.Fatalf("route_kind tag = %q, want chat", points[0].Tags["route_kind"])
	}
}
