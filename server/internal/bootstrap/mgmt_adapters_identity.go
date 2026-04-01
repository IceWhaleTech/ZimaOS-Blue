package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

type mgmtUserAdapter struct {
	service *user.Service
}

func (a *mgmtUserAdapter) ListUsers(ctx context.Context) ([]tools.AdminUserInfo, error) {
	resp, err := a.service.List(ctx, &user.ListUsersQuery{})
	if err != nil {
		return nil, err
	}
	result := make([]tools.AdminUserInfo, 0, len(resp.Users))
	for _, u := range resp.Users {
		result = append(result, tools.AdminUserInfo{
			ID:       u.ID.String(),
			Username: u.Username,
			Role:     string(u.Role),
			Locked:   u.Status == user.StatusLocked,
		})
	}
	return result, nil
}

func (a *mgmtUserAdapter) LockUser(ctx context.Context, id string) error {
	uid, err := parseUserUUID(id)
	if err != nil {
		return err
	}
	return a.service.Lock(ctx, uid)
}

func (a *mgmtUserAdapter) UnlockUser(ctx context.Context, id string) error {
	uid, err := parseUserUUID(id)
	if err != nil {
		return err
	}
	return a.service.Unlock(ctx, uid)
}

type mgmtAPIKeyAdapter struct {
	service *auth.APIKeyService
}

func (a *mgmtAPIKeyAdapter) ListKeys(ctx context.Context) ([]tools.AdminAPIKeyInfo, error) {
	userID := tools.GetUserID(ctx)
	if userID == "" {
		userID = "default"
	}
	keys, err := a.service.ListKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]tools.AdminAPIKeyInfo, 0, len(keys))
	for _, k := range keys {
		result = append(result, tools.AdminAPIKeyInfo{
			ID:        k.ID,
			Name:      k.Name,
			Prefix:    k.Prefix,
			CreatedAt: k.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}

func (a *mgmtAPIKeyAdapter) CreateKey(ctx context.Context, name string) (*tools.AdminAPIKeyCreateResult, error) {
	userID := tools.GetUserID(ctx)
	if userID == "" {
		userID = "default"
	}
	info, err := a.service.CreateKey(ctx, &auth.CreateKeyRequest{
		Name:   name,
		UserID: userID,
		Scopes: []string{"chat", "api"},
	})
	if err != nil {
		return nil, err
	}
	return &tools.AdminAPIKeyCreateResult{
		ID:     info.ID,
		Name:   info.Name,
		Key:    info.Key,
		Prefix: info.Prefix,
	}, nil
}

func (a *mgmtAPIKeyAdapter) RevokeKey(ctx context.Context, id string) error {
	userID := tools.GetUserID(ctx)
	if userID == "" {
		userID = "default"
	}
	return a.service.RevokeKey(ctx, id, userID)
}

func parseUserUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("invalid UUID: %s", s)
	}
	return id, nil
}
