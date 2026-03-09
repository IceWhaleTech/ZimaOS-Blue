package proxy

import (
	"strings"
	"testing"
)

func TestRequestBodyForLog_DefaultRedacted(t *testing.T) {
	t.Setenv("ZIMA_PROXY_LOG_UPSTREAM_BODY", "")
	ph := &ProxyHandler{}
	body := []byte(`{"messages":[{"role":"user","content":"secret prompt content"}]}`)

	got := ph.requestBodyForLog(body)
	if strings.Contains(got, "secret prompt content") {
		t.Fatalf("expected request body to be redacted by default, got=%q", got)
	}
	if !strings.Contains(got, "redacted request body") || !strings.Contains(got, "bytes=") {
		t.Fatalf("expected redacted marker with byte length, got=%q", got)
	}
}

func TestRequestBodyForLog_OptInRawBody(t *testing.T) {
	t.Setenv("ZIMA_PROXY_LOG_UPSTREAM_BODY", "1")
	ph := &ProxyHandler{}
	body := []byte(`{"messages":[{"role":"user","content":"visible prompt"}]}`)

	got := ph.requestBodyForLog(body)
	if got != string(body) {
		t.Fatalf("expected full body logging when opt-in enabled, got=%q", got)
	}
}

func TestRequestBodyForLog_OptInUsesMasker(t *testing.T) {
	t.Setenv("ZIMA_PROXY_LOG_UPSTREAM_BODY", "1")
	dm := NewDataMasker(&MaskingConfig{
		Enabled: true,
		Rules: []*MaskingRule{
			{
				ID:          "req_secret",
				Name:        "Request Secret",
				Category:    MaskingCredentials,
				Pattern:     `secret`,
				Replacement: "[MASKED]",
				Direction:   MaskingRequest,
				Enabled:     true,
			},
		},
	})
	ph := &ProxyHandler{dataMasker: dm}
	body := []byte(`{"messages":[{"role":"user","content":"secret value"}]}`)

	got := ph.requestBodyForLog(body)
	if strings.Contains(got, "secret value") || strings.Contains(got, `"secret"`) {
		t.Fatalf("expected request body to be masked when opt-in enabled with masker, got=%q", got)
	}
	if !strings.Contains(got, "[MASKED]") {
		t.Fatalf("expected masked marker in logged body, got=%q", got)
	}
}

func TestUpstreamRequestShapeForLog(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want string
	}{
		{name: "chat completions", body: []byte(`{"messages":[{"role":"user","content":"hi"}]}`), want: "chat_completions"},
		{name: "responses", body: []byte(`{"input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`), want: "responses"},
		{name: "hybrid", body: []byte(`{"messages":[{"role":"user","content":"hi"}],"input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`), want: "hybrid"},
		{name: "unknown", body: []byte(`{"model":"gpt-5"}`), want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := upstreamRequestShapeForLog(tt.body); got != tt.want {
				t.Fatalf("shape = %q, want %q", got, tt.want)
			}
		})
	}
}
