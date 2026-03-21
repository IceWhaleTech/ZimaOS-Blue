package stt

import (
	"context"
	"fmt"
	"sync"
	"time"
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

// NewServiceWithProvider creates a new STT service with an existing provider.
func NewServiceWithProvider(provider *WhisperProvider) Service {
	s := &service{
		providers:       make(map[ProviderType]Provider),
		defaultProvider: ProviderWhisper,
	}
	s.providers[ProviderWhisper] = provider
	return s
}

// NewServiceFromProvider wraps any Provider into a Service.
func NewServiceFromProvider(provider Provider) Service {
	s := &service{
		providers:       make(map[ProviderType]Provider),
		defaultProvider: provider.Type(),
	}
	s.providers[provider.Type()] = provider
	return s
}

// createProvider creates a provider based on the configuration.
func createProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.Type {
	case ProviderWhisper:
		return NewWhisperProvider(&WhisperConfig{
			ModelPath:   cfg.Model,
			DefaultLang: cfg.DefaultLanguage,
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
	if err := ensureProviderInitialized(provider); err != nil {
		return nil, err
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
	if err := ensureProviderInitialized(provider); err != nil {
		return err
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

// GetWhisperProvider returns the Whisper ASR provider if available.
func (s *service) GetWhisperProvider() *WhisperProvider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.providers[ProviderWhisper]; ok {
		if wp, ok := p.(*WhisperProvider); ok {
			return wp
		}
	}
	return nil
}

// PeekWhisperProvider returns the current Whisper ASR provider without creating a new one.
func (s *service) PeekWhisperProvider() *WhisperProvider {
	return s.GetWhisperProvider()
}

// Close releases all provider resources held by the service.
func (s *service) Close() error {
	s.mu.RLock()
	providers := make([]Provider, 0, len(s.providers))
	for _, provider := range s.providers {
		providers = append(providers, provider)
	}
	s.mu.RUnlock()

	for _, provider := range providers {
		if closer, ok := provider.(interface{ Close() }); ok {
			closer.Close()
		}
	}
	return nil
}

// lazyService implements lazy initialization for STT service.
type lazyService struct {
	modelPath string
	idleAfter time.Duration

	mu       sync.Mutex
	service  Service
	provider *WhisperProvider
	timer    *time.Timer
	inUse    int
}

// NewLazyService creates a new STT service that initializes Whisper on first use.
func NewLazyService(modelPath string, idleAfter time.Duration) Service {
	return &lazyService{
		modelPath: modelPath,
		idleAfter: idleAfter,
	}
}

func (s *lazyService) getRuntime() (*WhisperProvider, Service) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}

	if s.provider == nil {
		s.provider = NewWhisperProvider(&WhisperConfig{
			ModelPath:      s.modelPath,
			DeferModelLoad: true,
		})
		s.service = NewServiceWithProvider(s.provider)
	}

	return s.provider, s.service
}

func (s *lazyService) acquireRuntime() (*WhisperProvider, Service, func()) {
	provider, service := s.getRuntime()

	s.mu.Lock()
	s.inUse++
	s.mu.Unlock()

	var once sync.Once
	release := func() {
		once.Do(func() {
			s.release()
		})
	}
	return provider, service, release
}

func (s *lazyService) release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.inUse > 0 {
		s.inUse--
	}
	if s.inUse != 0 || s.provider == nil || s.idleAfter <= 0 {
		return
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(s.idleAfter, s.reclaimIdle)
}

func (s *lazyService) touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inUse != 0 || s.provider == nil || s.idleAfter <= 0 {
		return
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(s.idleAfter, s.reclaimIdle)
}

func (s *lazyService) reclaimIdle() {
	s.mu.Lock()
	if s.inUse != 0 || s.provider == nil {
		s.timer = nil
		s.mu.Unlock()
		return
	}
	provider := s.provider
	s.timer = nil
	s.mu.Unlock()

	provider.Close()
}

func (s *lazyService) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	provider, service, release := s.acquireRuntime()
	defer release()

	if err := ensureWhisperProviderInitialized(provider); err != nil {
		return nil, err
	}
	return service.Transcribe(ctx, req)
}

func (s *lazyService) TranscribeWithProvider(ctx context.Context, providerType ProviderType, req *TranscribeRequest) (*TranscribeResponse, error) {
	provider, service, release := s.acquireRuntime()
	defer release()

	if providerType == ProviderWhisper {
		if err := ensureWhisperProviderInitialized(provider); err != nil {
			return nil, err
		}
	}
	return service.TranscribeWithProvider(ctx, providerType, req)
}

func (s *lazyService) TranscribeStream(ctx context.Context, req *TranscribeRequest, callback StreamCallback) error {
	provider, service, release := s.acquireRuntime()
	defer release()

	if err := ensureWhisperProviderInitialized(provider); err != nil {
		return err
	}
	return service.TranscribeStream(ctx, req, callback)
}

func (s *lazyService) ListProviders() []ProviderType {
	_, service := s.getRuntime()
	s.touch()
	return service.ListProviders()
}

func (s *lazyService) GetDefaultProvider() ProviderType {
	return ProviderWhisper
}

func (s *lazyService) GetWhisperProvider() *WhisperProvider {
	provider, _ := s.getRuntime()
	s.touch()
	return provider
}

func (s *lazyService) PeekWhisperProvider() *WhisperProvider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.provider
}

func (s *lazyService) Close() error {
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	provider := s.provider
	s.inUse = 0
	s.mu.Unlock()

	if provider != nil {
		provider.Close()
	}
	return nil
}

func ensureProviderInitialized(provider Provider) error {
	if whisperProvider, ok := provider.(*WhisperProvider); ok {
		return ensureWhisperProviderInitialized(whisperProvider)
	}
	return nil
}

func ensureWhisperProviderInitialized(provider *WhisperProvider) error {
	if provider == nil {
		return ErrProviderNotFound
	}
	return provider.EnsureInitialized()
}
