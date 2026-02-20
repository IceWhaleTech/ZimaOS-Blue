package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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

// parsedRequest holds pre-parsed request data to avoid redundant JSON parsing.
// Created once in ServeHTTP and passed through the call chain.
// Uses gjson for zero-alloc field extraction instead of map[string]interface{}.
type parsedRequest struct {
	body           []byte
	model          string
	originalModel  string         // model before routing (for cost savings tracking)
	streaming      bool
	cacheKey       string         // computed once if cache enabled
	routed         *RouteDecision // non-nil if rule engine rerouted the model
	upstreamFormat ProviderType   // set when request was converted to non-OpenAI format
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

// readBody reads an io.Reader into a []byte using a pooled buffer.
func readBody(r io.Reader) ([]byte, error) {
	buf := bodyBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bodyBufferPool.Put(buf)
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, err
	}
	// Return a copy — the buffer goes back to the pool
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
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
type ProxyHandler struct {
	router          *Router            // Legacy router (fallback only)
	connPool        *ConnectionPool    // HTTP connection pool
	failover        *FailoverHandler   // Failover handler
	providerPool    *providerpool.Pool // Provider Pool for routing and API keys
	cache           *CCCache           // Response cache (cc-cache)
	prunerMw        *pruner.Middleware // Context pruner middleware (optional)
	apiKeyValidator func(key string) ([]string, error)
	modelRouter     *ModelRouter       // Model family routing + background downgrade
	ruleEngine      *RuleEngine        // Condition-based tier routing (header/body/tool/tag)
	routingEnabled  atomic.Bool        // Toggle for model routing (rule engine + model router)
	authProber      *AuthProber        // Auth strategy probing with memory
	providerMemory  *ProviderMemory    // Provider capability memory (format, model, tools, throttle)
	routingStats    *RoutingStats      // Routing cost savings tracker
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

// SetCache sets the response cache for the proxy handler.
func (ph *ProxyHandler) SetCache(cache *CCCache) {
	ph.cache = cache
}

// SetPruner sets the context pruner middleware for the proxy handler.
func (ph *ProxyHandler) SetPruner(mw *pruner.Middleware) {
	ph.prunerMw = mw
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

// SetAuthProber sets the auth strategy prober.
func (ph *ProxyHandler) SetAuthProber(ap *AuthProber) {
	ph.authProber = ap
}

// GetCache returns the cache instance for external access.
func (ph *ProxyHandler) GetCache() *CCCache {
	return ph.cache
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
	requestStart := time.Now()

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
		if pruned, err := ph.prunerMw.ProcessRequest(r.Context(), bodyBytes); err == nil {
			bodyBytes = pruned
		}
	}

	// Parse body ONCE — extract model and streaming flag with gjson (zero-alloc)
	pr := &parsedRequest{body: bodyBytes}
	if gjson.ValidBytes(bodyBytes) {
		pr.model = gjson.GetBytes(bodyBytes, "model").Str
		pr.streaming = gjson.GetBytes(bodyBytes, "stream").Bool()
	}

	// Model routing: evaluate rule engine to potentially swap to a cheaper model
	pr.originalModel = pr.model
	ph.applyModelRouting(r, pr)

	// Compute canonical cache key from post-routing body (pr.body may differ from bodyBytes after model swap)
	if ph.cache != nil {
		pr.cacheKey = ph.cache.GenerateCanonicalKey(pr.body)
	}

	// Try cache lookup — uses canonical key for semantic dedup
	if ph.cache != nil && pr.cacheKey != "" {
		if entry, ok := ph.cache.Get(pr.cacheKey); ok && entry != nil {
			latencyMs := time.Since(requestStart).Milliseconds()
			ph.cache.RecordLatencySaved(latencyMs)
			w.Header().Set("X-Cache", "HIT")
			w.Header().Set("X-Cache-Key", pr.cacheKey[:16])
			if pr.streaming {
				ph.writeSSEFromCache(w, entry)
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(entry.StatusCode)
				w.Write(entry.Body)
			}
			return
		}

		// Singleflight: deduplicate concurrent identical non-streaming requests
		if !pr.streaming {
			if sf := ph.cache.GetSingleflight(); sf != nil {
				sfEntry, sfErr := sf.Do(r.Context(), pr.cacheKey, 30*time.Second, func() (*CCCacheEntry, error) {
					return ph.forwardAndCache(r, pr)
				})
				if sfErr != nil {
					http.Error(w, SanitizeError(sfErr), http.StatusBadGateway)
					return
				}
				if sfEntry != nil {
					w.Header().Set("X-Cache", "MISS")
					w.Header().Set("X-Cache-Key", pr.cacheKey[:16])
					w.Header().Set("Content-Type", "application/json")
					for k, v := range sfEntry.Headers {
						if k != "Content-Type" {
							w.Header().Set(k, v)
						}
					}
					w.WriteHeader(sfEntry.StatusCode)
					w.Write(sfEntry.Body)
					return
				}
			}
		}
	}

	// Fallback: direct forward with auth probing + provider failover
	if ph.providerPool == nil || ph.providerPool.Router == nil {
		http.Error(w, "no provider pool configured", http.StatusServiceUnavailable)
		return
	}
	ph.setRouteHeaders(w, pr)
	routingMode := ph.extractRoutingMode(r)
	slog.Debug("[proxy] routing request", "model", pr.model, "streaming", pr.streaming, "mode", routingMode)

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

		if format == providerpool.APIFormatAnthropic {
			pr.upstreamFormat = ProviderTypeAnthropic
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

	ph.copyResponseWithCache(w, finalResp, pr)
}

// forwardAndCache routes, forwards, and caches a request. Used by singleflight.
func (ph *ProxyHandler) forwardAndCache(r *http.Request, pr *parsedRequest) (*CCCacheEntry, error) {
	routeReq := &providerpool.RouteRequest{
		ModelID: pr.model,
		Mode:    providerpool.RoutingMode(ph.extractRoutingMode(r)),
	}

	var finalEntry *CCCacheEntry
	err := ph.providerPool.Router.RouteWithFallback(r.Context(), routeReq, func(result *providerpool.RouteResult) error {
		pid := result.Provider.ID
		burl := result.Provider.BaseURL

		// Throttle check
		if ph.providerMemory.IsThrottled(pid, burl) {
			return fmt.Errorf("provider %s is throttled", pid)
		}

		// Try all Format × Model combinations on this provider
		resp, effectiveFormat, _, tryErr := ph.tryOnProvider(r, result, pr)
		if tryErr != nil {
			return tryErr
		}
		defer resp.Body.Close()

		// Guard: if upstream unexpectedly returns SSE, don't buffer/cache it
		if isStreamingResponse(resp) {
			return fmt.Errorf("upstream returned streaming response for non-streaming request")
		}

		respBody, err := readBody(resp.Body)
		if err != nil {
			return err
		}

		// Convert Anthropic response to OpenAI format
		if effectiveFormat == providerpool.APIFormatAnthropic && resp.StatusCode == http.StatusOK {
			fc := NewFormatConverter()
			if converted, convErr := fc.ConvertResponse(respBody, ProviderTypeAnthropic); convErr == nil {
				respBody = converted
			}
		}

		if ph.cache != nil && resp.StatusCode == http.StatusOK {
			ph.cache.Set(pr.cacheKey, respBody, resp.StatusCode, resp.Header, result.Provider.ID, pr.model)
		}

		headerMap := make(map[string]string)
		for k, v := range resp.Header {
			if len(v) > 0 {
				headerMap[k] = v[0]
			}
		}
		finalEntry = &CCCacheEntry{
			Body:       respBody,
			StatusCode: resp.StatusCode,
			Headers:    headerMap,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return finalEntry, nil
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

	targetURL, err := url.Parse(provider.BaseURL)
	if err != nil {
		return nil, err
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

	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL.String(), io.NopCloser(bytes.NewReader(body)))
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
func (ph *ProxyHandler) allFormatsForProvider(pid, burl string, provider *providerpool.Provider) []providerpool.APIFormat {
	// Use stack-allocated array — at most 4 formats (detected, remembered, default, remaining)
	var buf [4]providerpool.APIFormat
	n := 0

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
	}

	// 2. In-memory remembered format (from recent successful requests)
	if remembered, ok := ph.providerMemory.RecallFormat(pid, burl); ok {
		f := providerpool.APIFormat(remembered)
		if !has(f) {
			buf[n] = f
			n++
		}
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

	result := make([]providerpool.APIFormat, n)
	copy(result, buf[:n])
	return result
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
func (ph *ProxyHandler) allModelsForProvider(pid, burl, originalModel string, routedModel string) []string {
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

	result := make([]string, n)
	copy(result, buf[:n])
	return result
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

	formats := ph.allFormatsForProvider(pid, burl, result.Provider)
	models := ph.allModelsForProvider(pid, burl, pr.model, routedModel)

	if len(models) == 0 {
		return nil, "", "", fmt.Errorf("all models blacklisted on provider %s", pid)
	}

	// If provider has a known format (detected or remembered), only use that one
	if result.Provider.DetectedFormat != "" {
		formats = []providerpool.APIFormat{result.Provider.DetectedFormat}
	} else if remembered, ok := ph.providerMemory.RecallFormat(pid, burl); ok {
		formats = []providerpool.APIFormat{providerpool.APIFormat(remembered)}
	}

	// Fast path: single model + single format + model matches request (most common happy path).
	// Avoids loop overhead, sjson.SetBytes, and slice iteration.
	if len(models) == 1 && len(formats) == 1 && models[0] == pr.model {
		format := formats[0]
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
		// Fall through to error handling in the general loop
	}

	var lastErr error
	for _, model := range models {
		// Build body with this model
		forwardBody := pr.body
		if model != pr.model {
			if newBody, err := sjson.SetBytes(pr.body, "model", model); err == nil {
				forwardBody = newBody
			}
		}

		for _, format := range formats {
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

			// 4xx: blacklist this model on this provider, try next model/format
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

	targetURL, err := url.Parse(provider.BaseURL)
	if err != nil {
		return nil, err
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
	}

	req.Host = targetURL.Host

	if provider.SkipTLSVerify {
		return ph.connPool.GetInsecureClient(provider.Name).Do(req)
	}
	return ph.connPool.GetClient(provider.Name).Do(req)
}

// copyResponse copies the response to the client
func (ph *ProxyHandler) copyResponse(w http.ResponseWriter, resp *http.Response) {
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	if isStreamingResponse(resp) {
		ph.copyStreamingResponseWithCapture(w, resp, false)
	} else {
		io.Copy(w, resp.Body)
	}
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

// copyResponseWithCache copies response and caches if applicable.
// Uses pre-parsed request data to avoid redundant JSON parsing.
func (ph *ProxyHandler) copyResponseWithCache(w http.ResponseWriter, resp *http.Response, pr *parsedRequest) {
	copyHeaders(w.Header(), resp.Header)

	if isStreamingResponse(resp) || pr.streaming {
		w.Header().Set("X-Cache", "MISS")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(resp.StatusCode)

		// If upstream returned Anthropic SSE, convert to OpenAI SSE
		if pr.upstreamFormat == ProviderTypeAnthropic {
			if err := sharedConverter.ConvertStreamingResponse(resp.Body, ProviderTypeAnthropic, w); err != nil {
				slog.Warn("[proxy] anthropic stream conversion error", "error", err)
			}
			return
		}

		// Stream to client while capturing for cache
		needCapture := ph.cache != nil && resp.StatusCode == http.StatusOK
		captured := ph.copyStreamingResponseWithCapture(w, resp, needCapture)
		// Cache the assembled non-streaming response
		if needCapture && len(captured) > 0 {
			assembled := assembleNonStreamingResponse(parseSSEChunks(captured))
			if len(assembled) > 0 {
				// Reuse pre-computed cacheKey and model — no re-parsing needed
				ph.cache.Set(pr.cacheKey, assembled, resp.StatusCode, resp.Header, "", pr.model)
			}
		}
		return
	}

	respBody, err := readBody(resp.Body)
	if err != nil {
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(resp.StatusCode)
		return
	}

	// Convert non-streaming Anthropic response to OpenAI format
	if pr.upstreamFormat == ProviderTypeAnthropic && resp.StatusCode == http.StatusOK {
		if converted, convErr := sharedConverter.ConvertResponse(respBody, ProviderTypeAnthropic); convErr == nil {
			respBody = converted
		}
	}

	if ph.cache != nil && resp.StatusCode == http.StatusOK {
		// Reuse pre-computed cacheKey and model — no re-parsing needed
		ph.cache.Set(pr.cacheKey, respBody, resp.StatusCode, resp.Header, "", pr.model)
		w.Header().Set("X-Cache", "MISS")
		w.Header().Set("X-Cache-Key", pr.cacheKey[:16])
	} else {
		w.Header().Set("X-Cache", "BYPASS")
		if ph.cache != nil {
			ph.cache.RecordBypass()
		}
	}

	// Track routing cost savings (non-streaming only — streaming has no full body here)
	if resp.StatusCode == http.StatusOK && pr.originalModel != pr.model {
		ph.routingStats.Record(pr.originalModel, pr.model, respBody)
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

// writeSSEFromCache converts a cached non-streaming response to SSE and writes it.
func (ph *ProxyHandler) writeSSEFromCache(w http.ResponseWriter, entry *CCCacheEntry) {
	sseData := convertToSSE(entry.Body)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(entry.StatusCode)
	w.Write(sseData)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// copyStreamingResponseWithCapture streams SSE to client while capturing raw data.
// Returns the captured SSE bytes for cache assembly.
// When needCapture is false, streams directly without buffering (zero-copy path).
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

	// Fast path: no capture needed (cache disabled or non-200)
	if !needCapture {
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				flusher.Flush()
			}
			if err != nil {
				break
			}
		}
		return nil
	}

	var capture bytes.Buffer
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			flusher.Flush()
			capture.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return capture.Bytes()
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

// copyHeaders copies headers from src to dst
func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		if hopByHopHeaders[key] {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
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
			pr.routed = decision
			pr.model = decision.Model
			pr.body = replaceModelInBody(pr.body, decision.Model)
			return
		}
	}

	// 2. Model router: family-based routing + background task downgrade
	if ph.modelRouter != nil {
		isBackground := ph.modelRouter.IsBackgroundRequest(r)
		route, err := ph.modelRouter.RouteModel(pr.model, isBackground)
		if err == nil && route.TargetModel != pr.model {
			pr.model = route.TargetModel
			pr.body = replaceModelInBody(pr.body, route.TargetModel)
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
