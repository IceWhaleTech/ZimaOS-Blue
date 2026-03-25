package claudecode

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// VersionResponse is the response for GET /api/v1/claudecode/version
type VersionResponse struct {
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
	Valid      bool   `json:"valid"`
	Version    string `json:"version,omitempty"`
	BinaryPath string `json:"binary_path"`
	Message    string `json:"message"`
	DryRunOK   bool   `json:"dry_run_ok"`
}

// ConfigResponse is the response for GET /api/v1/claudecode/config
type ConfigResponse struct {
	Enabled            bool                      `json:"enabled"`
	DefaultModel       string                    `json:"default_model"`
	SandboxEnabled     bool                      `json:"sandbox_enabled"`
	NetworkEnabled     bool                      `json:"network_enabled"`
	WhitelistEnabled   bool                      `json:"whitelist_enabled"`
	DirectoryWhitelist []DirectoryWhitelistEntry `json:"directory_whitelist,omitempty"`
}

// DirectoryWhitelistEntry represents a whitelisted directory
type DirectoryWhitelistEntry struct {
	Path  string `json:"path"`
	Alias string `json:"alias,omitempty"`
}

// ConfigRequest is the request for PUT /api/v1/claudecode/config
type ConfigRequest struct {
	Enabled            *bool                      `json:"enabled,omitempty"`
	DefaultModel       *string                    `json:"default_model,omitempty"`
	SandboxEnabled     *bool                      `json:"sandbox_enabled,omitempty"`
	NetworkEnabled     *bool                      `json:"network_enabled,omitempty"`
	WhitelistEnabled   *bool                      `json:"whitelist_enabled,omitempty"`
	DirectoryWhitelist *[]DirectoryWhitelistEntry `json:"directory_whitelist,omitempty"`
}

// ClaudeCodePersistentConfig is the configuration saved to disk
type ClaudeCodePersistentConfig struct {
	Enabled            bool                      `json:"enabled"`
	DefaultModel       string                    `json:"default_model"`
	SandboxEnabled     bool                      `json:"sandbox_enabled"`
	NetworkEnabled     bool                      `json:"network_enabled"`
	WhitelistEnabled   bool                      `json:"whitelist_enabled"`
	DirectoryWhitelist []DirectoryWhitelistEntry `json:"directory_whitelist,omitempty"`
}

const claudecodeKVKey = "config:claudecode"

// Handler handles Claude Code CLI version management API endpoints.
type Handler struct {
	binaryManager  *BinaryManager
	lastCheck      *time.Time
	dataDir        string
	kv             kvstore.Store
	configMu       sync.RWMutex
	config         *ClaudeCodePersistentConfig
	healthChecker  *HealthChecker
	circuitBreaker *CircuitBreaker
	cache          *MemoryCache
}

// ConfigSnapshot returns a thread-safe copy of the current Claude Code config.
func (h *Handler) ConfigSnapshot() ClaudeCodePersistentConfig {
	if h == nil {
		return ClaudeCodePersistentConfig{}
	}
	h.configMu.RLock()
	defer h.configMu.RUnlock()
	if h.config == nil {
		return ClaudeCodePersistentConfig{}
	}
	snapshot := *h.config
	if len(h.config.DirectoryWhitelist) > 0 {
		snapshot.DirectoryWhitelist = append([]DirectoryWhitelistEntry{}, h.config.DirectoryWhitelist...)
	}
	return snapshot
}

// DirectoryWhitelistSnapshot returns a thread-safe copy of whitelist settings.
func (h *Handler) DirectoryWhitelistSnapshot() (enabled bool, entries []DirectoryWhitelistEntry) {
	h.configMu.RLock()
	defer h.configMu.RUnlock()

	enabled = h.config.WhitelistEnabled
	if len(h.config.DirectoryWhitelist) == 0 {
		return enabled, nil
	}
	entries = make([]DirectoryWhitelistEntry, 0, len(h.config.DirectoryWhitelist))
	entries = append(entries, h.config.DirectoryWhitelist...)
	return enabled, entries
}

// DirectoryWhitelistRoots returns normalized absolute whitelist roots when enabled.
func (h *Handler) DirectoryWhitelistRoots() []string {
	enabled, entries := h.DirectoryWhitelistSnapshot()
	if !enabled || len(entries) == 0 {
		return nil
	}
	roots := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		p := strings.TrimSpace(entry.Path)
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		clean := filepath.Clean(abs)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		roots = append(roots, clean)
	}
	return roots
}

