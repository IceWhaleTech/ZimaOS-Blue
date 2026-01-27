package homeassistant

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler handles Home Assistant HTTP requests.
type Handler struct {
	service Service
}

// NewHandler creates a new Home Assistant handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the Home Assistant routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.POST("/connect", h.Connect)
	g.POST("/disconnect", h.Disconnect)
	g.GET("/status", h.Status)

	g.GET("/entities", h.GetEntities)
	g.GET("/entities/:id", h.GetEntity)
	g.POST("/entities/:id/control", h.ControlEntity)

	g.GET("/scenes", h.GetScenes)
	g.POST("/scenes/:id/activate", h.ActivateScene)

	g.GET("/automations", h.GetAutomations)
	g.POST("/automations/:id/trigger", h.TriggerAutomation)
	g.POST("/automations/:id/toggle", h.ToggleAutomation)

	g.POST("/command", h.ProcessCommand)
}

// ConnectRequest represents the connect API request.
type connectRequest struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// Connect connects to Home Assistant.
func (h *Handler) Connect(c echo.Context) error {
	var req connectRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}
	if req.Token == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "token is required")
	}

	config := &Config{
		URL:   req.URL,
		Token: req.Token,
	}

	if err := h.service.Connect(c.Request().Context(), config); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "connected",
		"message": "Successfully connected to Home Assistant",
	})
}

// Disconnect disconnects from Home Assistant.
func (h *Handler) Disconnect(c echo.Context) error {
	if err := h.service.Disconnect(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "disconnected",
		"message": "Disconnected from Home Assistant",
	})
}

// Status returns the connection status.
func (h *Handler) Status(c echo.Context) error {
	connected := h.service.IsConnected()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"connected": connected,
	})
}

// GetEntities returns all entities or filtered by domain.
func (h *Handler) GetEntities(c echo.Context) error {
	if !h.service.IsConnected() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not connected to Home Assistant")
	}

	domain := c.QueryParam("domain")

	var entities []*Entity
	var err error

	if domain != "" {
		entities, err = h.service.GetEntitiesByDomain(c.Request().Context(), domain)
	} else {
		entities, err = h.service.GetAllEntities(c.Request().Context())
	}

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, entities)
}

// GetEntity returns a single entity.
func (h *Handler) GetEntity(c echo.Context) error {
	if !h.service.IsConnected() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not connected to Home Assistant")
	}

	entityID := c.Param("id")
	if entityID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "entity ID is required")
	}

	entity, err := h.service.GetEntity(c.Request().Context(), entityID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	return c.JSON(http.StatusOK, entity)
}

// ControlEntityRequest represents the control entity API request.
type controlEntityRequest struct {
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
}

// ControlEntity controls an entity.
func (h *Handler) ControlEntity(c echo.Context) error {
	if !h.service.IsConnected() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not connected to Home Assistant")
	}

	entityID := c.Param("id")
	if entityID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "entity ID is required")
	}

	var req controlEntityRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.Action == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "action is required")
	}

	if err := h.service.ControlEntity(c.Request().Context(), entityID, req.Action, req.Parameters); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "success",
		"entity_id": entityID,
		"action":    req.Action,
	})
}

// GetScenes returns all scenes.
func (h *Handler) GetScenes(c echo.Context) error {
	if !h.service.IsConnected() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not connected to Home Assistant")
	}

	scenes, err := h.service.GetScenes(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, scenes)
}

// ActivateScene activates a scene.
func (h *Handler) ActivateScene(c echo.Context) error {
	if !h.service.IsConnected() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not connected to Home Assistant")
	}

	sceneID := c.Param("id")
	if sceneID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "scene ID is required")
	}

	if err := h.service.ActivateScene(c.Request().Context(), sceneID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":   "success",
		"scene_id": sceneID,
	})
}

// GetAutomations returns all automations.
func (h *Handler) GetAutomations(c echo.Context) error {
	if !h.service.IsConnected() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not connected to Home Assistant")
	}

	automations, err := h.service.GetAutomations(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, automations)
}

// TriggerAutomation triggers an automation.
func (h *Handler) TriggerAutomation(c echo.Context) error {
	if !h.service.IsConnected() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not connected to Home Assistant")
	}

	automationID := c.Param("id")
	if automationID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "automation ID is required")
	}

	if err := h.service.TriggerAutomation(c.Request().Context(), automationID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":        "success",
		"automation_id": automationID,
	})
}

// ToggleAutomationRequest represents the toggle automation API request.
type toggleAutomationRequest struct {
	Enable bool `json:"enable"`
}

// ToggleAutomation enables or disables an automation.
func (h *Handler) ToggleAutomation(c echo.Context) error {
	if !h.service.IsConnected() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not connected to Home Assistant")
	}

	automationID := c.Param("id")
	if automationID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "automation ID is required")
	}

	var req toggleAutomationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.service.ToggleAutomation(c.Request().Context(), automationID, req.Enable); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	status := "disabled"
	if req.Enable {
		status = "enabled"
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":        status,
		"automation_id": automationID,
	})
}

// CommandRequest represents the natural language command API request.
type commandRequest struct {
	Command string `json:"command"`
}

// ProcessCommand processes a natural language command.
func (h *Handler) ProcessCommand(c echo.Context) error {
	if !h.service.IsConnected() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not connected to Home Assistant")
	}

	var req commandRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.Command == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "command is required")
	}

	result, err := h.service.ProcessCommand(c.Request().Context(), req.Command)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, result)
}
