package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/providerpool"
)

// ProxyHandler handles incoming proxy requests.
//
// Architecture:
//   Client --[Proxy API Key]--> Proxy --[Provider API Key]--> Upstream (Anthropic/OpenAI)
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
// - route:cloud - Force cloud provider (zimaos-trial)
// - route:local - Force local provider
type ProxyHandler struct {
	router       *Router                // Legacy router (fallback only)
	connPool     *ConnectionPool        // HTTP connection pool
	failover     *FailoverHandler       // Failover handler
	providerPool *providerpool.Pool     // Provider Pool for routing and API keys
	cache        *CCCache               // Response cache (cc-cache)
	apiKeyValidator func(key string) ([]string, error) // API key validator returns scopes
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
// This should be called during server initialization.
func (ph *ProxyHandler) SetProviderPool(pool *providerpool.Pool) {
	ph.providerPool = pool
}

// SetCache sets the response cache for the proxy handler.
// This should be called during server initialization.
func (ph *ProxyHandler) SetCache(cache *CCCache) {
	ph.cache = cache
}

// SetAPIKeyValidator sets the API key validator function.
// The validator returns scopes for a valid key, or error for invalid key.
func (ph *ProxyHandler) SetAPIKeyValidator(validator func(key string) ([]string, error)) {
	ph.apiKeyValidator = validator
}

// GetCache returns the cache instance for external access (e.g., API handlers).
func (ph *ProxyHandler) GetCache() *CCCache {
	return ph.cache
}

// extractRoutingMode extracts routing mode from API key scopes.
// Returns "auto", "cloud", or "local".
func (ph *ProxyHandler) extractRoutingMode(r *http.Request) string {
	// Get API key from header (x-api-key for Anthropic, Authorization for OpenAI)
	apiKey := r.Header.Get("x-api-key")
	if apiKey == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			apiKey = strings.TrimPrefix(auth, "Bearer ")
		}
	}

	if apiKey == "" || ph.apiKeyValidator == nil {
		return "auto" // Default to auto mode
	}

	// Validate key and get scopes
	scopes, err := ph.apiKeyValidator(apiKey)
	if err != nil {
		return "auto"
	}

	// Check for routing scope
	for _, scope := range scopes {
		if scope == "route:cloud" {
			return "cloud"
		}
		if scope == "route:local" {
			return "local"
		}
	}

	return "auto"
}

