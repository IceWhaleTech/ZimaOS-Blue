package user

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	// ErrUserNotFound is returned when a user is not found.
	ErrUserNotFound = errors.New("user not found")
	// ErrUserExists is returned when a user already exists.
	ErrUserExists = errors.New("user already exists")
	// ErrUsernameExists is returned when username is taken.
	ErrUsernameExists = errors.New("username already exists")
	// ErrEmailExists is returned when email is taken.
	ErrEmailExists = errors.New("email already exists")
	// ErrSessionNotFound is returned when a session is not found.
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionExpired is returned when a session has expired.
	ErrSessionExpired = errors.New("session expired")
	// ErrSessionRevoked is returned when a session has been revoked.
	ErrSessionRevoked = errors.New("session revoked")
)

// Repository defines the interface for user data access.
type Repository interface {
	// Create creates a new user.
	Create(ctx context.Context, user *User) error
	// GetByID retrieves a user by ID.
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	// GetByUsername retrieves a user by username.
	GetByUsername(ctx context.Context, username string) (*User, error)
	// GetByEmail retrieves a user by email.
	GetByEmail(ctx context.Context, email string) (*User, error)
	// Update updates a user.
	Update(ctx context.Context, user *User) error
	// Delete soft-deletes a user.
	Delete(ctx context.Context, id uuid.UUID) error
	// List retrieves users with pagination and filtering.
	List(ctx context.Context, query *ListUsersQuery) (*ListUsersResponse, error)
	// ExistsByUsername checks if a username exists.
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	// ExistsByEmail checks if an email exists.
	ExistsByEmail(ctx context.Context, email string) (bool, error)

	// Password history
	// AddPasswordHistory adds a password hash to history.
	AddPasswordHistory(ctx context.Context, userID uuid.UUID, passwordHash string) error
	// GetPasswordHistory retrieves password history for a user.
	GetPasswordHistory(ctx context.Context, userID uuid.UUID, limit int) ([]string, error)

	// Sessions
	// CreateSession creates a new session.
	CreateSession(ctx context.Context, session *Session) error
	// GetSessionByID retrieves a session by ID.
	GetSessionByID(ctx context.Context, id uuid.UUID) (*Session, error)
	// GetSessionByRefreshToken retrieves a session by refresh token.
	GetSessionByRefreshToken(ctx context.Context, token string) (*Session, error)
	// GetUserSessions retrieves all sessions for a user.
	GetUserSessions(ctx context.Context, userID uuid.UUID) ([]*Session, error)
	// RevokeSession revokes a session.
	RevokeSession(ctx context.Context, id uuid.UUID) error
	// RevokeUserSessions revokes all sessions for a user.
	RevokeUserSessions(ctx context.Context, userID uuid.UUID) error
	// CleanupExpiredSessions removes expired sessions.
	CleanupExpiredSessions(ctx context.Context) error
}
