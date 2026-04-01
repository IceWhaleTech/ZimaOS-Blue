package bootstrap

import (
	"context"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type sessionListAdapter struct {
	store         *memory.Store
	agentSessions *agentsessions.Service
}

func sessionScopedUserID(ctx context.Context, requestedUserID string) string {
	if ctxUserID := strings.TrimSpace(tools.GetUserID(ctx)); ctxUserID != "" {
		return ctxUserID
	}
	return strings.TrimSpace(requestedUserID)
}

func buildSessionSummary(conv memory.Conversation) tools.SessionSummary {
	return tools.SessionSummary{
		ID:        conv.ID,
		Title:     conv.Title,
		UserID:    conv.UserID,
		Pinned:    conv.Pinned,
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
	}
}

func buildSessionSummaryPtr(conv *memory.Conversation) *tools.SessionSummary {
	if conv == nil {
		return nil
	}
	summary := buildSessionSummary(*conv)
	return &summary
}

func buildSessionMessage(msg memory.Message) tools.SessionMessage {
	return tools.SessionMessage{
		ID:         msg.ID,
		Role:       msg.Role,
		Content:    msg.Content,
		ToolCallID: msg.ToolCallID,
		ToolName:   msg.ToolName,
		Provider:   msg.Provider,
		Model:      msg.Model,
		CreatedAt:  msg.CreatedAt,
	}
}
