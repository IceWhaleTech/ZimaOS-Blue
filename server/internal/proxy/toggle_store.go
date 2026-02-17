package proxy

import (
	"context"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
)

const toggleStoreKey = "proxy:feature_toggles"

// ToggleState holds persisted feature toggle states.
type ToggleState struct {
	CacheEnabled   bool `json:"cache_enabled"`
	PrunerEnabled  bool `json:"pruner_enabled"`
	RoutingEnabled bool `json:"routing_enabled"`
}

// ToggleStore persists feature toggle states via kvstore.
type ToggleStore struct {
	mu sync.Mutex
	kv kvstore.Store
}

// NewToggleStore creates a new toggle store.
func NewToggleStore(kv kvstore.Store) *ToggleStore {
	return &ToggleStore{kv: kv}
}

// Load retrieves persisted toggle state. Returns zero-value state if not found.
func (ts *ToggleStore) Load(ctx context.Context) (*ToggleState, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	var state ToggleState
	if err := ts.kv.GetJSON(ctx, toggleStoreKey, &state); err != nil {
		if err == kvstore.ErrKeyNotFound {
			return &state, nil
		}
		return nil, err
	}
	return &state, nil
}

// Save persists the current toggle state.
func (ts *ToggleStore) Save(ctx context.Context, state *ToggleState) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.kv.SetJSON(ctx, toggleStoreKey, state, 0*time.Second)
}
