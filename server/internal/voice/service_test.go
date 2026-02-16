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
	if voiceSvc.ttsCache == nil {
		t.Fatal("TTS cache should be initialized")
	}

	if len(voiceSvc.ttsCache) != 0 {
		t.Error("Cache should be empty initially")
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

	if voiceSvc.ttsCache == nil {
		t.Error("TTS cache should be initialized")
	}
}

func TestCacheKeyGeneration(t *testing.T) {
	tests := []struct {
		name    string
		req1    *SynthesizeRequest
		req2    *SynthesizeRequest
		sameKey bool
	}{
		{
			name: "identical requests",
			req1: &SynthesizeRequest{
				Text:   "Hello",
				Format: "mp3",
			},
			req2: &SynthesizeRequest{
				Text:   "Hello",
				Format: "mp3",
			},
			sameKey: true,
		},
		{
			name: "different text",
			req1: &SynthesizeRequest{
				Text:   "Hello",
				Format: "mp3",
			},
			req2: &SynthesizeRequest{
				Text:   "World",
				Format: "mp3",
			},
			sameKey: false,
		},
		{
			name: "different format",
			req1: &SynthesizeRequest{
				Text:   "Hello",
				Format: "mp3",
			},
			req2: &SynthesizeRequest{
				Text:   "Hello",
				Format: "wav",
			},
			sameKey: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := generateCacheKey(tt.req1)
			key2 := generateCacheKey(tt.req2)

			if tt.sameKey && key1 != key2 {
				t.Errorf("Expected same cache key, got different: %s vs %s", key1, key2)
			}
			if !tt.sameKey && key1 == key2 {
				t.Errorf("Expected different cache keys, got same: %s", key1)
			}
		})
	}
}

func generateCacheKey(req *SynthesizeRequest) string {
	return req.Text + ":" + req.Format
}
