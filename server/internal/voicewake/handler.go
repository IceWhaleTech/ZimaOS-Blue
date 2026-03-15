package voicewake

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	manager *Manager
}

func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/status", h.GetStatus)
	g.POST("/restart", h.Restart)
}

func (h *Handler) GetStatus(c echo.Context) error {
	if h == nil || h.manager == nil {
		return c.JSON(http.StatusOK, Status{Supported: false, Reason: "unsupported"})
	}
	return c.JSON(http.StatusOK, h.manager.Status())
}

func (h *Handler) Restart(c echo.Context) error {
	if h == nil || h.manager == nil {
		return c.JSON(http.StatusOK, Status{Supported: false, Reason: "unsupported"})
	}
	_ = h.manager.Restart(c.Request().Context())
	return c.JSON(http.StatusOK, h.manager.Status())
}
