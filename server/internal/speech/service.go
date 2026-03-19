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
	DataDir      string
	OpenAIAPIKey string
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
	// SetEditBeforeSend enables or disables edit-before-send.
	SetEditBeforeSend(enabled bool)
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
	sttService    stt.Service
	ttsService    tts.Service
	asrProvider   stt.Provider
	ttsProvider   tts.Provider
	espeakManager *EspeakManager
	config        *Config
	initConfig    *InitConfig
	initialized   bool
	initializing  bool
	asrPermDenied bool   // macOS STT permission denied
	asrPermError  string // macOS STT permission error message
	mu            sync.RWMutex
}

type serviceSnapshot struct {
	sttService     stt.Service
	ttsService     tts.Service
	asrProvider    stt.Provider
	ttsProvider    tts.Provider
	espeakManager  *EspeakManager
	ttsConfigured  string
	asrConfigured  string
	editBeforeSend bool
	asrPermDenied  bool
	asrPermError   string
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
		// On Windows, always use native STT — whisper is not available in Windows builds
		s.config.ASR.Provider = "windows-native"
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
		fmt.Println("[Speech] Attempting to initialize Windows Native ASR...")
		windowsASR := NewWindowsNativeASR()
		if windowsASR != nil {
			s.asrProvider = windowsASR
			fmt.Println("[Speech] Windows Native ASR initialized successfully")
		} else {
			fmt.Println("[Speech] WARNING: Failed to initialize Windows Native ASR")
			fmt.Println("[Speech] This may be due to:")
			fmt.Println("  1. Windows Speech Recognition not installed")
			fmt.Println("  2. Language pack not installed")
			fmt.Println("  3. CGO not enabled or C++ compiler not available")
			// Keep the provider setting so it shows in available providers
			// but mark as not ready
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

	snap := s.snapshot()

	resp := &StatusResponse{
		TTS: TTSStatus{
			Ready:    false,
			Provider: "none",
		},
		ASR: ASRStatus{
			Ready:          false,
			Provider:       "none",
			EditBeforeSend: snap.editBeforeSend,
		},
	}

	// Get TTS status from configured provider
	if snap.ttsConfigured != "" {
		resp.TTS.Provider = snap.ttsConfigured
		if snap.ttsProvider != nil {
			resp.TTS.Ready = true
			resp.TTS.ModelName = snap.ttsProvider.Name()
		}
	}

	// Populate available TTS providers from the TTS service
	hasEspeak := false
	if snap.ttsService != nil {
		for _, pt := range snap.ttsService.ListProviders() {
			name := string(pt)
			resp.TTS.AvailableProviders = append(resp.TTS.AvailableProviders, name)
			if name == "espeak-ng" {
				hasEspeak = true
			}
		}

		// Populate TTS component download statuses (Kokoro, Vocoder)
		resp.TTS.Components = getTTSComponentStatuses(snap.ttsService)
	}

	// Populate eSpeak status only when espeak-ng is an available provider
	if hasEspeak && snap.espeakManager != nil {
		resp.Espeak = &EspeakStatus{
			Installed:     snap.espeakManager.IsLibraryInstalled(),
			Path:          snap.espeakManager.GetLibraryPath(),
			LanguageCount: snap.espeakManager.LanguageCount(),
			DataSize:      snap.espeakManager.GetDataSize(),
			StaticLinked:  true,
		}
	}

	// Get ASR status from configured provider
	if snap.asrConfigured != "" {
		resp.ASR.Provider = snap.asrConfigured
		if snap.asrProvider != nil {
			resp.ASR.Ready = true
			// Use the actual provider type from the provider object, not the config
			resp.ASR.Provider = string(snap.asrProvider.Type())
			resp.ASR.ModelName = string(snap.asrProvider.Type())

			// Get download status from Whisper provider
			if whisperProvider, ok := snap.asrProvider.(*stt.WhisperProvider); ok {
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
	if snap.asrPermDenied {
		resp.ASR.PermissionDenied = true
		resp.ASR.PermissionError = snap.asrPermError
		resp.ASR.PermissionAppName = GetTCCAppName()
	}

	// Report macOS on-device status
	if snap.asrProvider != nil {
		if macosSTT, ok := snap.asrProvider.(*MacOSNativeSTT); ok {
			resp.ASR.OnDeviceSupported = macosSTT.SupportsOnDevice()
			resp.ASR.OnDeviceOnly = macosSTT.RequireOnDevice()
			resp.ASR.DictationAvailable = macosSTT.DictationAvailable()
		}
		// Report Windows native available languages
		if _, ok := snap.asrProvider.(*windowsNativeASR); ok && snap.ttsService != nil {
			// Get languages from TTS service voices
			if providers := snap.ttsService.ListProviders(); len(providers) > 0 {
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
	resp.ASR.Models = listASRModels(snap.asrProvider, resp)

	// Populate available ASR providers
	resp.ASR.AvailableProviders = s.listAvailableASRProviders()

	// Populate TTS models
	resp.TTS.Models = listTTSModels(snap.ttsProvider)

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
func listASRModels(provider stt.Provider, st *StatusResponse) []interface{} {
	// Native providers don't need model list when ready
	if (st.ASR.Provider == "macos-native" || st.ASR.Provider == "windows-native") && st.ASR.Ready {
		return []interface{}{}
	}

	// Whisper models
	if provider != nil {
		if lister, ok := provider.(interface{ ListModels() []interface{} }); ok {
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
func listTTSModels(provider tts.Provider) []interface{} {
	if provider != nil {
		if lister, ok := provider.(interface{ ListModels() []interface{} }); ok {
			return lister.ListModels()
		}
	}
	return []interface{}{}
}

// getTTSComponentStatuses returns download/readiness status for TTS components
// (Kokoro model, vocoder) so the frontend can poll a single /speech/status endpoint.
// Components that are not compiled in are omitted entirely.
func getTTSComponentStatuses(ttsService tts.Service) map[string]*ComponentDownloadStatus {
	if ttsService == nil {
		return nil
	}
	components := make(map[string]*ComponentDownloadStatus)

	// Kokoro status
	raw := ttsService.GetKokoroStatus()
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
	rawV := ttsService.GetVocoderStatus()
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
	snap := s.snapshot()

	resp := &ModelsResponse{
		TTS: []ModelInfo{},
		ASR: []ModelInfo{},
	}

	// Get ASR models if provider supports it
	if snap.asrProvider != nil {
		if lister, ok := snap.asrProvider.(interface{ ListModels() []interface{} }); ok {
			models := lister.ListModels()
			for _, m := range models {
				if modelInfo, ok := m.(ModelInfo); ok {
					resp.ASR = append(resp.ASR, modelInfo)
				}
			}
		}
	}

	// Get TTS models if provider supports it
	if snap.ttsProvider != nil {
		if lister, ok := snap.ttsProvider.(interface{ ListModels() []interface{} }); ok {
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.asrProvider
}

// GetTTSProvider returns the TTS provider.
func (s *service) GetTTSProvider() tts.Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ttsProvider
}

// GetSTTService returns the underlying STT service.
func (s *service) GetSTTService() stt.Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sttService
}

// GetTTSService returns the underlying TTS service.
func (s *service) GetTTSService() tts.Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ttsService
}

// IsEditBeforeSendEnabled returns whether edit-before-send is enabled.
func (s *service) IsEditBeforeSendEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.ASR.EditBeforeSend
}

// SetEditBeforeSend enables or disables edit-before-send.
func (s *service) SetEditBeforeSend(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.ASR.EditBeforeSend = enabled
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
	snap := s.snapshot()

	var resp *stt.TranscribeResponse
	var err error

	// Use ASR provider if available
	if snap.asrProvider != nil {
		if transcriber, ok := snap.asrProvider.(interface {
			Transcribe(context.Context, *stt.TranscribeRequest) (*stt.TranscribeResponse, error)
		}); ok {
			resp, err = transcriber.Transcribe(ctx, req)
		}
	} else if snap.sttService != nil {
		resp, err = snap.sttService.Transcribe(ctx, req)
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
		Editable:   snap.editBeforeSend,
	}, nil
}

func (s *service) snapshot() serviceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap := serviceSnapshot{
		sttService:    s.sttService,
		ttsService:    s.ttsService,
		asrProvider:   s.asrProvider,
		ttsProvider:   s.ttsProvider,
		espeakManager: s.espeakManager,
		asrPermDenied: s.asrPermDenied,
		asrPermError:  s.asrPermError,
	}
	if s.config != nil {
		snap.ttsConfigured = s.config.TTS.Provider
		snap.asrConfigured = s.config.ASR.Provider
		snap.editBeforeSend = s.config.ASR.EditBeforeSend
	}
	return snap
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
