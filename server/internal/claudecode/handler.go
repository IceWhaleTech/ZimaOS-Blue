package claudecode

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// VersionResponse is the response for GET /api/v1/claudecode/version
type VersionResponse struct {
	EmbeddedVersion  string       `json:"embedded_version,omitempty"`
	InstalledVersion string       `json:"installed_version,omitempty"`
	SystemVersion    string       `json:"system_version,omitempty"`
	LatestVersion    string       `json:"latest_version,omitempty"`
	ActiveVersion    string       `json:"active_version"`
	Mode             string       `json:"mode"`
	Source           BinarySource `json:"source"`
	BinaryPath       string       `json:"binary_path"`
	Platform         string       `json:"platform"`
	UpdateAvailable  bool         `json:"update_available"`
	Validated        bool         `json:"validated"`
	LastCheck        *time.Time   `json:"last_check,omitempty"`
}

// CheckUpdateResponse is the response for POST /api/v1/claudecode/version/check
type CheckUpdateResponse struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseNotes    string `json:"release_notes,omitempty"`
	ReleaseDate     string `json:"release_date,omitempty"`
	DownloadSize    string `json:"download_size,omitempty"`
}

// UpdateRequest is the request for POST /api/v1/claudecode/version/update
type UpdateRequest struct {
	Version string `json:"version"` // "latest" or specific version
}

// UpdateResponse is the response for POST /api/v1/claudecode/version/update
type UpdateResponse struct {
	Success         bool   `json:"success"`
	PreviousVersion string `json:"previous_version"`
	NewVersion      string `json:"new_version"`
	Message         string `json:"message"`
}

// ClearCacheResponse is the response for POST /api/v1/claudecode/cache/clear
type ClearCacheResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ValidateRequest is the request for POST /api/v1/claudecode/validate
type ValidateRequest struct {
	BinaryPath string `json:"binary_path,omitempty"` // Optional: specific path to validate
}

// ValidateResponse is the response for POST /api/v1/claudecode/validate
type ValidateResponse struct {
	Valid       bool   `json:"valid"`
	Version     string `json:"version,omitempty"`
	BinaryPath  string `json:"binary_path"`
	Message     string `json:"message"`
	DryRunOK    bool   `json:"dry_run_ok"`
}

// ConfigResponse is the response for GET /api/v1/claudecode/config
type ConfigResponse struct {
	Enabled        bool   `json:"enabled"`
	DefaultModel   string `json:"default_model"`
	SandboxEnabled bool   `json:"sandbox_enabled"`
	NetworkEnabled bool   `json:"network_enabled"`
}

// ConfigRequest is the request for PUT /api/v1/claudecode/config
type ConfigRequest struct {
	Enabled        *bool   `json:"enabled,omitempty"`
	DefaultModel   *string `json:"default_model,omitempty"`
	SandboxEnabled *bool   `json:"sandbox_enabled,omitempty"`
	NetworkEnabled *bool   `json:"network_enabled,omitempty"`
}

// ClaudeCodePersistentConfig is the configuration saved to disk
type ClaudeCodePersistentConfig struct {
	Enabled        bool   `json:"enabled"`
	DefaultModel   string `json:"default_model"`
	SandboxEnabled bool   `json:"sandbox_enabled"`
	NetworkEnabled bool   `json:"network_enabled"`
}

// Handler handles Claude Code CLI version management API endpoints.
type Handler struct {
	binaryManager  *BinaryManager
	lastCheck      *time.Time
	dataDir        string
	configMu       sync.RWMutex
	config         *ClaudeCodePersistentConfig
	healthChecker  *HealthChecker
	circuitBreaker *CircuitBreaker
	cache          *MemoryCache
}

// NewHandler creates a new Handler.
func NewHandler(binaryManager *BinaryManager) *Handler {
	return NewHandlerWithDataDir(binaryManager, "")
}

// NewHandlerWithDataDir creates a new Handler with a data directory for persistent config.
func NewHandlerWithDataDir(binaryManager *BinaryManager, dataDir string) *Handler {
	if binaryManager == nil {
		binaryManager = DefaultBinaryManager
	}
	h := &Handler{
		binaryManager: binaryManager,
		dataDir:       dataDir,
		config: &ClaudeCodePersistentConfig{
			Enabled:        true, // Default to enabled
			DefaultModel:   "sonnet",
			SandboxEnabled: true, // Default to sandbox enabled for security
			NetworkEnabled: true, // Default to network enabled
		},
		healthChecker:  NewHealthChecker(DefaultHealthCheckConfig(), binaryManager),
		circuitBreaker: NewCircuitBreaker(DefaultCircuitBreakerConfig()),
		cache:          NewMemoryCache(DefaultCacheConfig()),
	}
	// Load existing config if available
	if dataDir != "" {
		h.loadConfig()
	}
	return h
}

