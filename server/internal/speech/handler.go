package speech

import (
	"bytes"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tts"
)

// Handler handles unified speech HTTP requests.
type Handler struct {
	service        Service
	espeakManager  *EspeakManager
}

// NewHandler creates a new speech handler.
func NewHandler(svc Service) *Handler {
	return &Handler{
		service:       svc,
		espeakManager: NewEspeakManager("./data"),
	}
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

	// Transcription with edit support
	g.POST("/transcribe", h.Transcribe)
	g.POST("/confirm", h.ConfirmTranscription)

	// eSpeak-NG language pack management
	espeak := g.Group("/espeak")
	espeak.GET("/languages", h.ListEspeakLanguages)
	espeak.POST("/download", h.DownloadEspeakLanguage)
	espeak.POST("/download-all", h.DownloadAllEspeakLanguages)
	espeak.DELETE("/language", h.DeleteEspeakLanguage)
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

	// Return basic status
	return c.JSON(http.StatusOK, map[string]interface{}{
		"ready":    true,
		"provider": provider.Type(),
		"name":     provider.Name(),
	})
}

// ListASRModels returns available ASR models.
func (h *Handler) ListASRModels(c echo.Context) error {
	provider := h.service.GetASRProvider()
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

	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": []interface{}{},
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

	// Current providers don't support model downloads
	return c.JSON(http.StatusBadRequest, map[string]string{
		"error": "current provider does not support model downloads",
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

	// Current providers don't support model switching
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

	return c.JSON(http.StatusOK, map[string]string{
		"status":   "switched",
		"message":  "Switched to provider: " + req.Provider,
		"provider": req.Provider,
	})
}

// ListEspeakLanguages returns available eSpeak-NG language packs.
func (h *Handler) ListEspeakLanguages(c echo.Context) error {
	languages := h.espeakManager.ListLanguagePacks()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"languages": languages,
	})
}

// DownloadEspeakLanguage downloads an eSpeak-NG language pack.
func (h *Handler) DownloadEspeakLanguage(c echo.Context) error {
	var req struct {
		LangCode string `json:"lang_code"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	if err := h.espeakManager.DownloadLanguagePack(req.LangCode); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "downloaded",
		"message": "Language pack downloaded: " + req.LangCode,
	})
}

// DownloadAllEspeakLanguages downloads all eSpeak-NG language packs at once.
func (h *Handler) DownloadAllEspeakLanguages(c echo.Context) error {
	if err := h.espeakManager.DownloadAllLanguagePacks(); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "downloaded",
		"message": "All language packs downloaded successfully",
	})
}

// DeleteEspeakLanguage deletes an eSpeak-NG language pack.
func (h *Handler) DeleteEspeakLanguage(c echo.Context) error {
	langCode := c.QueryParam("lang_code")
	if langCode == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "lang_code is required",
		})
	}

	if err := h.espeakManager.DeleteLanguagePack(langCode); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "deleted",
		"message": "Language pack deleted: " + langCode,
	})
}
