package mfa

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

var (
	// ErrCredentialExists is returned when a credential with the same ID already exists.
	ErrCredentialExists = errors.New("credential already exists")
	// ErrMaxCredentialsReached is returned when the maximum number of credentials is reached.
	ErrMaxCredentialsReached = errors.New("maximum credentials reached")
)

// CredentialStore defines the interface for storing WebAuthn credentials.
type CredentialStore interface {
	// Create creates a new credential.
	Create(ctx context.Context, userID uuid.UUID, cred *WebAuthnCredential) error
	// GetByUser retrieves all credentials for a user.
	GetByUser(ctx context.Context, userID uuid.UUID) ([]WebAuthnCredential, error)
	// GetByID retrieves a credential by ID.
	GetByID(ctx context.Context, userID uuid.UUID, credID []byte) (*WebAuthnCredential, error)
	// Update updates a credential.
	Update(ctx context.Context, userID uuid.UUID, cred *WebAuthnCredential) error
	// Delete deletes a credential.
	Delete(ctx context.Context, userID uuid.UUID, credID []byte) error
	// Count returns the number of credentials for a user.
	Count(ctx context.Context, userID uuid.UUID) (int, error)
}

// InMemoryCredentialStore is an in-memory implementation of CredentialStore.
type InMemoryCredentialStore struct {
	mu          sync.RWMutex
	credentials map[uuid.UUID][]WebAuthnCredential
}

// NewInMemoryCredentialStore creates a new in-memory credential store.
func NewInMemoryCredentialStore() *InMemoryCredentialStore {
	return &InMemoryCredentialStore{
		credentials: make(map[uuid.UUID][]WebAuthnCredential),
	}
}

// Create creates a new credential.
func (s *InMemoryCredentialStore) Create(ctx context.Context, userID uuid.UUID, cred *WebAuthnCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	creds := s.credentials[userID]

	// Check for duplicate
	for _, c := range creds {
		if base64.StdEncoding.EncodeToString(c.ID) == base64.StdEncoding.EncodeToString(cred.ID) {
			return ErrCredentialExists
		}
	}

	s.credentials[userID] = append(creds, *cred)
	return nil
}

// GetByUser retrieves all credentials for a user.
func (s *InMemoryCredentialStore) GetByUser(ctx context.Context, userID uuid.UUID) ([]WebAuthnCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	creds := s.credentials[userID]
	if creds == nil {
		return []WebAuthnCredential{}, nil
	}

	// Return a copy
	result := make([]WebAuthnCredential, len(creds))
	copy(result, creds)
	return result, nil
}

// GetByID retrieves a credential by ID.
func (s *InMemoryCredentialStore) GetByID(ctx context.Context, userID uuid.UUID, credID []byte) (*WebAuthnCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	creds := s.credentials[userID]
	credIDStr := base64.StdEncoding.EncodeToString(credID)

	for _, c := range creds {
		if base64.StdEncoding.EncodeToString(c.ID) == credIDStr {
			return &c, nil
		}
	}

	return nil, ErrCredentialNotFound
}

// Update updates a credential.
func (s *InMemoryCredentialStore) Update(ctx context.Context, userID uuid.UUID, cred *WebAuthnCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	creds := s.credentials[userID]
	credIDStr := base64.StdEncoding.EncodeToString(cred.ID)

	for i, c := range creds {
		if base64.StdEncoding.EncodeToString(c.ID) == credIDStr {
			s.credentials[userID][i] = *cred
			return nil
		}
	}

	return ErrCredentialNotFound
}

// Delete deletes a credential.
func (s *InMemoryCredentialStore) Delete(ctx context.Context, userID uuid.UUID, credID []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	creds := s.credentials[userID]
	credIDStr := base64.StdEncoding.EncodeToString(credID)

	for i, c := range creds {
		if base64.StdEncoding.EncodeToString(c.ID) == credIDStr {
			s.credentials[userID] = append(creds[:i], creds[i+1:]...)
			return nil
		}
	}

	return ErrCredentialNotFound
}

// Count returns the number of credentials for a user.
func (s *InMemoryCredentialStore) Count(ctx context.Context, userID uuid.UUID) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.credentials[userID]), nil
}

// CredentialManagerConfig holds configuration for the credential manager.
type CredentialManagerConfig struct {
	// MaxCredentialsPerUser is the maximum number of credentials per user.
	MaxCredentialsPerUser int
}

