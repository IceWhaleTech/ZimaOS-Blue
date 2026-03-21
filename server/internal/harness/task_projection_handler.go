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
	run, err := h.manager.Get(c.Request().Context(), c.Param("id"))
	if err != nil || !isVisibleUserTaskRun(run) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	userID := harnessUserID(c)
	if userID != "" && run.UserID != "" && run.UserID != userID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	scope := "background"
	if conversationID := strings.TrimSpace(c.QueryParam("conversation_id")); conversationID != "" && conversationID == strings.TrimSpace(run.ConversationID) {
		scope = "current"
	}
	projection, err := h.service.projectionForRun(c.Request().Context(), run, scope)
	if err != nil || projection == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	return c.JSON(http.StatusOK, projection)
}

func (h *UserTaskProjectionHandler) CancelTask(c echo.Context) error {
	run, err := h.manager.Get(c.Request().Context(), c.Param("id"))
	if err != nil || !isVisibleUserTaskRun(run) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	userID := harnessUserID(c)
	if userID != "" && run.UserID != "" && run.UserID != userID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	if err := h.manager.Cancel(c.Request().Context(), run.ID, "cancelled by user"); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	projection, err := h.service.Get(c.Request().Context(), run.ID, "current")
	if err != nil || projection == nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
	}
	return c.JSON(http.StatusOK, projection)
}

func parsePositiveInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}
