package harness

import (
	"net/http"
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
	g.GET("/tasks/:id", h.GetTask)
	g.POST("/tasks/:id/actions/:action", h.PerformTaskAction)
	g.POST("/tasks/:id/cancel", h.CancelTask)
	g.POST("/tasks/:id/resume", h.ResumeTask)
}

type userTaskResumeRequest struct {
	Decision string                 `json:"decision,omitempty"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
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
	taskID := strings.TrimSpace(c.Param("id"))
	if taskID == "" {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	return h.performNamedTaskAction(c, taskID, "cancel")
}

func (h *UserTaskProjectionHandler) ResumeTask(c echo.Context) error {
	taskID := strings.TrimSpace(c.Param("id"))
	if taskID == "" {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	return h.performNamedTaskAction(c, taskID, "resume")
}

func (h *UserTaskProjectionHandler) PerformTaskAction(c echo.Context) error {
	taskID := strings.TrimSpace(c.Param("id"))
	if taskID == "" {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	action := strings.ToLower(strings.TrimSpace(c.Param("action")))
	if action == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "action is required"})
	}
	return h.performNamedTaskAction(c, taskID, action)
}

func (h *UserTaskProjectionHandler) performNamedTaskAction(
	c echo.Context,
	taskID string,
	action string,
) error {
	userID := harnessUserID(c)
	if run, err := h.manager.Get(c.Request().Context(), taskID); err == nil && isVisibleUserTaskRun(run) {
		if userID != "" && run.UserID != "" && run.UserID != userID {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
		}
		switch action {
		case "cancel":
			return h.performTaskRunAction(c, run, "cancel", map[string]interface{}{"reason": "cancelled by user"})
		case "resume":
			if !canResumeRun(run) {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "task is not resumable"})
			}
			input, bindErr := h.bindResumeTaskInput(c)
			if bindErr != nil {
				return bindErr
			}
			return h.performTaskRunAction(c, run, "resume", input)
		default:
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "unsupported task action"})
		}
	}

	group, err := h.manager.GetGroup(c.Request().Context(), taskID)
	if err != nil || !isVisibleUserTaskGroup(group) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	if userID != "" && group.OwnerUserID != "" && group.OwnerUserID != userID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	switch action {
	case "cancel":
		if _, err := h.manager.PerformGroupAction(c.Request().Context(), group.ID, "cancel", map[string]interface{}{"reason": "cancelled by user"}); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		projection, projectErr := h.service.Get(c.Request().Context(), group.ID, projectionScopeForConversation(strings.TrimSpace(c.QueryParam("conversation_id")), userTaskGroupConversationID(group)))
		if projectErr != nil || projection == nil {
			return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
		}
		return c.JSON(http.StatusOK, projection)
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unsupported task action"})
	}
}

func (h *UserTaskProjectionHandler) performTaskRunAction(
	c echo.Context,
	run *Run,
	action string,
	input map[string]interface{},
) error {
	updated, err := h.manager.PerformAction(c.Request().Context(), run.ID, action, input)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	conversationID := run.ConversationID
	if updated != nil && strings.TrimSpace(updated.ConversationID) != "" {
		conversationID = updated.ConversationID
	}
	projection, projectErr := h.service.Get(c.Request().Context(), run.ID, projectionScopeForConversation(strings.TrimSpace(c.QueryParam("conversation_id")), conversationID))
	if projectErr != nil || projection == nil {
		if updated == nil {
			return c.JSON(http.StatusOK, map[string]string{"status": strings.ToLower(strings.TrimSpace(action))})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"id":     updated.ID,
			"status": updated.Status,
		})
	}
	return c.JSON(http.StatusOK, projection)
}

func (h *UserTaskProjectionHandler) bindResumeTaskInput(c echo.Context) (map[string]interface{}, error) {
	var req userTaskResumeRequest
	if c.Request().Body != nil && c.Request().ContentLength != 0 {
		if err := c.Bind(&req); err != nil {
			return nil, c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		}
	}
	return map[string]interface{}{
		"decision": strings.TrimSpace(req.Decision),
		"payload":  cloneMetadataMap(req.Payload),
	}, nil
}

func projectionScopeForConversation(requestedConversationID string, taskConversationID string) string {
	if strings.TrimSpace(requestedConversationID) != "" && strings.TrimSpace(requestedConversationID) == strings.TrimSpace(taskConversationID) {
		return "current"
	}
	return "background"
}
