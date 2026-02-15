package rbac

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrRoleNotFound       = errors.New("role not found")
	ErrAssignmentNotFound = errors.New("assignment not found")
	ErrDuplicateAssignment = errors.New("user already has this role")
)

// UserRoleAssignment represents a user-role assignment
type UserRoleAssignment struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	RoleName  string    `json:"role_name"`
	AssignedBy string   `json:"assigned_by,omitempty"`
	AssignedAt time.Time `json:"assigned_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Revoked   bool       `json:"revoked"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	RevokedBy *string    `json:"revoked_by,omitempty"`
}

// UserRoleService handles user-role assignments
type UserRoleService struct {
	db   *sql.DB
	rbac *RBAC
}

// NewUserRoleService creates a new user role service
func NewUserRoleService(dbPath string, rbac *RBAC) (*UserRoleService, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)

	svc := &UserRoleService{db: db, rbac: rbac}
	if err := svc.initDB(); err != nil {
		db.Close()
		return nil, err
	}

	return svc, nil
}

// initDB initializes the database schema
func (s *UserRoleService) initDB() error {
	_, err := s.db.Exec(`PRAGMA journal_mode=WAL`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_role_assignments (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			role_name TEXT NOT NULL,
			assigned_by TEXT,
			assigned_at DATETIME NOT NULL,
			expires_at DATETIME,
			revoked INTEGER NOT NULL DEFAULT 0,
			revoked_at DATETIME,
			revoked_by TEXT,
			UNIQUE(user_id, role_name)
		);
		CREATE INDEX IF NOT EXISTS idx_user_role_user_id ON user_role_assignments(user_id);
		CREATE INDEX IF NOT EXISTS idx_user_role_role_name ON user_role_assignments(role_name);
	`)
	return err
}

// Close closes the database connection
func (s *UserRoleService) Close() error {
	return s.db.Close()
}

// AssignRoleRequest represents a request to assign a role to a user
type AssignRoleRequest struct {
	UserID     string
	RoleName   string
	AssignedBy string
	ExpiresAt  *time.Time
}

// AssignRole assigns a role to a user
func (s *UserRoleService) AssignRole(ctx context.Context, req *AssignRoleRequest) (*UserRoleAssignment, error) {
	// Verify role exists
	if s.rbac.GetRole(req.RoleName) == nil {
		return nil, ErrRoleNotFound
	}

	id := uuid.New().String()
	now := time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_role_assignments (id, user_id, role_name, assigned_by, assigned_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, role_name) DO UPDATE SET
			revoked = 0,
			revoked_at = NULL,
			revoked_by = NULL,
			assigned_by = excluded.assigned_by,
			assigned_at = excluded.assigned_at,
			expires_at = excluded.expires_at
	`, id, req.UserID, req.RoleName, req.AssignedBy, now, req.ExpiresAt)
	if err != nil {
		return nil, err
	}

	return &UserRoleAssignment{
		ID:         id,
		UserID:     req.UserID,
		RoleName:   req.RoleName,
		AssignedBy: req.AssignedBy,
		AssignedAt: now,
		ExpiresAt:  req.ExpiresAt,
	}, nil
}

// RevokeRoleRequest represents a request to revoke a role from a user
type RevokeRoleRequest struct {
	UserID    string
	RoleName  string
	RevokedBy string
}

// RevokeRole revokes a role from a user
func (s *UserRoleService) RevokeRole(ctx context.Context, req *RevokeRoleRequest) error {
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		UPDATE user_role_assignments
		SET revoked = 1, revoked_at = ?, revoked_by = ?
		WHERE user_id = ? AND role_name = ? AND revoked = 0
	`, now, req.RevokedBy, req.UserID, req.RoleName)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrAssignmentNotFound
	}

	return nil
}

// GetUserRoles returns all active roles for a user
func (s *UserRoleService) GetUserRoles(ctx context.Context, userID string) ([]*UserRoleAssignment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, role_name, assigned_by, assigned_at, expires_at, revoked, revoked_at, revoked_by
		FROM user_role_assignments
		WHERE user_id = ? AND revoked = 0 AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY assigned_at DESC
	`, userID, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanAssignments(rows)
}

