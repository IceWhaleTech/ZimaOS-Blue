package user

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SQLiteRepository implements Repository using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLiteRepository.
func NewSQLiteRepository(db *sql.DB) (*SQLiteRepository, error) {
	repo := &SQLiteRepository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}
	return repo, nil
}

// migrate creates the necessary tables.
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

// Create creates a new user.
func (r *SQLiteRepository) Create(ctx context.Context, user *User) error {
	query := `INSERT INTO users (
		id, username, email, password_hash, mfa_secret, mfa_enabled,
		role, status, failed_login_attempts, locked_until, last_login_at,
		created_at, updated_at, deleted_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		user.ID.String(),
		user.Username,
		user.Email,
		user.PasswordHash,
		user.MFASecret,
		user.MFAEnabled,
		user.Role,
		user.Status,
		user.FailedLoginAttempts,
		nullTime(user.LockedUntil),
		nullTime(user.LastLoginAt),
		user.CreatedAt,
		user.UpdatedAt,
		nullTime(user.DeletedAt),
	)

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

// GetByID retrieves a user by ID.
func (r *SQLiteRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `SELECT id, username, email, password_hash, mfa_secret, mfa_enabled,
		role, status, failed_login_attempts, locked_until, last_login_at,
		created_at, updated_at, deleted_at
		FROM users WHERE id = ? AND deleted_at IS NULL`

	return r.scanUser(r.db.QueryRowContext(ctx, query, id.String()))
}

// GetByUsername retrieves a user by username.
func (r *SQLiteRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	query := `SELECT id, username, email, password_hash, mfa_secret, mfa_enabled,
		role, status, failed_login_attempts, locked_until, last_login_at,
		created_at, updated_at, deleted_at
		FROM users WHERE username = ? AND deleted_at IS NULL`

	return r.scanUser(r.db.QueryRowContext(ctx, query, username))
}

// GetByEmail retrieves a user by email.
func (r *SQLiteRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT id, username, email, password_hash, mfa_secret, mfa_enabled,
		role, status, failed_login_attempts, locked_until, last_login_at,
		created_at, updated_at, deleted_at
		FROM users WHERE email = ? AND deleted_at IS NULL`

	return r.scanUser(r.db.QueryRowContext(ctx, query, email))
}

// Update updates a user.
func (r *SQLiteRepository) Update(ctx context.Context, user *User) error {
	user.UpdatedAt = time.Now().UTC()

	query := `UPDATE users SET
		username = ?, email = ?, password_hash = ?, mfa_secret = ?, mfa_enabled = ?,
		role = ?, status = ?, failed_login_attempts = ?, locked_until = ?,
		last_login_at = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.MFASecret,
		user.MFAEnabled,
		user.Role,
		user.Status,
		user.FailedLoginAttempts,
		nullTime(user.LockedUntil),
		nullTime(user.LastLoginAt),
		user.UpdatedAt,
		user.ID.String(),
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

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

// Delete soft-deletes a user.
func (r *SQLiteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	query := `UPDATE users SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, now, now, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

// List retrieves users with pagination and filtering.
func (r *SQLiteRepository) List(ctx context.Context, query *ListUsersQuery) (*ListUsersResponse, error) {
	// Set defaults
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	// Build query
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "deleted_at IS NULL")

	if query.Search != "" {
		conditions = append(conditions, "(username LIKE ? OR email LIKE ?)")
		searchPattern := "%" + query.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	if query.Role != nil {
		conditions = append(conditions, "role = ?")
		args = append(args, *query.Role)
	}

	if query.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, *query.Status)
	}

	whereClause := strings.Join(conditions, " AND ")

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users WHERE %s", whereClause)
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	// Build order clause
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

	// Fetch users
	offset := (query.Page - 1) * query.PageSize
	selectQuery := fmt.Sprintf(`SELECT id, username, email, password_hash, mfa_secret, mfa_enabled,
		role, status, failed_login_attempts, locked_until, last_login_at,
		created_at, updated_at, deleted_at
		FROM users WHERE %s ORDER BY %s %s LIMIT ? OFFSET ?`,
		whereClause, orderBy, orderDir)

	args = append(args, query.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user, err := r.scanUserFromRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate users: %w", err)
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
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = ? AND deleted_at IS NULL)`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, username).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check username: %w", err)
	}
	return exists, nil
}

// ExistsByEmail checks if an email exists.
func (r *SQLiteRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = ? AND deleted_at IS NULL)`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, email).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check email: %w", err)
	}
	return exists, nil
}

// AddPasswordHistory adds a password hash to history.
func (r *SQLiteRepository) AddPasswordHistory(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `INSERT INTO password_history (id, user_id, password_hash, created_at) VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, uuid.New().String(), userID.String(), passwordHash, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to add password history: %w", err)
	}
	return nil
}

// GetPasswordHistory retrieves password history for a user.
func (r *SQLiteRepository) GetPasswordHistory(ctx context.Context, userID uuid.UUID, limit int) ([]string, error) {
	query := `SELECT password_hash FROM password_history WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, userID.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get password history: %w", err)
	}
	defer rows.Close()

	var hashes []string
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, fmt.Errorf("failed to scan password hash: %w", err)
		}
		hashes = append(hashes, hash)
	}

	return hashes, rows.Err()
}

