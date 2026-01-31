package tunnel

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestAutoManager_Start_ShowsAllProviderFailures verifies that when multiple providers
// fail, the error message includes all of them (e.g. "serveo: ...; bore: ...").
func TestAutoManager_Start_ShowsAllProviderFailures(t *testing.T) {
	m := NewAutoManager()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.Start(ctx, &Config{Port: 9999, Subdomain: "echo-test12345"})
	if err == nil {
		t.Fatal("expected error when no tunnel can be established")
	}
	msg := err.Error()
	if !strings.Contains(msg, "all providers failed") {
		t.Errorf("error should mention 'all providers failed', got: %s", msg)
	}
	// All providers should be attempted; error should mention at least bore and serveo (and possibly localtunnel)
	if !strings.Contains(msg, "serveo") {
		t.Errorf("error should mention serveo, got: %s", msg)
	}
	if !strings.Contains(msg, "bore") {
		t.Errorf("error should mention bore, got: %s", msg)
	}
}
