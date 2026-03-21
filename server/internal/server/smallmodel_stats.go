package server

import (
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

// SmallModelStats tracks small-model routing/fallback observability.
type SmallModelStats struct {
	mu sync.Mutex

	ShortQARouteAttempts       int64            `json:"short_qa_route_attempts"`
	ShortQARouteSuccess        int64            `json:"short_qa_route_success"`
	ImageQARouteAttempts       int64            `json:"image_qa_route_attempts"`
	ImageQARouteSuccess        int64            `json:"image_qa_route_success"`
	ToolDispatchRouteAttempts  int64            `json:"tool_dispatch_route_attempts"`
	ToolDispatchRouteSuccess   int64            `json:"tool_dispatch_route_success"`
	SummaryAttempts            int64            `json:"summary_attempts"`
	SummarySuccess             int64            `json:"summary_success"`
	ContextCompressAttempts    int64            `json:"context_compress_attempts"`
	ContextCompressSuccess     int64            `json:"context_compress_success"`
	DocExtractAttempts         int64            `json:"doc_extract_attempts"`
	DocExtractSuccess          int64            `json:"doc_extract_success"`
	FallbackTotal              int64            `json:"small_model_fallback_total"`
	TimeoutTotal               int64            `json:"small_model_timeout_total"`
	LatencyMs                  float64          `json:"small_model_latency_ms"`
	LatencySamples             int64            `json:"small_model_latency_samples"`
	LatencyMsTotal             int64            `json:"small_model_latency_ms_total"`
	ShortQALatencyMs           float64          `json:"short_qa_latency_ms"`
	ShortQALatencySamples      int64            `json:"short_qa_latency_samples"`
	ShortQALatencyMsTotal      int64            `json:"short_qa_latency_ms_total"`
	ImageQALatencyMs           float64          `json:"image_qa_latency_ms"`
	ImageQALatencySamples      int64            `json:"image_qa_latency_samples"`
	ImageQALatencyMsTotal      int64            `json:"image_qa_latency_ms_total"`
	ToolDispatchLatencyMs      float64          `json:"tool_dispatch_latency_ms"`
	ToolDispatchLatencySamples int64            `json:"tool_dispatch_latency_samples"`
	ToolDispatchLatencyTotal   int64            `json:"tool_dispatch_latency_ms_total"`
	SummaryLatencyMs           float64          `json:"summary_latency_ms"`
	SummaryLatencySamples      int64            `json:"summary_latency_samples"`
	SummaryLatencyTotal        int64            `json:"summary_latency_ms_total"`
	ContextCompressLatencyMs   float64          `json:"context_compress_latency_ms"`
	ContextCompressSamples     int64            `json:"context_compress_latency_samples"`
	ContextCompressTotal       int64            `json:"context_compress_latency_ms_total"`
	DocExtractLatencyMs        float64          `json:"doc_extract_latency_ms"`
	DocExtractLatencySamples   int64            `json:"doc_extract_latency_samples"`
	DocExtractLatencyTotal     int64            `json:"doc_extract_latency_ms_total"`
	AutoRollbackTotal          int64            `json:"auto_rollback_total"`
	DeepResearchFallback       int64            `json:"no_provider_deepresearch_total"`
	IRTakeover                 int64            `json:"ir_takeover_total"`
	FallbackReasons            map[string]int64 `json:"fallback_reasons"`
}

func NewSmallModelStats() *SmallModelStats {
	return &SmallModelStats{
		FallbackReasons: map[string]int64{},
	}
}

func (s *SmallModelStats) RecordShortQARoute(success bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ShortQARouteAttempts++
	if success {
		s.ShortQARouteSuccess++
	}
}

func (s *SmallModelStats) RecordImageQARoute(success bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ImageQARouteAttempts++
	if success {
		s.ImageQARouteSuccess++
	}
}

func (s *SmallModelStats) RecordToolDispatchRoute(success bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ToolDispatchRouteAttempts++
	if success {
		s.ToolDispatchRouteSuccess++
	}
}

func (s *SmallModelStats) RecordSummaryAttempt() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SummaryAttempts++
}

