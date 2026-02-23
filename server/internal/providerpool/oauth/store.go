// Package oauth provides OAuth token management for LLM providers.
package oauth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Token represents a stored OAuth token with metadata.
type Token struct {
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

// Store persists OAuth tokens to disk.
type Store struct {
	path string
	mu   sync.RWMutex
}

// NewStore creates a new file-based OAuth token store.
func NewStore(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "providers")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create oauth store dir: %w", err)
	}
	return &Store{path: filepath.Join(dir, "oauth_tokens.json")}, nil
}

type storeData struct {
	Tokens    map[string]*Token `json:"tokens"`
	UpdatedAt time.Time         `json:"updated_at"`
}

func (s *Store) load() (*storeData, error) {
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

func (s *Store) save(sd *storeData) error {
	sd.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

// SaveToken persists a token for the given provider.
func (s *Store) SaveToken(providerID string, token *Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sd, err := s.load()
	if err != nil {
		return err
	}
	token.ProviderID = providerID
	token.UpdatedAt = time.Now()
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now()
	}
	sd.Tokens[providerID] = token
	return s.save(sd)
}

// LoadToken retrieves a token for the given provider.
func (s *Store) LoadToken(providerID string) (*Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sd, err := s.load()
	if err != nil {
		return nil, err
	}
	token, ok := sd.Tokens[providerID]
	if !ok {
		return nil, fmt.Errorf("no oauth token for provider %s", providerID)
	}
	return token, nil
}

// DeleteToken removes a token for the given provider.
func (s *Store) DeleteToken(providerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sd, err := s.load()
	if err != nil {
		return err
	}
	delete(sd.Tokens, providerID)
	return s.save(sd)
}

// ListTokens returns all stored tokens.
func (s *Store) ListTokens() (map[string]*Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sd, err := s.load()
	if err != nil {
		return nil, err
	}
	return sd.Tokens, nil
}
