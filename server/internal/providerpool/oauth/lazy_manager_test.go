package oauth

import (
	"context"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

type noopTokenStore struct{}

func (noopTokenStore) SaveToken(string, *Token) error                  { return nil }
func (noopTokenStore) LoadToken(string, string) (*Token, error)        { return nil, nil }
func (noopTokenStore) LoadTokenByEmail(string, string) (*Token, error) { return nil, nil }
func (noopTokenStore) LoadTokens(string) ([]*Token, error)             { return nil, nil }
func (noopTokenStore) DeleteToken(string, string) error                { return nil }
func (noopTokenStore) ListTokens() (map[string]*Token, error)          { return map[string]*Token{}, nil }

func TestLazyManagerStartAuthAppliesDeferredPortAndInitializesOnce(t *testing.T) {
	var initCount atomic.Int32
	lazy := NewLazyManager(func() (*Manager, error) {
		initCount.Add(1)
		return NewManager(noopTokenStore{}), nil
	})

	lazy.SetPort(4242)

	result, err := lazy.StartAuth(context.Background(), "provider-1", "gemini-cli")
	if err != nil {
		t.Fatalf("StartAuth() error = %v", err)
	}
	if got := initCount.Load(); got != 1 {
		t.Fatalf("init count = %d, want 1", got)
	}

	parsed, err := url.Parse(result.AuthURL)
	if err != nil {
		t.Fatalf("Parse(AuthURL) error = %v", err)
	}
	if got := parsed.Query().Get("redirect_uri"); got != "http://localhost:4242/oauth2callback" {
		t.Fatalf("redirect_uri = %q, want %q", got, "http://localhost:4242/oauth2callback")
	}

	if _, err := lazy.StartAuth(context.Background(), "provider-2", "gemini-cli"); err != nil {
		t.Fatalf("second StartAuth() error = %v", err)
	}
	if got := initCount.Load(); got != 1 {
		t.Fatalf("init count after second StartAuth = %d, want 1", got)
	}
}

func TestLazyManagerWarmAsyncInitializesInBackground(t *testing.T) {
	lazy := NewLazyManager(func() (*Manager, error) {
		return NewManager(noopTokenStore{}), nil
	})

	lazy.WarmAsync()

	deadline := time.Now().Add(2 * time.Second)
	for !lazy.IsReady() {
		if time.Now().After(deadline) {
			t.Fatal("lazy manager did not become ready after WarmAsync")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
