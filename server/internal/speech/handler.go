package speech

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tts"
)

// kvKeyTTSProvider is the kvstore key for the persisted TTS provider.
const kvKeyTTSProvider = "speech.tts.provider"

// Handler handles unified speech HTTP requests.
type Handler struct {
	service        Service
	espeakManager  *EspeakManager
	kv             kvstore.Store
}

// NewHandler creates a new speech handler.
func NewHandler(svc Service, kv kvstore.Store, dataPath string) *Handler {
	em := NewEspeakManager(dataPath)
	svc.SetEspeakManager(em)
	return &Handler{
		service:       svc,
		espeakManager: em,
		kv:            kv,
	}
}

// GetEspeakManager returns the EspeakManager instance.
func (h *Handler) GetEspeakManager() *EspeakManager {
	return h.espeakManager
}

// GetPersistedTTSProvider returns the persisted TTS provider from kvstore, or empty string if not set.
func (h *Handler) GetPersistedTTSProvider() string {
	if h.kv == nil {
		return ""
	}
	val, err := h.kv.Get(context.Background(), kvKeyTTSProvider)
	if err != nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// RegisterRoutes registers unified speech management routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Lazy initialization endpoint
	g.POST("/init", h.Init)

	// Unified status (includes models)
	g.GET("/status", h.GetStatus)

	// ASR model management
	asr := g.Group("/asr")
	asr.POST("/download", h.DownloadASRModel)
	asr.POST("/download/cancel", h.CancelASRDownload)
	asr.POST("/switch", h.SwitchASRModel)
	asr.POST("/on-device", h.SetASROnDevice)
	asr.GET("/offline-languages", h.GetOfflineLanguages)

	// TTS management
	tts := g.Group("/tts")
	tts.POST("/provider", h.SwitchTTSProvider)
	tts.GET("/config", h.GetTTSConfig)
	tts.POST("/config", h.SetTTSConfig)

	// Transcription
	g.POST("/transcribe", h.Transcribe)

	// eSpeak-NG language list (detailed per-language data; summary status is in /status)
	espeak := g.Group("/espeak")
	espeak.GET("/languages", h.ListEspeakLanguages)

	// Vocoder management (under TTS group)
	tts.POST("/vocoder/download", h.DownloadVocoder)
	tts.POST("/vocoder/download/cancel", h.CancelVocoderDownload)

	// Kokoro model management (under TTS group)
	tts.POST("/kokoro/download", h.DownloadKokoro)
	tts.POST("/kokoro/download/cancel", h.CancelKokoroDownload)
}

// GetStatus returns the unified speech status.
func (h *Handler) GetStatus(c echo.Context) error {
	return c.JSON(http.StatusOK, h.service.GetStatus())
}

// Init initializes TTS/STT services lazily.
func (h *Handler) Init(c echo.Context) error {
	if h.service.IsInitialized() {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "already initialized",
		})
	}

	go func() {
		_ = h.service.Initialize()
	}()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "initialization started",
	})
}

// DownloadASRModel starts downloading an ASR model.
func (h *Handler) DownloadASRModel(c echo.Context) error {
	provider := h.service.GetASRProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "ASR provider not configured",
		})
	}

	var req DownloadRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	// Check if provider is Whisper (supports model downloads)
	whisperProvider, ok := provider.(*stt.WhisperProvider)
	if !ok {
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "ready",
			"message": "Current provider does not require model downloads (system native)",
		})
	}

	// Start download in background with a new context (not tied to request)
	go func() {
		ctx := context.Background()
		_ = whisperProvider.DownloadModel(ctx, req.ModelType)
	}()

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "downloading",
		"message": "Model download started for " + req.ModelType,
	})
}

// CancelASRDownload cancels the current ASR model download.
func (h *Handler) CancelASRDownload(c echo.Context) error {
	provider := h.service.GetASRProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "ASR provider not configured",
		})
	}

	whisperProvider, ok := provider.(*stt.WhisperProvider)
	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "current provider does not support download cancellation",
		})
	}

	whisperProvider.CancelDownload()

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "cancelled",
		"message": "Download cancelled",
	})
}

