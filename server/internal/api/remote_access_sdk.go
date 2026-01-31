package api

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/ngrok"
)

// SDKRemoteAccessHandler handles remote access API requests using SDK tunnel manager.
type SDKRemoteAccessHandler struct {
	tunnelManager *ngrok.SDKTunnelManager
	repository    *ngrok.Repository
}

// NewSDKRemoteAccessHandler creates a new SDK-based remote access handler.
func NewSDKRemoteAccessHandler(tm *ngrok.SDKTunnelManager, repo *ngrok.Repository) *SDKRemoteAccessHandler {
	return &SDKRemoteAccessHandler{
		tunnelManager: tm,
		repository:    repo,
	}
}

// RegisterRoutes registers remote access routes.
func (h *SDKRemoteAccessHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/remote-access")

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

// StartRemoteAccess starts the remote access tunnel.
func (h *SDKRemoteAccessHandler) StartRemoteAccess(c echo.Context) error {
	var req struct {
		Port      int    `json:"port"`
		Authtoken string `json:"authtoken,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Default port
	if req.Port == 0 {
		req.Port = 8080
	}

	// Start tunnel
	if err := h.tunnelManager.Start(c.Request().Context(), req.Port, req.Authtoken); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Remote access started",
	})
}

// StopRemoteAccess stops the remote access tunnel.
func (h *SDKRemoteAccessHandler) StopRemoteAccess(c echo.Context) error {
	if err := h.tunnelManager.Stop(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Remote access stopped",
	})
}

// GetRemoteAccessStatus returns the current tunnel status.
func (h *SDKRemoteAccessHandler) GetRemoteAccessStatus(c echo.Context) error {
	tunnelStatus := h.tunnelManager.GetStatus()

	// If tunnel is not active in memory, check database for persisted state
	if !tunnelStatus.Active && h.repository != nil {
		session, err := h.repository.GetActiveSession(c.Request().Context())
		if err == nil && session != nil {
			// Return persisted state
			tunnelStatus = ngrok.TunnelStatus{
				Active:        session.Status == "active",
				Connecting:    session.Status == "connecting",
				URL:           session.TunnelURL,
				StartedAt:     session.StartedAt,
				ExpiresAt:     session.ExpiresAt,
				RemainingTime: formatRemainingTime(time.Until(session.ExpiresAt)),
				RenewedCount:  session.RenewedCount,
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"tunnel":  tunnelStatus,
	})
}

// GetQRCode generates a QR code for the tunnel URL.
func (h *SDKRemoteAccessHandler) GetQRCode(c echo.Context) error {
	url := h.tunnelManager.GetURL()
	if url == "" {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   "No active tunnel",
		})
	}

	qrcode, err := ngrok.GenerateQRCode(url, 200)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"url":     url,
		"qrcode":  qrcode,
	})
}

// GetRemoteAccessConfig returns the remote access configuration.
func (h *SDKRemoteAccessHandler) GetRemoteAccessConfig(c echo.Context) error {
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

// UpdateRemoteAccessConfig updates the remote access configuration.
func (h *SDKRemoteAccessHandler) UpdateRemoteAccessConfig(c echo.Context) error {
	if h.repository == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Configuration storage not available",
		})
	}

	var config ngrok.RemoteAccessConfig
	if err := c.Bind(&config); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if err := h.repository.SaveConfig(c.Request().Context(), &config); err != nil {
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
func (h *SDKRemoteAccessHandler) GetRemoteAccessLogs(c echo.Context) error {
	if h.repository == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Log storage not available",
		})
	}

	// Parse pagination parameters
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit := 50
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr != "" {
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
func (h *SDKRemoteAccessHandler) GetDiagnostics(c echo.Context) error {
	diagnostics := map[string]interface{}{
		"tunnel_running":     h.tunnelManager.IsRunning(),
		"firewall_exception": ngrok.CheckFirewallException(),
		"os": map[string]string{
			"platform": runtime.GOOS,
		},
	}

	// Get recent errors from database
	if h.repository != nil {
		logs, err := h.repository.GetLogs(c.Request().Context(), 10, 0)
		if err == nil {
			diagnostics["recent_errors"] = logs
		}

		// Get active session
		session, err := h.repository.GetActiveSession(c.Request().Context())
		if err == nil && session != nil {
			diagnostics["active_session"] = session
		}
	}

	// Generate troubleshooting hints
	hints := []string{}
	if !diagnostics["firewall_exception"].(bool) && runtime.GOOS == "windows" {
		hints = append(hints, "Windows Firewall exception not found. You may need to run as administrator.")
	}
	if !diagnostics["tunnel_running"].(bool) {
		hints = append(hints, "Tunnel is not running. Click 'Enable Remote Access' to start.")
	}
	if len(hints) == 0 {
		hints = append(hints, "Everything looks good!")
	}
	diagnostics["hints"] = hints

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":     true,
		"diagnostics": diagnostics,
	})
}

// formatRemainingTime formats a duration into a human-readable string.
func formatRemainingTime(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	return fmt.Sprintf("%dm", minutes)
}
