package server

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const (
	providerAccelerationSlowTTFTThresholdMs     = 1500.0
	providerAccelerationLargeInputThreshold     = 1200
	providerAccelerationHighCacheReuseThreshold = 0.6
	providerAccelerationMaxRecentTurns          = 6
	providerAccelerationWarmupFailureLimit      = 2
	providerAccelerationZeroBenefitLimit        = 2
	providerAccelerationCooldownDuration        = 5 * time.Minute
)

type providerAccelerationRoute struct {
	ProviderID string
	Model      string
}

func (r providerAccelerationRoute) key() string {
	providerID := strings.ToLower(strings.TrimSpace(r.ProviderID))
	model := strings.ToLower(strings.TrimSpace(r.Model))
	if providerID == "" || model == "" {
		return ""
	}
	return providerID + "::" + model
}

func (r providerAccelerationRoute) valid() bool {
	return r.key() != ""
}

type providerAccelerationObservation struct {
	Route                    providerAccelerationRoute
	TTFTMs                   float64
	InputTokens              int
	CacheReadInputTokens     int
	CacheCreationInputTokens int
	CacheReuseRatio          float64
	CacheReuseKnown          bool
	Accelerated              bool
	RecordedAt               time.Time
}

type providerAccelerationWarmupDecision struct {
	Eligible bool
	Route    providerAccelerationRoute
	Reason   string
}

type providerAccelerationPendingWarmup struct {
	Route     providerAccelerationRoute
	StartedAt time.Time
}

type providerAccelerationTurn struct {
	Route     providerAccelerationRoute
	StartedAt time.Time
}

type providerAccelerationState struct {
	Recent                    []providerAccelerationObservation
	PendingWarmup             *providerAccelerationPendingWarmup
	CooldownUntil             time.Time
	CooldownReason            string
	ConsecutiveWarmupFailures int
	ConsecutiveZeroBenefit    int
}

type providerAccelerationMetrics struct {
	eligibilityCount     atomic.Int64
	warmupStarted        atomic.Int64
	warmupCanceled       atomic.Int64
	warmupFailed         atomic.Int64
	routeDrift           atomic.Int64
	cacheReadTokens      atomic.Int64
	acceleratedTurns     atomic.Int64
	baselineTurns        atomic.Int64
	acceleratedTTFTTotal atomic.Int64
	baselineTTFTTotal    atomic.Int64
}

func (m *providerAccelerationMetrics) estimatedTTFTDeltaMs() int64 {
	if m == nil {
		return 0
	}
	acceleratedTurns := m.acceleratedTurns.Load()
	baselineTurns := m.baselineTurns.Load()
	if acceleratedTurns == 0 || baselineTurns == 0 {
		return 0
	}
	acceleratedAvg := m.acceleratedTTFTTotal.Load() / acceleratedTurns
	baselineAvg := m.baselineTTFTTotal.Load() / baselineTurns
	return baselineAvg - acceleratedAvg
}

