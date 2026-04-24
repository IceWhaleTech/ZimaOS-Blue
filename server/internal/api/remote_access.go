package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// RemoteAccessHandler handles remote access API requests.
type RemoteAccessHandler struct {
	tunnelManager *ngrok.TunnelManager
	repository    *ngrok.Repository
	serverPort    int
}

// NewRemoteAccessHandler creates a new remote access handler.
func NewRemoteAccessHandler(tm *ngrok.TunnelManager, serverPort int) *RemoteAccessHandler {
	return &RemoteAccessHandler{
		tunnelManager: tm,
		serverPort:    serverPort,
	}
}

// NewRemoteAccessHandlerWithRepo creates a new remote access handler with repository.
func NewRemoteAccessHandlerWithRepo(tm *ngrok.TunnelManager, repo *ngrok.Repository, serverPort int) *RemoteAccessHandler {
	return &RemoteAccessHandler{
		tunnelManager: tm,
		repository:    repo,
		serverPort:    serverPort,
	}
}

// RegisterRoutes registers remote access routes.
func (h *RemoteAccessHandler) RegisterRoutes(e *echo.Echo) {
	h.RegisterGroupRoutes(e.Group("/api/v1"))
}

// RegisterGroupRoutes registers remote access routes on an existing API group.
func (h *RemoteAccessHandler) RegisterGroupRoutes(g *echo.Group) {
	g = g.Group("/remote-access")

	g.GET("/ngrok/status", h.GetNgrokStatus)

	// Tunnel endpoints
	g.POST("/start", h.StartRemoteAccess)
	g.POST("/stop", h.StopRemoteAccess)
	g.GET("/status", h.GetRemoteAccessStatus)
	g.GET("/qrcode", h.GetQRCode)

	// Configuration endpoints
	g.GET("/config", h.GetRemoteAccessConfig)
	g.PUT("/config", h.UpdateRemoteAccessConfig)
	g.GET("/logs", h.GetRemoteAccessLogs)

	// Diagnostic endpoints
	g.GET("/diagnostics", h.GetDiagnostics)
}

// GetNgrokStatus returns the ngrok installation status (from PATH).
func (h *RemoteAccessHandler) GetNgrokStatus(c echo.Context) error {
	status, err := ngrok.GetNgrokStatus(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}
	return c.JSON(http.StatusOK, status)
}

// StartRemoteAccessRequest represents the start remote access request.
type StartRemoteAccessRequest struct {
	Port      int    `json:"port"`
	Authtoken string `json:"authtoken,omitempty"`
}

// StartRemoteAccess starts the remote access tunnel.
func (h *RemoteAccessHandler) StartRemoteAccess(c echo.Context) error {
	var req StartRemoteAccessRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Default port - use server's actual listening port
	if req.Port == 0 {
		req.Port = resolveListeningPort(h.serverPort)
	}
	if req.Port == 0 {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Server listening port is not available yet",
		})
	}

	// Check if already running
	if h.tunnelManager.IsRunning() {
		status := h.tunnelManager.GetStatus()
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Tunnel already running",
			"tunnel":  status,
		})
	}

	// Start tunnel
	if err := h.tunnelManager.Start(c.Request().Context(), req.Port, req.Authtoken); err != nil {
		if err == ngrok.ErrNgrokNotInstalled {
			return c.JSON(http.StatusPreconditionFailed, map[string]interface{}{
				"success":    false,
				"error":      "ngrok is not installed",
				"error_code": "NGROK_NOT_INSTALLED",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Tunnel started",
	})
}

// StopRemoteAccess stops the remote access tunnel.
func (h *RemoteAccessHandler) StopRemoteAccess(c echo.Context) error {
	if err := h.tunnelManager.Stop(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Tunnel stopped",
	})
}

// GetRemoteAccessStatus returns the current remote access status.
func (h *RemoteAccessHandler) GetRemoteAccessStatus(c echo.Context) error {
	// Get ngrok installation status (from PATH)
	ngrokStatus, _ := ngrok.GetNgrokStatus(c.Request().Context())

	// Get tunnel status from memory
	tunnelStatus := h.tunnelManager.GetStatus()

	// If tunnel is not running in memory but repository is available,
	// check if there's an active session in the database
	if !tunnelStatus.Active && h.repository != nil {
		session, err := h.repository.GetActiveSession(c.Request().Context())
		if err == nil && session != nil {
			// Found an active session in database
			tunnelStatus = ngrok.TunnelStatus{
				Active:       true,
				Connecting:   session.Status == "connecting",
				URL:          session.TunnelURL,
				StartedAt:    session.StartedAt,
				ExpiresAt:    session.ExpiresAt,
				RenewedCount: session.RenewedCount,
			}

			// Calculate remaining time
			remaining := session.ExpiresAt.Sub(timeutil.NowTime())
			if remaining > 0 {
				hours := int(remaining.Hours())
				minutes := int(remaining.Minutes()) % 60
				if hours > 0 {
					tunnelStatus.RemainingTime = fmt.Sprintf("%dh %dm", hours, minutes)
				} else {
					tunnelStatus.RemainingTime = fmt.Sprintf("%dm", minutes)
				}
			} else {
				tunnelStatus.RemainingTime = "expired"
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"ngrok_installed": ngrokStatus.Installed,
		"ngrok_version":   ngrokStatus.Version,
		"tunnel":          tunnelStatus,
	})
}

// GetQRCode returns the raw QR payload for the current tunnel URL.
func (h *RemoteAccessHandler) GetQRCode(c echo.Context) error {
	// Check if tunnel is running
	if !h.tunnelManager.IsRunning() {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Tunnel is not running",
		})
	}

	// Get tunnel URL
	url := h.tunnelManager.GetURL()
	if url == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Tunnel URL not available yet",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"url":     url,
		"qr_url":  url,
	})
}

