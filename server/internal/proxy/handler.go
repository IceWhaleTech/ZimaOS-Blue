package proxy

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ProxyHandler handles incoming proxy requests
type ProxyHandler struct {
	router   *Router
	connPool *ConnectionPool
	failover *FailoverHandler
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(router *Router, connPool *ConnectionPool, failover *FailoverHandler) *ProxyHandler {
	return &ProxyHandler{
		router:   router,
		connPool: connPool,
		failover: failover,
	}
}

// ServeHTTP implements http.Handler
func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Select provider based on route
	provider, err := ph.router.SelectProvider(r)
	if err != nil {
		http.Error(w, "No available provider", http.StatusServiceUnavailable)
		return
	}

	// Forward request with failover
	resp, err := ph.failover.Execute(r.Context(), provider, func(p *Provider) (*http.Response, error) {
		return ph.forwardRequest(r, p)
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response to client
	ph.copyResponse(w, resp)
}

// forwardRequest forwards the request to upstream provider
func (ph *ProxyHandler) forwardRequest(r *http.Request, provider *Provider) (*http.Response, error) {
	// Parse provider endpoint
	targetURL, err := url.Parse(provider.Config.Endpoint)
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

	// Copy headers
	copyHeaders(req.Header, r.Header)

	// Set/override API key
	if provider.Config.APIKey != "" {
		// Determine header based on provider
		if strings.Contains(provider.Config.Endpoint, "anthropic") {
			req.Header.Set("x-api-key", provider.Config.APIKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		} else {
			req.Header.Set("Authorization", "Bearer "+provider.Config.APIKey)
		}
	}

	// Set host header
	req.Host = targetURL.Host

	// Get client and send request
	client := ph.connPool.GetClient(provider.Config.Name)
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