func (h *ChatHandler) providerAccelerationWarmupDecision(convID, requestModel string, state memory.ConversationCommandState) providerAccelerationWarmupDecision {
	decision := providerAccelerationWarmupDecision{Reason: "insufficient_history"}
	if h == nil {
		decision.Reason = "handler_unavailable"
		return decision
	}
	targetProviderID, targetProvider, ok := h.providerWarmupTarget(convID, state)
	if !ok || !supportsProviderSidePromptWarmup(targetProvider) || !supportsPromptCacheKey(targetProvider) {
		decision.Reason = "provider_unsupported"
		return decision
	}

	now := timeutil.NowTime()
	h.providerAccelerationMu.Lock()
	defer h.providerAccelerationMu.Unlock()

	accelState := h.providerAcceleration[convID]
	if accelState == nil || len(accelState.Recent) < 2 {
		return decision
	}
	if accelState.CooldownUntil.After(now) {
		decision.Reason = "cooldown_active"
		return decision
	}

	last := accelState.Recent[len(accelState.Recent)-1]
	prev := accelState.Recent[len(accelState.Recent)-2]
	if last.Route.key() == "" || last.Route.key() != prev.Route.key() {
		decision.Reason = "route_unstable"
		return decision
	}
	if !strings.EqualFold(last.Route.ProviderID, targetProviderID) {
		decision.Reason = "provider_mismatch"
		return decision
	}

	stableModel := h.resolveStableModel(requestModel, last.Route.Model)
	if stableModel == "" {
		stableModel = last.Route.Model
	}
	if !strings.EqualFold(stableModel, last.Route.Model) {
		decision.Reason = "model_mismatch"
		return decision
	}

	for _, obs := range []providerAccelerationObservation{prev, last} {
		if obs.TTFTMs <= providerAccelerationSlowTTFTThresholdMs {
			decision.Reason = "ttft_below_threshold"
			return decision
		}
		if obs.InputTokens <= providerAccelerationLargeInputThreshold {
			decision.Reason = "input_below_threshold"
			return decision
		}
		if obs.CacheReuseKnown && obs.CacheReuseRatio >= providerAccelerationHighCacheReuseThreshold {
			decision.Reason = "cache_reuse_high"
			return decision
		}
	}

	decision.Eligible = true
	decision.Route = last.Route
	decision.Reason = "eligible"
	if h.providerAccelerationMetrics != nil {
		h.providerAccelerationMetrics.eligibilityCount.Add(1)
	}
	return decision
}

func (h *ChatHandler) recordProviderAccelerationObservation(convID string, obs providerAccelerationObservation) {
	if h == nil || strings.TrimSpace(convID) == "" || !obs.Route.valid() {
		return
	}
	if obs.RecordedAt.IsZero() {
		obs.RecordedAt = timeutil.NowTime()
	}
	h.providerAccelerationMu.Lock()
	defer h.providerAccelerationMu.Unlock()

	accelState := h.providerAcceleration[convID]
	if accelState == nil {
		accelState = &providerAccelerationState{}
		h.providerAcceleration[convID] = accelState
	}
	accelState.Recent = append(accelState.Recent, obs)
	if len(accelState.Recent) > providerAccelerationMaxRecentTurns {
		accelState.Recent = append([]providerAccelerationObservation(nil), accelState.Recent[len(accelState.Recent)-providerAccelerationMaxRecentTurns:]...)
	}
}

func (h *ChatHandler) noteProviderAccelerationWarmupStarted(convID string, route providerAccelerationRoute) {
	if h == nil || strings.TrimSpace(convID) == "" || !route.valid() {
		return
	}
	h.providerAccelerationMu.Lock()
	defer h.providerAccelerationMu.Unlock()

	accelState := h.providerAcceleration[convID]
	if accelState == nil {
		accelState = &providerAccelerationState{}
		h.providerAcceleration[convID] = accelState
	}
	accelState.PendingWarmup = &providerAccelerationPendingWarmup{
		Route:     route,
		StartedAt: timeutil.NowTime(),
	}
	if h.providerAccelerationMetrics != nil {
		h.providerAccelerationMetrics.warmupStarted.Add(1)
	}
}

func (h *ChatHandler) noteProviderAccelerationWarmupCanceled(convID, reason string) {
	if h == nil || strings.TrimSpace(convID) == "" {
		return
	}
	h.providerAccelerationMu.Lock()
	defer h.providerAccelerationMu.Unlock()

	if accelState := h.providerAcceleration[convID]; accelState != nil {
		accelState.PendingWarmup = nil
	}
	if h.providerAccelerationMetrics != nil {
		h.providerAccelerationMetrics.warmupCanceled.Add(1)
	}
	logger.Debug().Str("conv_id", convID).Str("reason", strings.TrimSpace(reason)).Msg("[warmup] provider acceleration pending warmup cleared")
}