// loadConfig loads the configuration from disk.
func (h *Handler) loadConfig() {
	if h.dataDir == "" {
		return
	}
	configPath := filepath.Join(h.dataDir, "claudecode_config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return // File doesn't exist or can't be read, use defaults
	}
	var config ClaudeCodePersistentConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return // Invalid JSON, use defaults
	}
	h.configMu.Lock()
	h.config = &config
	h.configMu.Unlock()
}

// saveConfig saves the configuration to disk.
func (h *Handler) saveConfig() error {
	if h.dataDir == "" {
		return nil
	}
	if err := os.MkdirAll(h.dataDir, 0755); err != nil {
		return err
	}
	h.configMu.RLock()
	data, err := json.MarshalIndent(h.config, "", "  ")
	h.configMu.RUnlock()
	if err != nil {
		return err
	}
	configPath := filepath.Join(h.dataDir, "claudecode_config.json")
	return os.WriteFile(configPath, data, 0644)
}

// IsEnabled returns whether Claude Code CLI is enabled.
func (h *Handler) IsEnabled() bool {
	h.configMu.RLock()
	defer h.configMu.RUnlock()
	return h.config.Enabled
}

// RegisterRoutes registers the Claude Code CLI API routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Version management
	g.GET("/version", h.GetVersion)
	g.POST("/version/check", h.CheckForUpdates)
	g.POST("/version/update", h.Update)
	g.POST("/cache/clear", h.ClearCache)
	g.POST("/validate", h.Validate)
	g.GET("/config", h.GetConfig)
	g.PUT("/config", h.SetConfig)

	// Reliability features
	g.GET("/health", h.GetHealth)
	g.POST("/health/check", h.TriggerHealthCheck)
	g.GET("/circuit", h.GetCircuitStatus)
	g.POST("/circuit/reset", h.ResetCircuit)
	g.GET("/response-cache/stats", h.GetResponseCacheStats)
	g.POST("/response-cache/clear", h.ClearResponseCache)
	g.GET("/reliability/metrics", h.GetReliabilityMetrics)
}

// GetVersion returns the current version information.
// GET /api/v1/claudecode/version
func (h *Handler) GetVersion(c echo.Context) error {
	info, err := h.binaryManager.GetVersionInfo()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	resp := VersionResponse{
		EmbeddedVersion:  info.EmbeddedVersion,
		InstalledVersion: info.InstalledVersion,
		SystemVersion:    info.SystemVersion,
		ActiveVersion:    info.ActiveVersion,
		Mode:             "auto", // TODO: get from config
		Source:           info.Source,
		BinaryPath:       info.BinaryPath,
		Platform:         info.Platform,
		UpdateAvailable:  info.UpdateAvailable,
		Validated:        info.Validated,
		LastCheck:        h.lastCheck,
	}

	return c.JSON(http.StatusOK, resp)
}

// CheckForUpdates checks if a newer version is available.
// POST /api/v1/claudecode/version/check
func (h *Handler) CheckForUpdates(c echo.Context) error {
	currentVersion, _ := h.binaryManager.GetInstalledVersion()

	updateAvailable, latestVersion, err := h.binaryManager.CheckForUpdates()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	now := time.Now()
	h.lastCheck = &now

	resp := CheckUpdateResponse{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVersion,
		UpdateAvailable: updateAvailable,
	}

	return c.JSON(http.StatusOK, resp)
}

// Update updates to the specified version.
// POST /api/v1/claudecode/version/update
func (h *Handler) Update(c echo.Context) error {
	var req UpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	previousVersion, _ := h.binaryManager.GetInstalledVersion()

	version := req.Version
	if version == "" {
		version = "latest"
	}

	if err := h.binaryManager.Update(version); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	newVersion, _ := h.binaryManager.GetInstalledVersion()

	resp := UpdateResponse{
		Success:         true,
		PreviousVersion: previousVersion,
		NewVersion:      newVersion,
		Message:         "Claude Code CLI updated successfully",
	}

	return c.JSON(http.StatusOK, resp)
}