// GetRoleUsers returns all users with a specific role
func (s *UserRoleService) GetRoleUsers(ctx context.Context, roleName string) ([]*UserRoleAssignment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, role_name, assigned_by, assigned_at, expires_at, revoked, revoked_at, revoked_by
		FROM user_role_assignments
		WHERE role_name = ? AND revoked = 0 AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY assigned_at DESC
	`, roleName, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanAssignments(rows)
}

// HasRole checks if a user has a specific role
func (s *UserRoleService) HasRole(ctx context.Context, userID, roleName string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_role_assignments
		WHERE user_id = ? AND role_name = ? AND revoked = 0
		AND (expires_at IS NULL OR expires_at > ?)
	`, userID, roleName, time.Now()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasPermission checks if a user has a specific permission through any of their roles
func (s *UserRoleService) HasPermission(ctx context.Context, userID, permission string) (bool, error) {
	roles, err := s.GetUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, assignment := range roles {
		if s.rbac.HasPermission(assignment.RoleName, permission) {
			return true, nil
		}
	}

	return false, nil
}

// GetUserPermissions returns all permissions for a user through their roles
func (s *UserRoleService) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	roles, err := s.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	permSet := make(map[string]bool)
	for _, assignment := range roles {
		perms := s.rbac.GetAllPermissions(assignment.RoleName)
		for _, perm := range perms {
			permSet[perm] = true
		}
	}

	result := make([]string, 0, len(permSet))
	for perm := range permSet {
		result = append(result, perm)
	}

	return result, nil
}

// CleanupExpiredAssignments removes expired role assignments
func (s *UserRoleService) CleanupExpiredAssignments(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE user_role_assignments
		SET revoked = 1, revoked_at = ?
		WHERE expires_at IS NOT NULL AND expires_at < ? AND revoked = 0
	`, time.Now(), time.Now())
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// GetAssignmentHistory returns the assignment history for a user
func (s *UserRoleService) GetAssignmentHistory(ctx context.Context, userID string) ([]*UserRoleAssignment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, role_name, assigned_by, assigned_at, expires_at, revoked, revoked_at, revoked_by
		FROM user_role_assignments
		WHERE user_id = ?
		ORDER BY assigned_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanAssignments(rows)
}

// scanAssignments scans rows into UserRoleAssignment slice
func (s *UserRoleService) scanAssignments(rows *sql.Rows) ([]*UserRoleAssignment, error) {
	var assignments []*UserRoleAssignment
	for rows.Next() {
		var a UserRoleAssignment
		var assignedBy, revokedBy sql.NullString
		var expiresAt, revokedAt sql.NullTime
		var revoked int

		err := rows.Scan(
			&a.ID, &a.UserID, &a.RoleName, &assignedBy, &a.AssignedAt,
			&expiresAt, &revoked, &revokedAt, &revokedBy,
		)
		if err != nil {
			return nil, err
		}

		if assignedBy.Valid {
			a.AssignedBy = assignedBy.String
		}
		if expiresAt.Valid {
			a.ExpiresAt = &expiresAt.Time
		}
		a.Revoked = revoked == 1
		if revokedAt.Valid {
			a.RevokedAt = &revokedAt.Time
		}
		if revokedBy.Valid {
			a.RevokedBy = &revokedBy.String
		}

		assignments = append(assignments, &a)
	}

	return assignments, rows.Err()
}

// BulkAssignRole assigns a role to multiple users
func (s *UserRoleService) BulkAssignRole(ctx context.Context, userIDs []string, roleName, assignedBy string) error {
	// Verify role exists
	if s.rbac.GetRole(roleName) == nil {
		return ErrRoleNotFound
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO user_role_assignments (id, user_id, role_name, assigned_by, assigned_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, role_name) DO UPDATE SET
			revoked = 0,
			revoked_at = NULL,
			revoked_by = NULL,
			assigned_by = excluded.assigned_by,
			assigned_at = excluded.assigned_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, userID := range userIDs {
		id := uuid.New().String()
		_, err := stmt.ExecContext(ctx, id, userID, roleName, assignedBy, now)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// BulkRevokeRole revokes a role from multiple users
func (s *UserRoleService) BulkRevokeRole(ctx context.Context, userIDs []string, roleName, revokedBy string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	stmt, err := tx.PrepareContext(ctx, `
		UPDATE user_role_assignments
		SET revoked = 1, revoked_at = ?, revoked_by = ?
		WHERE user_id = ? AND role_name = ? AND revoked = 0
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, userID := range userIDs {
		_, err := stmt.ExecContext(ctx, now, revokedBy, userID, roleName)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
