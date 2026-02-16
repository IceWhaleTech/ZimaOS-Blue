package tunnel

import (
	"context"
	"testing"
	"time"
)

func TestAutoManager_ProviderOrder(t *testing.T) {
	am := NewAutoManager()

	expectedProviders := []Provider{
		ProviderCloudflare,
	}

	if len(am.providerOrder) != len(expectedProviders) {
		t.Errorf("Expected %d providers, got %d", len(expectedProviders), len(am.providerOrder))
	}

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

	if am.onError != nil {
		am.onError(context.Canceled)
	}

	if !errorReceived {
		t.Error("Error callback should have been called")
	}
}

func TestAutoManager_ConcurrentAccess(t *testing.T) {
	am := NewAutoManager()

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

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestImmediateFailureThreshold(t *testing.T) {
	if ImmediateFailureThreshold != 5*time.Second {
		t.Errorf("Expected threshold to be 5 seconds, got %v", ImmediateFailureThreshold)
	}
}

func TestBlacklistDuration(t *testing.T) {
	if BlacklistDuration != 24*time.Hour {
		t.Errorf("Expected duration to be 24 hours, got %v", BlacklistDuration)
	}
}