// GetRemoteAccessConfig returns the remote access configuration.
func (h *RemoteAccessHandler) GetRemoteAccessConfig(c echo.Context) error {
	if h.repository == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Configuration storage not available",
		})
	}

	config, err := h.repository.GetConfig(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  config,
	})
}

// UpdateRemoteAccessConfigRequest represents the update config request.
type UpdateRemoteAccessConfigRequest struct {
	Enabled               bool   `json:"enabled"`
	NgrokAuthtoken        string `json:"ngrok_authtoken,omitempty"`
	NotificationEmail     string `json:"notification_email,omitempty"`
	NotifyOnURLChange     bool   `json:"notify_on_url_change"`
	NotifyOnExpiryWarning bool   `json:"notify_on_expiry_warning"`
	NotifyOnError         bool   `json:"notify_on_error"`
}

// UpdateRemoteAccessConfig updates the remote access configuration.
func (h *RemoteAccessHandler) UpdateRemoteAccessConfig(c echo.Context) error {
	if h.repository == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Configuration storage not available",
		})
	}

	var req UpdateRemoteAccessConfigRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	config := &ngrok.RemoteAccessConfig{
		Enabled:               req.Enabled,
		NgrokAuthtoken:        req.NgrokAuthtoken,
		NotificationEmail:     req.NotificationEmail,
		NotifyOnURLChange:     req.NotifyOnURLChange,
		NotifyOnExpiryWarning: req.NotifyOnExpiryWarning,
		NotifyOnError:         req.NotifyOnError,
	}

	if err := h.repository.SaveConfig(c.Request().Context(), config); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Configuration updated",
	})
}

// GetRemoteAccessLogs returns the remote access logs.
func (h *RemoteAccessHandler) GetRemoteAccessLogs(c echo.Context) error {
	if h.repository == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Configuration storage not available",
		})
	}

	// Parse pagination parameters
	limit := 50
	offset := 0

	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := c.QueryParam("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	logs, err := h.repository.GetLogs(c.Request().Context(), limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"logs":    logs,
		"limit":   limit,
		"offset":  offset,
	})
}

// GetDiagnostics returns diagnostic information for troubleshooting.
func (h *RemoteAccessHandler) GetDiagnostics(c echo.Context) error {
	diagnostics := make(map[string]interface{})

	// Check ngrok installation (from PATH)
	ngrokStatus, _ := ngrok.GetNgrokStatus(c.Request().Context())
	diagnostics["ngrok_installed"] = ngrokStatus.Installed
	diagnostics["ngrok_path"] = ngrokStatus.Path
	diagnostics["ngrok_version"] = ngrokStatus.Version

	// Check firewall status (Windows only)
	diagnostics["firewall_exception"] = ngrok.CheckFirewallException()

	// Check if tunnel is running
	diagnostics["tunnel_running"] = h.tunnelManager.IsRunning()

	// Get recent error logs
	if h.repository != nil {
		logs, err := h.repository.GetLogs(c.Request().Context(), 10, 0)
		if err == nil {
			// Filter for error logs
			errorLogs := make([]*ngrok.RemoteAccessLog, 0)
			for _, log := range logs {
				if log.EventType == "stderr" || log.EventType == "error" {
					errorLogs = append(errorLogs, log)
				}
			}
			diagnostics["recent_errors"] = errorLogs
		}

		// Get active session info
		session, err := h.repository.GetActiveSession(c.Request().Context())
		if err == nil && session != nil {
			diagnostics["active_session"] = map[string]interface{}{
				"id":            session.ID,
				"status":        session.Status,
				"started_at":    session.StartedAt,
				"error_message": session.ErrorMessage,
			}
		}
	}

	// System information
	diagnostics["os"] = map[string]interface{}{
		"platform": "windows", // This should be dynamic based on runtime.GOOS
	}

	// Troubleshooting hints
	hints := make([]string, 0)
	if !ngrokStatus.Installed {
		hints = append(hints, "Ngrok is not installed. Please install ngrok and ensure it is in PATH.")
	}
	if ngrokStatus.Installed && !ngrok.CheckFirewallException() {
		hints = append(hints, "Windows Firewall exception not found. You may need to run as administrator or manually add firewall rule.")
	}
	if h.repository != nil {
		logs, _ := h.repository.GetLogs(c.Request().Context(), 5, 0)
		hasStderrErrors := false
		for _, log := range logs {
			if log.EventType == "stderr" && log.Message != "" {
				hasStderrErrors = true
				break
			}
		}
		if hasStderrErrors {
			hints = append(hints, "Ngrok process reported errors. Check logs for details. This may indicate antivirus blocking or network issues.")
		}
	}

	diagnostics["hints"] = hints

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":     true,
		"diagnostics": diagnostics,
	})
}
