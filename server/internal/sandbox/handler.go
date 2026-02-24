package sandbox

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// Handler handles sandbox API endpoints.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new sandbox handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// Manager returns the underlying sandbox manager.
func (h *Handler) Manager() *Manager { return h.manager }

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

// Info handles GET /api/v1/sandbox/info
func (h *Handler) Info(c echo.Context) error {
	config := h.manager.GetConfig()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"supported":       h.manager.IsSupported(),
		"default_timeout": config.DefaultTimeout.String(),
		"max_timeout":     config.MaxTimeout.String(),
		"memory_limit":    config.MemoryLimit,
		"cpu_limit":       config.CPULimit,
		"process_limit":   config.ProcessLimit,
		"network_enabled": config.NetworkEnabled,
	})
}

// RegisterRoutes registers the sandbox routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.POST("/execute", h.Execute)
	g.GET("/status/:id", h.GetStatus)
	g.POST("/kill/:id", h.Kill)
	g.GET("/info", h.Info)
}

// secondsToDuration converts seconds to time.Duration.
func secondsToDuration(secs int) time.Duration {
	return time.Duration(secs) * time.Second
}
