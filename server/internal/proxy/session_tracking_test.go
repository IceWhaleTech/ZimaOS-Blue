package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCopyResponse_UpdatesSessionUsage_NonStreaming(t *testing.T) {
	sm := NewSessionMonitor(nil)
	session := sm.StartSession("127.0.0.1", "test-agent")
	if session == nil {
		t.Fatal("expected session to be created")
	}

	ph := NewProxyHandler(nil, nil, nil)
	ph.SetSessionMonitor(sm)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"1","usage":{"prompt_tokens":12,"completion_tokens":5,"total_tokens":17}}`,
		)),
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = req.WithContext(WithSessionID(context.Background(), session.ID))

	rr := httptest.NewRecorder()
	ph.copyResponse(rr, resp, &parsedRequest{
		resolvedProvider: "openai",
		resolvedModel:    "gpt-4o",
	}, req)

	got, ok := sm.GetSession(session.ID)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if got.Provider != "openai" || got.Model != "gpt-4o" {
		t.Fatalf("provider/model = %s/%s, want openai/gpt-4o", got.Provider, got.Model)
	}
	if got.TokensIn != 12 || got.TokensOut != 5 {
		t.Fatalf("tokens = %d/%d, want 12/5", got.TokensIn, got.TokensOut)
	}
	if got.RequestCount != 1 {
		t.Fatalf("request_count = %d, want 1", got.RequestCount)
	}
}

func TestCopyResponse_UpdatesSessionUsage_Streaming(t *testing.T) {
	sm := NewSessionMonitor(nil)
	session := sm.StartSession("127.0.0.1", "test-agent")
	if session == nil {
		t.Fatal("expected session to be created")
	}

	ph := NewProxyHandler(nil, nil, nil)
	ph.SetSessionMonitor(sm)

	sse := "" +
		`data: {"id":"1","choices":[{"delta":{"content":"hello"}}]}` + "\n\n" +
		`data: {"id":"1","usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12},"choices":[{"delta":{},"finish_reason":"stop"}]}` + "\n\n" +
		`data: [DONE]` + "\n\n"

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(sse)),
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = req.WithContext(WithSessionID(context.Background(), session.ID))

	rr := httptest.NewRecorder()
	ph.copyResponse(rr, resp, &parsedRequest{
		streaming:        true,
		resolvedProvider: "openai",
		resolvedModel:    "gpt-4o",
	}, req)

	got, ok := sm.GetSession(session.ID)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if got.TokensIn != 10 || got.TokensOut != 2 {
		t.Fatalf("tokens = %d/%d, want 10/2", got.TokensIn, got.TokensOut)
	}
	if got.RequestCount != 1 {
		t.Fatalf("request_count = %d, want 1", got.RequestCount)
	}
}

func TestProxyServerWrapWithSessionTracking_FallbackUpdate(t *testing.T) {
	sm := NewSessionMonitor(nil)
	ps := &ProxyServer{
		sessionMonitor: sm,
	}

	handler := ps.wrapWithSessionTracking(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Actual-Provider", "anthropic")
		w.Header().Set("X-Actual-Model", "claude-3-7-sonnet")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	sessionID := rr.Header().Get("X-Session-ID")
	if sessionID == "" {
		t.Fatal("expected X-Session-ID header")
	}
	got, ok := sm.GetSession(sessionID)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if got.Provider != "anthropic" || got.Model != "claude-3-7-sonnet" {
		t.Fatalf("provider/model = %s/%s, want anthropic/claude-3-7-sonnet", got.Provider, got.Model)
	}
	if got.RequestCount != 1 {
		t.Fatalf("request_count = %d, want 1", got.RequestCount)
	}
	if got.Status != SessionStatusCompleted {
		t.Fatalf("status = %s, want %s", got.Status, SessionStatusCompleted)
	}
}
