package tunnel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLocaltunnel_MockServer(t *testing.T) {
	// Mock localtunnel server response
	body := localtunnelInfo{
		ID:           "test-id",
		IP:           "127.0.0.1",
		Port:         12345,
		URL:          "https://test-id.localtunnel.me",
		MaxConnCount: 1,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/foo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.RawQuery != "new" && r.URL.Path != "/foo" {
			t.Errorf("expected query new for random tunnel, got %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()

	ctx := context.Background()

	// Random tunnel: GET server/?new
	info, err := requestLocaltunnel(ctx, server.URL, "")
	if err != nil {
		t.Fatalf("requestLocaltunnel: %v", err)
	}
	if info.URL != body.URL {
		t.Errorf("url = %s, want %s", info.URL, body.URL)
	}
	if info.Port != body.Port {
		t.Errorf("port = %d, want %d", info.Port, body.Port)
	}
	if info.IP != body.IP {
		t.Errorf("ip = %s, want %s", info.IP, body.IP)
	}

	// Named subdomain: GET server/foo
	body.ID = "foo"
	body.URL = "https://foo.localtunnel.me"
	info2, err := requestLocaltunnel(ctx, server.URL, "foo")
	if err != nil {
		t.Fatalf("requestLocaltunnel subdomain: %v", err)
	}
	if info2.URL != body.URL {
		t.Errorf("subdomain url = %s, want %s", info2.URL, body.URL)
	}
}
