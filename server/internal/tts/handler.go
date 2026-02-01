package tts

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
)

// Handler handles TTS management HTTP requests.
type Handler struct {
	service        Service
	sherpaProvider *SherpaProvider
}

// NewHandler creates a new TTS handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// SetSherpaProvider sets the Sherpa provider for model management.
func (h *Handler) SetSherpaProvider(provider *SherpaProvider) {
	h.sherpaProvider = provider
}

// RegisterRoutes registers the TTS management routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/providers", h.ListProviders)
	g.GET("/voices", h.ListVoices)

	// Sherpa model management routes (native Go, no Python)
	sherpa := g.Group("/sherpa")
	sherpa.GET("/status", h.GetSherpaStatus)
	sherpa.GET("/models", h.AvailableModels)
	sherpa.POST("/download", h.DownloadSherpaModel)
	sherpa.POST("/switch", h.SwitchSherpaModel)
	sherpa.DELETE("/model", h.DeleteSherpaModel)

	// Legacy routes for backward compatibility
	kokoro := g.Group("/kokoro")
	kokoro.GET("/status", h.GetSherpaStatus)
	kokoro.POST("/download", h.DownloadSherpaModel)
	kokoro.DELETE("/model", h.DeleteSherpaModel)
}

// ListProviders returns available TTS providers.
func (h *Handler) ListProviders(c echo.Context) error {
	providers := h.service.ListProviders()

	result := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		info := map[string]interface{}{
			"type":    string(p),
			"enabled": true,
		}

		// Add Sherpa-specific info
		if (p == ProviderSherpa || p == ProviderKokoro) && h.sherpaProvider != nil {
			info["model_ready"] = h.sherpaProvider.IsModelReady()
			info["model_dir"] = h.sherpaProvider.GetModelDir()
			info["native"] = true // No Python dependency
		}

		result = append(result, info)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"providers":        result,
		"default_provider": string(h.service.GetDefaultProvider()),
	})
}

// ListVoices returns available TTS voices.
func (h *Handler) ListVoices(c echo.Context) error {
	voices, err := h.service.ListVoices(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"voices": voices,
	})
}

// GetSherpaStatus returns the Sherpa model status.
func (h *Handler) GetSherpaStatus(c echo.Context) error {
	if h.sherpaProvider == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Sherpa provider not available")
	}

	status := h.sherpaProvider.GetModelStatus()

	return c.JSON(http.StatusOK, status)
}

// DownloadSherpaModelRequest represents the download request.
type DownloadSherpaModelRequest struct {
	ModelType string `json:"model_type"` // "kokoro-en", "kokoro-multi", "piper-en", "vits-zh"
}

// DownloadSherpaModel starts downloading the Sherpa model.
func (h *Handler) DownloadSherpaModel(c echo.Context) error {
	if h.sherpaProvider == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Sherpa provider not available")
	}

	var req DownloadSherpaModelRequest
	if err := c.Bind(&req); err != nil {
		req.ModelType = "kokoro-en" // Default to English Kokoro
	}
	if req.ModelType == "" {
		req.ModelType = "kokoro-en"
	}

	// Check if already downloading
	if h.sherpaProvider.GetDownloadManager().IsDownloading() {
		return echo.NewHTTPError(http.StatusConflict, "download already in progress")
	}

	// Start download in background
	go func() {
		ctx := context.Background()
		if err := h.sherpaProvider.GetDownloadManager().Download(ctx, req.ModelType); err != nil {
			// Log error
			_ = err
		} else {
			// Refresh model status after successful download
			h.sherpaProvider.RefreshModelStatus()
		}
	}()

	return c.JSON(http.StatusAccepted, map[string]interface{}{
		"status":     "downloading",
		"message":    "Model download started",
		"model_type": req.ModelType,
	})
}

// DeleteSherpaModel deletes the downloaded Sherpa model.
func (h *Handler) DeleteSherpaModel(c echo.Context) error {
	if h.sherpaProvider == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Sherpa provider not available")
	}

	// Get model directory and delete files
	modelDir := h.sherpaProvider.GetModelDir()
	if modelDir == "" {
		return echo.NewHTTPError(http.StatusInternalServerError, "model directory not configured")
	}

	// Delete model directory
	modelPath := h.sherpaProvider.getModelPath()
	if err := os.RemoveAll(modelPath); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete model: "+err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "deleted",
		"message": "Model files deleted",
	})
}

// SwitchSherpaModel switches to a different TTS model.
func (h *Handler) SwitchSherpaModel(c echo.Context) error {
	if h.sherpaProvider == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Sherpa provider not available")
	}

	var req struct {
		ModelType string `json:"model_type"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	if req.ModelType == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "model_type is required")
	}

	// Validate model type
	validModels := map[string]bool{
		"kokoro-en":    true,
		"kokoro-multi": true,
		"piper-en":     true,
		"vits-zh":      true,
	}
	if !validModels[req.ModelType] {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid model type")
	}

	// Switch the model
	if err := h.sherpaProvider.SwitchModel(req.ModelType); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to switch model: "+err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":     "ok",
		"message":    "Model switched successfully",
		"model_type": req.ModelType,
	})
}

// GetSherpaDownloadProgress returns the current download progress.
func (h *Handler) GetSherpaDownloadProgress(c echo.Context) error {
	if h.sherpaProvider == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Sherpa provider not available")
	}

	mgr := h.sherpaProvider.GetDownloadManager()
	if !mgr.IsDownloading() {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"downloading": false,
		})
	}

	progress := mgr.GetProgress()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"downloading": true,
		"progress":    progress,
	})
}

// AvailableModels returns the list of available models for download.
func (h *Handler) AvailableModels(c echo.Context) error {
	models := []map[string]interface{}{
		{
			"id":          "kokoro-en",
			"name":        "Kokoro English",
			"description": "High-quality English TTS (82M parameters)",
			"languages":   []string{"en-US", "en-GB"},
			"size":        "~100MB",
		},
		{
			"id":          "kokoro-multi",
			"name":        "Kokoro Multilingual",
			"description": "English and Chinese TTS",
			"languages":   []string{"en-US", "zh-CN"},
			"size":        "~150MB",
		},
		{
			"id":          "piper-en",
			"name":        "Piper English",
			"description": "Fast English TTS (Lessac voice)",
			"languages":   []string{"en-US"},
			"size":        "~60MB",
		},
		{
			"id":          "vits-zh",
			"name":        "VITS Chinese",
			"description": "Chinese TTS (AISHELL3)",
			"languages":   []string{"zh-CN"},
			"size":        "~100MB",
		},
	}

	// Check which models are downloaded
	if h.sherpaProvider != nil {
		modelDir := h.sherpaProvider.GetModelDir()
		for i, model := range models {
			modelID := model["id"].(string)
			var checkPath string
			switch modelID {
			case "kokoro-en":
				checkPath = filepath.Join(modelDir, "kokoro-en-v0_19")
			case "kokoro-multi":
				checkPath = filepath.Join(modelDir, "kokoro-multi-lang-v1_0")
			case "piper-en":
				checkPath = filepath.Join(modelDir, "vits-piper-en_US-lessac-medium")
			case "vits-zh":
				checkPath = filepath.Join(modelDir, "vits-zh-aishell3")
			}
			if _, err := os.Stat(checkPath); err == nil {
				models[i]["downloaded"] = true
			} else {
				models[i]["downloaded"] = false
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": models,
	})
}
