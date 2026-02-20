package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

// Realistic request body for benchmarks
var benchBody = []byte(`{"model":"gpt-4","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hello, how are you today?"}],"temperature":0.7,"top_p":1.0,"max_tokens":1024,"stream":false}`)

var benchBodyLarge = []byte(`{"model":"claude-3-opus","messages":[` +
	`{"role":"system","content":"You are a helpful coding assistant. Always provide clear explanations."},` +
	`{"role":"user","content":"` + strings.Repeat("Write a function that ", 50) + `"}` +
	`],"temperature":0.5,"top_p":0.9,"max_tokens":4096,"stream":true}`)

// BenchmarkCanonicalKey_Gjson benchmarks the optimized gjson-based CanonicalKey
func BenchmarkCanonicalKey_Gjson(b *testing.B) {
	c := NewCanonicalizer()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.CanonicalKey(benchBody)
	}
}

// BenchmarkCanonicalKey_Gjson_Large benchmarks with a larger request body
func BenchmarkCanonicalKey_Gjson_Large(b *testing.B) {
	c := NewCanonicalizer()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.CanonicalKey(benchBodyLarge)
	}
}

// BenchmarkCanonicalKey_Legacy benchmarks the old json.Unmarshal path (via CanonicalKeyFromParsed)
func BenchmarkCanonicalKey_Legacy(b *testing.B) {
	c := NewCanonicalizer()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Simulate old path: unmarshal then call CanonicalKeyFromParsed
		var req map[string]interface{}
		if err := json.Unmarshal(benchBody, &req); err == nil {
			c.CanonicalKeyFromParsed(req)
		}
	}
}

// BenchmarkReadBody benchmarks the pooled body reader
func BenchmarkReadBody(b *testing.B) {
	data := benchBody
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r := strings.NewReader(string(data))
		readBody(r)
	}
}

// BenchmarkReadBody_Large benchmarks body reading with larger payloads
func BenchmarkReadBody_Large(b *testing.B) {
	data := benchBodyLarge
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r := strings.NewReader(string(data))
		readBody(r)
	}
}

// BenchmarkIsModelNotConfiguredError benchmarks error pattern matching
func BenchmarkIsModelNotConfiguredError(b *testing.B) {
	body := []byte(`{"error":{"message":"The model gpt-4 is not available on this endpoint","type":"invalid_request_error"}}`)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		isModelNotConfiguredError(404, body)
	}
}

// BenchmarkIsModelNotConfiguredError_NoMatch benchmarks when no pattern matches
func BenchmarkIsModelNotConfiguredError_NoMatch(b *testing.B) {
	body := []byte(`{"error":{"message":"Rate limit exceeded","type":"rate_limit_error"}}`)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		isModelNotConfiguredError(400, body)
	}
}

// BenchmarkFormatConverter_Singleton benchmarks the shared converter
func BenchmarkFormatConverter_Singleton(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sharedConverter.ConvertRequest(benchBody, ProviderTypeAnthropic)
	}
}

// BenchmarkFormatConverter_NewPerRequest benchmarks allocating a new converter per request (old behavior)
func BenchmarkFormatConverter_NewPerRequest(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		fc := NewFormatConverter()
		fc.ConvertRequest(benchBody, ProviderTypeAnthropic)
	}
}

// BenchmarkBuildUpstreamRequest benchmarks request building
func BenchmarkBuildUpstreamRequest(b *testing.B) {
	ph := &ProxyHandler{}
	provider := &providerpool.Provider{
		ID:      "test",
		BaseURL: "https://api.openai.com",
	}
	route := &providerpool.RouteResult{
		Provider: provider,
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ph.buildUpstreamRequestWithFormat(r, route, benchBody, providerpool.APIFormatOpenAI)
	}
}

// BenchmarkBuildUpstreamRequest_Anthropic benchmarks request building with format conversion
func BenchmarkBuildUpstreamRequest_Anthropic(b *testing.B) {
	ph := &ProxyHandler{}
	provider := &providerpool.Provider{
		ID:      "test",
		BaseURL: "https://api.anthropic.com",
	}
	route := &providerpool.RouteResult{
		Provider: provider,
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ph.buildUpstreamRequestWithFormat(r, route, benchBody, providerpool.APIFormatAnthropic)
	}
}

// BenchmarkAllModelsForProvider benchmarks model candidate list building
func BenchmarkAllModelsForProvider(b *testing.B) {
	ph := &ProxyHandler{
		providerMemory: NewProviderMemory(),
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ph.allModelsForProvider("provider-1", "https://api.openai.com", "gpt-4", "")
	}
}

// BenchmarkAllFormatsForProvider benchmarks format priority list building
func BenchmarkAllFormatsForProvider(b *testing.B) {
	ph := &ProxyHandler{
		providerMemory: NewProviderMemory(),
	}
	provider := &providerpool.Provider{
		ID:        "test",
		APIFormat: providerpool.APIFormatOpenAI,
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ph.allFormatsForProvider("test", "https://api.openai.com", provider)
	}
}

// BenchmarkProviderMemory_IsThrottled benchmarks throttle check
func BenchmarkProviderMemory_IsThrottled(b *testing.B) {
	pm := NewProviderMemory()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pm.IsThrottled("provider-1", "https://api.openai.com")
	}
}

// BenchmarkProviderMemory_IsBlacklisted benchmarks blacklist check
func BenchmarkProviderMemory_IsBlacklisted(b *testing.B) {
	pm := NewProviderMemory()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pm.IsModelBlacklisted("provider-1", "https://api.openai.com", "gpt-4")
	}
}

