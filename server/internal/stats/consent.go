package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ConsentStatus represents the user's consent status for statistics collection.
type ConsentStatus struct {
	Consented   bool      `json:"consented"`
	ConsentedAt time.Time `json:"consented_at,omitempty"`
	RevokedAt   time.Time `json:"revoked_at,omitempty"`
	Version     string    `json:"version"` // Consent version for tracking policy changes
}

// ConsentManager manages user consent for statistics collection.
type ConsentManager struct {
	mu          sync.RWMutex
	storagePath string
	status      *ConsentStatus
	collector   *StatisticsCollector
}

// NewConsentManager creates a new consent manager.
func NewConsentManager(storagePath string, collector *StatisticsCollector) *ConsentManager {
	if storagePath == "" {
		home, _ := os.UserHomeDir()
		storagePath = filepath.Join(home, ".local", "share", "zimaos-blue", "stats")
	}

	manager := &ConsentManager{
		storagePath: storagePath,
		collector:   collector,
		status: &ConsentStatus{
			Consented: false,
			Version:   "1.0",
		},
	}

	// Load existing consent status
	manager.loadStatus()

	// Update collector based on consent
	if collector != nil {
		collector.SetEnabled(manager.status.Consented)
	}

	return manager
}

// GetStatus returns the current consent status.
func (m *ConsentManager) GetStatus() *ConsentStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy
	return &ConsentStatus{
		Consented:   m.status.Consented,
		ConsentedAt: m.status.ConsentedAt,
		RevokedAt:   m.status.RevokedAt,
		Version:     m.status.Version,
	}
}

// SetConsent sets the user's consent status.
func (m *ConsentManager) SetConsent(consented bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := timeutil.NowTime()

	if consented {
		m.status.Consented = true
		m.status.ConsentedAt = now
		m.status.RevokedAt = time.Time{}
	} else {
		m.status.Consented = false
		m.status.RevokedAt = now
	}

	// Update collector
	if m.collector != nil {
		m.collector.SetEnabled(consented)
	}

	return m.saveStatus()
}

// RevokeConsent revokes consent and optionally clears collected data.
func (m *ConsentManager) RevokeConsent(clearData bool) error {
	if err := m.SetConsent(false); err != nil {
		return err
	}

	if clearData && m.collector != nil {
		return m.collector.Clear()
	}

	return nil
}

// IsConsented returns whether the user has consented.
func (m *ConsentManager) IsConsented() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status.Consented
}

// loadStatus loads consent status from disk.
func (m *ConsentManager) loadStatus() {
	statusFile := filepath.Join(m.storagePath, "consent.json")
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return
	}

	var status ConsentStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return
	}

	m.status = &status
}

// saveStatus saves consent status to disk.
func (m *ConsentManager) saveStatus() error {
	if err := os.MkdirAll(m.storagePath, 0755); err != nil {
		return err
	}

	statusFile := filepath.Join(m.storagePath, "consent.json")
	data, err := json.MarshalIndent(m.status, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(statusFile, data, 0644)
}

// ConsentInfo provides information about what data is collected.
type ConsentInfo struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	DataTypes   []string `json:"data_types"`
	Purpose     string   `json:"purpose"`
	Retention   string   `json:"retention"`
	Version     string   `json:"version"`
}

// GetConsentInfo returns information about what data is collected.
func GetConsentInfo() *ConsentInfo {
	return &ConsentInfo{
		Title:       "Usage Statistics Collection",
		Description: "We collect anonymous usage statistics to improve the application.",
		DataTypes: []string{
			"API call counts by provider and model",
			"Token usage (input, output, cache)",
			"Response latency metrics",
			"Error rates and types",
			"Tool calling usage",
		},
		Purpose:   "To understand usage patterns, identify performance issues, and improve the application.",
		Retention: "Statistics are retained for 90 days by default.",
		Version:   "1.0",
	}
}
