package sockipc

import (
	"context"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
)

// PushIPCAdapter adapts push.Service to the sockipc.PushBackend interface.
type PushIPCAdapter struct {
	svc *push.Service
}

// NewPushIPCAdapter creates a new adapter.
func NewPushIPCAdapter(svc *push.Service) *PushIPCAdapter {
	return &PushIPCAdapter{svc: svc}
}

func (a *PushIPCAdapter) Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string) (PushResult, error) {
	r, err := a.svc.Add(ctx, ownerID, message, fireAt, recurring, sessionID)
	if err != nil {
		return PushResult{}, err
	}
	return toPushResult(r), nil
}

func (a *PushIPCAdapter) List(ctx context.Context, ownerID string) ([]PushResult, error) {
	list, err := a.svc.List(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	results := make([]PushResult, len(list))
	for i, r := range list {
		results[i] = toPushResult(r)
	}
	return results, nil
}

func (a *PushIPCAdapter) Delete(ctx context.Context, ownerID, id string) error {
	return a.svc.Delete(ctx, ownerID, id)
}

func (a *PushIPCAdapter) Clear(ctx context.Context, ownerID string) (int64, error) {
	return a.svc.Clear(ctx, ownerID)
}

func toPushResult(r *push.PushNotification) PushResult {
	return PushResult{
		ID:        r.ID,
		Message:   r.Message,
		FireAt:    r.FireAt,
		Recurring: r.Recurring,
		SessionID: r.SessionID,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
	}
}
