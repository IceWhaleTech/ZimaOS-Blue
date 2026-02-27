package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ResolvedRoute carries the actual provider/model chosen by the router.
// Callers embed a pointer in the request context; the proxy handler populates it.
type ResolvedRoute struct {
	Provider   string // display name
	ProviderID string // internal ID (for sticky routing)
	Model      string
}

type resolvedRouteKeyType struct{}

// WithResolvedRoute returns a context carrying a ResolvedRoute pointer.
// After the proxy handler completes, the struct will be populated.
func WithResolvedRoute(ctx context.Context, rr *ResolvedRoute) context.Context {
	return context.WithValue(ctx, resolvedRouteKeyType{}, rr)
}

func getResolvedRoute(ctx context.Context) *ResolvedRoute {
	rr, _ := ctx.Value(resolvedRouteKeyType{}).(*ResolvedRoute)
	return rr
}

// Pinned provider context — used for sticky routing during tool rounds.
type pinnedProviderKeyType struct{}

// WithPinnedProvider returns a context carrying a preferred provider ID.
// The proxy handler reads this to set RouteRequest.PreferredProviderID.
func WithPinnedProvider(ctx context.Context, providerID string) context.Context {
	return context.WithValue(ctx, pinnedProviderKeyType{}, providerID)
}

// GetPinnedProvider returns the pinned provider ID from context, or "".
func GetPinnedProvider(ctx context.Context) string {
	v, _ := ctx.Value(pinnedProviderKeyType{}).(string)
	return v
}

// GetResolvedRouteFromContext is the exported version of getResolvedRoute.
func GetResolvedRouteFromContext(ctx context.Context) *ResolvedRoute {
	return getResolvedRoute(ctx)
}

// parsedRequest holds pre-parsed request data to avoid redundant JSON parsing.
// Created once in ServeHTTP and passed through the call chain.
// Uses gjson for zero-alloc field extraction instead of map[string]interface{}.
// Layout: pointer-sized fields first, then bools — minimizes padding for cache-line efficiency.
type parsedRequest struct {
	body               []byte
	requestedModel     string         // model value from request body before normalization/routing
	model              string
	originalModel      string         // model before routing (for cost savings tracking)
	upstreamFormat     ProviderType   // set when request was converted to non-OpenAI format
	routed             *RouteDecision // non-nil if rule engine rerouted the model
	streaming          bool
	singleProvider     bool   // true when only one provider matches current routing mode
	resolvedProvider   string // actual provider name after routing
	resolvedProviderID string // actual provider ID after routing
	resolvedModel      string // actual model ID after routing
}

// sseBufferPool reuses 32KB buffers for SSE streaming to reduce GC pressure.
var sseBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 32768) // 32KB
		return &buf
	},
}

// bodyBufferPool reuses bytes.Buffer for reading request/response bodies.
var bodyBufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 8192)) // 8KB initial
	},
}

// parsedRequestPool reuses parsedRequest structs to reduce per-request heap allocs.
var parsedRequestPool = sync.Pool{
	New: func() interface{} {
		return &parsedRequest{}
	},
}

// acquireParsedRequest gets a zeroed parsedRequest from the pool.
func acquireParsedRequest() *parsedRequest {
	pr := parsedRequestPool.Get().(*parsedRequest)
	*pr = parsedRequest{} // zero all fields
	return pr
}

// releaseParsedRequest returns a parsedRequest to the pool.
func releaseParsedRequest(pr *parsedRequest) {
	pr.body = nil // release reference to body bytes
	pr.routed = nil
	parsedRequestPool.Put(pr)
}

type providerRaceStat struct {
	Attempts      int
	EmptyRuns     int
	CooldownUntil time.Time
}

type providerExecOutcome struct {
	resp               *http.Response
	upstreamFormat     ProviderType
	resolvedProvider   string
	resolvedProviderID string
	resolvedModel      string
}

const providerRaceStatWindow = 100

func (ph *ProxyHandler) isSingleProviderMode(mode providerpool.RoutingMode) bool {
	if ph.providerPool == nil || ph.providerPool.Registry == nil {
		return false
	}
	providers := ph.providerPool.Registry.ListEnabled()
	count := 0
	for _, p := range providers {
		if mode == providerpool.RoutingModeCloud && p.Location != providerpool.ProviderLocationCloud {
			continue
		}
		if mode == providerpool.RoutingModeLocal && p.Location != providerpool.ProviderLocationLocal {
			continue
		}
		count++
		if count > 1 {
			return false
		}
	}
	return count == 1
}

// isTransientNetworkError returns true if the error is a transient network error
// that should be retried on the same provider (connection reset, EOF, timeout, etc.).
// These are Go-level errors from client.Do(), not HTTP status codes.
func isTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// io.EOF / io.ErrUnexpectedEOF — connection dropped
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	// syscall errors: connection reset, broken pipe, connection refused
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	// net.Error with Timeout()
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	// net.OpError — only retry if timeout or temporary
	var opErr *net.OpError
	if errors.As(err, &opErr) && (opErr.Timeout() || opErr.Temporary()) { //nolint:staticcheck // Temporary is deprecated but still useful
		return true
	}
	// String-based fallback for wrapped errors
	msg := err.Error()
	if strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "TLS handshake timeout") ||
		strings.Contains(msg, "server closed idle connection") ||
		strings.Contains(msg, "http2: timeout awaiting response headers") {
		return true
	}
	return false
}

// probeWithRetry wraps AuthProber.ProbeAndForward with transparent retry for
// transient network errors. Retries indefinitely with increasing backoff
// (500ms → 1s → 2s → 4s → 8s → 15s cap) until the request succeeds or the
// context is cancelled (e.g. user disconnects).
func (ph *ProxyHandler) probeWithRetry(
	ctx context.Context,
	provider *providerpool.Provider,
	apiKey *providerpool.APIKey,
	effectiveFormat providerpool.APIFormat,
	buildReq func() (*http.Request, error),
	doReq func(*http.Request) (*http.Response, error),
) (*http.Response, error) {
	const maxDelay = 15 * time.Second

	var lastErr error
	delay := 500 * time.Millisecond
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			slog.Warn("[proxy] retrying after transient network error",
				"provider", provider.ID, "attempt", attempt+1, "delay", delay, "error", lastErr)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			// Exponential backoff capped at maxDelay
			delay = delay * 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}

		resp, err := ph.authProber.ProbeAndForward(provider, apiKey, effectiveFormat, buildReq, doReq)
		if err == nil {
			return resp, nil
		}
		if !isTransientNetworkError(err) {
			return nil, err
		}
		lastErr = err
	}
}

func shouldRetryTransientUpstream5xx(singleProvider bool, statusCode int, body []byte, attempt int) bool {
	if !singleProvider || attempt >= 2 {
		return false
	}
	switch statusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
	default:
		return false
	}
	// Don't retry errors that are likely permanent/misconfiguration.
	if isFormatMismatchError(statusCode, body) || isModelNotConfiguredError(statusCode, body) || isAuthRelatedBody(body) {
		return false
	}
	return true
}

func transientUpstreamRetryDelay(attempt int) time.Duration {
	switch attempt {
	case 0:
		return 500 * time.Millisecond
	case 1:
		return 1500 * time.Millisecond
	default:
		return 3 * time.Second
	}
}

// readBody reads an io.Reader into a []byte using a pooled buffer.
// The returned slice owns its memory (copied from pool buffer).
// For small bodies (≤8KB), uses a stack-friendly path.
func readBody(r io.Reader) ([]byte, error) {
	buf := bodyBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	if _, err := buf.ReadFrom(r); err != nil {
		bodyBufferPool.Put(buf)
		return nil, err
	}
	// Steal the buffer's backing array if it's reasonably sized.
	// This avoids the copy — buf.Bytes() returns a slice of the internal array,
	// and we don't return buf to the pool so the caller owns the memory.
	data := buf.Bytes()
	if buf.Cap() <= 1<<20 { // ≤1MB: don't pool, let caller own it
		return data, nil
	}
	// Oversized: copy and return buffer to pool
	out := make([]byte, len(data))
	copy(out, data)
	bodyBufferPool.Put(buf)
	return out, nil
}

// readErrorBody reads a small error response body directly without pool overhead.
// Error bodies are typically <1KB, so io.ReadAll is cheaper than pool get/put/copy.
func readErrorBody(r io.Reader) []byte {
	data, _ := io.ReadAll(io.LimitReader(r, 4096))
	return data
}

// ProxyHandler handles incoming proxy requests.
//
// Architecture:
//
//	Client --[Proxy API Key]--> Proxy --[Provider API Key]--> Upstream (Anthropic/OpenAI)
//
// - Proxy API Key: Used by Authenticator to validate client requests (optional)
// - Provider API Key: Retrieved from Provider Pool to call upstream APIs
//
// The Proxy uses Provider Pool's Router to:
// 1. Select the best provider based on model and routing strategy
// 2. Get the API key for the selected provider
// 3. Forward the request to the upstream provider
//
// Routing modes (determined by API Key scope):
// - route:auto - Auto select best provider (default)
// - route:cloud - Force cloud provider (zimaos-blue-trial)
// - route:local - Force local provider
// Layout: hot-path fields first (same cache line), cold fields after.
type ProxyHandler struct {
	// Hot path — accessed on every request (first 64-byte cache line)
	connPool           *ConnectionPool    // HTTP connection pool
	providerPool       *providerpool.Pool // Provider Pool for routing and API keys
	authProber         *AuthProber        // Auth strategy probing with memory
	providerMemory     *ProviderMemory    // Provider capability memory
	providerRaceConfig ProviderRaceConfig // Concurrent provider race config
	providerRaceStats  map[string]*providerRaceStat
	providerRaceMu     sync.Mutex
	routingEnabled     atomic.Bool       // Toggle for model routing
	promptCacheEnabled atomic.Bool       // Toggle for Anthropic prompt caching
	promptCacheStats   *PromptCacheStats // Prompt cache effectiveness stats

	// Warm path — accessed conditionally
	failover      *FailoverHandler          // Failover handler
	prunerMw      *pruner.Middleware        // Context pruner middleware (optional)
	prunerFactory func() *pruner.Middleware // Lazy factory for pruner middleware
	prunerOnce    sync.Once                 // Ensures pruner is created only once
	modelRouter   *ModelRouter              // Model family routing + background downgrade
	ruleEngine    *RuleEngine               // Condition-based tier routing
	tierResolver  *TierResolver             // Dynamic model tier classification

	// Cold path — rarely accessed per-request
	dataMasker      *DataMasker // Data masking for request/response content
	router          *Router     // Legacy router (fallback only)
	apiKeyValidator func(key string) ([]string, error)
	routingStats    *RoutingStats           // Routing cost savings tracker
	pipelineStats   *PipelineStatsCollector // Unified pipeline stats collector
	oauthManager    OAuthTokenProvider      // OAuth token provider (optional)
	sessionMonitor  *SessionMonitor         // Session monitor for security tracking
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(router *Router, connPool *ConnectionPool, failover *FailoverHandler) *ProxyHandler {
	ph := &ProxyHandler{
		router:             router,
		connPool:           connPool,
		failover:           failover,
		promptCacheStats:   &PromptCacheStats{},
		providerRaceConfig: normalizeProviderRaceConfig(DefaultProviderRaceConfig()),
		providerRaceStats:  make(map[string]*providerRaceStat),
	}
	ph.routingEnabled.Store(true)
	ph.authProber = NewAuthProber() // default prober, can be overridden
	ph.providerMemory = NewProviderMemory()
	ph.routingStats = NewRoutingStats()
	return ph
}

// SetProviderPool sets the Provider Pool for routing and API key lookup.
// Triggers background auth probing for all enabled providers so the first
// real request hits the cached-strategy fast path instead of probing live.
func (ph *ProxyHandler) SetProviderPool(pool *providerpool.Pool) {
	ph.providerPool = pool
	if pool != nil {
		go ph.warmAuthStrategies()
	}
}

// SetPruner sets the context pruner middleware for the proxy handler.
func (ph *ProxyHandler) SetPruner(mw *pruner.Middleware) {
	ph.prunerMw = mw
	ph.prunerOnce.Do(func() {}) // mark as initialized so factory won't run
}

// SetPrunerFactory sets a lazy factory that creates the pruner middleware on first use.
// The factory is called at most once, on the first request that needs pruning.
func (ph *ProxyHandler) SetPrunerFactory(factory func() *pruner.Middleware) {
	ph.prunerFactory = factory
}

// SetProviderRaceConfig sets concurrent provider race behavior.
func (ph *ProxyHandler) SetProviderRaceConfig(cfg ProviderRaceConfig) {
	ph.providerRaceMu.Lock()
	defer ph.providerRaceMu.Unlock()
	ph.providerRaceConfig = normalizeProviderRaceConfig(cfg)
}

func normalizeProviderRaceConfig(cfg ProviderRaceConfig) ProviderRaceConfig {
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = 2
	}
	if cfg.MinProviders <= 0 {
		cfg.MinProviders = 2
	}
	if cfg.EmptyRateMinSamples <= 0 {
		cfg.EmptyRateMinSamples = 10
	}
	if cfg.EmptyRateCooldown <= 0 {
		cfg.EmptyRateCooldown = 2 * time.Minute
	}
	if cfg.EmptyRateCooldownThreshold < 0 {
		cfg.EmptyRateCooldownThreshold = 0
	}
	if cfg.EmptyRateSinkThreshold < 0 {
		cfg.EmptyRateSinkThreshold = 0
	}
	if cfg.EmptyRateExcludeThreshold < 0 {
		cfg.EmptyRateExcludeThreshold = 0
	}
	if cfg.EmptyRateCooldownThreshold > 1 {
		cfg.EmptyRateCooldownThreshold = 1
	}
	if cfg.EmptyRateSinkThreshold > 1 {
		cfg.EmptyRateSinkThreshold = 1
	}
	if cfg.EmptyRateExcludeThreshold > 1 {
		cfg.EmptyRateExcludeThreshold = 1
	}
	return cfg
}

func (ph *ProxyHandler) getProviderRaceConfig() ProviderRaceConfig {
	ph.providerRaceMu.Lock()
	defer ph.providerRaceMu.Unlock()
	return ph.providerRaceConfig
}

func (ph *ProxyHandler) getProviderRaceStat(providerID string) providerRaceStat {
	ph.providerRaceMu.Lock()
	defer ph.providerRaceMu.Unlock()
	if stat, ok := ph.providerRaceStats[providerID]; ok && stat != nil {
		return *stat
	}
	return providerRaceStat{}
}

