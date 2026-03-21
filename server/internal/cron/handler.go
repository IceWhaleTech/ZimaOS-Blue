package cron

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/reclaim"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for cron job management.
type Handler struct {
	service *Service
	logger  *zap.Logger

	// Lazy init support
	initFn          func() *Service
	serviceInitHook func(*Service)
	lazy            *reclaim.Managed[*Service]
}

// NewHandler creates a new cron handler.
func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger.With(zap.String("handler", "cron")),
	}
}

// NewLazyHandler creates a handler that defers Service creation to the first API call.
// This avoids starting the robfig/cron scheduler goroutine when nobody uses cron.
func NewLazyHandler(initFn func() *Service, logger *zap.Logger) *Handler {
	h := &Handler{
		logger: logger.With(zap.String("handler", "cron")),
		initFn: initFn,
	}
	h.lazy = reclaim.NewManaged[*Service](0, func() (*Service, error) {
		if h.initFn == nil {
			return nil, nil
		}
		svc := h.initFn()
		if svc == nil {
			return nil, fmt.Errorf("cron service unavailable")
		}
		return svc, nil
	}, func(_ context.Context, svc *Service) error {
		if svc == nil {
			return nil
		}
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return svc.Stop(stopCtx)
	})
	return h
}

// svc returns the service, initializing lazily if needed.
func (h *Handler) svc() *Service {
	if h == nil {
		return nil
	}
	if h.service != nil || h.lazy == nil {
		return h.service
	}
	svc, _ := h.lazy.Get()
	return svc
}

// GetService returns the cron service for cleanup purposes.
func (h *Handler) GetService() *Service {
	return h.svc()
}

// SetIdleReclaim configures idle reclaim for the lazily initialized cron service.
func (h *Handler) SetIdleReclaim(idleAfter time.Duration) {
	if h == nil || h.lazy == nil {
		return
	}
	h.lazy.SetIdleAfter(idleAfter)
}

func (h *Handler) withService(fn func(*Service) error) error {
	if h == nil {
		return nil
	}
	if h.service != nil || h.lazy == nil {
		return fn(h.service)
	}
	svc, release, err := h.lazy.Acquire()
	if err != nil {
		return err
	}
	defer release()
	return fn(svc)
}

// SetServiceInitHook configures a callback that runs once the lazy service is created.
func (h *Handler) SetServiceInitHook(fn func(*Service)) {
	if h == nil || fn == nil {
		return
	}
	if h.serviceInitHook == nil {
		h.serviceInitHook = fn
	} else {
		prev := h.serviceInitHook
		h.serviceInitHook = func(svc *Service) {
			prev(svc)
			fn(svc)
		}
	}
	if h.lazy != nil {
		svc, ok := h.lazy.Peek()
		h.lazy.SetOnCreateSilently(h.serviceInitHook)
		if ok && svc != nil {
			fn(svc)
		}
		return
	}
	if h.service != nil {
		fn(h.service)
	}
}

// CreateRequest represents a create job request.
type CreateRequest struct {
	Name        string                 `json:"name" validate:"required"`
	Description string                 `json:"description"`
	Schedule    string                 `json:"schedule" validate:"required"`
	Handler     string                 `json:"handler" validate:"required"`
	Payload     map[string]interface{} `json:"payload"`
}

// UpdateRequest represents an update job request.
type UpdateRequest struct {
	Name        string                 `json:"name" validate:"required"`
	Description string                 `json:"description"`
	Schedule    string                 `json:"schedule" validate:"required"`
	Payload     map[string]interface{} `json:"payload"`
}

type listResponse struct {
	Jobs []*Job `json:"jobs"`
}

type executionsResponse struct {
	Executions []*JobExecution `json:"executions"`
}

type statusResponse struct {
	Running    bool `json:"running"`
	JobCount   int  `json:"job_count"`
	ActiveJobs int  `json:"active_jobs"`
}

// RegisterRoutes registers cron routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	cron := g.Group("/cron")

	cron.GET("", h.List)
	cron.POST("", h.Create)

	cron.GET("/jobs", h.ListWrapped)
	cron.POST("/jobs", h.Create)
	cron.GET("/status", h.Status)
	cron.GET("/jobs/:id/executions", h.GetExecutionsWrapped)
	cron.DELETE("/jobs/:id", h.Delete)
	cron.POST("/jobs/:id/enable", h.Enable)
	cron.POST("/jobs/:id/disable", h.Disable)
	cron.POST("/jobs/:id/trigger", h.Trigger)

	cron.GET("/:id", h.Get)
	cron.PUT("/:id", h.Update)
	cron.DELETE("/:id", h.Delete)
	cron.POST("/:id/enable", h.Enable)
	cron.POST("/:id/disable", h.Disable)
	cron.POST("/:id/trigger", h.Trigger)
	cron.GET("/:id/executions", h.GetExecutions)
}

