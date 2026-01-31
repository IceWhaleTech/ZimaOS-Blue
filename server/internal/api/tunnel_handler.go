package api

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"sync"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tunnel"
)

// TunnelHandler handles remote access API requests with multiple provider support.
type TunnelHandler struct {
	mu         sync.RWMutex
	managers   map[tunnel.Provider]tunnel.Manager
	active     tunnel.Manager
	repository *ngrok.Repository
	serverPort int
}

// NewTunnelHandler creates a new tunnel handler with multiple provider support.
func NewTunnelHandler(repo *ngrok.Repository, serverPort int) *TunnelHandler {
	h := &TunnelHandler{
		managers:   make(map[tunnel.Provider]tunnel.Manager),
		repository: repo,
		serverPort: serverPort,
	}

	// Initialize providers (Auto uses Bore/Serveo/LocalTunnel in parallel)
	h.managers[tunnel.ProviderAuto] = tunnel.NewAutoManager()
	h.managers[tunnel.ProviderNgrok] = tunnel.NewNgrokManager()
	h.managers[tunnel.ProviderCloudflare] = tunnel.NewCloudflareManager()

	// Log tunnel URL when available (format: "Tunnel URL: http://bore.pub:2877")
	for _, m := range h.managers {
		m.SetOnURLChange(func(url string) {
			if h.repository != nil && url != "" {
				h.repository.AddLog(context.Background(), "", "url", fmt.Sprintf("Tunnel URL: %s", url), nil)
			}
		})
	}

	return h
}

// RegisterRoutes registers tunnel routes.
func (h *TunnelHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/tunnel")

	// Provider info
	g.GET("/providers", h.GetProviders)

	// Tunnel endpoints
	g.POST("/start", h.StartTunnel)
	g.POST("/stop", h.StopTunnel)
	g.GET("/status", h.GetTunnelStatus)
	g.GET("/qrcode", h.GetQRCode)

	// Configuration endpoints
	g.GET("/config", h.GetTunnelConfig)
	g.PUT("/config", h.UpdateTunnelConfig)
	g.GET("/logs", h.GetTunnelLogs)

	// Diagnostic endpoints
	g.GET("/diagnostics", h.GetDiagnostics)

	// Also register under old path for backwards compatibility
	old := e.Group("/api/v1/remote-access")
	old.GET("/providers", h.GetProviders)
	old.POST("/start", h.StartTunnel)
	old.POST("/stop", h.StopTunnel)
	old.GET("/status", h.GetTunnelStatus)
	old.GET("/qrcode", h.GetQRCode)
	old.GET("/config", h.GetTunnelConfig)
	old.PUT("/config", h.UpdateTunnelConfig)
	old.GET("/logs", h.GetTunnelLogs)
	old.GET("/diagnostics", h.GetDiagnostics)
}

// GetProviders returns available tunnel providers.
func (h *TunnelHandler) GetProviders(c echo.Context) error {
	providers := tunnel.GetProviderInfos()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":   true,
		"providers": providers,
	})
}

// StartTunnelRequest represents the start tunnel request.
type StartTunnelRequest struct {
	Provider        string `json:"provider"`
	Port            int    `json:"port"`
	NgrokAuthtoken  string `json:"ngrok_authtoken,omitempty"`
	NgrokDomain     string `json:"ngrok_domain,omitempty"`
	CloudflareToken string `json:"cloudflare_token,omitempty"`
}

