package speech

import (
	"context"
	"fmt"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tts"
)

// InitConfig holds configuration for lazy initialization of TTS/STT services.
type InitConfig struct {
	DataDir       string
	OpenAIAPIKey  string
}

// Service provides unified speech functionality (TTS + ASR).
type Service interface {
	// GetStatus returns the unified speech status.
	GetStatus() *StatusResponse
	// GetModels returns all available models.
	GetModels() *ModelsResponse
	// GetASRProvider returns the ASR provider if available.
	GetASRProvider() stt.Provider
	// GetTTSProvider returns the TTS provider if available.
	GetTTSProvider() tts.Provider
	// GetSTTService returns the underlying STT service.
	GetSTTService() stt.Service
	// GetTTSService returns the underlying TTS service.
	GetTTSService() tts.Service
	// IsEditBeforeSendEnabled returns whether edit-before-send is enabled.
	IsEditBeforeSendEnabled() bool
	// Initialize initializes TTS/STT services lazily.
	Initialize() error
	// IsInitialized returns whether services have been initialized.
	IsInitialized() bool
	// SetTTSProvider sets the TTS provider.
	SetTTSProvider(provider tts.Provider)
}

// service implements the Service interface.
type service struct {
	sttService   stt.Service
	ttsService   tts.Service
	asrProvider  stt.Provider
	ttsProvider  tts.Provider
	config       *Config
	initConfig   *InitConfig
	initialized  bool
	initializing bool
	mu           sync.RWMutex
}

// NewService creates a new unified speech service.
func NewService(cfg *Config, sttSvc stt.Service, ttsSvc tts.Service) Service {
	s := &service{
		sttService: sttSvc,
		ttsService: ttsSvc,
		config:     cfg,
	}

	// Providers will be initialized on demand
	return s
}

// NewServiceWithProviders creates a service with explicit providers.
func NewServiceWithProviders(cfg *Config, sttSvc stt.Service, ttsSvc tts.Service, asrProvider stt.Provider, ttsProvider tts.Provider) Service {
	return &service{
		sttService:  sttSvc,
		ttsService:  ttsSvc,
		asrProvider: asrProvider,
		ttsProvider: ttsProvider,
		config:      cfg,
		initialized: asrProvider != nil || ttsProvider != nil,
	}
}

// NewServiceWithInitConfig creates a service with lazy initialization config.
func NewServiceWithInitConfig(cfg *Config, initCfg *InitConfig) Service {
	return &service{
		config:     cfg,
		initConfig: initCfg,
	}
}

// Initialize initializes TTS/STT services lazily.
func (s *service) Initialize() error {
	s.mu.Lock()
	if s.initialized || s.initializing {
		s.mu.Unlock()
		return nil
	}
	s.initializing = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.initializing = false
		s.initialized = true
		s.mu.Unlock()
	}()

	if s.initConfig == nil {
		return fmt.Errorf("no init config provided")
	}

	// Initialize TTS provider based on configuration
	if s.config.TTS.Provider == "" {
		s.config.TTS.Provider = "espeak-ng" // Default to eSpeak-NG
	}

	// TTS provider initialization is handled by ttsService
	// No direct provider creation here anymore

	// Initialize ASR provider based on configuration
	if s.config.ASR.Provider == "" {
		s.config.ASR.Provider = "none" // No default ASR provider
	}

	// Create Sherpa ASR provider if configured
	if s.config.ASR.Provider == "sherpa" {
		modelDir := s.config.ASR.ModelDir
		if modelDir == "" {
			modelDir = s.initConfig.DataDir + "/sherpa-asr"
		}

		modelType := s.config.ASR.Model
		if modelType == "" {
			modelType = "whisper-tiny" // Default model
		}

		sherpaProvider := stt.NewSherpaProvider(&stt.SherpaConfig{
			ModelDir:    modelDir,
			ModelType:   modelType,
			DefaultLang: s.config.ASR.DefaultLang,
			MaxDuration: s.config.ASR.MaxDuration,
		})

		s.asrProvider = sherpaProvider
	}

	return nil
}

// IsInitialized returns whether services have been initialized.
func (s *service) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initialized
}

