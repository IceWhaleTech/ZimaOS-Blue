package permission

import (
	"context"
	"database/sql"
	"errors"

	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
)

var (
	// ErrPermissionNotFound is returned when a permission is not found
	ErrPermissionNotFound = errors.New("permission not found")
)

// Repository handles permission persistence
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new permission repository
func NewRepository(db *sql.DB) (*Repository, error) {
	repo := &Repository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, err
	}
	return repo, nil
}

// migrate creates the necessary database tables
func (r *Repository) migrate() error {
	query := `
		CREATE TABLE IF NOT EXISTS user_permissions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			permission TEXT NOT NULL,
			granted_by TEXT,
			granted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, permission)
		);
		CREATE INDEX IF NOT EXISTS idx_user_permissions_user_id ON user_permissions(user_id);
	`
	_, err := r.db.Exec(query)
	return err
}

func (r *Repository) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "user_permissions")
}

// GetUserPermissions returns all permissions for a user
func (r *Repository) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var permissions []string
	_, err := r.table(ctx).Select(&permissions,
		z.Fields("permission"),
		z.Where(z.Eq("user_id", userID.String())),
	)
	if err != nil {
		return nil, err
	}
	return permissions, nil
}

// SetUserPermissions replaces all permissions for a user
func (r *Repository) SetUserPermissions(ctx context.Context, userID uuid.UUID, permissions []string, grantedBy *string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	t := z.TableContext(ctx, tx, "user_permissions")

	// Delete existing permissions
	_, err = t.Delete(z.Where(z.Eq("user_id", userID.String())))
	if err != nil {
		return err
	}

	// Insert new permissions
	for _, perm := range permissions {
		_, err = t.Insert(map[string]interface{}{
			"id":         uuid.New().String(),
			"user_id":    userID.String(),
			"permission": perm,
			"granted_by": grantedBy,
		})
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// AddPermission adds a single permission to a user
func (r *Repository) AddPermission(ctx context.Context, userID uuid.UUID, permission string, grantedBy *string) error {
	_, err := r.table(ctx).InsertIgnore(map[string]interface{}{
		"id":         uuid.New().String(),
		"user_id":    userID.String(),
		"permission": permission,
		"granted_by": grantedBy,
	})
	return err
}

// RemovePermission removes a single permission from a user
func (r *Repository) RemovePermission(ctx context.Context, userID uuid.UUID, permission string) error {
	_, err := r.table(ctx).Delete(
		z.Where(z.Eq("user_id", userID.String()), z.Eq("permission", permission)),
	)
	return err
}

// HasPermission checks if a user has a specific permission
func (r *Repository) HasPermission(ctx context.Context, userID uuid.UUID, permission string) (bool, error) {
	var count int64
	_, err := r.table(ctx).Select(&count,
		z.Fields("count(1)"),
		z.Where(z.Eq("user_id", userID.String()), z.Eq("permission", permission)),
	)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteUserPermissions removes all permissions for a user
func (r *Repository) DeleteUserPermissions(ctx context.Context, userID uuid.UUID) error {
	_, err := r.table(ctx).Delete(z.Where(z.Eq("user_id", userID.String())))
	return err
}

// GetUsersWithPermission returns all user IDs that have a specific permission
func (r *Repository) GetUsersWithPermission(ctx context.Context, permission string) ([]uuid.UUID, error) {
	var idStrs []string
	_, err := r.table(ctx).Select(&idStrs,
		z.Fields("user_id"),
		z.Where(z.Eq("permission", permission)),
	)
	if err != nil {
		return nil, err
	}

	var userIDs []uuid.UUID
	for _, idStr := range idStrs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		userIDs = append(userIDs, id)
	}
	return userIDs, nil
}