// StartTunnel starts the tunnel with the specified provider.
func (h *TunnelHandler) StartTunnel(c echo.Context) error {
	var req StartTunnelRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Default provider; map legacy/local-only providers to auto (Auto tries Bore/Serveo/LocalTunnel in parallel)
	provider := tunnel.Provider(req.Provider)
	if provider == "" {
		provider = tunnel.ProviderAuto // Default to auto mode
	}
	if provider == tunnel.ProviderLocalhostRun || provider == tunnel.ProviderServeo || provider == tunnel.ProviderLocalTunnel {
		provider = tunnel.ProviderAuto
	}

	// Default port to server port
	port := req.Port
	if port == 0 {
		port = h.serverPort
	}

	// Get manager for provider
	h.mu.RLock()
	manager, ok := h.managers[provider]
	h.mu.RUnlock()

	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Unknown provider: %s", provider),
		})
	}

	// Stop any existing tunnel
	h.mu.Lock()
	if h.active != nil && h.active.IsRunning() {
		h.active.Stop()
	}
	h.mu.Unlock()

	// Build config
	cfg := &tunnel.Config{
		Provider:        provider,
		Port:            port,
		NgrokAuthtoken:  req.NgrokAuthtoken,
		NgrokDomain:     req.NgrokDomain,
		CloudflareToken: req.CloudflareToken,
	}

	// Try to get tokens and subdomain from saved config if not provided
	if h.repository != nil {
		savedConfig, err := h.repository.GetConfig(c.Request().Context())
		if err == nil && savedConfig != nil {
			// Set subdomain for SSH-based tunnels (serveo, localhost.run)
			if savedConfig.TunnelSubdomain != "" {
				cfg.Subdomain = savedConfig.TunnelSubdomain
			}
			if provider == tunnel.ProviderNgrok && cfg.NgrokAuthtoken == "" && savedConfig.NgrokAuthtoken != "" {
				cfg.NgrokAuthtoken = savedConfig.NgrokAuthtoken
			}
			if provider == tunnel.ProviderNgrok && cfg.NgrokDomain == "" && savedConfig.NgrokDomain != "" {
				cfg.NgrokDomain = savedConfig.NgrokDomain
			}
			if provider == tunnel.ProviderCloudflare && cfg.CloudflareToken == "" && savedConfig.CloudflareToken != "" {
				cfg.CloudflareToken = savedConfig.CloudflareToken
			}
		}
		// Auto (Serveo): ensure we always have a subdomain (echo-xxx)
		if provider == tunnel.ProviderAuto && cfg.Subdomain == "" {
			if sub, err := h.repository.EnsureTunnelSubdomain(c.Request().Context()); err == nil {
				cfg.Subdomain = sub
			}
		}
	}

	// Idempotent start: if this manager is already running, return success with current status
	h.mu.RLock()
	alreadyRunning := h.active == manager && manager.IsRunning()
	h.mu.RUnlock()
	if alreadyRunning {
		status := manager.GetStatus()
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":  true,
			"message":  "Tunnel already running",
			"provider": provider,
			"tunnel":   status,
		})
	}

	// Start tunnel
	if err := manager.Start(c.Request().Context(), cfg); err != nil {
		// "tunnel already running" -> return success with current status (idempotent)
		if err.Error() == "tunnel already running" {
			h.mu.RLock()
			active := h.active
			h.mu.RUnlock()
			if active == manager && manager.IsRunning() {
				status := manager.GetStatus()
				return c.JSON(http.StatusOK, map[string]interface{}{
					"success":  true,
					"message":  "Tunnel already running",
					"provider": provider,
					"tunnel":   status,
				})
			}
		}
		// Log error
		if h.repository != nil {
			h.repository.AddLog(context.Background(), "", "error", err.Error(), nil)
		}
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	h.mu.Lock()
	h.active = manager
	h.mu.Unlock()

	// Log start with domain when available
	if h.repository != nil {
		msg := fmt.Sprintf("Tunnel started with provider: %s", provider)
		if url := manager.GetURL(); url != "" {
			msg = fmt.Sprintf("Tunnel started with provider: %s, domain: %s", provider, url)
		}
		h.repository.AddLog(context.Background(), "", "started", msg, nil)
	}

	// Get current status (URL may not be available yet for async providers)
	status := manager.GetStatus()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "Tunnel started",
		"provider": provider,
		"tunnel":   status,
	})
}