// GetStatus returns the unified speech status.
func (s *service) GetStatus() *StatusResponse {
	resp := &StatusResponse{
		TTS: TTSStatus{
			Ready:    false,
			Provider: "none",
		},
		ASR: ASRStatus{
			Ready:          false,
			Provider:       "none",
			EditBeforeSend: s.config.ASR.EditBeforeSend,
		},
	}

	// Get TTS status from configured provider
	if s.config.TTS.Provider != "" {
		resp.TTS.Provider = s.config.TTS.Provider
		if s.ttsProvider != nil {
			resp.TTS.Ready = true
			resp.TTS.ModelType = s.ttsProvider.Name()
		}
	}

	// Get ASR status from configured provider
	if s.config.ASR.Provider != "" {
		resp.ASR.Provider = s.config.ASR.Provider
		if s.asrProvider != nil {
			resp.ASR.Ready = true
			resp.ASR.ModelType = s.asrProvider.Name()
		}
	}

	return resp
}

// GetModels returns all available models.
func (s *service) GetModels() *ModelsResponse {
	resp := &ModelsResponse{
		TTS: []ModelInfo{},
		ASR: []ModelInfo{},
	}

	// Get ASR models if provider supports it
	if s.asrProvider != nil {
		if lister, ok := s.asrProvider.(interface{ ListModels() []interface{} }); ok {
			models := lister.ListModels()
			for _, m := range models {
				if modelInfo, ok := m.(ModelInfo); ok {
					resp.ASR = append(resp.ASR, modelInfo)
				}
			}
		}
	}

	// Get TTS models if provider supports it
	if s.ttsProvider != nil {
		if lister, ok := s.ttsProvider.(interface{ ListModels() []interface{} }); ok {
			models := lister.ListModels()
			for _, m := range models {
				if modelInfo, ok := m.(ModelInfo); ok {
					resp.TTS = append(resp.TTS, modelInfo)
				}
			}
		}
	}

	return resp
}

// GetASRProvider returns the ASR provider.
func (s *service) GetASRProvider() stt.Provider {
	return s.asrProvider
}

// GetTTSProvider returns the TTS provider.
func (s *service) GetTTSProvider() tts.Provider {
	return s.ttsProvider
}

// GetSTTService returns the underlying STT service.
func (s *service) GetSTTService() stt.Service {
	return s.sttService
}

// GetTTSService returns the underlying TTS service.
func (s *service) GetTTSService() tts.Service {
	return s.ttsService
}

// IsEditBeforeSendEnabled returns whether edit-before-send is enabled.
func (s *service) IsEditBeforeSendEnabled() bool {
	return s.config.ASR.EditBeforeSend
}

// SetASRProvider sets the ASR provider.
func (s *service) SetASRProvider(provider stt.Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asrProvider = provider
}

// SetTTSProvider sets the TTS provider.
func (s *service) SetTTSProvider(provider tts.Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ttsProvider = provider
}

// Transcribe transcribes audio using the configured ASR provider.
func (s *service) Transcribe(ctx context.Context, req *stt.TranscribeRequest) (*TranscriptionResult, error) {
	var resp *stt.TranscribeResponse
	var err error

	// Use ASR provider if available
	if s.asrProvider != nil {
		if transcriber, ok := s.asrProvider.(interface{ Transcribe(context.Context, *stt.TranscribeRequest) (*stt.TranscribeResponse, error) }); ok {
			resp, err = transcriber.Transcribe(ctx, req)
		}
	} else if s.sttService != nil {
		resp, err = s.sttService.Transcribe(ctx, req)
	} else {
		return nil, ErrNoASRProvider
	}

	if err != nil {
		return nil, err
	}

	return &TranscriptionResult{
		Text:       resp.Text,
		Language:   resp.Language,
		Duration:   resp.Duration,
		Confidence: resp.Confidence,
		Editable:   s.config.ASR.EditBeforeSend,
	}, nil
}

// Error definitions
var (
	ErrNoASRProvider = &SpeechError{Code: "NO_ASR_PROVIDER", Message: "no ASR provider available"}
)

// SpeechError represents a speech-related error.
type SpeechError struct {
	Code    string
	Message string
}

func (e *SpeechError) Error() string {
	return e.Message
}
