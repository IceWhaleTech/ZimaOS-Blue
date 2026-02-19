package speech

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
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
func NewHandler(svc Service, kv kvstore.Store) *Handler {
	return &Handler{
		service:       svc,
		espeakManager: NewEspeakManager("./data"),
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

	// Unified status and models
	g.GET("/status", h.GetStatus)
	g.GET("/models", h.GetModels)

	// ASR model management
	asr := g.Group("/asr")
	asr.GET("/status", h.GetASRStatus)
	asr.GET("/models", h.ListASRModels)
	asr.POST("/download", h.DownloadASRModel)
	asr.POST("/download/cancel", h.CancelASRDownload)
	asr.POST("/switch", h.SwitchASRModel)
	asr.DELETE("/model", h.DeleteASRModel)

	// TTS model management (Sherpa)
	tts := g.Group("/tts")
	tts.GET("/status", h.GetTTSStatus)
	tts.GET("/models", h.ListTTSModels)
	tts.POST("/download", h.DownloadTTSModel)
	tts.POST("/switch", h.SwitchTTSModel)
	tts.DELETE("/model", h.DeleteTTSModel)
	tts.POST("/provider", h.SwitchTTSProvider)
	tts.GET("/config", h.GetTTSConfig)
	tts.POST("/config", h.SetTTSConfig)

	// Transcription with edit support
	g.POST("/transcribe", h.Transcribe)
	g.POST("/confirm", h.ConfirmTranscription)

	// eSpeak-NG status (engine is statically linked, data dir detected at runtime)
	espeak := g.Group("/espeak")
	espeak.GET("/languages", h.ListEspeakLanguages)
	espeak.GET("/library/status", h.GetEspeakLibraryStatus)

	// Vocoder management
	vocoder := g.Group("/vocoder")
	vocoder.GET("/status", h.GetVocoderStatus)
	vocoder.POST("/download", h.DownloadVocoder)
	vocoder.POST("/download/cancel", h.CancelVocoderDownload)

	// Kokoro model management
	kokoro := g.Group("/kokoro")
	kokoro.GET("/status", h.GetKokoroStatus)
	kokoro.POST("/download", h.DownloadKokoro)
	kokoro.POST("/download/cancel", h.CancelKokoroDownload)
}

// GetStatus returns the unified speech status.
func (h *Handler) GetStatus(c echo.Context) error {
	status := h.service.GetStatus()
	return c.JSON(http.StatusOK, status)
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

// GetModels returns all available models.
func (h *Handler) GetModels(c echo.Context) error {
	models := h.service.GetModels()
	return c.JSON(http.StatusOK, models)
}

// GetASRStatus returns the ASR model status.
func (h *Handler) GetASRStatus(c echo.Context) error {
	provider := h.service.GetASRProvider()
	if provider == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"ready":    false,
			"provider": "none",
			"message":  "ASR provider not configured",
		})
	}

	// Check if provider is Whisper (has detailed status with download progress)
	if whisperProvider, ok := provider.(*stt.WhisperProvider); ok {
		status := whisperProvider.GetModelStatus()
		return c.JSON(http.StatusOK, status)
	}

	// Return basic status for other providers
	return c.JSON(http.StatusOK, map[string]interface{}{
		"ready":    true,
		"provider": provider.Type(),
		"name":     provider.Name(),
	})
}

// ListASRModels returns available ASR models.
func (h *Handler) ListASRModels(c echo.Context) error {
	provider := h.service.GetASRProvider()

	var providerInfo string
	if provider != nil {
		providerInfo = fmt.Sprintf("type=%s name=%s", provider.Type(), provider.Name())
	} else {
		providerInfo = "nil"
	}
	fmt.Printf("[ListASRModels] provider: %s\n", providerInfo)

	var models []interface{}

	// If macOS native STT is active, include it as a built-in model entry
	if provider != nil && provider.Type() == ProviderMacOSNative {
		models = append(models, map[string]interface{}{
			"id":          "macos-native",
			"name":        "speech.asrModelInfo.macosNative.name",
			"description": "speech.asrModelInfo.macosNative.description",
			"size":        "",
			"downloaded":  true,
			"active":      true,
		})
	}

	// If provider supports ListModels (e.g. Whisper), add those too
	if provider != nil {
		if lister, ok := provider.(interface{ ListModels() []interface{} }); ok {
			models = append(models, lister.ListModels()...)
			return c.JSON(http.StatusOK, map[string]interface{}{
				"models": models,
			})
		}
	}

	// Add available Whisper models from metadata
	for _, m := range stt.GetAvailableASRModels() {
		models = append(models, m)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": models,
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
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "current provider does not support model downloads",
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

// SwitchASRModel switches to a different ASR model.
func (h *Handler) SwitchASRModel(c echo.Context) error {
	provider := h.service.GetASRProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "ASR provider not configured",
		})
	}

	var req SwitchRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
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

