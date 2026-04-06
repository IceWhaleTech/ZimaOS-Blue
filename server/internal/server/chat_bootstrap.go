package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type chatBootstrapTaskProjectionService interface {
	List(ctx context.Context, filter harness.UserTaskProjectionFilter) ([]harness.UserTaskProjection, error)
}

type ConversationBootstrapToolApprovalSource interface {
	GetPending(userID string) map[string]any
	GetPendingBySession(sessionID string) map[string]any
}

type ConversationBootstrapQuestionSource interface {
	GetPending(userID string) *tools.QuestionRequest
	GetPendingBySession(sessionID string) *tools.QuestionRequest
}

type ConversationBootstrapExecApprovalSource interface {
	GetPending(userID string) *tools.ApprovalRequest
	GetPendingBySession(sessionID string) *tools.ApprovalRequest
}

type conversationBootstrapResponse struct {
	CommandState        conversationCommandStateResponse `json:"command_state"`
	ActiveStream        ConversationActiveStreamResponse `json:"active_stream"`
	CurrentTasks        []harness.UserTaskProjection     `json:"current_tasks"`
	BackgroundTasks     []harness.UserTaskProjection     `json:"background_tasks"`
	PendingApproval     map[string]any                   `json:"pending_approval"`
	PendingQuestion     *tools.QuestionRequest           `json:"pending_question"`
	PendingExecApproval *tools.ApprovalRequest           `json:"pending_exec_approval"`
}

func (h *ChatHandler) SetTaskProjectionService(service chatBootstrapTaskProjectionService) {
	if h == nil {
		return
	}
	h.taskProjectionService = service
}

func (h *ChatHandler) SetConversationBootstrapToolApprovalSource(source ConversationBootstrapToolApprovalSource) {
	if h == nil {
		return
	}
	h.toolApprovalPendingSource = source
}

func (h *ChatHandler) SetConversationBootstrapQuestionSource(source ConversationBootstrapQuestionSource) {
	if h == nil {
		return
	}
	h.questionPendingSource = source
}

func (h *ChatHandler) SetConversationBootstrapExecApprovalSource(source ConversationBootstrapExecApprovalSource) {
	if h == nil {
		return
	}
	h.execApprovalPendingSource = source
}

func (h *ChatHandler) conversationBootstrapTasks(
	ctx context.Context,
	userID string,
	convID string,
) ([]harness.UserTaskProjection, []harness.UserTaskProjection) {
	if h == nil || h.taskProjectionService == nil {
		return []harness.UserTaskProjection{}, []harness.UserTaskProjection{}
	}

	currentTasks, err := h.taskProjectionService.List(ctx, harness.UserTaskProjectionFilter{
		UserID:         strings.TrimSpace(userID),
		ConversationID: convID,
		Scope:          "current",
		Limit:          10,
	})
	if err != nil {
		logger.Warn().
			Err(err).
			Str("conversation_id", convID).
			Msg("[chat] failed to load current task projections for bootstrap")
		currentTasks = []harness.UserTaskProjection{}
	}

	backgroundTasks, err := h.taskProjectionService.List(ctx, harness.UserTaskProjectionFilter{
		UserID:         strings.TrimSpace(userID),
		ConversationID: convID,
		Scope:          "background",
		Limit:          5,
	})
	if err != nil {
		logger.Warn().
			Err(err).
			Str("conversation_id", convID).
			Msg("[chat] failed to load background task projections for bootstrap")
		backgroundTasks = []harness.UserTaskProjection{}
	}

	return currentTasks, backgroundTasks
}

func (h *ChatHandler) conversationBootstrapPendingApproval(convID, userID string) map[string]any {
	if h == nil || h.toolApprovalPendingSource == nil {
		return nil
	}
	if pending := h.toolApprovalPendingSource.GetPendingBySession(strings.TrimSpace(convID)); pending != nil {
		return pending
	}
	return h.toolApprovalPendingSource.GetPending(strings.TrimSpace(userID))
}

func (h *ChatHandler) conversationBootstrapPendingQuestion(convID, userID string) *tools.QuestionRequest {
	if h == nil || h.questionPendingSource == nil {
		return nil
	}
	if pending := h.questionPendingSource.GetPendingBySession(strings.TrimSpace(convID)); pending != nil {
		return pending
	}
	return h.questionPendingSource.GetPending(strings.TrimSpace(userID))
}

func (h *ChatHandler) conversationBootstrapPendingExecApproval(convID, userID string) *tools.ApprovalRequest {
	if h == nil || h.execApprovalPendingSource == nil {
		return nil
	}
	if pending := h.execApprovalPendingSource.GetPendingBySession(strings.TrimSpace(convID)); pending != nil {
		return pending
	}
	return h.execApprovalPendingSource.GetPending(strings.TrimSpace(userID))
}

func (h *ChatHandler) conversationBootstrap(
	ctx context.Context,
	convID string,
	userID string,
) (conversationBootstrapResponse, error) {
	state, err := h.getConversationCommandState(ctx, convID)
	if err != nil {
		return conversationBootstrapResponse{}, err
	}

	streamID := strings.TrimSpace(h.activeStreamIDForConversation(convID))
	currentTasks, backgroundTasks := h.conversationBootstrapTasks(ctx, userID, convID)
	pendingApproval := h.conversationBootstrapPendingApproval(convID, userID)
	pendingQuestion := h.conversationBootstrapPendingQuestion(convID, userID)
	pendingExecApproval := h.conversationBootstrapPendingExecApproval(convID, userID)

	return conversationBootstrapResponse{
		CommandState: commandStateToResponse(state),
		ActiveStream: ConversationActiveStreamResponse{
			ConversationID: convID,
			Active:         streamID != "",
			StreamID:       streamID,
		},
		CurrentTasks:        currentTasks,
		BackgroundTasks:     backgroundTasks,
		PendingApproval:     pendingApproval,
		PendingQuestion:     pendingQuestion,
		PendingExecApproval: pendingExecApproval,
	}, nil
}

func (h *ChatHandler) GetConversationBootstrap(c echo.Context) error {
	convID := c.Param("id")
	if _, err := h.checkConversationOwnership(c, convID); err != nil {
		return err
	}

	payload, err := h.conversationBootstrap(c.Request().Context(), convID, getUserIDFromContext(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load conversation bootstrap")
	}
	return c.JSON(http.StatusOK, payload)
}

func defaultConversationCommandState(convID string) memory.ConversationCommandState {
	return normalizeConversationCommandToolState(memory.ConversationCommandState{
		ConversationID:      convID,
		WebSearchEnabled:    true,
		DeepResearchEnabled: true,
	})
}