// ClearCache clears the downloaded binaries cache.
// POST /api/v1/claudecode/cache/clear
func (h *Handler) ClearCache(c echo.Context) error {
	if err := h.binaryManager.ClearCache(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	resp := ClearCacheResponse{
		Success: true,
		Message: "Cache cleared successfully",
	}

	return c.JSON(http.StatusOK, resp)
}

// Validate validates a Claude Code CLI binary.
// POST /api/v1/claudecode/validate
func (h *Handler) Validate(c echo.Context) error {
	var req ValidateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	var binaryPath string
	var err error

	if req.BinaryPath != "" {
		// Validate specific path
		binaryPath = req.BinaryPath
	} else {
		// Get current binary path
		binaryPath, err = h.binaryManager.GetBinaryPath()
		if err != nil {
			return c.JSON(http.StatusOK, ValidateResponse{
				Valid:      false,
				BinaryPath: "",
				Message:    "No Claude Code CLI binary found: " + err.Error(),
				DryRunOK:   false,
			})
		}
	}

	// Validate the binary
	version, validateErr := h.binaryManager.ValidateBinary(binaryPath)

	resp := ValidateResponse{
		BinaryPath: binaryPath,
		Version:    version,
	}

	if validateErr != nil {
		resp.Valid = false
		resp.DryRunOK = false
		resp.Message = "Binary validation failed: " + validateErr.Error()
	} else {
		resp.Valid = true
		resp.DryRunOK = true
		resp.Message = "Claude Code CLI validated successfully"
	}

	return c.JSON(http.StatusOK, resp)
}

// GetConfig returns the Claude Code CLI configuration.
// GET /api/v1/claudecode/config
func (h *Handler) GetConfig(c echo.Context) error {
	h.configMu.RLock()
	resp := ConfigResponse{
		Enabled:        h.config.Enabled,
		DefaultModel:   h.config.DefaultModel,
		SandboxEnabled: h.config.SandboxEnabled,
		NetworkEnabled: h.config.NetworkEnabled,
	}
	h.configMu.RUnlock()

	return c.JSON(http.StatusOK, resp)
}

// SetConfig updates the Claude Code CLI configuration.
// PUT /api/v1/claudecode/config
func (h *Handler) SetConfig(c echo.Context) error {
	var req ConfigRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	h.configMu.Lock()
	if req.Enabled != nil {
		h.config.Enabled = *req.Enabled
	}
	if req.DefaultModel != nil && *req.DefaultModel != "" {
		h.config.DefaultModel = *req.DefaultModel
	}
	if req.SandboxEnabled != nil {
		h.config.SandboxEnabled = *req.SandboxEnabled
	}
	if req.NetworkEnabled != nil {
		h.config.NetworkEnabled = *req.NetworkEnabled
	}
	h.configMu.Unlock()

	// Save to disk
	if err := h.saveConfig(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to save configuration: " + err.Error(),
		})
	}

	// Return updated config
	h.configMu.RLock()
	resp := ConfigResponse{
		Enabled:        h.config.Enabled,
		DefaultModel:   h.config.DefaultModel,
		SandboxEnabled: h.config.SandboxEnabled,
		NetworkEnabled: h.config.NetworkEnabled,
	}
	h.configMu.RUnlock()

	return c.JSON(http.StatusOK, resp)
}

// GetHealth returns the current health status.
// GET /api/v1/claudecode/health
func (h *Handler) GetHealth(c echo.Context) error {
	if h.healthChecker == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "unknown",
			"message": "Health checker not initialized",
		})
	}
	status := h.healthChecker.Status()
	return c.JSON(http.StatusOK, status)
}

// TriggerHealthCheck triggers an immediate health check.
// POST /api/v1/claudecode/health/check
func (h *Handler) TriggerHealthCheck(c echo.Context) error {
	if h.healthChecker == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "unknown",
			"message": "Health checker not initialized",
		})
	}
	status := h.healthChecker.Check(c.Request().Context())
	return c.JSON(http.StatusOK, status)
}

// GetCircuitStatus returns the current circuit breaker status.
// GET /api/v1/claudecode/circuit
func (h *Handler) GetCircuitStatus(c echo.Context) error {
	if h.circuitBreaker == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"state":   "unknown",
			"message": "Circuit breaker not initialized",
		})
	}
	status := h.circuitBreaker.Status()
	return c.JSON(http.StatusOK, status)
}

// ResetCircuit resets the circuit breaker to closed state.
// POST /api/v1/claudecode/circuit/reset
func (h *Handler) ResetCircuit(c echo.Context) error {
	if h.circuitBreaker == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "Circuit breaker not initialized",
		})
	}
	h.circuitBreaker.Reset()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Circuit breaker reset to closed state",
		"status":  h.circuitBreaker.Status(),
	})
}

// GetResponseCacheStats returns the response cache statistics.
// GET /api/v1/claudecode/response-cache/stats
func (h *Handler) GetResponseCacheStats(c echo.Context) error {
	if h.cache == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "Response cache not initialized",
		})
	}
	stats := h.cache.Stats()
	return c.JSON(http.StatusOK, stats)
}

// ClearResponseCache clears the response cache.
// POST /api/v1/claudecode/response-cache/clear
func (h *Handler) ClearResponseCache(c echo.Context) error {
	if h.cache == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "Response cache not initialized",
		})
	}
	h.cache.Clear()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Response cache cleared",
	})
}

// GetReliabilityMetrics returns the reliability metrics.
// GET /api/v1/claudecode/reliability/metrics
func (h *Handler) GetReliabilityMetrics(c echo.Context) error {
	metrics := make(map[string]interface{})

	if h.cache != nil {
		metrics["cache"] = h.cache.Stats()
	}
	if h.circuitBreaker != nil {
		metrics["circuit_breaker"] = h.circuitBreaker.Status()
	}
	if h.healthChecker != nil {
		metrics["health"] = h.healthChecker.Status()
	}

	return c.JSON(http.StatusOK, metrics)
}