func (h *ChatHandler) noteProviderAccelerationWarmupFailure(convID string, route providerAccelerationRoute, reason string) {
	if h == nil || strings.TrimSpace(convID) == "" {
		return
	}
	h.providerAccelerationMu.Lock()
	defer h.providerAccelerationMu.Unlock()

	accelState := h.providerAcceleration[convID]
	if accelState == nil {
		accelState = &providerAccelerationState{}
		h.providerAcceleration[convID] = accelState
	}
	if pending := accelState.PendingWarmup; pending != nil && pending.Route.key() == route.key() {
		accelState.PendingWarmup = nil
	}
	accelState.ConsecutiveWarmupFailures++
	if accelState.ConsecutiveWarmupFailures >= providerAccelerationWarmupFailureLimit {
		h.setProviderAccelerationCooldownLocked(accelState, "warmup_failure")
	}
	if h.providerAccelerationMetrics != nil {
		h.providerAccelerationMetrics.warmupFailed.Add(1)
	}
	logger.Debug().
		Str("conv_id", convID).
		Str("route", route.key()).
		Str("reason", strings.TrimSpace(reason)).
		Int("consecutive_failures", accelState.ConsecutiveWarmupFailures).
		Msg("[warmup] provider acceleration warmup failure recorded")
}

func (h *ChatHandler) claimProviderAccelerationTurn(convID, requestModel string, state memory.ConversationCommandState) (providerAccelerationTurn, bool) {
	var turn providerAccelerationTurn
	if h == nil || strings.TrimSpace(convID) == "" {
		return turn, false
	}
	targetProviderID, _, ok := h.providerWarmupTarget(convID, state)
	if !ok {
		return turn, false
	}

	now := timeutil.NowTime()
	h.providerAccelerationMu.Lock()
	defer h.providerAccelerationMu.Unlock()

	accelState := h.providerAcceleration[convID]
	if accelState == nil || accelState.PendingWarmup == nil {
		return turn, false
	}
	if accelState.CooldownUntil.After(now) {
		return turn, false
	}
	if now.Sub(accelState.PendingWarmup.StartedAt) > warmupTTL {
		accelState.PendingWarmup = nil
		return turn, false
	}
	if !strings.EqualFold(accelState.PendingWarmup.Route.ProviderID, targetProviderID) {
		return turn, false
	}

	stableModel := h.resolveStableModel(requestModel, accelState.PendingWarmup.Route.Model)
	if stableModel == "" {
		stableModel = accelState.PendingWarmup.Route.Model
	}
	if !strings.EqualFold(stableModel, accelState.PendingWarmup.Route.Model) {
		return turn, false
	}

	turn = providerAccelerationTurn{
		Route:     accelState.PendingWarmup.Route,
		StartedAt: accelState.PendingWarmup.StartedAt,
	}
	accelState.PendingWarmup = nil
	return turn, true
}

