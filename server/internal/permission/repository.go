package permission

import (
	"context"
	"database/sql"
	"errors"

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

// GetUserPermissions returns all permissions for a user
func (r *Repository) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	query := `SELECT permission FROM user_permissions WHERE user_id = ?`
	rows, err := r.db.QueryContext(ctx, query, userID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return nil, err
		}
		permissions = append(permissions, perm)
	}

	return permissions, rows.Err()
}

// SetUserPermissions replaces all permissions for a user
func (r *Repository) SetUserPermissions(ctx context.Context, userID uuid.UUID, permissions []string, grantedBy *string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing permissions
	_, err = tx.ExecContext(ctx, `DELETE FROM user_permissions WHERE user_id = ?`, userID.String())
	if err != nil {
		return err
	}

	// Insert new permissions
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO user_permissions (id, user_id, permission, granted_by, granted_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, perm := range permissions {
		_, err = stmt.ExecContext(ctx, uuid.New().String(), userID.String(), perm, grantedBy)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// AddPermission adds a single permission to a user
func (r *Repository) AddPermission(ctx context.Context, userID uuid.UUID, permission string, grantedBy *string) error {
	query := `
		INSERT OR IGNORE INTO user_permissions (id, user_id, permission, granted_by, granted_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	_, err := r.db.ExecContext(ctx, query, uuid.New().String(), userID.String(), permission, grantedBy)
	return err
}

// RemovePermission removes a single permission from a user
func (r *Repository) RemovePermission(ctx context.Context, userID uuid.UUID, permission string) error {
	query := `DELETE FROM user_permissions WHERE user_id = ? AND permission = ?`
	_, err := r.db.ExecContext(ctx, query, userID.String(), permission)
	return err
}

// HasPermission checks if a user has a specific permission
func (r *Repository) HasPermission(ctx context.Context, userID uuid.UUID, permission string) (bool, error) {
	query := `SELECT COUNT(*) FROM user_permissions WHERE user_id = ? AND permission = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, userID.String(), permission).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteUserPermissions removes all permissions for a user
func (r *Repository) DeleteUserPermissions(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM user_permissions WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, userID.String())
	return err
}

// GetUsersWithPermission returns all user IDs that have a specific permission
func (r *Repository) GetUsersWithPermission(ctx context.Context, permission string) ([]uuid.UUID, error) {
	query := `SELECT user_id FROM user_permissions WHERE permission = ?`
	rows, err := r.db.QueryContext(ctx, query, permission)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []uuid.UUID
	for rows.Next() {
		var idStr string
		if err := rows.Scan(&idStr); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		userIDs = append(userIDs, id)
	}

	return userIDs, rows.Err()
}
