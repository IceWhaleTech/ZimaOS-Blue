package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func TestTransientUpstreamRetryDelay(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: 500 * time.Millisecond},
		{attempt: 1, want: 1500 * time.Millisecond},
		{attempt: 2, want: 3 * time.Second},
	}

	for _, tc := range tests {
		if got := transientUpstreamRetryDelay(tc.attempt); got != tc.want {
			t.Fatalf("transientUpstreamRetryDelay(%d) = %s, want %s", tc.attempt, got, tc.want)
		}
	}
}

func TestShouldRetryTransientUpstream5xx(t *testing.T) {
	tests := []struct {
		name          string
		single        bool
		routingSingle bool
		status        int
		body          string
		attempt       int
		want          bool
	}{
		{
			name:          "non single-provider never retries",
			single:        false,
			routingSingle: false,
			status:        429,
			body:          `{"error":{"type":"overloaded_error"}}`,
			attempt:       0,
			want:          false,
		},
		{
			name:          "429 retries until 5 total attempts when routing is truly single-provider",
			single:        true,
			routingSingle: true,
			status:        429,
			body:          `{"error":"rate limit exceeded"}`,
			attempt:       3,
			want:          true,
		},
		{
			name:          "429 stops after 5 total attempts when routing is truly single-provider",
			single:        true,
			routingSingle: true,
			status:        429,
			body:          `{"error":"rate limit exceeded"}`,
			attempt:       4,
			want:          false,
		},
		{
			name:          "rate-limit-like 500 fails fast when outer fallback is available",
			single:        true,
			routingSingle: false,
			status:        500,
			body:          `{"error":{"type":"overloaded_error","message":"构建请求失败"}}`,
			attempt:       0,
			want:          false,
		},
		{
			name:          "529 never retries",
			single:        true,
			routingSingle: true,
			status:        529,
			body:          `{"error":{"type":"overloaded_error","message":"Overloaded"}}`,
			attempt:       0,
			want:          false,
		},
		{
			name:          "wrapped 500 overloaded still retries when routing is truly single-provider",
			single:        true,
			routingSingle: true,
			status:        500,
			body:          `{"error":{"type":"overloaded_error","message":"构建请求失败"}}`,
			attempt:       2,
			want:          true,
		},
		{
			name:          "plain overloaded 500 still retries when routing is truly single-provider",
			single:        true,
			routingSingle: true,
			status:        500,
			body:          `{"error":{"type":"overloaded_error","message":"Overloaded"}}`,
			attempt:       2,
			want:          true,
		},
		{
			name:          "generic 500 does not retry",
			single:        true,
			routingSingle: true,
			status:        500,
			body:          `{"error":"internal error"}`,
			attempt:       0,
			want:          false,
		},
		{
			name:          "transient 503 retries with short policy",
			single:        true,
			routingSingle: false,
			status:        503,
			body:          `{"error":"service unavailable"}`,
			attempt:       1,
			want:          true,
		},
		{
			name:          "transient 503 stops after short policy limit",
			single:        true,
			routingSingle: false,
			status:        503,
			body:          `{"error":"service unavailable"}`,
			attempt:       2,
			want:          false,
		},
	}

	for _, tc := range tests {
		if got := shouldRetryTransientUpstream5xx(tc.single, tc.routingSingle, tc.status, []byte(tc.body), tc.attempt); got != tc.want {
			t.Fatalf("%s: shouldRetryTransientUpstream5xx(%v, %v, %d, %q, %d) = %v, want %v",
				tc.name, tc.single, tc.routingSingle, tc.status, tc.body, tc.attempt, got, tc.want)
		}
	}
}

