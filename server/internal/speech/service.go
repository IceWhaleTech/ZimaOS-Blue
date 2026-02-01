package speech

import (
	"context"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tts"
)

// Service provides unified speech functionality (TTS + ASR).
type Service interface {
	// GetStatus returns the unified speech status.
	GetStatus() *StatusResponse
	// GetModels returns all available models.
	GetModels() *ModelsResponse
	// GetASRProvider returns the Sherpa ASR provider if available.
	GetASRProvider() *stt.SherpaProvider
	// GetTTSProvider returns the Sherpa TTS provider if available.
	GetTTSProvider() *tts.SherpaProvider
	// GetSTTService returns the underlying STT service.
	GetSTTService() stt.Service
	// GetTTSService returns the underlying TTS service.
	GetTTSService() tts.Service
	// IsEditBeforeSendEnabled returns whether edit-before-send is enabled.
	IsEditBeforeSendEnabled() bool
}

// service implements the Service interface.
type service struct {
	sttService     stt.Service
	ttsService     tts.Service
	sherpaASR      *stt.SherpaProvider
	sherpaTTS      *tts.SherpaProvider
	config         *Config
	mu             sync.RWMutex
}

// NewService creates a new unified speech service.
func NewService(cfg *Config, sttSvc stt.Service, ttsSvc tts.Service) Service {
	s := &service{
		sttService: sttSvc,
		ttsService: ttsSvc,
		config:     cfg,
	}

	// Try to get Sherpa providers if they exist
	// This would require type assertion from the service
	// For now, we'll create them directly if configured

	return s
}

// NewServiceWithProviders creates a service with explicit Sherpa providers.
func NewServiceWithProviders(cfg *Config, sttSvc stt.Service, ttsSvc tts.Service, sherpaASR *stt.SherpaProvider, sherpaTTS *tts.SherpaProvider) Service {
	return &service{
		sttService: sttSvc,
		ttsService: ttsSvc,
		sherpaASR:  sherpaASR,
		sherpaTTS:  sherpaTTS,
		config:     cfg,
	}
}

// GetStatus returns the unified speech status.
func (s *service) GetStatus() *StatusResponse {
	resp := &StatusResponse{
		TTS: TTSStatus{
			Ready:    true,
			Provider: "default",
		},
		ASR: ASRStatus{
			Ready:          false,
			Provider:       "none",
			EditBeforeSend: s.config.ASR.EditBeforeSend,
		},
	}

	// Get TTS status
	if s.sherpaTTS != nil {
		status := s.sherpaTTS.GetModelStatus()
		resp.TTS.Ready = status.Ready
		resp.TTS.Provider = "sherpa"
		resp.TTS.ModelType = status.ModelType
	}

	// Get ASR status
	if s.sherpaASR != nil {
		status := s.sherpaASR.GetModelStatus()
		resp.ASR.Ready = status.Ready
		resp.ASR.Provider = "sherpa"
		resp.ASR.ModelType = status.ModelType
		resp.ASR.StreamingSupported = status.StreamingSupported
		resp.ASR.Downloading = status.Downloading
		resp.ASR.HasPending = status.HasPending

		if status.Progress != nil {
			resp.ASR.Progress = &Progress{
				File:       status.Progress.File,
				Downloaded: status.Progress.Downloaded,
				Total:      status.Progress.Total,
				Percentage: status.Progress.Percentage,
				SpeedHuman: status.Progress.SpeedHuman,
				ETA:        status.Progress.ETA,
			}
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

	// Get ASR models
	if s.sherpaASR != nil {
		models := s.sherpaASR.ListModels()
		for _, m := range models {
			resp.ASR = append(resp.ASR, ModelInfo{
				ID:          m.ID,
				Name:        m.Name,
				Description: m.Description,
				Type:        "asr",
				Languages:   m.Languages,
				Size:        m.Size,
				Streaming:   m.Streaming,
				Downloaded:  m.Downloaded,
			})
		}
	}

	// Get TTS models (if Sherpa TTS has similar method)
	// For now, return empty TTS models list

	return resp
}

// GetASRProvider returns the Sherpa ASR provider.
func (s *service) GetASRProvider() *stt.SherpaProvider {
	return s.sherpaASR
}

// GetTTSProvider returns the Sherpa TTS provider.
func (s *service) GetTTSProvider() *tts.SherpaProvider {
	return s.sherpaTTS
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

// SetASRProvider sets the Sherpa ASR provider.
func (s *service) SetASRProvider(provider *stt.SherpaProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sherpaASR = provider
}

// SetTTSProvider sets the Sherpa TTS provider.
func (s *service) SetTTSProvider(provider *tts.SherpaProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sherpaTTS = provider
}

// Transcribe transcribes audio using the configured ASR provider.
func (s *service) Transcribe(ctx context.Context, req *stt.TranscribeRequest) (*TranscriptionResult, error) {
	var resp *stt.TranscribeResponse
	var err error

	// Use Sherpa ASR if available and ready
	if s.sherpaASR != nil && s.sherpaASR.IsModelReady() {
		resp, err = s.sherpaASR.Transcribe(ctx, req)
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