func (ph *ProxyHandler) recordProviderRaceAttempt(providerID string, emptyRun bool) {
	cfg := ph.getProviderRaceConfig()
	if !cfg.Enabled || providerID == "" {
		return
	}

	ph.providerRaceMu.Lock()
	defer ph.providerRaceMu.Unlock()

	stat := ph.providerRaceStats[providerID]
	if stat == nil {
		stat = &providerRaceStat{}
		ph.providerRaceStats[providerID] = stat
	}
	stat.Attempts++
	if emptyRun {
		stat.EmptyRuns++
	}
	// Keep bounded moving history so old incidents naturally decay.
	if stat.Attempts > providerRaceStatWindow {
		stat.Attempts = (stat.Attempts + 1) / 2
		stat.EmptyRuns = (stat.EmptyRuns + 1) / 2
	}
	if stat.Attempts >= cfg.EmptyRateMinSamples && cfg.EmptyRateCooldownThreshold > 0 {
		emptyRate := float64(stat.EmptyRuns) / float64(stat.Attempts)
		if emptyRate >= cfg.EmptyRateCooldownThreshold {
			until := timeutil.NowTime().Add(cfg.EmptyRateCooldown)
			if until.After(stat.CooldownUntil) {
				stat.CooldownUntil = until
			}
		}
	}
}

func (ph *ProxyHandler) isProviderInRaceCooldown(providerID string) bool {
	stat := ph.getProviderRaceStat(providerID)
	return !stat.CooldownUntil.IsZero() && timeutil.NowTime().Before(stat.CooldownUntil)
}

func isEmptyRunError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "returned no response") ||
		strings.Contains(msg, "returned empty streaming response")
}

func hasToolMessagesInRequest(body []byte) bool {
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return false
	}
	for _, msg := range messages.Array() {
		role := strings.ToLower(strings.TrimSpace(msg.Get("role").Str))
		if role == "tool" {
			return true
		}
		toolCalls := msg.Get("tool_calls")
		if toolCalls.Exists() && toolCalls.IsArray() && len(toolCalls.Array()) > 0 {
			return true
		}
	}
	return false
}

// ensurePruner lazily initializes the pruner middleware via factory if not yet created.
func (ph *ProxyHandler) ensurePruner() *pruner.Middleware {
	if ph.prunerMw != nil {
		return ph.prunerMw
	}
	if ph.prunerFactory != nil {
		ph.prunerOnce.Do(func() {
			ph.prunerMw = ph.prunerFactory()
		})
	}
	return ph.prunerMw
}

// OAuthTokenProvider provides OAuth access tokens for providers.
type OAuthTokenProvider interface {
	GetAccessToken(providerID string) (string, error)
	GetCopilotAccessToken(providerID string) (string, error)
	GetCopilotEndpoint() string
}

// SetOAuthManager sets the OAuth token provider for OAuth-authenticated providers.
func (ph *ProxyHandler) SetOAuthManager(m OAuthTokenProvider) {
	ph.oauthManager = m
}

// SetAPIKeyValidator sets the API key validator function.
func (ph *ProxyHandler) SetAPIKeyValidator(validator func(key string) ([]string, error)) {
	ph.apiKeyValidator = validator
}

// SetModelRouter sets the model router for family-based routing and background downgrade.
func (ph *ProxyHandler) SetModelRouter(mr *ModelRouter) {
	ph.modelRouter = mr
}

// SetRuleEngine sets the condition-based rule engine for tier routing.
func (ph *ProxyHandler) SetRuleEngine(re *RuleEngine) {
	ph.ruleEngine = re
}

// SetTierResolver sets the dynamic tier resolver and wires it into the
// rule engine and model router for tier-based model resolution.
func (ph *ProxyHandler) SetTierResolver(tr *TierResolver) {
	ph.tierResolver = tr
	if ph.ruleEngine != nil {
		ph.ruleEngine.SetTierResolver(tr)
	}
	if ph.modelRouter != nil {
		ph.modelRouter.SetTierResolver(tr)
	}
}

// SetAuthProber sets the auth strategy prober.
func (ph *ProxyHandler) SetAuthProber(ap *AuthProber) {
	ph.authProber = ap
}

// SetDataMasker sets the data masker for request/response content masking.
func (ph *ProxyHandler) SetDataMasker(dm *DataMasker) {
	ph.dataMasker = dm
}

// SetSessionMonitor wires session monitoring into the real request chain.
func (ph *ProxyHandler) SetSessionMonitor(sm *SessionMonitor) {
	ph.sessionMonitor = sm
}

// GetProviderMemory returns the provider memory instance for external access.
func (ph *ProxyHandler) GetProviderMemory() *ProviderMemory {
	return ph.providerMemory
}

// IsRoutingEnabled returns whether model routing is enabled.
func (ph *ProxyHandler) IsRoutingEnabled() bool {
	return ph.routingEnabled.Load()
}

// SetRoutingEnabled toggles model routing on/off at runtime.
func (ph *ProxyHandler) SetRoutingEnabled(enabled bool) {
	ph.routingEnabled.Store(enabled)
}

// IsPromptCacheEnabled returns whether Anthropic prompt caching is enabled.
func (ph *ProxyHandler) IsPromptCacheEnabled() bool {
	return ph.promptCacheEnabled.Load()
}

// SetPromptCacheEnabled toggles Anthropic prompt caching on/off at runtime.
func (ph *ProxyHandler) SetPromptCacheEnabled(enabled bool) {
	ph.promptCacheEnabled.Store(enabled)
}

// GetPromptCacheStats returns prompt cache stats snapshot.
func (ph *ProxyHandler) GetPromptCacheStats() PromptCacheSnapshot {
	if ph.promptCacheStats == nil {
		return PromptCacheSnapshot{}
	}
	return ph.promptCacheStats.Snapshot()
}

// GetRoutingRules returns the current routing rules with their enabled state.
func (ph *ProxyHandler) GetRoutingRules() []RoutingRule {
	if ph.ruleEngine == nil {
		return nil
	}
	return ph.ruleEngine.GetRules()
}

// SetRoutingRuleEnabled enables or disables a single routing rule by name.
func (ph *ProxyHandler) SetRoutingRuleEnabled(name string, enabled bool) bool {
	if ph.ruleEngine == nil {
		return false
	}
	return ph.ruleEngine.SetRuleEnabled(name, enabled)
}

// GetRoutingStats returns a snapshot of routing cost savings.
func (ph *ProxyHandler) GetRoutingStats() RoutingStatsSnapshot {
	return ph.routingStats.Snapshot()
}

// GetRoutingStatsRef returns the RoutingStats reference for external wiring.
func (ph *ProxyHandler) GetRoutingStatsRef() *RoutingStats {
	return ph.routingStats
}

// SetPipelineStats sets the unified pipeline stats collector.
func (ph *ProxyHandler) SetPipelineStats(ps *PipelineStatsCollector) {
	ph.pipelineStats = ps
}

// GetPipelineStats returns the pipeline stats collector (may be nil).
func (ph *ProxyHandler) GetPipelineStats() *PipelineStatsCollector {
	return ph.pipelineStats
}

// warmAuthStrategies probes auth strategies for all enabled providers in the background.
// This runs once at startup so the first real request hits the cached fast path.
func (ph *ProxyHandler) warmAuthStrategies() {
	pool := ph.providerPool
	if pool == nil || pool.Registry == nil {
		return
	}

	providers := pool.Registry.ListEnabled()
	for _, p := range providers {
		// Skip if already cached (e.g. from a previous warmup)
		if _, ok := ph.authProber.Recall(p.ID, p.BaseURL); ok {
			continue
		}

		apiKey, _ := pool.Registry.GetAPIKey(p.ID)
		if apiKey == nil || apiKey.Key == "" {
			// No key → AuthNone, cache it directly
			ph.authProber.Remember(p.ID, p.BaseURL, AuthNone)
			continue
		}

		// Try a lightweight HEAD/GET on the models endpoint to probe auth
		strategies := ph.authProber.Strategies(p, apiKey, p.APIFormat)
		for _, strat := range strategies {
			probeURL := strings.TrimSuffix(p.BaseURL, "/") + "/v1/models"
			req, err := http.NewRequest(http.MethodGet, probeURL, nil)
			if err != nil {
				continue
			}
			ph.authProber.Apply(req, strat, apiKey, p)

			var client *http.Client
			if p.SkipTLSVerify {
				client = ph.connPool.GetInsecureClient(p.Name)
			} else {
				client = ph.connPool.GetClient(p.Name)
			}

			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			if !isAuthError(resp.StatusCode) {
				ph.authProber.Remember(p.ID, p.BaseURL, strat)
				slog.Debug("[proxy] auth warmup success", "provider", p.ID, "strategy", strat.String())
				break
			}
		}
	}
	slog.Info("[proxy] auth warmup complete", "providers", len(providers))

	// Phase 2: probe tool call support on each provider
	ph.warmToolCallSupport(providers)
}

