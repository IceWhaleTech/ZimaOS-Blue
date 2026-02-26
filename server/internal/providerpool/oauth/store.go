// Package oauth provides OAuth token management for LLM providers.
package oauth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Token represents a stored OAuth token with metadata.
type Token struct {
	ID           string    `json:"id"`
	ProviderID   string    `json:"provider_id"`
	ProviderType string    `json:"provider_type"` // "antigravity", "gemini-cli", "copilot"
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenExpiry  time.Time `json:"token_expiry"`
	Scopes       []string  `json:"scopes,omitempty"`
	Email        string    `json:"email,omitempty"`
	ProjectID    string    `json:"project_id,omitempty"` // Google Cloud Code project
	Endpoint     string    `json:"endpoint,omitempty"`   // API endpoint URL
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Expired returns true if the access token has expired (with 60s buffer).
func (t *Token) Expired() bool {
	return time.Now().After(t.TokenExpiry.Add(-60 * time.Second))
}

// TokenStore is the interface for OAuth token persistence.
type TokenStore interface {
	SaveToken(providerID string, token *Token) error
	LoadToken(providerID, tokenID string) (*Token, error)
	LoadTokenByEmail(providerID, email string) (*Token, error)
	LoadTokens(providerID string) ([]*Token, error)
	DeleteToken(providerID, tokenID string) error
	ListTokens() (map[string]*Token, error)
}

// FileStore persists OAuth tokens to a JSON file on disk.
type FileStore struct {
	path string
	mu   sync.RWMutex
}

// NewFileStore creates a new file-based OAuth token store.
func NewFileStore(dataDir string) (*FileStore, error) {
	dir := filepath.Join(dataDir, "providers")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create oauth store dir: %w", err)
	}
	return &FileStore{path: filepath.Join(dir, "oauth_tokens.json")}, nil
}

type storeData struct {
	Tokens    map[string]*Token `json:"tokens"`
	UpdatedAt time.Time         `json:"updated_at"`
}

func (s *FileStore) load() (*storeData, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &storeData{Tokens: make(map[string]*Token)}, nil
		}
		return nil, err
	}
	var sd storeData
	if err := json.Unmarshal(data, &sd); err != nil {
		return nil, err
	}
	if sd.Tokens == nil {
		sd.Tokens = make(map[string]*Token)
	}
	return &sd, nil
}

func (s *FileStore) save(sd *storeData) error {
	sd.UpdatedAt = timeutil.NowTime()
	data, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

// SaveToken persists a token for the given provider.
func (s *FileStore) SaveToken(providerID string, token *Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sd, err := s.load()
	if err != nil {
		return err
	}
	token.ProviderID = providerID
	token.UpdatedAt = timeutil.NowTime()
	if token.CreatedAt.IsZero() {
		token.CreatedAt = timeutil.NowTime()
	}
	key := providerID + ":" + token.ID
	sd.Tokens[key] = token
	return s.save(sd)
}

// LoadToken retrieves a specific token for the given provider.
func (s *FileStore) LoadToken(providerID, tokenID string) (*Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sd, err := s.load()
	if err != nil {
		return nil, err
	}
	key := providerID + ":" + tokenID
	token, ok := sd.Tokens[key]
	if !ok {
		// Fallback: try legacy key (providerID only)
		token, ok = sd.Tokens[providerID]
		if !ok {
			return nil, fmt.Errorf("no oauth token for provider %s token %s", providerID, tokenID)
		}
	}
	return token, nil
}

// LoadTokenByEmail loads an OAuth token by email address for a given provider.
// This is used to find existing tokens when re-authenticating to avoid duplicates.
func (s *FileStore) LoadTokenByEmail(providerID, email string) (*Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sd, err := s.load()
	if err != nil {
		return nil, err
	}
	for _, token := range sd.Tokens {
		if token.ProviderID == providerID && token.Email == email {
			return token, nil
		}
	}
	return nil, nil
}

// LoadTokens retrieves all tokens for the given provider.
func (s *FileStore) LoadTokens(providerID string) ([]*Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sd, err := s.load()
	if err != nil {
		return nil, err
	}
	var tokens []*Token
	for _, token := range sd.Tokens {
		if token.ProviderID == providerID {
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}

// DeleteToken removes a specific token for the given provider.
func (s *FileStore) DeleteToken(providerID, tokenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sd, err := s.load()
	if err != nil {
		return err
	}
	key := providerID + ":" + tokenID
	delete(sd.Tokens, key)
	// Also try legacy key
	delete(sd.Tokens, providerID)
	return s.save(sd)
}

// ListTokens returns all stored tokens.
func (s *FileStore) ListTokens() (map[string]*Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sd, err := s.load()
	if err != nil {
		return nil, err
	}
	return sd.Tokens, nil
}
