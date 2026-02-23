package extauth

import (
	"context"
	"sync"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// MemoryStateStore is an in-memory implementation of StateStore.
type MemoryStateStore struct {
	states map[string]*AuthState
	mu     sync.RWMutex
}

// NewMemoryStateStore creates a new in-memory state store.
func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{
		states: make(map[string]*AuthState),
	}
}

// Save saves an auth state.
func (s *MemoryStateStore) Save(ctx context.Context, state *AuthState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[state.State] = state
	return nil
}

// Get retrieves an auth state by state parameter.
func (s *MemoryStateStore) Get(ctx context.Context, state string) (*AuthState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	authState, ok := s.states[state]
	if !ok {
		return nil, ErrInvalidState
	}

	return authState, nil
}

// Delete deletes an auth state.
func (s *MemoryStateStore) Delete(ctx context.Context, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, state)
	return nil
}

// Cleanup removes expired states.
func (s *MemoryStateStore) Cleanup(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := timeutil.NowTime()
	for state, authState := range s.states {
		if now.After(authState.ExpiresAt) {
			delete(s.states, state)
		}
	}

	return nil
}

// MemoryAccountStore is an in-memory implementation of AccountStore.
type MemoryAccountStore struct {
	accounts map[string]*LinkedAccount // key: providerID:providerUserID
	byUser   map[string][]string       // userID -> list of keys
	mu       sync.RWMutex
}

// NewMemoryAccountStore creates a new in-memory account store.
func NewMemoryAccountStore() *MemoryAccountStore {
	return &MemoryAccountStore{
		accounts: make(map[string]*LinkedAccount),
		byUser:   make(map[string][]string),
	}
}

// Save saves a linked account.
func (s *MemoryAccountStore) Save(ctx context.Context, account *LinkedAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := account.ProviderID + ":" + account.ProviderUserID

	// Check if already linked to another user
	if existing, ok := s.accounts[key]; ok && existing.UserID != account.UserID {
		return ErrAccountAlreadyLinked
	}

	s.accounts[key] = account

	// Update user index
	found := false
	for _, k := range s.byUser[account.UserID] {
		if k == key {
			found = true
			break
		}
	}
	if !found {
		s.byUser[account.UserID] = append(s.byUser[account.UserID], key)
	}

	return nil
}

// GetByProviderUser gets a linked account by provider and provider user ID.
func (s *MemoryAccountStore) GetByProviderUser(ctx context.Context, providerID, providerUserID string) (*LinkedAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := providerID + ":" + providerUserID
	account, ok := s.accounts[key]
	if !ok {
		return nil, ErrUserNotFound
	}

	return account, nil
}

// GetByUser gets all linked accounts for a user.
func (s *MemoryAccountStore) GetByUser(ctx context.Context, userID string) ([]*LinkedAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := s.byUser[userID]
	accounts := make([]*LinkedAccount, 0, len(keys))
	for _, key := range keys {
		if account, ok := s.accounts[key]; ok {
			accounts = append(accounts, account)
		}
	}

	return accounts, nil
}

// Delete deletes a linked account.
func (s *MemoryAccountStore) Delete(ctx context.Context, userID, providerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find and delete the account
	var keyToDelete string
	for key, account := range s.accounts {
		if account.UserID == userID && account.ProviderID == providerID {
			keyToDelete = key
			break
		}
	}

	if keyToDelete == "" {
		return nil
	}

	delete(s.accounts, keyToDelete)

	// Update user index
	keys := s.byUser[userID]
	newKeys := make([]string, 0, len(keys)-1)
	for _, k := range keys {
		if k != keyToDelete {
			newKeys = append(newKeys, k)
		}
	}
	s.byUser[userID] = newKeys

	return nil
}