// SwitchASRModel switches to a different ASR model or provider.
func (h *Handler) SwitchASRModel(c echo.Context) error {
	var req SwitchRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	// Check if switching to a native provider
	if req.ModelType == "windows-native" || req.ModelType == "macos-native" {
		if svc, ok := h.service.(*service); ok {
			svc.mu.Lock()
			svc.config.ASR.Provider = req.ModelType

			// Close old provider if it has Close method
			if svc.asrProvider != nil {
				if closer, ok := svc.asrProvider.(interface{ Close() error }); ok {
					closer.Close()
				}
			}

			// Create new provider
			if req.ModelType == "windows-native" && runtime.GOOS == "windows" {
				svc.asrProvider = NewWindowsNativeASR()
			} else if req.ModelType == "macos-native" && runtime.GOOS == "darwin" {
				macosSTT := NewMacOSNativeSTT()
				if err := macosSTT.Initialize(); err == nil {
					svc.asrProvider = macosSTT
				}
			}
			svc.mu.Unlock()
		}
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "switched",
			"message": "Switched to " + req.ModelType,
		})
	}

	// Otherwise, handle whisper model switching
	provider := h.service.GetASRProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "ASR provider not configured",
		})
	}

	// Already on the requested model
	if string(provider.Type()) == req.ModelType {
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "switched",
			"message": "Already using " + req.ModelType,
		})
	}

	// Check if provider supports model switching
	if switcher, ok := provider.(interface{ SwitchModel(string) error }); ok {
		if err := switcher.SwitchModel(req.ModelType); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		}
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "switched",
			"message": "Switched to model " + req.ModelType,
		})
	}

	return c.JSON(http.StatusBadRequest, map[string]string{
		"error": "current provider does not support model switching",
	})
}

