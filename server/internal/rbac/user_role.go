package rbac

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrRoleNotFound        = errors.New("role not found")
	ErrAssignmentNotFound  = errors.New("assignment not found")
	ErrDuplicateAssignment = errors.New("user already has this role")
)

// UserRoleAssignment represents a user-role assignment
type UserRoleAssignment struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	RoleName   string     `json:"role_name"`
	AssignedBy string     `json:"assigned_by,omitempty"`
	AssignedAt time.Time  `json:"assigned_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Revoked    bool       `json:"revoked"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	RevokedBy  *string    `json:"revoked_by,omitempty"`
}

// UserRoleService handles user-role assignments
type UserRoleService struct {
	db     *sql.DB
	readDB *sql.DB
	rbac   *RBAC
}

// NewUserRoleService creates a new user role service
func NewUserRoleService(dbPath string, rbac *RBAC) (*UserRoleService, error) {
	svc := &UserRoleService{rbac: rbac}
	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(dbPath, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(2)
		db.SetMaxIdleConns(1)
		openSvc := &UserRoleService{db: db, rbac: rbac}
		return openSvc.initDB()
	})
	if err != nil {
		return nil, err
	}

	svc.db = db
	readDB, readErr := openUserRoleReaderDB(dbPath)
	if readErr != nil || readDB == nil {
		readDB = db
	}
	svc.readDB = readDB
	return svc, nil
}

// initDB initializes the database schema
func (s *UserRoleService) initDB() error {
	_, err := s.db.Exec(`PRAGMA journal_mode=WAL`)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`PRAGMA synchronous=FULL`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`PRAGMA wal_autocheckpoint=1000`); err != nil {
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
	if s == nil {
		return nil
	}
	if s.readDB != nil && s.readDB != s.db {
		_ = s.readDB.Close()
	}
	return s.db.Close()
}

func (s *UserRoleService) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "user_role_assignments")
}

func (s *UserRoleService) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "user_role_assignments")
}

func (s *UserRoleService) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func openUserRoleReaderDB(dbPath string) (*sql.DB, error) {
	if dbPath == "" || dbPath == ":memory:" {
		return nil, nil
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := dbutil.OpenSQLiteWithRecovery(dsn, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(2)
		if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
			return fmt.Errorf("set user role reader busy timeout: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return db, nil
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
	if s.rbac.GetRole(req.RoleName) == nil {
		return nil, ErrRoleNotFound
	}

	id := uuid.New().String()
	now := timeutil.NowTime()

	_, err := s.table(ctx).Insert(
		map[string]interface{}{
			"id":          id,
			"user_id":     req.UserID,
			"role_name":   req.RoleName,
			"assigned_by": req.AssignedBy,
			"assigned_at": now,
			"expires_at":  req.ExpiresAt,
		},
		z.OnConflictDoUpdateSet(
			[]string{"user_id", "role_name"},
			[]string{"revoked", "revoked_at", "revoked_by", "assigned_by", "assigned_at", "expires_at"},
		),
	)
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
	now := timeutil.NowTime()
	n, err := s.table(ctx).Update(
		z.V{
			"revoked":    1,
			"revoked_at": now,
			"revoked_by": req.RevokedBy,
		},
		z.Where(
			z.Eq("user_id", req.UserID),
			z.Eq("role_name", req.RoleName),
			z.Eq("revoked", 0),
		),
	)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrAssignmentNotFound
	}
	return nil
}

// assignmentRow is used for scanning assignment rows from zorm
type assignmentRow struct {
	ID         string  `json:"id" zorm:"id"`
	UserID     string  `json:"user_id" zorm:"user_id"`
	RoleName   string  `json:"role_name" zorm:"role_name"`
	AssignedBy *string `json:"assigned_by" zorm:"assigned_by"`
	AssignedAt string  `json:"assigned_at" zorm:"assigned_at"`
	ExpiresAt  *string `json:"expires_at" zorm:"expires_at"`
	Revoked    int     `json:"revoked" zorm:"revoked"`
	RevokedAt  *string `json:"revoked_at" zorm:"revoked_at"`
	RevokedBy  *string `json:"revoked_by" zorm:"revoked_by"`
}

func rowToAssignment(row assignmentRow) *UserRoleAssignment {
	a := &UserRoleAssignment{
		ID:       row.ID,
		UserID:   row.UserID,
		RoleName: row.RoleName,
		Revoked:  row.Revoked == 1,
	}
	if row.AssignedBy != nil {
		a.AssignedBy = *row.AssignedBy
	}
	a.AssignedAt, _ = time.Parse(time.RFC3339, row.AssignedAt)
	if a.AssignedAt.IsZero() {
		a.AssignedAt, _ = time.Parse("2006-01-02 15:04:05", row.AssignedAt)
	}
	if row.ExpiresAt != nil {
		t, _ := time.Parse(time.RFC3339, *row.ExpiresAt)
		if t.IsZero() {
			t, _ = time.Parse("2006-01-02 15:04:05", *row.ExpiresAt)
		}
		if !t.IsZero() {
			a.ExpiresAt = &t
		}
	}
	if row.RevokedAt != nil {
		t, _ := time.Parse(time.RFC3339, *row.RevokedAt)
		if t.IsZero() {
			t, _ = time.Parse("2006-01-02 15:04:05", *row.RevokedAt)
		}
		if !t.IsZero() {
			a.RevokedAt = &t
		}
	}
	a.RevokedBy = row.RevokedBy
	return a
}

// GetUserRoles returns all active roles for a user
func (s *UserRoleService) GetUserRoles(ctx context.Context, userID string) ([]*UserRoleAssignment, error) {
	var rows []assignmentRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(
			z.Eq("user_id", userID),
			z.Eq("revoked", 0),
			z.Or(z.IsNull("expires_at"), z.Gt("expires_at", timeutil.NowTime())),
		),
		z.OrderBy("assigned_at DESC"),
	)
	if err != nil {
		return nil, err
	}

	assignments := make([]*UserRoleAssignment, len(rows))
	for i, row := range rows {
		assignments[i] = rowToAssignment(row)
	}
	return assignments, nil
}

// GetRoleUsers returns all users with a specific role
func (s *UserRoleService) GetRoleUsers(ctx context.Context, roleName string) ([]*UserRoleAssignment, error) {
	var rows []assignmentRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(
			z.Eq("role_name", roleName),
			z.Eq("revoked", 0),
			z.Or(z.IsNull("expires_at"), z.Gt("expires_at", timeutil.NowTime())),
		),
		z.OrderBy("assigned_at DESC"),
	)
	if err != nil {
		return nil, err
	}

	assignments := make([]*UserRoleAssignment, len(rows))
	for i, row := range rows {
		assignments[i] = rowToAssignment(row)
	}
	return assignments, nil
}

