package mediagen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

// AddMessageFunc is a function that adds a message to a conversation and returns the message ID.
type AddMessageFunc func(ctx context.Context, conversationID, role, content string) (messageID string, err error)

// UpdateTitleFunc is a function that updates a conversation's title.
type UpdateTitleFunc func(ctx context.Context, conversationID, title string) error

// ResolveConversationScopeFunc validates a conversation and returns the owning user scope.
type ResolveConversationScopeFunc func(ctx context.Context, conversationID string) (userID string, err error)

// Handler provides HTTP endpoints for media generation.
type Handler struct {
	manager                  *Manager
	storage                  *MediaStorage
	addMessage               AddMessageFunc
	updateTitle              UpdateTitleFunc
	resolveConversationScope ResolveConversationScopeFunc
	locale                   string
}

// NewHandler creates a new media generation handler.
func NewHandler(manager *Manager, storage *MediaStorage, locale string) *Handler {
	return &Handler{manager: manager, storage: storage, locale: locale}
}

// SetAddMessage sets the function for persisting messages into conversations.
func (h *Handler) SetAddMessage(fn AddMessageFunc) {
	h.addMessage = fn
}

// SetUpdateTitle sets the function for updating conversation titles.
func (h *Handler) SetUpdateTitle(fn UpdateTitleFunc) {
	h.updateTitle = fn
}

// SetResolveConversationScope sets the function for validating a conversation and resolving its owner scope.
func (h *Handler) SetResolveConversationScope(fn ResolveConversationScopeFunc) {
	h.resolveConversationScope = fn
}

func mediaScopeArgs(c echo.Context) []string {
	if claims := auth.GetUserFromContext(c); claims != nil {
		if claims.Role == "admin" {
			return nil
		}
		return []string{strings.TrimSpace(claims.UserID)}
	}
	return []string{""}
}

func mediaScopedContext(c echo.Context, fallbackUserID string) context.Context {
	ctx := c.Request().Context()
	fallbackUserID = strings.TrimSpace(fallbackUserID)
	// If we have auth claims with a real user, use that as the fallback
	if fallbackUserID == "" {
		if claims := auth.GetUserFromContext(c); claims != nil && strings.TrimSpace(claims.UserID) != "" && claims.Role != "admin" {
			fallbackUserID = strings.TrimSpace(claims.UserID)
		}
	}
	if fallbackUserID == "" {
		return ctx
	}
	return tools.WithUserID(ctx, fallbackUserID)
}

func unavailableMediaModelID(err error) string {
	if err == nil || !errors.Is(err, ErrModelUnavailable) {
		return ""
	}
	prefix := ErrModelUnavailable.Error() + ":"
	msg := err.Error()
	if !strings.HasPrefix(msg, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(msg, prefix))
}

func availableMediaModelIDs(models []MediaModelInfo) []string {
	ids := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		if strings.TrimSpace(model.ID) == "" {
			continue
		}
		if _, ok := seen[model.ID]; ok {
			continue
		}
		seen[model.ID] = struct{}{}
		ids = append(ids, model.ID)
	}
	return ids
}

func buildUnavailableModelHint(modelID string, models []MediaModelInfo) string {
	available := availableMediaModelIDs(models)
	if len(available) == 0 {
		if modelID == "" {
			return "Choose one of the configured media models, or omit the model field to let mediagen choose a default."
		}
		return fmt.Sprintf("Model %q is not available from the configured media providers. Omit the model field to let mediagen choose a default.", modelID)
	}

	const maxHintModels = 6
	display := available
	if len(display) > maxHintModels {
		display = display[:maxHintModels]
	}
	hint := fmt.Sprintf(
		"Model %q is not available from the configured media providers. Available models include: %s. Omit the model field to let mediagen choose a default.",
		modelID,
		strings.Join(display, ", "),
	)
	if len(available) > maxHintModels {
		hint += " Query /api/v1/media/models for the full list."
	}
	return hint
}

