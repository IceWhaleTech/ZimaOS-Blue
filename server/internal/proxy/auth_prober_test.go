package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func TestAuthProber_Strategies_Default(t *testing.T) {
	ap := NewAuthProber()
	provider := &providerpool.Provider{ID: "p1", BaseURL: "https://api.openai.com", APIFormat: providerpool.APIFormatOpenAI}
	key := &providerpool.APIKey{Key: "sk-test"}

	strategies := ap.Strategies(provider, key, providerpool.APIFormatOpenAI)
	if len(strategies) != 3 {
		t.Fatalf("expected 3 strategies, got %d", len(strategies))
	}
	if strategies[0] != AuthBearer {
		t.Errorf("expected AuthBearer first, got %s", strategies[0])
	}
}

func TestAuthProber_Strategies_Anthropic(t *testing.T) {
	ap := NewAuthProber()
	provider := &providerpool.Provider{ID: "p1", BaseURL: "https://api.anthropic.com", APIFormat: providerpool.APIFormatAnthropic}
	key := &providerpool.APIKey{Key: "sk-ant-test"}

	strategies := ap.Strategies(provider, key, providerpool.APIFormatAnthropic)
	if strategies[0] != AuthAnthropic {
		t.Errorf("expected AuthAnthropic first for anthropic provider, got %s", strategies[0])
	}
}

func TestAuthProber_Strategies_NoKey(t *testing.T) {
	ap := NewAuthProber()
	provider := &providerpool.Provider{ID: "p1", BaseURL: "https://localhost:11434", APIFormat: providerpool.APIFormatOllama}

	strategies := ap.Strategies(provider, nil, providerpool.APIFormatOllama)
	if len(strategies) != 1 || strategies[0] != AuthNone {
		t.Errorf("expected [AuthNone] for no-key provider, got %v", strategies)
	}
}

func TestAuthProber_CachedWinnerFirst(t *testing.T) {
	ap := NewAuthProber()
	provider := &providerpool.Provider{ID: "p1", BaseURL: "https://relay.example.com", APIFormat: providerpool.APIFormatAnthropic}
	key := &providerpool.APIKey{Key: "sk-test"}

	// Remember that Bearer works
	ap.Remember("p1", "https://relay.example.com", AuthBearer)

	strategies := ap.Strategies(provider, key, providerpool.APIFormatAnthropic)
	if strategies[0] != AuthBearer {
		t.Errorf("expected cached AuthBearer first, got %s", strategies[0])
	}
	// Should not duplicate
	seen := map[AuthStrategy]int{}
	for _, s := range strategies {
		seen[s]++
		if seen[s] > 1 {
			t.Errorf("duplicate strategy %s", s)
		}
	}
}

func TestAuthProber_Apply(t *testing.T) {
	ap := NewAuthProber()
	key := &providerpool.APIKey{Key: "sk-test-key"}
	provider := &providerpool.Provider{ID: "p1"}

	tests := []struct {
		strategy AuthStrategy
		wantAuth string
		wantXAPI string
	}{
		{AuthBearer, "Bearer sk-test-key", ""},
		{AuthXAPIKey, "", "sk-test-key"},
		{AuthAnthropic, "", "sk-test-key"},
		{AuthNone, "", ""},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		ap.Apply(req, tt.strategy, key, provider)
		if got := req.Header.Get("Authorization"); got != tt.wantAuth {
			t.Errorf("strategy %s: Authorization = %q, want %q", tt.strategy, got, tt.wantAuth)
		}
		if got := req.Header.Get("x-api-key"); got != tt.wantXAPI {
			t.Errorf("strategy %s: x-api-key = %q, want %q", tt.strategy, got, tt.wantXAPI)
		}
	}

	// Anthropic should also set version header
	req := httptest.NewRequest("POST", "/v1/messages", nil)
	ap.Apply(req, AuthAnthropic, key, provider)
	if got := req.Header.Get("anthropic-version"); got != "2023-06-01" {
		t.Errorf("AuthAnthropic: anthropic-version = %q, want 2023-06-01", got)
	}
}

func TestAuthProber_ProbeAndForward_FirstSuccess(t *testing.T) {
	ap := NewAuthProber()
	provider := &providerpool.Provider{ID: "p1", BaseURL: "https://api.example.com", APIFormat: providerpool.APIFormatOpenAI}
	key := &providerpool.APIKey{Key: "sk-test"}

	calls := 0
	resp, err := ap.ProbeAndForward(provider, key, providerpool.APIFormatOpenAI,
		func() (*http.Request, error) {
			return httptest.NewRequest("POST", "/v1/chat/completions", nil), nil
		},
		func(req *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if calls != 1 {
		t.Errorf("expected 1 call (cached or first strategy), got %d", calls)
	}
	// Should have remembered the strategy
	if _, ok := ap.Recall("p1", "https://api.example.com"); !ok {
		t.Error("expected strategy to be cached after success")
	}
}

func TestAuthProber_ProbeAndForward_FallsThrough401(t *testing.T) {
	ap := NewAuthProber()
	provider := &providerpool.Provider{ID: "p1", BaseURL: "https://relay.example.com", APIFormat: providerpool.APIFormatOpenAI}
	key := &providerpool.APIKey{Key: "sk-test"}

	calls := 0
	resp, err := ap.ProbeAndForward(provider, key, providerpool.APIFormatOpenAI,
		func() (*http.Request, error) {
			return httptest.NewRequest("POST", "/v1/chat/completions", nil), nil
		},
		func(req *http.Request) (*http.Response, error) {
			calls++
			// First two strategies return 401, third (AuthNone) returns 200
			if calls < 3 {
				return &http.Response{StatusCode: 401, Body: http.NoBody}, nil
			}
			return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls (2 x 401 + 1 success), got %d", calls)
	}
}

func TestAuthProber_ProbeAndForward_AllExhausted(t *testing.T) {
	ap := NewAuthProber()
	provider := &providerpool.Provider{ID: "p1", BaseURL: "https://bad.example.com", APIFormat: providerpool.APIFormatOpenAI}
	key := &providerpool.APIKey{Key: "sk-test"}

	_, err := ap.ProbeAndForward(provider, key, providerpool.APIFormatOpenAI,
		func() (*http.Request, error) {
			return httptest.NewRequest("POST", "/v1/chat/completions", nil), nil
		},
		func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 401, Body: http.NoBody}, nil
		},
	)
	if err == nil {
		t.Fatal("expected error when all strategies exhausted")
	}
	var authErr *AuthExhaustedError
	if !isAuthExhaustedError(err) {
		t.Errorf("expected AuthExhaustedError, got %T: %v", err, err)
	}
	_ = authErr
}

func TestAuthProber_CustomHeaders(t *testing.T) {
	ap := NewAuthProber()
	provider := &providerpool.Provider{
		ID:      "p1",
		Headers: map[string]string{"X-Custom-Auth": "my-token"},
	}
	key := &providerpool.APIKey{Key: "sk-test"}

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	ap.Apply(req, AuthBearer, key, provider)

	if got := req.Header.Get("X-Custom-Auth"); got != "my-token" {
		t.Errorf("custom header not applied: got %q", got)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer sk-test" {
		t.Errorf("bearer not applied alongside custom: got %q", got)
	}
}

func isAuthExhaustedError(err error) bool {
	_, ok := err.(*AuthExhaustedError)
	if ok {
		return true
	}
	return fmt.Sprintf("%T", err) == "*proxy.AuthExhaustedError"
}