// List returns all cron jobs.
// @Summary List cron jobs
// @Tags cron
// @Produce json
// @Success 200 {array} Job
// @Router /api/v1/cron [get]
func (h *Handler) List(c echo.Context) error {
	var jobs []*Job
	if err := h.withService(func(svc *Service) error {
		jobs = svc.List()
		return nil
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, jobs)
}

// ListWrapped returns cron jobs in the legacy CLI response shape.
func (h *Handler) ListWrapped(c echo.Context) error {
	var jobs []*Job
	if err := h.withService(func(svc *Service) error {
		jobs = svc.List()
		return nil
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, listResponse{Jobs: jobs})
}

// Status returns scheduler status in the legacy CLI response shape.
func (h *Handler) Status(c echo.Context) error {
	var service *Service
	if err := h.withService(func(svc *Service) error {
		service = svc
		return nil
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	jobs := service.List()
	activeJobs := 0
	for _, job := range jobs {
		if job.Enabled {
			activeJobs++
		}
	}

	return c.JSON(http.StatusOK, statusResponse{
		Running:    service.Config().Enabled,
		JobCount:   len(jobs),
		ActiveJobs: activeJobs,
	})
}

// Create creates a new cron job.
// @Summary Create cron job
// @Tags cron
// @Accept json
// @Produce json
// @Param request body CreateRequest true "Create request"
// @Success 201 {object} Job
// @Failure 400 {object} map[string]string
// @Router /api/v1/cron [post]
func (h *Handler) Create(c echo.Context) error {
	var req CreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if req.Name == "" || req.Schedule == "" || req.Handler == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name, schedule, and handler are required"})
	}

	var job *Job
	err := h.withService(func(svc *Service) error {
		var err error
		job, err = svc.Create(req.Name, req.Description, req.Schedule, req.Handler, req.Payload)
		return err
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, job)
}

// Get returns a cron job by ID.
// @Summary Get cron job
// @Tags cron
// @Produce json
// @Param id path string true "Job ID"
// @Success 200 {object} Job
// @Failure 404 {object} map[string]string
// @Router /api/v1/cron/{id} [get]
func (h *Handler) Get(c echo.Context) error {
	id := c.Param("id")

	var (
		job    *Job
		exists bool
	)
	if err := h.withService(func(svc *Service) error {
		job, exists = svc.Get(id)
		return nil
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if !exists {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "job not found"})
	}

	return c.JSON(http.StatusOK, job)
}

// Update updates a cron job.
// @Summary Update cron job
// @Tags cron
// @Accept json
// @Produce json
// @Param id path string true "Job ID"
// @Param request body UpdateRequest true "Update request"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/cron/{id} [put]
func (h *Handler) Update(c echo.Context) error {
	id := c.Param("id")

	var req UpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.withService(func(svc *Service) error {
		return svc.Update(id, req.Name, req.Description, req.Schedule, req.Payload)
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

// Delete deletes a cron job.
// @Summary Delete cron job
// @Tags cron
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/cron/{id} [delete]
func (h *Handler) Delete(c echo.Context) error {
	id := c.Param("id")

	if err := h.withService(func(svc *Service) error {
		return svc.Delete(id)
	}); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// Enable enables a cron job.
// @Summary Enable cron job
// @Tags cron
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/cron/{id}/enable [post]
func (h *Handler) Enable(c echo.Context) error {
	id := c.Param("id")

	if err := h.withService(func(svc *Service) error {
		return svc.Enable(id)
	}); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "enabled"})
}

// Disable disables a cron job.
// @Summary Disable cron job
// @Tags cron
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/cron/{id}/disable [post]
func (h *Handler) Disable(c echo.Context) error {
	id := c.Param("id")

	if err := h.withService(func(svc *Service) error {
		return svc.Disable(id)
	}); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "disabled"})
}

// Trigger manually triggers a cron job.
// @Summary Trigger cron job
// @Tags cron
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/cron/{id}/trigger [post]
func (h *Handler) Trigger(c echo.Context) error {
	id := c.Param("id")

	if err := h.withService(func(svc *Service) error {
		return svc.Trigger(id)
	}); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "triggered"})
}

// GetExecutions returns executions for a cron job.
// @Summary Get job executions
// @Tags cron
// @Produce json
// @Param id path string true "Job ID"
// @Param limit query int false "Limit" default(20)
// @Success 200 {array} JobExecution
// @Failure 404 {object} map[string]string
// @Router /api/v1/cron/{id}/executions [get]
func (h *Handler) GetExecutions(c echo.Context) error {
	id := c.Param("id")
	limit := parseExecutionLimit(c)

	var executions []*JobExecution
	err := h.withService(func(svc *Service) error {
		var err error
		executions, err = svc.GetExecutions(id, limit)
		return err
	})
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, executions)
}

// GetExecutionsWrapped returns executions in the legacy CLI response shape.
func (h *Handler) GetExecutionsWrapped(c echo.Context) error {
	id := c.Param("id")
	limit := parseExecutionLimit(c)

	var executions []*JobExecution
	err := h.withService(func(svc *Service) error {
		var err error
		executions, err = svc.GetExecutions(id, limit)
		return err
	})
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, executionsResponse{Executions: executions})
}

func parseExecutionLimit(c echo.Context) int {
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	return limit
}
