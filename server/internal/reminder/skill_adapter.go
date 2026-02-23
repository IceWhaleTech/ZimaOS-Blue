package reminder

import (
	"context"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
)

// SkillAdapter adapts reminder.Service to builtin.ReminderServiceInterface.
type SkillAdapter struct {
	resolve func() *Service
}

// NewSkillAdapter creates a new adapter with lazy resolution.
func NewSkillAdapter(resolve func() *Service) *SkillAdapter {
	return &SkillAdapter{resolve: resolve}
}

func (a *SkillAdapter) svc() *Service {
	return a.resolve()
}

func reminderToInfo(r *Reminder) builtin.ReminderInfo {
	return builtin.ReminderInfo{
		ID:        r.ID,
		Message:   r.Message,
		FireAt:    r.FireAt,
		Recurring: r.Recurring,
		SessionID: r.SessionID,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
	}
}

func (a *SkillAdapter) Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string) (builtin.ReminderInfo, error) {
	s := a.svc()
	if s == nil {
		return builtin.ReminderInfo{}, errServiceUnavailable
	}
	r, err := s.Add(ctx, ownerID, message, fireAt, recurring, sessionID)
	if err != nil {
		return builtin.ReminderInfo{}, err
	}
	return reminderToInfo(r), nil
}

func (a *SkillAdapter) List(ctx context.Context, ownerID string) ([]builtin.ReminderInfo, error) {
	s := a.svc()
	if s == nil {
		return nil, errServiceUnavailable
	}
	reminders, err := s.List(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	result := make([]builtin.ReminderInfo, len(reminders))
	for i, r := range reminders {
		result[i] = reminderToInfo(r)
	}
	return result, nil
}

func (a *SkillAdapter) Delete(ctx context.Context, ownerID, id string) error {
	s := a.svc()
	if s == nil {
		return errServiceUnavailable
	}
	return s.Delete(ctx, ownerID, id)
}

func (a *SkillAdapter) Clear(ctx context.Context, ownerID string) (int64, error) {
	s := a.svc()
	if s == nil {
		return 0, errServiceUnavailable
	}
	return s.Clear(ctx, ownerID)
}

var errServiceUnavailable = fmt.Errorf("reminder service not available")
