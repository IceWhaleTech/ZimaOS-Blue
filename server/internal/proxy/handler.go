package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
)

// ResolvedRoute carries the actual provider/model chosen by the router.
// Callers embed a pointer in the request context; the proxy handler populates it.
type ResolvedRoute struct {
	Provider string
	Model    string
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

// GetResolvedRouteFromContext is the exported version of getResolvedRoute.
func GetResolvedRouteFromContext(ctx context.Context) *ResolvedRoute {
	return getResolvedRoute(ctx)
}

// parsedRequest holds pre-parsed request data to avoid redundant JSON parsing.
// Created once in ServeHTTP and passed through the call chain.
// Uses gjson for zero-alloc field extraction instead of map[string]interface{}.
// Layout: pointer-sized fields first, then bools — minimizes padding for cache-line efficiency.
type parsedRequest struct {
	body             []byte
	model            string
	originalModel    string         // model before routing (for cost savings tracking)
	upstreamFormat   ProviderType   // set when request was converted to non-OpenAI format
	routed           *RouteDecision // non-nil if rule engine rerouted the model
	streaming        bool
	resolvedProvider string // actual provider name after routing
	resolvedModel    string // actual model ID after routing
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
	connPool       *ConnectionPool    // HTTP connection pool
	providerPool   *providerpool.Pool // Provider Pool for routing and API keys
	authProber     *AuthProber        // Auth strategy probing with memory
	providerMemory *ProviderMemory    // Provider capability memory
	routingEnabled atomic.Bool        // Toggle for model routing
	promptCacheEnabled atomic.Bool    // Toggle for Anthropic prompt caching

	// Warm path — accessed conditionally
	failover        *FailoverHandler   // Failover handler
	prunerMw        *pruner.Middleware // Context pruner middleware (optional)
	modelRouter     *ModelRouter       // Model family routing + background downgrade
	ruleEngine      *RuleEngine        // Condition-based tier routing
	tierResolver    *TierResolver      // Dynamic model tier classification

	// Cold path — rarely accessed per-request
	router          *Router            // Legacy router (fallback only)
	apiKeyValidator func(key string) ([]string, error)
	routingStats    *RoutingStats      // Routing cost savings tracker
	pipelineStats   *PipelineStatsCollector // Unified pipeline stats collector
	oauthManager    OAuthTokenProvider // OAuth token provider (optional)
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(router *Router, connPool *ConnectionPool, failover *FailoverHandler) *ProxyHandler {
	ph := &ProxyHandler{
		router:   router,
		connPool: connPool,
		failover: failover,
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
}

// OAuthTokenProvider provides OAuth access tokens for providers.
type OAuthTokenProvider interface {
	GetAccessToken(providerID string) (string, error)
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
		strategies := ph.authProber.Strategies(p, apiKey)
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
	if ph.prunerMw != nil && ph.prunerMw.Enabled() {
		// Read prune stats slot from context (set by chat handler before bridge call).
		// If present, the middleware populates it with per-request stats.
		if pruned, err := ph.prunerMw.ProcessRequest(r.Context(), bodyBytes); err == nil {
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
	slog.Debug("[proxy] routing request", "model", pr.model, "streaming", pr.streaming, "mode", routingMode, "body", string(pr.body))

	routeReq := &providerpool.RouteRequest{
		ModelID: pr.model,
		Mode:    providerpool.RoutingMode(routingMode),
	}

	// Failover is enabled by default; only disabled when explicitly configured off
	failoverDisabled := ph.failover != nil && ph.failover.Config() != nil && !ph.failover.Config().Enabled

	var finalResp *http.Response
	executeOnProvider := func(result *providerpool.RouteResult) error {
		pid := result.Provider.ID
		burl := result.Provider.BaseURL

		// Throttle check: skip provider if recently 429'd
		if ph.providerMemory.IsThrottled(pid, burl) {
			slog.Debug("[proxy] skipping throttled provider", "provider", pid)
			return fmt.Errorf("provider %s is throttled", pid)
		}

		// Try all Model × Format combinations on this provider
		resp, format, _, tryErr := ph.tryOnProvider(r, result, pr)
		if tryErr != nil {
			return tryErr
		}

		// For streaming responses, peek at the first bytes to detect empty streams.
		// Some providers return HTTP 200 with an empty body or only "[DONE]".
		// Detecting this early allows RouteWithFallback to try the next provider.
		if pr.streaming && resp.Body != nil {
			peekBuf := make([]byte, 32)
			n, peekErr := resp.Body.Read(peekBuf)
			if n == 0 || peekErr != nil {
				resp.Body.Close()
				slog.Warn("[proxy] streaming response body empty", "provider", pid, "error", peekErr)
				return fmt.Errorf("provider %s returned empty streaming response", pid)
			}
			// Reconstruct body: peeked bytes + rest of original body
			resp.Body = &peekReader{prefix: peekBuf[:n], rest: resp.Body}
		}

		if format == providerpool.APIFormatAnthropic {
			pr.upstreamFormat = ProviderTypeAnthropic
		}
		// Capture resolved provider/model for downstream headers
		pr.resolvedProvider = result.Provider.Name
		if pr.resolvedProvider == "" {
			pr.resolvedProvider = result.Provider.ID
		}
		if result.Model != nil && result.Model.ID != "" {
			pr.resolvedModel = result.Model.ID
		}
		// Also propagate via context for callers that can't read response headers
		if rr := getResolvedRoute(r.Context()); rr != nil {
			rr.Provider = pr.resolvedProvider
			rr.Model = pr.resolvedModel
		}
		finalResp = resp
		return nil
	}

	var err error
	if failoverDisabled {
		// Failover explicitly disabled: only try the primary provider
		result, routeErr := ph.providerPool.Router.Route(routeReq)
		if routeErr != nil {
			err = routeErr
		} else {
			err = executeOnProvider(result)
		}
	} else {
		err = ph.providerPool.Router.RouteWithFallback(r.Context(), routeReq, executeOnProvider)
	}

	if err != nil {
		slog.Error("[proxy] all providers failed", "model", pr.model, "error", err)
		http.Error(w, SanitizeError(err), http.StatusBadGateway)
		return
	}
	defer finalResp.Body.Close()
	slog.Debug("[proxy] upstream response", "status", finalResp.StatusCode, "streaming", pr.streaming, "content_type", finalResp.Header.Get("Content-Type"))

	ph.copyResponse(w, finalResp, pr)
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

	targetURL := provider.ParsedBaseURL()
	if targetURL == nil {
		return nil, fmt.Errorf("invalid provider base URL: %s", provider.BaseURL)
	}

	// Build upstream URL - avoid duplicate path segments
	upstreamURL := *targetURL
	requestPath := r.URL.Path

	// Format conversion: if provider expects Anthropic format but request is OpenAI,
	// convert body and switch path to /v1/messages.
	if effectiveFormat == providerpool.APIFormatAnthropic && strings.Contains(requestPath, "/chat/completions") {
		converted, newPath, convErr := sharedConverter.ConvertRequest(body, ProviderTypeAnthropic)
		if convErr == nil {
			body = converted
			requestPath = newPath // "/v1/messages"
			// Apply Anthropic prompt caching if enabled
			if ph.promptCacheEnabled.Load() {
				if cached, cacheErr := InjectPromptCaching(body); cacheErr == nil {
					body = cached
				}
			}
			slog.Debug("[proxy] converted OpenAI→Anthropic format", "provider", provider.ID, "path", newPath)
		} else {
			slog.Warn("[proxy] format conversion failed, sending as-is", "provider", provider.ID, "error", convErr)
		}
	}

	if targetURL.Path != "" && targetURL.Path != "/" {
		basePath := strings.TrimSuffix(targetURL.Path, "/")
		if strings.HasPrefix(requestPath, basePath) {
			upstreamURL.Path = requestPath
		} else {
			upstreamURL.Path = singleJoiningSlash(targetURL.Path, requestPath)
		}
	} else {
		upstreamURL.Path = requestPath
	}
	upstreamURL.RawQuery = r.URL.RawQuery

	fullURL := upstreamURL.String()
	slog.Debug("[proxy] upstream request", "url", fullURL, "method", r.Method, "format", effectiveFormat, "body_len", len(body), "body", string(body))

	req, err := http.NewRequestWithContext(r.Context(), r.Method, fullURL, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return nil, err
	}

	copyHeaders(req.Header, r.Header)
	req.Host = targetURL.Host

	return req, nil
}

// tryModelAliases attempts common model name aliases when the original gets model_not_found.
func (ph *ProxyHandler) tryModelAliases(r *http.Request, result *providerpool.RouteResult, pr *parsedRequest, failedModel string, effectiveFormat providerpool.APIFormat) *http.Response {
	aliases, ok := ModelAliases[failedModel]
	if !ok {
		return nil
	}
	pid := result.Provider.ID
	burl := result.Provider.BaseURL

	for _, alias := range aliases {
		aliasBody, err := sjson.SetBytes(pr.body, "model", alias)
		if err != nil {
			continue
		}
		slog.Debug("[proxy] trying model alias", "provider", pid, "original", failedModel, "alias", alias)

		resp, probeErr := ph.authProber.ProbeAndForward(
			result.Provider, result.APIKey,
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
	if !has(provider.APIFormat) {
		buf[n] = provider.APIFormat
		n++
	}

	// 4. Remaining formats
	for _, f := range [...]providerpool.APIFormat{providerpool.APIFormatOpenAI, providerpool.APIFormatAnthropic} {
		if !has(f) {
			buf[n] = f
			n++
		}
	}

	return buf, n, known
}

// persistDetectedFormat saves the detected API format to the provider for persistence across restarts.
// Only updates if the format changed, to avoid unnecessary writes.
func (ph *ProxyHandler) persistDetectedFormat(provider *providerpool.Provider, format providerpool.APIFormat) {
	if provider.DetectedFormat == format {
		return // already persisted
	}
	provider.DetectedFormat = format
	provider.DetectedAt = time.Now()
	if ph.providerPool != nil && ph.providerPool.Registry != nil {
		// Async persist — don't block the request path on DB write
		go func(p *providerpool.Provider) {
			if err := ph.providerPool.Registry.Update(p); err != nil {
				slog.Warn("[proxy] failed to persist detected format", "provider", p.ID, "format", format, "error", err)
			}
		}(provider)
	}
}

// allModelsForProvider returns model candidates to try for a provider.
// Remembered alias first, then original model, then ModelAliases.
// Uses stack-allocated array for the common case (≤8 candidates).
func (ph *ProxyHandler) allModelsForProvider(pid, burl, originalModel string, routedModel string) ([8]string, int) {
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
		if !ph.providerMemory.IsModelBlacklisted(pid, burl, alias) {
			add(alias)
		}
	}

	// 2. Routed model (from provider pool)
	if routedModel != "" && !has(routedModel) {
		if !ph.providerMemory.IsModelBlacklisted(pid, burl, routedModel) {
			add(routedModel)
		}
	}

	// 3. Original model
	if !has(originalModel) {
		if !ph.providerMemory.IsModelBlacklisted(pid, burl, originalModel) {
			add(originalModel)
		}
	}

	// 4. ModelAliases
	if aliases, ok := ModelAliases[originalModel]; ok {
		for _, alias := range aliases {
			if !has(alias) && !ph.providerMemory.IsModelBlacklisted(pid, burl, alias) {
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
	burl := result.Provider.BaseURL

	routedModel := ""
	if result.Model != nil && result.Model.ID != pr.model {
		routedModel = result.Model.ID
	}

	formatsBuf, nFormats, formatKnown := ph.allFormatsForProvider(pid, burl, result.Provider)
	allFormats := nFormats // remember full count before truncation
	modelsBuf, nModels := ph.allModelsForProvider(pid, burl, pr.model, routedModel)

	if nModels == 0 {
		return nil, "", "", fmt.Errorf("all models blacklisted on provider %s", pid)
	}

	// If format is known (detected or remembered), only try that one — skip fallback formats.
	// On format mismatch (422 / wrapped 404), nFormats is expanded back to allFormats.
	if formatKnown {
		nFormats = 1
	}

	// Fast path: single model + single format + model matches request (most common happy path).
	// Avoids loop overhead, sjson.SetBytes, and slice iteration.
	if nModels == 1 && nFormats == 1 && modelsBuf[0] == pr.model {
		format := formatsBuf[0]
		slog.Debug("[proxy] trying", "provider", pid, "format", format, "model", pr.model)
		resp, probeErr := ph.authProber.ProbeAndForward(
			result.Provider,
			result.APIKey,
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
			return nil, "", "", probeErr
		}
		if resp.StatusCode < 400 {
			ph.providerMemory.RememberFormat(pid, burl, string(format))
			ph.persistDetectedFormat(result.Provider, format)
			return resp, format, pr.model, nil
		}
		// Format mismatch on fast path — expand to all formats and fall through to general loop
		if allFormats > 1 && isFormatMismatchError(resp.StatusCode, readErrorBody(resp.Body)) {
			resp.Body.Close()
			slog.Warn("[proxy] format mismatch on fast path, expanding to all formats",
				"provider", pid, "format", format, "model", pr.model)
			ph.providerMemory.ForgetFormat(pid, burl)
			nFormats = allFormats
		}
		// Fall through to error handling in the general loop
	}

	var lastErr error
	for mi := 0; mi < nModels; mi++ {
		model := modelsBuf[mi]
		// Build body with this model
		forwardBody := pr.body
		if model != pr.model {
			if newBody, err := sjson.SetBytes(pr.body, "model", model); err == nil {
				forwardBody = newBody
			}
		}

		for fi := 0; fi < nFormats; fi++ {
			format := formatsBuf[fi]
			slog.Debug("[proxy] trying", "provider", pid, "format", format, "model", model)

			resp, probeErr := ph.authProber.ProbeAndForward(
				result.Provider,
				result.APIKey,
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
				lastErr = probeErr
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
			statusCode := resp.StatusCode
			errBody := readErrorBody(resp.Body)
			resp.Body.Close()
			errStr := string(errBody)
			if len(errStr) > 256 {
				errStr = errStr[:256]
			}

			if statusCode == http.StatusTooManyRequests {
				retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
				ph.providerMemory.RememberThrottle(pid, burl, retryAfter)
				return nil, "", "", fmt.Errorf("provider %s throttled (429)", pid)
			}

			if statusCode >= 500 {
				// 5xx: server error, skip entire provider
				slog.Warn("[proxy] upstream 5xx", "provider", pid, "status", statusCode)
				return nil, "", "", fmt.Errorf("upstream %d: %s", statusCode, errStr)
			}

			// Format mismatch: 422, or 404/400 with format-related error body.
			// Check BEFORE isModelNotConfiguredError since both match 404/422.
			// Don't blacklist the model — try the next format instead.
			if isFormatMismatchError(statusCode, errBody) {
				slog.Warn("[proxy] format mismatch, trying next format",
					"provider", pid, "format", format, "model", model, "status", statusCode, "body", errStr)
				lastErr = fmt.Errorf("provider returned %d: %s", statusCode, errStr)
				// If we're on the last format and there are more available, expand
				if fi == nFormats-1 && allFormats > nFormats {
					ph.providerMemory.ForgetFormat(pid, burl)
					nFormats = allFormats
				}
				continue
			}

			// Check if this is a "not configured" / "model not found" error
			if isModelNotConfiguredError(statusCode, errBody) {
				slog.Warn("[proxy] model not configured on provider, blacklisting and trying next",
					"provider", pid, "format", format, "model", model, "status", statusCode, "body", errStr)
				// Blacklist THIS model on THIS provider (per-provider scope) so we don't retry it
				ph.providerMemory.BlacklistModel(pid, burl, model)
				lastErr = fmt.Errorf("model %s not configured on provider %s: %s", model, pid, errStr)
				// Skip remaining formats for this model — if model isn't configured,
				// trying a different format won't help
				break
			}

			// Other 4xx: blacklist this model on this provider, try next model/format
			slog.Warn("[proxy] 4xx, trying next combination",
				"provider", pid, "format", format, "model", model, "status", statusCode, "body", errStr)
			ph.providerMemory.BlacklistModel(pid, burl, model)
			lastErr = fmt.Errorf("provider returned %d: %s", statusCode, errStr)
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

// formatMismatchPatterns are error body patterns that indicate the request format
// doesn't match what the provider expects (e.g. OpenAI body sent to Anthropic endpoint).
var formatMismatchPatterns = [][]byte{
	[]byte("unsupported request"),
	[]byte("bad_response_status_code"),
	[]byte("openai_error"),
	[]byte("invalid request format"),
	[]byte("unexpected content type"),
}

// isFormatMismatchError returns true if the error indicates a request format mismatch
// rather than a model configuration issue. These errors should trigger format fallback,
// not model blacklisting.
func isFormatMismatchError(statusCode int, body []byte) bool {
	if statusCode != 400 && statusCode != 404 && statusCode != 422 {
		return false
	}
	// 422 is almost always a format mismatch
	if statusCode == 422 {
		return true
	}
	// For 400/404, check body for format-related patterns
	lower := toLowerBytes(body)
	for _, pattern := range formatMismatchPatterns {
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

// forwardToProvider forwards the request to upstream provider.
// Deprecated: use buildUpstreamRequest + AuthProber.ProbeAndForward instead.
func (ph *ProxyHandler) forwardToProvider(r *http.Request, route *providerpool.RouteResult) (*http.Response, error) {
	provider := route.Provider

	targetURL := provider.ParsedBaseURL()
	if targetURL == nil {
		return nil, fmt.Errorf("invalid provider base URL: %s", provider.BaseURL)
	}

	// Build upstream URL - avoid duplicate path segments
	upstreamURL := *targetURL
	requestPath := r.URL.Path

	if targetURL.Path != "" && targetURL.Path != "/" {
		basePath := strings.TrimSuffix(targetURL.Path, "/")
		if strings.HasPrefix(requestPath, basePath) {
			upstreamURL.Path = requestPath
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
		isAnthropicEndpoint := strings.Contains(upstreamURL.Path, "/messages")
		if provider.APIFormat == providerpool.APIFormatAnthropic && isAnthropicEndpoint {
			req.Header.Set("x-api-key", route.APIKey.Key)
			req.Header.Set("anthropic-version", "2023-06-01")
			req.Header.Del("Authorization")
		} else {
			req.Header.Set("Authorization", "Bearer "+route.APIKey.Key)
			req.Header.Del("x-api-key")
		}
	} else if route.OAuth != nil && route.OAuth.Connected && ph.oauthManager != nil {
		// OAuth authentication — get fresh access token
		token, oauthErr := ph.oauthManager.GetAccessToken(provider.ID)
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
func (ph *ProxyHandler) copyResponse(w http.ResponseWriter, resp *http.Response, pr *parsedRequest) {
	copyHeaders(w.Header(), resp.Header)

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

		// If upstream returned Anthropic SSE, convert to OpenAI SSE
		if pr.upstreamFormat == ProviderTypeAnthropic {
			if err := sharedConverter.ConvertStreamingResponse(resp.Body, ProviderTypeAnthropic, w); err != nil {
				slog.Warn("[proxy] anthropic stream conversion error", "error", err)
			}
			return
		}

		ph.copyStreamingResponseWithCapture(w, resp, false)
		return
	}

	respBody, err := readBody(resp.Body)
	if err != nil {
		w.WriteHeader(resp.StatusCode)
		return
	}

	// Convert non-streaming Anthropic response to OpenAI format
	if pr.upstreamFormat == ProviderTypeAnthropic && resp.StatusCode == http.StatusOK {
		if converted, convErr := sharedConverter.ConvertResponse(respBody, ProviderTypeAnthropic); convErr == nil {
			respBody = converted
		}
	}

	// Track routing cost savings (non-streaming only — streaming has no full body here)
	if resp.StatusCode == http.StatusOK && pr.originalModel != pr.model {
		ph.routingStats.Record(pr.originalModel, pr.model, respBody)
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
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