// StopTunnel stops the active tunnel.
func (h *TunnelHandler) StopTunnel(c echo.Context) error {
	h.mu.Lock()
	active := h.active
	h.mu.Unlock()

	if active == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "No active tunnel",
		})
	}

	if err := active.Stop(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Clear active tunnel reference
	h.mu.Lock()
	h.active = nil
	h.mu.Unlock()

	// Log stop
	if h.repository != nil {
		h.repository.AddLog(context.Background(), "", "stopped", "Tunnel stopped", nil)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Tunnel stopped",
	})
}

// GetTunnelStatus returns the current tunnel status.
func (h *TunnelHandler) GetTunnelStatus(c echo.Context) error {
	h.mu.RLock()
	active := h.active
	h.mu.RUnlock()

	if active == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"tunnel": tunnel.Status{
				Active: false,
			},
		})
	}

	status := active.GetStatus()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"tunnel":  status,
	})
}

// GetQRCode generates a QR code for the tunnel URL.
func (h *TunnelHandler) GetQRCode(c echo.Context) error {
	h.mu.RLock()
	active := h.active
	h.mu.RUnlock()

	if active == nil || !active.IsRunning() {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   "No active tunnel",
		})
	}

	url := active.GetURL()
	if url == "" {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   "Tunnel URL not available yet",
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

// GetTunnelConfig returns the tunnel configuration.
func (h *TunnelHandler) GetTunnelConfig(c echo.Context) error {
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

	// Normalize legacy providers to auto for API response
	if config.DefaultProvider == "localhost_run" || config.DefaultProvider == "serveo" {
		config.DefaultProvider = "auto"
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  config,
	})
}

// UpdateTunnelConfig updates the tunnel configuration.
func (h *TunnelHandler) UpdateTunnelConfig(c echo.Context) error {
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

	// Normalize legacy providers to auto when saving
	if config.DefaultProvider == "localhost_run" || config.DefaultProvider == "serveo" {
		config.DefaultProvider = "auto"
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

// GetTunnelLogs returns the tunnel logs.
func (h *TunnelHandler) GetTunnelLogs(c echo.Context) error {
	if h.repository == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Log storage not available",
		})
	}

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

// GetDiagnostics returns diagnostic information.
func (h *TunnelHandler) GetDiagnostics(c echo.Context) error {
	h.mu.RLock()
	active := h.active
	h.mu.RUnlock()

	diagnostics := map[string]interface{}{
		"tunnel_running":     active != nil && active.IsRunning(),
		"firewall_exception": ngrok.CheckFirewallException(),
		"os": map[string]string{
			"platform": runtime.GOOS,
		},
		"ssh_available":         checkSSHAvailable(),
		"bore_available":        tunnel.CheckBoreAvailable(),
		"cloudflared_installed": tunnel.CheckCloudflaredInstalled(),
	}

	if active != nil {
		diagnostics["active_provider"] = active.GetProvider()
	}

	// Get recent error logs only
	if h.repository != nil {
		logs, err := h.repository.GetErrorLogs(c.Request().Context(), 10, 0)
		if err == nil {
			diagnostics["recent_errors"] = logs
		}
	}

	// Generate hints
	hints := []string{}
	// Serveo uses built-in Go SSH, so system ssh is not required for Auto mode
	if !diagnostics["cloudflared_installed"].(bool) {
		hints = append(hints, "cloudflared is not installed. Install it to use Cloudflare Tunnel.")
	}
	if len(hints) == 0 {
		hints = append(hints, "All tunnel providers are available.")
	}
	diagnostics["hints"] = hints

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":     true,
		"diagnostics": diagnostics,
	})
}

// checkSSHAvailable checks if SSH client is available.
func checkSSHAvailable() bool {
	_, err := exec.LookPath("ssh")
	return err == nil
}