func (s *SmallModelStats) RecordSummarySuccess() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SummarySuccess++
}

func (s *SmallModelStats) RecordContextCompressAttempt() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ContextCompressAttempts++
}

func (s *SmallModelStats) RecordContextCompressSuccess() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ContextCompressSuccess++
}

func (s *SmallModelStats) RecordDocExtractAttempt() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DocExtractAttempts++
}

func (s *SmallModelStats) RecordDocExtractSuccess() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DocExtractSuccess++
}

func (s *SmallModelStats) RecordFallback(reason string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if reason == "" {
		reason = "unknown"
	}
	s.FallbackTotal++
	if reason == smallmodel.FallbackReasonTimeout {
		s.TimeoutTotal++
	}
	s.FallbackReasons[reason]++
}

func (s *SmallModelStats) RecordLatency(d time.Duration) {
	s.RecordLatencyWithScene("", d)
}

func (s *SmallModelStats) RecordLatencyWithScene(scene string, d time.Duration) {
	if s == nil || d <= 0 {
		return
	}
	ms := d.Milliseconds()
	if ms < 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LatencySamples++
	s.LatencyMsTotal += ms
	switch scene {
	case "short_qa":
		s.ShortQALatencySamples++
		s.ShortQALatencyMsTotal += ms
	case "image_qa":
		s.ImageQALatencySamples++
		s.ImageQALatencyMsTotal += ms
	case "tool_dispatch":
		s.ToolDispatchLatencySamples++
		s.ToolDispatchLatencyTotal += ms
	case "summary":
		s.SummaryLatencySamples++
		s.SummaryLatencyTotal += ms
	case "context_compress":
		s.ContextCompressSamples++
		s.ContextCompressTotal += ms
	case "doc_extract":
		s.DocExtractLatencySamples++
		s.DocExtractLatencyTotal += ms
	}
}

func (s *SmallModelStats) RecordDeepResearchFallback() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DeepResearchFallback++
}

func (s *SmallModelStats) RecordAutoRollback() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AutoRollbackTotal++
}

func (s *SmallModelStats) RecordIRTakeover() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IRTakeover++
}

