package voice

import (
	"testing"
)

func TestSynthesizeCaching(t *testing.T) {
	cfg := &ServiceConfig{
		TTSService: nil,
	}

	svc := NewService(cfg)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}

	voiceSvc := svc.(*service)
	if len(voiceSvc.ttsIndex) != 0 {
		t.Error("Cache index should be empty initially")
	}
	if voiceSvc.ttsCacheDir == "" {
		t.Error("Cache dir should be set")
	}
}

func TestServiceCreation(t *testing.T) {
	cfg := &ServiceConfig{
		TTSService: nil,
	}

	svc := NewService(cfg)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}

	voiceSvc := svc.(*service)
	if voiceSvc.sessions == nil {
		t.Error("Sessions map should be initialized")
	}
	if voiceSvc.ttsCacheDir == "" {
		t.Error("TTS cache dir should be set")
	}
}

func TestCacheHash(t *testing.T) {
	h1 := ttsCacheHash("provider:Hello:mp3:1.00:1.00:1.00")
	h2 := ttsCacheHash("provider:Hello:mp3:1.00:1.00:1.00")
	h3 := ttsCacheHash("provider:World:mp3:1.00:1.00:1.00")

	if h1 != h2 {
		t.Errorf("Same input should produce same hash: %s vs %s", h1, h2)
	}
	if h1 == h3 {
		t.Error("Different input should produce different hash")
	}
	if len(h1) != 32 {
		t.Errorf("Hash should be 32 hex chars, got %d", len(h1))
	}
}
