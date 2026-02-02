package tunnel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// BlacklistDuration is how long a provider stays blacklisted
	BlacklistDuration = 24 * time.Hour
	// ImmediateFailureThreshold defines "immediate close" (< 5 seconds)
	ImmediateFailureThreshold = 5 * time.Second
)

// BlacklistEntry represents a blacklisted provider
type BlacklistEntry struct {
	Provider      Provider  `json:"provider"`
	BlacklistedAt time.Time `json:"blacklisted_at"`
	Reason        string    `json:"reason,omitempty"`
}

// Blacklist manages provider blacklisting
type Blacklist struct {
	mu      sync.RWMutex
	entries map[Provider]BlacklistEntry
	path    string
}

// NewBlacklist creates a new blacklist manager
func NewBlacklist(dataDir string) (*Blacklist, error) {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dataDir = filepath.Join(home, ".zimaos-echo")
	}

	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	path := filepath.Join(dataDir, "tunnel_blacklist.json")
	bl := &Blacklist{
		entries: make(map[Provider]BlacklistEntry),
		path:    path,
	}

	// Load existing blacklist
	if err := bl.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Clean expired entries
	bl.cleanExpired()

	return bl, nil
}

// IsBlacklisted checks if a provider is currently blacklisted
func (bl *Blacklist) IsBlacklisted(provider Provider) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	entry, exists := bl.entries[provider]
	if !exists {
		return false
	}

	// Check if expired
	if time.Since(entry.BlacklistedAt) > BlacklistDuration {
		return false
	}

	return true
}

// Add adds a provider to the blacklist
func (bl *Blacklist) Add(provider Provider, reason string) error {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	bl.entries[provider] = BlacklistEntry{
		Provider:      provider,
		BlacklistedAt: time.Now(),
		Reason:        reason,
	}

	return bl.save()
}

// Remove removes a provider from the blacklist
func (bl *Blacklist) Remove(provider Provider) error {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	delete(bl.entries, provider)
	return bl.save()
}

// GetAll returns all blacklisted providers
func (bl *Blacklist) GetAll() []BlacklistEntry {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	entries := make([]BlacklistEntry, 0, len(bl.entries))
	for _, entry := range bl.entries {
		// Only include non-expired entries
		if time.Since(entry.BlacklistedAt) <= BlacklistDuration {
			entries = append(entries, entry)
		}
	}

	return entries
}

// cleanExpired removes expired entries
func (bl *Blacklist) cleanExpired() {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	now := time.Now()
	for provider, entry := range bl.entries {
		if now.Sub(entry.BlacklistedAt) > BlacklistDuration {
			delete(bl.entries, provider)
		}
	}

	bl.save()
}

// load loads the blacklist from disk
func (bl *Blacklist) load() error {
	data, err := os.ReadFile(bl.path)
	if err != nil {
		return err
	}

	var entries []BlacklistEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	for _, entry := range entries {
		bl.entries[entry.Provider] = entry
	}

	return nil
}

// save saves the blacklist to disk
func (bl *Blacklist) save() error {
	entries := make([]BlacklistEntry, 0, len(bl.entries))
	for _, entry := range bl.entries {
		entries = append(entries, entry)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(bl.path, data, 0644)
}
