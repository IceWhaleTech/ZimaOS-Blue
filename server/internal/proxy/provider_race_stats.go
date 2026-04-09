package proxy

import (
	"sync/atomic"
	"time"
)

// ProviderRaceStats tracks runtime effectiveness of provider racing.
type ProviderRaceStats struct {
	requestsTotal           int64
	successfulRaces         int64
	hits                    int64
	winnerLatencyTotalNs    int64
	estimatedSavedTotalNs   int64
	estimatedSavingsSamples int64
}

// ProviderRaceStatsSnapshot is a point-in-time copy for APIs/UI.
type ProviderRaceStatsSnapshot struct {
	RequestsTotal             int64   `json:"requests_total"`
	SuccessfulRaces           int64   `json:"successful_races"`
	Hits                      int64   `json:"hits"`
	HitRate                   float64 `json:"hit_rate"`
	AvgWinnerLatencyMs        float64 `json:"avg_winner_latency_ms"`
	EstimatedLatencySavedMs   float64 `json:"estimated_latency_saved_ms"`
	EstimatedSavingsSamples   int64   `json:"estimated_savings_samples"`
}

func newProviderRaceStats() *ProviderRaceStats {
	return &ProviderRaceStats{}
}

func (s *ProviderRaceStats) RecordRequest() {
	if s == nil {
		return
	}
	atomic.AddInt64(&s.requestsTotal, 1)
}

func (s *ProviderRaceStats) RecordSuccess(primaryProviderID, winnerProviderID string, winnerLatency, estimatedSaved time.Duration) {
	if s == nil {
		return
	}

	atomic.AddInt64(&s.successfulRaces, 1)
	if winnerLatency > 0 {
		atomic.AddInt64(&s.winnerLatencyTotalNs, int64(winnerLatency))
	}
	if primaryProviderID != "" && winnerProviderID != "" && primaryProviderID != winnerProviderID {
		atomic.AddInt64(&s.hits, 1)
	}
	if estimatedSaved > 0 {
		atomic.AddInt64(&s.estimatedSavedTotalNs, int64(estimatedSaved))
		atomic.AddInt64(&s.estimatedSavingsSamples, 1)
	}
}

func (s *ProviderRaceStats) Snapshot() ProviderRaceStatsSnapshot {
	if s == nil {
		return ProviderRaceStatsSnapshot{}
	}

	requestsTotal := atomic.LoadInt64(&s.requestsTotal)
	successfulRaces := atomic.LoadInt64(&s.successfulRaces)
	hits := atomic.LoadInt64(&s.hits)
	estimatedSamples := atomic.LoadInt64(&s.estimatedSavingsSamples)

	snapshot := ProviderRaceStatsSnapshot{
		RequestsTotal:           requestsTotal,
		SuccessfulRaces:         successfulRaces,
		Hits:                    hits,
		EstimatedSavingsSamples: estimatedSamples,
	}
	if requestsTotal > 0 {
		snapshot.HitRate = float64(hits) / float64(requestsTotal)
	}
	if successfulRaces > 0 {
		snapshot.AvgWinnerLatencyMs = float64(atomic.LoadInt64(&s.winnerLatencyTotalNs)) /
			float64(successfulRaces) / float64(time.Millisecond)
	}
	if estimatedSamples > 0 {
		snapshot.EstimatedLatencySavedMs = float64(atomic.LoadInt64(&s.estimatedSavedTotalNs)) /
			float64(estimatedSamples) / float64(time.Millisecond)
	}
	return snapshot
}