// toolCallProbeBody is a minimal OpenAI chat completion request with a tool definition.
// Used to detect whether a provider supports function calling (tool_use).
// The request is intentionally minimal to minimize cost — most providers reject or
// return a very short response. We only care about the HTTP status code.
var toolCallProbeBody = []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"noop","description":"no-op","parameters":{"type":"object","properties":{}}}}],"max_tokens":1}`)

// DisableResponsesContinuationHeader tells proxy request shaping logic to strip
// previous_response_id before forwarding to Responses endpoints.
const DisableResponsesContinuationHeader = "X-Zima-Disable-Responses-Continuation"

// warmToolCallSupport sends a minimal tool-bearing request to each provider to detect
// whether it supports native function calling. Results are cached in ProviderMemory
// so the routing layer can skip providers that would 422 on tool_calls.
func (ph *ProxyHandler) warmToolCallSupport(providers []*providerpool.Provider) {
	pool := ph.providerPool
	if pool == nil || pool.Registry == nil {
		return
	}

	for _, p := range providers {
		burl := p.EffectiveBaseURL()
		// Responses endpoints don't have a reliable tool-probe signal and the probe
		// itself may cause protocol-side effects. Skip warm probing entirely.
		if isResponsesEndpointBaseURL(burl) {
			slog.Debug("[proxy] tool probe skipped for responses endpoint", "provider", p.ID)
			continue
		}

		// Skip if already probed
		if _, ok := ph.providerMemory.RecallToolCap(p.ID, burl); ok {
			continue
		}

		apiKey, _ := pool.Registry.GetAPIKey(p.ID)
		if apiKey == nil || apiKey.Key == "" {
			continue
		}

		// Recall auth strategy (just probed in phase 1)
		authStrat, _ := ph.authProber.Recall(p.ID, burl)

		probeURL := strings.TrimSuffix(burl, "/") + "/v1/chat/completions"
		probeBody := toolCallProbeBody
		req, err := http.NewRequest(http.MethodPost, probeURL, io.NopCloser(bytes.NewReader(probeBody)))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		ph.authProber.Apply(req, authStrat, apiKey, p)

		var client *http.Client
		if p.SkipTLSVerify {
			client = ph.connPool.GetInsecureClient(p.Name)
		} else {
			client = ph.connPool.GetClient(p.Name)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req = req.WithContext(ctx)
		resp, err := client.Do(req)
		cancel()
		if err != nil {
			slog.Debug("[proxy] tool probe failed", "provider", p.ID, "error", err)
			continue
		}
		errBody := readErrorBody(resp.Body)
		resp.Body.Close()

		if isFormatMismatchError(resp.StatusCode, errBody) {
			// Explicit format/tool schema mismatch means no native tool support.
			ph.providerMemory.RememberToolCap(p.ID, burl, ToolCapNone)
			slog.Info("[proxy] tool probe: no tool support", "provider", p.ID, "status", resp.StatusCode)
		} else {
			// Any other response (200, 400 validation, 401, 429, 500, etc.) doesn't
			// prove missing tool support. Keep provider eligible for tool calls.
			ph.providerMemory.RememberToolCap(p.ID, burl, ToolCapNative)
			slog.Debug("[proxy] tool probe: tools supported", "provider", p.ID, "status", resp.StatusCode)
		}
	}
	slog.Info("[proxy] tool call probe complete")
}

// extractRoutingMode extracts routing mode from API key scopes.
func (ph *ProxyHandler) extractRoutingMode(r *http.Request) string {
	apiKey := r.Header.Get("x-api-key")
	if apiKey == "" {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			apiKey = auth[7:] // len("Bearer ") = 7
		}
	}

	if apiKey == "" || ph.apiKeyValidator == nil {
		return "auto"
	}

	scopes, err := ph.apiKeyValidator(apiKey)
	if err != nil {
		return "auto"
	}

	for _, scope := range scopes {
		switch scope {
		case "route:cloud":
			return "cloud"
		case "route:local":
			return "local"
		}
	}
	return "auto"
}

func (ph *ProxyHandler) executeOnRouteResult(
	r *http.Request,
	result *providerpool.RouteResult,
	pr *parsedRequest,
	hasTools bool,
) (*providerExecOutcome, error) {
	pid := result.Provider.ID
	burl := result.Provider.EffectiveBaseURL()

	// Tool cap check: skip providers known to not support function calling.
	// This avoids wasting a round-trip on providers that will 422.
	if hasTools && !pr.singleProvider {
		if cap, ok := ph.providerMemory.RecallToolCap(pid, burl); ok && cap == ToolCapNone {
			// Responses-based endpoints may have been mis-probed previously via
			// /v1/chat/completions. Drop stale cache and re-try once on real traffic.
			if isResponsesEndpointBaseURL(burl) {
				ph.providerMemory.ForgetToolCap(pid, burl)
				slog.Info("[proxy] cleared stale no-tool cache for responses endpoint", "provider", pid)
			} else {
				slog.Debug("[proxy] skipping provider (no tool support)", "provider", pid)
				return nil, fmt.Errorf("provider %s does not support tool calls", pid)
			}
		}
	}

	// Throttle check: skip provider if recently 429'd
	if !pr.singleProvider && ph.providerMemory.IsThrottled(pid, burl) {
		slog.Debug("[proxy] skipping throttled provider", "provider", pid)
		return nil, fmt.Errorf("provider %s is throttled", pid)
	}

	// Try all Model × Format combinations on this provider
	resp, format, _, tryErr := ph.tryOnProvider(r, result, pr)
	if tryErr != nil {
		return nil, tryErr
	}
	if resp == nil {
		// All format/model combos failed. Do NOT infer ToolCapNone here —
		// the failure may be caused by network issues, model not found, 404,
		// etc., not by lack of tool support. Tool capability is only reliably
		// determined by the warmToolCallSupport probe (explicit format mismatch
		// on a tool-bearing request).
		return nil, fmt.Errorf("provider %s returned no response", pid)
	}

	// For streaming responses, peek at the first bytes to detect empty streams.
	// Some providers return HTTP 200 with an empty body or only "[DONE]".
	// Detecting this early allows fallback/race to try the next provider.
	if pr.streaming && resp.Body != nil {
		peekBuf := make([]byte, 32)
		n, peekErr := resp.Body.Read(peekBuf)
		if n == 0 || peekErr != nil {
			resp.Body.Close()
			slog.Warn("[proxy] streaming response body empty", "provider", pid, "error", peekErr)
			return nil, fmt.Errorf("provider %s returned empty streaming response", pid)
		}
		// Reconstruct body: peeked bytes + rest of original body
		resp.Body = &peekReader{prefix: peekBuf[:n], rest: resp.Body}
	}

	outcome := &providerExecOutcome{
		resp:               resp,
		resolvedProviderID: result.Provider.ID,
	}
	switch format {
	case providerpool.APIFormatAnthropic:
		outcome.upstreamFormat = ProviderTypeAnthropic
	case providerpool.APIFormatCloudCode:
		outcome.upstreamFormat = ProviderTypeCloudCode
	}
	// Remember tool support on success
	if hasTools {
		ph.providerMemory.RememberToolCap(pid, burl, ToolCapNative)
	}
	outcome.resolvedProvider = result.Provider.Name
	if outcome.resolvedProvider == "" {
		outcome.resolvedProvider = result.Provider.ID
	}
	if result.Model != nil && result.Model.ID != "" {
		outcome.resolvedModel = result.Model.ID
	}
	return outcome, nil
}

func (ph *ProxyHandler) applyProviderExecOutcome(ctx context.Context, pr *parsedRequest, out *providerExecOutcome) {
	if out == nil {
		return
	}
	pr.upstreamFormat = out.upstreamFormat
	pr.resolvedProvider = out.resolvedProvider
	pr.resolvedProviderID = out.resolvedProviderID
	pr.resolvedModel = out.resolvedModel
	if rr := getResolvedRoute(ctx); rr != nil {
		rr.Provider = out.resolvedProvider
		rr.ProviderID = out.resolvedProviderID
		rr.Model = out.resolvedModel
	}
}

func (ph *ProxyHandler) routeResultForCandidate(c *providerpool.RouteCandidate) *providerpool.RouteResult {
	res := &providerpool.RouteResult{
		Provider: c.Provider,
		Model:    c.Model,
	}
	if ph.providerPool == nil || ph.providerPool.Registry == nil {
		return res
	}
	if apiKey, err := ph.providerPool.Registry.GetAPIKey(c.Provider.ID); err == nil {
		res.APIKey = apiKey
	}
	if res.APIKey == nil {
		if oauthCfg, err := ph.providerPool.Registry.GetOAuthConfig(c.Provider.ID); err == nil {
			res.OAuth = oauthCfg
		}
	}
	return res
}

func (ph *ProxyHandler) executeWithProviderRace(
	r *http.Request,
	routeReq *providerpool.RouteRequest,
	pr *parsedRequest,
	hasTools bool,
) (*providerExecOutcome, error) {
	cfg := ph.getProviderRaceConfig()
	if !cfg.Enabled || ph.providerPool == nil || ph.providerPool.Router == nil {
		return nil, fmt.Errorf("provider race disabled")
	}

	primary, err := ph.providerPool.Router.Route(routeReq)
	if err != nil {
		return nil, err
	}

	candidates := make([]*providerpool.RouteCandidate, 0, 1+len(primary.Fallbacks))
	candidates = append(candidates, &providerpool.RouteCandidate{
		Provider: primary.Provider,
		Model:    primary.Model,
		Score:    1,
	})
	candidates = append(candidates, primary.Fallbacks...)
	if len(candidates) < cfg.MinProviders {
		return nil, fmt.Errorf("not enough providers for race")
	}

	raced := make([]*providerpool.RouteCandidate, 0, len(candidates))
	sink := make([]*providerpool.RouteCandidate, 0, len(candidates))
	for _, c := range candidates {
		if ph.isProviderInRaceCooldown(c.Provider.ID) {
			continue
		}
		stat := ph.getProviderRaceStat(c.Provider.ID)
		if stat.Attempts >= cfg.EmptyRateMinSamples {
			rate := float64(stat.EmptyRuns) / float64(stat.Attempts)
			if cfg.EmptyRateExcludeThreshold > 0 && rate >= cfg.EmptyRateExcludeThreshold {
				continue
			}
			if cfg.EmptyRateSinkThreshold > 0 && rate >= cfg.EmptyRateSinkThreshold {
				sink = append(sink, c)
				continue
			}
		}
		raced = append(raced, c)
	}
	raced = append(raced, sink...)
	if len(raced) < cfg.MinProviders {
		return nil, fmt.Errorf("not enough eligible providers for race")
	}
	if len(raced) > cfg.MaxParallel {
		raced = raced[:cfg.MaxParallel]
	}

	type raceResult struct {
		providerID string
		outcome    *providerExecOutcome
		err        error
		latency    time.Duration
	}

	raceCtx, cancel := context.WithCancel(r.Context())
	defer cancel()

	ch := make(chan raceResult, len(raced))
	var wg sync.WaitGroup
	for _, candidate := range raced {
		wg.Add(1)
		go func(c *providerpool.RouteCandidate) {
			defer wg.Done()
			start := timeutil.NowTime()
			res := ph.routeResultForCandidate(c)
			outcome, execErr := ph.executeOnRouteResult(r.Clone(raceCtx), res, pr, hasTools)
			ch <- raceResult{
				providerID: c.Provider.ID,
				outcome:    outcome,
				err:        execErr,
				latency:    timeutil.SinceTime(start),
			}
		}(candidate)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	var winner *providerExecOutcome
	var firstErr error
	tried := make([]string, 0, len(raced))
	for item := range ch {
		tried = append(tried, item.providerID)
		if item.err == nil && item.outcome != nil && item.outcome.resp != nil {
			ph.providerPool.Router.UpdateLatency(item.providerID, item.latency)
			ph.providerPool.Router.RecordSuccess(item.providerID)
			ph.recordProviderRaceAttempt(item.providerID, false)
			if winner == nil {
				winner = item.outcome
				cancel()
			} else {
				item.outcome.resp.Body.Close()
			}
			continue
		}
		ph.providerPool.Router.UpdateLatency(item.providerID, item.latency*2)
		ph.providerPool.Router.RecordFailure(item.providerID, item.err)
		ph.recordProviderRaceAttempt(item.providerID, isEmptyRunError(item.err))
		if firstErr == nil && item.err != nil {
			firstErr = item.err
		}
	}
	if winner != nil {
		return winner, nil
	}

	// Race failed: continue serial failover on untried providers.
	fallbackReq := *routeReq
	fallbackReq.Exclude = append(append([]string{}, routeReq.Exclude...), tried...)
	var serialWinner *providerExecOutcome
	serialErr := ph.providerPool.Router.RouteWithFallback(r.Context(), &fallbackReq, func(result *providerpool.RouteResult) error {
		out, execErr := ph.executeOnRouteResult(r, result, pr, hasTools)
		if execErr != nil {
			return execErr
		}
		serialWinner = out
		return nil
	})
	if serialErr != nil {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, serialErr
	}
	if serialWinner == nil {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, ErrAllProvidersFailed
	}
	return serialWinner, nil
}

// ServeHTTP implements http.Handler.
func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Handle /v1/models specially
	if r.URL.Path == "/v1/models" || strings.HasSuffix(r.URL.Path, "/models") {
		ph.handleModels(w, r)
		return
	}

	// Read request body
	bodyBytes, _ := readBody(r.Body)
	r.Body.Close()

	// Apply context pruner to reduce token usage (if enabled)
	if mw := ph.ensurePruner(); mw != nil && mw.Enabled() {
		// Read prune stats slot from context (set by chat handler before bridge call).
		// If present, the middleware populates it with per-request stats.
		if pruned, err := mw.ProcessRequest(r.Context(), bodyBytes); err == nil {
			bodyBytes = pruned
		}
	}

	// Parse body ONCE — extract model and streaming flag with gjson (zero-alloc)
	pr := acquireParsedRequest()
	defer releaseParsedRequest(pr)
	pr.body = bodyBytes
	if gjson.ValidBytes(bodyBytes) {
		bodyStr := unsafeString(bodyBytes)
		pr.model = gjson.Get(bodyStr, "model").Str
		pr.requestedModel = pr.model
		pr.streaming = gjson.Get(bodyStr, "stream").Bool()
	}

	// "auto" means "use any available model" — normalize to empty so the router
	// picks from allCandidates and allModelsForProvider doesn't send "auto" upstream.
	if pr.model == "auto" {
		pr.model = ""
	}

	// Model routing: evaluate rule engine to potentially swap to a cheaper model
	pr.originalModel = pr.model
	ph.applyModelRouting(r, pr)

	// Forward with auth probing + provider failover
	if ph.providerPool == nil || ph.providerPool.Router == nil {
		http.Error(w, "no provider pool configured", http.StatusServiceUnavailable)
		return
	}
	ph.setRouteHeaders(w, pr)
	routingMode := ph.extractRoutingMode(r)
	slog.Info("[proxy] routing request body",
		"requested_model", pr.requestedModel,
		"model", pr.model,
		"streaming", pr.streaming,
		"mode", routingMode,
		"body", ph.requestBodyForLog(pr.body),
	)

	routeReq := &providerpool.RouteRequest{
		ModelID:             pr.model,
		Mode:                providerpool.RoutingMode(routingMode),
		PreferredProviderID: GetPinnedProvider(r.Context()),
	}
	pr.singleProvider = ph.isSingleProviderMode(routeReq.Mode)

	// When the request includes tools, require providers that support function calling.
	// This prevents routing to providers/models that would reject tool_calls.
	hasTools := gjson.GetBytes(pr.body, "tools").Exists()
	hasToolMessages := hasToolMessagesInRequest(pr.body)
	if hasTools {
		routeReq.RequireCap = &providerpool.ModelCapabilities{FunctionCall: true}
	}
	slog.Info("[proxy] route request",
		"requested_model", pr.requestedModel,
		"model", pr.model,
		"mode", routeReq.Mode,
		"preferred_provider", routeReq.PreferredProviderID,
		"has_tools", hasTools,
		"has_tool_messages", hasToolMessages,
		"single_provider_mode", pr.singleProvider,
	)

	// Failover is enabled by default; only disabled when explicitly configured off
	failoverDisabled := ph.failover != nil && ph.failover.Config() != nil && !ph.failover.Config().Enabled

	var finalResp *http.Response
	applyOutcome := func(out *providerExecOutcome) {
		if out == nil {
			return
		}
		ph.applyProviderExecOutcome(r.Context(), pr, out)
		finalResp = out.resp
	}
	executeOnProvider := func(result *providerpool.RouteResult) error {
		outcome, execErr := ph.executeOnRouteResult(r, result, pr, hasTools)
		if execErr != nil {
			return execErr
		}
		applyOutcome(outcome)
		return nil
	}

	var err error
	raceTried := false
	if !failoverDisabled {
		if outcome, raceErr := ph.executeWithProviderRace(r, routeReq, pr, hasTools); raceErr == nil {
			applyOutcome(outcome)
			raceTried = true
		}
	}
	if finalResp == nil && failoverDisabled {
		// Failover explicitly disabled: only try the primary provider
		result, routeErr := ph.providerPool.Router.Route(routeReq)
		if routeErr != nil {
			err = routeErr
		} else {
			err = executeOnProvider(result)
		}
	} else if finalResp == nil && !raceTried {
		err = ph.providerPool.Router.RouteWithFallback(r.Context(), routeReq, executeOnProvider)
	}

	// Startup race: if no providers are available and the pool hasn't finished its
	// initial model refresh, wait briefly and retry. This avoids 503s for requests
	// that arrive before Pool.Start() completes the background model fetch.
	if finalResp == nil && errors.Is(err, providerpool.ErrNoAvailableProvider) && !ph.providerPool.IsReady() {
		slog.Info("[proxy] no providers yet, waiting for pool readiness", "model", pr.model)
		if ph.providerPool.WaitReady(5 * time.Second) {
			slog.Info("[proxy] pool ready, retrying route", "model", pr.model)
			if failoverDisabled {
				result, routeErr := ph.providerPool.Router.Route(routeReq)
				if routeErr != nil {
					err = routeErr
				} else {
					err = executeOnProvider(result)
				}
			} else {
				err = ph.providerPool.Router.RouteWithFallback(r.Context(), routeReq, executeOnProvider)
			}
		}
	}

	// Retry with tools stripped: if all providers failed and the request had tools,
	// strip the tools/tool_choice fields and retry. This handles providers that reject
	// the request body because they don't understand tool-related fields (422).
	// Skip if no providers exist at all — stripping fields won't conjure providers.
	// Skip if messages contain tool_calls or tool role — this is a tool round where
	// stripping tools would leave orphaned tool_calls/tool_result, causing worse errors.
	if err != nil && hasTools && !hasToolMessages && !errors.Is(err, providerpool.ErrNoAvailableProvider) {
		strippedBody := pr.body
		if b, e := sjson.DeleteBytes(strippedBody, "tools"); e == nil {
			strippedBody = b
		}
		if b, e := sjson.DeleteBytes(strippedBody, "tool_choice"); e == nil {
			strippedBody = b
		}
		if len(strippedBody) != len(pr.body) {
			slog.Info("[proxy] retrying without tools after all providers failed",
				"model", pr.model, "original_error", err)
			pr.body = strippedBody
			hasTools = false
			routeReq.RequireCap = nil
			finalResp = nil
			if failoverDisabled {
				result, routeErr := ph.providerPool.Router.Route(routeReq)
				if routeErr != nil {
					err = routeErr
				} else {
					err = executeOnProvider(result)
				}
			} else {
				err = ph.providerPool.Router.RouteWithFallback(r.Context(), routeReq, executeOnProvider)
			}
		}
	}

	// Retry with images stripped: if still failing and the request has image content,
	// strip image_url entries from messages and retry. Some providers/models don't
	// support vision and reject multimodal content.
	// Skip if no providers exist at all.
	if err != nil && !errors.Is(err, providerpool.ErrNoAvailableProvider) {
		if strippedBody, didStrip := stripImageContent(pr.body); didStrip {
			slog.Info("[proxy] retrying without images after all providers failed",
				"model", pr.model, "original_error", err)
			pr.body = strippedBody
			finalResp = nil
			if failoverDisabled {
				result, routeErr := ph.providerPool.Router.Route(routeReq)
				if routeErr != nil {
					err = routeErr
				} else {
					err = executeOnProvider(result)
				}
			} else {
				err = ph.providerPool.Router.RouteWithFallback(r.Context(), routeReq, executeOnProvider)
			}
		}
	}

	if err != nil {
		slog.Error("[proxy] all providers failed",
			"model", pr.model,
			"mode", routeReq.Mode,
			"has_tools", hasTools,
			"has_tool_messages", hasToolMessages,
			"single_provider_mode", pr.singleProvider,
			"preferred_provider", routeReq.PreferredProviderID,
			"error", err,
		)
		// Propagate original status code so the bridge can distinguish
		// non-retryable errors from transient server failures.
		statusCode := http.StatusBadGateway
		errMsg := err.Error()
		switch {
		case errors.Is(err, providerpool.ErrNoAvailableProvider) || strings.Contains(errMsg, "no available provider"):
			// 503 = no provider can serve this model — retrying won't help.
			statusCode = http.StatusServiceUnavailable
		case strings.Contains(errMsg, "not configured on provider"):
			// Model/endpoint mismatch is not transient; surface as client error
			// so chat layer does not retry pre-content failures.
			statusCode = http.StatusBadRequest
		case strings.HasPrefix(errMsg, "upstream 400:"):
			statusCode = http.StatusBadRequest
		case strings.HasPrefix(errMsg, "provider returned "):
			var upstreamStatus int
			if _, scanErr := fmt.Sscanf(errMsg, "provider returned %d:", &upstreamStatus); scanErr == nil {
				if upstreamStatus >= 400 && upstreamStatus < 500 {
					statusCode = upstreamStatus
				}
			}
		case strings.Contains(errMsg, "overloaded (529)"):
			statusCode = 529
		case strings.Contains(errMsg, "throttled (429)"):
			statusCode = http.StatusTooManyRequests
		}
		http.Error(w, SanitizeError(err), statusCode)
		return
	}
	defer finalResp.Body.Close()
	slog.Debug("[proxy] upstream response", "status", finalResp.StatusCode, "streaming", pr.streaming, "content_type", finalResp.Header.Get("Content-Type"))

	ph.copyResponse(w, finalResp, pr, r)
}

// routeRequest uses Provider Pool Router to select provider.
func (ph *ProxyHandler) routeRequest(model string) (*providerpool.RouteResult, error) {
	return ph.routeRequestWithMode(model, "auto")
}

// routeRequestWithMode routes request with specific routing mode.
func (ph *ProxyHandler) routeRequestWithMode(model string, mode string) (*providerpool.RouteResult, error) {
	if ph.providerPool == nil {
		return nil, ErrNoAvailableProvider
	}

	return ph.providerPool.Router.Route(&providerpool.RouteRequest{
		ModelID: model,
		Mode:    providerpool.RoutingMode(mode),
	})
}

// buildUpstreamRequest creates an HTTP request to the upstream provider without auth headers.
// Auth is applied separately by AuthProber.
func (ph *ProxyHandler) buildUpstreamRequest(r *http.Request, route *providerpool.RouteResult, body []byte) (*http.Request, error) {
	return ph.buildUpstreamRequestWithFormat(r, route, body, route.Provider.APIFormat)
}

// buildUpstreamRequestWithFormat creates an upstream request using the given effective API format.
func (ph *ProxyHandler) buildUpstreamRequestWithFormat(r *http.Request, route *providerpool.RouteResult, body []byte, effectiveFormat providerpool.APIFormat) (*http.Request, error) {
	provider := route.Provider
	if disableResponsesContinuation(r) {
		body = stripPreviousResponseID(body)
	}

	// Use effective base URL (considers DetectedEndpoint)
	effectiveBaseURL := provider.EffectiveBaseURL()
	targetURL := provider.ParsedEffectiveBaseURL()
	if targetURL == nil {
		return nil, fmt.Errorf("invalid provider base URL: %s", effectiveBaseURL)
	}

	// Copilot: use dynamic endpoint from token response if available
	// The endpoint is returned from GetCopilotToken and cached in the OAuth manager.
	// We must call GetCopilotAccessToken here (not just GetCopilotEndpoint) to trigger
	// the token exchange and cache the endpoint BEFORE building the request.
	if provider.APIFormat == providerpool.APIFormatCopilot && ph.oauthManager != nil {
		// Try to get the Copilot access token - this will also fetch and cache the endpoint
		copilotToken, tokenErr := ph.oauthManager.GetCopilotAccessToken(provider.ID)
		if tokenErr != nil {
			slog.Warn("[proxy] failed to get Copilot access token, will use default endpoint", "provider", provider.ID, "error", tokenErr)
		} else if copilotToken != "" {
			// Now the endpoint should be cached
			copilotEndpoint := ph.oauthManager.GetCopilotEndpoint()
			if copilotEndpoint != "" {
				newURL, err := url.Parse(copilotEndpoint)
				if err == nil {
					targetURL.Host = newURL.Host
					targetURL.Scheme = newURL.Scheme
					slog.Info("[proxy] using dynamic Copilot endpoint", "provider", provider.ID, "endpoint", copilotEndpoint)
				}
			}
		}
	}

	// Build upstream URL - avoid duplicate path segments
	upstreamURL := *targetURL
	requestPath := r.URL.Path

	bridgeCtx := &UpstreamRequestBridgeContext{
		Route:              route,
		Provider:           provider,
		EffectiveFormat:    effectiveFormat,
		TargetURL:          targetURL,
		RequestPath:        requestPath,
		Body:               body,
		PromptCacheEnabled: ph.promptCacheEnabled.Load(),
	}
	ph.applyUpstreamRequestBridges(bridgeCtx)
	effectiveFormat = bridgeCtx.EffectiveFormat
	requestPath = bridgeCtx.RequestPath
	body = bridgeCtx.Body

	if targetURL.Path != "" && targetURL.Path != "/" {
		basePath := strings.TrimSuffix(targetURL.Path, "/")
		if strings.HasPrefix(requestPath, basePath) {
			// Request path already includes the base path (e.g. base=/v1, req=/v1/chat/completions)
			upstreamURL.Path = requestPath
		} else if idx := strings.LastIndex(basePath, "/v"); idx >= 0 {
			// Base URL ends with a version segment (e.g. /api/v1, /api/paas/v4).
			// If request path also starts with a version prefix (/v1/, /v2/, etc.),
			// strip it to avoid duplication like /api/v1/v1/... or /api/paas/v4/v1/...
			// This handles both same-version (OpenRouter /api/v1 + /v1/chat/completions)
			// and cross-version (GLM /api/paas/v4 + /v1/chat/completions) cases.
			if trimmed := stripVersionPrefix(requestPath); trimmed != "" {
				upstreamURL.Path = singleJoiningSlash(basePath, trimmed)
			} else {
				upstreamURL.Path = singleJoiningSlash(targetURL.Path, requestPath)
			}
		} else {
			upstreamURL.Path = singleJoiningSlash(targetURL.Path, requestPath)
		}
	} else {
		upstreamURL.Path = requestPath
	}

	// URI-based format enforcement: the final URL path is the source of truth.
	// Ensure the body format matches the endpoint, regardless of effectiveFormat.
	finalPath := upstreamURL.Path
	if strings.HasSuffix(finalPath, "/messages") && effectiveFormat != providerpool.APIFormatAnthropic {
		// Final URL is Anthropic endpoint but body wasn't converted — fix it.
		// This happens when base URL already contains /messages or format was misdetected.
		converted, _, convErr := sharedConverter.ConvertRequestWithCaching(body, ProviderTypeAnthropic, ph.promptCacheEnabled.Load())
		if convErr == nil {
			body = converted
			slog.Info("[proxy] URI fixup: endpoint is /messages, converted body to Anthropic",
				"provider", provider.ID, "final_path", finalPath)
		}
	}
	if strings.HasSuffix(finalPath, "/responses") &&
		gjson.GetBytes(body, "messages").Exists() {
		converted, convErr := convertOpenAIChatCompletionsToResponses(body)
		if convErr == nil {
			body = converted
			slog.Info("[proxy] URI fixup: endpoint is /responses, converted body to Responses",
				"provider", provider.ID, "final_path", finalPath)
		}
	}
	if strings.HasSuffix(finalPath, "/responses") {
		body = clampResponsesMaxOutputTokens(body)
	}
	if endpointFormat, ok := detectEndpointFixedFormatFromPath(finalPath); ok {
		effectiveFormat = endpointFormat
	}
	upstreamURL.RawQuery = r.URL.RawQuery

	fullURL := upstreamURL.String()
	slog.Debug("[proxy] upstream request", "url", fullURL, "method", r.Method, "format", effectiveFormat, "body_len", len(body))
	slog.Info("[proxy] upstream request body",
		"url", fullURL,
		"method", r.Method,
		"format", effectiveFormat,
		"body", ph.requestBodyForLog(body),
	)

	req, err := http.NewRequestWithContext(r.Context(), r.Method, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	copyHeaders(req.Header, r.Header)
	req.Host = targetURL.Host

	// Copilot API requires Editor-Version header for IDE authentication
	if provider.APIFormat == providerpool.APIFormatCopilot {
		req.Header.Set("Editor-Version", "vscode/1.85.0")
	}

	// CloudCode API requires specific headers for Google Cloud Code
	if effectiveFormat == providerpool.APIFormatCloudCode {
		providerType := "antigravity"
		if route.OAuth != nil && route.OAuth.ProviderType != "" {
			providerType = route.OAuth.ProviderType
		}
		switch providerType {
		case "antigravity":
			req.Header.Set("User-Agent", "antigravity/1.0")
			req.Header.Set("X-Goog-Api-Client", "google-cloud-sdk vscode_cloudshelleditor/0.1")
		case "gemini-cli":
			req.Header.Set("User-Agent", "google-api-nodejs-client/9.15.1")
			req.Header.Set("X-Goog-Api-Client", "gl-node/22.17.0")
		}
	}

	return req, nil
}

// tryModelAliases attempts common model name aliases when the original gets model_not_found.
func (ph *ProxyHandler) tryModelAliases(r *http.Request, result *providerpool.RouteResult, pr *parsedRequest, failedModel string, effectiveFormat providerpool.APIFormat) *http.Response {
	aliases, ok := ModelAliases[failedModel]
	if !ok {
		return nil
	}
	pid := result.Provider.ID
	burl := result.Provider.EffectiveBaseURL()

	// OAuth token handling for Copilot and CloudCode providers
	apiKeyToUse := result.APIKey
	if ph.oauthManager != nil {
		if result.Provider.APIFormat == providerpool.APIFormatCopilot {
			copilotToken, tokenErr := ph.oauthManager.GetCopilotAccessToken(result.Provider.ID)
			if tokenErr == nil && copilotToken != "" {
				apiKeyToUse = &providerpool.APIKey{Key: copilotToken}
			}
		} else if result.Provider.APIFormat == providerpool.APIFormatCloudCode {
			accessToken, tokenErr := ph.oauthManager.GetAccessToken(result.Provider.ID)
			if tokenErr == nil && accessToken != "" {
				apiKeyToUse = &providerpool.APIKey{Key: accessToken}
			}
		}
	}

	for _, alias := range aliases {
		aliasBody, err := sjson.SetBytes(pr.body, "model", alias)
		if err != nil {
			continue
		}
		slog.Debug("[proxy] trying model alias", "provider", pid, "original", failedModel, "alias", alias)

		resp, probeErr := ph.authProber.ProbeAndForward(
			result.Provider, apiKeyToUse, effectiveFormat,
			func() (*http.Request, error) {
				return ph.buildUpstreamRequestWithFormat(r, result, aliasBody, effectiveFormat)
			},
			func(req *http.Request) (*http.Response, error) {
				if result.Provider.SkipTLSVerify {
					return ph.connPool.GetInsecureClient(result.Provider.Name).Do(req)
				}
				return ph.connPool.GetClient(result.Provider.Name).Do(req)
			},
		)
		if probeErr != nil {
			continue
		}
		if resp.StatusCode < 400 {
			ph.providerMemory.RememberModelAlias(pid, burl, failedModel, alias)
			ph.providerMemory.RememberFormat(pid, burl, string(effectiveFormat))
			if effectiveFormat == providerpool.APIFormatAnthropic {
				pr.upstreamFormat = ProviderTypeAnthropic
			}
			return resp
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	return nil
}

// allFormatsForProvider returns format candidates to try for a provider.
// Persisted format first, then remembered (in-memory), then provider default, then remaining.
// Returns (buf, count, known) where known=true means the first format is from detection/memory.
func (ph *ProxyHandler) allFormatsForProvider(pid, burl string, provider *providerpool.Provider) ([4]providerpool.APIFormat, int, bool) {
	var buf [4]providerpool.APIFormat
	n := 0
	known := false

	has := func(f providerpool.APIFormat) bool {
		for i := 0; i < n; i++ {
			if buf[i] == f {
				return true
			}
		}
		return false
	}

	// 0. Endpoint-locked format (highest priority).
	// If the provider base URL already points to a protocol-specific endpoint,
	// lock to that family and avoid cross-format fallback probing.
	if endpointFormat, ok := detectEndpointFixedFormat(burl); ok {
		buf[n] = endpointFormat
		n++
		return buf, n, true
	}

	// 1. Persisted detected format (highest priority — survives restarts)
	if provider.DetectedFormat != "" {
		buf[n] = provider.DetectedFormat
		n++
		known = true
	}

	// 2. In-memory remembered format (from recent successful requests)
	if remembered, ok := ph.providerMemory.RecallFormat(pid, burl); ok {
		f := providerpool.APIFormat(remembered)
		if !has(f) {
			buf[n] = f
			n++
		}
		known = true
	}

	// 3. Provider default
	if provider.APIFormat != "" && !has(provider.APIFormat) {
		buf[n] = provider.APIFormat
		n++
	}

	// Non-third-party providers should use a single canonical family.
	// Custom/third-party relays may still need cross-family fallback.
	if isSingleFormatProvider(provider) {
		return buf, n, known
	}

	// 4. Infer format from base URL — if the URL hints at Anthropic, prefer it
	if strings.HasSuffix(burl, "/messages") || strings.Contains(burl, "anthropic") {
		if !has(providerpool.APIFormatAnthropic) {
			buf[n] = providerpool.APIFormatAnthropic
			n++
		}
	}

	// 5. Remaining formats — skip cross-family fallbacks for providers that
	// definitively only support one API family. Copilot, CloudCode, and Ollama
	// should stay single-format; probing OpenAI/Anthropic alternates just adds
	// guaranteed 404s and noisy mismatch logs.
	switch provider.APIFormat {
	case providerpool.APIFormatCopilot, providerpool.APIFormatCloudCode, providerpool.APIFormatOllama, providerpool.APIFormatResponses:
		// Single-family providers: no cross-format fallback.
	default:
		// All other providers (including generic openai, anthropic, cloudcode, unknown)
		// — try both families as fallback
		for _, f := range [...]providerpool.APIFormat{providerpool.APIFormatAnthropic, providerpool.APIFormatOpenAI} {
			if !has(f) {
				buf[n] = f
				n++
			}
		}
	}

	return buf, n, known
}

// detectEndpointFixedFormat infers API format from endpoint-specific base URLs.
// Returns (format, true) only when the path itself implies a fixed protocol family.
func detectEndpointFixedFormat(raw string) (providerpool.APIFormat, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	return detectEndpointFixedFormatFromPath(u.Path)
}

func detectEndpointFixedFormatFromPath(path string) (providerpool.APIFormat, bool) {
	path = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(path)), "/")
	switch {
	case strings.HasSuffix(path, "/responses"):
		return providerpool.APIFormatResponses, true
	case strings.HasSuffix(path, "/messages"), strings.HasSuffix(path, "/anthropic"):
		return providerpool.APIFormatAnthropic, true
	default:
		return "", false
	}
}

func isSingleFormatProvider(provider *providerpool.Provider) bool {
	if provider == nil {
		return false
	}
	switch provider.APIFormat {
	case providerpool.APIFormatCopilot, providerpool.APIFormatCloudCode, providerpool.APIFormatOllama, providerpool.APIFormatResponses:
		return true
	}
	// Explicitly custom providers are third-party and can need format probing/fallback.
	if provider.Type == providerpool.ProviderTypeCustom || strings.HasPrefix(provider.ID, "custom-") {
		return false
	}
	// For known non-custom provider types, keep a single format.
	switch provider.Type {
	case providerpool.ProviderTypeBuiltin, providerpool.ProviderTypePlatform, providerpool.ProviderTypeIDE, providerpool.ProviderTypeTrial, providerpool.ProviderTypeACP, providerpool.ProviderTypeMedia:
		return true
	}
	// Unknown/empty type: keep legacy cross-family fallback behavior.
	return false
}

// persistDetectedFormat saves the detected API format to the provider for persistence across restarts.
// Only updates if the format changed, to avoid unnecessary writes.
func (ph *ProxyHandler) persistDetectedFormat(provider *providerpool.Provider, format providerpool.APIFormat) {
	if provider.DetectedFormat == format {
		return // already persisted
	}
	provider.DetectedFormat = format
	provider.DetectedAt = timeutil.NowTime()
	if ph.providerPool != nil && ph.providerPool.Registry != nil {
		// Async persist — don't block the request path on DB write
		go func(p *providerpool.Provider) {
			if err := ph.providerPool.Registry.Update(p); err != nil {
				slog.Warn("[proxy] failed to persist detected format", "provider", p.ID, "format", format, "error", err)
			}
		}(provider)
	}
}

// persistDetectedEndpoint saves the detected endpoint to the provider for persistence across restarts.
// This is called when an alternate base URL works (e.g., international endpoint instead of China).
// Only updates if the endpoint changed, to avoid unnecessary writes.
func (ph *ProxyHandler) persistDetectedEndpoint(provider *providerpool.Provider, endpoint string) {
	if provider.DetectedEndpoint == endpoint {
		return // already persisted
	}
	provider.DetectedEndpoint = endpoint
	provider.ResetParsedURL() // Invalidate cached parsed URL
	if ph.providerPool != nil && ph.providerPool.Registry != nil {
		// Async persist — don't block the request path on DB write
		go func(p *providerpool.Provider) {
			if err := ph.providerPool.Registry.Update(p); err != nil {
				slog.Warn("[proxy] failed to persist detected endpoint", "provider", p.ID, "endpoint", endpoint, "error", err)
			}
		}(provider)
	}
}

// clearDetectedFormat clears the persisted format when a format mismatch is detected.
// This prevents the wrong format from being loaded again after restart.
func (ph *ProxyHandler) clearDetectedFormat(provider *providerpool.Provider) {
	if provider.DetectedFormat == "" {
		return // nothing to clear
	}
	provider.DetectedFormat = ""
	if ph.providerPool != nil && ph.providerPool.Registry != nil {
		go func(p *providerpool.Provider) {
			if err := ph.providerPool.Registry.Update(p); err != nil {
				slog.Warn("[proxy] failed to clear detected format", "provider", p.ID, "error", err)
			}
		}(provider)
	}
}

// tryEndpointFallback attempts to find a working endpoint by trying alternate base URLs
// when the primary endpoint fails with auth error (401/403).
// Returns (response, format used, model used, error).
// If an alternate endpoint works, it persists the new endpoint.
func (ph *ProxyHandler) tryEndpointFallback(
	r *http.Request,
	result *providerpool.RouteResult,
	pr *parsedRequest,
	formatsBuf [4]providerpool.APIFormat,
	nFormats int,
	modelsBuf [8]string,
	nModels int,
	originalErr error,
) (*http.Response, providerpool.APIFormat, string, error) {
	provider := result.Provider
	pid := provider.ID

	// Check if provider has alternate URLs to try
	if len(provider.AlternateBaseURLs) == 0 {
		slog.Info("[proxy] endpoint fallback skipped: no alternate URLs", "provider", provider.ID)
		return nil, "", "", originalErr
	}

	// Don't retry if we've already tried an alternate (avoid infinite loop)
	// This is indicated by DetectedEndpoint being set to a non-primary URL
	isAlternate := false
	for _, alt := range provider.AlternateBaseURLs {
		if provider.EffectiveBaseURL() == alt {
			isAlternate = true
			break
		}
	}
	if isAlternate {
		slog.Info("[proxy] endpoint fallback skipped: already on alternate", "provider", provider.ID, "url", provider.EffectiveBaseURL())
		return nil, "", "", originalErr
	}

	slog.Info("[proxy] endpoint auth failed, trying alternate endpoints",
		"provider", pid, "primary", provider.EffectiveBaseURL(), "alternates", provider.AlternateBaseURLs, "error", originalErr)

	// Try each alternate endpoint
	for _, altURL := range provider.AlternateBaseURLs {
		slog.Info("[proxy] === trying alternate endpoint ===", "provider", pid, "url", altURL, "nModels", nModels, "nFormats", nFormats)

		// Temporarily set the endpoint
		oldDetected := provider.DetectedEndpoint
		provider.DetectedEndpoint = altURL
		provider.ResetParsedURL()
		slog.Info("[proxy] set DetectedEndpoint", "provider", pid, "new", altURL, "effective", provider.EffectiveBaseURL())

		// Try the request with this endpoint
		for mi := 0; mi < nModels; mi++ {
			model := modelsBuf[mi]
			for fi := 0; fi < nFormats; fi++ {
				format := formatsBuf[fi]
				slog.Info("[proxy] trying model/format combo", "provider", pid, "model", model, "format", format)

				// Build body with this model
				forwardBody := pr.body
				if model != pr.model {
					if newBody, err := sjson.SetBytes(pr.body, "model", model); err == nil {
						forwardBody = newBody
					}
				}

				apiKeyToUse := result.APIKey
				resp, probeErr := ph.probeWithRetry(
					r.Context(),
					provider,
					apiKeyToUse,
					format,
					func() (*http.Request, error) {
						return ph.buildUpstreamRequestWithFormat(r, result, forwardBody, format)
					},
					func(req *http.Request) (*http.Response, error) {
						if provider.SkipTLSVerify {
							return ph.connPool.GetInsecureClient(provider.Name).Do(req)
						}
						return ph.connPool.GetClient(provider.Name).Do(req)
					},
				)

				if probeErr != nil {
					// Auth failed again, try next format/model
					slog.Info("[proxy] alternate probe failed", "provider", pid, "model", model, "format", format, "error", probeErr)
					continue
				}

				if resp.StatusCode < 400 {
					// Success! Persist this endpoint
					slog.Info("[proxy] alternate endpoint worked, persisting",
						"provider", pid, "endpoint", altURL, "format", format, "model", model)
					ph.persistDetectedEndpoint(provider, altURL)
					ph.providerMemory.RememberFormat(pid, altURL, string(format))
					ph.persistDetectedFormat(provider, format)
					if model != pr.model {
						ph.providerMemory.RememberModelAlias(pid, altURL, pr.model, model)
					}
					return resp, format, model, nil
				}

				// Non-auth error (e.g., 400, 404), try next
				resp.Body.Close()
			}
		}

		// This alternate failed, restore and try next
		provider.DetectedEndpoint = oldDetected
		provider.ResetParsedURL()
	}

	// All alternates failed, return original error
	return nil, "", "", originalErr
}

// Remembered alias first, then original model, then ModelAliases.
// Uses stack-allocated array for the common case (≤8 candidates).
func (ph *ProxyHandler) allModelsForProvider(pid, burl, originalModel string, routedModel string, ignoreBlacklist bool) ([8]string, int) {
	var buf [8]string
	n := 0

	has := func(s string) bool {
		for i := 0; i < n; i++ {
			if buf[i] == s {
				return true
			}
		}
		return false
	}

	add := func(s string) {
		if n < len(buf) {
			buf[n] = s
			n++
		}
	}

	// 1. Remembered alias (highest priority)
	if alias, ok := ph.providerMemory.RecallModelAlias(pid, burl, originalModel); ok {
		if ignoreBlacklist || !ph.providerMemory.IsModelBlacklisted(pid, burl, alias) {
			add(alias)
		}
	}

	// 2. Routed model (from provider pool)
	if routedModel != "" && !has(routedModel) {
		if ignoreBlacklist || !ph.providerMemory.IsModelBlacklisted(pid, burl, routedModel) {
			add(routedModel)
		}
	}

	// 3. Original model (skip empty — happens when "auto" is normalized to "")
	if originalModel != "" && !has(originalModel) {
		if ignoreBlacklist || !ph.providerMemory.IsModelBlacklisted(pid, burl, originalModel) {
			add(originalModel)
		}
	}

	// 4. ModelAliases
	if aliases, ok := ModelAliases[originalModel]; ok {
		for _, alias := range aliases {
			if !has(alias) && (ignoreBlacklist || !ph.providerMemory.IsModelBlacklisted(pid, burl, alias)) {
				add(alias)
			}
		}
	}

	return buf, n
}

// tryOnProvider tries all Model × Format combinations on a single provider.
// Model is the outer loop: if a model isn't configured, skip it entirely.
// Format is the inner loop: try detected/remembered format first, then others.
// If the provider already has a detected format, only that format is tried.
// Returns (response, format used, model used, error).
func (ph *ProxyHandler) tryOnProvider(
	r *http.Request,
	result *providerpool.RouteResult,
	pr *parsedRequest,
) (*http.Response, providerpool.APIFormat, string, error) {
	pid := result.Provider.ID
	burl := result.Provider.EffectiveBaseURL()

	routedModel := ""
	if result.Model != nil && result.Model.ID != pr.model {
		routedModel = result.Model.ID
	}

	// Copilot special handling: get JWT token if provider uses OAuth
	// The token needs to be fetched fresh for each request as it may expire
	apiKeyToUse := result.APIKey

	// OAuth token handling for Copilot and CloudCode providers
	if ph.oauthManager != nil {
		if result.Provider.APIFormat == providerpool.APIFormatCopilot {
			// Copilot requires exchanging GitHub OAuth token for a Copilot-specific JWT
			copilotToken, tokenErr := ph.oauthManager.GetCopilotAccessToken(result.Provider.ID)
			if tokenErr == nil && copilotToken != "" {
				apiKeyToUse = &providerpool.APIKey{Key: copilotToken}
				slog.Debug("[proxy] using Copilot JWT token", "provider", pid)
			} else if tokenErr != nil {
				slog.Warn("[proxy] failed to get Copilot token, falling back to stored key", "provider", pid, "error", tokenErr)
			}
		} else if result.Provider.APIFormat == providerpool.APIFormatCloudCode {
			// CloudCode (Antigravity/Gemini CLI) requires OAuth access token
			accessToken, tokenErr := ph.oauthManager.GetAccessToken(result.Provider.ID)
			if tokenErr == nil && accessToken != "" {
				apiKeyToUse = &providerpool.APIKey{Key: accessToken}
				slog.Debug("[proxy] using CloudCode OAuth token", "provider", pid)
			} else if tokenErr != nil {
				slog.Warn("[proxy] failed to get CloudCode token, falling back to stored key", "provider", pid, "error", tokenErr)
			}
		}
	}

	formatsBuf, nFormats, formatKnown := ph.allFormatsForProvider(pid, burl, result.Provider)
	allFormats := nFormats // remember full count before truncation
	if endpointFormat, ok := detectEndpointFixedFormatFromPath(r.URL.Path); ok {
		formatsBuf[0] = endpointFormat
		nFormats = 1
		allFormats = 1
		formatKnown = true
	}
	modelsBuf, nModels := ph.allModelsForProvider(pid, burl, pr.model, routedModel, pr.singleProvider)

	slog.Debug("[proxy] tryOnProvider setup",
		"provider", pid, "nModels", nModels, "nFormats", nFormats,
		"allFormats", allFormats, "formatKnown", formatKnown,
		"format0", formatsBuf[0], "model0", modelsBuf[0],
		"requestModel", pr.model)

	if nModels == 0 {
		return nil, "", "", fmt.Errorf("all models blacklisted on provider %s", pid)
	}

	// If format is known (detected or remembered), only try that one — skip fallback formats.
	// On format mismatch (422 / wrapped 404), nFormats is expanded back to allFormats.
	if formatKnown {
		nFormats = 1
	}

	var lastErr error

	// Fast path: single model + single format + model matches request (most common happy path).
	// Avoids loop overhead, sjson.SetBytes, and slice iteration.
	var fastPathTriedFormat providerpool.APIFormat // track what fast path already tried
	if nModels == 1 && nFormats == 1 && modelsBuf[0] == pr.model {
		format := formatsBuf[0]
		fastPathTriedFormat = format
		for upstreamAttempt := 0; ; upstreamAttempt++ {
			slog.Debug("[proxy] trying", "provider", pid, "format", format, "model", pr.model)
			resp, probeErr := ph.probeWithRetry(
				r.Context(),
				result.Provider,
				apiKeyToUse,
				format,
				func() (*http.Request, error) {
					return ph.buildUpstreamRequestWithFormat(r, result, pr.body, format)
				},
				func(req *http.Request) (*http.Response, error) {
					if result.Provider.SkipTLSVerify {
						return ph.connPool.GetInsecureClient(result.Provider.Name).Do(req)
					}
					return ph.connPool.GetClient(result.Provider.Name).Do(req)
				},
			)
			if probeErr != nil {
				if errors.Is(probeErr, context.Canceled) || errors.Is(probeErr, context.DeadlineExceeded) {
					return nil, "", "", probeErr
				}
				// Try endpoint fallback when auth is exhausted
				hasKey := result.APIKey != nil && result.APIKey.Key != ""
				slog.Info("[proxy] fast path probe error, trying endpoint fallback",
					"provider", pid, "model", pr.model, "format", format, "error", probeErr,
					"baseURL", result.Provider.BaseURL, "effectiveURL", result.Provider.EffectiveBaseURL(),
					"hasKey", hasKey, "keyLen", len(result.APIKey.Key), "alternates", result.Provider.AlternateBaseURLs)
				if len(result.Provider.AlternateBaseURLs) > 0 {
					resp, fmt, mdl, err := ph.tryEndpointFallback(
						r, result, pr, formatsBuf, nFormats, modelsBuf, nModels, probeErr,
					)
					if err == nil {
						slog.Info("[proxy] fast path endpoint fallback succeeded", "provider", pid)
						return resp, fmt, mdl, nil
					}
					slog.Info("[proxy] fast path endpoint fallback failed", "provider", pid, "error", err)
				}
				return nil, "", "", probeErr
			}
			if resp.StatusCode < 400 {
				ph.providerMemory.RememberFormat(pid, burl, string(format))
				ph.persistDetectedFormat(result.Provider, format)
				return resp, format, pr.model, nil
			}
			// Read error body and close response before deciding what to do
			errBody := readErrorBody(resp.Body)
			resp.Body.Close()
			statusCode := resp.StatusCode
			if shouldRetryTransientUpstream5xx(pr.singleProvider, statusCode, errBody, upstreamAttempt) {
				delay := transientUpstreamRetryDelay(upstreamAttempt)
				slog.Warn("[proxy] transient upstream 5xx, retrying in single-provider mode",
					"provider", pid, "status", statusCode, "attempt", upstreamAttempt+1, "delay", delay)
				select {
				case <-r.Context().Done():
					return nil, "", "", r.Context().Err()
				case <-time.After(delay):
				}
				continue
			}

			known404ModelNotConfigured := treatKnown404AsModelNotConfigured(result.Provider, formatKnown, statusCode)

			// Format mismatch on fast path — expand to all formats and fall through to general loop.
			// Exception: some curated providers return generic 404 for unknown models
			// (e.g. Copilot). In that case, treat it as model-not-configured.
			if allFormats > 1 && isFormatMismatchError(statusCode, errBody) && !known404ModelNotConfigured {
				errStr := string(errBody)
				if len(errStr) > 256 {
					errStr = errStr[:256]
				}
				slog.Warn("[proxy] format mismatch on fast path, expanding to all formats",
					"provider", pid, "format", format, "model", pr.model)
				ph.providerMemory.ForgetFormat(pid, burl)
				ph.clearDetectedFormat(result.Provider)
				nFormats = allFormats
				lastErr = fmt.Errorf("provider returned %d: %s", statusCode, errStr)
				// Fall through to general loop which will try remaining formats
			} else if allFormats <= 1 {
				// Only one format available and it failed — handle error inline
				// since the general loop would skip this already-tried combo.
				errStr := string(errBody)
				if len(errStr) > 256 {
					errStr = errStr[:256]
				}
				if statusCode == http.StatusTooManyRequests {
					retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
					if !pr.singleProvider {
						ph.providerMemory.RememberThrottle(pid, burl, retryAfter)
					}
					return nil, "", "", fmt.Errorf("provider %s throttled (429)", pid)
				}
				if statusCode == 529 {
					if !pr.singleProvider {
						ph.providerMemory.RememberThrottle(pid, burl, 30*time.Second)
					}
					return nil, "", "", fmt.Errorf("provider %s overloaded (529)", pid)
				}
				if statusCode >= 500 {
					// 5xx: server error, skip entire provider
					errMsg := errStr
					if len(errMsg) > 500 {
						errMsg = errMsg[:500]
					}
					slog.Warn("[proxy] upstream 5xx error",
						"provider", pid, "status", statusCode, "error", errMsg)
					return nil, "", "", fmt.Errorf("upstream %d: %s", statusCode, errStr)
				}
				if isFormatMismatchError(statusCode, errBody) && !known404ModelNotConfigured {
					return nil, "", "", fmt.Errorf("provider returned %d: %s", statusCode, errStr)
				}
				if isModelNotConfiguredError(statusCode, errBody) || known404ModelNotConfigured {
					if !pr.singleProvider {
						ph.providerMemory.BlacklistModel(pid, burl, pr.model)
					}
					return nil, "", "", fmt.Errorf("model %s not configured on provider %s: %s", pr.model, pid, errStr)
				}
				// Auth errors — don't blacklist model
				if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden ||
					(statusCode == http.StatusBadRequest && isAuthRelatedBody(errBody)) {
					return nil, "", "", fmt.Errorf("provider %s auth error (%d): %s", pid, statusCode, errStr)
				}
				if !pr.singleProvider {
					ph.providerMemory.BlacklistModel(pid, burl, pr.model)
				}
				return nil, "", "", fmt.Errorf("provider returned %d: %s", statusCode, errStr)
			}
			// else: format expanded, fall through to general loop
			break
		}
	}

	for mi := 0; mi < nModels; mi++ {
		model := modelsBuf[mi]
		// Build body with this model
		forwardBody := pr.body
		if model != pr.model {
			if newBody, err := sjson.SetBytes(pr.body, "model", model); err == nil {
				forwardBody = newBody
			}
		}

		allFormatsMismatch := true // tracks if every format failed with format mismatch
		formatsTried := 0          // count of formats actually attempted (not skipped)
		for fi := 0; fi < nFormats; fi++ {
			format := formatsBuf[fi]
			// Skip format already tried (and failed) on fast path for the same model
			if fastPathTriedFormat != "" && format == fastPathTriedFormat && model == pr.model {
				continue
			}
			formatsTried++
			slog.Debug("[proxy] trying", "provider", pid, "format", format, "model", model)

			var resp *http.Response
			var probeErr error
			var preReadStatusCode int
			var preReadErrBody []byte
			var preReadRetryAfter string
			for upstreamAttempt := 0; ; upstreamAttempt++ {
				resp, probeErr = ph.probeWithRetry(
					r.Context(),
					result.Provider,
					apiKeyToUse,
					format,
					func() (*http.Request, error) {
						return ph.buildUpstreamRequestWithFormat(r, result, forwardBody, format)
					},
					func(req *http.Request) (*http.Response, error) {
						if result.Provider.SkipTLSVerify {
							return ph.connPool.GetInsecureClient(result.Provider.Name).Do(req)
						}
						return ph.connPool.GetClient(result.Provider.Name).Do(req)
					},
				)
				if probeErr != nil {
					break
				}
				if resp.StatusCode < 400 {
					break
				}
				errBody := readErrorBody(resp.Body)
				preReadRetryAfter = resp.Header.Get("Retry-After")
				resp.Body.Close()
				if shouldRetryTransientUpstream5xx(pr.singleProvider, resp.StatusCode, errBody, upstreamAttempt) {
					delay := transientUpstreamRetryDelay(upstreamAttempt)
					slog.Warn("[proxy] transient upstream 5xx, retrying in single-provider mode",
						"provider", pid, "status", resp.StatusCode, "attempt", upstreamAttempt+1, "delay", delay)
					select {
					case <-r.Context().Done():
						return nil, "", "", r.Context().Err()
					case <-time.After(delay):
					}
					continue
				}
				preReadStatusCode = resp.StatusCode
				preReadErrBody = errBody
				break
			}
			if probeErr != nil {
				lastErr = probeErr
				allFormatsMismatch = false
				if errors.Is(probeErr, context.Canceled) || errors.Is(probeErr, context.DeadlineExceeded) {
					return nil, "", "", probeErr
				}

				// Try endpoint fallback when auth is exhausted (all auth strategies failed)
				// This handles the case where the API key works with a different endpoint
				hasKey := result.APIKey != nil && result.APIKey.Key != ""
				slog.Info("[proxy] probe error, trying endpoint fallback",
					"provider", pid, "model", model, "format", format, "error", probeErr,
					"baseURL", result.Provider.BaseURL, "effectiveURL", result.Provider.EffectiveBaseURL(),
					"hasKey", hasKey, "keyLen", len(result.APIKey.Key), "alternates", result.Provider.AlternateBaseURLs)
				if len(result.Provider.AlternateBaseURLs) > 0 {
					resp, fmt, mdl, err := ph.tryEndpointFallback(
						r, result, pr, formatsBuf, nFormats, modelsBuf, nModels, probeErr,
					)
					if err == nil {
						slog.Info("[proxy] endpoint fallback succeeded", "provider", pid)
						return resp, fmt, mdl, nil
					}
					slog.Info("[proxy] endpoint fallback failed", "provider", pid, "error", err)
				}

				continue
			}

			if resp.StatusCode < 400 {
				// Success — remember what worked
				ph.providerMemory.RememberFormat(pid, burl, string(format))
				if model != pr.model {
					ph.providerMemory.RememberModelAlias(pid, burl, pr.model, model)
				}
				// Persist detected format to provider (survives restarts)
				ph.persistDetectedFormat(result.Provider, format)
				return resp, format, model, nil
			}

			// Handle error — use lightweight reader for small error bodies
			statusCode := preReadStatusCode
			errBody := preReadErrBody
			if statusCode == 0 {
				statusCode = resp.StatusCode
				errBody = readErrorBody(resp.Body)
				preReadRetryAfter = resp.Header.Get("Retry-After")
				resp.Body.Close()
			}
			errStr := string(errBody)
			if len(errStr) > 256 {
				errStr = errStr[:256]
			}

			if statusCode == http.StatusTooManyRequests {
				retryAfter := parseRetryAfter(preReadRetryAfter)
				if !pr.singleProvider {
					ph.providerMemory.RememberThrottle(pid, burl, retryAfter)
				}
				return nil, "", "", fmt.Errorf("provider %s throttled (429)", pid)
			}

			// 529: Anthropic "overloaded" — treat like 429, fast-fail without retry.
			// The model/provider is temporarily at capacity; retrying wastes time.
			if statusCode == 529 {
				if !pr.singleProvider {
					ph.providerMemory.RememberThrottle(pid, burl, 30*time.Second)
				}
				slog.Warn("[proxy] upstream overloaded (529), fast-fail", "provider", pid, "model", model)
				return nil, "", "", fmt.Errorf("provider %s overloaded (529)", pid)
			}

			if statusCode >= 500 {
				// 5xx: server error, skip entire provider
				errMsg := errStr
				if len(errMsg) > 500 {
					errMsg = errMsg[:500]
				}
				slog.Warn("[proxy] upstream 5xx error",
					"provider", pid, "status", statusCode, "error", errMsg)
				return nil, "", "", fmt.Errorf("upstream %d: %s", statusCode, errStr)
			}

			known404ModelNotConfigured := treatKnown404AsModelNotConfigured(result.Provider, formatKnown, statusCode)

			// Format mismatch: 422, or 404/400 with format-related error body.
			// Check BEFORE isModelNotConfiguredError since both match 404/422.
			// Don't blacklist the model — try the next format instead.
			//
			// Exception: when the format is already known (detected or remembered),
			// a 404 can be model-not-found for curated providers (e.g. Copilot).
			// Let those fall through to isModelNotConfiguredError.
			if isFormatMismatchError(statusCode, errBody) && !known404ModelNotConfigured {
				slog.Warn("[proxy] format mismatch, trying next format",
					"provider", pid, "format", format, "model", model, "status", statusCode, "body", errStr)
				lastErr = fmt.Errorf("provider returned %d: %s", statusCode, errStr)
				// If we're on the last format and there are more available, expand
				if fi == nFormats-1 && allFormats > nFormats {
					ph.providerMemory.ForgetFormat(pid, burl)
					ph.clearDetectedFormat(result.Provider)
					nFormats = allFormats
				}
				continue
			}

			// Check if this is a "not configured" / "model not found" error.
			// When the format is already known, a 404 means the model doesn't
			// exist on this provider (e.g. Copilot returns generic "page not found"
			// for unknown models) — treat it as model-not-configured even if the
			// body doesn't contain model-specific keywords.
			if isModelNotConfiguredError(statusCode, errBody) || known404ModelNotConfigured {
				slog.Warn("[proxy] model not configured on provider, blacklisting and trying next",
					"provider", pid, "format", format, "model", model, "status", statusCode, "body", errStr)
				// Blacklist THIS model on THIS provider (per-provider scope) so we don't retry it
				if !pr.singleProvider {
					ph.providerMemory.BlacklistModel(pid, burl, model)
				}
				lastErr = fmt.Errorf("model %s not configured on provider %s: %s", model, pid, errStr)
				allFormatsMismatch = false
				// Skip remaining formats for this model — if model isn't configured,
				// trying a different format won't help
				break
			}

			// 400 invalid_request_error: the request body itself is malformed
			// (e.g. orphaned tool_result, bad schema). Retrying with different
			// auth/format/model won't help — return immediately without blacklisting.
			if statusCode == http.StatusBadRequest && strings.Contains(errStr, "invalid_request_error") {
				slog.Warn("[proxy] 400 invalid_request_error, not retryable",
					"provider", pid, "format", format, "model", model, "body", errStr)
				return nil, "", "", fmt.Errorf("upstream 400: %s", errStr)
			}

			// Auth errors (401, 403, or 400 with auth-related body): the provider
			// rejected our credentials. Don't blacklist the model — it's not the
			// model's fault. Skip entire provider so failover can try another.
			if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden ||
				(statusCode == http.StatusBadRequest && isAuthRelatedBody(errBody)) {
				slog.Warn("[proxy] auth error, skipping provider",
					"provider", pid, "format", format, "model", model, "status", statusCode, "body", errStr)

				// Try endpoint fallback when auth fails
				// This will try alternate base URLs if available
				if len(result.Provider.AlternateBaseURLs) > 0 {
					resp, fmt, mdl, err := ph.tryEndpointFallback(
						r, result, pr, formatsBuf, nFormats, modelsBuf, nModels,
						fmt.Errorf("provider %s auth error (%d): %s", pid, statusCode, errStr),
					)
					if err == nil {
						return resp, fmt, mdl, nil
					}
					// Endpoint fallback failed, continue with original error
				}

				return nil, "", "", fmt.Errorf("provider %s auth error (%d): %s", pid, statusCode, errStr)
			}

			// Other 4xx: blacklist this model on this provider, try next model/format
			slog.Warn("[proxy] 4xx, trying next combination",
				"provider", pid, "format", format, "model", model, "status", statusCode, "body", errStr)
			if !pr.singleProvider {
				ph.providerMemory.BlacklistModel(pid, burl, model)
			}
			lastErr = fmt.Errorf("provider returned %d: %s", statusCode, errStr)
			allFormatsMismatch = false
		}
		// If every format returned format mismatch for this model, the provider
		// doesn't understand any of our formats — skip remaining model aliases.
		// But only if we actually tried at least one format (not all skipped by fast path).
		if allFormatsMismatch && formatsTried > 0 {
			triedFormats := make([]string, 0, nFormats)
			for fi := 0; fi < nFormats; fi++ {
				triedFormats = append(triedFormats, string(formatsBuf[fi]))
			}
			slog.Warn("[proxy] all formats returned mismatch, skipping remaining models",
				"provider", pid, "model", model, "tried_formats", triedFormats, "last_error", lastErr)
			break
		}
	}
	return nil, "", "", lastErr
}

// notConfiguredPatterns are pre-allocated pattern slices for isModelNotConfiguredError.
// Stored as []byte to avoid string→[]byte conversion on each check.
var notConfiguredPatternsEN = [][]byte{
	[]byte("not configured"),
	[]byte("not enabled"),
	[]byte("not available"),
	[]byte("not supported"),
	[]byte("no access"),
	[]byte("model disabled"),
	[]byte("model unavailable"),
	[]byte("model not found"),
	[]byte("does not exist"),
	[]byte("invalid model"),
	[]byte("unknown model"),
	[]byte("not authorized"),
	[]byte("permission denied"),
}

var notConfiguredPatternsCN = [][]byte{
	[]byte("未配置"),   // not configured
	[]byte("未启用"),   // not enabled
	[]byte("不可用"),   // not available
	[]byte("不支持"),   // not supported
	[]byte("模型不存在"), // model does not exist
	[]byte("未找到"),   // not found
	[]byte("无权限"),   // no permission
}

// isModelNotConfiguredError checks if the error response indicates the model
// is listed but not actually configured/available on this provider.
// Uses bytes.Contains with pre-lowered patterns to avoid strings.ToLower allocation.
func isModelNotConfiguredError(statusCode int, body []byte) bool {
	if statusCode != 400 && statusCode != 403 && statusCode != 404 && statusCode != 422 {
		return false
	}
	return isModelNotConfiguredBody(body)
}

// isModelNotConfiguredBody checks if the error body contains model-not-configured patterns.
// Separated from isModelNotConfiguredError so isFormatMismatchError can also use it.
func isModelNotConfiguredBody(body []byte) bool {
	lower := toLowerBytes(body)

	for _, pattern := range notConfiguredPatternsEN {
		if bytes.Contains(lower, pattern) {
			return true
		}
	}

	// Chinese patterns — match against original body (no case folding needed for CJK)
	for _, pattern := range notConfiguredPatternsCN {
		if bytes.Contains(body, pattern) {
			return true
		}
	}

	return false
}

// treatKnown404AsModelNotConfigured returns true for providers where a
// format-known 404 should be interpreted as model-not-configured instead of
// format mismatch. This is primarily needed for curated providers (for example,
// Copilot) that return generic 404 bodies for unknown model IDs.
func treatKnown404AsModelNotConfigured(provider *providerpool.Provider, formatKnown bool, statusCode int) bool {
	if !formatKnown || statusCode != http.StatusNotFound || provider == nil {
		return false
	}

	switch provider.APIFormat {
	case providerpool.APIFormatCopilot, providerpool.APIFormatAnthropic:
		return true
	default:
		return false
	}
}

// formatMismatchPatterns are error body patterns that indicate the request format
// doesn't match what the provider expects (e.g. OpenAI body sent to Anthropic endpoint).
var formatMismatchPatterns = [][]byte{
	[]byte("unsupported request"),
	[]byte("unsupported legacy protocol"),
	[]byte("bad_response_status_code"),
	[]byte("openai_error"),
	[]byte("invalid request format"),
	[]byte("please use /v1/responses"),
	[]byte("unexpected content type"),
}

// formatMismatch422Patterns are body patterns that confirm a 422 is a genuine format
// mismatch rather than a parameter validation error. Without these patterns, a 422
// could be an unsupported parameter, missing field, or other validation failure.
var formatMismatch422Patterns = [][]byte{
	[]byte("unsupported request"),
	[]byte("unsupported legacy protocol"),
	[]byte("bad_response_status_code"),
	[]byte("openai_error"),
	[]byte("invalid request format"),
	[]byte("please use /v1/responses"),
	[]byte("unexpected content type"),
	[]byte("malformed"),
	[]byte("unknown field"),         // wrong schema fields
	[]byte("unexpected field"),      // wrong schema fields
	[]byte("additional properties"), // JSON schema validation — wrong format
}

// isFormatMismatchError returns true if the error indicates a request format mismatch
// rather than a model configuration issue. These errors should trigger format fallback,
// not model blacklisting.
// NOTE: Must be called BEFORE isModelNotConfiguredError since both match 422/404.
// CAVEAT: Callers should also check formatKnown — when the format is already detected,
// a 404 "page not found" is more likely a model-not-found error (e.g. GitHub Copilot
// returns generic 404 for unknown models) than a format mismatch.
func isFormatMismatchError(statusCode int, body []byte) bool {
	if statusCode != 400 && statusCode != 404 && statusCode != 422 && statusCode != 502 {
		return false
	}
	lower := toLowerBytes(body)

	// Explicit protocol/format mismatch signals should win even when the body
	// also includes broad "not supported"/"not configured" wording.
	containsFormatPattern := false
	for _, pattern := range formatMismatchPatterns {
		if bytes.Contains(lower, pattern) {
			containsFormatPattern = true
			break
		}
	}

	// Some relays normalize upstream 400/404 protocol mismatches into 502.
	// Only treat 502 as format mismatch when the body has explicit evidence.
	if statusCode == 502 {
		return containsFormatPattern
	}

	// For 422: require body evidence of format mismatch. A bare 422 without
	// recognizable patterns is more likely a parameter validation error (e.g.
	// unsupported tool format, unknown model) than a format mismatch.
	if statusCode == 422 {
		if len(body) == 0 {
			// Empty body 422 — ambiguous, treat as format mismatch for safety
			return true
		}
		if isModelNotConfiguredBody(body) && !containsFormatPattern {
			return false
		}
		for _, pattern := range formatMismatch422Patterns {
			if bytes.Contains(lower, pattern) {
				return true
			}
		}
		return false
	}

	// If the body mentions model-not-configured and there is no explicit
	// format mismatch marker, it's not a format mismatch.
	if isModelNotConfiguredBody(body) && !containsFormatPattern {
		return false
	}
	// For 404: a short "page not found" / "not found" body (without model-specific
	// keywords) almost certainly means the endpoint doesn't exist — i.e. wrong API
	// format (e.g. /v1/messages on an OpenAI-only provider like GitHub Copilot).
	if statusCode == 404 {
		if bytes.Contains(lower, []byte("page not found")) || bytes.Contains(lower, []byte("not found")) {
			return true
		}
	}
	// For 400/404, check body for format-related patterns
	return containsFormatPattern
}

// authRelatedPatterns are body patterns indicating an authentication/authorization
// failure (as opposed to a model or format issue).
var authRelatedPatterns = [][]byte{
	[]byte("authorization"),
	[]byte("authenticate"),
	[]byte("unauthorized"),
	[]byte("forbidden"),
	[]byte("invalid.*token"),
	[]byte("invalid api key"),
	[]byte("invalid key"),
	[]byte("api key"),
	[]byte("access denied"),
	[]byte("credentials"),
}

// isAuthRelatedBody checks if an error body indicates an auth problem.
func isAuthRelatedBody(body []byte) bool {
	lower := toLowerBytes(body)
	for _, pattern := range authRelatedPatterns {
		if bytes.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// toLowerBytes lowercases ASCII bytes in-place on a stack-allocated copy.
// For bodies ≤4KB, uses stack buffer to avoid heap allocation.
func toLowerBytes(b []byte) []byte {
	n := len(b)
	// Cap at 4KB — error bodies are typically small
	if n > 4096 {
		n = 4096
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		c := b[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return out
}

// parseRetryAfter parses the Retry-After header value into a duration.
func parseRetryAfter(val string) time.Duration {
	if val == "" {
		return 30 * time.Second
	}
	if secs, err := strconv.Atoi(val); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := time.Parse(time.RFC1123, val); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 30 * time.Second
}

// shouldLogUpstreamRequestBody enables full upstream request body logging for
// temporary debugging. Set env ZIMA_PROXY_LOG_UPSTREAM_BODY=1 (or true/yes/on)
// to log every upstream attempt, including retries/fallbacks.
func (ph *ProxyHandler) shouldLogUpstreamRequestBody() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("ZIMA_PROXY_LOG_UPSTREAM_BODY")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (ph *ProxyHandler) requestBodyForLog(body []byte) string {
	if ph.dataMasker != nil && ph.dataMasker.IsEnabled() {
		return string(ph.dataMasker.MaskRequestBytes(body))
	}
	return string(body)
}

// forwardToProvider forwards the request to upstream provider.
// Deprecated: use buildUpstreamRequest + AuthProber.ProbeAndForward instead.
func (ph *ProxyHandler) forwardToProvider(r *http.Request, route *providerpool.RouteResult) (*http.Response, error) {
	provider := route.Provider

	// Use effective base URL (considers DetectedEndpoint)
	effectiveBaseURL := provider.EffectiveBaseURL()
	targetURL := provider.ParsedEffectiveBaseURL()
	if targetURL == nil {
		return nil, fmt.Errorf("invalid provider base URL: %s", effectiveBaseURL)
	}

	// Build upstream URL - avoid duplicate path segments
	upstreamURL := *targetURL
	requestPath := r.URL.Path

	if targetURL.Path != "" && targetURL.Path != "/" {
		basePath := strings.TrimSuffix(targetURL.Path, "/")
		if strings.HasPrefix(requestPath, basePath) {
			upstreamURL.Path = requestPath
		} else if idx := strings.LastIndex(basePath, "/v"); idx >= 0 {
			if trimmed := stripVersionPrefix(requestPath); trimmed != "" {
				upstreamURL.Path = singleJoiningSlash(basePath, trimmed)
			} else {
				upstreamURL.Path = singleJoiningSlash(targetURL.Path, requestPath)
			}
		} else {
			upstreamURL.Path = singleJoiningSlash(targetURL.Path, requestPath)
		}
	} else {
		upstreamURL.Path = requestPath
	}
	upstreamURL.RawQuery = r.URL.RawQuery

	// Create request
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL.String(), r.Body)
	if err != nil {
		return nil, err
	}

	// Copy headers
	copyHeaders(req.Header, r.Header)

	// Set authentication
	// Use x-api-key only for Anthropic-native endpoints (/v1/messages);
	// for OpenAI-compatible endpoints (/v1/chat/completions), always use Bearer token
	// because most relay/proxy services expect Bearer auth on OpenAI endpoints.
	slog.Info("[proxy] auth", "provider", provider.ID, "format", provider.APIFormat, "has_key", route.APIKey != nil && route.APIKey.Key != "", "path", upstreamURL.Path)
	if route.APIKey != nil && route.APIKey.Key != "" {
		isAnthropicEndpoint := strings.HasSuffix(upstreamURL.Path, "/messages")
		if isAnthropicEndpoint {
			req.Header.Set("x-api-key", route.APIKey.Key)
			req.Header.Set("anthropic-version", "2023-06-01")
			req.Header.Del("Authorization")
		} else {
			req.Header.Set("Authorization", "Bearer "+route.APIKey.Key)
			req.Header.Del("x-api-key")
		}
	} else if route.OAuth != nil && route.OAuth.Connected && ph.oauthManager != nil {
		// OAuth authentication — get fresh access token
		var token string
		var oauthErr error
		if provider.APIFormat == providerpool.APIFormatCopilot {
			// Copilot requires exchanging GitHub OAuth token for a Copilot-specific JWT
			token, oauthErr = ph.oauthManager.GetCopilotAccessToken(provider.ID)
			// Copilot API requires Editor-Version header for IDE authentication
			if oauthErr == nil {
				req.Header.Set("Editor-Version", "vscode/1.85.0")
			}
		} else {
			token, oauthErr = ph.oauthManager.GetAccessToken(provider.ID)
		}
		if oauthErr != nil {
			slog.Warn("[proxy] oauth token error", "provider", provider.ID, "error", oauthErr)
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Del("x-api-key")
		}
	}

	req.Host = targetURL.Host

	if provider.SkipTLSVerify {
		return ph.connPool.GetInsecureClient(provider.Name).Do(req)
	}
	return ph.connPool.GetClient(provider.Name).Do(req)
}

// handleModels handles GET /v1/models
func (ph *ProxyHandler) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ph.providerPool == nil {
		http.Error(w, "Provider pool not configured", http.StatusServiceUnavailable)
		return
	}

	var allModels []map[string]interface{}
	for _, provider := range ph.providerPool.Registry.ListEnabled() {
		models, err := ph.providerPool.Discovery.GetModels(provider.ID)
		if err != nil {
			continue
		}
		for _, model := range models {
			if !model.Enabled {
				continue
			}
			allModels = append(allModels, map[string]interface{}{
				"id":       model.ID,
				"object":   "model",
				"created":  model.CreatedAt.Unix(),
				"owned_by": provider.ID,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data":   allModels,
	})
}

// copyResponse copies response headers and body to the client.
// Uses pre-parsed request data to avoid redundant JSON parsing.
func (ph *ProxyHandler) copyResponse(w http.ResponseWriter, resp *http.Response, pr *parsedRequest, r *http.Request) {
	copyHeaders(w.Header(), resp.Header)
	upstreamPath := ""
	if resp.Request != nil && resp.Request.URL != nil {
		upstreamPath = strings.TrimSuffix(resp.Request.URL.Path, "/")
	}
	isResponsesEndpoint := strings.HasSuffix(upstreamPath, "/responses")

	// Inject actual provider/model so the bridge can propagate them to chat handler
	if pr.resolvedProvider != "" {
		w.Header().Set("X-Actual-Provider", pr.resolvedProvider)
	}
	if pr.resolvedModel != "" {
		w.Header().Set("X-Actual-Model", pr.resolvedModel)
	}

	if isStreamingResponse(resp) || pr.streaming {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(resp.StatusCode)

		// If upstream returned non-OpenAI SSE, convert to OpenAI SSE.
		if pr.upstreamFormat == ProviderTypeAnthropic || pr.upstreamFormat == ProviderTypeCloudCode {
			reader := io.Reader(resp.Body)
			var upstreamCapture bytes.Buffer
			if pr.upstreamFormat == ProviderTypeAnthropic && resp.StatusCode == http.StatusOK {
				reader = io.TeeReader(resp.Body, &upstreamCapture)
			}
			if err := sharedConverter.ConvertStreamingResponse(reader, pr.upstreamFormat, w); err != nil {
				slog.Warn("[proxy] stream conversion error", "source", pr.upstreamFormat, "error", err)
			}
			if pr.upstreamFormat == ProviderTypeAnthropic && upstreamCapture.Len() > 0 {
				ph.recordPromptCacheFromSSE(pr, upstreamCapture.Bytes())
			}
			return
		}

		needCapture := ph.shouldCaptureSessionUsage(r)
		captured := ph.copyStreamingResponseWithCapture(w, resp, needCapture)
		if needCapture {
			tokensIn, tokensOut := parseUsageTokensFromSSE(captured)
			ph.updateSessionUsage(r, pr, tokensIn, tokensOut, resp.StatusCode < http.StatusBadRequest)
		}
		return
	}

	// Fast path: no conversion, masking, or routing stats needed — stream directly.
	needsConversion := (pr.upstreamFormat == ProviderTypeAnthropic || pr.upstreamFormat == ProviderTypeCloudCode) && resp.StatusCode == http.StatusOK
	needsResponsesCompat := isResponsesEndpoint && resp.StatusCode == http.StatusOK
	needsMasking := ph.dataMasker != nil && ph.dataMasker.IsEnabled()
	needsRoutingStats := resp.StatusCode == http.StatusOK && pr.originalModel != pr.model
	needsSessionUsage := ph.shouldCaptureSessionUsage(r)
	if !needsConversion && !needsResponsesCompat && !needsMasking && !needsRoutingStats && !needsSessionUsage {
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	respBody, err := readBody(resp.Body)
	if err != nil {
		w.WriteHeader(resp.StatusCode)
		return
	}

	if pr.upstreamFormat == ProviderTypeAnthropic && resp.StatusCode == http.StatusOK {
		ph.recordPromptCacheFromBody(pr, respBody)
	}

	// Convert non-streaming non-OpenAI response to OpenAI format.
	if (pr.upstreamFormat == ProviderTypeAnthropic || pr.upstreamFormat == ProviderTypeCloudCode) && resp.StatusCode == http.StatusOK {
		if converted, convErr := sharedConverter.ConvertResponse(respBody, pr.upstreamFormat); convErr == nil {
			respBody = converted
		}
	}
	// Convert Responses API payload to chat-completions payload when request routing
	// went through a /responses endpoint.
	if isResponsesEndpoint && resp.StatusCode == http.StatusOK {
		if converted, convErr := convertResponsesToOpenAIChatCompletions(respBody); convErr == nil {
			respBody = converted
		}
	}

	// Mask sensitive data in response body (non-streaming only)
	if dm := ph.dataMasker; dm != nil && dm.IsEnabled() {
		masked := dm.MaskResponseBytes(respBody)
		if !bytes.Equal(masked, respBody) {
			respBody = masked
			w.Header().Set("X-Data-Masked", "true")
		}
	}

	// Track routing cost savings (non-streaming only — streaming has no full body here)
	if resp.StatusCode == http.StatusOK && pr.originalModel != pr.model {
		ph.routingStats.Record(pr.originalModel, pr.model, respBody)
	}
	if needsSessionUsage {
		inTokens, outTokens := parseUsageTokens(respBody)
		ph.updateSessionUsage(r, pr, int64(inTokens), int64(outTokens), resp.StatusCode < http.StatusBadRequest)
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func (ph *ProxyHandler) shouldCaptureSessionUsage(r *http.Request) bool {
	if ph.providerPool != nil && ph.providerPool.UsageTracker != nil {
		return true
	}
	return ph.sessionMonitor != nil && SessionIDFromContext(r.Context()) != ""
}

func (ph *ProxyHandler) updateSessionUsage(r *http.Request, pr *parsedRequest, tokensIn, tokensOut int64, success bool) {
	model := pr.resolvedModel
	if model == "" {
		model = pr.model
	}

	if ph.sessionMonitor != nil {
		sessionID := SessionIDFromContext(r.Context())
		if sessionID != "" {
			provider := pr.resolvedProvider
			ph.sessionMonitor.UpdateSession(sessionID, provider, model, tokensIn, tokensOut)
		}
	}

	if ph.providerPool != nil && ph.providerPool.UsageTracker != nil {
		providerID := pr.resolvedProviderID
		if providerID == "" {
			// Fallback for non-standard paths; provider ID is preferred for aggregation keys.
			providerID = pr.resolvedProvider
		}
		if providerID != "" {
			ph.providerPool.UsageTracker.RecordRequest(providerID, model, tokensIn, tokensOut, 0, success)
		}
	}
}

func parseUsageTokensFromSSE(data []byte) (int64, int64) {
	if len(data) == 0 {
		return 0, 0
	}
	chunks := parseSSEChunks(data)
	for i := len(chunks) - 1; i >= 0; i-- {
		usage, ok := chunks[i]["usage"]
		if !ok || usage == nil {
			continue
		}
		asMap, ok := usage.(map[string]interface{})
		if !ok {
			continue
		}
		return usageMapTokens(asMap)
	}
	return 0, 0
}

func (ph *ProxyHandler) recordPromptCacheFromBody(pr *parsedRequest, body []byte) {
	if len(body) == 0 || ph.promptCacheStats == nil {
		return
	}
	usage := gjson.GetBytes(body, "usage")
	if !usage.Exists() {
		return
	}
	input := usage.Get("input_tokens").Int()
	cacheRead := usage.Get("cache_read_input_tokens").Int()
	cacheCreation := usage.Get("cache_creation_input_tokens").Int()
	ph.recordPromptCache(pr, input, cacheRead, cacheCreation)
}

func (ph *ProxyHandler) recordPromptCacheFromSSE(pr *parsedRequest, data []byte) {
	if len(data) == 0 || ph.promptCacheStats == nil {
		return
	}
	chunks := parseSSEChunks(data)
	for i := len(chunks) - 1; i >= 0; i-- {
		usage, ok := chunks[i]["usage"]
		if !ok || usage == nil {
			continue
		}
		asMap, ok := usage.(map[string]interface{})
		if !ok {
			continue
		}
		input := getIntField(asMap, "input_tokens")
		cacheRead := getIntField(asMap, "cache_read_input_tokens")
		cacheCreation := getIntField(asMap, "cache_creation_input_tokens")
		ph.recordPromptCache(pr, input, cacheRead, cacheCreation)
		return
	}
}

func (ph *ProxyHandler) recordPromptCache(pr *parsedRequest, input, cacheRead, cacheCreation int64) {
	if ph.promptCacheStats == nil {
		return
	}
	if input == 0 && cacheRead == 0 && cacheCreation == 0 {
		return
	}
	ph.promptCacheStats.Record(int(input), int(cacheRead), int(cacheCreation))

	provider := pr.resolvedProvider
	if provider == "" {
		provider = pr.resolvedProviderID
	}
	model := pr.resolvedModel
	if model == "" {
		model = pr.model
	}
	LogTokenChurn(provider, model, int(input), int(cacheRead), int(cacheCreation))
}

func usageMapTokens(usage map[string]interface{}) (int64, int64) {
	prompt := getIntField(usage, "prompt_tokens")
	completion := getIntField(usage, "completion_tokens")
	if prompt > 0 || completion > 0 {
		return prompt, completion
	}
	return getIntField(usage, "input_tokens"), getIntField(usage, "output_tokens")
}

func getIntField(m map[string]interface{}, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case int32:
		return int64(n)
	default:
		return 0
	}
}

// copyStreamingResponseWithCapture streams SSE to client while capturing raw data.
// Returns the captured SSE bytes for cache assembly.
// When needCapture is false, streams directly without buffering (minimal-copy path).
// Uses pooled 32KB buffers to reduce GC pressure.
func (ph *ProxyHandler) copyStreamingResponseWithCapture(w http.ResponseWriter, resp *http.Response, needCapture bool) []byte {
	flusher, ok := w.(http.Flusher)
	if !ok {
		data, _ := readBody(resp.Body)
		w.Write(data)
		return data
	}

	bufPtr := sseBufferPool.Get().(*[]byte)
	buf := *bufPtr
	defer sseBufferPool.Put(bufPtr)

	// Fast path: no capture needed — use io.CopyBuffer for kernel-optimized copy
	if !needCapture {
		fw := &flushWriter{w: w, f: flusher}
		io.CopyBuffer(fw, resp.Body, buf)
		return nil
	}

	var capture bytes.Buffer
	fw := &flushWriter{w: w, f: flusher}
	tee := io.TeeReader(resp.Body, &capture)
	io.CopyBuffer(fw, tee, buf)
	return capture.Bytes()
}

// flushWriter wraps a ResponseWriter+Flusher to flush after every Write.
// This enables io.CopyBuffer to drive the streaming loop efficiently.
type flushWriter struct {
	w io.Writer
	f http.Flusher
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	fw.f.Flush()
	return n, err
}

// peekReader prepends already-read bytes back onto a reader.
// Used to reconstruct a response body after peeking at the first bytes.
type peekReader struct {
	prefix []byte
	rest   io.ReadCloser
	off    int
}

func (pr *peekReader) Read(p []byte) (int, error) {
	if pr.off < len(pr.prefix) {
		n := copy(p, pr.prefix[pr.off:])
		pr.off += n
		return n, nil
	}
	return pr.rest.Read(p)
}

func (pr *peekReader) Close() error {
	return pr.rest.Close()
}

// isStreamingResponse checks if response is streaming
func isStreamingResponse(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	return strings.Contains(ct, "text/event-stream") || strings.Contains(ct, "application/x-ndjson")
}

// Pre-allocated hop-by-hop headers map for O(1) lookup
var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailers":            true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

// copyHeaders copies headers from src to dst, skipping hop-by-hop headers.
// Uses direct slice assignment instead of Add() to avoid per-value map lookups.
func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		if hopByHopHeaders[key] {
			continue
		}
		dst[key] = values
	}
}

// isHopByHopHeader checks if header is a hop-by-hop header
func isHopByHopHeader(header string) bool {
	return hopByHopHeaders[header]
}

// applyModelRouting evaluates the rule engine and model router to potentially
// swap the requested model to a cheaper/smaller one. Mutates pr.model and pr.body.
func (ph *ProxyHandler) applyModelRouting(r *http.Request, pr *parsedRequest) {
	if pr.model == "" || !ph.routingEnabled.Load() {
		return
	}

	// 1. Rule engine: condition-based tier routing (header, body size, tool pattern, system tag)
	if ph.ruleEngine != nil {
		req := RouteRequest{
			Headers:  r.Header,
			BodySize: len(pr.body),
		}

		// Extract tools/system only if pre-computed flags say rules need them
		if ph.ruleEngine.needsTools {
			toolNames := gjson.GetBytes(pr.body, "tools.#.function.name").Array()
			if len(toolNames) > 0 {
				tools := make([]string, len(toolNames))
				for j, t := range toolNames {
					tools[j] = t.Str
				}
				req.ToolNames = tools
			}
		}
		if ph.ruleEngine.needsSystem {
			messages := gjson.GetBytes(pr.body, "messages")
			if messages.Exists() {
				for _, msg := range messages.Array() {
					if msg.Get("role").Str == "system" {
						req.SystemMessage = msg.Get("content").Str
						break
					}
				}
			}
			if req.SystemMessage == "" {
				req.SystemMessage = gjson.GetBytes(pr.body, "system").Str
			}
		}

		decision := ph.ruleEngine.Evaluate(&req)
		if decision != nil && decision.Matched {
			// Verify the target model is actually available in the provider pool
			// before committing to the swap. If no provider supports the target model,
			// skip the routing and keep the original model.
			if ph.providerPool != nil && ph.providerPool.Router != nil && !ph.providerPool.Router.HasModel(decision.Model) {
				slog.Debug("[proxy] routing rule matched but target model not available, skipping",
					"rule", decision.Rule, "target", decision.Model, "original", pr.model)
			} else {
				pr.routed = decision
				pr.model = decision.Model
				pr.body = replaceModelInBody(pr.body, decision.Model)
				return
			}
		}
	}

	// 2. Model router: family-based routing + background task downgrade
	if ph.modelRouter != nil {
		isBackground := ph.modelRouter.IsBackgroundRequest(r)
		route, err := ph.modelRouter.RouteModel(pr.model, isBackground)
		if err == nil && route.TargetModel != pr.model {
			// Verify the target model is available before swapping
			if ph.providerPool != nil && ph.providerPool.Router != nil && !ph.providerPool.Router.HasModel(route.TargetModel) {
				slog.Debug("[proxy] model router target not available, keeping original",
					"target", route.TargetModel, "original", pr.model)
			} else {
				pr.model = route.TargetModel
				pr.body = replaceModelInBody(pr.body, route.TargetModel)
			}
		}
	}
}

// modelKeyPattern is the byte pattern for locating the "model" JSON key.
var modelKeyPattern = []byte(`"model"`)

// replaceModelInBody replaces the "model" field value in JSON body using direct
// byte scanning. Single allocation (the output []byte). Falls back to sjson if
// the field can't be located by simple scan.
func replaceModelInBody(body []byte, newModel string) []byte {
	// Find "model" key
	idx := bytes.Index(body, modelKeyPattern)
	if idx < 0 {
		return body
	}
	// Skip past "model" and find the colon, then the opening quote of the value
	pos := idx + len(modelKeyPattern)
	for pos < len(body) && body[pos] != ':' {
		pos++
	}
	pos++ // skip ':'
	for pos < len(body) && (body[pos] == ' ' || body[pos] == '\t') {
		pos++
	}
	if pos >= len(body) || body[pos] != '"' {
		// Not a string value — fall back
		if nb, err := sjson.SetBytes(body, "model", newModel); err == nil {
			return nb
		}
		return body
	}
	valStart := pos // opening quote
	pos++           // skip opening quote
	for pos < len(body) && body[pos] != '"' {
		if body[pos] == '\\' {
			pos++ // skip escaped char
		}
		pos++
	}
	if pos >= len(body) {
		return body
	}
	valEnd := pos + 1 // past closing quote

	// Build output: body[:valStart] + "newModel" + body[valEnd:]
	newQuotedLen := 2 + len(newModel) // quotes + model name
	out := make([]byte, len(body)-(valEnd-valStart)+newQuotedLen)
	n := copy(out, body[:valStart])
	out[n] = '"'
	n++
	n += copy(out[n:], newModel)
	out[n] = '"'
	n++
	copy(out[n:], body[valEnd:])
	return out
}

// setRouteHeaders adds routing observability headers to the response.
func (ph *ProxyHandler) setRouteHeaders(w http.ResponseWriter, pr *parsedRequest) {
	if pr.routed != nil {
		w.Header().Set("X-Route-Rule", pr.routed.Rule)
		w.Header().Set("X-Route-Model", pr.routed.Model)
		if pr.routed.Tier != "" {
			w.Header().Set("X-Route-Tier", string(pr.routed.Tier))
		}
	}
}

// stripImageContent removes image_url entries from messages[].content arrays.
// Returns the modified body and true if any images were stripped.
func stripImageContent(body []byte) ([]byte, bool) {
	bodyStr := unsafeString(body)
	messages := gjson.Get(bodyStr, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return body, false
	}
	stripped := false
	result := make([]byte, len(body))
	copy(result, body)
	// Iterate messages in reverse so index shifts don't affect earlier entries
	msgs := messages.Array()
	for i := len(msgs) - 1; i >= 0; i-- {
		content := msgs[i].Get("content")
		if !content.IsArray() {
			continue
		}
		parts := content.Array()
		var textParts []gjson.Result
		hasImage := false
		for _, part := range parts {
			if part.Get("type").Str == "image_url" {
				hasImage = true
			} else {
				textParts = append(textParts, part)
			}
		}
		if !hasImage {
			continue
		}
		stripped = true
		path := fmt.Sprintf("messages.%d.content", i)
		if len(textParts) == 0 {
			// All content was images — replace with empty text
			if b, e := sjson.SetBytes(result, path, ""); e == nil {
				result = b
			}
		} else if len(textParts) == 1 && textParts[0].Get("type").Str == "text" {
			// Single text part remaining — simplify to plain string
			if b, e := sjson.SetBytes(result, path, textParts[0].Get("text").Str); e == nil {
				result = b
			}
		} else {
			// Multiple non-image parts — rebuild the array as raw JSON
			var buf bytes.Buffer
			buf.WriteByte('[')
			for j, tp := range textParts {
				if j > 0 {
					buf.WriteByte(',')
				}
				buf.WriteString(tp.Raw)
			}
			buf.WriteByte(']')
			if b, e := sjson.SetRawBytes(result, path, buf.Bytes()); e == nil {
				result = b
			}
		}
	}
	return result, stripped
}

// singleJoiningSlash joins two URL paths
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

// stripVersionPrefix strips a leading /v{N} prefix from a path (e.g. /v1/chat/completions → /chat/completions).
// Returns the stripped path, or "" if the path doesn't start with a version prefix.
func stripVersionPrefix(path string) string {
	if len(path) < 3 || path[0] != '/' || path[1] != 'v' {
		return ""
	}
	// Find end of version number: /v1, /v2, /v1beta, /v4, etc.
	i := 2
	for i < len(path) && path[i] != '/' {
		i++
	}
	if i >= len(path) {
		// Path is just "/v1" with no trailing content
		return "/"
	}
	return path[i:] // e.g. "/chat/completions"
}

// isResponsesEndpointBaseURL reports whether a provider base URL already targets
// a fixed responses endpoint path (e.g. /backend-api/codex/responses).
func isResponsesEndpointBaseURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	path := strings.TrimSuffix(u.Path, "/")
	return path != "" && strings.HasSuffix(path, "/responses")
}

func disableResponsesContinuation(r *http.Request) bool {
	if r == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(r.Header.Get(DisableResponsesContinuationHeader)))
	return v == "1" || v == "true" || v == "yes"
}

func stripPreviousResponseID(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	out, err := sjson.DeleteBytes(body, "previous_response_id")
	if err != nil {
		return body
	}
	return out
}
