package proxy

import (
	"encoding/json"
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
type ProxyHandler struct {
	router       *Router                // Legacy router (fallback only)
	connPool     *ConnectionPool        // HTTP connection pool
	failover     *FailoverHandler       // Failover handler
	providerPool *providerpool.Pool     // Provider Pool for routing and API keys
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

// ServeHTTP implements http.Handler.
// Routes requests through Provider Pool to get the best provider and API key.
func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read request body for model extraction
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body.Close()

	// Extract model from request body
	model := ""
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err == nil {
		if m, ok := reqBody["model"].(string); ok {
			model = m
		}
	}

	// Route through Provider Pool to get provider + API key
	route, err := ph.routeRequest(model)
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

	// Copy response to client
	ph.copyResponse(w, resp)
}

// routeRequest uses Provider Pool Router to select provider and get API key.
// Returns RouteResult containing Provider info and API key.
func (ph *ProxyHandler) routeRequest(model string) (*providerpool.RouteResult, error) {
	if ph.providerPool == nil {
		return nil, ErrNoAvailableProvider
	}

	return ph.providerPool.Router.Route(&providerpool.RouteRequest{
		ModelID: model,
	})
}

// forwardToProvider forwards the request to upstream provider using route result.
// The API key is obtained from Provider Pool, not from config file.
func (ph *ProxyHandler) forwardToProvider(r *http.Request, route *providerpool.RouteResult) (*http.Response, error) {
	provider := route.Provider

	// Parse provider endpoint
	targetURL, err := url.Parse(provider.BaseURL)
	if err != nil {
		return nil, err
	}

	// Build upstream URL
	upstreamURL := *targetURL
	upstreamURL.Path = singleJoiningSlash(targetURL.Path, r.URL.Path)
	upstreamURL.RawQuery = r.URL.RawQuery

	// Create new request
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL.String(), r.Body)
	if err != nil {
		return nil, err
	}

	// Copy headers (excluding hop-by-hop headers)
	copyHeaders(req.Header, r.Header)

	// Set API key from Provider Pool (not from config file!)
	// The API key is dynamically managed by Provider Pool
	if route.APIKey != nil && route.APIKey.Key != "" {
		if strings.Contains(provider.BaseURL, "anthropic") {
			req.Header.Set("x-api-key", route.APIKey.Key)
			req.Header.Set("anthropic-version", "2023-06-01")
		} else {
			req.Header.Set("Authorization", "Bearer "+route.APIKey.Key)
		}
	}

	// Set host header
	req.Host = targetURL.Host

	// Send request using connection pool
	client := ph.connPool.GetClient(provider.Name)
	return client.Do(req)
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
