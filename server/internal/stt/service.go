package stt

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

// ServiceConfig holds the configuration for the STT service.
type ServiceConfig struct {
	DefaultProvider ProviderType
	Providers       []ProviderConfig
}

// NewService creates a new STT service.
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
	case ProviderWhisperAPI:
		return NewWhisperAPIProvider(&WhisperAPIConfig{
			APIKey:      cfg.APIKey,
			BaseURL:     cfg.BaseURL,
			Model:       cfg.Model,
			MaxDuration: cfg.MaxDuration,
		}), nil
	case ProviderWhisperLocal:
		return NewWhisperLocalProvider(&WhisperLocalConfig{
			BaseURL:     cfg.BaseURL,
			Model:       cfg.Model,
			MaxDuration: cfg.MaxDuration,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type)
	}
}

// Transcribe transcribes audio using the default provider.
func (s *service) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	return s.TranscribeWithProvider(ctx, s.defaultProvider, req)
}

// TranscribeWithProvider transcribes audio using a specific provider.
func (s *service) TranscribeWithProvider(ctx context.Context, providerType ProviderType, req *TranscribeRequest) (*TranscribeResponse, error) {
	s.mu.RLock()
	provider, ok := s.providers[providerType]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrProviderNotFound
	}

	return provider.Transcribe(ctx, req)
}

// TranscribeStream transcribes audio with streaming results.
func (s *service) TranscribeStream(ctx context.Context, req *TranscribeRequest, callback StreamCallback) error {
	s.mu.RLock()
	provider, ok := s.providers[s.defaultProvider]
	s.mu.RUnlock()

	if !ok {
		return ErrProviderNotFound
	}

	return provider.TranscribeStream(ctx, req, callback)
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
