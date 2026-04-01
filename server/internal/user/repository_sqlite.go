package user

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
)

// SQLiteRepository implements Repository using SQLite.
type SQLiteRepository struct {
	db     *sql.DB
	readDB *sql.DB
}

// NewSQLiteRepository creates a new SQLiteRepository.
func NewSQLiteRepository(db *sql.DB) (*SQLiteRepository, error) {
	return NewSQLiteRepositoryWithReadDB(db, db)
}

// NewSQLiteRepositoryWithReadDB creates a new SQLiteRepository with separate
// write and read database handles.
func NewSQLiteRepositoryWithReadDB(writeDB, readDB *sql.DB) (*SQLiteRepository, error) {
	if readDB == nil {
		readDB = writeDB
	}
	repo := &SQLiteRepository{db: writeDB, readDB: readDB}
	if err := repo.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}
	return repo, nil
}

func (r *SQLiteRepository) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE,
			password_hash TEXT NOT NULL,
			mfa_secret TEXT,
			mfa_enabled INTEGER NOT NULL DEFAULT 0,
			role TEXT NOT NULL DEFAULT 'user',
			status TEXT NOT NULL DEFAULT 'active',
			failed_login_attempts INTEGER NOT NULL DEFAULT 0,
			locked_until DATETIME,
			last_login_at DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			deleted_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_users_status ON users(status) WHERE deleted_at IS NULL`,
		`CREATE TABLE IF NOT EXISTS password_history (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_password_history_user_id ON password_history(user_id)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			refresh_token TEXT UNIQUE NOT NULL,
			user_agent TEXT,
			ip_address TEXT,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL,
			revoked_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_refresh_token ON sessions(refresh_token)`,
	}

	for _, query := range queries {
		if _, err := r.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
	}
	return nil
}

func (r *SQLiteRepository) usersTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "users")
}

func (r *SQLiteRepository) usersReadTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.readDB, "users")
}

func (r *SQLiteRepository) sessionsTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "sessions")
}

func (r *SQLiteRepository) sessionsReadTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.readDB, "sessions")
}

func (r *SQLiteRepository) passwordHistoryTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "password_history")
}

func (r *SQLiteRepository) passwordHistoryReadTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.readDB, "password_history")
}

// userRow is the intermediate struct for zorm scanning
type userRow struct {
	ID                  string  `json:"id" zorm:"id"`
	Username            string  `json:"username" zorm:"username"`
	Email               *string `json:"email" zorm:"email"`
	PasswordHash        string  `json:"password_hash" zorm:"password_hash"`
	MFASecret           *string `json:"mfa_secret" zorm:"mfa_secret"`
	MFAEnabled          int     `json:"mfa_enabled" zorm:"mfa_enabled"`
	Role                string  `json:"role" zorm:"role"`
	Status              string  `json:"status" zorm:"status"`
	FailedLoginAttempts int     `json:"failed_login_attempts" zorm:"failed_login_attempts"`
	LockedUntil         *string `json:"locked_until" zorm:"locked_until"`
	LastLoginAt         *string `json:"last_login_at" zorm:"last_login_at"`
	CreatedAt           string  `json:"created_at" zorm:"created_at"`
	UpdatedAt           string  `json:"updated_at" zorm:"updated_at"`
	DeletedAt           *string `json:"deleted_at" zorm:"deleted_at"`
}

func parseTimeStr(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02T15:04:05Z", s)
	}
	return t
}

func parseTimePtrStr(s *string) *time.Time {
	if s == nil {
		return nil
	}
	t := parseTimeStr(*s)
	if t.IsZero() {
		return nil
	}
	return &t
}

func rowToUser(row userRow) *User {
	u := &User{
		Username:            row.Username,
		Email:               row.Email,
		PasswordHash:        row.PasswordHash,
		MFASecret:           row.MFASecret,
		MFAEnabled:          row.MFAEnabled != 0,
		Role:                Role(row.Role),
		Status:              Status(row.Status),
		FailedLoginAttempts: row.FailedLoginAttempts,
		LockedUntil:         parseTimePtrStr(row.LockedUntil),
		LastLoginAt:         parseTimePtrStr(row.LastLoginAt),
		CreatedAt:           parseTimeStr(row.CreatedAt),
		UpdatedAt:           parseTimeStr(row.UpdatedAt),
		DeletedAt:           parseTimePtrStr(row.DeletedAt),
	}
	u.ID, _ = uuid.Parse(row.ID)
	return u
}

// Create creates a new user.
func (r *SQLiteRepository) Create(ctx context.Context, user *User) error {
	_, err := r.usersTable(ctx).Insert(map[string]interface{}{
		"id":                    user.ID.String(),
		"username":              user.Username,
		"email":                 user.Email,
		"password_hash":         user.PasswordHash,
		"mfa_secret":            user.MFASecret,
		"mfa_enabled":           user.MFAEnabled,
		"role":                  user.Role,
		"status":                user.Status,
		"failed_login_attempts": user.FailedLoginAttempts,
		"locked_until":          user.LockedUntil,
		"last_login_at":         user.LastLoginAt,
		"created_at":            user.CreatedAt,
		"updated_at":            user.UpdatedAt,
		"deleted_at":            user.DeletedAt,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.username") {
			return ErrUsernameExists
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.email") {
			return ErrEmailExists
		}
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) findOneUser(ctx context.Context, opts ...z.ZormItem) (*User, error) {
	var rows []userRow
	opts = append(opts, z.Limit(1))
	_, err := r.usersReadTable(ctx).Select(&rows, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrUserNotFound
	}
	return rowToUser(rows[0]), nil
}

// GetByID retrieves a user by ID.
func (r *SQLiteRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return r.findOneUser(ctx, z.Where(z.Eq("id", id.String()), z.IsNull("deleted_at")))
}

// GetByUsername retrieves a user by username.
func (r *SQLiteRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	return r.findOneUser(ctx, z.Where(z.Eq("username", username), z.IsNull("deleted_at")))
}

// GetByEmail retrieves a user by email.
func (r *SQLiteRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	return r.findOneUser(ctx, z.Where(z.Eq("email", email), z.IsNull("deleted_at")))
}

// Update updates a user.
func (r *SQLiteRepository) Update(ctx context.Context, user *User) error {
	user.UpdatedAt = timeutil.NowTime().UTC()
	n, err := r.usersTable(ctx).Update(
		z.V{
			"username":              user.Username,
			"email":                 user.Email,
			"password_hash":         user.PasswordHash,
			"mfa_secret":            user.MFASecret,
			"mfa_enabled":           user.MFAEnabled,
			"role":                  user.Role,
			"status":                user.Status,
			"failed_login_attempts": user.FailedLoginAttempts,
			"locked_until":          user.LockedUntil,
			"last_login_at":         user.LastLoginAt,
			"updated_at":            user.UpdatedAt,
		},
		z.Where(z.Eq("id", user.ID.String()), z.IsNull("deleted_at")),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.username") {
			return ErrUsernameExists
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.email") {
			return ErrEmailExists
		}
		return fmt.Errorf("failed to update user: %w", err)
	}
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// Delete soft-deletes a user.
func (r *SQLiteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	now := timeutil.NowTime().UTC()
	n, err := r.usersTable(ctx).Update(
		z.V{"deleted_at": now, "updated_at": now},
		z.Where(z.Eq("id", id.String()), z.IsNull("deleted_at")),
	)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// List retrieves users with pagination and filtering.
func (r *SQLiteRepository) List(ctx context.Context, query *ListUsersQuery) (*ListUsersResponse, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	// Build conditions
	conds := []interface{}{z.IsNull("deleted_at")}
	if query.Search != "" {
		searchPattern := "%" + query.Search + "%"
		conds = append(conds, z.Or(z.Like("username", searchPattern), z.Like("email", searchPattern)))
	}
	if query.Role != nil {
		conds = append(conds, z.Eq("role", string(*query.Role)))
	}
	if query.Status != nil {
		conds = append(conds, z.Eq("status", string(*query.Status)))
	}

	// Count total
	var total int64
	_, err := r.usersReadTable(ctx).Select(&total,
		z.Fields("count(1)"),
		z.Where(conds...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	// Build order
	orderBy := "created_at"
	if query.SortBy != "" {
		allowedSorts := map[string]bool{"username": true, "email": true, "created_at": true, "updated_at": true, "last_login_at": true}
		if allowedSorts[query.SortBy] {
			orderBy = query.SortBy
		}
	}
	orderDir := "DESC"
	if query.SortDir == "asc" {
		orderDir = "ASC"
	}

	offset := (query.Page - 1) * query.PageSize
	var rows []userRow
	_, err = r.usersReadTable(ctx).Select(&rows,
		z.Where(conds...),
		z.OrderBy(orderBy+" "+orderDir),
		z.Limit(query.PageSize, offset),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	users := make([]*User, len(rows))
	for i, row := range rows {
		users[i] = rowToUser(row)
	}

	totalPages := int(total) / query.PageSize
	if int(total)%query.PageSize > 0 {
		totalPages++
	}

	return &ListUsersResponse{
		Users:      users,
		Total:      total,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: totalPages,
	}, nil
}

// ExistsByUsername checks if a username exists.
func (r *SQLiteRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	_, err := r.usersReadTable(ctx).Select(&count,
		z.Fields("count(1)"),
		z.Where(z.Eq("username", username), z.IsNull("deleted_at")),
	)
	if err != nil {
		return false, fmt.Errorf("failed to check username: %w", err)
	}
	return count > 0, nil
}

// ExistsByEmail checks if an email exists.
func (r *SQLiteRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	_, err := r.usersReadTable(ctx).Select(&count,
		z.Fields("count(1)"),
		z.Where(z.Eq("email", email), z.IsNull("deleted_at")),
	)
	if err != nil {
		return false, fmt.Errorf("failed to check email: %w", err)
	}
	return count > 0, nil
}

// AnyUserExists checks if any non-deleted user exists.
func (r *SQLiteRepository) AnyUserExists(ctx context.Context) (bool, error) {
	var count int64
	_, err := r.usersReadTable(ctx).Select(&count,
		z.Fields("count(1)"),
		z.Where(z.IsNull("deleted_at")),
	)
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}
	return count > 0, nil
}

// AdminExists checks if any admin user exists.
func (r *SQLiteRepository) AdminExists(ctx context.Context) (bool, error) {
	var count int64
	_, err := r.usersReadTable(ctx).Select(&count,
		z.Fields("count(1)"),
		z.Where(z.Eq("role", "admin"), z.IsNull("deleted_at")),
	)
	if err != nil {
		return false, fmt.Errorf("failed to check admin exists: %w", err)
	}
	return count > 0, nil
}

// AddPasswordHistory adds a password hash to history.
func (r *SQLiteRepository) AddPasswordHistory(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	_, err := r.passwordHistoryTable(ctx).Insert(map[string]interface{}{
		"id":            uuid.New().String(),
		"user_id":       userID.String(),
		"password_hash": passwordHash,
		"created_at":    timeutil.NowTime().UTC(),
	})
	if err != nil {
		return fmt.Errorf("failed to add password history: %w", err)
	}
	return nil
}

// GetPasswordHistory retrieves password history for a user.
func (r *SQLiteRepository) GetPasswordHistory(ctx context.Context, userID uuid.UUID, limit int) ([]string, error) {
	var hashes []string
	_, err := r.passwordHistoryReadTable(ctx).Select(&hashes,
		z.Fields("password_hash"),
		z.Where(z.Eq("user_id", userID.String())),
		z.OrderBy("created_at DESC, rowid DESC"),
		z.Limit(limit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get password history: %w", err)
	}
	return hashes, nil
}

// sessionRow is the intermediate struct for zorm scanning
type sessionRow struct {
	ID           string  `json:"id" zorm:"id"`
	UserID       string  `json:"user_id" zorm:"user_id"`
	RefreshToken string  `json:"refresh_token" zorm:"refresh_token"`
	UserAgent    string  `json:"user_agent" zorm:"user_agent"`
	IPAddress    string  `json:"ip_address" zorm:"ip_address"`
	ExpiresAt    string  `json:"expires_at" zorm:"expires_at"`
	CreatedAt    string  `json:"created_at" zorm:"created_at"`
	RevokedAt    *string `json:"revoked_at" zorm:"revoked_at"`
}

func rowToSession(row sessionRow) *Session {
	s := &Session{
		RefreshToken: row.RefreshToken,
		UserAgent:    row.UserAgent,
		IPAddress:    row.IPAddress,
		ExpiresAt:    parseTimeStr(row.ExpiresAt),
		CreatedAt:    parseTimeStr(row.CreatedAt),
		RevokedAt:    parseTimePtrStr(row.RevokedAt),
	}
	s.ID, _ = uuid.Parse(row.ID)
	s.UserID, _ = uuid.Parse(row.UserID)
	return s
}

// CreateSession creates a new session.
func (r *SQLiteRepository) CreateSession(ctx context.Context, session *Session) error {
	_, err := r.sessionsTable(ctx).Insert(map[string]interface{}{
		"id":            session.ID.String(),
		"user_id":       session.UserID.String(),
		"refresh_token": session.RefreshToken,
		"user_agent":    session.UserAgent,
		"ip_address":    session.IPAddress,
		"expires_at":    session.ExpiresAt,
		"created_at":    session.CreatedAt,
		"revoked_at":    session.RevokedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) findOneSession(ctx context.Context, opts ...z.ZormItem) (*Session, error) {
	var rows []sessionRow
	opts = append(opts, z.Limit(1))
	_, err := r.sessionsReadTable(ctx).Select(&rows, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrSessionNotFound
	}
	return rowToSession(rows[0]), nil
}

// GetSessionByID retrieves a session by ID.
func (r *SQLiteRepository) GetSessionByID(ctx context.Context, id uuid.UUID) (*Session, error) {
	return r.findOneSession(ctx, z.Where(z.Eq("id", id.String())))
}

// GetSessionByRefreshToken retrieves a session by refresh token.
func (r *SQLiteRepository) GetSessionByRefreshToken(ctx context.Context, token string) (*Session, error) {
	return r.findOneSession(ctx, z.Where(z.Eq("refresh_token", token)))
}

// GetUserSessions retrieves all sessions for a user.
func (r *SQLiteRepository) GetUserSessions(ctx context.Context, userID uuid.UUID) ([]*Session, error) {
	var rows []sessionRow
	_, err := r.sessionsReadTable(ctx).Select(&rows,
		z.Where(
			z.Eq("user_id", userID.String()),
			z.IsNull("revoked_at"),
			z.Gt("expires_at", timeutil.NowTime().UTC()),
		),
		z.OrderBy("created_at DESC"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}

	sessions := make([]*Session, len(rows))
	for i, row := range rows {
		sessions[i] = rowToSession(row)
	}
	return sessions, nil
}

// RevokeSession revokes a session.
func (r *SQLiteRepository) RevokeSession(ctx context.Context, id uuid.UUID) error {
	n, err := r.sessionsTable(ctx).Update(
		z.V{"revoked_at": timeutil.NowTime().UTC()},
		z.Where(z.Eq("id", id.String()), z.IsNull("revoked_at")),
	)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}
	if n == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// RevokeUserSessions revokes all sessions for a user.
func (r *SQLiteRepository) RevokeUserSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := r.sessionsTable(ctx).Update(
		z.V{"revoked_at": timeutil.NowTime().UTC()},
		z.Where(z.Eq("user_id", userID.String()), z.IsNull("revoked_at")),
	)
	if err != nil {
		return fmt.Errorf("failed to revoke user sessions: %w", err)
	}
	return nil
}

// CleanupExpiredSessions removes expired sessions.
func (r *SQLiteRepository) CleanupExpiredSessions(ctx context.Context) error {
	_, err := r.sessionsTable(ctx).Delete(
		z.Where(z.Or(
			z.Lt("expires_at", timeutil.NowTime().UTC().Add(-24*time.Hour)),
			z.IsNotNull("revoked_at"),
		)),
	)
	if err != nil {
		return fmt.Errorf("failed to cleanup sessions: %w", err)
	}
	return nil
}

func nullTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}