// HasRole checks if a user has a specific role
func (s *UserRoleService) HasRole(ctx context.Context, userID, roleName string) (bool, error) {
	var count int64
	_, err := s.readTable(ctx).Select(&count,
		z.Fields("count(1)"),
		z.Where(
			z.Eq("user_id", userID),
			z.Eq("role_name", roleName),
			z.Eq("revoked", 0),
			z.Or(z.IsNull("expires_at"), z.Gt("expires_at", timeutil.NowTime())),
		),
	)
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
	now := timeutil.NowTime()
	n, err := s.table(ctx).Update(
		z.V{
			"revoked":    1,
			"revoked_at": now,
		},
		z.Where(
			z.IsNotNull("expires_at"),
			z.Lt("expires_at", now),
			z.Eq("revoked", 0),
		),
	)
	return int64(n), err
}

// GetAssignmentHistory returns the assignment history for a user
func (s *UserRoleService) GetAssignmentHistory(ctx context.Context, userID string) ([]*UserRoleAssignment, error) {
	var rows []assignmentRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(z.Eq("user_id", userID)),
		z.OrderBy("assigned_at DESC"),
	)
	if err != nil {
		return nil, err
	}

	assignments := make([]*UserRoleAssignment, len(rows))
	for i, row := range rows {
		assignments[i] = rowToAssignment(row)
	}
	return assignments, nil
}

// BulkAssignRole assigns a role to multiple users
func (s *UserRoleService) BulkAssignRole(ctx context.Context, userIDs []string, roleName, assignedBy string) error {
	if s.rbac.GetRole(roleName) == nil {
		return ErrRoleNotFound
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	t := z.TableContext(ctx, tx, "user_role_assignments")
	now := timeutil.NowTime()

	for _, userID := range userIDs {
		_, err := t.Insert(
			map[string]interface{}{
				"id":          uuid.New().String(),
				"user_id":     userID,
				"role_name":   roleName,
				"assigned_by": assignedBy,
				"assigned_at": now,
			},
			z.OnConflictDoUpdateSet(
				[]string{"user_id", "role_name"},
				[]string{"revoked", "revoked_at", "revoked_by", "assigned_by", "assigned_at"},
			),
		)
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

	t := z.TableContext(ctx, tx, "user_role_assignments")
	now := timeutil.NowTime()

	for _, userID := range userIDs {
		_, err := t.Update(
			z.V{
				"revoked":    1,
				"revoked_at": now,
				"revoked_by": revokedBy,
			},
			z.Where(
				z.Eq("user_id", userID),
				z.Eq("role_name", roleName),
				z.Eq("revoked", 0),
			),
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
