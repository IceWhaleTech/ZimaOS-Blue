package harness

import (
	"database/sql"
	"net/http"
	"strings"

	agentpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/labstack/echo/v4"
)

type AgentCompatHandler struct {
	manager              *Controller
	store                *agentpkg.Store
	runner               *agentpkg.Runner
	defaultWorkspaceRoot string
}

func NewAgentCompatHandler(manager *Controller, store *agentpkg.Store, runner *agentpkg.Runner, defaultWorkspaceRoot string) *AgentCompatHandler {
	return &AgentCompatHandler{
		manager:              manager,
		store:                store,
		runner:               runner,
		defaultWorkspaceRoot: strings.TrimSpace(defaultWorkspaceRoot),
	}
}

func (h *AgentCompatHandler) RegisterRoutes(g *echo.Group) {
	if h == nil || h.manager == nil || h.store == nil || h.runner == nil || g == nil {
		return
	}
	g.POST("/tasks", h.CreateTask)
	g.GET("/tasks", h.ListTasks)
	g.GET("/tasks/:id", h.GetTask)
	g.POST("/tasks/:id/cancel", h.CancelTask)
	g.POST("/tasks/:id/message", h.SendMessage)
	g.POST("/tasks/:id/answer", h.SubmitAnswer)
	g.DELETE("/tasks/:id", h.DeleteTask)
}

func (h *AgentCompatHandler) CreateTask(c echo.Context) error {
	var req agentpkg.CreateTaskRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Goal == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "goal is required"})
	}
	run, err := h.manager.Submit(c.Request().Context(), RunSpec{
		Kind:           RunKindAgentTask,
		Goal:           req.Goal,
		UserID:         harnessUserID(c),
		ConversationID: req.ConversationID,
		SessionID:      req.ConversationID,
		WorkspaceRoot:  h.defaultWorkspaceRoot,
		Metadata:       map[string]interface{}{"context": req.Context},
	})
	if err != nil {
		return c.JSON(http.StatusTooManyRequests, map[string]string{"error": err.Error()})
	}
	task, err := h.store.Get(c.Request().Context(), run.ID, harnessUserID(c))
	if err != nil {
		return c.JSON(http.StatusCreated, run)
	}
	return c.JSON(http.StatusCreated, task)
}

func (h *AgentCompatHandler) ListTasks(c echo.Context) error {
	runs, err := h.manager.List(c.Request().Context(), RunFilter{
		UserID: harnessUserID(c),
		Kind:   RunKindAgentTask,
		Limit:  50,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	tasks := make([]*agentpkg.Task, 0, len(runs))
	for _, run := range runs {
		task, err := h.store.Get(c.Request().Context(), run.ID, harnessUserID(c))
		if err == nil {
			tasks = append(tasks, task)
			continue
		}
		tasks = append(tasks, runToTask(run))
	}
	return c.JSON(http.StatusOK, tasks)
}

func (h *AgentCompatHandler) GetTask(c echo.Context) error {
	run, err := h.manager.Get(c.Request().Context(), c.Param("id"))
	if err != nil || run.Kind != RunKindAgentTask {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	userID := harnessUserID(c)
	if userID != "" && run.UserID != "" && run.UserID != userID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	task, err := h.store.Get(c.Request().Context(), run.ID, userID)
	if err == nil {
		return c.JSON(http.StatusOK, task)
	}
	return c.JSON(http.StatusOK, runToTask(*run))
}

func (h *AgentCompatHandler) CancelTask(c echo.Context) error {
	id := c.Param("id")
	run, err := h.manager.Get(c.Request().Context(), id)
	if err != nil || run.Kind != RunKindAgentTask {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found or not running"})
	}
	userID := harnessUserID(c)
	if userID != "" && run.UserID != "" && run.UserID != userID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found or not running"})
	}
	if err := h.manager.Cancel(c.Request().Context(), id, "cancelled by user"); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found or not running"})
	}
	_ = h.store.SetStatus(c.Request().Context(), id, agentpkg.TaskStatusCancelled, "cancelled by user", userID)
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *AgentCompatHandler) DeleteTask(c echo.Context) error {
	id := c.Param("id")
	userID := harnessUserID(c)
	if err := h.store.Delete(c.Request().Context(), id, userID); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	_ = h.manager.Delete(c.Request().Context(), id)
	return c.NoContent(http.StatusNoContent)
}

func (h *AgentCompatHandler) SendMessage(c echo.Context) error {
	var req struct {
		Message string `json:"message"`
	}
	if err := c.Bind(&req); err != nil || req.Message == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "message is required"})
	}
	if !h.runner.EnqueueMessage(c.Param("id"), req.Message) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found or not running"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "queued"})
}

func (h *AgentCompatHandler) SubmitAnswer(c echo.Context) error {
	var req struct {
		Answers []agentpkg.QuestionAnswer `json:"answers"`
	}
	if err := c.Bind(&req); err != nil || len(req.Answers) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "answers array is required"})
	}
	if !h.runner.SubmitAnswers(c.Param("id"), req.Answers) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no pending question for this task"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "answered"})
}

func runToTask(run Run) *agentpkg.Task {
	return &agentpkg.Task{
		ID:             run.ID,
		UserID:         run.UserID,
		ConversationID: run.ConversationID,
		Goal:           run.Goal,
		Status:         taskStatusFromRun(run.Status),
		RuntimeState:   run.RuntimeState,
		CurrentStep:    run.CurrentStep,
		Progress:       run.Progress,
		Result:         run.Result,
		Error:          run.Error,
		CreatedAt:      run.CreatedAt,
		UpdatedAt:      run.UpdatedAt,
	}
}

func taskStatusFromRun(status RunStatus) agentpkg.TaskStatus {
	switch status {
	case RunStatusPlanning:
		return agentpkg.TaskStatusPlanning
	case RunStatusWaitingInput:
		return agentpkg.TaskStatusWaitingInput
	case RunStatusExecuting, RunStatusVerifying:
		return agentpkg.TaskStatusExecuting
	case RunStatusCompleted:
		return agentpkg.TaskStatusCompleted
	case RunStatusFailed:
		return agentpkg.TaskStatusFailed
	case RunStatusCancelled:
		return agentpkg.TaskStatusCancelled
	case RunStatusAborted:
		return agentpkg.TaskStatusAborted
	default:
		return agentpkg.TaskStatusPending
	}
}
