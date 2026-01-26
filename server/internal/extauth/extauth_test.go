package extauth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultClaimMapping(t *testing.T) {
	mapping := DefaultClaimMapping()

	assert.Equal(t, "sub", mapping.Subject)
	assert.Equal(t, "email", mapping.Email)
	assert.Equal(t, "name", mapping.Name)
	assert.Equal(t, "picture", mapping.Picture)
	assert.Equal(t, "groups", mapping.Groups)
	assert.Equal(t, "roles", mapping.Roles)
}

func TestNewProviderClient(t *testing.T) {
	t.Run("nil config returns error", func(t *testing.T) {
		_, err := NewProviderClient(nil)
		assert.Error(t, err)
	})

	t.Run("valid config creates client", func(t *testing.T) {
		config := &ProviderConfig{
			ID:           "test",
			Name:         "Test Provider",
			Type:         ProviderGeneric,
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			IssuerURL:    "https://example.com",
			RedirectURL:  "https://app.example.com/callback",
		}

		client, err := NewProviderClient(config)
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, config, client.Config())
	})
}

func TestProviderClient_DefaultScopes(t *testing.T) {
	tests := []struct {
		providerType ProviderType
		expected     []string
	}{
		{ProviderGitHub, []string{"user:email", "read:user"}},
		{ProviderGoogle, []string{"openid", "email", "profile"}},
		{ProviderMicrosoft, []string{"openid", "email", "profile", "offline_access"}},
		{ProviderGeneric, []string{"openid", "email", "profile"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			client, _ := NewProviderClient(&ProviderConfig{
				ID:          "test",
				Type:        tt.providerType,
				ClientID:    "client-id",
				RedirectURL: "https://example.com/callback",
			})

			scopes := client.defaultScopes()
			assert.Equal(t, tt.expected, scopes)
		})
	}
}

func TestMemoryStateStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStateStore()

	t.Run("save and get state", func(t *testing.T) {
		state := &AuthState{
			State:      "test-state",
			Nonce:      "test-nonce",
			ProviderID: "google",
			CreatedAt:  time.Now(),
			ExpiresAt:  time.Now().Add(10 * time.Minute),
		}

		err := store.Save(ctx, state)
		require.NoError(t, err)

		retrieved, err := store.Get(ctx, "test-state")
		require.NoError(t, err)
		assert.Equal(t, state.State, retrieved.State)
		assert.Equal(t, state.Nonce, retrieved.Nonce)
		assert.Equal(t, state.ProviderID, retrieved.ProviderID)
	})

	t.Run("get non-existent state returns error", func(t *testing.T) {
		_, err := store.Get(ctx, "non-existent")
		assert.Equal(t, ErrInvalidState, err)
	})

	t.Run("delete state", func(t *testing.T) {
		state := &AuthState{
			State:     "to-delete",
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}

		err := store.Save(ctx, state)
		require.NoError(t, err)

		err = store.Delete(ctx, "to-delete")
		require.NoError(t, err)

		_, err = store.Get(ctx, "to-delete")
		assert.Equal(t, ErrInvalidState, err)
	})

	t.Run("cleanup expired states", func(t *testing.T) {
		// Add expired state
		expiredState := &AuthState{
			State:     "expired",
			ExpiresAt: time.Now().Add(-1 * time.Minute),
		}
		store.Save(ctx, expiredState)

		// Add valid state
		validState := &AuthState{
			State:     "valid",
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		store.Save(ctx, validState)

		err := store.Cleanup(ctx)
		require.NoError(t, err)

		// Expired should be gone
		_, err = store.Get(ctx, "expired")
		assert.Equal(t, ErrInvalidState, err)

		// Valid should still exist
		_, err = store.Get(ctx, "valid")
		assert.NoError(t, err)
	})
}

func TestMemoryAccountStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryAccountStore()

	t.Run("save and get account", func(t *testing.T) {
		account := &LinkedAccount{
			ID:             "acc-1",
			UserID:         "user-1",
			ProviderID:     "google",
			ProviderUserID: "google-user-123",
			Email:          "test@example.com",
		}

		err := store.Save(ctx, account)
		require.NoError(t, err)

		retrieved, err := store.GetByProviderUser(ctx, "google", "google-user-123")
		require.NoError(t, err)
		assert.Equal(t, account.ID, retrieved.ID)
		assert.Equal(t, account.UserID, retrieved.UserID)
		assert.Equal(t, account.Email, retrieved.Email)
	})

	t.Run("get accounts by user", func(t *testing.T) {
		// Add another account for the same user
		account2 := &LinkedAccount{
			ID:             "acc-2",
			UserID:         "user-1",
			ProviderID:     "github",
			ProviderUserID: "github-user-456",
		}
		store.Save(ctx, account2)

		accounts, err := store.GetByUser(ctx, "user-1")
		require.NoError(t, err)
		assert.Len(t, accounts, 2)
	})

	t.Run("prevent linking to different user", func(t *testing.T) {
		account := &LinkedAccount{
			ID:             "acc-3",
			UserID:         "user-2", // Different user
			ProviderID:     "google",
			ProviderUserID: "google-user-123", // Same provider user
		}

		err := store.Save(ctx, account)
		assert.Equal(t, ErrAccountAlreadyLinked, err)
	})

	t.Run("delete account", func(t *testing.T) {
		err := store.Delete(ctx, "user-1", "github")
		require.NoError(t, err)

		accounts, err := store.GetByUser(ctx, "user-1")
		require.NoError(t, err)
		assert.Len(t, accounts, 1)
	})
}

func TestIsEmailDomainAllowed(t *testing.T) {
	tests := []struct {
		email          string
		allowedDomains []string
		expected       bool
	}{
		{"user@example.com", []string{"example.com"}, true},
		{"user@example.com", []string{"other.com"}, false},
		{"user@example.com", []string{"Example.COM"}, true}, // case insensitive
		{"user@sub.example.com", []string{"example.com"}, false},
		{"invalid-email", []string{"example.com"}, false},
		{"user@example.com", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := isEmailDomainAllowed(tt.email, tt.allowedDomains)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateRandomString(t *testing.T) {
	s1, err := generateRandomString(32)
	require.NoError(t, err)
	assert.NotEmpty(t, s1)

	s2, err := generateRandomString(32)
	require.NoError(t, err)
	assert.NotEmpty(t, s2)

	// Should be different
	assert.NotEqual(t, s1, s2)
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "test-verifier"
	challenge := generateCodeChallenge(verifier)

	assert.NotEmpty(t, challenge)
	assert.NotEqual(t, verifier, challenge)

	// Same verifier should produce same challenge
	challenge2 := generateCodeChallenge(verifier)
	assert.Equal(t, challenge, challenge2)
}

func TestMapError(t *testing.T) {
	tests := []struct {
		err            error
		expectedStatus int
	}{
		{ErrProviderNotFound, 404},
		{ErrProviderDisabled, 403},
		{ErrInvalidState, 400},
		{ErrStateExpired, 400},
		{ErrInvalidCode, 401},
		{ErrInvalidToken, 401},
		{ErrUserNotFound, 404},
		{ErrAccountAlreadyLinked, 409},
		{ErrEmailNotAllowed, 403},
		{ErrDiscoveryFailed, 502},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			httpErr := mapError(tt.err)
			assert.Equal(t, tt.expectedStatus, httpErr.Code)
		})
	}
}

func TestProviderTypes(t *testing.T) {
	assert.Equal(t, ProviderType("generic"), ProviderGeneric)
	assert.Equal(t, ProviderType("google"), ProviderGoogle)
	assert.Equal(t, ProviderType("github"), ProviderGitHub)
	assert.Equal(t, ProviderType("microsoft"), ProviderMicrosoft)
	assert.Equal(t, ProviderType("keycloak"), ProviderKeycloak)
	assert.Equal(t, ProviderType("authentik"), ProviderAuthentik)
	assert.Equal(t, ProviderType("auth0"), ProviderAuth0)
}
