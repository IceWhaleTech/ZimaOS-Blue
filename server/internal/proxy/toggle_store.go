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
	PrunerEnabled      bool            `json:"pruner_enabled"`
	RoutingEnabled     bool            `json:"routing_enabled"`
	MaskingEnabled     bool            `json:"masking_enabled"`
	PrunerBackend      string          `json:"pruner_backend,omitempty"`
	RoutingRules       map[string]bool `json:"routing_rules,omitempty"`
	PromptCacheEnabled bool            `json:"prompt_cache_enabled"`
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

// Load retrieves persisted toggle state. Returns nil if no state has been saved yet.
func (ts *ToggleStore) Load(ctx context.Context) (*ToggleState, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	var state ToggleState
	if err := ts.kv.GetJSON(ctx, toggleStoreKey, &state); err != nil {
		if err == kvstore.ErrKeyNotFound {
			return nil, nil // no saved state — caller should keep defaults
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
