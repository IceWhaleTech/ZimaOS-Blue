package speech

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/stt"
)

// Handler handles unified speech HTTP requests.
type Handler struct {
	service Service
}

// NewHandler creates a new speech handler.
func NewHandler(svc Service) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes registers unified speech management routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
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

	// Transcription with edit support
	g.POST("/transcribe", h.Transcribe)
	g.POST("/confirm", h.ConfirmTranscription)
}

// GetStatus returns the unified speech status.
func (h *Handler) GetStatus(c echo.Context) error {
	status := h.service.GetStatus()
	return c.JSON(http.StatusOK, status)
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
			"message":  "Sherpa ASR provider not configured",
		})
	}

	status := provider.GetModelStatus()
	return c.JSON(http.StatusOK, status)
}

// ListASRModels returns available ASR models.
func (h *Handler) ListASRModels(c echo.Context) error {
	provider := h.service.GetASRProvider()
	if provider == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"models": []interface{}{},
		})
	}

	models := provider.ListModels()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": models,
	})
}

// DownloadASRModel starts downloading an ASR model.
func (h *Handler) DownloadASRModel(c echo.Context) error {
	provider := h.service.GetASRProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Sherpa ASR provider not configured",
		})
	}

	var req DownloadRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	// Start download in background
	go func() {
		ctx := context.Background()
		provider.DownloadModel(ctx, req.ModelType)
	}()

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "downloading",
		"message": "Download started for model: " + req.ModelType,
	})
}

// SwitchASRModel switches to a different ASR model.
func (h *Handler) SwitchASRModel(c echo.Context) error {
	provider := h.service.GetASRProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Sherpa ASR provider not configured",
		})
	}

	var req SwitchRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	if err := provider.SwitchModel(req.ModelType); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "switched",
		"message": "Switched to model: " + req.ModelType,
	})
}

// DeleteASRModel deletes a downloaded ASR model.
func (h *Handler) DeleteASRModel(c echo.Context) error {
	provider := h.service.GetASRProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Sherpa ASR provider not configured",
		})
	}

	modelType := c.QueryParam("model_type")
	if err := provider.DeleteModel(modelType); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "deleted",
		"message": "Model deleted",
	})
}

// GetTTSStatus returns the TTS model status.
func (h *Handler) GetTTSStatus(c echo.Context) error {
	provider := h.service.GetTTSProvider()
	if provider == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"ready":    false,
			"provider": "none",
			"message":  "Sherpa TTS provider not configured",
		})
	}

	status := provider.GetModelStatus()
	return c.JSON(http.StatusOK, status)
}

// ListTTSModels returns available TTS models.
func (h *Handler) ListTTSModels(c echo.Context) error {
	provider := h.service.GetTTSProvider()
	if provider == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"models": []interface{}{},
		})
	}

	models := provider.ListModels()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": models,
	})
}

// DownloadTTSModel starts downloading a TTS model.
func (h *Handler) DownloadTTSModel(c echo.Context) error {
	provider := h.service.GetTTSProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Sherpa TTS provider not configured",
		})
	}

	var req DownloadRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	// Start download in background
	go func() {
		ctx := context.Background()
		if err := provider.GetDownloadManager().Download(ctx, req.ModelType); err != nil {
			fmt.Printf("TTS model download failed: %v\n", err)
		} else {
			fmt.Printf("TTS model download completed: %s\n", req.ModelType)
			// Refresh model status after successful download
			provider.RefreshModelStatus()
		}
	}()

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "downloading",
		"message": "Download started for model: " + req.ModelType,
	})
}

// SwitchTTSModel switches to a different TTS model.
func (h *Handler) SwitchTTSModel(c echo.Context) error {
	provider := h.service.GetTTSProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Sherpa TTS provider not configured",
		})
	}

	var req SwitchRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	if err := provider.SwitchModel(req.ModelType); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "switched",
		"message": "Switched to model: " + req.ModelType,
	})
}

// DeleteTTSModel deletes a downloaded TTS model.
func (h *Handler) DeleteTTSModel(c echo.Context) error {
	provider := h.service.GetTTSProvider()
	if provider == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Sherpa TTS provider not configured",
		})
	}

	modelType := c.QueryParam("model_type")
	if err := provider.DeleteModel(modelType); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "deleted",
		"message": "Model deleted",
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

	// Use Sherpa ASR if available
	provider := h.service.GetASRProvider()
	if provider != nil && provider.IsModelReady() {
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
