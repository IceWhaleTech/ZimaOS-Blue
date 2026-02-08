package view

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/personality/controller"
)

// Handler handles personality API requests
type Handler struct {
	service *controller.Service
}

// NewHandler creates a new handler
func NewHandler(service *controller.Service) *Handler {
	return &Handler{service: service}
}

// Create creates a new personality
func (h *Handler) Create(c echo.Context) error {
	var req struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		SystemPrompt  string `json:"system_prompt"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	p, err := h.service.Create(req.Name, req.Description, req.SystemPrompt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, p)
}

// List lists all personalities
func (h *Handler) List(c echo.Context) error {
	personalities, err := h.service.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, personalities)
}

// GetByID gets a personality by ID
func (h *Handler) GetByID(c echo.Context) error {
	id := c.Param("id")
	p, err := h.service.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	return c.JSON(http.StatusOK, p)
}

// Update updates a personality
func (h *Handler) Update(c echo.Context) error {
	id := c.Param("id")
	var req struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	p, err := h.service.Update(id, req.Name, req.Description, req.SystemPrompt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, p)
}

// Delete deletes a personality
func (h *Handler) Delete(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.Delete(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}

// Activate activates a personality
func (h *Handler) Activate(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.Activate(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "activated"})
}

// GetActive gets the active personality
func (h *Handler) GetActive(c echo.Context) error {
	p, err := h.service.GetActive()
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no active personality"})
	}
	return c.JSON(http.StatusOK, p)
}

// RegisterRoutes registers personality routes
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.GetByID)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.POST("/:id/activate", h.Activate)
	g.GET("/active", h.GetActive)
}