func (h *Handler) writeMediaRequestError(c echo.Context, err error) error {
	if err == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "unknown media generation error"})
	}
	if errors.Is(err, ErrProviderNotFound) {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "no media provider is configured and enabled",
			"hint":  "Configure a provider in /api/v1/media/providers, add an API key, and enable it before generating media.",
		})
	}
	if errors.Is(err, ErrModelUnavailable) {
		modelID := unavailableMediaModelID(err)
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "requested media model is not available",
			"hint":  buildUnavailableModelHint(modelID, h.manager.Models()),
		})
	}
	if errors.Is(err, ErrUnsupportedType) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if errors.Is(err, ErrManagerClosed) {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

// RegisterRoutes registers all media generation routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.POST("/images/generations", h.GenerateImage)
	g.POST("/videos/generations", h.GenerateVideo)
	g.GET("/fallback/render/:token", h.RenderFallbackPage)
	g.GET("/fallback/models/:id/status", h.GetFallbackModelStatus)
	g.GET("/tasks/:id", h.GetTask)
	g.POST("/tasks/:id/cancel", h.CancelTask)
	g.POST("/tasks/:id/retry", h.RetryTask)
	g.GET("/tasks/:id/stream", h.StreamTask)
	g.GET("/models", h.ListModels)

	// IR-based direct generation pipeline
	g.POST("/classify", h.ClassifyIntent)
	g.POST("/generate", h.DirectGenerate)
	g.GET("/tasks/by-message/:message_id", h.GetTaskByMessage)

	// Stats
	g.GET("/stats", h.GetMediaStats)

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

func (h *Handler) GetFallbackModelStatus(c echo.Context) error {
	status, err := h.manager.GetFallbackModelStatus(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, status)
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
		return h.writeMediaRequestError(c, err)
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
		return h.writeMediaRequestError(c, err)
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
	task, err := h.manager.GetTask(c.Param("id"), mediaScopeArgs(c)...)
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
	if _, err := h.manager.GetTask(taskID, mediaScopeArgs(c)...); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Disable WriteTimeout for this long-lived SSE connection.
	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{})

	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			task, err := h.manager.GetTask(taskID, mediaScopeArgs(c)...)
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
			if task.FallbackInfo != nil {
				evt["fallback_info"] = task.FallbackInfo
			}

			switch task.Status {
			case TaskStatusSucceeded:
				if task.Response != nil {
					evt["response"] = task.Response
				}
				writeSSE(w, "complete", evt)
				writeSSE(w, "done", "[DONE]")
				return nil
			case TaskStatusCancelled:
				if task.Error != "" {
					evt["error"] = task.Error
				}
				writeSSE(w, "cancelled", evt)
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

	// Optional category filter
	if catFilter := c.QueryParam("category"); catFilter != "" {
		filtered := make([]MediaModelInfo, 0)
		for _, m := range models {
			if string(m.Category) == catFilter {
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
		return h.writeMediaRequestError(c, err)
	}

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Disable WriteTimeout for this long-lived SSE connection.
	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{})

	// Send initial event
	writeSSE(w, "started", map[string]interface{}{
		"task_id":       task.ID,
		"status":        string(task.Status),
		"type":          string(task.Type),
		"fallback_info": task.FallbackInfo,
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
			current, err := h.manager.GetTask(task.ID, mediaScopeArgs(c)...)
			if err != nil {
				writeSSE(w, "error", map[string]string{"error": err.Error()})
				return nil
			}

			evt := map[string]interface{}{
				"id":       current.ID,
				"status":   string(current.Status),
				"progress": current.Progress,
			}
			if current.FallbackInfo != nil {
				evt["fallback_info"] = current.FallbackInfo
			}

			switch current.Status {
			case TaskStatusSucceeded:
				if current.Response != nil {
					evt["response"] = current.Response
				}
				writeSSE(w, "complete", evt)
				writeSSE(w, "done", "[DONE]")
				return nil
			case TaskStatusCancelled:
				if current.Error != "" {
					evt["error"] = current.Error
				}
				writeSSE(w, "cancelled", evt)
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

// RenderFallbackPage serves the temporary local HTML page used by browser screenshots.
func (h *Handler) RenderFallbackPage(c echo.Context) error {
	if h.manager == nil {
		return echo.NewHTTPError(http.StatusNotFound, "fallback render not available")
	}
	doc, ok := h.manager.RenderFallbackPage(c.Param("token"))
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "fallback render not found")
	}
	return c.HTML(http.StatusOK, doc)
}

// GetMediaStats handles GET /stats — returns aggregated media generation statistics.
func (h *Handler) GetMediaStats(c echo.Context) error {
	if h.manager.taskStore == nil {
		return c.JSON(http.StatusOK, &MediaStats{
			CostByModel: map[string]float64{},
			TasksByType: map[string]int64{},
		})
	}
	stats, err := h.manager.taskStore.GetStats(mediaScopeArgs(c)...)
	if err != nil {
		return h.writeMediaRequestError(c, err)
	}
	return c.JSON(http.StatusOK, stats)
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
		BaseURL  *string `json:"base_url"`
		Priority *int    `json:"priority"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	updated, err := h.manager.UpdateConfig(id, req.BaseURL, req.Priority)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, updated)
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

// --- IR-based direct generation pipeline ---

// classifyRequest is the request body for POST /classify.
type classifyRequest struct {
	Message    string `json:"message"`
	HasImages  bool   `json:"has_images"`
	ImageCount int    `json:"image_count"`
	Locale     string `json:"locale"`
}

// classifyResponse is the response for POST /classify.
type classifyResponse struct {
	Intent *MediaIntent     `json:"intent"`
	Models []MediaModelInfo `json:"models,omitempty"`
}

// ClassifyIntent handles POST /classify — IR-based intent classification.
func (h *Handler) ClassifyIntent(c echo.Context) error {
	var req classifyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request: " + err.Error()})
	}

	locale := strings.TrimSpace(req.Locale)
	if locale == "" {
		locale = h.locale
	}

	intent := ClassifyMediaIntent(req.Message, req.HasImages, req.ImageCount, locale)
	if intent == nil {
		return c.JSON(http.StatusOK, classifyResponse{Intent: nil})
	}

	// Filter models by detected category
	allModels := h.manager.Models()
	var matched []MediaModelInfo
	for _, m := range allModels {
		if m.Category == intent.Category {
			matched = append(matched, m)
		}
	}

	return c.JSON(http.StatusOK, classifyResponse{
		Intent: intent,
		Models: matched,
	})
}

// directGenerateRequest is the request body for POST /generate.
type directGenerateRequest struct {
	Category        MediaCategory  `json:"category"`
	Prompt          string         `json:"prompt"`
	Model           string         `json:"model"`
	Params          map[string]any `json:"params"`
	ReferenceImages []string       `json:"reference_images,omitempty"`
	ConversationID  string         `json:"conversation_id,omitempty"`
	MessageID       string         `json:"message_id,omitempty"`
	Source          string         `json:"source,omitempty"` // "web" or "channel"
}

// DirectGenerate handles POST /generate — creates a persistent media task and returns immediately.
// The task runs asynchronously; clients poll GET /tasks/:id for status.
func (h *Handler) DirectGenerate(c echo.Context) error {
	var req directGenerateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request: " + err.Error()})
	}
	if req.Prompt == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "prompt is required"})
	}
	if req.Category == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "category is required"})
	}

	// Build MediaRequest from category + params
	mediaReq := &MediaRequest{
		Prompt: req.Prompt,
		Model:  req.Model,
		Extra:  req.Params,
	}

	// Set type based on category
	switch req.Category {
	case CategoryT2I, CategoryI2I:
		mediaReq.Type = MediaTypeImage
	case CategoryT2V, CategoryI2V, CategoryKF2V:
		mediaReq.Type = MediaTypeVideo
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unknown category: " + string(req.Category)})
	}

	// Extract common params
	if req.Params != nil {
		if v, ok := req.Params["size"].(string); ok {
			mediaReq.Size = v
		}
		if v, ok := req.Params["quality"].(string); ok {
			mediaReq.Quality = v
		}
		if v, ok := req.Params["style"].(string); ok {
			mediaReq.Style = v
		}
		if v, ok := req.Params["negative_prompt"].(string); ok {
			mediaReq.NegativePrompt = v
		}
		if v, ok := req.Params["n"].(float64); ok {
			mediaReq.N = int(v)
		}
		if v, ok := req.Params["duration"].(float64); ok {
			mediaReq.Duration = int(v)
		}
	}

	// Handle reference images for i2v/i2i/kf2v
	if len(req.ReferenceImages) > 0 {
		mediaReq.ReferenceURL = req.ReferenceImages[0]
		mediaReq.ReferenceURLs = req.ReferenceImages
	}

	source := req.Source
	if source == "" {
		source = "web"
	}
	if mediaReq.Extra == nil {
		mediaReq.Extra = map[string]any{}
	}
	if _, ok := mediaReq.Extra["source"]; !ok {
		mediaReq.Extra["source"] = source
	}

	resolvedConversationUserID := ""
	scopedCtx := c.Request().Context()
	if req.ConversationID != "" && h.resolveConversationScope != nil {
		var err error
		resolvedConversationUserID, err = h.resolveConversationScope(scopedCtx, req.ConversationID)
		if err != nil {
			return err
		}
	}
	// Always call mediaScopedContext so auth claims are used as fallback when no ConversationID.
	scopedCtx = mediaScopedContext(c, resolvedConversationUserID)

	// Set conversation title early — before task creation so it works even if generation fails.
	if req.ConversationID != "" && h.updateTitle != nil {
		locale := h.locale
		if al := c.Request().Header.Get("Accept-Language"); al != "" {
			locale = al
		}
		categoryLabel := categoryDisplayName(req.Category, locale)
		runes := []rune(req.Prompt)
		promptSnippet := req.Prompt
		if len(runes) > 20 {
			promptSnippet = string(runes[:20]) + "..."
		}
		title := categoryLabel + ": " + promptSnippet
		_ = h.updateTitle(scopedCtx, req.ConversationID, title)
	}

	// Create persistent task and start async generation
	task, err := h.manager.CreateTask(scopedCtx, mediaReq, req.MessageID, string(req.Category), source)
	if err != nil {
		return h.writeMediaRequestError(c, err)
	}

	// If conversation_id is provided and we have a message store, persist messages into the conversation.
	// This makes media tasks visible from any device — the assistant message contains the task_id marker.
	var userMsgID, assistantMsgID string
	if req.ConversationID != "" && h.addMessage != nil {
		ctx := scopedCtx

		// Build user message: prompt + reference images (saved to local storage)
		userContent := req.Prompt
		if len(req.ReferenceImages) > 0 && h.storage != nil {
			for _, dataURL := range req.ReferenceImages {
				ct, b64 := parseDataURL(dataURL)
				if b64 == "" {
					continue
				}
				localURL, err := h.storage.StoreBase64(b64, ct, MediaTypeImage)
				if err != nil {
					continue
				}
				userContent += "\n\n![image](" + localURL + ")"
			}
		}

		// Create user message with the prompt + images
		uid, err := h.addMessage(ctx, req.ConversationID, "user", userContent)
		if err == nil {
			userMsgID = uid
		}

		// Create assistant message with media_task_id marker
		marker := fmt.Sprintf(`[media_task:%s]`, task.ID)
		aid, err := h.addMessage(ctx, req.ConversationID, "assistant", marker)
		if err == nil {
			assistantMsgID = aid
			// Update task's message_id to point to the assistant message
			task.MessageID = assistantMsgID
			if h.manager.taskStore != nil {
				h.manager.taskStore.UpdateMessageID(task.ID, assistantMsgID)
			}
		}
	}

	// Return task ID immediately — client polls for status
	return c.JSON(http.StatusAccepted, map[string]interface{}{
		"task_id":              task.ID,
		"message_id":           task.MessageID,
		"user_message_id":      userMsgID,
		"assistant_message_id": assistantMsgID,
		"status":               string(task.Status),
		"category":             task.Category,
		"model":                task.Model,
	})
}

// categoryDisplayName returns a human-readable label for a media category.
func categoryDisplayName(cat MediaCategory, locale string) string {
	cn := strings.HasPrefix(locale, "zh")
	switch cat {
	case CategoryT2I:
		if cn {
			return "文生图"
		}
		return "Text to Image"
	case CategoryT2V:
		if cn {
			return "文生视频"
		}
		return "Text to Video"
	case CategoryI2V:
		if cn {
			return "图生视频"
		}
		return "Image to Video"
	case CategoryI2I:
		if cn {
			return "图片编辑"
		}
		return "Image Edit"
	case CategoryKF2V:
		if cn {
			return "关键帧生视频"
		}
		return "Keyframe to Video"
	default:
		if cn {
			return "媒体生成"
		}
		return "Media"
	}
}

// GetTaskByMessage handles GET /tasks/by-message/:message_id — returns tasks for a message.
func (h *Handler) GetTaskByMessage(c echo.Context) error {
	messageID := c.Param("message_id")
	if messageID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "message_id is required"})
	}
	tasks, err := h.manager.GetTasksByMessage(messageID, mediaScopeArgs(c)...)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"tasks": tasks,
	})
}

// RetryTask handles POST /tasks/:id/retry — re-submits a failed/cancelled task with the same parameters.
func (h *Handler) RetryTask(c echo.Context) error {
	taskID := c.Param("id")
	if taskID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "task id is required"})
	}

	oldTask, err := h.manager.GetTask(taskID, mediaScopeArgs(c)...)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	if oldTask.Status != TaskStatusFailed && oldTask.Status != TaskStatusCancelled {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "only failed or cancelled tasks can be retried"})
	}
	if oldTask.Request == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "task has no request data"})
	}

	// Ensure model is set on the request (CreateTask uses req.Model for provider lookup)
	if oldTask.Request.Model == "" {
		oldTask.Request.Model = oldTask.Model
	}

	newTask, err := h.manager.CreateTask(mediaScopedContext(c, oldTask.UserID), oldTask.Request, oldTask.MessageID, oldTask.Category, oldTask.Source)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"task_id":    newTask.ID,
		"message_id": newTask.MessageID,
		"status":     string(newTask.Status),
	})
}

// CancelTask cancels a pending or processing media generation task.
func (h *Handler) CancelTask(c echo.Context) error {
	taskID := c.Param("id")
	if taskID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "task id is required"})
	}

	if h.manager.CancelTask(taskID, mediaScopeArgs(c)...) {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"task_id": taskID,
			"status":  "cancelled",
		})
	}

	return c.JSON(http.StatusNotFound, map[string]interface{}{
		"success": false,
		"task_id": taskID,
		"error":   "task not found or already completed",
	})
}

// parseDataURL extracts content type and base64 data from a data URL.
// Input: "data:image/png;base64,iVBOR..." → ("image/png", "iVBOR...")
// Falls back to treating the whole string as raw base64 if not a data URL.
func parseDataURL(dataURL string) (contentType, b64 string) {
	if !strings.HasPrefix(dataURL, "data:") {
		return "image/png", dataURL
	}
	// data:image/png;base64,iVBOR...
	rest := dataURL[5:] // strip "data:"
	semicolonIdx := strings.Index(rest, ";")
	if semicolonIdx < 0 {
		return "image/png", dataURL
	}
	contentType = rest[:semicolonIdx]
	after := rest[semicolonIdx+1:]
	if strings.HasPrefix(after, "base64,") {
		b64 = after[7:]
	}
	return contentType, b64
}
