package webhook

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for webhook management.
type Handler struct {
	service *Service
	logger  *zap.Logger
}

// NewHandler creates a new webhook handler.
func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger.With(zap.String("handler", "webhook")),
	}
}

// CreateRequest represents a create webhook request.
type CreateRequest struct {
	Name        string      `json:"name" validate:"required"`
	Type        WebhookType `json:"type" validate:"required"`
	Description string      `json:"description"`
}

// UpdateRequest represents an update webhook request.
type UpdateRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// RegisterRoutes registers webhook routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	webhooks := g.Group("/webhooks")
	webhooks.GET("", h.List)
	webhooks.POST("", h.Create)
	webhooks.GET("/:id", h.Get)
	webhooks.PUT("/:id", h.Update)
	webhooks.DELETE("/:id", h.Delete)
	webhooks.POST("/:id/regenerate-secret", h.RegenerateSecret)
	webhooks.GET("/:id/events", h.GetEvents)
}

// List returns all webhooks.
// @Summary List webhooks
// @Tags webhooks
// @Produce json
// @Success 200 {array} Webhook
// @Router /api/v1/webhooks [get]
func (h *Handler) List(c echo.Context) error {
	webhooks := h.service.List()

	// Hide secrets in response
	for _, w := range webhooks {
		w.Secret = ""
	}

	return c.JSON(http.StatusOK, webhooks)
}

// Create creates a new webhook.
// @Summary Create webhook
// @Tags webhooks
// @Accept json
// @Produce json
// @Param request body CreateRequest true "Create request"
// @Success 201 {object} Webhook
// @Failure 400 {object} map[string]string
// @Router /api/v1/webhooks [post]
func (h *Handler) Create(c echo.Context) error {
	var req CreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if req.Name == "" || req.Type == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name and type are required"})
	}

	webhook, err := h.service.Create(req.Name, req.Type, req.Description)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, webhook)
}

// Get returns a webhook by ID.
// @Summary Get webhook
// @Tags webhooks
// @Produce json
// @Param id path string true "Webhook ID"
// @Success 200 {object} Webhook
// @Failure 404 {object} map[string]string
// @Router /api/v1/webhooks/{id} [get]
func (h *Handler) Get(c echo.Context) error {
	id := c.Param("id")

	webhook, exists := h.service.Get(id)
	if !exists {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "webhook not found"})
	}

	// Hide secret
	webhook.Secret = ""

	return c.JSON(http.StatusOK, webhook)
}

// Update updates a webhook.
// @Summary Update webhook
// @Tags webhooks
// @Accept json
// @Produce json
// @Param id path string true "Webhook ID"
// @Param request body UpdateRequest true "Update request"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/webhooks/{id} [put]
func (h *Handler) Update(c echo.Context) error {
	id := c.Param("id")

	var req UpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.service.Update(id, req.Name, req.Description, req.Enabled); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

// Delete deletes a webhook.
// @Summary Delete webhook
// @Tags webhooks
// @Param id path string true "Webhook ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/webhooks/{id} [delete]
func (h *Handler) Delete(c echo.Context) error {
	id := c.Param("id")

	if err := h.service.Delete(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// RegenerateSecret regenerates the webhook secret.
// @Summary Regenerate webhook secret
// @Tags webhooks
// @Produce json
// @Param id path string true "Webhook ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/webhooks/{id}/regenerate-secret [post]
func (h *Handler) RegenerateSecret(c echo.Context) error {
	id := c.Param("id")

	secret, err := h.service.RegenerateSecret(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"secret": secret})
}

// GetEvents returns events for a webhook.
// @Summary Get webhook events
// @Tags webhooks
// @Produce json
// @Param id path string true "Webhook ID"
// @Param limit query int false "Limit" default(20)
// @Success 200 {array} WebhookEvent
// @Failure 404 {object} map[string]string
// @Router /api/v1/webhooks/{id}/events [get]
func (h *Handler) GetEvents(c echo.Context) error {
	id := c.Param("id")
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	events, err := h.service.GetEvents(id, limit)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, events)
}

// IngressHandler handles incoming webhook requests.
// This should be mounted at /hooks/:id
func (h *Handler) IngressHandler(c echo.Context) error {
	id := c.Param("id")
	h.service.HandleRequest(c.Response().Writer, c.Request(), id)
	return nil
}
