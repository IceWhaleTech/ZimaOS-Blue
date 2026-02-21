package speech

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tts"
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
	// SetASRProvider sets the ASR provider.
	SetASRProvider(provider stt.Provider)
	// SetASRPermissionDenied records that ASR permission was denied (e.g. macOS TCC).
	SetASRPermissionDenied(errMsg string)
	// SetEspeakManager sets the eSpeak manager for status reporting.
	SetEspeakManager(em *EspeakManager)
}

// service implements the Service interface.
type service struct {
	sttService       stt.Service
	ttsService       tts.Service
	asrProvider      stt.Provider
	ttsProvider      tts.Provider
	espeakManager    *EspeakManager
	config           *Config
	initConfig       *InitConfig
	initialized      bool
	initializing     bool
	asrPermDenied    bool   // macOS STT permission denied
	asrPermError     string // macOS STT permission error message
	mu               sync.RWMutex
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
		if runtime.GOOS == "darwin" {
			s.config.TTS.Provider = "macos-native"
		} else if runtime.GOOS == "windows" {
			s.config.TTS.Provider = "windows-native"
		} else {
			s.config.TTS.Provider = "edge-tts"
		}
	}

	// TTS provider initialization is handled by ttsService
	// No direct provider creation here anymore

	// Initialize ASR provider based on configuration
	// On macOS, always use native STT — whisper is not available in macOS builds
	if runtime.GOOS == "darwin" {
		s.config.ASR.Provider = "macos-native"
	} else if runtime.GOOS == "windows" {
		if s.config.ASR.Provider == "" {
			s.config.ASR.Provider = "windows-native"
		}
	} else if s.config.ASR.Provider == "" {
		s.config.ASR.Provider = "none"
	}

	// Create macOS native ASR provider if configured
	if s.config.ASR.Provider == "macos-native" && runtime.GOOS == "darwin" {
		macosSTT := NewMacOSNativeSTT()
		if err := macosSTT.Initialize(); err == nil {
			s.asrProvider = macosSTT
		} else {
			s.asrPermDenied = true
			s.asrPermError = err.Error()
		}
	}

	// Create Windows native ASR provider if configured
	if s.config.ASR.Provider == "windows-native" && runtime.GOOS == "windows" {
		windowsASR := NewWindowsNativeASR()
		if windowsASR != nil {
			s.asrProvider = windowsASR
		}
	}

	// Create Sherpa ASR provider if configured
	if s.config.ASR.Provider == "sherpa" {
		modelDir := s.config.ASR.ModelDir
		if modelDir == "" {
			modelDir = s.initConfig.DataDir + "/whisper-models"
		}

		modelType := s.config.ASR.Model
		if modelType == "" {
			modelType = "whisper-tiny" // Default model
		}

		whisperProvider := stt.NewWhisperProvider(&stt.WhisperConfig{
			ModelPath:   modelDir,
			DefaultLang: s.config.ASR.DefaultLang,
			MaxDuration: s.config.ASR.MaxDuration,
		})

		s.asrProvider = whisperProvider
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
	// Ensure services are initialized so permission/readiness state is accurate
	if !s.IsInitialized() && s.initConfig != nil {
		_ = s.Initialize()
	}

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
			resp.TTS.ModelName = s.ttsProvider.Name()
		}
	}

	// Populate available TTS providers from the TTS service
	hasEspeak := false
	if s.ttsService != nil {
		for _, pt := range s.ttsService.ListProviders() {
			name := string(pt)
			resp.TTS.AvailableProviders = append(resp.TTS.AvailableProviders, name)
			if name == "espeak-ng" {
				hasEspeak = true
			}
		}

		// Populate TTS component download statuses (Kokoro, Vocoder)
		resp.TTS.Components = s.getTTSComponentStatuses()
	}

	// Populate eSpeak status only when espeak-ng is an available provider
	if hasEspeak && s.espeakManager != nil {
		resp.Espeak = &EspeakStatus{
			Installed:     s.espeakManager.IsLibraryInstalled(),
			Path:          s.espeakManager.GetLibraryPath(),
			LanguageCount: s.espeakManager.LanguageCount(),
			DataSize:      s.espeakManager.GetDataSize(),
			StaticLinked:  true,
		}
	}

	// Get ASR status from configured provider
	if s.config.ASR.Provider != "" {
		resp.ASR.Provider = s.config.ASR.Provider
		if s.asrProvider != nil {
			resp.ASR.Ready = true
			resp.ASR.ModelName = string(s.asrProvider.Type())

			// Get download status from Whisper provider
			if whisperProvider, ok := s.asrProvider.(*stt.WhisperProvider); ok {
				status := whisperProvider.GetModelStatus()
				resp.ASR.Downloading = status.Downloading
				resp.ASR.HasPending = status.HasPending
				// Use the actual model name from status (e.g., "whisper-tiny")
				if status.ModelType != "" {
					resp.ASR.ModelName = status.ModelType
				}
				if status.Downloading && status.Progress != nil {
					// Add to downloads list
					resp.ASR.Downloads = []DownloadStatus{{
						ModelType: status.ModelType,
						Progress: Progress{
							File:       status.Progress.File,
							Downloaded: status.Progress.Downloaded,
							Total:      status.Progress.Total,
							Percentage: status.Progress.Percentage,
							SpeedHuman: status.Progress.SpeedHuman,
							ETA:        status.Progress.ETA,
						},
					}}
					resp.ASR.ModelName = status.ModelType
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
		}
	}

	// Report macOS permission denied state
	if s.asrPermDenied {
		resp.ASR.PermissionDenied = true
		resp.ASR.PermissionError = s.asrPermError
		resp.ASR.PermissionAppName = GetTCCAppName()
	}

	// Report macOS on-device status
	if s.asrProvider != nil {
		if macosSTT, ok := s.asrProvider.(*MacOSNativeSTT); ok {
			resp.ASR.OnDeviceSupported = macosSTT.SupportsOnDevice()
			resp.ASR.OnDeviceOnly = macosSTT.RequireOnDevice()
			resp.ASR.DictationAvailable = macosSTT.DictationAvailable()
		}
		// Report Windows native available languages
		if _, ok := s.asrProvider.(*windowsNativeASR); ok && s.ttsService != nil {
			// Get languages from TTS service voices
			if providers := s.ttsService.ListProviders(); len(providers) > 0 {
				for _, pt := range providers {
					if string(pt) == "windows-native" {
						// Windows native languages will be shown
						resp.ASR.OfflineLanguages = []string{"en-US", "zh-CN", "ja-JP", "ko-KR"}
						break
					}
				}
			}
		}
	}

	// Populate ASR models
	resp.ASR.Models = s.listASRModels(resp)

	// Populate available ASR providers
	resp.ASR.AvailableProviders = s.listAvailableASRProviders()

	// Populate TTS models
	resp.TTS.Models = s.listTTSModels()

	return resp
}

// listAvailableASRProviders returns available ASR providers based on platform.
func (s *service) listAvailableASRProviders() []string {
	providers := []string{}

	// Add platform-specific native providers
	if runtime.GOOS == "darwin" {
		providers = append(providers, "macos-native")
	} else if runtime.GOOS == "windows" {
		providers = append(providers, "windows-native")
	}

	// Whisper is only available on Linux
	if runtime.GOOS == "linux" {
		providers = append(providers, "whisper")
	}

	return providers
}

// listASRModels returns ASR models for the status response.
// Native providers (macOS/Windows) return empty list when ready.
func (s *service) listASRModels(st *StatusResponse) []interface{} {
	// Native providers don't need model list when ready
	if (st.ASR.Provider == "macos-native" || st.ASR.Provider == "windows-native") && st.ASR.Ready {
		return []interface{}{}
	}

	// Whisper models
	if s.asrProvider != nil {
		if lister, ok := s.asrProvider.(interface{ ListModels() []interface{} }); ok {
			return lister.ListModels()
		}
	}
	// Fallback: available whisper models from metadata
	models := make([]interface{}, 0, len(stt.GetAvailableASRModels()))
	for _, m := range stt.GetAvailableASRModels() {
		models = append(models, m)
	}
	return models
}

// listTTSModels returns TTS models for the status response.
func (s *service) listTTSModels() []interface{} {
	if s.ttsProvider != nil {
		if lister, ok := s.ttsProvider.(interface{ ListModels() []interface{} }); ok {
			return lister.ListModels()
		}
	}
	return []interface{}{}
}

// getTTSComponentStatuses returns download/readiness status for TTS components
// (Kokoro model, vocoder) so the frontend can poll a single /speech/status endpoint.
// Components that are not compiled in are omitted entirely.
func (s *service) getTTSComponentStatuses() map[string]*ComponentDownloadStatus {
	if s.ttsService == nil {
		return nil
	}
	components := make(map[string]*ComponentDownloadStatus)

	// Kokoro status
	raw := s.ttsService.GetKokoroStatus()
	kokoro := &ComponentDownloadStatus{}
	if v, ok := raw["ready"].(bool); ok {
		kokoro.Ready = v
	}
	if v, ok := raw["downloading"].(bool); ok {
		kokoro.Downloading = v
	}
	if v, ok := raw["error"].(string); ok {
		kokoro.Error = v
	}
	if v, ok := raw["init_stage"].(string); ok {
		kokoro.InitStage = v
	}
	if p, ok := raw["progress"].(*downloader.DownloadProgress); ok && p != nil {
		kokoro.Progress = p.Percentage
		kokoro.Speed = p.SpeedHuman
		kokoro.ETA = p.ETA
		kokoro.File = p.File
		kokoro.FileIndex = p.FileIndex
		kokoro.TotalFiles = p.TotalFiles
		kokoro.DownloadedSize = p.DownloadedHuman
	}
	if !isNotCompiledIn(kokoro.Error) {
		components["kokoro"] = kokoro
	}

	// Vocoder status
	rawV := s.ttsService.GetVocoderStatus()
	vocoder := &ComponentDownloadStatus{}
	if v, ok := rawV["ready"].(bool); ok {
		vocoder.Ready = v
	}
	if v, ok := rawV["downloading"].(bool); ok {
		vocoder.Downloading = v
	}
	if v, ok := rawV["error"].(string); ok {
		vocoder.Error = v
	}
	if !isNotCompiledIn(vocoder.Error) {
		components["vocoder"] = vocoder
	}

	if len(components) == 0 {
		return nil
	}
	return components
}

// isNotCompiledIn returns true if the error string indicates the component
// was not compiled into this binary (e.g. "not compiled in").
func isNotCompiledIn(errMsg string) bool {
	return strings.Contains(errMsg, "not compiled in")
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

// SetASRProvider sets the ASR provider and updates the config to match.
func (s *service) SetASRProvider(provider stt.Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asrProvider = provider
	if provider != nil {
		s.config.ASR.Provider = string(provider.Type())
	}
}

// SetTTSProvider sets the TTS provider.
func (s *service) SetTTSProvider(provider tts.Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ttsProvider = provider
}

// SetASRPermissionDenied records that ASR permission was denied.
func (s *service) SetASRPermissionDenied(errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asrPermDenied = true
	s.asrPermError = errMsg
}

// SetEspeakManager sets the eSpeak manager for status reporting.
func (s *service) SetEspeakManager(em *EspeakManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.espeakManager = em
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