func (s *SmallModelStats) Snapshot() SmallModelStats {
	if s == nil {
		return SmallModelStats{FallbackReasons: map[string]int64{}}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := SmallModelStats{
		ShortQARouteAttempts:       s.ShortQARouteAttempts,
		ShortQARouteSuccess:        s.ShortQARouteSuccess,
		ImageQARouteAttempts:       s.ImageQARouteAttempts,
		ImageQARouteSuccess:        s.ImageQARouteSuccess,
		ToolDispatchRouteAttempts:  s.ToolDispatchRouteAttempts,
		ToolDispatchRouteSuccess:   s.ToolDispatchRouteSuccess,
		SummaryAttempts:            s.SummaryAttempts,
		SummarySuccess:             s.SummarySuccess,
		ContextCompressAttempts:    s.ContextCompressAttempts,
		ContextCompressSuccess:     s.ContextCompressSuccess,
		DocExtractAttempts:         s.DocExtractAttempts,
		DocExtractSuccess:          s.DocExtractSuccess,
		FallbackTotal:              s.FallbackTotal,
		TimeoutTotal:               s.TimeoutTotal,
		LatencySamples:             s.LatencySamples,
		LatencyMsTotal:             s.LatencyMsTotal,
		ShortQALatencySamples:      s.ShortQALatencySamples,
		ShortQALatencyMsTotal:      s.ShortQALatencyMsTotal,
		ImageQALatencySamples:      s.ImageQALatencySamples,
		ImageQALatencyMsTotal:      s.ImageQALatencyMsTotal,
		ToolDispatchLatencySamples: s.ToolDispatchLatencySamples,
		ToolDispatchLatencyTotal:   s.ToolDispatchLatencyTotal,
		SummaryLatencySamples:      s.SummaryLatencySamples,
		SummaryLatencyTotal:        s.SummaryLatencyTotal,
		ContextCompressSamples:     s.ContextCompressSamples,
		ContextCompressTotal:       s.ContextCompressTotal,
		DocExtractLatencySamples:   s.DocExtractLatencySamples,
		DocExtractLatencyTotal:     s.DocExtractLatencyTotal,
		AutoRollbackTotal:          s.AutoRollbackTotal,
		DeepResearchFallback:       s.DeepResearchFallback,
		IRTakeover:                 s.IRTakeover,
		FallbackReasons:            make(map[string]int64, len(s.FallbackReasons)),
	}
	if cp.LatencySamples > 0 {
		cp.LatencyMs = float64(cp.LatencyMsTotal) / float64(cp.LatencySamples)
	}
	if cp.ShortQALatencySamples > 0 {
		cp.ShortQALatencyMs = float64(cp.ShortQALatencyMsTotal) / float64(cp.ShortQALatencySamples)
	}
	if cp.ImageQALatencySamples > 0 {
		cp.ImageQALatencyMs = float64(cp.ImageQALatencyMsTotal) / float64(cp.ImageQALatencySamples)
	}
	if cp.ToolDispatchLatencySamples > 0 {
		cp.ToolDispatchLatencyMs = float64(cp.ToolDispatchLatencyTotal) / float64(cp.ToolDispatchLatencySamples)
	}
	if cp.SummaryLatencySamples > 0 {
		cp.SummaryLatencyMs = float64(cp.SummaryLatencyTotal) / float64(cp.SummaryLatencySamples)
	}
	if cp.ContextCompressSamples > 0 {
		cp.ContextCompressLatencyMs = float64(cp.ContextCompressTotal) / float64(cp.ContextCompressSamples)
	}
	if cp.DocExtractLatencySamples > 0 {
		cp.DocExtractLatencyMs = float64(cp.DocExtractLatencyTotal) / float64(cp.DocExtractLatencySamples)
	}
	for k, v := range s.FallbackReasons {
		cp.FallbackReasons[k] = v
	}
	return cp
}

func (s *SmallModelStats) Reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ShortQARouteAttempts = 0
	s.ShortQARouteSuccess = 0
	s.ImageQARouteAttempts = 0
	s.ImageQARouteSuccess = 0
	s.ToolDispatchRouteAttempts = 0
	s.ToolDispatchRouteSuccess = 0
	s.SummaryAttempts = 0
	s.SummarySuccess = 0
	s.ContextCompressAttempts = 0
	s.ContextCompressSuccess = 0
	s.DocExtractAttempts = 0
	s.DocExtractSuccess = 0
	s.FallbackTotal = 0
	s.TimeoutTotal = 0
	s.LatencyMs = 0
	s.LatencySamples = 0
	s.LatencyMsTotal = 0
	s.ShortQALatencyMs = 0
	s.ShortQALatencySamples = 0
	s.ShortQALatencyMsTotal = 0
	s.ImageQALatencyMs = 0
	s.ImageQALatencySamples = 0
	s.ImageQALatencyMsTotal = 0
	s.ToolDispatchLatencyMs = 0
	s.ToolDispatchLatencySamples = 0
	s.ToolDispatchLatencyTotal = 0
	s.SummaryLatencyMs = 0
	s.SummaryLatencySamples = 0
	s.SummaryLatencyTotal = 0
	s.ContextCompressLatencyMs = 0
	s.ContextCompressSamples = 0
	s.ContextCompressTotal = 0
	s.DocExtractLatencyMs = 0
	s.DocExtractLatencySamples = 0
	s.DocExtractLatencyTotal = 0
	s.AutoRollbackTotal = 0
	s.DeepResearchFallback = 0
	s.IRTakeover = 0
	s.FallbackReasons = map[string]int64{}
}
