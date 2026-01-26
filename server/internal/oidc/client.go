package oidc

import (
	"errors"
	"sync"
)

var (
	// ErrClientNotFound is returned when a client is not found.
	ErrClientNotFound = errors.New("client not found")
	// ErrInvalidClientSecret is returned when the client secret is invalid.
	ErrInvalidClientSecret = errors.New("invalid client secret")
	// ErrInvalidRedirectURI is returned when the redirect URI is not allowed.
	ErrInvalidRedirectURI = errors.New("invalid redirect URI")
	// ErrInvalidScope is returned when a scope is not allowed.
	ErrInvalidScope = errors.New("invalid scope")
	// ErrInvalidGrantType is returned when a grant type is not allowed.
	ErrInvalidGrantType = errors.New("invalid grant type")
)

// Client represents an OIDC client.
type Client struct {
	ID                string
	Secret            string
	RedirectURIs      []string
	AllowedScopes     []string
	AllowedGrantTypes []string
	Public            bool
}

// ClientStore manages OIDC clients.
type ClientStore struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

// NewClientStore creates a new client store.
func NewClientStore() *ClientStore {
	return &ClientStore{
		clients: make(map[string]*Client),
	}
}

// NewClientStoreFromConfig creates a client store from configuration.
func NewClientStoreFromConfig(configs []ClientConfig) *ClientStore {
	store := NewClientStore()
	for _, cfg := range configs {
		client := &Client{
			ID:                cfg.ClientID,
			Secret:            cfg.ClientSecret,
			RedirectURIs:      cfg.RedirectURIs,
			AllowedScopes:     cfg.AllowedScopes,
			AllowedGrantTypes: cfg.AllowedGrantTypes,
			Public:            cfg.Public,
		}
		store.Register(client)
	}
	return store
}

// Register registers a new client.
func (s *ClientStore) Register(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client.ID] = client
}

// Get retrieves a client by ID.
func (s *ClientStore) Get(clientID string) (*Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[clientID]
	if !ok {
		return nil, ErrClientNotFound
	}

	return client, nil
}

// Authenticate authenticates a client.
func (s *ClientStore) Authenticate(clientID, clientSecret string) (*Client, error) {
	client, err := s.Get(clientID)
	if err != nil {
		return nil, err
	}

	// Public clients don't need a secret
	if client.Public {
		return client, nil
	}

	// Verify secret
	if client.Secret != clientSecret {
		return nil, ErrInvalidClientSecret
	}

	return client, nil
}

// ValidateRedirectURI validates a redirect URI for a client.
func (s *ClientStore) ValidateRedirectURI(clientID, redirectURI string) error {
	client, err := s.Get(clientID)
	if err != nil {
		return err
	}

	for _, uri := range client.RedirectURIs {
		if uri == redirectURI {
			return nil
		}
	}

	return ErrInvalidRedirectURI
}

// ValidateScopes validates scopes for a client.
func (s *ClientStore) ValidateScopes(clientID string, scopes []string) error {
	client, err := s.Get(clientID)
	if err != nil {
		return err
	}

	allowedMap := make(map[string]bool)
	for _, scope := range client.AllowedScopes {
		allowedMap[scope] = true
	}

	for _, scope := range scopes {
		if !allowedMap[scope] {
			return ErrInvalidScope
		}
	}

	return nil
}

// ValidateGrantType validates a grant type for a client.
func (s *ClientStore) ValidateGrantType(clientID, grantType string) error {
	client, err := s.Get(clientID)
	if err != nil {
		return err
	}

	for _, gt := range client.AllowedGrantTypes {
		if gt == grantType {
			return nil
		}
	}

	return ErrInvalidGrantType
}

// List returns all registered clients.
func (s *ClientStore) List() []*Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make([]*Client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}

	return clients
}

// Delete removes a client.
func (s *ClientStore) Delete(clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.clients[clientID]; !ok {
		return ErrClientNotFound
	}

	delete(s.clients, clientID)
	return nil
}
