package autoreply

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for auto-reply management.
type Handler struct {
	service *Service
	logger  *zap.Logger
}

// NewHandler creates a new auto-reply handler.
func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger.With(zap.String("handler", "autoreply")),
	}
}

// CreateRequest represents a create rule request.
type CreateRequest struct {
	Name         string      `json:"name" validate:"required"`
	TriggerType  TriggerType `json:"trigger_type" validate:"required"`
	TriggerValue string      `json:"trigger_value" validate:"required"`
	Responses    []string    `json:"responses" validate:"required"`
	Priority     int         `json:"priority"`
}

// UpdateRequest represents an update rule request.
type UpdateRequest struct {
	Name         string      `json:"name" validate:"required"`
	TriggerType  TriggerType `json:"trigger_type" validate:"required"`
	TriggerValue string      `json:"trigger_value" validate:"required"`
	Responses    []string    `json:"responses" validate:"required"`
	Priority     int         `json:"priority"`
}

// TestRequest represents a test request.
type TestRequest struct {
	Message string `json:"message" validate:"required"`
	Channel string `json:"channel"`
}

// TestResponse represents a test response.
type TestResponse struct {
	Matched  bool   `json:"matched"`
	RuleID   string `json:"rule_id,omitempty"`
	RuleName string `json:"rule_name,omitempty"`
	Response string `json:"response,omitempty"`
}

// RegisterRoutes registers auto-reply routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	autoreply := g.Group("/autoreply")
	autoreply.GET("/rules", h.List)
	autoreply.POST("/rules", h.Create)
	autoreply.GET("/rules/:id", h.Get)
	autoreply.PUT("/rules/:id", h.Update)
	autoreply.DELETE("/rules/:id", h.Delete)
	autoreply.POST("/rules/:id/enable", h.Enable)
	autoreply.POST("/rules/:id/disable", h.Disable)
	autoreply.PUT("/rules/:id/channels", h.SetChannels)
	autoreply.POST("/test", h.Test)
}

// List returns all auto-reply rules.
// @Summary List auto-reply rules
// @Tags autoreply
// @Produce json
// @Success 200 {array} Rule
// @Router /api/v1/autoreply/rules [get]
func (h *Handler) List(c echo.Context) error {
	rules := h.service.List()
	return c.JSON(http.StatusOK, rules)
}

// Create creates a new auto-reply rule.
// @Summary Create auto-reply rule
// @Tags autoreply
// @Accept json
// @Produce json
// @Param request body CreateRequest true "Create request"
// @Success 201 {object} Rule
// @Failure 400 {object} map[string]string
// @Router /api/v1/autoreply/rules [post]
func (h *Handler) Create(c echo.Context) error {
	var req CreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if req.Name == "" || req.TriggerValue == "" || len(req.Responses) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name, trigger_value, and responses are required"})
	}

	rule, err := h.service.Create(req.Name, req.TriggerType, req.TriggerValue, req.Responses, req.Priority)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, rule)
}

// Get returns an auto-reply rule by ID.
// @Summary Get auto-reply rule
// @Tags autoreply
// @Produce json
// @Param id path string true "Rule ID"
// @Success 200 {object} Rule
// @Failure 404 {object} map[string]string
// @Router /api/v1/autoreply/rules/{id} [get]
func (h *Handler) Get(c echo.Context) error {
	id := c.Param("id")

	rule, exists := h.service.Get(id)
	if !exists {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "rule not found"})
	}

	return c.JSON(http.StatusOK, rule)
}

// Update updates an auto-reply rule.
// @Summary Update auto-reply rule
// @Tags autoreply
// @Accept json
// @Produce json
// @Param id path string true "Rule ID"
// @Param request body UpdateRequest true "Update request"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/autoreply/rules/{id} [put]
func (h *Handler) Update(c echo.Context) error {
	id := c.Param("id")

	var req UpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.service.Update(id, req.Name, req.TriggerType, req.TriggerValue, req.Responses, req.Priority); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

// Delete deletes an auto-reply rule.
// @Summary Delete auto-reply rule
// @Tags autoreply
// @Param id path string true "Rule ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/autoreply/rules/{id} [delete]
func (h *Handler) Delete(c echo.Context) error {
	id := c.Param("id")

	if err := h.service.Delete(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// Enable enables an auto-reply rule.
// @Summary Enable auto-reply rule
// @Tags autoreply
// @Param id path string true "Rule ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/autoreply/rules/{id}/enable [post]
func (h *Handler) Enable(c echo.Context) error {
	id := c.Param("id")

	if err := h.service.Enable(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "enabled"})
}

// Disable disables an auto-reply rule.
// @Summary Disable auto-reply rule
// @Tags autoreply
// @Param id path string true "Rule ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/autoreply/rules/{id}/disable [post]
func (h *Handler) Disable(c echo.Context) error {
	id := c.Param("id")

	if err := h.service.Disable(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "disabled"})
}

// SetChannelsRequest represents a set channels request.
type SetChannelsRequest struct {
	Channels []string `json:"channels"`
}

// SetChannels sets the channels for an auto-reply rule.
// @Summary Set channels for auto-reply rule
// @Tags autoreply
// @Accept json
// @Produce json
// @Param id path string true "Rule ID"
// @Param request body SetChannelsRequest true "Set channels request"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/autoreply/rules/{id}/channels [put]
func (h *Handler) SetChannels(c echo.Context) error {
	id := c.Param("id")

	var req SetChannelsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.service.SetChannels(id, req.Channels); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

// Test tests a message against all rules.
// @Summary Test auto-reply rules
// @Tags autoreply
// @Accept json
// @Produce json
// @Param request body TestRequest true "Test request"
// @Success 200 {object} TestResponse
// @Router /api/v1/autoreply/test [post]
func (h *Handler) Test(c echo.Context) error {
	var req TestRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if req.Message == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "message is required"})
	}

	response, rule, err := h.service.Test(req.Message, req.Channel)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	result := TestResponse{
		Matched: rule != nil,
	}

	if rule != nil {
		result.RuleID = rule.ID
		result.RuleName = rule.Name
		result.Response = response
	}

	return c.JSON(http.StatusOK, result)
}

// Unused but kept for potential future use
var _ = strconv.Atoi
