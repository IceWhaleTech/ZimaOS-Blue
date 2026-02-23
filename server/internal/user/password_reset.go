package user

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

var (
	// ErrResetTokenNotFound is returned when a reset token is not found.
	ErrResetTokenNotFound = errors.New("reset token not found")
	// ErrResetTokenExpired is returned when a reset token has expired.
	ErrResetTokenExpired = errors.New("reset token expired")
	// ErrResetTokenUsed is returned when a reset token has already been used.
	ErrResetTokenUsed = errors.New("reset token already used")
	// ErrTooManyResetRequests is returned when too many reset requests have been made.
	ErrTooManyResetRequests = errors.New("too many reset requests")
)

// PasswordResetToken represents a password reset token.
type PasswordResetToken struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	Token     string     `json:"-" db:"token"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	UsedAt    *time.Time `json:"-" db:"used_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

// IsValid returns true if the token is not expired and not used.
func (t *PasswordResetToken) IsValid() bool {
	if t.UsedAt != nil {
		return false
	}
	return timeutil.NowTime().Before(t.ExpiresAt)
}

// PasswordResetConfig holds configuration for password reset.
type PasswordResetConfig struct {
	// TokenTTL is how long reset tokens are valid.
	TokenTTL time.Duration
	// MaxRequestsPerHour limits reset requests per user per hour.
	MaxRequestsPerHour int
	// TokenLength is the length of the reset token in bytes.
	TokenLength int
}

// DefaultPasswordResetConfig returns the default configuration.
func DefaultPasswordResetConfig() *PasswordResetConfig {
	return &PasswordResetConfig{
		TokenTTL:           time.Hour,
		MaxRequestsPerHour: 3,
		TokenLength:        32,
	}
}

// PasswordResetStore defines the interface for storing reset tokens.
type PasswordResetStore interface {
	// Create creates a new reset token.
	Create(ctx context.Context, token *PasswordResetToken) error
	// GetByToken retrieves a reset token by token string.
	GetByToken(ctx context.Context, token string) (*PasswordResetToken, error)
	// MarkUsed marks a token as used.
	MarkUsed(ctx context.Context, id uuid.UUID) error
	// CountRecentByUser counts recent tokens for a user.
	CountRecentByUser(ctx context.Context, userID uuid.UUID, since time.Time) (int, error)
	// DeleteExpired removes expired tokens.
	DeleteExpired(ctx context.Context) error
}

// InMemoryPasswordResetStore is an in-memory implementation of PasswordResetStore.
type InMemoryPasswordResetStore struct {
	mu     sync.RWMutex
	tokens map[string]*PasswordResetToken
}

// NewInMemoryPasswordResetStore creates a new in-memory store.
func NewInMemoryPasswordResetStore() *InMemoryPasswordResetStore {
	return &InMemoryPasswordResetStore{
		tokens: make(map[string]*PasswordResetToken),
	}
}

// Create creates a new reset token.
func (s *InMemoryPasswordResetStore) Create(ctx context.Context, token *PasswordResetToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token.Token] = token
	return nil
}

// GetByToken retrieves a reset token by token string.
func (s *InMemoryPasswordResetStore) GetByToken(ctx context.Context, token string) (*PasswordResetToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tokens[token]
	if !ok {
		return nil, ErrResetTokenNotFound
	}
	return t, nil
}

// MarkUsed marks a token as used.
func (s *InMemoryPasswordResetStore) MarkUsed(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tokens {
		if t.ID == id {
			now := timeutil.NowTime()
			t.UsedAt = &now
			return nil
		}
	}
	return ErrResetTokenNotFound
}

// CountRecentByUser counts recent tokens for a user.
func (s *InMemoryPasswordResetStore) CountRecentByUser(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, t := range s.tokens {
		if t.UserID == userID && t.CreatedAt.After(since) {
			count++
		}
	}
	return count, nil
}

// DeleteExpired removes expired tokens.
func (s *InMemoryPasswordResetStore) DeleteExpired(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := timeutil.NowTime()
	for token, t := range s.tokens {
		if now.After(t.ExpiresAt) {
			delete(s.tokens, token)
		}
	}
	return nil
}

// PasswordResetService handles password reset operations.
type PasswordResetService struct {
	store  PasswordResetStore
	config *PasswordResetConfig
}

// NewPasswordResetService creates a new password reset service.
func NewPasswordResetService(store PasswordResetStore, config *PasswordResetConfig) *PasswordResetService {
	if config == nil {
		config = DefaultPasswordResetConfig()
	}
	return &PasswordResetService{
		store:  store,
		config: config,
	}
}

// RequestReset creates a new password reset token for a user.
func (s *PasswordResetService) RequestReset(ctx context.Context, userID uuid.UUID) (*PasswordResetToken, error) {
	// Check rate limit
	since := timeutil.NowTime().Add(-time.Hour)
	count, err := s.store.CountRecentByUser(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	if count >= s.config.MaxRequestsPerHour {
		return nil, ErrTooManyResetRequests
	}

	// Generate token
	tokenBytes := make([]byte, s.config.TokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	tokenStr := base64.URLEncoding.EncodeToString(tokenBytes)

	token := &PasswordResetToken{
		ID:        uuid.New(),
		UserID:    userID,
		Token:     tokenStr,
		ExpiresAt: timeutil.NowTime().Add(s.config.TokenTTL),
		CreatedAt: timeutil.NowTime(),
	}

	if err := s.store.Create(ctx, token); err != nil {
		return nil, err
	}

	return token, nil
}

// ValidateToken validates a reset token and returns the associated user ID.
func (s *PasswordResetService) ValidateToken(ctx context.Context, tokenStr string) (uuid.UUID, error) {
	token, err := s.store.GetByToken(ctx, tokenStr)
	if err != nil {
		return uuid.Nil, err
	}

	if token.UsedAt != nil {
		return uuid.Nil, ErrResetTokenUsed
	}

	if timeutil.NowTime().After(token.ExpiresAt) {
		return uuid.Nil, ErrResetTokenExpired
	}

	return token.UserID, nil
}

// ConsumeToken marks a token as used.
func (s *PasswordResetService) ConsumeToken(ctx context.Context, tokenStr string) error {
	token, err := s.store.GetByToken(ctx, tokenStr)
	if err != nil {
		return err
	}

	if token.UsedAt != nil {
		return ErrResetTokenUsed
	}

	if timeutil.NowTime().After(token.ExpiresAt) {
		return ErrResetTokenExpired
	}

	return s.store.MarkUsed(ctx, token.ID)
}

// Cleanup removes expired tokens.
func (s *PasswordResetService) Cleanup(ctx context.Context) error {
	return s.store.DeleteExpired(ctx)
}

// PasswordResetRequest represents a request to initiate password reset.
type PasswordResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// PasswordResetConfirmRequest represents a request to confirm password reset.
type PasswordResetConfirmRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required"`
}

// PasswordResetResponse represents the response for password reset initiation.
type PasswordResetResponse struct {
	Message string `json:"message"`
}
