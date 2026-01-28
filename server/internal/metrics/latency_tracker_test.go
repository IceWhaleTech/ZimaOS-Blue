package metrics

import (
	"testing"
	"time"
)

func TestLatencyTracker_RecordLatency(t *testing.T) {
	tracker := NewLatencyTracker(1000)

	tracker.RecordLatency("sonnet", 1500)
	tracker.RecordLatency("sonnet", 2000)
	tracker.RecordLatency("sonnet", 1000)

	stats := tracker.GetLatencyStats()
	if stats.Samples != 3 {
		t.Errorf("Expected 3 samples, got %d", stats.Samples)
	}
	if stats.Min != 1000 {
		t.Errorf("Expected min 1000, got %.2f", stats.Min)
	}
	if stats.Max != 2000 {
		t.Errorf("Expected max 2000, got %.2f", stats.Max)
	}
	if stats.Avg != 1500 {
		t.Errorf("Expected avg 1500, got %.2f", stats.Avg)
	}
}

func TestLatencyTracker_GetLatencyStatsByModel(t *testing.T) {
	tracker := NewLatencyTracker(1000)

	tracker.RecordLatency("sonnet", 1500)
	tracker.RecordLatency("opus", 3000)

	sonnetStats := tracker.GetLatencyStatsByModel("sonnet")
	if sonnetStats == nil {
		t.Fatal("Expected stats for sonnet, got nil")
	}
	if sonnetStats.Avg != 1500 {
		t.Errorf("Expected avg 1500 for sonnet, got %.2f", sonnetStats.Avg)
	}

	opusStats := tracker.GetLatencyStatsByModel("opus")
	if opusStats == nil {
		t.Fatal("Expected stats for opus, got nil")
	}
	if opusStats.Avg != 3000 {
		t.Errorf("Expected avg 3000 for opus, got %.2f", opusStats.Avg)
	}

	// Non-existent model
	unknownStats := tracker.GetLatencyStatsByModel("nonexistent")
	if unknownStats != nil {
		t.Error("Expected nil for non-existent model")
	}
}

func TestLatencyTracker_Percentiles(t *testing.T) {
	tracker := NewLatencyTracker(1000)

	// Add 100 samples
	for i := 1; i <= 100; i++ {
		tracker.RecordLatency("sonnet", float64(i*10))
	}

	stats := tracker.GetLatencyStats()

	// P50 should be around 500
	if stats.P50 < 490 || stats.P50 > 510 {
		t.Errorf("Expected P50 around 500, got %.2f", stats.P50)
	}

	// P95 should be around 950
	if stats.P95 < 940 || stats.P95 > 960 {
		t.Errorf("Expected P95 around 950, got %.2f", stats.P95)
	}

	// P99 should be around 990
	if stats.P99 < 980 || stats.P99 > 1000 {
		t.Errorf("Expected P99 around 990, got %.2f", stats.P99)
	}
}

func TestLatencyTracker_RecordSpeed(t *testing.T) {
	tracker := NewLatencyTracker(1000)

	tracker.RecordSpeed("sonnet", 50.0, 200.0, 55.0)
	tracker.RecordSpeed("sonnet", 60.0, 180.0, 65.0)

	stats := tracker.GetSpeedStats()
	if stats.TokensPerSecond != 60.0 {
		t.Errorf("Expected current TPS 60.0, got %.2f", stats.TokensPerSecond)
	}
	if stats.AvgTokensPerSecond != 55.0 {
		t.Errorf("Expected avg TPS 55.0, got %.2f", stats.AvgTokensPerSecond)
	}
	if stats.MaxTokensPerSecond != 60.0 {
		t.Errorf("Expected max TPS 60.0, got %.2f", stats.MaxTokensPerSecond)
	}
}

func TestLatencyTracker_GetSpeedStatsByModel(t *testing.T) {
	tracker := NewLatencyTracker(1000)

	tracker.RecordSpeed("sonnet", 50.0, 200.0, 55.0)
	tracker.RecordSpeed("opus", 35.0, 350.0, 40.0)

	sonnetStats := tracker.GetSpeedStatsByModel("sonnet")
	if sonnetStats == nil {
		t.Fatal("Expected stats for sonnet, got nil")
	}
	if sonnetStats.TokensPerSecond != 50.0 {
		t.Errorf("Expected TPS 50.0 for sonnet, got %.2f", sonnetStats.TokensPerSecond)
	}

	opusStats := tracker.GetSpeedStatsByModel("opus")
	if opusStats == nil {
		t.Fatal("Expected stats for opus, got nil")
	}
	if opusStats.TokensPerSecond != 35.0 {
		t.Errorf("Expected TPS 35.0 for opus, got %.2f", opusStats.TokensPerSecond)
	}
}

