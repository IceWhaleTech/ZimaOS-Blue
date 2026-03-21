package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigUsesRelayDriverWhenCDPURLIsSet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CDPURL = "http://127.0.0.1:18792?token=test"

	if !cfg.UsesRelayDriver() {
		t.Fatalf("UsesRelayDriver() = false, want true")
	}
	if got := cfg.EffectivePoolSize(); got != 1 {
		t.Fatalf("EffectivePoolSize() = %d, want 1", got)
	}
}

func TestConfigUsesRelayDriverWhenBuiltInRelayEnabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RelayEnabled = true
	cfg.RelayToken = "test-token"

	if !cfg.UsesRelayDriver() {
		t.Fatalf("UsesRelayDriver() = false, want true")
	}
	if got := cfg.EffectiveCDPURL(); got != "http://127.0.0.1:18792?token=test-token" {
		t.Fatalf("EffectiveCDPURL() = %q, want %q", got, "http://127.0.0.1:18792?token=test-token")
	}
}

func TestResolveCDPWebSocketURL_ConvertsCDPHTTPPathToWS(t *testing.T) {
	got, err := resolveCDPWebSocketURL(context.Background(), "http://127.0.0.1:18888/cdp?token=test")
	if err != nil {
		t.Fatalf("resolveCDPWebSocketURL() error = %v", err)
	}
	want := "ws://127.0.0.1:18888/cdp?token=test"
	if got != want {
		t.Fatalf("resolveCDPWebSocketURL() = %q, want %q", got, want)
	}
}

func TestResolveCDPWebSocketURL_ResolvesVersionEndpointAndPreservesQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/json/version")
		}
		if got := r.URL.Query().Get("token"); got != "test-token" {
			t.Fatalf("token query = %q, want %q", got, "test-token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1:9999/cdp"}`))
	}))
	defer srv.Close()

	got, err := resolveCDPWebSocketURL(context.Background(), srv.URL+"?token=test-token")
	if err != nil {
		t.Fatalf("resolveCDPWebSocketURL() error = %v", err)
	}
	want := "ws://127.0.0.1:9999/cdp?token=test-token"
	if got != want {
		t.Fatalf("resolveCDPWebSocketURL() = %q, want %q", got, want)
	}
}
