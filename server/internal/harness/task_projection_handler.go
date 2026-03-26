package harness

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type UserTaskProjectionHandler struct {
	manager *Controller
	service *UserTaskProjectionService
}

func NewUserTaskProjectionHandler(manager *Controller, detailProvider RunDetailProvider) *UserTaskProjectionHandler {
	if manager == nil {
		return nil
	}
	return &UserTaskProjectionHandler{
		manager: manager,
		service: NewUserTaskProjectionService(manager, detailProvider),
	}
}

func (h *UserTaskProjectionHandler) RegisterRoutes(g *echo.Group) {
	if h == nil || h.manager == nil || h.service == nil || g == nil {
		return
	}
	g.GET("/tasks", h.ListTasks)
	g.GET("/tasks/:id", h.GetTask)
	g.POST("/tasks/:id/cancel", h.CancelTask)
}

func (h *UserTaskProjectionHandler) ListTasks(c echo.Context) error {
	scope := normalizedProjectionScope(c.QueryParam("scope"))
	limit := normalizedProjectionLimit(scope, parsePositiveInt(c.QueryParam("limit")))
	projections, err := h.service.List(c.Request().Context(), UserTaskProjectionFilter{
		UserID:         harnessUserID(c),
		ConversationID: strings.TrimSpace(c.QueryParam("conversation_id")),
		Scope:          scope,
		Limit:          limit,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, projections)
}

func (h *UserTaskProjectionHandler) GetTask(c echo.Context) error {
	taskID := strings.TrimSpace(c.Param("id"))
	if taskID == "" {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	userID := harnessUserID(c)
	conversationID := strings.TrimSpace(c.QueryParam("conversation_id"))

	if run, err := h.manager.Get(c.Request().Context(), taskID); err == nil {
		if !isVisibleUserTaskRun(run) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
		}
		if userID != "" && run.UserID != "" && run.UserID != userID {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
		}
		projection, projectErr := h.service.Get(c.Request().Context(), run.ID, projectionScopeForConversation(conversationID, run.ConversationID))
		if projectErr != nil || projection == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
		}
		return c.JSON(http.StatusOK, projection)
	}

	group, err := h.manager.GetGroup(c.Request().Context(), taskID)
	if err != nil || !isVisibleUserTaskGroup(group) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	if userID != "" && group.OwnerUserID != "" && group.OwnerUserID != userID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	projection, projectErr := h.service.Get(c.Request().Context(), group.ID, projectionScopeForConversation(conversationID, userTaskGroupConversationID(group)))
	if projectErr != nil || projection == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	return c.JSON(http.StatusOK, projection)
}

func (h *UserTaskProjectionHandler) CancelTask(c echo.Context) error {
	userID := harnessUserID(c)
	taskID := strings.TrimSpace(c.Param("id"))
	if taskID == "" {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}

	if run, err := h.manager.Get(c.Request().Context(), taskID); err == nil && isVisibleUserTaskRun(run) {
		if userID != "" && run.UserID != "" && run.UserID != userID {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
		}
		if err := h.manager.Cancel(c.Request().Context(), run.ID, "cancelled by user"); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		projection, projectErr := h.service.Get(c.Request().Context(), run.ID, projectionScopeForConversation(strings.TrimSpace(c.QueryParam("conversation_id")), run.ConversationID))
		if projectErr != nil || projection == nil {
			return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
		}
		return c.JSON(http.StatusOK, projection)
	}

	group, err := h.manager.GetGroup(c.Request().Context(), taskID)
	if err != nil || !isVisibleUserTaskGroup(group) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	if userID != "" && group.OwnerUserID != "" && group.OwnerUserID != userID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	if err := h.manager.CancelGroup(c.Request().Context(), group.ID, "cancelled by user"); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	projection, projectErr := h.service.Get(c.Request().Context(), group.ID, projectionScopeForConversation(strings.TrimSpace(c.QueryParam("conversation_id")), userTaskGroupConversationID(group)))
	if projectErr != nil || projection == nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
	}
	return c.JSON(http.StatusOK, projection)
}

func projectionScopeForConversation(requestedConversationID string, taskConversationID string) string {
	if strings.TrimSpace(requestedConversationID) != "" && strings.TrimSpace(requestedConversationID) == strings.TrimSpace(taskConversationID) {
		return "current"
	}
	return "background"
}

func parsePositiveInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}
