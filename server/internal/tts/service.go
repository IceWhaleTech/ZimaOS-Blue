package tts

import (
	"context"
	"fmt"
	"sync"
)

// service implements the Service interface.
type service struct {
	providers         map[ProviderType]Provider
	defaultProvider   ProviderType
	speed             float32
	pitch             float32
	volume            float32
	dataPath          string
	vocoderManager    *VocoderModelManager
	kokoroManager     *KokoroModelManager
	mu                sync.RWMutex
}

// ServiceConfig holds the configuration for the TTS service.
type ServiceConfig struct {
	DefaultProvider ProviderType
	Providers       []ProviderConfig
	DataPath        string // Path for model downloads
}

// NewService creates a new TTS service.
func NewService(cfg *ServiceConfig) (Service, error) {
	s := &service{
		providers:       make(map[ProviderType]Provider),
		defaultProvider: cfg.DefaultProvider,
		speed:           1.0,
		pitch:           0,
		volume:          1.0,
		dataPath:        cfg.DataPath,
	}

	// Initialize vocoder manager if dataPath provided
	if cfg.DataPath != "" {
		s.vocoderManager = NewVocoderModelManager(cfg.DataPath)
		s.kokoroManager = NewKokoroModelManager(cfg.DataPath)
	}

	// Initialize providers
	for _, providerCfg := range cfg.Providers {
		if !providerCfg.Enabled {
			continue
		}

		provider, err := createProvider(providerCfg, cfg.DataPath)
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
func createProvider(cfg ProviderConfig, dataPath string) (Provider, error) {
	switch cfg.Type {
	case ProviderEspeakNG:
		dp := ""
		if cfg.BaseURL != "" {
			dp = cfg.BaseURL
		}
		return NewEspeakNGAdapter(dp), nil
	case ProviderEdge:
		return NewEdgeTTSProvider(), nil
	case ProviderMacOSNative:
		return NewMacOSNativeTTS(), nil
	case ProviderWindowsNative:
		provider := NewWindowsNativeTTSProvider()
		if provider == nil {
			return nil, fmt.Errorf("failed to create Windows native TTS provider")
		}
		return provider, nil
	case ProviderKokoro:
		return NewKokoroProvider(dataPath), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type)
	}
}

// Synthesize synthesizes text using the default provider.
func (s *service) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	s.mu.RLock()
	defaultProvider := s.defaultProvider
	providersCount := len(s.providers)
	s.mu.RUnlock()

	if providersCount == 0 || defaultProvider == "" {
		return nil, ErrNoProviderConfigured
	}

	return s.SynthesizeWithProvider(ctx, defaultProvider, req)
}

// SynthesizeWithProvider synthesizes text using a specific provider.
func (s *service) SynthesizeWithProvider(ctx context.Context, providerType ProviderType, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	s.mu.RLock()
	provider, ok := s.providers[providerType]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, providerType)
	}

	return provider.Synthesize(ctx, req)
}

// SynthesizeStream synthesizes text with streaming audio output.
func (s *service) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
	s.mu.RLock()
	defaultProvider := s.defaultProvider
	provider, ok := s.providers[defaultProvider]
	s.mu.RUnlock()

	if !ok || defaultProvider == "" {
		return ErrNoProviderConfigured
	}

	return provider.SynthesizeStream(ctx, req, callback)
}

// ListVoices returns available voices from the default provider.
func (s *service) ListVoices(ctx context.Context) ([]Voice, error) {
	s.mu.RLock()
	defaultProvider := s.defaultProvider
	provider, ok := s.providers[defaultProvider]
	s.mu.RUnlock()

	if !ok || defaultProvider == "" {
		return nil, ErrNoProviderConfigured
	}

	return provider.ListVoices(ctx)
}

// ListProviders returns all supported provider types.
func (s *service) ListProviders() []ProviderType {
	result := []ProviderType{ProviderEdge}
	if (&EspeakNGAdapter{}).Available() {
		result = append(result, ProviderEspeakNG)
	}
	if (&MacOSNativeTTS{}).Available() {
		result = append(result, ProviderMacOSNative)
	}
	if WindowsNativeAvailable() {
		result = append(result, ProviderWindowsNative)
	}
	// Kokoro is listed only when compiled in
	if KokoroAvailable() {
		result = append(result, ProviderKokoro)
	}
	return result
}

// GetDefaultProvider returns the default provider type.
func (s *service) GetDefaultProvider() ProviderType {
	return s.defaultProvider
}

// SetDefaultProvider sets the default provider type.
// If the provider is not yet initialized, it will be created on demand.
func (s *service) SetDefaultProvider(providerType ProviderType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.providers[providerType]; !ok {
		// Lazily create the provider on demand
		provider, err := createProvider(ProviderConfig{Type: providerType, Enabled: true}, s.dataPath)
		if err != nil {
			return fmt.Errorf("provider %s not available: %w", providerType, err)
		}
		// Check if the provider is actually functional (e.g. not a stub)
		type availableChecker interface{ Available() bool }
		if ac, ok := provider.(availableChecker); ok && !ac.Available() {
			return fmt.Errorf("provider %s not available: not included in this build", providerType)
		}
		s.providers[providerType] = provider
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

// GetVocoderStatus returns vocoder model status.
func (s *service) GetVocoderStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.vocoderManager == nil {
		return map[string]interface{}{
			"ready":       false,
			"error":       "vocoder manager not initialized",
			"downloading": false,
		}
	}

	return s.vocoderManager.GetModelStatus()
}

// DownloadVocoderModel starts downloading the vocoder model.
func (s *service) DownloadVocoderModel(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.vocoderManager == nil {
		return fmt.Errorf("vocoder manager not initialized")
	}

	return s.vocoderManager.DownloadModel(ctx)
}

// CancelVocoderDownload cancels the vocoder download.
func (s *service) CancelVocoderDownload() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.vocoderManager != nil {
		s.vocoderManager.CancelDownload()
	}
}

// GetKokoroStatus returns Kokoro model status.
func (s *service) GetKokoroStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.kokoroManager == nil {
		return map[string]interface{}{
			"ready":       false,
			"error":       "kokoro manager not initialized",
			"downloading": false,
			"init_stage":  "",
		}
	}

	result := s.kokoroManager.GetModelStatus()

	// Add init_stage from the Kokoro provider if it exists
	if p, ok := s.providers[ProviderKokoro]; ok {
		type initStager interface{ GetInitStage() string }
		if is, ok := p.(initStager); ok {
			result["init_stage"] = is.GetInitStage()
		}
	}

	return result
}

// DownloadKokoroModel starts downloading the Kokoro model.
func (s *service) DownloadKokoroModel(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.kokoroManager == nil {
		return fmt.Errorf("kokoro manager not initialized")
	}

	return s.kokoroManager.DownloadModel(ctx)
}

// CancelKokoroDownload cancels the Kokoro download.
func (s *service) CancelKokoroDownload() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.kokoroManager != nil {
		s.kokoroManager.CancelDownload()
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

	// Close any provider that implements a Close method (e.g. Kokoro idle timer)
	type closer interface{ Close() }
	for pt, p := range s.providers {
		if pt == ProviderEspeakNG {
			continue // already handled above
		}
		if c, ok := p.(closer); ok {
			c.Close()
		}
	}
}
