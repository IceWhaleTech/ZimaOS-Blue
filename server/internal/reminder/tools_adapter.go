package reminder

import (
	"context"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// ToolsAdapter adapts reminder.Service to tools.ReminderServiceInterface.
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
		return nil, fmt.Errorf("reminder service not available")
	}
	return s, nil
}

func reminderToResult(r *Reminder) tools.ReminderResult {
	return tools.ReminderResult{
		ID:        r.ID,
		Message:   r.Message,
		FireAt:    r.FireAt,
		Recurring: r.Recurring,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
	}
}

func (a *ToolsAdapter) Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string) (tools.ReminderResult, error) {
	s, err := a.svc()
	if err != nil {
		return tools.ReminderResult{}, err
	}
	r, err := s.Add(ctx, ownerID, message, fireAt, recurring, sessionID)
	if err != nil {
		return tools.ReminderResult{}, err
	}
	return reminderToResult(r), nil
}

func (a *ToolsAdapter) List(ctx context.Context, ownerID string) ([]tools.ReminderResult, error) {
	s, err := a.svc()
	if err != nil {
		return nil, err
	}
	reminders, err := s.List(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	result := make([]tools.ReminderResult, len(reminders))
	for i, r := range reminders {
		result[i] = reminderToResult(r)
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

var _ tools.ReminderServiceInterface = (*ToolsAdapter)(nil)
