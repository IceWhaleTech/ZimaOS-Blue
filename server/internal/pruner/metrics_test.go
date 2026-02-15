package pruner

import "testing"

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		min  int
		max  int
	}{
		{"empty", "", 0, 0},
		{"short ascii", "hello world", 1, 5},
		{"go code", "func main() {\n\tfmt.Println(\"hello\")\n}", 5, 15},
		{"cjk text", "你好世界这是一段中文", 5, 10},
		{"mixed", "Hello 你好 World 世界", 3, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.text)
			if got < tt.min || got > tt.max {
				t.Errorf("EstimateTokens(%q) = %d, want [%d, %d]", tt.text, got, tt.min, tt.max)
			}
		})
	}
}

func TestStats_Snapshot(t *testing.T) {
	s := NewStats()
	s.Record(&PruneResponse{OriginalTokens: 100, PrunedTokens: 60, LatencyMs: 10.0})
	s.Record(&PruneResponse{OriginalTokens: 200, PrunedTokens: 80, LatencyMs: 20.0})
	s.RecordPassthrough()

	snap := s.Snapshot()
	if snap.TotalRequests != 3 {
		t.Errorf("expected 3 total requests, got %d", snap.TotalRequests)
	}
	if snap.PrunedRequests != 2 {
		t.Errorf("expected 2 pruned requests, got %d", snap.PrunedRequests)
	}
	if snap.PassthroughRequests != 1 {
		t.Errorf("expected 1 passthrough, got %d", snap.PassthroughRequests)
	}
	if snap.TokensSaved != 160 {
		t.Errorf("expected 160 tokens saved, got %d", snap.TokensSaved)
	}
	if snap.AvgCompressionRate < 0.4 || snap.AvgCompressionRate > 0.5 {
		t.Errorf("expected avg compression ~0.47, got %f", snap.AvgCompressionRate)
	}
}
