package tts

import (
	"fmt"
)

// ProviderFactory creates TTS providers based on type
type ProviderFactory struct{}

// CreateProvider creates a TTS provider based on the provider type
func (pf *ProviderFactory) CreateProvider(providerType string) (Provider, error) {
	switch providerType {
	case "edge-tts":
		return NewEdgeTTSProvider(&EdgeTTSConfig{
			DefaultVoice:  "en-US-AriaNeural",
			DefaultFormat: FormatMP3,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

// GetAvailableProviders returns list of available provider types
func (pf *ProviderFactory) GetAvailableProviders() []string {
	return []string{"edge-tts"}
}
