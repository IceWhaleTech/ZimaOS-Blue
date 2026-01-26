package oidc

import (
	"testing"
)

func TestClientStore(t *testing.T) {
	store := NewClientStore()

	// Test Register and Get
	client := &Client{
		ID:                "test-client",
		Secret:            "test-secret",
		RedirectURIs:      []string{"http://localhost:3000/callback"},
		AllowedScopes:     []string{ScopeOpenID, ScopeProfile},
		AllowedGrantTypes: []string{GrantTypeAuthorizationCode},
		Public:            false,
	}

	store.Register(client)

	retrieved, err := store.Get("test-client")
	if err != nil {
		t.Fatalf("failed to get client: %v", err)
	}

	if retrieved.ID != client.ID {
		t.Errorf("expected client ID %s, got %s", client.ID, retrieved.ID)
	}

	// Test Get non-existent client
	_, err = store.Get("non-existent")
	if err != ErrClientNotFound {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}
}

func TestClientStoreAuthenticate(t *testing.T) {
	store := NewClientStore()

	// Register confidential client
	confidentialClient := &Client{
		ID:     "confidential",
		Secret: "secret123",
		Public: false,
	}
	store.Register(confidentialClient)

	// Register public client
	publicClient := &Client{
		ID:     "public",
		Secret: "",
		Public: true,
	}
	store.Register(publicClient)

	// Test confidential client authentication
	_, err := store.Authenticate("confidential", "secret123")
	if err != nil {
		t.Errorf("failed to authenticate confidential client: %v", err)
	}

	// Test wrong secret
	_, err = store.Authenticate("confidential", "wrong-secret")
	if err != ErrInvalidClientSecret {
		t.Errorf("expected ErrInvalidClientSecret, got %v", err)
	}

	// Test public client authentication (no secret needed)
	_, err = store.Authenticate("public", "")
	if err != nil {
		t.Errorf("failed to authenticate public client: %v", err)
	}
}

func TestClientStoreValidateRedirectURI(t *testing.T) {
	store := NewClientStore()

	client := &Client{
		ID:           "test-client",
		RedirectURIs: []string{"http://localhost:3000/callback", "https://example.com/callback"},
	}
	store.Register(client)

	// Test valid redirect URI
	err := store.ValidateRedirectURI("test-client", "http://localhost:3000/callback")
	if err != nil {
		t.Errorf("expected valid redirect URI, got error: %v", err)
	}

	// Test invalid redirect URI
	err = store.ValidateRedirectURI("test-client", "http://evil.com/callback")
	if err != ErrInvalidRedirectURI {
		t.Errorf("expected ErrInvalidRedirectURI, got %v", err)
	}
}

func TestClientStoreValidateScopes(t *testing.T) {
	store := NewClientStore()

	client := &Client{
		ID:            "test-client",
		AllowedScopes: []string{ScopeOpenID, ScopeProfile},
	}
	store.Register(client)

	// Test valid scopes
	err := store.ValidateScopes("test-client", []string{ScopeOpenID})
	if err != nil {
		t.Errorf("expected valid scopes, got error: %v", err)
	}

	// Test invalid scope
	err = store.ValidateScopes("test-client", []string{ScopeEmail})
	if err != ErrInvalidScope {
		t.Errorf("expected ErrInvalidScope, got %v", err)
	}
}

func TestClientStoreValidateGrantType(t *testing.T) {
	store := NewClientStore()

	client := &Client{
		ID:                "test-client",
		AllowedGrantTypes: []string{GrantTypeAuthorizationCode},
	}
	store.Register(client)

	// Test valid grant type
	err := store.ValidateGrantType("test-client", GrantTypeAuthorizationCode)
	if err != nil {
		t.Errorf("expected valid grant type, got error: %v", err)
	}

	// Test invalid grant type
	err = store.ValidateGrantType("test-client", GrantTypeRefreshToken)
	if err != ErrInvalidGrantType {
		t.Errorf("expected ErrInvalidGrantType, got %v", err)
	}
}

func TestClientStoreFromConfig(t *testing.T) {
	configs := []ClientConfig{
		{
			ClientID:          "client1",
			ClientSecret:      "secret1",
			RedirectURIs:      []string{"http://localhost:3000/callback"},
			AllowedScopes:     []string{ScopeOpenID},
			AllowedGrantTypes: []string{GrantTypeAuthorizationCode},
			Public:            false,
		},
		{
			ClientID:          "client2",
			ClientSecret:      "",
			RedirectURIs:      []string{"http://localhost:4000/callback"},
			AllowedScopes:     []string{ScopeOpenID, ScopeProfile},
			AllowedGrantTypes: []string{GrantTypeAuthorizationCode, GrantTypeRefreshToken},
			Public:            true,
		},
	}

	store := NewClientStoreFromConfig(configs)

	// Verify both clients are registered
	client1, err := store.Get("client1")
	if err != nil {
		t.Fatalf("failed to get client1: %v", err)
	}
	if client1.Public {
		t.Error("client1 should not be public")
	}

	client2, err := store.Get("client2")
	if err != nil {
		t.Fatalf("failed to get client2: %v", err)
	}
	if !client2.Public {
		t.Error("client2 should be public")
	}
}

func TestClientStoreDelete(t *testing.T) {
	store := NewClientStore()

	client := &Client{ID: "test-client"}
	store.Register(client)

	// Delete client
	err := store.Delete("test-client")
	if err != nil {
		t.Errorf("failed to delete client: %v", err)
	}

	// Verify client is deleted
	_, err = store.Get("test-client")
	if err != ErrClientNotFound {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}

	// Delete non-existent client
	err = store.Delete("non-existent")
	if err != ErrClientNotFound {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}
}

func TestClientStoreList(t *testing.T) {
	store := NewClientStore()

	store.Register(&Client{ID: "client1"})
	store.Register(&Client{ID: "client2"})
	store.Register(&Client{ID: "client3"})

	clients := store.List()
	if len(clients) != 3 {
		t.Errorf("expected 3 clients, got %d", len(clients))
	}
}
