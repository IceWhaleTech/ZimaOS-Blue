package stt

import (
	"testing"
	"time"
)

func TestLazyServicePeekAndIdleReclaim(t *testing.T) {
	svc := NewLazyService(t.TempDir(), 20*time.Millisecond)

	if provider := svc.PeekWhisperProvider(); provider != nil {
		t.Fatal("PeekWhisperProvider() created a provider unexpectedly")
	}

	provider := svc.GetWhisperProvider()
	if provider == nil {
		t.Fatal("GetWhisperProvider() returned nil")
	}
	provider.initialized = true

	time.Sleep(80 * time.Millisecond)

	if got := svc.PeekWhisperProvider(); got != provider {
		t.Fatal("expected lazy service to retain the same Whisper provider instance")
	}
	if provider.initialized {
		t.Fatal("expected idle reclaim to unload the Whisper provider state")
	}
}
