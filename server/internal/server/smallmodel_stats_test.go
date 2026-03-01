package server

import (
	"testing"
	"time"
)

func TestSmallModelStatsRecordAndReset(t *testing.T) {
	s := NewSmallModelStats()
	s.RecordShortQARoute(true)
	s.RecordShortQARoute(false)
	s.RecordToolDispatchRoute(true)
	s.RecordToolDispatchRoute(false)
	s.RecordSummaryAttempt()
	s.RecordSummarySuccess()
	s.RecordSummaryAttempt()
	s.RecordDocExtractAttempt()
	s.RecordDocExtractSuccess()
	s.RecordDocExtractAttempt()
	s.RecordLatencyWithScene("short_qa", 20*time.Millisecond)
	s.RecordLatencyWithScene("tool_dispatch", 40*time.Millisecond)
	s.RecordLatencyWithScene("summary", 50*time.Millisecond)
	s.RecordLatencyWithScene("doc_extract", 30*time.Millisecond)
	s.RecordShadowQualityDelta(0.1)
	s.RecordShadowQualityDelta(0.5)
	s.RecordFallback("model_unready")
	s.RecordFallback("timeout")
	s.RecordShadow("short_qa_shadow", true)
	s.RecordShadow("tool_dispatch_shadow", false)
	s.RecordAutoRollback()
	s.RecordDeepResearchFallback()
	s.RecordIRTakeover()

	snap := s.Snapshot()
	if snap.ShortQARouteAttempts != 2 {
		t.Fatalf("ShortQARouteAttempts = %d, want 2", snap.ShortQARouteAttempts)
	}
	if snap.ShortQARouteSuccess != 1 {
		t.Fatalf("ShortQARouteSuccess = %d, want 1", snap.ShortQARouteSuccess)
	}
	if snap.ToolDispatchRouteAttempts != 2 {
		t.Fatalf("ToolDispatchRouteAttempts = %d, want 2", snap.ToolDispatchRouteAttempts)
	}
	if snap.ToolDispatchRouteSuccess != 1 {
		t.Fatalf("ToolDispatchRouteSuccess = %d, want 1", snap.ToolDispatchRouteSuccess)
	}
	if snap.SummaryAttempts != 2 || snap.SummarySuccess != 1 {
		t.Fatalf("unexpected summary counters: attempts=%d success=%d", snap.SummaryAttempts, snap.SummarySuccess)
	}
	if snap.DocExtractAttempts != 2 || snap.DocExtractSuccess != 1 {
		t.Fatalf("unexpected doc_extract counters: attempts=%d success=%d", snap.DocExtractAttempts, snap.DocExtractSuccess)
	}
	if snap.ShortQAShadowTotal != 1 || snap.ToolShadowTotal != 1 {
		t.Fatalf("unexpected shadow totals: %+v", snap)
	}
	if snap.ShadowFailures != 1 {
		t.Fatalf("ShadowFailures = %d, want 1", snap.ShadowFailures)
	}
	if snap.FallbackTotal != 2 {
		t.Fatalf("FallbackTotal = %d, want 2", snap.FallbackTotal)
	}
	if snap.TimeoutTotal != 1 {
		t.Fatalf("TimeoutTotal = %d, want 1", snap.TimeoutTotal)
	}
	if snap.LatencySamples != 4 || snap.LatencyMsTotal != 140 {
		t.Fatalf("unexpected latency counters: samples=%d total=%d", snap.LatencySamples, snap.LatencyMsTotal)
	}
	if snap.LatencyMs != 35 {
		t.Fatalf("LatencyMs = %v, want 35", snap.LatencyMs)
	}
	if snap.ShortQALatencySamples != 1 || snap.ShortQALatencyMs != 20 {
		t.Fatalf("unexpected short_qa latency stats: samples=%d avg=%v", snap.ShortQALatencySamples, snap.ShortQALatencyMs)
	}
	if snap.ToolDispatchLatencySamples != 1 || snap.ToolDispatchLatencyMs != 40 {
		t.Fatalf("unexpected tool_dispatch latency stats: samples=%d avg=%v", snap.ToolDispatchLatencySamples, snap.ToolDispatchLatencyMs)
	}
	if snap.SummaryLatencySamples != 1 || snap.SummaryLatencyMs != 50 {
		t.Fatalf("unexpected summary latency stats: samples=%d avg=%v", snap.SummaryLatencySamples, snap.SummaryLatencyMs)
	}
	if snap.DocExtractLatencySamples != 1 || snap.DocExtractLatencyMs != 30 {
		t.Fatalf("unexpected doc_extract latency stats: samples=%d avg=%v", snap.DocExtractLatencySamples, snap.DocExtractLatencyMs)
	}
	if snap.ShadowQualitySamples != 2 {
		t.Fatalf("ShadowQualitySamples = %d, want 2", snap.ShadowQualitySamples)
	}
	if snap.ShadowQualityDelta != 0.3 {
		t.Fatalf("ShadowQualityDelta = %v, want 0.3", snap.ShadowQualityDelta)
	}
	if snap.DeepResearchFallback != 1 {
		t.Fatalf("DeepResearchFallback = %d, want 1", snap.DeepResearchFallback)
	}
	if snap.AutoRollbackTotal != 1 {
		t.Fatalf("AutoRollbackTotal = %d, want 1", snap.AutoRollbackTotal)
	}
	if snap.IRTakeover != 1 {
		t.Fatalf("IRTakeover = %d, want 1", snap.IRTakeover)
	}
	if snap.FallbackReasons["model_unready"] != 1 {
		t.Fatalf("fallback reasons = %+v", snap.FallbackReasons)
	}

	s.Reset()
	snap = s.Snapshot()
	if snap.ShortQARouteAttempts != 0 ||
		snap.ToolDispatchRouteAttempts != 0 ||
		snap.SummaryAttempts != 0 ||
		snap.DocExtractAttempts != 0 ||
		snap.FallbackTotal != 0 ||
		snap.TimeoutTotal != 0 ||
		snap.LatencySamples != 0 ||
		snap.LatencyMsTotal != 0 ||
		snap.ShortQALatencySamples != 0 ||
		snap.ToolDispatchLatencySamples != 0 ||
		snap.SummaryLatencySamples != 0 ||
		snap.DocExtractLatencySamples != 0 ||
		snap.ShadowQualitySamples != 0 ||
		snap.AutoRollbackTotal != 0 ||
		snap.DeepResearchFallback != 0 ||
		snap.IRTakeover != 0 ||
		len(snap.FallbackReasons) != 0 {
		t.Fatalf("expected reset snapshot, got %+v", snap)
	}
}
