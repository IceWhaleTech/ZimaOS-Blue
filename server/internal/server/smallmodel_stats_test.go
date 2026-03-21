package server

import (
	"math"
	"testing"
	"time"
)

func TestSmallModelStatsRecordAndReset(t *testing.T) {
	s := NewSmallModelStats()
	s.RecordShortQARoute(true)
	s.RecordShortQARoute(false)
	s.RecordImageQARoute(true)
	s.RecordImageQARoute(false)
	s.RecordToolDispatchRoute(true)
	s.RecordToolDispatchRoute(false)
	s.RecordSummaryAttempt()
	s.RecordSummarySuccess()
	s.RecordSummaryAttempt()
	s.RecordContextCompressAttempt()
	s.RecordContextCompressSuccess()
	s.RecordContextCompressAttempt()
	s.RecordDocExtractAttempt()
	s.RecordDocExtractSuccess()
	s.RecordDocExtractAttempt()
	s.RecordLatencyWithScene("short_qa", 20*time.Millisecond)
	s.RecordLatencyWithScene("image_qa", 25*time.Millisecond)
	s.RecordLatencyWithScene("tool_dispatch", 40*time.Millisecond)
	s.RecordLatencyWithScene("summary", 50*time.Millisecond)
	s.RecordLatencyWithScene("context_compress", 35*time.Millisecond)
	s.RecordLatencyWithScene("doc_extract", 30*time.Millisecond)
	s.RecordFallback("model_unready")
	s.RecordFallback("timeout")
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
	if snap.ImageQARouteAttempts != 2 {
		t.Fatalf("ImageQARouteAttempts = %d, want 2", snap.ImageQARouteAttempts)
	}
	if snap.ImageQARouteSuccess != 1 {
		t.Fatalf("ImageQARouteSuccess = %d, want 1", snap.ImageQARouteSuccess)
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
	if snap.ContextCompressAttempts != 2 || snap.ContextCompressSuccess != 1 {
		t.Fatalf("unexpected context_compress counters: attempts=%d success=%d", snap.ContextCompressAttempts, snap.ContextCompressSuccess)
	}
	if snap.DocExtractAttempts != 2 || snap.DocExtractSuccess != 1 {
		t.Fatalf("unexpected doc_extract counters: attempts=%d success=%d", snap.DocExtractAttempts, snap.DocExtractSuccess)
	}
	if snap.FallbackTotal != 2 {
		t.Fatalf("FallbackTotal = %d, want 2", snap.FallbackTotal)
	}
	if snap.TimeoutTotal != 1 {
		t.Fatalf("TimeoutTotal = %d, want 1", snap.TimeoutTotal)
	}
	if snap.LatencySamples != 6 || snap.LatencyMsTotal != 200 {
		t.Fatalf("unexpected latency counters: samples=%d total=%d", snap.LatencySamples, snap.LatencyMsTotal)
	}
	if math.Abs(snap.LatencyMs-33.333333333333336) > 1e-9 {
		t.Fatalf("LatencyMs = %v, want %v", snap.LatencyMs, 33.333333333333336)
	}
	if snap.ShortQALatencySamples != 1 || snap.ShortQALatencyMs != 20 {
		t.Fatalf("unexpected short_qa latency stats: samples=%d avg=%v", snap.ShortQALatencySamples, snap.ShortQALatencyMs)
	}
	if snap.ImageQALatencySamples != 1 || snap.ImageQALatencyMs != 25 {
		t.Fatalf("unexpected image_qa latency stats: samples=%d avg=%v", snap.ImageQALatencySamples, snap.ImageQALatencyMs)
	}
	if snap.ToolDispatchLatencySamples != 1 || snap.ToolDispatchLatencyMs != 40 {
		t.Fatalf("unexpected tool_dispatch latency stats: samples=%d avg=%v", snap.ToolDispatchLatencySamples, snap.ToolDispatchLatencyMs)
	}
	if snap.SummaryLatencySamples != 1 || snap.SummaryLatencyMs != 50 {
		t.Fatalf("unexpected summary latency stats: samples=%d avg=%v", snap.SummaryLatencySamples, snap.SummaryLatencyMs)
	}
	if snap.ContextCompressSamples != 1 || snap.ContextCompressLatencyMs != 35 {
		t.Fatalf("unexpected context_compress latency stats: samples=%d avg=%v", snap.ContextCompressSamples, snap.ContextCompressLatencyMs)
	}
	if snap.DocExtractLatencySamples != 1 || snap.DocExtractLatencyMs != 30 {
		t.Fatalf("unexpected doc_extract latency stats: samples=%d avg=%v", snap.DocExtractLatencySamples, snap.DocExtractLatencyMs)
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
		snap.ImageQARouteAttempts != 0 ||
		snap.ToolDispatchRouteAttempts != 0 ||
		snap.SummaryAttempts != 0 ||
		snap.ContextCompressAttempts != 0 ||
		snap.DocExtractAttempts != 0 ||
		snap.FallbackTotal != 0 ||
		snap.TimeoutTotal != 0 ||
		snap.LatencySamples != 0 ||
		snap.LatencyMsTotal != 0 ||
		snap.ShortQALatencySamples != 0 ||
		snap.ImageQALatencySamples != 0 ||
		snap.ToolDispatchLatencySamples != 0 ||
		snap.SummaryLatencySamples != 0 ||
		snap.ContextCompressSamples != 0 ||
		snap.DocExtractLatencySamples != 0 ||
		snap.AutoRollbackTotal != 0 ||
		snap.DeepResearchFallback != 0 ||
		snap.IRTakeover != 0 ||
		len(snap.FallbackReasons) != 0 {
		t.Fatalf("expected reset snapshot, got %+v", snap)
	}
}
