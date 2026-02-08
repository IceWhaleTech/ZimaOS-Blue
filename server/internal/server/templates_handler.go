package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// TemplatesHandler handles templates API requests
type TemplatesHandler struct{}

// NewTemplatesHandler creates a new templates handler
func NewTemplatesHandler() *TemplatesHandler {
	return &TemplatesHandler{}
}

// GetTemplates returns available templates
func (h *TemplatesHandler) GetTemplates(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"templates": []interface{}{},
	})
}

// RegisterRoutes registers templates routes
func (h *TemplatesHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/templates", h.GetTemplates)
}
