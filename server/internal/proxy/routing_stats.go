package proxy

import (
	"sync/atomic"

	"github.com/tidwall/gjson"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

// RoutingStats tracks cost savings from model routing (rule engine + background downgrade).
type RoutingStats struct {
	routedRequests    int64 // requests where model was swapped
	inputTokensRouted int64 // input tokens on routed requests
	costSavedMicro    int64 // cost saved in micro-dollars (1e-6 USD) for precision
}

// NewRoutingStats creates a new RoutingStats instance.
func NewRoutingStats() *RoutingStats {
	return &RoutingStats{}
}

// Record calculates and accumulates cost savings for a routed request.
// originalModel is the model the client requested; actualModel is what was used.
func (s *RoutingStats) Record(originalModel, actualModel string, respBody []byte) {
	if originalModel == "" || actualModel == "" || originalModel == actualModel {
		return
	}

	inputTokens, outputTokens := parseUsageTokens(respBody)
	s.RecordTokens(originalModel, actualModel, inputTokens, outputTokens)
}

// RecordTokens accumulates cost savings from pre-parsed token counts.
// Used by streaming paths where tokens are already extracted.
func (s *RoutingStats) RecordTokens(originalModel, actualModel string, inputTokens, outputTokens int) {
	if originalModel == "" || actualModel == "" || originalModel == actualModel {
		return
	}
	if inputTokens == 0 && outputTokens == 0 {
		return
	}

	origPrice := providerpool.MatchModelPricing(originalModel)
	actualPrice := providerpool.MatchModelPricing(actualModel)
	if origPrice == nil || actualPrice == nil {
		return
	}

	// Cost per token = price per 1M tokens / 1_000_000
	// Store in micro-dollars (multiply by 1e6) to avoid float atomics
	inputSaved := (origPrice.InputPrice - actualPrice.InputPrice) * float64(inputTokens) / 1_000_000
	outputSaved := (origPrice.OutputPrice - actualPrice.OutputPrice) * float64(outputTokens) / 1_000_000
	totalSavedMicro := int64((inputSaved + outputSaved) * 1_000_000)

	if totalSavedMicro > 0 {
		atomic.AddInt64(&s.routedRequests, 1)
		atomic.AddInt64(&s.inputTokensRouted, int64(inputTokens+outputTokens))
		atomic.AddInt64(&s.costSavedMicro, totalSavedMicro)
	}
}

// RoutingStatsSnapshot is a point-in-time copy of routing stats.
type RoutingStatsSnapshot struct {
	RoutedRequests int64   `json:"routed_requests"`
	TokensRouted   int64   `json:"tokens_routed"`
	CostSavedUSD   float64 `json:"cost_saved_usd"`
}

// Snapshot returns a point-in-time copy.
func (s *RoutingStats) Snapshot() RoutingStatsSnapshot {
	micro := atomic.LoadInt64(&s.costSavedMicro)
	return RoutingStatsSnapshot{
		RoutedRequests: atomic.LoadInt64(&s.routedRequests),
		TokensRouted:   atomic.LoadInt64(&s.inputTokensRouted),
		CostSavedUSD:   float64(micro) / 1_000_000,
	}
}

// Load restores persisted counters (called on startup).
func (s *RoutingStats) Load(routedRequests, tokensRouted, costSavedMicro int64) {
	atomic.StoreInt64(&s.routedRequests, routedRequests)
	atomic.StoreInt64(&s.inputTokensRouted, tokensRouted)
	atomic.StoreInt64(&s.costSavedMicro, costSavedMicro)
}

// parseUsageTokens extracts input/output token counts from a response body.
// Supports OpenAI and Anthropic formats.
func parseUsageTokens(body []byte) (input, output int) {
	if len(body) == 0 {
		return 0, 0
	}
	if pt := gjson.GetBytes(body, "usage.prompt_tokens"); pt.Exists() {
		return int(pt.Int()), int(gjson.GetBytes(body, "usage.completion_tokens").Int())
	}
	if it := gjson.GetBytes(body, "usage.input_tokens"); it.Exists() {
		return int(it.Int()), int(gjson.GetBytes(body, "usage.output_tokens").Int())
	}
	return 0, 0
}
