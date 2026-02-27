package oauth

import (
	"testing"
	"time"
)

type mockTokenStore struct {
	tokens  []*Token
	deleted []string
}

func (m *mockTokenStore) SaveToken(providerID string, token *Token) error { return nil }
func (m *mockTokenStore) LoadToken(providerID, tokenID string) (*Token, error) {
	return nil, nil
}
func (m *mockTokenStore) LoadTokenByEmail(providerID, email string) (*Token, error) {
	return nil, nil
}
func (m *mockTokenStore) LoadTokens(providerID string) ([]*Token, error) {
	return m.tokens, nil
}
func (m *mockTokenStore) DeleteToken(providerID, tokenID string) error {
	m.deleted = append(m.deleted, providerID+":"+tokenID)
	return nil
}
func (m *mockTokenStore) ListTokens() (map[string]*Token, error) { return nil, nil }

func TestTokenExpired_ZeroExpiryIsNotExpired(t *testing.T) {
	tok := &Token{}
	if tok.Expired() {
		t.Fatal("zero expiry should be treated as not expired")
	}
}

func TestGetAccessToken_PrunesExpiredTokenWithoutRefresh(t *testing.T) {
	store := &mockTokenStore{
		tokens: []*Token{
			{
				ID:          "tok-1",
				ProviderID:  "github-copilot",
				TokenExpiry: time.Now().Add(-10 * time.Minute),
			},
		},
	}
	m := NewManager(store)

	_, err := m.GetAccessToken("github-copilot")
	if err == nil {
		t.Fatal("expected error for expired token without refresh token")
	}
	if len(store.deleted) != 1 {
		t.Fatalf("expected token to be deleted, got %d deletions", len(store.deleted))
	}
	if store.deleted[0] != "github-copilot:tok-1" {
		t.Fatalf("unexpected deleted token key: %s", store.deleted[0])
	}
}
