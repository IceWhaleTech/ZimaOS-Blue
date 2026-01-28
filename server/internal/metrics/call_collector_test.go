package metrics

import (
	"testing"
	"time"
)

func TestCallCollector_RecordCall(t *testing.T) {
	collector := NewCallCollector(1000, time.Hour)

	// Record a successful call
	collector.RecordCall(CallRecord{
		Model:       "sonnet",
		Success:     true,
		LatencyMs:   1500,
		InputTokens: 100,
		OutputTokens: 50,
	})

	stats := collector.GetCallStats()
	if stats.TotalCalls != 1 {
		t.Errorf("Expected 1 total call, got %d", stats.TotalCalls)
	}
	if stats.SuccessfulCalls != 1 {
		t.Errorf("Expected 1 successful call, got %d", stats.SuccessfulCalls)
	}
	if stats.SuccessRate != 100 {
		t.Errorf("Expected 100%% success rate, got %.2f%%", stats.SuccessRate)
	}
}

func TestCallCollector_RecordFailedCall(t *testing.T) {
	collector := NewCallCollector(1000, time.Hour)

	// Record a failed call
	collector.RecordCall(CallRecord{
		Model:     "opus",
		Success:   false,
		ErrorType: ErrorTypeTimeout,
		LatencyMs: 30000,
	})

	stats := collector.GetCallStats()
	if stats.TotalCalls != 1 {
		t.Errorf("Expected 1 total call, got %d", stats.TotalCalls)
	}
	if stats.FailedCalls != 1 {
		t.Errorf("Expected 1 failed call, got %d", stats.FailedCalls)
	}
	if stats.TimeoutCalls != 1 {
		t.Errorf("Expected 1 timeout call, got %d", stats.TimeoutCalls)
	}
	if stats.SuccessRate != 0 {
		t.Errorf("Expected 0%% success rate, got %.2f%%", stats.SuccessRate)
	}
}

func TestCallCollector_GetModelStats(t *testing.T) {
	collector := NewCallCollector(1000, time.Hour)

	// Record calls for different models
	collector.RecordCall(CallRecord{
		Model:       "sonnet",
		Success:     true,
		LatencyMs:   1500,
		InputTokens: 100,
		OutputTokens: 50,
	})
	collector.RecordCall(CallRecord{
		Model:       "sonnet",
		Success:     true,
		LatencyMs:   2000,
		InputTokens: 200,
		OutputTokens: 100,
	})
	collector.RecordCall(CallRecord{
		Model:       "opus",
		Success:     true,
		LatencyMs:   3000,
		InputTokens: 500,
		OutputTokens: 250,
	})

	modelStats := collector.GetModelStats()
	if len(modelStats) != 2 {
		t.Errorf("Expected 2 models, got %d", len(modelStats))
	}

	// Check sonnet stats (should be first due to more calls)
	sonnetStats := modelStats[0]
	if sonnetStats.Model != "sonnet" {
		t.Errorf("Expected sonnet to be first, got %s", sonnetStats.Model)
	}
	if sonnetStats.Calls != 2 {
		t.Errorf("Expected 2 calls for sonnet, got %d", sonnetStats.Calls)
	}
	if sonnetStats.InputTokens != 300 {
		t.Errorf("Expected 300 input tokens for sonnet, got %d", sonnetStats.InputTokens)
	}
	if sonnetStats.AvgLatency != 1750 {
		t.Errorf("Expected 1750ms avg latency for sonnet, got %.2f", sonnetStats.AvgLatency)
	}
}

func TestCallCollector_GetModelStatsByName(t *testing.T) {
	collector := NewCallCollector(1000, time.Hour)

	collector.RecordCall(CallRecord{
		Model:       "haiku",
		Success:     true,
		LatencyMs:   500,
		InputTokens: 50,
		OutputTokens: 25,
	})

	stats := collector.GetModelStatsByName("haiku")
	if stats == nil {
		t.Fatal("Expected stats for haiku, got nil")
	}
	if stats.Calls != 1 {
		t.Errorf("Expected 1 call, got %d", stats.Calls)
	}

	// Non-existent model
	stats = collector.GetModelStatsByName("nonexistent")
	if stats != nil {
		t.Error("Expected nil for non-existent model")
	}
}

func TestCallCollector_Percentile(t *testing.T) {
	tests := []struct {
		data     []float64
		p        float64
		expected float64
	}{
		{[]float64{1, 2, 3, 4, 5}, 50, 3},
		{[]float64{1, 2, 3, 4, 5}, 0, 1},
		{[]float64{1, 2, 3, 4, 5}, 100, 5},
		{[]float64{1}, 50, 1},
		{[]float64{}, 50, 0},
	}

	for _, tt := range tests {
		result := percentile(tt.data, tt.p)
		if result != tt.expected {
			t.Errorf("percentile(%v, %.0f) = %.2f, expected %.2f", tt.data, tt.p, result, tt.expected)
		}
	}
}

func TestCallCollector_Reset(t *testing.T) {
	collector := NewCallCollector(1000, time.Hour)

	collector.RecordCall(CallRecord{
		Model:   "sonnet",
		Success: true,
	})

	stats := collector.GetCallStats()
	if stats.TotalCalls != 1 {
		t.Errorf("Expected 1 call before reset, got %d", stats.TotalCalls)
	}

	collector.Reset()

	stats = collector.GetCallStats()
	if stats.TotalCalls != 0 {
		t.Errorf("Expected 0 calls after reset, got %d", stats.TotalCalls)
	}
}

func TestCallCollector_TrimRecords(t *testing.T) {
	collector := NewCallCollector(5, time.Hour)

	// Add more records than max
	for i := 0; i < 10; i++ {
		collector.RecordCall(CallRecord{
			Model:   "sonnet",
			Success: true,
		})
	}

	collector.mu.RLock()
	recordCount := len(collector.records)
	collector.mu.RUnlock()

	if recordCount > 5 {
		t.Errorf("Expected at most 5 records, got %d", recordCount)
	}
}

func TestPeriodToDuration(t *testing.T) {
	tests := []struct {
		period   string
		expected time.Duration
	}{
		{PeriodRealtime, time.Minute},
		{PeriodHourly, time.Hour},
		{PeriodDaily, 24 * time.Hour},
		{PeriodWeekly, 7 * 24 * time.Hour},
		{PeriodMonthly, 30 * 24 * time.Hour},
		{"unknown", 24 * time.Hour}, // default
	}

	for _, tt := range tests {
		result := periodToDuration(tt.period)
		if result != tt.expected {
			t.Errorf("periodToDuration(%s) = %v, expected %v", tt.period, result, tt.expected)
		}
	}
}
