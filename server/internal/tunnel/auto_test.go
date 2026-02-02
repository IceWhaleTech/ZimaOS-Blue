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

func TestAutoManager_BlacklistIntegration(t *testing.T) {
	// Create auto manager
	am := NewAutoManager()

	// Verify blacklist is initialized
	if am.blacklist == nil {
		t.Error("Blacklist should be initialized")
	}

	// Add a provider to blacklist
	err := am.blacklist.Add(ProviderBore, "test failure")
	if err != nil {
		t.Fatalf("Failed to add provider to blacklist: %v", err)
	}

	// Verify provider is blacklisted
	if !am.blacklist.IsBlacklisted(ProviderBore) {
		t.Error("Provider should be blacklisted")
	}
}

func TestAutoManager_ProviderOrder(t *testing.T) {
	am := NewAutoManager()

	// Verify provider order includes Cloudflare
	expectedProviders := []Provider{
		ProviderBore,
		ProviderServeo,
		ProviderLocalTunnel,
		ProviderCloudflare,
	}

	if len(am.providerOrder) != len(expectedProviders) {
		t.Errorf("Expected %d providers, got %d", len(expectedProviders), len(am.providerOrder))
	}

	// Verify all expected providers are present
	providerMap := make(map[Provider]bool)
	for _, p := range am.providerOrder {
		providerMap[p] = true
	}

	for _, expected := range expectedProviders {
		if !providerMap[expected] {
			t.Errorf("Provider %s not found in provider order", expected)
		}
	}
}

func TestAutoManager_GetProvider(t *testing.T) {
	am := NewAutoManager()

	if am.GetProvider() != ProviderAuto {
		t.Errorf("Expected provider to be %s, got %s", ProviderAuto, am.GetProvider())
	}
}

func TestAutoManager_InitialState(t *testing.T) {
	am := NewAutoManager()

	// Verify initial state
	if am.IsRunning() {
		t.Error("Manager should not be running initially")
	}

	if am.GetURL() != "" {
		t.Error("URL should be empty initially")
	}

	status := am.GetStatus()
	if status.Active {
		t.Error("Status should not be active initially")
	}

	if status.Provider != ProviderAuto {
		t.Errorf("Status provider should be %s, got %s", ProviderAuto, status.Provider)
	}
}

func TestAutoManager_StopWhenNotRunning(t *testing.T) {
	am := NewAutoManager()

	// Stopping when not running should not error
	err := am.Stop()
	if err != nil {
		t.Errorf("Stop should not error when not running: %v", err)
	}
}

func TestAutoManager_URLCallback(t *testing.T) {
	am := NewAutoManager()

	urlReceived := ""
	am.SetOnURLChange(func(url string) {
		urlReceived = url
	})

	// Simulate URL change (this would normally happen when a provider connects)
	testURL := "https://test.example.com"
	if am.onURLChange != nil {
		am.onURLChange(testURL)
	}

	if urlReceived != testURL {
		t.Errorf("Expected URL %s, got %s", testURL, urlReceived)
	}
}

func TestAutoManager_ErrorCallback(t *testing.T) {
	am := NewAutoManager()

	errorReceived := false
	am.SetOnError(func(err error) {
		errorReceived = true
	})

	// Simulate error
	if am.onError != nil {
		am.onError(context.Canceled)
	}

	if !errorReceived {
		t.Error("Error callback should have been called")
	}
}

func TestAutoManager_ConcurrentAccess(t *testing.T) {
	am := NewAutoManager()

	// Test concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = am.IsRunning()
			_ = am.GetURL()
			_ = am.GetStatus()
			_ = am.GetProvider()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestImmediateFailureThreshold(t *testing.T) {
	// Verify the threshold is set correctly
	if ImmediateFailureThreshold != 5*time.Second {
		t.Errorf("Expected threshold to be 5 seconds, got %v", ImmediateFailureThreshold)
	}
}

func TestBlacklistDuration(t *testing.T) {
	// Verify the blacklist duration is set correctly
	if BlacklistDuration != 24*time.Hour {
		t.Errorf("Expected duration to be 24 hours, got %v", BlacklistDuration)
	}
}
