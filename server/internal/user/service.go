package user

import (
	"context"
	"errors"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/password"
	"github.com/google/uuid"
)

var (
	// ErrInvalidCredentials is returned when login credentials are invalid.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrAccountLocked is returned when the account is locked.
	ErrAccountLocked = errors.New("account is locked")
	// ErrAccountDisabled is returned when the account is disabled.
	ErrAccountDisabled = errors.New("account is disabled")
	// ErrMFARequired is returned when MFA verification is required.
	ErrMFARequired = errors.New("MFA verification required")
	// ErrInvalidPassword is returned when password doesn't meet policy.
	ErrInvalidPassword = errors.New("password does not meet requirements")
)

// ServiceConfig holds configuration for the user service.
type ServiceConfig struct {
	// LockoutThreshold is the number of failed attempts before lockout.
	LockoutThreshold int
	// LockoutDuration is how long the account is locked.
	LockoutDuration time.Duration
	// PasswordHistoryCount is the number of passwords to check for reuse.
	PasswordHistoryCount int
	// SessionDuration is how long sessions are valid.
	SessionDuration time.Duration
}

// DefaultServiceConfig returns the default service configuration.
func DefaultServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		LockoutThreshold:     5,
		LockoutDuration:      15 * time.Minute,
		PasswordHistoryCount: 5,
		SessionDuration:      30 * 24 * time.Hour, // 30 days
	}
}

// Service provides user management operations.
type Service struct {
	repo           Repository
	passwordHasher *password.Hasher
	passwordPolicy *password.Policy
	config         *ServiceConfig
}

// NewService creates a new user service.
func NewService(repo Repository, hasher *password.Hasher, policy *password.Policy, config *ServiceConfig) *Service {
	if hasher == nil {
		hasher = password.NewHasher(nil)
	}
	if policy == nil {
		policy = password.NewPolicy(nil)
	}
	if config == nil {
		config = DefaultServiceConfig()
	}
	return &Service{
		repo:           repo,
		passwordHasher: hasher,
		passwordPolicy: policy,
		config:         config,
	}
}

// Create creates a new user.
func (s *Service) Create(ctx context.Context, req *CreateUserRequest) (*User, error) {
	// Validate password
	if errs := s.passwordPolicy.Validate(req.Password); len(errs) > 0 {
		return nil, ErrInvalidPassword
	}

	// Check if username exists
	exists, err := s.repo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameExists
	}

	// Check if email exists
	if req.Email != nil {
		exists, err = s.repo.ExistsByEmail(ctx, *req.Email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrEmailExists
		}
	}

	// Hash password
	hash, err := s.passwordHasher.Hash(req.Password)
	if err != nil {
		return nil, err
	}

	// Create user
	user := NewUser(req.Username, hash)
	user.Email = req.Email
	if req.Role != "" {
		user.Role = req.Role
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Add to password history
	_ = s.repo.AddPasswordHistory(ctx, user.ID, hash)

	return user, nil
}

// GetByID retrieves a user by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByUsername retrieves a user by username.
func (s *Service) GetByUsername(ctx context.Context, username string) (*User, error) {
	return s.repo.GetByUsername(ctx, username)
}

// GetByEmail retrieves a user by email.
func (s *Service) GetByEmail(ctx context.Context, email string) (*User, error) {
	return s.repo.GetByEmail(ctx, email)
}

// Update updates a user.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *UpdateUserRequest) (*User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check email uniqueness if changing
	if req.Email != nil && (user.Email == nil || *req.Email != *user.Email) {
		exists, err := s.repo.ExistsByEmail(ctx, *req.Email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrEmailExists
		}
		user.Email = req.Email
	}

	if req.Role != nil {
		user.Role = *req.Role
	}

	if req.Status != nil {
		user.Status = *req.Status
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// Delete soft-deletes a user.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// Revoke all sessions
	_ = s.repo.RevokeUserSessions(ctx, id)
	return s.repo.Delete(ctx, id)
}

// List retrieves users with pagination and filtering.
func (s *Service) List(ctx context.Context, query *ListUsersQuery) (*ListUsersResponse, error) {
	return s.repo.List(ctx, query)
}

