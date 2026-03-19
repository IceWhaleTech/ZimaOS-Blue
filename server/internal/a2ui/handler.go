package a2ui

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles A2UI HTTP requests.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new A2UI handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers the A2UI routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/canvases", h.ListCanvases)
	g.POST("/canvases", h.CreateCanvas)
	g.GET("/canvases/:id", h.GetCanvas)
	g.PUT("/canvases/:id", h.UpdateCanvas)
	g.DELETE("/canvases/:id", h.DeleteCanvas)
	g.POST("/canvases/:id/actions/:actionId", h.ExecuteAction)
}

// ListCanvases returns all canvas IDs.
func (h *Handler) ListCanvases(c echo.Context) error {
	ids := h.manager.ListCanvases()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"canvases": ids,
		"count":    len(ids),
	})
}

// createCanvasRequest represents the create canvas API request.
type createCanvasRequest struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Components  componentList          `json:"components"`
	Layout      string                 `json:"layout"`
	Metadata    map[string]interface{} `json:"metadata"`
	TTLSeconds  int                    `json:"ttl_seconds"`
}

// CreateCanvas creates a new canvas.
func (h *Handler) CreateCanvas(c echo.Context) error {
	var req createCanvasRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.ID == "" {
		req.ID = "canvas_" + uuid.New().String()
	}

	canvas := &Canvas{
		ID:          req.ID,
		Title:       req.Title,
		Description: req.Description,
		Components:  []Component(req.Components),
		Layout:      req.Layout,
		Metadata:    req.Metadata,
		CreatedAt:   time.Now(),
	}

	if req.TTLSeconds > 0 {
		expiry := canvas.CreatedAt.Add(secondsToDuration(req.TTLSeconds))
		canvas.ExpiresAt = &expiry
	}

	if err := h.manager.CreateCanvas(canvas); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, canvas)
}

// GetCanvas returns a canvas by ID.
func (h *Handler) GetCanvas(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "canvas ID is required")
	}

	canvas, err := h.manager.GetCanvas(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	return c.JSON(http.StatusOK, canvas)
}

// updateCanvasRequest represents the update canvas API request.
type updateCanvasRequest struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Components  componentList          `json:"components"`
	Layout      string                 `json:"layout"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// UpdateCanvas updates an existing canvas.
func (h *Handler) UpdateCanvas(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "canvas ID is required")
	}

	// Get existing canvas first
	existing, err := h.manager.GetCanvas(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	var req updateCanvasRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Update fields
	if req.Title != "" {
		existing.Title = req.Title
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.Components != nil {
		existing.Components = []Component(req.Components)
	}
	if req.Layout != "" {
		existing.Layout = req.Layout
	}
	if req.Metadata != nil {
		existing.Metadata = req.Metadata
	}

	if err := h.manager.UpdateCanvas(existing); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, existing)
}

// DeleteCanvas deletes a canvas.
func (h *Handler) DeleteCanvas(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "canvas ID is required")
	}

	// Check if canvas exists
	if _, err := h.manager.GetCanvas(id); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	h.manager.DeleteCanvas(id)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "deleted",
		"canvas_id": id,
	})
}

// executeActionRequest represents the execute action API request.
type executeActionRequest struct {
	FormData map[string]interface{} `json:"form_data"`
}

// ExecuteAction executes an action on a canvas.
func (h *Handler) ExecuteAction(c echo.Context) error {
	canvasID := c.Param("id")
	if canvasID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "canvas ID is required")
	}

	actionID := c.Param("actionId")
	if actionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "action ID is required")
	}

	var req executeActionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	result, err := h.manager.ExecuteAction(c.Request().Context(), canvasID, actionID, req.FormData)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, result)
}

// secondsToDuration converts seconds to time.Duration.
func secondsToDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}
