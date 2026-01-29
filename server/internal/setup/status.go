// Package setup provides setup wizard and first-run experience management.
package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FirstRunStatus represents the first-run wizard status (v0.10.3).
// This is separate from the general SetupStatus used by the setup wizard.
type FirstRunStatus struct {
	// First run status
	FirstRunCompleted bool      `json:"first_run_completed"`
	CompletedAt       time.Time `json:"completed_at,omitempty"`

	// CLI status
	CLIInstalled bool   `json:"cli_installed"`
	CLIVersion   string `json:"cli_version,omitempty"`
	CLIPath      string `json:"cli_path,omitempty"`
	CLISkipped   bool   `json:"cli_skipped"`

	// Provider detection
	OllamaDetected  bool     `json:"ollama_detected"`
	OllamaEndpoint  string   `json:"ollama_endpoint,omitempty"`
	OllamaModels    []string `json:"ollama_models,omitempty"`

	// Configured providers
	ProvidersConfigured []string `json:"providers_configured,omitempty"`
	DefaultProvider     string   `json:"default_provider,omitempty"`

	// Statistics
	StatisticsOptIn bool `json:"statistics_opt_in"`

	// Environment
	HasAnthropicKey bool `json:"has_anthropic_key"`
	HasOpenAIKey    bool `json:"has_openai_key"`
	CCSwitchActive  bool `json:"cc_switch_active"`
}

// FirstRunManager manages first-run status persistence and retrieval (v0.10.3).
type FirstRunManager struct {
	mu          sync.RWMutex
	status      *FirstRunStatus
	storagePath string
}

// NewFirstRunManager creates a new first-run manager.
func NewFirstRunManager(storagePath string) *FirstRunManager {
	if storagePath == "" {
		home, _ := os.UserHomeDir()
		storagePath = filepath.Join(home, ".local", "share", "zimaos-echo")
	}

	manager := &FirstRunManager{
		storagePath: storagePath,
		status:      &FirstRunStatus{},
	}

	// Load existing status
	manager.loadStatus()

	return manager
}

// GetStatus returns the current first-run status.
func (m *FirstRunManager) GetStatus() *FirstRunStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy
	status := *m.status
	return &status
}

// IsFirstRun returns true if this is the first run.
func (m *FirstRunManager) IsFirstRun() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return !m.status.FirstRunCompleted
}

// MarkComplete marks the first-run as complete.
func (m *FirstRunManager) MarkComplete() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.FirstRunCompleted = true
	m.status.CompletedAt = time.Now()

	return m.saveStatusLocked()
}

// SetCLIStatus updates the CLI installation status.
func (m *FirstRunManager) SetCLIStatus(installed bool, version, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.CLIInstalled = installed
	m.status.CLIVersion = version
	m.status.CLIPath = path

	return m.saveStatusLocked()
}

// SetCLISkipped marks CLI download as skipped.
func (m *FirstRunManager) SetCLISkipped(skipped bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.CLISkipped = skipped

	return m.saveStatusLocked()
}

// SetOllamaStatus updates the Ollama detection status.
func (m *FirstRunManager) SetOllamaStatus(detected bool, endpoint string, models []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.OllamaDetected = detected
	m.status.OllamaEndpoint = endpoint
	m.status.OllamaModels = models

	return m.saveStatusLocked()
}

// SetProvidersConfigured updates the list of configured providers.
func (m *FirstRunManager) SetProvidersConfigured(providers []string, defaultProvider string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.ProvidersConfigured = providers
	m.status.DefaultProvider = defaultProvider

	return m.saveStatusLocked()
}

// SetStatisticsOptIn updates the statistics opt-in status.
func (m *FirstRunManager) SetStatisticsOptIn(optIn bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.StatisticsOptIn = optIn

	return m.saveStatusLocked()
}

// SetEnvironmentStatus updates the environment detection status.
func (m *FirstRunManager) SetEnvironmentStatus(hasAnthropicKey, hasOpenAIKey, ccSwitchActive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.HasAnthropicKey = hasAnthropicKey
	m.status.HasOpenAIKey = hasOpenAIKey
	m.status.CCSwitchActive = ccSwitchActive

	return m.saveStatusLocked()
}

// Reset resets the first-run status.
func (m *FirstRunManager) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status = &FirstRunStatus{}

	return m.saveStatusLocked()
}

// loadStatus loads the first-run status from disk.
func (m *FirstRunManager) loadStatus() {
	statusFile := filepath.Join(m.storagePath, "first_run_status.json")
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return
	}

	var status FirstRunStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return
	}

	m.status = &status
}

// saveStatusLocked saves the first-run status to disk (must hold lock).
func (m *FirstRunManager) saveStatusLocked() error {
	if err := os.MkdirAll(m.storagePath, 0755); err != nil {
		return err
	}

	statusFile := filepath.Join(m.storagePath, "first_run_status.json")
	data, err := json.MarshalIndent(m.status, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(statusFile, data, 0644)
}

// SaveStatus saves the first-run status to disk.
func (m *FirstRunManager) SaveStatus() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveStatusLocked()
}

// LoadStatus reloads the first-run status from disk.
func (m *FirstRunManager) LoadStatus() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadStatus()
}

// GetStoragePath returns the storage path.
func (m *FirstRunManager) GetStoragePath() string {
	return m.storagePath
}
