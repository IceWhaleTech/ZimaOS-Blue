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
