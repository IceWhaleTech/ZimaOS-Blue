package mediagen

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// Handler provides HTTP endpoints for media generation.
type Handler struct {
	manager *Manager
	storage *MediaStorage
}

// NewHandler creates a new media generation handler.
func NewHandler(manager *Manager, storage *MediaStorage) *Handler {
	return &Handler{manager: manager, storage: storage}
}

// RegisterRoutes registers all media generation routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.POST("/images/generations", h.GenerateImage)
	g.POST("/videos/generations", h.GenerateVideo)
	g.GET("/tasks/:id", h.GetTask)
	g.GET("/tasks/:id/stream", h.StreamTask)
	g.GET("/models", h.ListModels)

	// Provider management
	g.GET("/providers", h.ListProviders)
	g.GET("/providers/:id", h.GetProvider)
	g.PUT("/providers/:id", h.UpdateProvider)
	g.POST("/providers/:id/enable", h.EnableProvider)
	g.POST("/providers/:id/disable", h.DisableProvider)
	g.POST("/providers/:id/keys", h.SetProviderKey)
	g.DELETE("/providers/:id/keys", h.RemoveProviderKey)
	g.POST("/providers/:id/test", h.TestProvider)
}

// RegisterStorageRoutes registers the static file serving route for generated media.
func (h *Handler) RegisterStorageRoutes(e *echo.Echo) {
	if h.storage != nil {
		e.GET("/api/media/generated/*", echo.WrapHandler(
			http.StripPrefix("/api/media/generated/", h.storage),
		))
	}
}

// GenerateImage handles POST /images/generations (OpenAI-compatible).
func (h *Handler) GenerateImage(c echo.Context) error {
	var req MediaRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request: " + err.Error()})
	}
	if req.Prompt == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "prompt is required"})
	}
	req.Type = MediaTypeImage
	if req.N == 0 {
		req.N = 1
	}

	// Check if client wants SSE streaming
	if c.QueryParam("stream") == "true" {
		return h.streamGeneration(c, &req)
	}

	task, err := h.manager.Generate(c.Request().Context(), &req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// For sync providers, wait for completion
	if task.Status == TaskStatusSucceeded {
		return c.JSON(http.StatusOK, task.Response)
	}

	// For async providers, wait with timeout
	waitCtx := c.Request().Context()
	result, err := h.manager.WaitForTask(waitCtx, task.ID)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"task_id": task.ID,
			"status":  string(task.Status),
			"message": "Image generation in progress. Poll GET /api/media/tasks/" + task.ID,
		})
	}
	return c.JSON(http.StatusOK, result.Response)
}

// GenerateVideo handles POST /videos/generations.
func (h *Handler) GenerateVideo(c echo.Context) error {
	var req MediaRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request: " + err.Error()})
	}
	if req.Prompt == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "prompt is required"})
	}
	req.Type = MediaTypeVideo

	// Check if client wants SSE streaming
	if c.QueryParam("stream") == "true" {
		return h.streamGeneration(c, &req)
	}

	task, err := h.manager.Generate(c.Request().Context(), &req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Video always returns task ID immediately (too slow to block)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"task_id":  task.ID,
		"status":   string(task.Status),
		"type":     string(task.Type),
		"progress": task.Progress,
		"message":  "Video generation started. Stream progress at GET /api/media/tasks/" + task.ID + "/stream",
	})
}

// GetTask handles GET /tasks/:id.
func (h *Handler) GetTask(c echo.Context) error {
	task, err := h.manager.GetTask(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, task)
}

// StreamTask handles GET /tasks/:id/stream — SSE endpoint for real-time progress.
// Frontend connects to this for loading animations and progressive status updates.
func (h *Handler) StreamTask(c echo.Context) error {
	taskID := c.Param("id")

	// Verify task exists
	if _, err := h.manager.GetTask(taskID); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			task, err := h.manager.GetTask(taskID)
			if err != nil {
				writeSSE(w, "error", map[string]string{"error": err.Error()})
				return nil
			}

			evt := map[string]interface{}{
				"id":       task.ID,
				"status":   string(task.Status),
				"progress": task.Progress,
				"type":     string(task.Type),
			}

			switch task.Status {
			case TaskStatusSucceeded:
				if task.Response != nil {
					evt["response"] = task.Response
				}
				writeSSE(w, "complete", evt)
				writeSSE(w, "done", "[DONE]")
				return nil
			case TaskStatusFailed:
				evt["error"] = task.Error
				writeSSE(w, "error", evt)
				return nil
			default:
				writeSSE(w, "progress", evt)
			}
		}
	}
}

