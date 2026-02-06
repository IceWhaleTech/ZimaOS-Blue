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
	speed           float32
	pitch           float32
	volume          float32
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
		speed:           1.0,
		pitch:           0,
		volume:          100,
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
	case ProviderEspeakNG:
		dataPath := "./data"
		if cfg.BaseURL != "" {
			dataPath = cfg.BaseURL
		}
		return NewEspeakNGAdapter(dataPath), nil
	case ProviderEdge:
		return NewEdgeTTSProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type)
	}
}

// Synthesize synthesizes text using the default provider.
func (s *service) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	return s.SynthesizeWithProvider(ctx, s.defaultProvider, req)
}

// SynthesizeWithProvider synthesizes text using a specific provider with fallback.
func (s *service) SynthesizeWithProvider(ctx context.Context, providerType ProviderType, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	s.mu.RLock()
	provider, ok := s.providers[providerType]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrProviderNotFound
	}

	// Try the specified provider
	result, err := provider.Synthesize(ctx, req)
	if err == nil {
		return result, nil
	}

	// If specified provider fails, try fallback to default provider
	if providerType != s.defaultProvider {
		s.mu.RLock()
		defaultProvider, ok := s.providers[s.defaultProvider]
		s.mu.RUnlock()

		if ok {
			result, fallbackErr := defaultProvider.Synthesize(ctx, req)
			if fallbackErr == nil {
				return result, nil
			}
		}
	}

	// If both fail, return original error
	return nil, err
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

// SetDefaultProvider sets the default provider type.
func (s *service) SetDefaultProvider(providerType ProviderType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.providers[providerType]; !ok {
		return fmt.Errorf("provider %s not available", providerType)
	}

	s.defaultProvider = providerType
	return nil
}

// GetProvider returns the provider instance for a given provider type.
func (s *service) GetProvider(providerType ProviderType) Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.providers[providerType]
}

// GetConfig returns the current TTS configuration (speed, pitch, volume).
func (s *service) GetConfig() (speed, pitch, volume float32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.speed, s.pitch, s.volume
}

// SetConfig sets the TTS configuration (speed, pitch, volume).
func (s *service) SetConfig(speed, pitch, volume float32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if speed > 0 {
		s.speed = speed
	}
	s.pitch = pitch
	if volume >= 0 {
		s.volume = volume
	}
}

// Close cleans up all provider resources.
func (s *service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close eSpeak-NG adapter if it exists
	if p, ok := s.providers[ProviderEspeakNG]; ok {
		if esp, ok := p.(*EspeakNGAdapter); ok {
			esp.Close()
		}
	}
}
