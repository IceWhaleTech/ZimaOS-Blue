package setup

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/providers"
)

// ProviderDetectionResult contains the results of provider auto-detection.
type ProviderDetectionResult struct {
	Ollama      *providers.OllamaInfo       `json:"ollama"`
	Environment *providers.EnvConfigDisplay `json:"environment"`
	CCSwitch    *CCSwitchInfo               `json:"cc_switch"`
	CLI         *claudecode.CLIStatus       `json:"cli"`
}

// CCSwitchInfo contains cc-switch detection results.
type CCSwitchInfo struct {
	Available     bool                      `json:"available"`
	ActiveProfile string                    `json:"active_profile,omitempty"`
	Profiles      []providers.ProfileDisplay `json:"profiles,omitempty"`
}

// ProviderHandler handles provider detection API requests.
type ProviderHandler struct {
	ollamaDetector *providers.OllamaDetector
	envReader      *providers.EnvironmentReader
	ccSwitch       *providers.CCSwitchIntegration
	downloadMgr    *claudecode.DownloadManager
}

// NewProviderHandler creates a new provider handler.
func NewProviderHandler(cacheDir string) *ProviderHandler {
	return &ProviderHandler{
		ollamaDetector: providers.NewOllamaDetector(),
		envReader:      providers.NewEnvironmentReader(),
		ccSwitch:       providers.NewCCSwitchIntegration(),
		downloadMgr:    claudecode.NewDownloadManager(cacheDir),
	}
}

// RegisterRoutes registers provider detection routes.
func (h *ProviderHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/providers")
	g.GET("/detect", h.DetectAll)
	g.GET("/ollama", h.GetOllama)
	g.GET("/ollama/models", h.GetOllamaModels)
	g.GET("/env", h.GetEnvConfig)
	g.GET("/cc-switch/profiles", h.GetCCSwitchProfiles)
	g.POST("/cc-switch/activate", h.ActivateCCSwitchProfile)

	// CLI routes
	cli := e.Group("/api/v1/cli")
	cli.GET("/status", h.GetCLIStatus)
	cli.GET("/download/info", h.GetDownloadInfo)
	cli.POST("/download", h.StartDownload)
	cli.GET("/download/progress", h.GetDownloadProgress)
	cli.DELETE("/download", h.CancelDownload)
}

// DetectAll detects all available providers.
func (h *ProviderHandler) DetectAll(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	result := &ProviderDetectionResult{}

	// Detect Ollama
	ollamaInfo, _ := h.ollamaDetector.Detect(ctx)
	result.Ollama = ollamaInfo

	// Read environment variables
	result.Environment = h.envReader.ReadConfigDisplay()

	// Check cc-switch
	ccSwitchInfo := &CCSwitchInfo{
		Available: h.ccSwitch.IsAvailable(),
	}
	if ccSwitchInfo.Available {
		if name, err := h.ccSwitch.GetActiveProfileName(); err == nil {
			ccSwitchInfo.ActiveProfile = name
		}
		if profiles, err := h.ccSwitch.ListProfilesDisplay(); err == nil {
			ccSwitchInfo.Profiles = profiles
		}
	}
	result.CCSwitch = ccSwitchInfo

	// Check CLI status
	cliStatus, _ := h.downloadMgr.GetCLIStatus(ctx)
	result.CLI = cliStatus

	return c.JSON(http.StatusOK, result)
}

// GetOllama returns Ollama detection status.
func (h *ProviderHandler) GetOllama(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	info, err := h.ollamaDetector.Detect(ctx)
	if err != nil {
		return c.JSON(http.StatusOK, &providers.OllamaInfo{
			Available: false,
		})
	}

	return c.JSON(http.StatusOK, info)
}

// GetOllamaModels returns available Ollama models.
func (h *ProviderHandler) GetOllamaModels(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	info, err := h.ollamaDetector.Detect(ctx)
	if err != nil || !info.Available {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"available": false,
			"models":    []string{},
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"available": true,
		"models":    info.GetModelNames(),
	})
}

// GetEnvConfig returns environment variable configuration.
func (h *ProviderHandler) GetEnvConfig(c echo.Context) error {
	return c.JSON(http.StatusOK, h.envReader.ReadConfigDisplay())
}

// GetCCSwitchProfiles returns cc-switch profiles.
func (h *ProviderHandler) GetCCSwitchProfiles(c echo.Context) error {
	if !h.ccSwitch.IsAvailable() {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"available": false,
			"profiles":  []providers.ProfileDisplay{},
		})
	}

	profiles, err := h.ccSwitch.ListProfilesDisplay()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	activeName, _ := h.ccSwitch.GetActiveProfileName()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"available":      true,
		"active_profile": activeName,
		"profiles":       profiles,
	})
}

// ActivateCCSwitchProfile activates a cc-switch profile.
func (h *ProviderHandler) ActivateCCSwitchProfile(c echo.Context) error {
	var req struct {
		Name string `json:"name"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if err := h.ccSwitch.ActivateProfile(req.Name); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Profile activated",
	})
}

// GetCLIStatus returns CLI installation status.
func (h *ProviderHandler) GetCLIStatus(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	status, err := h.downloadMgr.GetCLIStatus(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, status)
}

// GetDownloadInfo returns CLI download information.
func (h *ProviderHandler) GetDownloadInfo(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	info, err := h.downloadMgr.GetDownloadInfo(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, info)
}

// StartDownload starts CLI download.
func (h *ProviderHandler) StartDownload(c echo.Context) error {
	if h.downloadMgr.IsDownloading() {
		return c.JSON(http.StatusConflict, map[string]interface{}{
			"success": false,
			"error":   "Download already in progress",
		})
	}

	// Start download in background
	go func() {
		ctx := context.Background()
		h.downloadMgr.Download(ctx)
	}()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Download started",
	})
}

// GetDownloadProgress returns current download progress.
func (h *ProviderHandler) GetDownloadProgress(c echo.Context) error {
	if !h.downloadMgr.IsDownloading() {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"downloading": false,
		})
	}

	progress := h.downloadMgr.GetProgress()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"downloading": true,
		"progress":    progress,
	})
}

// CancelDownload cancels the current download.
func (h *ProviderHandler) CancelDownload(c echo.Context) error {
	h.downloadMgr.Cancel()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Download cancelled",
	})
}