// DefaultCredentialManagerConfig returns the default configuration.
func DefaultCredentialManagerConfig() *CredentialManagerConfig {
	return &CredentialManagerConfig{
		MaxCredentialsPerUser: 10,
	}
}

// CredentialManager manages WebAuthn credentials.
type CredentialManager struct {
	store  CredentialStore
	config *CredentialManagerConfig
}

// NewCredentialManager creates a new credential manager.
func NewCredentialManager(store CredentialStore, config *CredentialManagerConfig) *CredentialManager {
	if config == nil {
		config = DefaultCredentialManagerConfig()
	}
	return &CredentialManager{
		store:  store,
		config: config,
	}
}

// AddCredential adds a new credential for a user.
func (m *CredentialManager) AddCredential(ctx context.Context, userID uuid.UUID, cred *WebAuthnCredential) error {
	// Check max credentials
	count, err := m.store.Count(ctx, userID)
	if err != nil {
		return err
	}
	if count >= m.config.MaxCredentialsPerUser {
		return ErrMaxCredentialsReached
	}

	return m.store.Create(ctx, userID, cred)
}

// ListCredentials returns all credentials for a user.
func (m *CredentialManager) ListCredentials(ctx context.Context, userID uuid.UUID) ([]CredentialInfo, error) {
	creds, err := m.store.GetByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]CredentialInfo, len(creds))
	for i, c := range creds {
		result[i] = CredentialInfo{
			ID:         base64.URLEncoding.EncodeToString(c.ID),
			Name:       c.Name,
			CreatedAt:  c.CreatedAt,
			LastUsedAt: c.LastUsedAt,
		}
	}

	return result, nil
}

// GetCredential retrieves a credential by ID.
func (m *CredentialManager) GetCredential(ctx context.Context, userID uuid.UUID, credID string) (*CredentialInfo, error) {
	credIDBytes, err := base64.URLEncoding.DecodeString(credID)
	if err != nil {
		return nil, err
	}

	cred, err := m.store.GetByID(ctx, userID, credIDBytes)
	if err != nil {
		return nil, err
	}

	return &CredentialInfo{
		ID:         base64.URLEncoding.EncodeToString(cred.ID),
		Name:       cred.Name,
		CreatedAt:  cred.CreatedAt,
		LastUsedAt: cred.LastUsedAt,
	}, nil
}

// UpdateCredentialName updates the name of a credential.
func (m *CredentialManager) UpdateCredentialName(ctx context.Context, userID uuid.UUID, credID, name string) error {
	credIDBytes, err := base64.URLEncoding.DecodeString(credID)
	if err != nil {
		return err
	}

	cred, err := m.store.GetByID(ctx, userID, credIDBytes)
	if err != nil {
		return err
	}

	cred.Name = name
	return m.store.Update(ctx, userID, cred)
}

// DeleteCredential deletes a credential.
func (m *CredentialManager) DeleteCredential(ctx context.Context, userID uuid.UUID, credID string) error {
	credIDBytes, err := base64.URLEncoding.DecodeString(credID)
	if err != nil {
		return err
	}

	return m.store.Delete(ctx, userID, credIDBytes)
}

// GetRawCredentials returns all raw WebAuthn credentials for a user.
// This is used internally for WebAuthn ceremonies.
func (m *CredentialManager) GetRawCredentials(ctx context.Context, userID uuid.UUID) ([]WebAuthnCredential, error) {
	return m.store.GetByUser(ctx, userID)
}

// UpdateLastUsed updates the last used time of a credential.
func (m *CredentialManager) UpdateLastUsed(ctx context.Context, userID uuid.UUID, credID []byte) error {
	cred, err := m.store.GetByID(ctx, userID, credID)
	if err != nil {
		return err
	}

	cred.LastUsedAt = timeutil.NowTime().UTC()
	return m.store.Update(ctx, userID, cred)
}

// CredentialInfo represents public information about a credential.
type CredentialInfo struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
}

// ListCredentialsResponse represents the response for listing credentials.
type ListCredentialsResponse struct {
	Credentials []CredentialInfo `json:"credentials"`
	Count       int              `json:"count"`
	MaxAllowed  int              `json:"max_allowed"`
}

// UpdateCredentialRequest represents a request to update a credential.
type UpdateCredentialRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

// DeleteCredentialRequest represents a request to delete a credential.
type DeleteCredentialRequest struct {
	Password string `json:"password" validate:"required"`
}
