package push

import (
	"context"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
)

// SkillAdapter adapts push.Service to builtin.PushServiceInterface.
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

func toPushInfo(r *PushNotification) builtin.PushInfo {
	return builtin.PushInfo{
		ID:        r.ID,
		Message:   r.Message,
		FireAt:    r.FireAt,
		Recurring: r.Recurring,
		UntilAt:   r.UntilAt,
		SessionID: r.SessionID,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
	}
}

func (a *SkillAdapter) Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string, untilAt *time.Time) (builtin.PushInfo, error) {
	s := a.svc()
	if s == nil {
		return builtin.PushInfo{}, errServiceUnavailable
	}
	r, err := s.Add(ctx, ownerID, message, fireAt, recurring, sessionID, untilAt)
	if err != nil {
		return builtin.PushInfo{}, err
	}
	return toPushInfo(r), nil
}

func (a *SkillAdapter) List(ctx context.Context, ownerID string) ([]builtin.PushInfo, error) {
	s := a.svc()
	if s == nil {
		return nil, errServiceUnavailable
	}
	notifications, err := s.List(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	result := make([]builtin.PushInfo, len(notifications))
	for i, r := range notifications {
		result[i] = toPushInfo(r)
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

var errServiceUnavailable = fmt.Errorf("push service not available")
