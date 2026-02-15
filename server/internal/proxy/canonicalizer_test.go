package proxy

import (
	"encoding/json"
	"testing"
)

func TestCanonicalizer_CanonicalKey_SameRequest(t *testing.T) {
	c := NewCanonicalizer()

	body := `{"model":"claude-3-opus","messages":[{"role":"user","content":"hello"}],"temperature":0.7,"max_tokens":1024}`
	key1 := c.CanonicalKey([]byte(body))
	key2 := c.CanonicalKey([]byte(body))

	if key1 != key2 {
		t.Errorf("same request should produce same key: %s != %s", key1, key2)
	}
	if len(key1) != 16 { // FNV-1a hex = 16 chars
		t.Errorf("expected 16 char hex key, got %d", len(key1))
	}
}

func TestCanonicalizer_CanonicalKey_DifferentModels(t *testing.T) {
	c := NewCanonicalizer()

	body1 := `{"model":"claude-3-opus","messages":[{"role":"user","content":"hello"}]}`
	body2 := `{"model":"claude-3-sonnet","messages":[{"role":"user","content":"hello"}]}`

	if c.CanonicalKey([]byte(body1)) == c.CanonicalKey([]byte(body2)) {
		t.Error("different models should produce different keys")
	}
}

func TestCanonicalizer_CanonicalKey_TemperatureBucketing(t *testing.T) {
	c := NewCanonicalizer()

	// 0.71 and 0.74 both bucket to 0.7
	body1 := `{"model":"claude-3-opus","messages":[{"role":"user","content":"hi"}],"temperature":0.71}`
	body2 := `{"model":"claude-3-opus","messages":[{"role":"user","content":"hi"}],"temperature":0.74}`

	if c.CanonicalKey([]byte(body1)) != c.CanonicalKey([]byte(body2)) {
		t.Error("temperatures within same bucket should produce same key")
	}

	// 0.71 and 0.79 bucket to 0.7 and 0.8 respectively
	body3 := `{"model":"claude-3-opus","messages":[{"role":"user","content":"hi"}],"temperature":0.79}`
	if c.CanonicalKey([]byte(body1)) == c.CanonicalKey([]byte(body3)) {
		t.Error("temperatures in different buckets should produce different keys")
	}
}

func TestCanonicalizer_SanitizeBillingHeaders(t *testing.T) {
	c := NewCanonicalizer()

	// Request with billing system message
	withBilling := `{"model":"claude-3-opus","messages":[
		{"role":"system","content":"x-anthropic-billing-header: abc123"},
		{"role":"user","content":"hello"}
	]}`

	// Same request without billing message
	withoutBilling := `{"model":"claude-3-opus","messages":[
		{"role":"user","content":"hello"}
	]}`

	if c.CanonicalKey([]byte(withBilling)) != c.CanonicalKey([]byte(withoutBilling)) {
		t.Error("billing header should be stripped, producing same key")
	}
}

func TestCanonicalizer_CleanTrackingTokens(t *testing.T) {
	c := NewCanonicalizer()

	withTracking := `{"model":"claude-3-opus","messages":[
		{"role":"user","content":"explain this code cch=abc123 cc_version=1.0"}
	]}`

	withoutTracking := `{"model":"claude-3-opus","messages":[
		{"role":"user","content":"explain this code"}
	]}`

	if c.CanonicalKey([]byte(withTracking)) != c.CanonicalKey([]byte(withoutTracking)) {
		t.Error("tracking tokens should be stripped, producing same key")
	}
}

func TestCanonicalizer_InvalidJSON(t *testing.T) {
	c := NewCanonicalizer()

	key := c.CanonicalKey([]byte("not json"))
	if len(key) != 16 {
		t.Error("invalid JSON should still produce a valid FNV-1a key")
	}
}

