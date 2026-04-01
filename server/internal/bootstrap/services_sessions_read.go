package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func (a sessionListAdapter) ListSessions(ctx context.Context, limit, offset int, userID string) ([]tools.SessionSummary, error) {
	if a.store == nil {
		return nil, nil
	}
	userID = sessionScopedUserID(ctx, userID)
	var (
		convs []memory.Conversation
		err   error
	)
	if userID != "" {
		convs, err = a.store.ListConversations(ctx, limit, offset, userID)
	} else {
		convs, err = a.store.ListConversations(ctx, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	out := make([]tools.SessionSummary, 0, len(convs))
	for _, conv := range convs {
		out = append(out, buildSessionSummary(conv))
	}
	return out, nil
}

func (a sessionListAdapter) GetSession(ctx context.Context, sessionID string) (*tools.SessionSummary, error) {
	if a.store == nil {
		return nil, nil
	}
	conv, err := a.store.GetConversation(ctx, sessionID, sessionScopedUserID(ctx, ""))
	if err != nil {
		if err == memory.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return buildSessionSummaryPtr(conv), nil
}

func (a sessionListAdapter) GetSessionMessages(ctx context.Context, sessionID string, limit, offset int) ([]tools.SessionMessage, error) {
	if a.store == nil {
		return nil, nil
	}
	messages, err := a.store.GetMessages(ctx, sessionID, limit, offset, sessionScopedUserID(ctx, ""))
	if err != nil {
		return nil, err
	}
	out := make([]tools.SessionMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, buildSessionMessage(msg))
	}
	return out, nil
}
