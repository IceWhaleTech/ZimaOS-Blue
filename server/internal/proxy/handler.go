package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	body      []byte
	model     string
	streaming bool
	cacheKey  string // computed once if cache enabled
	routed    *RouteDecision // non-nil if rule engine rerouted the model
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
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(router *Router, connPool *ConnectionPool, failover *FailoverHandler) *ProxyHandler {
	ph := &ProxyHandler{
		router:   router,
		connPool: connPool,
		failover: failover,
	}
	ph.routingEnabled.Store(true)
	return ph
}

// SetProviderPool sets the Provider Pool for routing and API key lookup.
func (ph *ProxyHandler) SetProviderPool(pool *providerpool.Pool) {
	ph.providerPool = pool
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

// GetCache returns the cache instance for external access.
func (ph *ProxyHandler) GetCache() *CCCache {
	return ph.cache
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

	// Fallback: direct forward (streaming or no cache)
	ph.setRouteHeaders(w, pr)
	route, err := ph.routeRequestWithMode(pr.model, ph.extractRoutingMode(r))
	if err != nil {
		http.Error(w, SanitizeError(err), http.StatusServiceUnavailable)
		return
	}

	// Map model name if needed — sjson for zero-alloc in-place edit
	forwardBody := pr.body
	if route.Model != nil && pr.model != route.Model.ID {
		if newBody, err := sjson.SetBytes(pr.body, "model", route.Model.ID); err == nil {
			forwardBody = newBody
		}
	}

	// Forward request — use bytes.NewReader for zero-copy
	r.Body = io.NopCloser(bytes.NewReader(forwardBody))
	resp, err := ph.forwardToProvider(r, route)
	if err != nil {
		http.Error(w, SanitizeError(err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	ph.copyResponseWithCache(w, resp, pr)
}

// forwardAndCache routes, forwards, and caches a request. Used by singleflight.
func (ph *ProxyHandler) forwardAndCache(r *http.Request, pr *parsedRequest) (*CCCacheEntry, error) {
	route, err := ph.routeRequestWithMode(pr.model, ph.extractRoutingMode(r))
	if err != nil {
		return nil, err
	}

	forwardBody := pr.body
	if route.Model != nil && pr.model != route.Model.ID {
		if newBody, err := sjson.SetBytes(pr.body, "model", route.Model.ID); err == nil {
			forwardBody = newBody
		}
	}

	r.Body = io.NopCloser(bytes.NewReader(forwardBody))
	resp, err := ph.forwardToProvider(r, route)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Guard: if upstream unexpectedly returns SSE, don't buffer/cache it
	if isStreamingResponse(resp) {
		return nil, fmt.Errorf("upstream returned streaming response for non-streaming request")
	}

	respBody, err := readBody(resp.Body)
	if err != nil {
		return nil, err
	}

	// Cache the response
	if ph.cache != nil && resp.StatusCode == http.StatusOK {
		ph.cache.Set(pr.cacheKey, respBody, resp.StatusCode, resp.Header, route.Provider.ID, pr.model)
	}

	// Build entry to return
	headerMap := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headerMap[k] = v[0]
		}
	}

	return &CCCacheEntry{
		Body:       respBody,
		StatusCode: resp.StatusCode,
		Headers:    headerMap,
	}, nil
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

// forwardToProvider forwards the request to upstream provider.
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
	if route.APIKey != nil && route.APIKey.Key != "" {
		if provider.APIFormat == providerpool.APIFormatAnthropic {
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
		ph.copyStreamingResponseWithCapture(w, resp)
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
		w.WriteHeader(resp.StatusCode)
		// Stream to client while capturing for cache
		captured := ph.copyStreamingResponseWithCapture(w, resp)
		// Cache the assembled non-streaming response
		if ph.cache != nil && resp.StatusCode == http.StatusOK && len(captured) > 0 {
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
// Uses pooled 32KB buffers to reduce GC pressure.
func (ph *ProxyHandler) copyStreamingResponseWithCapture(w http.ResponseWriter, resp *http.Response) []byte {
	flusher, ok := w.(http.Flusher)
	if !ok {
		data, _ := readBody(resp.Body)
		w.Write(data)
		return data
	}

	var capture bytes.Buffer
	bufPtr := sseBufferPool.Get().(*[]byte)
	buf := *bufPtr
	defer sseBufferPool.Put(bufPtr)

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