func TestIsBillingHeader(t *testing.T) {
	tests := []struct {
		content  string
		expected bool
	}{
		{"x-anthropic-billing-header: token123", true},
		{"X-Anthropic-Billing-Header: TOKEN", true},
		{"x-anthropic-billing", true},
		{"some text with cch=abc", true},
		{"normal user message", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isBillingHeader(tt.content); got != tt.expected {
			t.Errorf("isBillingHeader(%q) = %v, want %v", tt.content, got, tt.expected)
		}
	}
}

func TestCleanTrackingTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello cch=abc123 world", "hello world"},
		{"cc_version=1.0 hello", "hello"},
		{"hello x-cc-session=sess123", "hello"},
		{"no tokens here", "no tokens here"},
		{"cch=a,cc_version=b;x-cc-session=c", ""},
	}

	for _, tt := range tests {
		if got := cleanTrackingTokens(tt.input); got != tt.expected {
			t.Errorf("cleanTrackingTokens(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestBucketFloat(t *testing.T) {
	tests := []struct {
		v, precision, expected float64
	}{
		{0.73, 0.1, 0.7},
		{0.75, 0.1, 0.8},
		{0.0, 0.1, 0.0},
		{1.0, 0.1, 1.0},
		{0.5, 0.0, 0.5}, // precision 0 returns as-is
	}

	for _, tt := range tests {
		got := bucketFloat(tt.v, tt.precision)
		if !almostEqual(got, tt.expected) {
			t.Errorf("bucketFloat(%v, %v) = %v, want %v", tt.v, tt.precision, got, tt.expected)
		}
	}
}

func almostEqual(a, b float64) bool {
	return (a-b) < 0.001 && (b-a) < 0.001
}

func TestGetFloat(t *testing.T) {
	m := map[string]interface{}{"temp": 0.7, "name": "test"}
	if got := getFloat(m, "temp"); got != 0.7 {
		t.Errorf("getFloat(temp) = %v, want 0.7", got)
	}
	if got := getFloat(m, "missing"); got != 0 {
		t.Errorf("getFloat(missing) = %v, want 0", got)
	}
}

func TestGetInt(t *testing.T) {
	m := map[string]interface{}{"max": float64(1024)}
	if got := getInt(m, "max"); got != 1024 {
		t.Errorf("getInt(max) = %v, want 1024", got)
	}
}

// Verify JSON round-trip doesn't affect key stability
func TestCanonicalizer_JSONStability(t *testing.T) {
	c := NewCanonicalizer()

	// Two JSON representations of the same data (different key ordering)
	body1 := `{"model":"claude-3-opus","temperature":0.7,"messages":[{"role":"user","content":"hi"}],"max_tokens":100}`
	body2 := `{"messages":[{"role":"user","content":"hi"}],"model":"claude-3-opus","max_tokens":100,"temperature":0.7}`

	// These should produce the same key since we extract fields individually
	key1 := c.CanonicalKey([]byte(body1))
	key2 := c.CanonicalKey([]byte(body2))

	if key1 != key2 {
		t.Error("different JSON key ordering should produce same canonical key")
	}
}

// Ensure the key is a valid hex string
func TestCanonicalizer_KeyFormat(t *testing.T) {
	c := NewCanonicalizer()
	body := `{"model":"test","messages":[]}`
	key := c.CanonicalKey([]byte(body))

	// Verify it's valid hex
	for _, ch := range key {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("key contains non-hex character: %c", ch)
		}
	}
}

// Benchmark canonical key generation
func BenchmarkCanonicalKey(b *testing.B) {
	c := NewCanonicalizer()
	body := []byte(`{"model":"claude-3-opus","messages":[{"role":"user","content":"explain this function"},{"role":"assistant","content":"sure"}],"temperature":0.7,"max_tokens":4096}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.CanonicalKey(body)
	}
}

// Ensure messages field is not required
func TestCanonicalizer_NoMessages(t *testing.T) {
	c := NewCanonicalizer()
	body := `{"model":"claude-3-opus","temperature":0.5}`
	key := c.CanonicalKey([]byte(body))
	if len(key) != 16 {
		t.Error("request without messages should still produce valid key")
	}
}

// Verify sanitizer doesn't modify non-billing system messages
func TestCanonicalizer_PreservesNormalSystemMessages(t *testing.T) {
	c := NewCanonicalizer()

	body1 := `{"model":"claude-3-opus","messages":[
		{"role":"system","content":"You are a helpful assistant"},
		{"role":"user","content":"hello"}
	]}`

	body2 := `{"model":"claude-3-opus","messages":[
		{"role":"user","content":"hello"}
	]}`

	if c.CanonicalKey([]byte(body1)) == c.CanonicalKey([]byte(body2)) {
		t.Error("normal system messages should NOT be stripped")
	}
}

// Verify unused import is not present
var _ = json.Marshal
