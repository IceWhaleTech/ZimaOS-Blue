//go:build darwin

package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestLibcurlHTTPNativeClient_Do_RealLibcurlSmoke(t *testing.T) {
	const libcurlPath = "/usr/lib/libcurl.4.dylib"
	if _, err := os.Stat(libcurlPath); err != nil {
		t.Skipf("libcurl not available at %s: %v", libcurlPath, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Native-Smoke", "1")
		_, _ = w.Write([]byte("native ok"))
	}))
	defer server.Close()

	client := newWebFetchHTTPNativeClient(WebFetchConfig{
		HTTPNativeEnabled: true,
		HTTPNativeLibrary: libcurlPath,
	})
	if client == nil {
		t.Fatal("expected native client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Do(ctx, webFetchHTTPNativeRequest{
		URL:               server.URL,
		Timeout:           10 * time.Second,
		MaxRedirects:      0,
		MaxResponseBytes:  1024,
		AllowPrivateHosts: true,
	})
	if err != nil {
		t.Fatalf("native libcurl request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status code = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := string(resp.Body); got != "native ok" {
		t.Fatalf("body = %q, want %q", got, "native ok")
	}
	if got := resp.Headers.Get("X-Native-Smoke"); got != "1" {
		t.Fatalf("X-Native-Smoke = %q, want %q", got, "1")
	}
	if got := resp.ContentType; got != "text/plain; charset=utf-8" {
		t.Fatalf("content type = %q, want %q", got, "text/plain; charset=utf-8")
	}
}
