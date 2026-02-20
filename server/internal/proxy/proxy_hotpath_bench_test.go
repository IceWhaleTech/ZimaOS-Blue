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
