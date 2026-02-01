package tts

import (
	"context"
	"fmt"
	"sync"
)

// service implements the Service interface.
type service struct {
	providers       map[ProviderType]Provider
	defaultProvider ProviderType
	mu              sync.RWMutex
}

// ServiceConfig holds the configuration for the TTS service.
type ServiceConfig struct {
	DefaultProvider ProviderType
	Providers       []ProviderConfig
}

// NewService creates a new TTS service.
func NewService(cfg *ServiceConfig) (Service, error) {
	s := &service{
		providers:       make(map[ProviderType]Provider),
		defaultProvider: cfg.DefaultProvider,
	}

	// Initialize providers
	for _, providerCfg := range cfg.Providers {
		if !providerCfg.Enabled {
			continue
		}

		provider, err := createProvider(providerCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider %s: %w", providerCfg.Type, err)
		}
		s.providers[providerCfg.Type] = provider
	}

	// Set default provider if not specified
	if s.defaultProvider == "" && len(s.providers) > 0 {
		for pt := range s.providers {
			s.defaultProvider = pt
			break
		}
	}

	return s, nil
}

// createProvider creates a provider based on the configuration.
func createProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.Type {
	case ProviderOpenAI:
		return NewOpenAIProvider(&OpenAIConfig{
			APIKey:        cfg.APIKey,
			BaseURL:       cfg.BaseURL,
			DefaultVoice:  cfg.DefaultVoice,
			DefaultFormat: cfg.DefaultFormat,
			MaxTextLength: cfg.MaxTextLength,
		}), nil
	case ProviderEdge:
		return NewEdgeTTSProvider(&EdgeTTSConfig{
			DefaultVoice:  cfg.DefaultVoice,
			DefaultFormat: cfg.DefaultFormat,
		}), nil
	case ProviderSherpa:
		return NewSherpaProvider(&SherpaConfig{
			ModelDir:      cfg.BaseURL, // Reuse BaseURL field for model directory
			ModelType:     cfg.DefaultVoice, // Reuse DefaultVoice for model type (kokoro, piper, etc.)
			DefaultFormat: cfg.DefaultFormat,
			MaxTextLength: cfg.MaxTextLength,
		}), nil
	case ProviderKokoro:
		// Legacy: redirect to Sherpa provider with kokoro model
		return NewSherpaProvider(&SherpaConfig{
			ModelDir:      cfg.BaseURL,
			ModelType:     "kokoro",
			DefaultFormat: cfg.DefaultFormat,
			MaxTextLength: cfg.MaxTextLength,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type)
	}
}

// Synthesize synthesizes text using the default provider.
func (s *service) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	return s.SynthesizeWithProvider(ctx, s.defaultProvider, req)
}

// SynthesizeWithProvider synthesizes text using a specific provider.
func (s *service) SynthesizeWithProvider(ctx context.Context, providerType ProviderType, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	s.mu.RLock()
	provider, ok := s.providers[providerType]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrProviderNotFound
	}

	return provider.Synthesize(ctx, req)
}

// SynthesizeStream synthesizes text with streaming audio output.
func (s *service) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
	s.mu.RLock()
	provider, ok := s.providers[s.defaultProvider]
	s.mu.RUnlock()

	if !ok {
		return ErrProviderNotFound
	}

	return provider.SynthesizeStream(ctx, req, callback)
}

// ListVoices returns available voices from the default provider.
func (s *service) ListVoices(ctx context.Context) ([]Voice, error) {
	s.mu.RLock()
	provider, ok := s.providers[s.defaultProvider]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrProviderNotFound
	}

	return provider.ListVoices(ctx)
}

// ListProviders returns all available providers.
func (s *service) ListProviders() []ProviderType {
	s.mu.RLock()
	defer s.mu.RUnlock()

	providers := make([]ProviderType, 0, len(s.providers))
	for pt := range s.providers {
		providers = append(providers, pt)
	}
	return providers
}

// GetDefaultProvider returns the default provider type.
func (s *service) GetDefaultProvider() ProviderType {
	return s.defaultProvider
}