// SetASROnDevice toggles on-device-only mode for macOS native STT.
func (h *Handler) SetASROnDevice(c echo.Context) error {
	provider := h.service.GetASRProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "ASR provider not configured",
		})
	}

	macosSTT, ok := provider.(*MacOSNativeSTT)
	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "on-device mode only available for macOS native STT",
		})
	}

	var req struct {
		OnDeviceOnly bool `json:"on_device_only"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	supportsOnDevice := macosSTT.SupportsOnDevice()
	dictationAvailable := macosSTT.DictationAvailable()
	slog.Info("[speech] SetASROnDevice",
		"on_device_only", req.OnDeviceOnly,
		"supports_on_device", supportsOnDevice,
		"dictation_available", dictationAvailable)

	// When enabling on-device, check prerequisites
	if req.OnDeviceOnly && !dictationAvailable {
		slog.Warn("[speech] on-device requested but Siri/Dictation is disabled")
		return c.JSON(http.StatusOK, map[string]interface{}{
			"on_device_only":      false,
			"on_device_supported": supportsOnDevice,
			"dictation_available": false,
			"error":               "dictation_disabled",
		})
	}

	macosSTT.SetRequireOnDevice(req.OnDeviceOnly)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"on_device_only":      req.OnDeviceOnly,
		"on_device_supported": supportsOnDevice,
		"dictation_available": dictationAvailable,
	})
}

// GetOfflineLanguages returns installed offline dictation languages (macOS/Windows native).
// Separated from /status because it reads system settings which is non-critical info.
func (h *Handler) GetOfflineLanguages(c echo.Context) error {
	provider := h.service.GetASRProvider()
	if provider == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"offline_languages": []string{},
		})
	}

	// Check for macOS native STT
	if macosSTT, ok := provider.(*MacOSNativeSTT); ok {
		langs := macosSTT.OfflineDictationLanguages()
		if langs == nil {
			langs = []string{}
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"offline_languages": langs,
		})
	}

	// Check for Windows native ASR
	if windowsASR, ok := provider.(*windowsNativeASR); ok {
		langs := windowsASR.InstalledLanguages()
		if langs == nil {
			langs = []string{}
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"offline_languages": langs,
		})
	}

	// Other providers don't have offline languages
	return c.JSON(http.StatusOK, map[string]interface{}{
		"offline_languages": []string{},
	})
}

// Transcribe transcribes audio and returns editable result.
func (h *Handler) Transcribe(c echo.Context) error {
	// Get audio file from form
	file, err := c.FormFile("audio")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "audio file required",
		})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to open audio file",
		})
	}
	defer src.Close()

	// Get format and language from form
	format := c.FormValue("format")
	if format == "" {
		format = "wav"
	}
	language := c.FormValue("language")

	// Create transcription request — pass reader directly, Transcribe does io.ReadAll internally
	req := &stt.TranscribeRequest{
		Audio:    src,
		Format:   stt.AudioFormat(format),
		Language: language,
	}

	// Use ASR provider if available
	provider := h.service.GetASRProvider()
	if provider != nil {
		resp, err := provider.Transcribe(c.Request().Context(), req)
		if err != nil {
			var onDeviceErr *OnDeviceUnavailableError
			if errors.As(err, &onDeviceErr) {
				return c.JSON(http.StatusUnprocessableEntity, map[string]string{
					"error":      err.Error(),
					"error_code": "on_device_unavailable",
					"locale":     onDeviceErr.Locale,
				})
			}
			var speechErr *SpeechError
			if errors.As(err, &speechErr) {
				return c.JSON(http.StatusUnprocessableEntity, map[string]string{
					"error":      speechErr.Message,
					"error_code": speechErr.Code,
				})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
		}

		return c.JSON(http.StatusOK, TranscriptionResult{
			Text:       resp.Text,
			Language:   resp.Language,
			Duration:   resp.Duration,
			Confidence: resp.Confidence,
			Editable:   h.service.IsEditBeforeSendEnabled(),
		})
	}

	// Fall back to STT service
	sttSvc := h.service.GetSTTService()
	if sttSvc == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "no ASR service available",
		})
	}

	resp, err := sttSvc.Transcribe(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, TranscriptionResult{
		Text:       resp.Text,
		Language:   resp.Language,
		Duration:   resp.Duration,
		Confidence: resp.Confidence,
		Editable:   h.service.IsEditBeforeSendEnabled(),
	})
}

// SwitchTTSProvider switches the TTS provider.
func (h *Handler) SwitchTTSProvider(c echo.Context) error {
	var req SwitchProviderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	ttsSvc := h.service.GetTTSService()
	if ttsSvc == nil {
		// If TTS service is not available, just save preference
		// The service will use it when initialized
		return c.JSON(http.StatusOK, map[string]string{
			"status":   "switched",
			"message":  "Provider preference saved: " + req.Provider,
			"provider": req.Provider,
		})
	}

	if err := ttsSvc.SetDefaultProvider(tts.ProviderType(req.Provider)); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Update the speech service's TTS provider
	provider := ttsSvc.GetProvider(tts.ProviderType(req.Provider))
	if provider != nil {
		h.service.SetTTSProvider(provider)
	}

	// Persist to kvstore
	if h.kv != nil {
		_ = h.kv.Set(context.Background(), kvKeyTTSProvider, req.Provider, 0)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":   "switched",
		"message":  "Switched to provider: " + req.Provider,
		"provider": req.Provider,
	})
}

// TTSConfigRequest represents a TTS config update request.
type TTSConfigRequest struct {
	Speed  float32 `json:"speed"`
	Pitch  float32 `json:"pitch"`
	Volume float32 `json:"volume"`
}

// GetTTSConfig returns the current TTS configuration.
func (h *Handler) GetTTSConfig(c echo.Context) error {
	ttsSvc := h.service.GetTTSService()
	if ttsSvc == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"speed":  1.0,
			"pitch":  0.0,
			"volume": 100.0,
		})
	}

	speed, pitch, volume := ttsSvc.GetConfig()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"speed":  speed,
		"pitch":  pitch,
		"volume": volume,
	})
}

// SetTTSConfig updates the TTS configuration.
func (h *Handler) SetTTSConfig(c echo.Context) error {
	var req TTSConfigRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	ttsSvc := h.service.GetTTSService()
	if ttsSvc == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "TTS service not available",
		})
	}

	ttsSvc.SetConfig(req.Speed, req.Pitch, req.Volume)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "updated",
		"speed":  req.Speed,
		"pitch":  req.Pitch,
		"volume": req.Volume,
	})
}

// ListEspeakLanguages returns available eSpeak-NG language dictionaries
// by scanning the real espeak-ng-data directory.
func (h *Handler) ListEspeakLanguages(c echo.Context) error {
	languages := h.espeakManager.ListLanguagePacks()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"languages": languages,
		"count":     len(languages),
	})
}

// DownloadVocoder starts downloading the vocoder model.
func (h *Handler) DownloadVocoder(c echo.Context) error {
	ctx := c.Request().Context()
	if err := h.service.GetTTSService().DownloadVocoderModel(ctx); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "downloading",
		"message": "vocoder model download started",
	})
}

// CancelVocoderDownload cancels the vocoder download.
func (h *Handler) CancelVocoderDownload(c echo.Context) error {
	h.service.GetTTSService().CancelVocoderDownload()
	return c.JSON(http.StatusOK, map[string]string{
		"status":  "cancelled",
		"message": "vocoder download cancelled",
	})
}

// DownloadKokoro starts downloading the Kokoro model.
func (h *Handler) DownloadKokoro(c echo.Context) error {
	go func() {
		ctx := context.Background()
		if err := h.service.GetTTSService().DownloadKokoroModel(ctx); err != nil {
			// Error is stored in model manager state, polled via GetKokoroStatus
			_ = err
		}
	}()

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "downloading",
		"message": "Kokoro model download started",
	})
}

// CancelKokoroDownload cancels the Kokoro download.
func (h *Handler) CancelKokoroDownload(c echo.Context) error {
	h.service.GetTTSService().CancelKokoroDownload()
	return c.JSON(http.StatusOK, map[string]string{
		"status":  "cancelled",
		"message": "Kokoro download cancelled",
	})
}
