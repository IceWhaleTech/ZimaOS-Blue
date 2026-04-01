package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func (a sessionListAdapter) CreateSession(ctx context.Context, title, userID string, pinned bool) (*tools.SessionSummary, error) {
	if a.store == nil {
		return nil, nil
	}
	userID = sessionScopedUserID(ctx, userID)
	var (
		conv *memory.Conversation
		err  error
	)
	if userID != "" {
		conv, err = a.store.CreateConversation(ctx, title, userID)
	} else {
		conv, err = a.store.CreateConversation(ctx, title)
	}
	if err != nil {
		return nil, err
	}
	if pinned {
		if err := a.store.PinConversation(ctx, conv.ID, userID); err != nil {
			return nil, err
		}
		conv, err = a.store.GetConversation(ctx, conv.ID, userID)
		if err != nil {
			return nil, err
		}
	}
	return buildSessionSummaryPtr(conv), nil
}

func (a sessionListAdapter) AppendSessionMessage(ctx context.Context, sessionID string, msg tools.SessionMessage) (*tools.SessionMessage, error) {
	if a.store == nil {
		return nil, nil
	}
	created, err := a.store.AddMessage(ctx, sessionID, memory.Message{
		Role:       msg.Role,
		Content:    msg.Content,
		ToolCallID: msg.ToolCallID,
		ToolName:   msg.ToolName,
		Provider:   msg.Provider,
		Model:      msg.Model,
	}, sessionScopedUserID(ctx, ""))
	if err != nil {
		return nil, err
	}
	message := buildSessionMessage(*created)
	return &message, nil
}