// BenchmarkAuthProber_Strategies benchmarks auth strategy resolution
func BenchmarkAuthProber_Strategies(b *testing.B) {
	ap := NewAuthProber()
	provider := &providerpool.Provider{
		ID:        "test",
		BaseURL:   "https://api.openai.com",
		APIFormat: providerpool.APIFormatOpenAI,
	}
	apiKey := &providerpool.APIKey{Key: "sk-test-key"}

	// Warm up: remember a strategy
	ap.Remember("test", "https://api.openai.com", AuthBearer)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ap.Strategies(provider, apiKey)
	}
}

// BenchmarkReplaceModelInBody benchmarks model replacement in JSON body
func BenchmarkReplaceModelInBody(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		replaceModelInBody(benchBody, "claude-3-opus")
	}
}

// BenchmarkCopyHeaders benchmarks header copying
func BenchmarkCopyHeaders(b *testing.B) {
	src := http.Header{
		"Content-Type":    {"application/json"},
		"Authorization":   {"Bearer sk-test"},
		"Accept":          {"*/*"},
		"User-Agent":      {"test-client/1.0"},
		"X-Custom-Header": {"value1", "value2"},
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		dst := make(http.Header)
		copyHeaders(dst, src)
	}
}

// BenchmarkExtractRoutingMode benchmarks routing mode extraction from API key
func BenchmarkExtractRoutingMode(b *testing.B) {
	ph := &ProxyHandler{}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.Header.Set("Authorization", "Bearer sk-test-key")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ph.extractRoutingMode(r)
	}
}

// BenchmarkE2E_ParseAndRoute simulates the full pre-network hot path:
// body read → gjson parse → cache key → routing → request build
func BenchmarkE2E_ParseAndRoute(b *testing.B) {
	ph := &ProxyHandler{
		providerMemory: NewProviderMemory(),
		authProber:     NewAuthProber(),
		routingStats:   NewRoutingStats(),
	}
	ph.routingEnabled.Store(true)

	// Set up a minimal cache
	cache := NewCCCache(nil)
	ph.cache = cache

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Simulate ServeHTTP pre-network path
		pr := &parsedRequest{body: benchBody}
		pr.model = "gpt-4"
		pr.streaming = false
		pr.originalModel = pr.model

		// Cache key generation (the optimized path)
		if ph.cache != nil {
			pr.cacheKey = ph.cache.GenerateCanonicalKey(pr.body)
		}

		// Model candidate building
		_ = ph.allModelsForProvider("provider-1", "https://api.openai.com", pr.model, "")

		// Format candidate building
		provider := &providerpool.Provider{
			ID:        "provider-1",
			BaseURL:   "https://api.openai.com",
			APIFormat: providerpool.APIFormatOpenAI,
		}
		_ = ph.allFormatsForProvider("provider-1", "https://api.openai.com", provider)
	}
}

// BenchmarkE2E_ParseAndRoute_Parallel simulates concurrent requests
func BenchmarkE2E_ParseAndRoute_Parallel(b *testing.B) {
	ph := &ProxyHandler{
		providerMemory: NewProviderMemory(),
		authProber:     NewAuthProber(),
		routingStats:   NewRoutingStats(),
	}
	ph.routingEnabled.Store(true)
	cache := NewCCCache(nil)
	ph.cache = cache

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pr := &parsedRequest{body: benchBody}
			pr.model = "gpt-4"
			pr.streaming = false
			pr.originalModel = pr.model

			if ph.cache != nil {
				pr.cacheKey = ph.cache.GenerateCanonicalKey(pr.body)
			}

			_ = ph.allModelsForProvider("provider-1", "https://api.openai.com", pr.model, "")

			provider := &providerpool.Provider{
				ID:        "provider-1",
				BaseURL:   "https://api.openai.com",
				APIFormat: providerpool.APIFormatOpenAI,
			}
			_ = ph.allFormatsForProvider("provider-1", "https://api.openai.com", provider)
		}
	})
}

// BenchmarkTryOnProvider_SingleModel benchmarks tryOnProvider with a single model (happy path)
// Uses b.N capped to avoid port exhaustion on local test servers.
func BenchmarkTryOnProvider_SingleModel(b *testing.B) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"id":"chatcmpl-1","choices":[{"message":{"content":"hi"}}]}`)
	}))
	defer upstream.Close()

	connConfig := DefaultConnectionConfig()
	connConfig.MaxIdleConnsPerHost = 50
	ph := &ProxyHandler{
		providerMemory: NewProviderMemory(),
		authProber:     NewAuthProber(),
		connPool:       NewConnectionPool(connConfig),
	}

	provider := &providerpool.Provider{
		ID:        "bench-provider",
		Name:      "bench",
		BaseURL:   upstream.URL,
		APIFormat: providerpool.APIFormatOpenAI,
	}
	result := &providerpool.RouteResult{
		Provider: provider,
		Model:    &providerpool.Model{ID: "gpt-4"},
		APIKey:   &providerpool.APIKey{Key: "sk-test"},
	}
	pr := &parsedRequest{
		body:  benchBody,
		model: "gpt-4",
	}

	// Warm up auth prober
	ph.authProber.Remember("bench-provider", upstream.URL, AuthBearer)

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		resp, _, _, err := ph.tryOnProvider(r, result, pr)
		if err != nil {
			b.Fatal(err)
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
}
