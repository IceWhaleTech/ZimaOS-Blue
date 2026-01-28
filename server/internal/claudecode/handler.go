package claudecode

import (
	"net/http"
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

// Handler handles Claude Code CLI version management API endpoints.
type Handler struct {
	binaryManager *BinaryManager
	lastCheck     *time.Time
}

// NewHandler creates a new Handler.
func NewHandler(binaryManager *BinaryManager) *Handler {
	if binaryManager == nil {
		binaryManager = DefaultBinaryManager
	}
	return &Handler{
		binaryManager: binaryManager,
	}
}

// RegisterRoutes registers the Claude Code CLI API routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/version", h.GetVersion)
	g.POST("/version/check", h.CheckForUpdates)
	g.POST("/version/update", h.Update)
	g.POST("/cache/clear", h.ClearCache)
	g.POST("/validate", h.Validate)
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
