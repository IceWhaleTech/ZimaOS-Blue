package providerpool

import (
	"os"
	"testing"
	"time"
)

func setupTrackerTest(t *testing.T) (*UsageTracker, func()) {
	tmpDir, err := os.MkdirTemp("", "tracker-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	storage, err := NewFileStorage(tmpDir, nil)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create storage: %v", err)
	}

	tracker := NewUsageTracker(storage, WithUsageFlushInterval(100*time.Millisecond), WithUsageBufferSize(100))

	cleanup := func() {
		tracker.Stop()
		os.RemoveAll(tmpDir)
	}

	return tracker, cleanup
}

func TestUsageTrackerRecord(t *testing.T) {
	tracker, cleanup := setupTrackerTest(t)
	defer cleanup()

	tracker.Start()

	// Record some usage
	tracker.RecordRequest("openai", "gpt-4", 1000, 500, 250, true, 0.05)
	tracker.RecordRequest("openai", "gpt-4", 2000, 1000, 300, true, 0.10)
	tracker.RecordRequest("anthropic", "claude-3", 1500, 750, 200, true, 0.08)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Check current stats
	stats := tracker.GetCurrentStats()

	if len(stats) != 2 {
		t.Errorf("Expected 2 provider:model combinations, got %d", len(stats))
	}

	openaiStats := stats["openai:gpt-4"]
	if openaiStats == nil {
		t.Fatal("Expected openai:gpt-4 stats")
	}

	if openaiStats.TotalInputTokens != 3000 {
		t.Errorf("Expected 3000 input tokens, got %d", openaiStats.TotalInputTokens)
	}

	if openaiStats.TotalOutputTokens != 1500 {
		t.Errorf("Expected 1500 output tokens, got %d", openaiStats.TotalOutputTokens)
	}

	if openaiStats.TotalRequests != 2 {
		t.Errorf("Expected 2 requests, got %d", openaiStats.TotalRequests)
	}
}

func TestUsageTrackerGetUsage(t *testing.T) {
	tracker, cleanup := setupTrackerTest(t)
	defer cleanup()

	tracker.Start()

	now := time.Now()

	// Record usage
	tracker.Record(&UsageRecord{
		ProviderID:   "openai",
		ModelID:      "gpt-4",
		Timestamp:    now,
		InputTokens:  1000,
		OutputTokens: 500,
		RequestCount: 1,
		LatencyMs:    250,
		Success:      true,
	})

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Get usage
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	records, err := tracker.GetUsage("openai", start, end)
	if err != nil {
		t.Fatalf("GetUsage failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}

func TestUsageTrackerGetSummary(t *testing.T) {
	tracker, cleanup := setupTrackerTest(t)
	defer cleanup()

	tracker.Start()

	now := time.Now()

	// Record multiple usage records
	for i := 0; i < 5; i++ {
		tracker.Record(&UsageRecord{
			ProviderID:    "openai",
			ModelID:       "gpt-4",
			Timestamp:     now,
			InputTokens:   1000,
			OutputTokens:  500,
			RequestCount:  1,
			LatencyMs:     int64(200 + i*50),
			Success:       i < 4, // 4 success, 1 failure
			EstimatedCost: 0.05,
		})
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Get summary
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	summary, err := tracker.GetSummary("openai", start, end)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if summary.TotalInputTokens != 5000 {
		t.Errorf("Expected 5000 input tokens, got %d", summary.TotalInputTokens)
	}

	if summary.TotalRequests != 5 {
		t.Errorf("Expected 5 requests, got %d", summary.TotalRequests)
	}

	if summary.SuccessfulRequests != 4 {
		t.Errorf("Expected 4 successful requests, got %d", summary.SuccessfulRequests)
	}

	if summary.FailedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", summary.FailedRequests)
	}
}

func TestUsageTrackerFlush(t *testing.T) {
	tracker, cleanup := setupTrackerTest(t)
	defer cleanup()

	tracker.Start()

	// Record usage
	tracker.RecordRequest("openai", "gpt-4", 1000, 500, 250, true, 0.05)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Check stats exist
	stats := tracker.GetCurrentStats()
	if len(stats) != 1 {
		t.Errorf("Expected 1 stat, got %d", len(stats))
	}

	// Trigger flush
	tracker.flush()

	// Stats should be cleared
	stats = tracker.GetCurrentStats()
	if len(stats) != 0 {
		t.Errorf("Expected 0 stats after flush, got %d", len(stats))
	}
}