// CreateSession creates a new session.
func (r *SQLiteRepository) CreateSession(ctx context.Context, session *Session) error {
	query := `INSERT INTO sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, created_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		session.ID.String(),
		session.UserID.String(),
		session.RefreshToken,
		session.UserAgent,
		session.IPAddress,
		session.ExpiresAt,
		session.CreatedAt,
		nullTime(session.RevokedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// GetSessionByID retrieves a session by ID.
func (r *SQLiteRepository) GetSessionByID(ctx context.Context, id uuid.UUID) (*Session, error) {
	query := `SELECT id, user_id, refresh_token, user_agent, ip_address, expires_at, created_at, revoked_at
		FROM sessions WHERE id = ?`
	return r.scanSession(r.db.QueryRowContext(ctx, query, id.String()))
}

// GetSessionByRefreshToken retrieves a session by refresh token.
func (r *SQLiteRepository) GetSessionByRefreshToken(ctx context.Context, token string) (*Session, error) {
	query := `SELECT id, user_id, refresh_token, user_agent, ip_address, expires_at, created_at, revoked_at
		FROM sessions WHERE refresh_token = ?`
	return r.scanSession(r.db.QueryRowContext(ctx, query, token))
}

// GetUserSessions retrieves all sessions for a user.
func (r *SQLiteRepository) GetUserSessions(ctx context.Context, userID uuid.UUID) ([]*Session, error) {
	query := `SELECT id, user_id, refresh_token, user_agent, ip_address, expires_at, created_at, revoked_at
		FROM sessions WHERE user_id = ? AND revoked_at IS NULL AND expires_at > ? ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID.String(), time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		session, err := r.scanSessionFromRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// RevokeSession revokes a session.
func (r *SQLiteRepository) RevokeSession(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE sessions SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, time.Now().UTC(), id.String())
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrSessionNotFound
	}

	return nil
}

// RevokeUserSessions revokes all sessions for a user.
func (r *SQLiteRepository) RevokeUserSessions(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, time.Now().UTC(), userID.String())
	if err != nil {
		return fmt.Errorf("failed to revoke user sessions: %w", err)
	}
	return nil
}

// CleanupExpiredSessions removes expired sessions.
func (r *SQLiteRepository) CleanupExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < ? OR revoked_at IS NOT NULL`
	_, err := r.db.ExecContext(ctx, query, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		return fmt.Errorf("failed to cleanup sessions: %w", err)
	}
	return nil
}

// Helper functions

func (r *SQLiteRepository) scanUser(row *sql.Row) (*User, error) {
	var user User
	var id, role, status string
	var email, mfaSecret sql.NullString
	var lockedUntil, lastLoginAt, deletedAt sql.NullTime

	err := row.Scan(
		&id,
		&user.Username,
		&email,
		&user.PasswordHash,
		&mfaSecret,
		&user.MFAEnabled,
		&role,
		&status,
		&user.FailedLoginAttempts,
		&lockedUntil,
		&lastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&deletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	user.ID, _ = uuid.Parse(id)
	user.Role = Role(role)
	user.Status = Status(status)

	if email.Valid {
		user.Email = &email.String
	}
	if mfaSecret.Valid {
		user.MFASecret = &mfaSecret.String
	}
	if lockedUntil.Valid {
		user.LockedUntil = &lockedUntil.Time
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Time
	}

	return &user, nil
}

func (r *SQLiteRepository) scanUserFromRows(rows *sql.Rows) (*User, error) {
	var user User
	var id, role, status string
	var email, mfaSecret sql.NullString
	var lockedUntil, lastLoginAt, deletedAt sql.NullTime

	err := rows.Scan(
		&id,
		&user.Username,
		&email,
		&user.PasswordHash,
		&mfaSecret,
		&user.MFAEnabled,
		&role,
		&status,
		&user.FailedLoginAttempts,
		&lockedUntil,
		&lastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	user.ID, _ = uuid.Parse(id)
	user.Role = Role(role)
	user.Status = Status(status)

	if email.Valid {
		user.Email = &email.String
	}
	if mfaSecret.Valid {
		user.MFASecret = &mfaSecret.String
	}
	if lockedUntil.Valid {
		user.LockedUntil = &lockedUntil.Time
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Time
	}

	return &user, nil
}

func (r *SQLiteRepository) scanSession(row *sql.Row) (*Session, error) {
	var session Session
	var id, userID string
	var revokedAt sql.NullTime

	err := row.Scan(
		&id,
		&userID,
		&session.RefreshToken,
		&session.UserAgent,
		&session.IPAddress,
		&session.ExpiresAt,
		&session.CreatedAt,
		&revokedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan session: %w", err)
	}

	session.ID, _ = uuid.Parse(id)
	session.UserID, _ = uuid.Parse(userID)

	if revokedAt.Valid {
		session.RevokedAt = &revokedAt.Time
	}

	return &session, nil
}

func (r *SQLiteRepository) scanSessionFromRows(rows *sql.Rows) (*Session, error) {
	var session Session
	var id, userID string
	var revokedAt sql.NullTime

	err := rows.Scan(
		&id,
		&userID,
		&session.RefreshToken,
		&session.UserAgent,
		&session.IPAddress,
		&session.ExpiresAt,
		&session.CreatedAt,
		&revokedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan session: %w", err)
	}

	session.ID, _ = uuid.Parse(id)
	session.UserID, _ = uuid.Parse(userID)

	if revokedAt.Valid {
		session.RevokedAt = &revokedAt.Time
	}

	return &session, nil
}

func nullTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}
