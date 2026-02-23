// Package user provides user management functionality.
package user

import (
	"time"

	"github.com/google/uuid"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Status represents the account status.
type Status string

const (
	// StatusActive indicates an active account.
	StatusActive Status = "active"
	// StatusLocked indicates a locked account (due to failed login attempts).
	StatusLocked Status = "locked"
	// StatusDisabled indicates a disabled account (by admin).
	StatusDisabled Status = "disabled"
)

// Role represents the user role.
type Role string

const (
	// RoleAdmin has full access.
	RoleAdmin Role = "admin"
	// RoleUser has standard access.
	RoleUser Role = "user"
	// RoleGuest has limited access.
	RoleGuest Role = "guest"
)

// User represents a user in the system.
type User struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id" db:"id"`
	// Username is the unique username.
	Username string `json:"username" db:"username"`
	// Email is the optional email address.
	Email *string `json:"email,omitempty" db:"email"`
	// PasswordHash is the Argon2id hashed password.
	PasswordHash string `json:"-" db:"password_hash"`
	// MFASecret is the encrypted TOTP secret.
	MFASecret *string `json:"-" db:"mfa_secret"`
	// MFAEnabled indicates if MFA is enabled.
	MFAEnabled bool `json:"mfa_enabled" db:"mfa_enabled"`
	// Role is the user's role.
	Role Role `json:"role" db:"role"`
	// Status is the account status.
	Status Status `json:"status" db:"status"`
	// FailedLoginAttempts tracks consecutive failed logins.
	FailedLoginAttempts int `json:"-" db:"failed_login_attempts"`
	// LockedUntil is when the account lockout expires.
	LockedUntil *time.Time `json:"-" db:"locked_until"`
	// LastLoginAt is the last successful login time.
	LastLoginAt *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	// CreatedAt is when the user was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	// UpdatedAt is when the user was last updated.
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	// DeletedAt is when the user was soft-deleted.
	DeletedAt *time.Time `json:"-" db:"deleted_at"`
}

// NewUser creates a new User with default values.
func NewUser(username, passwordHash string) *User {
	now := timeutil.NowTime().UTC()
	return &User{
		ID:           uuid.New(),
		Username:     username,
		PasswordHash: passwordHash,
		Role:         RoleUser,
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// IsActive returns true if the user account is active and not locked.
func (u *User) IsActive() bool {
	if u.Status != StatusActive {
		return false
	}
	if u.LockedUntil != nil && timeutil.NowTime().Before(*u.LockedUntil) {
		return false
	}
	return true
}

// IsLocked returns true if the account is currently locked.
func (u *User) IsLocked() bool {
	if u.Status == StatusLocked {
		return true
	}
	if u.LockedUntil != nil && timeutil.NowTime().Before(*u.LockedUntil) {
		return true
	}
	return false
}

// PasswordHistory stores previous password hashes for history checking.
type PasswordHistory struct {
	ID           uuid.UUID `db:"id"`
	UserID       uuid.UUID `db:"user_id"`
	PasswordHash string    `db:"password_hash"`
	CreatedAt    time.Time `db:"created_at"`
}

// Session represents a user session.
type Session struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	UserID       uuid.UUID  `json:"user_id" db:"user_id"`
	RefreshToken string     `json:"-" db:"refresh_token"`
	UserAgent    string     `json:"user_agent" db:"user_agent"`
	IPAddress    string     `json:"ip_address" db:"ip_address"`
	ExpiresAt    time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	RevokedAt    *time.Time `json:"-" db:"revoked_at"`
}

// NewSession creates a new Session.
func NewSession(userID uuid.UUID, refreshToken, userAgent, ipAddress string, expiresAt time.Time) *Session {
	return &Session{
		ID:           uuid.New(),
		UserID:       userID,
		RefreshToken: refreshToken,
		UserAgent:    userAgent,
		IPAddress:    ipAddress,
		ExpiresAt:    expiresAt,
		CreatedAt:    timeutil.NowTime().UTC(),
	}
}

// IsValid returns true if the session is not expired and not revoked.
func (s *Session) IsValid() bool {
	if s.RevokedAt != nil {
		return false
	}
	return timeutil.NowTime().Before(s.ExpiresAt)
}

// CreateUserRequest represents a request to create a user.
type CreateUserRequest struct {
	Username    string   `json:"username" validate:"required,min=3,max=50"`
	Email       *string  `json:"email,omitempty" validate:"omitempty,email"`
	Password    string   `json:"password" validate:"required"`
	Role        Role     `json:"role,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// UpdateUserRequest represents a request to update a user.
type UpdateUserRequest struct {
	Email       *string  `json:"email,omitempty" validate:"omitempty,email"`
	Role        *Role    `json:"role,omitempty"`
	Status      *Status  `json:"status,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// ResetPasswordRequest represents a request to reset a user's password.
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" validate:"required"`
}

// ChangePasswordRequest represents a request to change password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required"`
}

// ListUsersQuery represents query parameters for listing users.
type ListUsersQuery struct {
	Page     int    `query:"page"`
	PageSize int    `query:"page_size"`
	Search   string `query:"search"`
	Role     *Role  `query:"role"`
	Status   *Status `query:"status"`
	SortBy   string `query:"sort_by"`
	SortDir  string `query:"sort_dir"`
}

// ListUsersResponse represents a paginated list of users.
type ListUsersResponse struct {
	Users      []*User `json:"users"`
	Total      int64   `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
	TotalPages int     `json:"total_pages"`
}
