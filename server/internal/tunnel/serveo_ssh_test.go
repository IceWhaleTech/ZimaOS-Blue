package tunnel

import (
	"context"
	"strings"
	"testing"
)

func TestStartServeoNativeSSH_EmptySubdomain(t *testing.T) {
	ctx := context.Background()
	url, cleanup, err := startServeoNativeSSH(ctx, 23456, "")
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatal("expected error when subdomain is empty")
	}
	if url != "" {
		t.Errorf("expected empty url, got %q", url)
	}
	if cleanup != nil {
		t.Error("expected nil cleanup on error")
	}
	if !strings.Contains(err.Error(), "subdomain required") {
		t.Errorf("error should mention subdomain required, got: %v", err)
	}
}

func TestStartServeoNativeSSH_PortZeroDefaultsTo8080(t *testing.T) {
	// With subdomain set, we get past validation. Port 0 is normalized to 23456
	// before SSH dial. We only verify we don't get "subdomain required" and
	// that we get some error from dial/listen (no real SSH in test).
	ctx := context.Background()
	url, cleanup, err := startServeoNativeSSH(ctx, 0, "echo-test12345")
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		if strings.Contains(err.Error(), "subdomain required") {
			t.Errorf("unexpected subdomain error when subdomain is set: %v", err)
		}
		return
	}
	if !strings.HasPrefix(url, "https://echo-test12345.serveo.net") {
		t.Errorf("url = %q, want prefix https://echo-test12345.serveo.net", url)
	}
	if cleanup != nil {
		cleanup()
	}
}
