package oauth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestGoogleOAuthClientSecretsFromEnv(t *testing.T) {
	t.Setenv(EnvAntigravityClientSecret, "ant-secret")
	t.Setenv(EnvGeminiCLIClientSecret, "gem-secret")

	if got := AntigravityConfig().ClientSecret; got != "ant-secret" {
		t.Fatalf("antigravity client secret mismatch: got %q", got)
	}
	if got := GeminiCLIConfig().ClientSecret; got != "gem-secret" {
		t.Fatalf("gemini-cli client secret mismatch: got %q", got)
	}
}

func TestGoogleOAuthClientSecretsDefaultEmpty(t *testing.T) {
	t.Setenv(EnvAntigravityClientSecret, "")
	t.Setenv(EnvGeminiCLIClientSecret, "")

	if got := AntigravityConfig().ClientSecret; got != "" {
		t.Fatalf("expected empty antigravity secret, got %q", got)
	}
	if got := GeminiCLIConfig().ClientSecret; got != "" {
		t.Fatalf("expected empty gemini-cli secret, got %q", got)
	}
}

func TestExchangeCodeSkipsEmptyClientSecret(t *testing.T) {
	var captured url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse query: %v", err)
		}
		captured = values
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"r","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		TokenURL:     srv.URL,
		RedirectPort: 8080,
		RedirectPath: "/oauth-callback",
		ClientID:     "client-id",
	}
	m := NewManager(&mockTokenStore{})

	if _, err := m.exchangeCode(context.Background(), cfg, "code-1", "verifier-1"); err != nil {
		t.Fatalf("exchangeCode failed: %v", err)
	}
	if got := captured.Get("client_secret"); got != "" {
		t.Fatalf("expected no client_secret in request, got %q", got)
	}
}

func TestRefreshTokenIncludesClientSecretWhenPresent(t *testing.T) {
	var captured url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse query: %v", err)
		}
		captured = values
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"r","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer srv.Close()

	cfg := &ProviderConfig{
		TokenURL:     srv.URL,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}
	m := NewManager(&mockTokenStore{})

	if _, err := m.refreshToken(context.Background(), cfg, "refresh-1"); err != nil {
		t.Fatalf("refreshToken failed: %v", err)
	}
	if got := captured.Get("client_secret"); got != "client-secret" {
		t.Fatalf("expected client_secret in request, got %q", got)
	}
}
