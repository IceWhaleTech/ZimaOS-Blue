package push

import (
	"context"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// ToolsAdapter adapts push.Service to tools.PushServiceInterface.
type ToolsAdapter struct {
	resolve func() *Service
}

// NewToolsAdapter creates a new adapter with lazy resolution.
func NewToolsAdapter(resolve func() *Service) *ToolsAdapter {
	return &ToolsAdapter{resolve: resolve}
}

func (a *ToolsAdapter) svc() (*Service, error) {
	s := a.resolve()
	if s == nil {
		return nil, fmt.Errorf("push service not available")
	}
	return s, nil
}

func toPushResult(r *PushNotification) tools.PushResult {
	return tools.PushResult{
		ID:        r.ID,
		Message:   r.Message,
		FireAt:    r.FireAt,
		Recurring: r.Recurring,
		UntilAt:   r.UntilAt,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
	}
}

func (a *ToolsAdapter) Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string, untilAt *time.Time) (tools.PushResult, error) {
	s, err := a.svc()
	if err != nil {
		return tools.PushResult{}, err
	}
	r, err := s.Add(ctx, ownerID, message, fireAt, recurring, sessionID, untilAt)
	if err != nil {
		return tools.PushResult{}, err
	}
	return toPushResult(r), nil
}

func (a *ToolsAdapter) List(ctx context.Context, ownerID string) ([]tools.PushResult, error) {
	s, err := a.svc()
	if err != nil {
		return nil, err
	}
	notifications, err := s.List(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	result := make([]tools.PushResult, len(notifications))
	for i, r := range notifications {
		result[i] = toPushResult(r)
	}
	return result, nil
}

func (a *ToolsAdapter) Delete(ctx context.Context, ownerID, id string) error {
	s, err := a.svc()
	if err != nil {
		return err
	}
	return s.Delete(ctx, ownerID, id)
}

func (a *ToolsAdapter) Clear(ctx context.Context, ownerID string) (int64, error) {
	s, err := a.svc()
	if err != nil {
		return 0, err
	}
	return s.Clear(ctx, ownerID)
}

var _ tools.PushServiceInterface = (*ToolsAdapter)(nil)
