package stats

import (
	"math"
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

func TestStatisticsCollector_RecordMediaEvent(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, true)
	defer collector.Flush()
	collector.SetSyncPersist(true)

	event := &MediaEvent{
		Provider:   "mulerouter",
		Model:      "qwen-image-max",
		Type:       "image",
		Category:   "t2i",
		ImageCount: 2,
		CostUSD:    0.06,
		Success:    true,
		LatencyMs:  3000,
	}

	err := collector.RecordMediaEvent(event)
	if err != nil {
		t.Fatalf("RecordMediaEvent() error = %v", err)
	}

	if collector.GetMediaEventCount() != 1 {
		t.Errorf("GetMediaEventCount() = %d, want 1", collector.GetMediaEventCount())
	}
}

func TestStatisticsCollector_RecordMediaEvent_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, false)
	defer collector.Flush()

	err := collector.RecordMediaEvent(&MediaEvent{
		Provider: "mulerouter",
		Model:    "qwen-image-max",
		CostUSD:  0.03,
		Success:  true,
	})
	if err != nil {
		t.Fatalf("RecordMediaEvent() error = %v", err)
	}

	if collector.GetMediaEventCount() != 0 {
		t.Errorf("GetMediaEventCount() = %d, want 0 (disabled)", collector.GetMediaEventCount())
	}
}

func TestStatisticsCollector_MediaCostAggregation(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, true)
	defer collector.Flush()
	collector.SetSyncPersist(true)

	// Record media events
	events := []MediaEvent{
		{Provider: "mulerouter", Model: "qwen-image-max", Type: "image", ImageCount: 1, CostUSD: 0.03, Success: true},
		{Provider: "mulerouter", Model: "nano-banana-pro", Type: "image", ImageCount: 1, CostUSD: 0.15, Success: true},
		{Provider: "mulerouter", Model: "wan2.6-t2v", Type: "video", DurationSec: 5, CostUSD: 0.50, Success: true},
		{Provider: "mulerouter", Model: "qwen-image-max", Type: "image", ImageCount: 1, CostUSD: 0.03, Success: false}, // failed — should not count
	}

	for _, e := range events {
		event := e
		collector.RecordMediaEvent(&event)
	}

	stats, err := collector.GetStats("all")
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	// Only successful events count
	if stats.MediaCalls != 3 {
		t.Errorf("MediaCalls = %d, want 3", stats.MediaCalls)
	}

	expectedMediaCost := 0.03 + 0.15 + 0.50
	if math.Abs(stats.MediaCostUSD-expectedMediaCost) > 1e-9 {
		t.Errorf("MediaCostUSD = %f, want %f", stats.MediaCostUSD, expectedMediaCost)
	}

	// Media cost should be included in total estimated cost
	if math.Abs(stats.EstimatedCostUSD-expectedMediaCost) > 1e-9 {
		t.Errorf("EstimatedCostUSD = %f, want %f (media only)", stats.EstimatedCostUSD, expectedMediaCost)
	}

	// Check per-model breakdown
	if math.Abs(stats.MediaCostByModel["qwen-image-max"]-0.03) > 1e-9 {
		t.Errorf("MediaCostByModel[qwen-image-max] = %f, want 0.03", stats.MediaCostByModel["qwen-image-max"])
	}
	if stats.MediaCostByModel["wan2.6-t2v"] != 0.50 {
		t.Errorf("MediaCostByModel[wan2.6-t2v] = %f, want 0.50", stats.MediaCostByModel["wan2.6-t2v"])
	}
}

func TestStatisticsCollector_MediaEventPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create collector and record events
	collector := NewStatisticsCollector(tmpDir, true)
	collector.SetSyncPersist(true)

	collector.RecordMediaEvent(&MediaEvent{
		Provider:   "mulerouter",
		Model:      "qwen-image-max",
		Type:       "image",
		ImageCount: 1,
		CostUSD:    0.03,
		Success:    true,
	})
	collector.RecordMediaEvent(&MediaEvent{
		Provider:    "mulerouter",
		Model:       "wan2.6-t2v",
		Type:        "video",
		DurationSec: 5,
		CostUSD:     0.50,
		Success:     true,
	})
	collector.Flush()

	// Create new collector from same dir — should load persisted events
	collector2 := NewStatisticsCollector(tmpDir, true)
	defer collector2.Flush()

	if collector2.GetMediaEventCount() != 2 {
		t.Errorf("GetMediaEventCount() after reload = %d, want 2", collector2.GetMediaEventCount())
	}
}

func TestStatisticsCollector_ClearIncludesMedia(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, true)
	defer collector.Flush()
	collector.SetSyncPersist(true)

	collector.RecordMediaEvent(&MediaEvent{
		Provider: "mulerouter",
		Model:    "qwen-image-max",
		CostUSD:  0.03,
		Success:  true,
	})

	if collector.GetMediaEventCount() != 1 {
		t.Fatal("expected 1 media event before clear")
	}

	collector.Clear()

	if collector.GetMediaEventCount() != 0 {
		t.Errorf("GetMediaEventCount() after Clear() = %d, want 0", collector.GetMediaEventCount())
	}
}