// ChangePassword changes a user's password.
func (s *Service) ChangePassword(ctx context.Context, id uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Verify current password
	if err := s.passwordHasher.Verify(currentPassword, user.PasswordHash); err != nil {
		return ErrInvalidCredentials
	}

	// Validate new password
	history, _ := s.repo.GetPasswordHistory(ctx, id, s.config.PasswordHistoryCount)
	if errs := s.passwordPolicy.ValidateWithHistory(newPassword, history, s.passwordHasher); len(errs) > 0 {
		return ErrInvalidPassword
	}

	// Hash new password
	hash, err := s.passwordHasher.Hash(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = hash
	if err := s.repo.Update(ctx, user); err != nil {
		return err
	}

	// Add to password history
	_ = s.repo.AddPasswordHistory(ctx, id, hash)

	// Revoke all sessions (force re-login)
	_ = s.repo.RevokeUserSessions(ctx, id)

	return nil
}

// Lock locks a user account.
func (s *Service) Lock(ctx context.Context, id uuid.UUID) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	user.Status = StatusLocked
	return s.repo.Update(ctx, user)
}

// Unlock unlocks a user account.
func (s *Service) Unlock(ctx context.Context, id uuid.UUID) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	user.Status = StatusActive
	user.FailedLoginAttempts = 0
	user.LockedUntil = nil
	return s.repo.Update(ctx, user)
}

// Authenticate authenticates a user with username and password.
// Returns the user if successful, or an error.
// If MFA is enabled, returns ErrMFARequired.
func (s *Service) Authenticate(ctx context.Context, username, password string) (*User, error) {
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		if err == ErrUserNotFound {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Check account status
	if user.Status == StatusDisabled {
		return nil, ErrAccountDisabled
	}

	// Check if locked
	if user.IsLocked() {
		return nil, ErrAccountLocked
	}

	// Verify password
	if err := s.passwordHasher.Verify(password, user.PasswordHash); err != nil {
		// Increment failed attempts
		user.FailedLoginAttempts++
		if user.FailedLoginAttempts >= s.config.LockoutThreshold {
			lockUntil := time.Now().Add(s.config.LockoutDuration)
			user.LockedUntil = &lockUntil
		}
		_ = s.repo.Update(ctx, user)
		return nil, ErrInvalidCredentials
	}

	// Reset failed attempts on successful login
	user.FailedLoginAttempts = 0
	user.LockedUntil = nil
	now := time.Now().UTC()
	user.LastLoginAt = &now
	_ = s.repo.Update(ctx, user)

	// Check if MFA is required
	if user.MFAEnabled {
		return user, ErrMFARequired
	}

	return user, nil
}

// CreateSession creates a new session for a user.
func (s *Service) CreateSession(ctx context.Context, userID uuid.UUID, refreshToken, userAgent, ipAddress string) (*Session, error) {
	expiresAt := time.Now().Add(s.config.SessionDuration)
	session := NewSession(userID, refreshToken, userAgent, ipAddress, expiresAt)

	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

// GetSession retrieves a session by refresh token.
func (s *Service) GetSession(ctx context.Context, refreshToken string) (*Session, error) {
	session, err := s.repo.GetSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	if !session.IsValid() {
		if session.RevokedAt != nil {
			return nil, ErrSessionRevoked
		}
		return nil, ErrSessionExpired
	}

	return session, nil
}

// RevokeSession revokes a session.
func (s *Service) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	return s.repo.RevokeSession(ctx, sessionID)
}

// RevokeAllSessions revokes all sessions for a user.
func (s *Service) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	return s.repo.RevokeUserSessions(ctx, userID)
}

// GetUserSessions retrieves all active sessions for a user.
func (s *Service) GetUserSessions(ctx context.Context, userID uuid.UUID) ([]*Session, error) {
	return s.repo.GetUserSessions(ctx, userID)
}

// ExistsByUsername checks if a username exists.
func (s *Service) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	return s.repo.ExistsByUsername(ctx, username)
}

// AdminExists checks if any admin user exists.
func (s *Service) AdminExists(ctx context.Context) (bool, error) {
	return s.repo.AdminExists(ctx)
}

// ResetPassword resets a user's password (admin action, no current password required).
func (s *Service) ResetPassword(ctx context.Context, id uuid.UUID, newPassword string) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Validate new password
	if errs := s.passwordPolicy.Validate(newPassword); len(errs) > 0 {
		return ErrInvalidPassword
	}

	// Hash new password
	hash, err := s.passwordHasher.Hash(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = hash
	if err := s.repo.Update(ctx, user); err != nil {
		return err
	}

	// Add to password history
	_ = s.repo.AddPasswordHistory(ctx, id, hash)

	// Revoke all sessions (force re-login)
	_ = s.repo.RevokeUserSessions(ctx, id)

	return nil
}