func TestLatencyTracker_GetSpeedPercentiles(t *testing.T) {
	tracker := NewLatencyTracker(1000)

	// Add samples
	for i := 1; i <= 100; i++ {
		tracker.RecordSpeed("sonnet", float64(i), float64(i*10), float64(i))
	}

	percentiles := tracker.GetSpeedPercentiles()

	// TPS P50 should be around 50
	if percentiles.TPSP50 < 45 || percentiles.TPSP50 > 55 {
		t.Errorf("Expected TPS P50 around 50, got %.2f", percentiles.TPSP50)
	}

	// TTFT P50 should be around 500
	if percentiles.TTFTP50Ms < 490 || percentiles.TTFTP50Ms > 510 {
		t.Errorf("Expected TTFT P50 around 500, got %.2f", percentiles.TTFTP50Ms)
	}
}

func TestLatencyTracker_Reset(t *testing.T) {
	tracker := NewLatencyTracker(1000)

	tracker.RecordLatency("sonnet", 1500)
	tracker.RecordSpeed("sonnet", 50.0, 200.0, 55.0)

	stats := tracker.GetLatencyStats()
	if stats.Samples != 1 {
		t.Errorf("Expected 1 sample before reset, got %d", stats.Samples)
	}

	tracker.Reset()

	stats = tracker.GetLatencyStats()
	if stats.Samples != 0 {
		t.Errorf("Expected 0 samples after reset, got %d", stats.Samples)
	}
}

func TestLatencyTracker_GetModelSpeedResponses(t *testing.T) {
	tracker := NewLatencyTracker(1000)

	tracker.RecordSpeed("sonnet", 50.0, 200.0, 55.0)
	tracker.RecordSpeed("opus", 35.0, 350.0, 40.0)
	tracker.RecordSpeed("haiku", 75.0, 120.0, 80.0)

	responses := tracker.GetModelSpeedResponses()
	if len(responses) != 3 {
		t.Errorf("Expected 3 models, got %d", len(responses))
	}

	// Should be sorted by model name
	if responses[0].Model != "haiku" {
		t.Errorf("Expected first model to be haiku, got %s", responses[0].Model)
	}
}

func TestTimeBreakdownTracker_RecordBreakdown(t *testing.T) {
	tracker := NewTimeBreakdownTracker(1000)

	breakdown := TimeBreakdown{
		QueueTime:        100 * time.Millisecond,
		TimeToFirstToken: 200 * time.Millisecond,
		GenerationTime:   500 * time.Millisecond,
		TotalTime:        800 * time.Millisecond,
	}

	tracker.RecordBreakdown("sonnet", breakdown)

	avg := tracker.GetAverageBreakdown()
	if avg.TotalTime != 800*time.Millisecond {
		t.Errorf("Expected total time 800ms, got %v", avg.TotalTime)
	}
}

func TestTimeBreakdownTracker_GetAverageBreakdownByModel(t *testing.T) {
	tracker := NewTimeBreakdownTracker(1000)

	tracker.RecordBreakdown("sonnet", TimeBreakdown{
		QueueTime:        100 * time.Millisecond,
		TimeToFirstToken: 200 * time.Millisecond,
		GenerationTime:   500 * time.Millisecond,
		TotalTime:        800 * time.Millisecond,
	})

	tracker.RecordBreakdown("sonnet", TimeBreakdown{
		QueueTime:        200 * time.Millisecond,
		TimeToFirstToken: 300 * time.Millisecond,
		GenerationTime:   700 * time.Millisecond,
		TotalTime:        1200 * time.Millisecond,
	})

	avg := tracker.GetAverageBreakdownByModel("sonnet")
	if avg == nil {
		t.Fatal("Expected breakdown for sonnet, got nil")
	}

	expectedTotal := 1000 * time.Millisecond // (800 + 1200) / 2
	if avg.TotalTime != expectedTotal {
		t.Errorf("Expected avg total time %v, got %v", expectedTotal, avg.TotalTime)
	}

	// Non-existent model
	unknownAvg := tracker.GetAverageBreakdownByModel("nonexistent")
	if unknownAvg != nil {
		t.Error("Expected nil for non-existent model")
	}
}

func TestTimeBreakdownTracker_Reset(t *testing.T) {
	tracker := NewTimeBreakdownTracker(1000)

	tracker.RecordBreakdown("sonnet", TimeBreakdown{
		TotalTime: 800 * time.Millisecond,
	})

	avg := tracker.GetAverageBreakdown()
	if avg.TotalTime != 800*time.Millisecond {
		t.Errorf("Expected total time 800ms before reset, got %v", avg.TotalTime)
	}

	tracker.Reset()

	avg = tracker.GetAverageBreakdown()
	if avg.TotalTime != 0 {
		t.Errorf("Expected total time 0 after reset, got %v", avg.TotalTime)
	}
}