// DeleteASRModel deletes a downloaded ASR model.
func (h *Handler) DeleteASRModel(c echo.Context) error {
	provider := h.service.GetASRProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "ASR provider not configured",
		})
	}

	// Current providers don't support model deletion
	return c.JSON(http.StatusBadRequest, map[string]string{
		"error": "current provider does not support model deletion",
	})
}

// GetTTSStatus returns the TTS model status.
func (h *Handler) GetTTSStatus(c echo.Context) error {
	provider := h.service.GetTTSProvider()
	if provider == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"ready":    false,
			"provider": "none",
			"message":  "TTS provider not configured",
		})
	}

	// Return basic status for all providers
	return c.JSON(http.StatusOK, map[string]interface{}{
		"ready":    true,
		"provider": provider.Type(),
		"name":     provider.Name(),
	})
}

// ListTTSModels returns available TTS models.
func (h *Handler) ListTTSModels(c echo.Context) error {
	provider := h.service.GetTTSProvider()
	if provider == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"models": []interface{}{},
		})
	}

	// For providers that support model listing
	if lister, ok := provider.(interface{ ListModels() []interface{} }); ok {
		models := lister.ListModels()
		return c.JSON(http.StatusOK, map[string]interface{}{
			"models": models,
		})
	}

	// For other providers, return empty models list
	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": []interface{}{},
	})
}

// DownloadTTSModel starts downloading a TTS model.
func (h *Handler) DownloadTTSModel(c echo.Context) error {
	provider := h.service.GetTTSProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "TTS provider not configured",
		})
	}

	var req DownloadRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	// Current providers don't support model downloads
	return c.JSON(http.StatusBadRequest, map[string]string{
		"error": "current provider does not support model downloads",
	})
}

// SwitchTTSModel switches to a different TTS model.
func (h *Handler) SwitchTTSModel(c echo.Context) error {
	provider := h.service.GetTTSProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "TTS provider not configured",
		})
	}

	var req SwitchRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	// Current providers don't support model switching
	return c.JSON(http.StatusBadRequest, map[string]string{
		"error": "current provider does not support model switching",
	})
}

// DeleteTTSModel deletes a downloaded TTS model.
func (h *Handler) DeleteTTSModel(c echo.Context) error {
	provider := h.service.GetTTSProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "TTS provider not configured",
		})
	}

	// Current providers don't support model deletion
	return c.JSON(http.StatusBadRequest, map[string]string{
		"error": "current provider does not support model deletion",
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

	// Read audio data
	audioData, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to read audio data",
		})
	}

	// Get format and language from form
	format := c.FormValue("format")
	if format == "" {
		format = "wav"
	}
	language := c.FormValue("language")

	// Create transcription request
	req := &stt.TranscribeRequest{
		Audio:    bytes.NewReader(audioData),
		Format:   stt.AudioFormat(format),
		Language: language,
	}

	// Use ASR provider if available
	provider := h.service.GetASRProvider()
	if provider != nil {
		resp, err := provider.Transcribe(c.Request().Context(), req)
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

// ConfirmTranscription confirms edited transcription.
func (h *Handler) ConfirmTranscription(c echo.Context) error {
	var req ConfirmRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	// For now, just return success
	// In a full implementation, this would send the text to the chat
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "confirmed",
		"text":   req.Text,
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

// GetEspeakLibraryStatus returns whether the espeak-ng-data directory
// was found and how many language dictionaries are available.
func (h *Handler) GetEspeakLibraryStatus(c echo.Context) error {
	installed := h.espeakManager.IsLibraryInstalled()
	dataPath := h.espeakManager.GetLibraryPath()
	langCount := h.espeakManager.LanguageCount()
	dataSize := h.espeakManager.GetDataSize()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"installed":      installed,
		"path":           dataPath,
		"language_count": langCount,
		"data_size":      dataSize,
		"static_linked":  true, // engine is always statically linked via CGO
	})
}

// GetVocoderStatus returns the vocoder model status.
func (h *Handler) GetVocoderStatus(c echo.Context) error {
	status := h.service.GetTTSService().GetVocoderStatus()
	return c.JSON(http.StatusOK, status)
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

// GetKokoroStatus returns the Kokoro model status.
func (h *Handler) GetKokoroStatus(c echo.Context) error {
	raw := h.service.GetTTSService().GetKokoroStatus()
	// Flatten progress struct for frontend
	resp := map[string]interface{}{
		"ready":       raw["ready"],
		"downloading": raw["downloading"],
		"error":       raw["error"],
		"init_stage":  raw["init_stage"],
	}
	if p, ok := raw["progress"].(*downloader.DownloadProgress); ok && p != nil {
		resp["progress"] = p.Percentage
		resp["speed"] = p.SpeedHuman
		resp["eta"] = p.ETA
		resp["file"] = p.File
		resp["file_index"] = p.FileIndex
		resp["total_files"] = p.TotalFiles
		resp["downloaded_human"] = p.DownloadedHuman
	} else {
		resp["progress"] = 0
	}
	return c.JSON(http.StatusOK, resp)
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