// ServeHTTP implements http.Handler.
// Routes requests through Provider Pool to get the best provider and API key.
func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle /v1/models specially - return models from Provider Pool
	if r.URL.Path == "/v1/models" || strings.HasSuffix(r.URL.Path, "/models") {
		ph.handleModels(w, r)
		return
	}

	// Read request body for model extraction and cache key
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body.Close()

	// Extract model and check for streaming
	model := ""
	isStreaming := false
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err == nil {
		if m, ok := reqBody["model"].(string); ok {
			model = m
		}
		if stream, ok := reqBody["stream"].(bool); ok {
			isStreaming = stream
		}
	}

	fmt.Printf("[Proxy] ServeHTTP: model=%s, path=%s, providerPool=%v\n", model, r.URL.Path, ph.providerPool != nil)

	// Try cache lookup (only for non-streaming requests)
	if ph.cache != nil && !isStreaming {
		cacheKey := ph.cache.GenerateKey(r, bodyBytes)
		if entry, ok := ph.cache.Get(cacheKey); ok && entry != nil {
			// Cache hit - return cached response
			w.Header().Set("X-Cache", "HIT")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(entry.StatusCode)
			w.Write(entry.Body)
			return
		}
	}

	// Determine routing mode from API key scopes
	routingMode := ph.extractRoutingMode(r)

	// Route through Provider Pool to get provider + API key
	route, err := ph.routeRequestWithMode(model, routingMode)
	if err != nil {
		http.Error(w, "No available provider: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Forward request to selected provider
	r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	resp, err := ph.forwardToProvider(r, route)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response to client (and cache if applicable)
	ph.copyResponseWithCache(w, resp, r, bodyBytes, isStreaming)
}

// routeRequest uses Provider Pool Router to select provider and get API key.
// Returns RouteResult containing Provider info and API key.
func (ph *ProxyHandler) routeRequest(model string) (*providerpool.RouteResult, error) {
	return ph.routeRequestWithMode(model, "auto")
}

// routeRequestWithMode routes request with specific routing mode.
// Modes: "auto" (default), "cloud" (force cloud), "local" (force local)
func (ph *ProxyHandler) routeRequestWithMode(model string, mode string) (*providerpool.RouteResult, error) {
	if ph.providerPool == nil {
		fmt.Printf("[Proxy] routeRequestWithMode: providerPool is nil\n")
		return nil, ErrNoAvailableProvider
	}

	// Build route request with mode
	req := &providerpool.RouteRequest{
		ModelID: model,
		Mode:    providerpool.RoutingMode(mode),
	}

	result, err := ph.providerPool.Router.Route(req)
	if err != nil {
		fmt.Printf("[Proxy] routeRequestWithMode: Route failed: %v\n", err)
	} else {
		fmt.Printf("[Proxy] routeRequestWithMode: Route success, provider=%s\n", result.Provider.ID)
	}
	return result, err
}

// forwardToProvider forwards the request to upstream provider using route result.
// The API key is obtained from Provider Pool, not from config file.
func (ph *ProxyHandler) forwardToProvider(r *http.Request, route *providerpool.RouteResult) (*http.Response, error) {
	provider := route.Provider

	// Parse provider endpoint
	targetURL, err := url.Parse(provider.BaseURL)
	if err != nil {
		fmt.Printf("[Proxy] forwardToProvider: failed to parse BaseURL %s: %v\n", provider.BaseURL, err)
		return nil, err
	}

	// Build upstream URL
	upstreamURL := *targetURL

	// Handle path joining - avoid duplicate path segments
	// e.g., if BaseURL is "https://api.example.com/v1" and request path is "/v1/messages"
	// we should get "https://api.example.com/v1/messages", not "/v1/v1/messages"
	requestPath := r.URL.Path
	if targetURL.Path != "" && targetURL.Path != "/" {
		// Check if request path starts with the same segment as BaseURL path
		basePath := strings.TrimSuffix(targetURL.Path, "/")
		if strings.HasPrefix(requestPath, basePath) {
			// Request path already includes the base path, use it directly
			upstreamURL.Path = requestPath
		} else {
			upstreamURL.Path = singleJoiningSlash(targetURL.Path, requestPath)
		}
	} else {
		upstreamURL.Path = requestPath
	}
	upstreamURL.RawQuery = r.URL.RawQuery

	fmt.Printf("[Proxy] forwardToProvider: upstream URL = %s\n", upstreamURL.String())

	// Create new request
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL.String(), r.Body)
	if err != nil {
		fmt.Printf("[Proxy] forwardToProvider: failed to create request: %v\n", err)
		return nil, err
	}

	// Copy headers (excluding hop-by-hop headers)
	copyHeaders(req.Header, r.Header)

	// Set API key from Provider Pool (not from config file!)
	// The API key is dynamically managed by Provider Pool
	if route.APIKey != nil && route.APIKey.Key != "" {
		// Use APIFormat to determine authentication method
		fmt.Printf("[Proxy] forwardToProvider: APIFormat=%s, setting auth header\n", provider.APIFormat)
		if provider.APIFormat == providerpool.APIFormatAnthropic {
			req.Header.Set("x-api-key", route.APIKey.Key)
			req.Header.Set("anthropic-version", "2023-06-01")
			// Remove Authorization header if present (we use x-api-key for Anthropic)
			req.Header.Del("Authorization")
		} else {
			req.Header.Set("Authorization", "Bearer "+route.APIKey.Key)
			// Remove x-api-key header if present (we use Authorization for OpenAI)
			req.Header.Del("x-api-key")
		}
	} else {
		fmt.Printf("[Proxy] forwardToProvider: WARNING - no API key available!\n")
	}

	// Debug: log key headers
	fmt.Printf("[Proxy] forwardToProvider: Headers - x-api-key=%s, anthropic-version=%s\n",
		maskKey(req.Header.Get("x-api-key")), req.Header.Get("anthropic-version"))

	// Set host header
	req.Host = targetURL.Host

	// Send request using connection pool
	client := ph.connPool.GetClient(provider.Name)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[Proxy] forwardToProvider: request failed: %v\n", err)
		return nil, err
	}
	fmt.Printf("[Proxy] forwardToProvider: response status = %d\n", resp.StatusCode)

	// Debug: if error response, log the body
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("[Proxy] forwardToProvider: error response body = %s\n", string(bodyBytes))
		// Restore body for further processing
		resp.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	}

	return resp, nil
}