func TestIsContextCanceledError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context canceled", err: context.Canceled, want: true},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "wrapped canceled", err: errors.New("Post \"https://example.com\": context canceled"), want: true},
		{name: "wrapped deadline", err: errors.New("upstream timeout: deadline exceeded"), want: true},
		{name: "other error", err: errors.New("connection refused"), want: false},
	}

	for _, tc := range tests {
		if got := isContextCanceledError(tc.err); got != tc.want {
			t.Fatalf("%s: isContextCanceledError(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}

func TestIsResponsesContinuationRejectedError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{name: "no body", status: 500, body: "", want: false},
		{name: "non continuation", status: 500, body: `{"error":"backend overloaded"}`, want: false},
		{name: "continuation 400", status: 400, body: `{"error":"previous_response_id not found"}`, want: true},
		{name: "continuation 404", status: 404, body: `{"error":"response id does not exist"}`, want: true},
		{name: "continuation session expired", status: 400, body: `{"error":"continuation session expired"}`, want: true},
		{name: "continuation wrapped 5xx", status: 502, body: `{"error":"upstream error for previous response"}`, want: true},
	}

	for _, tc := range tests {
		if got := isResponsesContinuationRejectedError(tc.status, []byte(tc.body)); got != tc.want {
			t.Fatalf("%s: isResponsesContinuationRejectedError(%d, %q) = %v, want %v", tc.name, tc.status, tc.body, got, tc.want)
		}
	}
}

func TestProxyFailureStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "no provider",
			err:  providerpool.ErrNoAvailableProvider,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "wrapped overloaded build failure 500 maps to 502",
			err:  errors.New(`upstream 500: {"error":{"type":"overloaded_error","message":"构建请求失败"},"type":"error"}`),
			want: http.StatusBadGateway,
		},
		{
			name: "wrapped rate limit 500 maps to 429",
			err:  errors.New(`upstream 500: {"error":{"type":"rate_limit_error","message":"Rate limit exceeded"},"type":"error"}`),
			want: http.StatusTooManyRequests,
		},
		{
			name: "wrapped upstream 401 preserved",
			err:  errors.New(`upstream 401: {"error":{"message":"Invalid API key"}}`),
			want: http.StatusUnauthorized,
		},
		{
			name: "wrapped upstream 404 preserved",
			err:  errors.New(`upstream 404: {"error":{"message":"Model not found"}}`),
			want: http.StatusNotFound,
		},
		{
			name: "relay wrapped context window full maps to 400",
			err:  errors.New(`upstream 502: {"error":{"message":"Context window is full. Reduce conversation history, system prompt, or tools."}}`),
			want: http.StatusBadRequest,
		},
		{
			name: "generic 500 stays 502",
			err:  errors.New(`upstream 500: {"error":{"message":"internal error"}}`),
			want: http.StatusBadGateway,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := proxyFailureStatusCode(tc.err); got != tc.want {
				t.Fatalf("proxyFailureStatusCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestShouldRetryWithoutTools(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "no response", err: errors.New("provider prov_x returned no response"), want: false},
		{name: "upstream 502", err: errors.New("upstream 502: bad gateway"), want: false},
		{name: "provider 502", err: errors.New("provider returned 502: bad gateway"), want: false},
		{name: "opaque relay 502", err: errors.New("upstream 502: {\"error\":{\"message\":\"Upstream request failed\",\"type\":\"upstream_error\"}}"), want: true},
		{name: "explicit no tool support", err: errors.New("provider prov_x does not support tool calls"), want: true},
		{name: "tool field rejected 400", err: errors.New("provider returned 400: unknown field tool_choice"), want: true},
		{name: "function calling rejected 422", err: errors.New("provider returned 422: function calling not supported"), want: true},
		{name: "generic 400 without tool signal", err: errors.New("upstream 400: prompt too long"), want: false},
		{name: "generic 422 without tool signal", err: errors.New("provider returned 422: invalid parameter"), want: false},
	}

	for _, tc := range tests {
		if got := shouldRetryWithoutTools(tc.err); got != tc.want {
			t.Fatalf("%s: shouldRetryWithoutTools(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}

func TestShouldRetryWithoutPreviousResponseID(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "no response", err: errors.New("provider prov_x returned no response"), want: true},
		{name: "empty stream", err: errors.New("provider prov_x returned empty streaming response"), want: true},
		{name: "upstream 502", err: errors.New("upstream 502: bad gateway"), want: true},
		{name: "continuation session expired 400", err: errors.New("provider returned 400: continuation session expired"), want: true},
		{name: "continuation conversation mismatch 409", err: errors.New("provider returned 409: conversation mismatch for this thread"), want: true},
		{name: "invalid session 410", err: errors.New("provider returned 410: session does not exist"), want: true},
		{name: "auth error", err: errors.New("provider prov_x auth error (401): unauthorized"), want: false},
		{name: "generic invalid parameter 400", err: errors.New("provider returned 400: invalid parameter max_tokens"), want: false},
		{name: "model not found 404", err: errors.New("provider returned 404: model not found"), want: false},
	}

	for _, tc := range tests {
		if got := shouldRetryWithoutPreviousResponseID(tc.err); got != tc.want {
			t.Fatalf("%s: shouldRetryWithoutPreviousResponseID(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}

func TestParseStatusCodeFromUpstreamError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want int
	}{
		{name: "provider returned", msg: "provider returned 409: continuation mismatch", want: 409},
		{name: "upstream", msg: "upstream 502: bad gateway", want: 502},
		{name: "proxy returned", msg: "proxy returned 400: invalid request", want: 400},
		{name: "no status", msg: "timeout", want: 0},
		{name: "invalid status", msg: "provider returned 9: weird", want: 0},
	}
	for _, tc := range tests {
		if got := parseStatusCodeFromUpstreamError(tc.msg); got != tc.want {
			t.Fatalf("%s: parseStatusCodeFromUpstreamError(%q) = %d, want %d", tc.name, tc.msg, got, tc.want)
		}
	}
}

func TestShouldTreatStreamingProbeAsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		prefix []byte
		err    error
		want   bool
	}{
		{name: "empty eof", prefix: nil, err: io.EOF, want: true},
		{name: "empty nil err", prefix: nil, err: nil, want: true},
		{name: "empty non eof error", prefix: nil, err: errors.New("connection reset"), want: false},
		{name: "done only raw", prefix: []byte("[DONE]"), err: io.EOF, want: true},
		{name: "done only sse", prefix: []byte("data: [DONE]\n\n"), err: io.EOF, want: true},
		{name: "done marker but stream may continue", prefix: []byte("data: [DONE]\n\n"), err: nil, want: false},
		{name: "normal json with eof", prefix: []byte(`data: {"id":"x"}`), err: io.EOF, want: false},
	}

	for _, tc := range tests {
		if got := shouldTreatStreamingProbeAsEmpty(tc.prefix, tc.err); got != tc.want {
			t.Fatalf("%s: shouldTreatStreamingProbeAsEmpty(%q, %v) = %v, want %v", tc.name, string(tc.prefix), tc.err, got, tc.want)
		}
	}
}

func TestHasPreviousResponseContinuation(t *testing.T) {
	bodyWithPrevID := []byte(`{"model":"x","previous_response_id":"resp_123","input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	bodyWithoutPrevID := []byte(`{"model":"x","input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)

	tests := []struct {
		name         string
		body         []byte
		cachedPrevID string
		want         bool
	}{
		{name: "inline previous_response_id", body: bodyWithPrevID, cachedPrevID: "", want: true},
		{name: "cached previous_response_id", body: bodyWithoutPrevID, cachedPrevID: "resp_cached", want: true},
		{name: "no continuation", body: bodyWithoutPrevID, cachedPrevID: "", want: false},
		{name: "empty body but cached continuation", body: nil, cachedPrevID: "resp_cached", want: true},
	}

	for _, tc := range tests {
		if got := hasPreviousResponseContinuation(tc.body, tc.cachedPrevID); got != tc.want {
			t.Fatalf("%s: hasPreviousResponseContinuation(...) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDisableResponsesContinuationForRoute_SessionScopedAndStripsPreviousResponseID(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	route := &providerpool.RouteResult{
		Provider: &providerpool.Provider{ID: "provider-a"},
		Model:    &providerpool.Model{ID: "gpt-5.3-codex"},
	}

	reqA := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	reqA = reqA.WithContext(WithSessionID(context.Background(), "sess-disable-helper-a"))
	ph.setCachedResponsesPreviousIDForRoute(reqA, route, "resp_prev_a")

	body := []byte(`{"model":"gpt-5.3-codex","previous_response_id":"resp_prev_a","input":[{"role":"user","content":[{"type":"input_text","text":"continue"}]}]}`)
	rewritten := ph.disableResponsesContinuationForRoute(reqA, "provider-a", "gpt-5.3-codex", body)

	if bytes.Contains(rewritten, []byte(`"previous_response_id"`)) {
		t.Fatalf("previous_response_id should be stripped after disable: %s", string(rewritten))
	}
	if got := ph.getCachedResponsesPreviousIDForRoute(reqA, route); got != "" {
		t.Fatalf("cached previous_response_id should be cleared, got %q", got)
	}
	if !ph.isResponsesContinuationDisabledForRoute(reqA, route) {
		t.Fatal("continuation should be disabled for session A route")
	}

	reqB := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	reqB = reqB.WithContext(WithSessionID(context.Background(), "sess-disable-helper-b"))
	ph.setCachedResponsesPreviousIDForRoute(reqB, route, "resp_prev_b")
	if ph.isResponsesContinuationDisabledForRoute(reqB, route) {
		t.Fatal("continuation disable state must not leak across sessions")
	}
	if got := ph.getCachedResponsesPreviousIDForRoute(reqB, route); got != "resp_prev_b" {
		t.Fatalf("session B cache should remain intact, got %q", got)
	}
}
