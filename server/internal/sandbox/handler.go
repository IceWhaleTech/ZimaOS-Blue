package sandbox

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	appconfig "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/labstack/echo/v4"
)

// Handler handles sandbox API endpoints.
type Handler struct {
	manager           *Manager
	configStore       *appconfig.ConfigStore
	networkConfigHook func(networkEnabled bool)
}

// NewHandler creates a new sandbox handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// Manager returns the underlying sandbox manager.
func (h *Handler) Manager() *Manager { return h.manager }

// SetConfigStore wires the kv-backed config store used for persistence.
func (h *Handler) SetConfigStore(store *appconfig.ConfigStore) {
	h.configStore = store
}

// SetNetworkConfigHook wires an optional runtime update callback.
func (h *Handler) SetNetworkConfigHook(hook func(networkEnabled bool)) {
	h.networkConfigHook = hook
}

// ExecuteRequest represents a request to execute code in the sandbox.
type ExecuteRequest struct {
	Command     string            `json:"command" validate:"required"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	WorkDir     string            `json:"work_dir,omitempty"`
	Stdin       string            `json:"stdin,omitempty"`
	TimeoutSecs int               `json:"timeout_secs,omitempty"`
	MemoryMB    int               `json:"memory_mb,omitempty"`
}

// UpdateConfigRequest represents supported sandbox runtime config updates.
type UpdateConfigRequest struct {
	NetworkEnabled *bool `json:"network_enabled,omitempty"`
}

// Execute handles POST /api/v1/sandbox/execute
func (h *Handler) Execute(c echo.Context) error {
	var req ExecuteRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Command == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "command is required")
	}

	// Create execution request
	execReq := NewExecutionRequest(req.Command, req.Args...)
	execReq.Env = req.Env
	execReq.WorkDir = req.WorkDir
	execReq.Stdin = req.Stdin

	if req.TimeoutSecs > 0 {
		execReq.Timeout = secondsToDuration(req.TimeoutSecs)
	}

	if req.MemoryMB > 0 {
		execReq.MemoryLimit = int64(req.MemoryMB) * 1024 * 1024
	}

	// Execute
	result, err := h.manager.Execute(c.Request().Context(), execReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, result)
}

// GetStatus handles GET /api/v1/sandbox/status/:id
func (h *Handler) GetStatus(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "execution ID is required")
	}

	result, err := h.manager.GetStatus(id)
	if err != nil {
		if err == ErrExecutionNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "execution not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, result)
}

// Kill handles POST /api/v1/sandbox/kill/:id
func (h *Handler) Kill(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "execution ID is required")
	}

	if err := h.manager.Kill(id); err != nil {
		if err == ErrExecutionNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "execution not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "killed",
		"message": "execution killed successfully",
	})
}

// UpdateConfig handles PATCH /api/v1/sandbox/config
func (h *Handler) UpdateConfig(c echo.Context) error {
	var req UpdateConfigRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if h.manager == nil || !h.manager.SupportsNetworkEnabled() {
		return echo.NewHTTPError(http.StatusBadRequest, "network_enabled is not supported on this platform")
	}
	if req.NetworkEnabled == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "network_enabled is required")
	}

	if err := h.updateNetworkEnabled(*req.NetworkEnabled); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return h.Info(c)
}

// Info handles GET /api/v1/sandbox/info
func (h *Handler) Info(c echo.Context) error {
	config := h.manager.GetConfig()
	info := map[string]interface{}{
		"supported":                h.manager.IsSupported(),
		"network_toggle_supported": h.manager.SupportsNetworkEnabled(),
		"default_timeout":          config.DefaultTimeout.String(),
		"max_timeout":              config.MaxTimeout.String(),
		"memory_limit":             config.MemoryLimit,
		"cpu_limit":                config.CPULimit,
		"process_limit":            config.ProcessLimit,
	}
	if h.manager.SupportsNetworkEnabled() {
		info["network_enabled"] = config.NetworkEnabled
	}
	if reason := h.manager.SupportReason(); reason != "" {
		info["support_reason"] = reason
	}

	return c.JSON(http.StatusOK, info)
}

// RegisterRoutes registers the sandbox routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.POST("/execute", h.Execute)
	g.GET("/status/:id", h.GetStatus)
	g.POST("/kill/:id", h.Kill)
	g.PATCH("/config", h.UpdateConfig)
	g.GET("/info", h.Info)
}

// secondsToDuration converts seconds to time.Duration.
func secondsToDuration(secs int) time.Duration {
	return time.Duration(secs) * time.Second
}

func (h *Handler) updateNetworkEnabled(enabled bool) error {
	if h.manager == nil {
		return fmt.Errorf("sandbox manager not initialized")
	}

	if h.configStore != nil {
		securityCfg := appconfig.SecurityConfig{}
		if current := h.configStore.Config(); current != nil {
			securityCfg = current.Security
		} else if raw, err := h.configStore.GetSection("security"); err == nil {
			if err := json.Unmarshal(raw, &securityCfg); err != nil {
				return fmt.Errorf("decode security config: %w", err)
			}
		}
		securityCfg.Sandbox.NetworkEnabled = enabled

		raw, err := json.Marshal(securityCfg)
		if err != nil {
			return fmt.Errorf("marshal security config: %w", err)
		}
		if err := h.configStore.SetSection("security", raw); err != nil {
			return fmt.Errorf("persist security config: %w", err)
		}
	}

	if runtimeCfg := h.manager.GetConfig(); runtimeCfg != nil {
		runtimeCfg.NetworkEnabled = enabled
	}
	if h.networkConfigHook != nil {
		h.networkConfigHook(enabled)
	}

	return nil
}