// copyResponse copies the response to the client
func (ph *ProxyHandler) copyResponse(w http.ResponseWriter, resp *http.Response) {
	// Copy headers
	copyHeaders(w.Header(), resp.Header)

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Check if this is a streaming response
	if isStreamingResponse(resp) {
		ph.copyStreamingResponse(w, resp)
	} else {
		io.Copy(w, resp.Body)
	}
}

// handleModels handles GET /v1/models - returns models from Provider Pool
func (ph *ProxyHandler) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ph.providerPool == nil {
		http.Error(w, "Provider pool not configured", http.StatusServiceUnavailable)
		return
	}

	// Get all models from enabled providers
	var allModels []map[string]interface{}
	providers := ph.providerPool.Registry.ListEnabled()

	for _, provider := range providers {
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

	// Return OpenAI-compatible response
	response := map[string]interface{}{
		"object": "list",
		"data":   allModels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// copyResponseWithCache copies response to client and caches if applicable
func (ph *ProxyHandler) copyResponseWithCache(w http.ResponseWriter, resp *http.Response, r *http.Request, reqBody []byte, isStreaming bool) {
	// Copy headers
	copyHeaders(w.Header(), resp.Header)

	// Check if this is a streaming response
	if isStreamingResponse(resp) || isStreaming {
		w.Header().Set("X-Cache", "BYPASS")
		w.WriteHeader(resp.StatusCode)
		ph.copyStreamingResponse(w, resp)
		return
	}

	// Read response body for caching
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(resp.StatusCode)
		return
	}

	// Cache the response if cache is enabled
	if ph.cache != nil && resp.StatusCode == http.StatusOK {
		cacheKey := ph.cache.GenerateKey(r, reqBody)
		// Extract model from request for cache metadata
		model := ""
		var reqMap map[string]interface{}
		if json.Unmarshal(reqBody, &reqMap) == nil {
			if m, ok := reqMap["model"].(string); ok {
				model = m
			}
		}
		ph.cache.Set(cacheKey, respBody, resp.StatusCode, resp.Header, "", model)
		w.Header().Set("X-Cache", "MISS")
	} else {
		w.Header().Set("X-Cache", "BYPASS")
	}

	// Write response
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

// copyStreamingResponse handles SSE streaming responses
func (ph *ProxyHandler) copyStreamingResponse(w http.ResponseWriter, resp *http.Response) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, resp.Body)
		return
	}

	buf := make([]byte, 4096)
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

// isStreamingResponse checks if response is a streaming response
func isStreamingResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "application/x-ndjson")
}

// copyHeaders copies headers from src to dst
func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		// Skip hop-by-hop headers
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// isHopByHopHeader checks if header is a hop-by-hop header
func isHopByHopHeader(header string) bool {
	hopByHopHeaders := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Te":                  true,
		"Trailers":            true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}
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

// maskKey masks an API key for logging
func maskKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
