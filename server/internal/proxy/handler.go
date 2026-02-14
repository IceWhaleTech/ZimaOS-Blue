package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

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
	apiKeyValidator func(key string) ([]string, error)
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(router *Router, connPool *ConnectionPool, failover *FailoverHandler) *ProxyHandler {
	return &ProxyHandler{
		router:   router,
		connPool: connPool,
		failover: failover,
	}
}

// SetProviderPool sets the Provider Pool for routing and API key lookup.
func (ph *ProxyHandler) SetProviderPool(pool *providerpool.Pool) {
	ph.providerPool = pool
}

// SetCache sets the response cache for the proxy handler.
func (ph *ProxyHandler) SetCache(cache *CCCache) {
	ph.cache = cache
}

// SetAPIKeyValidator sets the API key validator function.
func (ph *ProxyHandler) SetAPIKeyValidator(validator func(key string) ([]string, error)) {
	ph.apiKeyValidator = validator
}

// GetCache returns the cache instance for external access.
func (ph *ProxyHandler) GetCache() *CCCache {
	return ph.cache
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
	fmt.Printf("[Proxy] ServeHTTP: method=%s, path=%s\n", r.Method, r.URL.Path)
	requestStart := time.Now()

	// Handle /v1/models specially
	if r.URL.Path == "/v1/models" || strings.HasSuffix(r.URL.Path, "/models") {
		ph.handleModels(w, r)
		return
	}

	// Read request body
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body.Close()

	// Extract model and streaming flag
	var model string
	var isStreaming bool
	var reqBody map[string]interface{}

	if json.Unmarshal(bodyBytes, &reqBody) == nil {
		if m, ok := reqBody["model"].(string); ok {
			model = m
		}
		if s, ok := reqBody["stream"].(bool); ok {
			isStreaming = s
		}
	}
	fmt.Printf("[Proxy] request: model=%s, streaming=%v\n", model, isStreaming)

	// Try cache lookup (non-streaming only) — uses canonical key for semantic dedup
	if ph.cache != nil && !isStreaming {
		cacheKey := ph.cache.GenerateCanonicalKey(bodyBytes)
		if entry, ok := ph.cache.Get(cacheKey); ok && entry != nil {
			latencyMs := time.Since(requestStart).Milliseconds()
			ph.cache.RecordLatencySaved(latencyMs)
			fmt.Printf("[Proxy] cache HIT (key=%s, saved=%dms)\n", cacheKey[:16], latencyMs)
			w.Header().Set("X-Cache", "HIT")
			w.Header().Set("X-Cache-Key", cacheKey[:16])
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(entry.StatusCode)
			w.Write(entry.Body)
			return
		}

		// Singleflight: deduplicate concurrent identical requests
		if sf := ph.cache.GetSingleflight(); sf != nil {
			sfEntry, sfErr := sf.Do(r.Context(), cacheKey, 30*time.Second, func() (*CCCacheEntry, error) {
				return ph.forwardAndCache(r, bodyBytes, reqBody, model, cacheKey)
			})
			if sfErr != nil {
				http.Error(w, sfErr.Error(), http.StatusBadGateway)
				return
			}
			if sfEntry != nil {
				w.Header().Set("X-Cache", "MISS")
				w.Header().Set("X-Cache-Key", cacheKey[:16])
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

	// Fallback: direct forward (streaming or no cache)
	route, err := ph.routeRequestWithMode(model, ph.extractRoutingMode(r))
	if err != nil {
		fmt.Printf("[Proxy] routing error: %v\n", err)
		http.Error(w, "No available provider: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	fmt.Printf("[Proxy] routed to provider=%s, model=%v\n", route.Provider.ID, route.Model)

	// Map model name if needed
	forwardBody := bodyBytes
	if route.Model != nil && model != route.Model.ID {
		reqBody["model"] = route.Model.ID
		if newBody, err := json.Marshal(reqBody); err == nil {
			forwardBody = newBody
		}
	}

	// Forward request
	r.Body = io.NopCloser(strings.NewReader(string(forwardBody)))
	resp, err := ph.forwardToProvider(r, route)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	ph.copyResponseWithCache(w, resp, r, bodyBytes, isStreaming)
}

// forwardAndCache routes, forwards, and caches a request. Used by singleflight.
func (ph *ProxyHandler) forwardAndCache(r *http.Request, bodyBytes []byte, reqBody map[string]interface{}, model, cacheKey string) (*CCCacheEntry, error) {
	route, err := ph.routeRequestWithMode(model, ph.extractRoutingMode(r))
	if err != nil {
		return nil, err
	}

	forwardBody := bodyBytes
	if route.Model != nil && model != route.Model.ID {
		reqBody["model"] = route.Model.ID
		if newBody, err := json.Marshal(reqBody); err == nil {
			forwardBody = newBody
		}
	}

	r.Body = io.NopCloser(strings.NewReader(string(forwardBody)))
	resp, err := ph.forwardToProvider(r, route)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Guard: if upstream unexpectedly returns SSE, don't buffer/cache it
	if isStreamingResponse(resp) {
		return nil, fmt.Errorf("upstream returned streaming response for non-streaming request")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Cache the response
	if ph.cache != nil && resp.StatusCode == http.StatusOK {
		ph.cache.Set(cacheKey, respBody, resp.StatusCode, resp.Header, route.Provider.ID, model)
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

	return ph.connPool.GetClient(provider.Name).Do(req)
}

// copyResponse copies the response to the client
func (ph *ProxyHandler) copyResponse(w http.ResponseWriter, resp *http.Response) {
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	if isStreamingResponse(resp) {
		ph.copyStreamingResponse(w, resp)
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

// copyResponseWithCache copies response and caches if applicable
func (ph *ProxyHandler) copyResponseWithCache(w http.ResponseWriter, resp *http.Response, r *http.Request, reqBody []byte, isStreaming bool) {
	copyHeaders(w.Header(), resp.Header)

	if isStreamingResponse(resp) || isStreaming {
		w.Header().Set("X-Cache", "BYPASS")
		if ph.cache != nil {
			ph.cache.RecordBypass()
		}
		w.WriteHeader(resp.StatusCode)
		ph.copyStreamingResponse(w, resp)
		return
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(resp.StatusCode)
		return
	}

	if ph.cache != nil && resp.StatusCode == http.StatusOK {
		cacheKey := ph.cache.GenerateCanonicalKey(reqBody)
		var model string
		var reqMap map[string]interface{}
		if json.Unmarshal(reqBody, &reqMap) == nil {
			if m, ok := reqMap["model"].(string); ok {
				model = m
			}
		}
		ph.cache.Set(cacheKey, respBody, resp.StatusCode, resp.Header, "", model)
		w.Header().Set("X-Cache", "MISS")
		w.Header().Set("X-Cache-Key", cacheKey[:16])
	} else {
		w.Header().Set("X-Cache", "BYPASS")
		if ph.cache != nil {
			ph.cache.RecordBypass()
		}
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

// copyStreamingResponse handles SSE streaming
func (ph *ProxyHandler) copyStreamingResponse(w http.ResponseWriter, resp *http.Response) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, resp.Body)
		return
	}

	buf := make([]byte, 8192) // Larger buffer for better throughput
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
