package tts

import (
	"context"
	"testing"
)

func TestEdgeTTSProvider(t *testing.T) {
	cfg := &EdgeTTSConfig{
		DefaultVoice:  "en-US-AriaNeural",
		DefaultFormat: FormatMP3,
		MaxTextLength: 5000,
	}

	provider := NewEdgeTTSProvider(cfg)
	if provider == nil {
		t.Fatal("NewEdgeTTSProvider returned nil")
	}

	if provider.Name() != "Microsoft Edge TTS" {
		t.Errorf("Expected provider name 'Microsoft Edge TTS', got '%s'", provider.Name())
	}

	if provider.Type() != ProviderEdge {
		t.Errorf("Expected provider type ProviderEdge, got %v", provider.Type())
	}

	formats := provider.SupportedFormats()
	if len(formats) == 0 {
		t.Error("Expected supported formats, got empty list")
	}

	if provider.MaxTextLength() != 5000 {
		t.Errorf("Expected max text length 5000, got %d", provider.MaxTextLength())
	}
}

func TestEdgeTTSListVoices(t *testing.T) {
	cfg := &EdgeTTSConfig{
		DefaultVoice:  "en-US-AriaNeural",
		DefaultFormat: FormatMP3,
	}

	provider := NewEdgeTTSProvider(cfg)
	ctx := context.Background()

	voices, err := provider.ListVoices(ctx)
	if err != nil {
		t.Fatalf("ListVoices failed: %v", err)
	}

	if len(voices) == 0 {
		t.Error("Expected voices, got empty list")
	}

	// Check for expected voices
	hasAriaNeural := false
	for _, v := range voices {
		if v.ID == "en-US-AriaNeural" {
			hasAriaNeural = true
			break
		}
	}

	if !hasAriaNeural {
		t.Error("Expected to find en-US-AriaNeural voice")
	}
}

func TestProviderFactory(t *testing.T) {
	factory := &ProviderFactory{}

	providers := factory.GetAvailableProviders()
	if len(providers) == 0 {
		t.Error("Expected available providers, got empty list")
	}

	// Test creating edge-tts provider
	provider, err := factory.CreateProvider("edge-tts")
	if err != nil {
		t.Fatalf("Failed to create edge-tts provider: %v", err)
	}

	if provider == nil {
		t.Fatal("CreateProvider returned nil")
	}

	if provider.Type() != ProviderEdge {
		t.Errorf("Expected ProviderEdge, got %v", provider.Type())
	}

	// Test creating unsupported provider
	_, err = factory.CreateProvider("unsupported")
	if err == nil {
		t.Error("Expected error for unsupported provider")
	}
}
