package cron

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for cron job management.
type Handler struct {
	service *Service
	logger  *zap.Logger
}

// NewHandler creates a new cron handler.
func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger.With(zap.String("handler", "cron")),
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

// RegisterRoutes registers cron routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	cron := g.Group("/cron")
	cron.GET("", h.List)
	cron.POST("", h.Create)
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
	jobs := h.service.List()
	return c.JSON(http.StatusOK, jobs)
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

	job, err := h.service.Create(req.Name, req.Description, req.Schedule, req.Handler, req.Payload)
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

	job, exists := h.service.Get(id)
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

	if err := h.service.Update(id, req.Name, req.Description, req.Schedule, req.Payload); err != nil {
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

	if err := h.service.Delete(id); err != nil {
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

	if err := h.service.Enable(id); err != nil {
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

	if err := h.service.Disable(id); err != nil {
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

	if err := h.service.Trigger(id); err != nil {
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
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	executions, err := h.service.GetExecutions(id, limit)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, executions)
}
