package stats

import (
	"testing"
	"time"
)

func TestStatisticsCollector_Record(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, true)
	defer collector.Flush()

	event := &APICallEvent{
		Provider:     "anthropic",
		Model:        "claude-3-sonnet",
		InputTokens:  100,
		OutputTokens: 200,
		LatencyMs:    500,
		Success:      true,
	}

	err := collector.Record(event)
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if collector.GetEventCount() != 1 {
		t.Errorf("GetEventCount() = %d, want 1", collector.GetEventCount())
	}
}

func TestStatisticsCollector_RecordDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, false)
	defer collector.Flush()

	event := &APICallEvent{
		Provider:     "anthropic",
		Model:        "claude-3-sonnet",
		InputTokens:  100,
		OutputTokens: 200,
	}

	err := collector.Record(event)
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if collector.GetEventCount() != 0 {
		t.Errorf("GetEventCount() = %d, want 0 (disabled)", collector.GetEventCount())
	}
}

func TestStatisticsCollector_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, true)
	defer collector.Flush()
	collector.SetSyncPersist(true)

	// Record some events
	events := []APICallEvent{
		{Provider: "anthropic", Model: "claude-3-sonnet", InputTokens: 100, OutputTokens: 200, LatencyMs: 500, Success: true},
		{Provider: "anthropic", Model: "claude-3-sonnet", InputTokens: 150, OutputTokens: 250, LatencyMs: 600, Success: true},
		{Provider: "openai", Model: "gpt-4o", InputTokens: 200, OutputTokens: 300, LatencyMs: 400, Success: true},
		{Provider: "openai", Model: "gpt-4o", InputTokens: 100, OutputTokens: 100, LatencyMs: 300, Success: false, ErrorType: "rate_limit"},
	}

	for _, e := range events {
		event := e
		collector.Record(&event)
	}

	stats, err := collector.GetStats("all")
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	if stats.TotalCalls != 4 {
		t.Errorf("TotalCalls = %d, want 4", stats.TotalCalls)
	}

	if stats.SuccessfulCalls != 3 {
		t.Errorf("SuccessfulCalls = %d, want 3", stats.SuccessfulCalls)
	}

	if stats.FailedCalls != 1 {
		t.Errorf("FailedCalls = %d, want 1", stats.FailedCalls)
	}

	if stats.CallsByProvider["anthropic"] != 2 {
		t.Errorf("CallsByProvider[anthropic] = %d, want 2", stats.CallsByProvider["anthropic"])
	}

	if stats.CallsByProvider["openai"] != 2 {
		t.Errorf("CallsByProvider[openai] = %d, want 2", stats.CallsByProvider["openai"])
	}

	if stats.InputTokens != 550 {
		t.Errorf("InputTokens = %d, want 550", stats.InputTokens)
	}

	if stats.OutputTokens != 850 {
		t.Errorf("OutputTokens = %d, want 850", stats.OutputTokens)
	}

	if stats.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", stats.ErrorCount)
	}

	if stats.ErrorsByType["rate_limit"] != 1 {
		t.Errorf("ErrorsByType[rate_limit] = %d, want 1", stats.ErrorsByType["rate_limit"])
	}
}

func TestStatisticsCollector_GetStatsByPeriod(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, true)
	defer collector.Flush()

	// Record an event
	event := &APICallEvent{
		Provider:     "anthropic",
		Model:        "claude-3-sonnet",
		InputTokens:  100,
		OutputTokens: 200,
		LatencyMs:    500,
		Success:      true,
		Timestamp:    time.Now().Add(-1 * time.Second),
	}
	collector.Record(event)

	// Test different periods
	periods := []string{"hour", "day", "week", "month", "all"}
	for _, period := range periods {
		stats, err := collector.GetStats(period)
		if err != nil {
			t.Errorf("GetStats(%q) error = %v", period, err)
		}
		if stats.TotalCalls != 1 {
			t.Errorf("GetStats(%q) TotalCalls = %d, want 1", period, stats.TotalCalls)
		}
	}
}

func TestStatisticsCollector_CalculateCost(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, true)
	defer collector.Flush()

	// Record events with known pricing
	events := []APICallEvent{
		{Provider: "anthropic", Model: "claude-3-sonnet", InputTokens: 1000000, OutputTokens: 1000000, Success: true},
	}

	for _, e := range events {
		event := e
		collector.Record(&event)
	}

	stats, _ := collector.GetStats("all")

	// claude-3-sonnet: $3/M input, $15/M output
	// Expected: $3 + $15 = $18
	expectedCost := 18.0
	if stats.EstimatedCostUSD != expectedCost {
		t.Errorf("EstimatedCostUSD = %f, want %f", stats.EstimatedCostUSD, expectedCost)
	}
}

func TestStatisticsCollector_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, true)
	defer collector.Flush()
	collector.SetSyncPersist(true)

	// Record some events
	for i := 0; i < 5; i++ {
		collector.Record(&APICallEvent{
			Provider:     "anthropic",
			Model:        "claude-3-sonnet",
			InputTokens:  100,
			OutputTokens: 200,
			Success:      true,
		})
	}

	if collector.GetEventCount() != 5 {
		t.Errorf("GetEventCount() = %d, want 5", collector.GetEventCount())
	}

	err := collector.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if collector.GetEventCount() != 0 {
		t.Errorf("GetEventCount() after Clear() = %d, want 0", collector.GetEventCount())
	}
}

func TestStatisticsCollector_GetRecentEvents(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, true)
	defer collector.Flush()
	collector.SetSyncPersist(true)

	// Record 10 events
	for i := 0; i < 10; i++ {
		collector.Record(&APICallEvent{
			Provider:     "anthropic",
			Model:        "claude-3-sonnet",
			InputTokens:  int64(i * 100),
			OutputTokens: 200,
			Success:      true,
		})
	}

	// Get last 5 events
	events := collector.GetRecentEvents(5)
	if len(events) != 5 {
		t.Errorf("len(GetRecentEvents(5)) = %d, want 5", len(events))
	}

	// Verify they are the most recent
	if events[0].InputTokens != 500 {
		t.Errorf("First event InputTokens = %d, want 500", events[0].InputTokens)
	}
}

func TestStatisticsCollector_LatencyPercentiles(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, true)
	defer collector.Flush()
	collector.SetSyncPersist(true)

	// Record events with varying latencies
	latencies := []int64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
	for _, lat := range latencies {
		collector.Record(&APICallEvent{
			Provider:     "anthropic",
			Model:        "claude-3-sonnet",
			InputTokens:  100,
			OutputTokens: 200,
			LatencyMs:    lat,
			Success:      true,
		})
	}

	stats, _ := collector.GetStats("all")

	if stats.MinLatencyMs != 100 {
		t.Errorf("MinLatencyMs = %d, want 100", stats.MinLatencyMs)
	}

	if stats.MaxLatencyMs != 1000 {
		t.Errorf("MaxLatencyMs = %d, want 1000", stats.MaxLatencyMs)
	}

	// Average should be 550
	if stats.AvgLatencyMs != 550 {
		t.Errorf("AvgLatencyMs = %f, want 550", stats.AvgLatencyMs)
	}
}

func TestStatisticsCollector_Export(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, true)
	defer collector.Flush()

	collector.Record(&APICallEvent{
		Provider:     "anthropic",
		Model:        "claude-3-sonnet",
		InputTokens:  100,
		OutputTokens: 200,
		Success:      true,
	})

	data, err := collector.Export("json")
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("Export() returned empty data")
	}
}
