package tts

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler handles TTS management HTTP requests.
type Handler struct {
	service Service
}

// NewHandler creates a new TTS handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the TTS management routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/providers", h.ListProviders)
	g.GET("/voices", h.ListVoices)
}

// ListProviders returns available TTS providers.
func (h *Handler) ListProviders(c echo.Context) error {
	providers := h.service.ListProviders()

	result := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		info := map[string]interface{}{
			"type":    string(p),
			"enabled": true,
		}
		result = append(result, info)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"providers":        result,
		"default_provider": string(h.service.GetDefaultProvider()),
	})
}

// ListVoices returns available TTS voices.
func (h *Handler) ListVoices(c echo.Context) error {
	voices, err := h.service.ListVoices(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"voices": voices,
	})
}
