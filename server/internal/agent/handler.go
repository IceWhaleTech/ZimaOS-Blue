package agent

import (
	"database/sql"
	"net/http"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

// Handler provides REST API for agent tasks.
type Handler struct {
	store  *Store
	runner *Runner
}

// NewHandler creates a new agent handler.
func NewHandler(store *Store, runner *Runner) *Handler {
	return &Handler{store: store, runner: runner}
}

// RegisterRoutes registers agent API routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.POST("/tasks", h.CreateTask)
	g.GET("/tasks", h.ListTasks)
	g.GET("/tasks/:id", h.GetTask)
	g.POST("/tasks/:id/cancel", h.CancelTask)
	g.POST("/tasks/:id/message", h.SendMessage)
	g.POST("/tasks/:id/answer", h.SubmitAnswer)
	g.DELETE("/tasks/:id", h.DeleteTask)
}

func getUserID(c echo.Context) string {
	if claims := auth.GetUserFromContext(c); claims != nil {
		return claims.UserID
	}
	return ""
}

func (h *Handler) getScopedTask(c echo.Context, id string) (*Task, error) {
	userID := getUserID(c)
	if userID != "" {
		return h.store.Get(c.Request().Context(), id, userID)
	}
	return h.store.Get(c.Request().Context(), id)
}

// CreateTask handles POST /api/v1/agent/tasks
func (h *Handler) CreateTask(c echo.Context) error {
	var req CreateTaskRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Goal == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "goal is required"})
	}

	userID := getUserID(c)
	task, err := h.runner.Submit(c.Request().Context(), userID, req.Goal, req.ConversationID, req.Context)
	if err != nil {
		return c.JSON(http.StatusTooManyRequests, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, task)
}

// ListTasks handles GET /api/v1/agent/tasks
func (h *Handler) ListTasks(c echo.Context) error {
	userID := getUserID(c)
	tasks, err := h.store.ListByUser(c.Request().Context(), userID, 50)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if tasks == nil {
		tasks = []*Task{}
	}
	return c.JSON(http.StatusOK, tasks)
}

// GetTask handles GET /api/v1/agent/tasks/:id
func (h *Handler) GetTask(c echo.Context) error {
	task, err := h.getScopedTask(c, c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	return c.JSON(http.StatusOK, task)
}

// CancelTask handles POST /api/v1/agent/tasks/:id/cancel
func (h *Handler) CancelTask(c echo.Context) error {
	id := c.Param("id")
	if _, err := h.getScopedTask(c, id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found or not running"})
	}
	if h.runner.Cancel(id) {
		_ = h.store.SetStatus(c.Request().Context(), id, TaskStatusCancelled, "cancelled by user", getUserID(c))
		return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
	}
	return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found or not running"})
}

// DeleteTask handles DELETE /api/v1/agent/tasks/:id
func (h *Handler) DeleteTask(c echo.Context) error {
	if err := h.store.Delete(c.Request().Context(), c.Param("id"), getUserID(c)); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.NoContent(http.StatusNoContent)
}

// SendMessage handles POST /api/v1/agent/tasks/:id/message
// Enqueues a user message for injection into a running task.
func (h *Handler) SendMessage(c echo.Context) error {
	var req struct {
		Message string `json:"message"`
	}
	if err := c.Bind(&req); err != nil || req.Message == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "message is required"})
	}
	id := c.Param("id")
	if _, err := h.getScopedTask(c, id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found or not running"})
	}
	if h.runner.EnqueueMessage(id, req.Message) {
		return c.JSON(http.StatusOK, map[string]string{"status": "queued"})
	}
	return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found or not running"})
}

// SubmitAnswer handles POST /api/v1/agent/tasks/:id/answer
// Delivers user answers to a pending ask_user call.
func (h *Handler) SubmitAnswer(c echo.Context) error {
	var req struct {
		Answers []QuestionAnswer `json:"answers"`
	}
	if err := c.Bind(&req); err != nil || len(req.Answers) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "answers array is required"})
	}
	id := c.Param("id")
	if _, err := h.getScopedTask(c, id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no pending question for this task"})
	}
	if h.runner.SubmitAnswers(id, req.Answers) {
		return c.JSON(http.StatusOK, map[string]string{"status": "answered"})
	}
	return c.JSON(http.StatusNotFound, map[string]string{"error": "no pending question for this task"})
}