// NewHandler creates a new Handler.
func NewHandler(binaryManager *BinaryManager) *Handler {
	return NewHandlerWithDataDir(binaryManager, "", nil)
}

// NewHandlerWithDataDir creates a new Handler with a data directory for persistent config.
func NewHandlerWithDataDir(binaryManager *BinaryManager, dataDir string, kv kvstore.Store) *Handler {
	if binaryManager == nil {
		binaryManager = DefaultBinaryManager
	}
	h := &Handler{
		binaryManager: binaryManager,
		dataDir:       dataDir,
		kv:            kv,
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
	if kv != nil {
		h.loadConfig()
	}
	return h
}

// loadConfig loads the configuration from kvstore.
func (h *Handler) loadConfig() {
	if h.kv == nil {
		return
	}
	var config ClaudeCodePersistentConfig
	if err := h.kv.GetJSON(context.Background(), claudecodeKVKey, &config); err != nil {
		return // Not found or error — use defaults
	}
	h.configMu.Lock()
	h.config = &config
	h.configMu.Unlock()
}

// saveConfig saves the configuration to kvstore.
func (h *Handler) saveConfig() error {
	if h.kv == nil {
		return nil
	}
	h.configMu.RLock()
	cfg := *h.config
	h.configMu.RUnlock()
	return h.kv.SetJSON(context.Background(), claudecodeKVKey, &cfg, 0)
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

	// Directory browsing for whitelist picker
	g.GET("/browse-dirs", h.BrowseDirs)
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
		InstalledVersion: info.InstalledVersion,
		SystemVersion:    info.SystemVersion,
		ActiveVersion:    info.ActiveVersion,
		Mode:             string(info.Source),
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

	now := timeutil.NowTime()
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
		Enabled:            h.config.Enabled,
		DefaultModel:       h.config.DefaultModel,
		SandboxEnabled:     h.config.SandboxEnabled,
		NetworkEnabled:     h.config.NetworkEnabled,
		WhitelistEnabled:   h.config.WhitelistEnabled,
		DirectoryWhitelist: h.config.DirectoryWhitelist,
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
	if req.WhitelistEnabled != nil {
		h.config.WhitelistEnabled = *req.WhitelistEnabled
	}
	if req.DirectoryWhitelist != nil {
		h.config.DirectoryWhitelist = *req.DirectoryWhitelist
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
		Enabled:            h.config.Enabled,
		DefaultModel:       h.config.DefaultModel,
		SandboxEnabled:     h.config.SandboxEnabled,
		NetworkEnabled:     h.config.NetworkEnabled,
		WhitelistEnabled:   h.config.WhitelistEnabled,
		DirectoryWhitelist: h.config.DirectoryWhitelist,
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

// BrowseDirEntry represents a directory entry in the browse response.
type BrowseDirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// BrowseDirsResponse is the response for GET /api/v1/claudecode/browse-dirs.
type BrowseDirsResponse struct {
	Current string           `json:"current"`
	Parent  string           `json:"parent,omitempty"`
	Dirs    []BrowseDirEntry `json:"dirs"`
	OS      string           `json:"os"` // "windows", "darwin", "linux"
}

// BrowseDirs lists subdirectories of a given path for the directory picker.
// GET /api/v1/claudecode/browse-dirs?path=/some/dir
func (h *Handler) BrowseDirs(c echo.Context) error {
	reqPath := c.QueryParam("path")

	// Default to home directory
	if reqPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			reqPath = "/"
			if runtime.GOOS == "windows" {
				reqPath = "C:\\"
			}
		} else {
			reqPath = home
		}
	}

	// Clean and resolve the path
	reqPath = filepath.Clean(reqPath)

	// Security: prevent path traversal — resolve to absolute
	absPath, err := filepath.Abs(reqPath)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid path"})
	}
	reqPath = absPath

	// Read directory entries
	entries, err := os.ReadDir(reqPath)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "cannot read directory: " + err.Error(),
		})
	}

	dirs := make([]BrowseDirEntry, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Skip hidden directories
		if strings.HasPrefix(name, ".") {
			continue
		}
		dirs = append(dirs, BrowseDirEntry{
			Name: name,
			Path: filepath.Join(reqPath, name),
		})
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	resp := BrowseDirsResponse{
		Current: reqPath,
		Dirs:    dirs,
		OS:      runtime.GOOS,
	}

	// Add parent unless we're at root
	parent := filepath.Dir(reqPath)
	if parent != reqPath {
		resp.Parent = parent
	}

	return c.JSON(http.StatusOK, resp)
}
