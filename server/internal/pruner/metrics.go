package pruner

import (
	"sync/atomic"
)

// Stats tracks cumulative pruning metrics (thread-safe).
type Stats struct {
	totalRequests       atomic.Int64
	prunedRequests      atomic.Int64
	passthroughRequests atomic.Int64
	totalTokensBefore   atomic.Int64
	totalTokensAfter    atomic.Int64
	totalLatencyUs      atomic.Int64 // microseconds
}

// NewStats creates a new Stats instance.
func NewStats() *Stats {
	return &Stats{}
}

// Record adds a single prune result to the cumulative stats.
func (s *Stats) Record(r *PruneResponse) {
	s.totalRequests.Add(1)
	s.prunedRequests.Add(1)
	s.totalTokensBefore.Add(int64(r.OriginalTokens))
	s.totalTokensAfter.Add(int64(r.PrunedTokens))
	s.totalLatencyUs.Add(int64(r.LatencyMs * 1000))
}

// RecordPassthrough records a request that was not pruned.
func (s *Stats) RecordPassthrough() {
	s.totalRequests.Add(1)
	s.passthroughRequests.Add(1)
}

// Snapshot returns a point-in-time copy of the stats.
type StatsSnapshot struct {
	TotalRequests       int64   `json:"total_requests"`
	PrunedRequests      int64   `json:"pruned_requests"`
	PassthroughRequests int64   `json:"passthrough_requests"`
	TotalTokensBefore   int64   `json:"total_tokens_before"`
	TotalTokensAfter    int64   `json:"total_tokens_after"`
	TokensSaved         int64   `json:"tokens_saved"`
	AvgCompressionRate  float64 `json:"avg_compression_rate"`
	AvgLatencyMs        float64 `json:"avg_latency_ms"`
}

// Snapshot returns a point-in-time copy of the pruning stats.
func (s *Stats) Snapshot() StatsSnapshot {
	total := s.totalRequests.Load()
	pruned := s.prunedRequests.Load()
	before := s.totalTokensBefore.Load()
	after := s.totalTokensAfter.Load()
	latencyUs := s.totalLatencyUs.Load()

	var avgCompression float64
	if before > 0 {
		avgCompression = float64(after) / float64(before)
	}
	var avgLatency float64
	if pruned > 0 {
		avgLatency = float64(latencyUs) / float64(pruned) / 1000.0
	}

	return StatsSnapshot{
		TotalRequests:       total,
		PrunedRequests:      pruned,
		PassthroughRequests: s.passthroughRequests.Load(),
		TotalTokensBefore:   before,
		TotalTokensAfter:    after,
		TokensSaved:         before - after,
		AvgCompressionRate:  avgCompression,
		AvgLatencyMs:        avgLatency,
	}
}