// ListModels handles GET /models.
func (h *Handler) ListModels(c echo.Context) error {
	models := h.manager.Models()
	if models == nil {
		models = []MediaModelInfo{}
	}

	// Optional type filter
	if typeFilter := c.QueryParam("type"); typeFilter != "" {
		filtered := make([]MediaModelInfo, 0)
		for _, m := range models {
			if string(m.Type) == typeFilter {
				filtered = append(filtered, m)
			}
		}
		models = filtered
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": models,
	})
}

// streamGeneration starts generation and streams progress via SSE.
func (h *Handler) streamGeneration(c echo.Context, req *MediaRequest) error {
	task, err := h.manager.Generate(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Send initial event
	writeSSE(w, "started", map[string]interface{}{
		"task_id": task.ID,
		"status":  string(task.Status),
		"type":    string(task.Type),
	})

	// If already complete (sync provider), send result immediately
	if task.Status == TaskStatusSucceeded {
		evt := map[string]interface{}{
			"id":       task.ID,
			"status":   "succeeded",
			"progress": 1.0,
		}
		if task.Response != nil {
			evt["response"] = task.Response
		}
		writeSSE(w, "complete", evt)
		writeSSE(w, "done", "[DONE]")
		return nil
	}

	// Poll and stream progress
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			current, err := h.manager.GetTask(task.ID)
			if err != nil {
				writeSSE(w, "error", map[string]string{"error": err.Error()})
				return nil
			}

			evt := map[string]interface{}{
				"id":       current.ID,
				"status":   string(current.Status),
				"progress": current.Progress,
			}

			switch current.Status {
			case TaskStatusSucceeded:
				if current.Response != nil {
					evt["response"] = current.Response
				}
				writeSSE(w, "complete", evt)
				writeSSE(w, "done", "[DONE]")
				return nil
			case TaskStatusFailed:
				evt["error"] = current.Error
				writeSSE(w, "error", evt)
				return nil
			default:
				writeSSE(w, "progress", evt)
			}
		}
	}
}

// writeSSE writes a Server-Sent Event to the response.
func writeSSE(w *echo.Response, event string, data interface{}) {
	var payload []byte
	switch v := data.(type) {
	case string:
		payload = []byte(v)
	default:
		payload, _ = json.Marshal(v)
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	w.Flush()
}

// --- Provider management endpoints ---

// ListProviders handles GET /providers.
func (h *Handler) ListProviders(c echo.Context) error {
	configs := h.manager.ListConfigs()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"providers": configs,
	})
}

// GetProvider handles GET /providers/:id.
func (h *Handler) GetProvider(c echo.Context) error {
	cfg := h.manager.GetConfig(c.Param("id"))
	if cfg == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
	}
	return c.JSON(http.StatusOK, cfg)
}

// UpdateProvider handles PUT /providers/:id.
func (h *Handler) UpdateProvider(c echo.Context) error {
	id := c.Param("id")
	cfg := h.manager.GetConfig(id)
	if cfg == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
	}

	var req struct {
		BaseURL string `json:"base_url"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if req.BaseURL != "" {
		cfg.BaseURL = req.BaseURL
	}
	return c.JSON(http.StatusOK, cfg)
}

// EnableProvider handles POST /providers/:id/enable.
func (h *Handler) EnableProvider(c echo.Context) error {
	if err := h.manager.Enable(c.Param("id")); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "enabled"})
}

// DisableProvider handles POST /providers/:id/disable.
func (h *Handler) DisableProvider(c echo.Context) error {
	if err := h.manager.Disable(c.Param("id")); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "disabled"})
}

// SetProviderKey handles POST /providers/:id/keys.
func (h *Handler) SetProviderKey(c echo.Context) error {
	id := c.Param("id")
	var req struct {
		Key string `json:"key"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if req.Key == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "key is required"})
	}
	if err := h.manager.SetAPIKey(id, req.Key); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	// Return updated config so frontend gets key_hash and models
	cfg := h.manager.GetConfig(id)
	if cfg != nil {
		return c.JSON(http.StatusOK, cfg)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "key_set"})
}

// RemoveProviderKey handles DELETE /providers/:id/keys.
func (h *Handler) RemoveProviderKey(c echo.Context) error {
	if err := h.manager.RemoveAPIKey(c.Param("id")); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "key_removed"})
}

// TestProvider handles POST /providers/:id/test.
func (h *Handler) TestProvider(c echo.Context) error {
	result := h.manager.TestProvider(c.Param("id"))
	return c.JSON(http.StatusOK, result)
}