func (h *ChatHandler) recordProviderAccelerationTurnResult(
	convID string,
	turn providerAccelerationTurn,
	actualProviderID string,
	actualModel string,
	ttftMs float64,
	inputTokens int,
	cacheReadTokens int,
	cacheCreationTokens int,
	accelerated bool,
) {
	if h == nil || strings.TrimSpace(convID) == "" {
		return
	}
	route := providerAccelerationRoute{
		ProviderID: strings.TrimSpace(actualProviderID),
		Model:      strings.TrimSpace(actualModel),
	}
	if !route.valid() {
		route = turn.Route
	}
	if !route.valid() {
		return
	}

	obs := providerAccelerationObservation{
		Route:                    route,
		TTFTMs:                   ttftMs,
		InputTokens:              inputTokens,
		CacheReadInputTokens:     cacheReadTokens,
		CacheCreationInputTokens: cacheCreationTokens,
		Accelerated:              accelerated,
		RecordedAt:               timeutil.NowTime(),
	}
	if inputTokens > 0 && (cacheReadTokens > 0 || cacheCreationTokens > 0 || accelerated) {
		obs.CacheReuseKnown = true
		obs.CacheReuseRatio = float64(cacheReadTokens) / float64(inputTokens)
	}

	h.providerAccelerationMu.Lock()
	accelState := h.providerAcceleration[convID]
	if accelState == nil {
		accelState = &providerAccelerationState{}
		h.providerAcceleration[convID] = accelState
	}

	if accelerated {
		if turn.Route.valid() && route.key() != turn.Route.key() {
			accelState.ConsecutiveZeroBenefit = 0
			h.setProviderAccelerationCooldownLocked(accelState, "route_drift")
			if h.providerAccelerationMetrics != nil {
				h.providerAccelerationMetrics.routeDrift.Add(1)
			}
			logger.Info().
				Str("conv_id", convID).
				Str("warmup_route", turn.Route.key()).
				Str("actual_route", route.key()).
				Msg("[warmup] provider acceleration suppressed after route drift")
		} else if inputTokens > 0 && cacheReadTokens == 0 {
			accelState.ConsecutiveZeroBenefit++
			if accelState.ConsecutiveZeroBenefit >= providerAccelerationZeroBenefitLimit {
				h.setProviderAccelerationCooldownLocked(accelState, "zero_benefit")
			}
		} else if cacheReadTokens > 0 {
			accelState.ConsecutiveZeroBenefit = 0
			accelState.ConsecutiveWarmupFailures = 0
		}
	}

	accelState.Recent = append(accelState.Recent, obs)
	if len(accelState.Recent) > providerAccelerationMaxRecentTurns {
		accelState.Recent = append([]providerAccelerationObservation(nil), accelState.Recent[len(accelState.Recent)-providerAccelerationMaxRecentTurns:]...)
	}
	estimatedDeltaMs := int64(0)
	if h.providerAccelerationMetrics != nil {
		if cacheReadTokens > 0 {
			h.providerAccelerationMetrics.cacheReadTokens.Add(int64(cacheReadTokens))
		}
		if accelerated {
			h.providerAccelerationMetrics.acceleratedTurns.Add(1)
			h.providerAccelerationMetrics.acceleratedTTFTTotal.Add(int64(ttftMs))
		} else {
			h.providerAccelerationMetrics.baselineTurns.Add(1)
			h.providerAccelerationMetrics.baselineTTFTTotal.Add(int64(ttftMs))
		}
		estimatedDeltaMs = h.providerAccelerationMetrics.estimatedTTFTDeltaMs()
	}
	h.providerAccelerationMu.Unlock()

	logger.Debug().
		Str("conv_id", convID).
		Str("route", route.key()).
		Bool("accelerated", accelerated).
		Float64("ttft_ms", ttftMs).
		Int("input_tokens", inputTokens).
		Int("cache_read_tokens", cacheReadTokens).
		Int64("estimated_ttft_delta_ms", estimatedDeltaMs).
		Msg("[warmup] provider acceleration turn recorded")
}

func (h *ChatHandler) setProviderAccelerationCooldownLocked(state *providerAccelerationState, reason string) {
	if state == nil {
		return
	}
	state.CooldownUntil = timeutil.NowTime().Add(providerAccelerationCooldownDuration)
	state.CooldownReason = strings.TrimSpace(reason)
}

func providerAccelerationRouteForModel(providerID, requestModel, fallbackModel string) providerAccelerationRoute {
	return providerAccelerationRoute{
		ProviderID: strings.TrimSpace(providerID),
		Model:      strings.TrimSpace(fallbackModel),
	}
}

func supportsAdaptiveProviderAcceleration(provider *providerpool.Provider) bool {
	return supportsProviderSidePromptWarmup(provider) && supportsPromptCacheKey(provider)
}
