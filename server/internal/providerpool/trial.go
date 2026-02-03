package providerpool

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// TrialProviderID is the ID of the trial provider
	TrialProviderID = "zimaos-trial"

	// TrialTokenLimit is the maximum number of tokens allowed for trial
	TrialTokenLimit int64 = 5000

	// TrialConversationLimit is the maximum number of conversations allowed for trial
	TrialConversationLimit int64 = 5

	// trialExhaustedMarkerFile is the filename for the trial exhausted marker
	// This file is stored in a location that is NOT backed up to prevent
	// users from restoring trial quota via backup restore
	trialExhaustedMarkerFile = ".trial_exhausted"
)

var (
	// ErrTrialQuotaExhausted is returned when trial quota is exhausted
	ErrTrialQuotaExhausted = errors.New("trial quota exhausted")
)

// TrialQuotaStatus represents the current trial quota status
type TrialQuotaStatus struct {
	TokensUsed        int64     `json:"tokens_used"`
	TokensRemaining   int64     `json:"tokens_remaining"`
	TokenLimit        int64     `json:"token_limit"`
	ConversationsUsed int64     `json:"conversations_used"`
	ConversationsLeft int64     `json:"conversations_left"`
	ConversationLimit int64     `json:"conversation_limit"`
	IsExhausted       bool      `json:"is_exhausted"`
	ExhaustedReason   string    `json:"exhausted_reason,omitempty"`
	LastUpdated       time.Time `json:"last_updated"`
}

// TrialQuotaManager manages trial quota tracking
type TrialQuotaManager struct {
	storage  Storage
	registry *Registry

	// markerDir is the directory where the exhausted marker is stored
	// This should be a system directory that is NOT backed up
	markerDir string

	mu                sync.RWMutex
	tokensUsed        int64
	conversationsUsed int64
	exhausted         bool
	exhaustedReason   string
	lastUpdated       time.Time
}

// NewTrialQuotaManager creates a new trial quota manager
func NewTrialQuotaManager(storage Storage, registry *Registry) *TrialQuotaManager {
	m := &TrialQuotaManager{
		storage:  storage,
		registry: registry,
	}
	m.loadFromStorage()
	return m
}

// NewTrialQuotaManagerWithMarkerDir creates a new trial quota manager with a custom marker directory
func NewTrialQuotaManagerWithMarkerDir(storage Storage, registry *Registry, markerDir string) *TrialQuotaManager {
	m := &TrialQuotaManager{
		storage:   storage,
		registry:  registry,
		markerDir: markerDir,
	}
	m.loadFromStorage()
	return m
}

// getMarkerPath returns the path to the trial exhausted marker file
func (m *TrialQuotaManager) getMarkerPath() string {
	if m.markerDir != "" {
		return filepath.Join(m.markerDir, trialExhaustedMarkerFile)
	}
	// Default to system temp directory which is not backed up
	return filepath.Join(os.TempDir(), "zimaos-echo", trialExhaustedMarkerFile)
}

// getMachineID returns a unique identifier for this machine
// This is used to prevent trial quota from being transferred between machines
func getMachineID() string {
	// Try to get machine-specific identifiers
	var parts []string

	// Hostname
	if hostname, err := os.Hostname(); err == nil {
		parts = append(parts, hostname)
	}

	// Home directory (unique per user)
	if home, err := os.UserHomeDir(); err == nil {
		parts = append(parts, home)
	}

	// If we have parts, hash them
	if len(parts) > 0 {
		h := sha256.New()
		for _, p := range parts {
			h.Write([]byte(p))
		}
		return hex.EncodeToString(h.Sum(nil))[:16]
	}

	return "default"
}

// checkExhaustedMarker checks if the trial exhausted marker exists
func (m *TrialQuotaManager) checkExhaustedMarker() bool {
	markerPath := m.getMarkerPath()
	data, err := os.ReadFile(markerPath)
	if err != nil {
		return false
	}

	// Verify the marker contains the correct machine ID
	// This prevents copying the marker file to another machine
	expectedID := getMachineID()
	return string(data) == expectedID
}

// writeExhaustedMarker writes the trial exhausted marker
func (m *TrialQuotaManager) writeExhaustedMarker() error {
	markerPath := m.getMarkerPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(markerPath), 0700); err != nil {
		return err
	}

	// Write machine ID to marker file
	machineID := getMachineID()
	return os.WriteFile(markerPath, []byte(machineID), 0600)
}

// loadFromStorage loads trial quota from storage
func (m *TrialQuotaManager) loadFromStorage() {
	// First check if trial is already exhausted via marker file
	// This marker is stored outside the backup directory to prevent
	// users from restoring trial quota via backup restore
	if m.checkExhaustedMarker() {
		m.mu.Lock()
		m.exhausted = true
		m.exhaustedReason = "permanently_exhausted"
		m.lastUpdated = time.Now()
		m.mu.Unlock()
		return
	}

	// Load usage records for trial provider
	start := time.Time{} // From the beginning
	end := time.Now()
	records, err := m.storage.LoadUsage(TrialProviderID, start, end)
	if err != nil {
		return
	}

	var totalTokens int64
	conversationSet := make(map[string]bool)

	for _, record := range records {
		totalTokens += record.InputTokens + record.OutputTokens
		if record.SessionID != "" {
			conversationSet[record.SessionID] = true
		}
	}

	m.mu.Lock()
	m.tokensUsed = totalTokens
	m.conversationsUsed = int64(len(conversationSet))
	m.lastUpdated = time.Now()

	// Check if exhausted
	if m.tokensUsed >= TrialTokenLimit {
		m.exhausted = true
		m.exhaustedReason = "token_limit"
		// Write marker to prevent restore
		m.writeExhaustedMarker()
	} else if m.conversationsUsed >= TrialConversationLimit {
		m.exhausted = true
		m.exhaustedReason = "conversation_limit"
		// Write marker to prevent restore
		m.writeExhaustedMarker()
	}
	m.mu.Unlock()
}

// CheckQuota checks if trial quota is available
func (m *TrialQuotaManager) CheckQuota() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.exhausted {
		return ErrTrialQuotaExhausted
	}
	return nil
}

// RecordUsage records usage and checks if quota is exhausted
func (m *TrialQuotaManager) RecordUsage(inputTokens, outputTokens int64, sessionID string) (exhausted bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update tokens
	m.tokensUsed += inputTokens + outputTokens
	m.lastUpdated = time.Now()

	// Track unique conversations
	// Note: In a real implementation, we'd track this in storage
	// For now, we increment if sessionID is provided
	if sessionID != "" {
		// This is a simplified approach - in production, we'd check if this is a new session
		// For now, we'll rely on the loadFromStorage to get accurate counts
	}

	// Check if exhausted
	if m.tokensUsed >= TrialTokenLimit {
		m.exhausted = true
		m.exhaustedReason = "token_limit"
		// Write marker to prevent restore via backup
		m.writeExhaustedMarker()
		return true, nil
	}

	return false, nil
}

// GetStatus returns the current trial quota status
func (m *TrialQuotaManager) GetStatus() *TrialQuotaStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tokensRemaining := TrialTokenLimit - m.tokensUsed
	if tokensRemaining < 0 {
		tokensRemaining = 0
	}

	conversationsLeft := TrialConversationLimit - m.conversationsUsed
	if conversationsLeft < 0 {
		conversationsLeft = 0
	}

	return &TrialQuotaStatus{
		TokensUsed:        m.tokensUsed,
		TokensRemaining:   tokensRemaining,
		TokenLimit:        TrialTokenLimit,
		ConversationsUsed: m.conversationsUsed,
		ConversationsLeft: conversationsLeft,
		ConversationLimit: TrialConversationLimit,
		IsExhausted:       m.exhausted,
		ExhaustedReason:   m.exhaustedReason,
		LastUpdated:       m.lastUpdated,
	}
}

// IsExhausted returns whether the trial quota is exhausted
func (m *TrialQuotaManager) IsExhausted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.exhausted
}

// DeleteTrialProvider deletes the trial provider from the registry
func (m *TrialQuotaManager) DeleteTrialProvider() error {
	if m.registry == nil {
		return nil
	}

	// Unregister the trial provider
	err := m.registry.Unregister(TrialProviderID)
	if err != nil && err != ErrProviderNotFound {
		return err
	}

	return nil
}

// HandleQuotaExhausted handles the quota exhausted event
// It deletes the trial provider and returns the appropriate error message
func (m *TrialQuotaManager) HandleQuotaExhausted() (message string, err error) {
	status := m.GetStatus()

	// Delete the trial provider
	if err := m.DeleteTrialProvider(); err != nil {
		return "", err
	}

	// Return appropriate message based on exhaustion reason
	switch status.ExhaustedReason {
	case "token_limit":
		return "trial_quota_exhausted_tokens", nil
	case "conversation_limit":
		return "trial_quota_exhausted_conversations", nil
	default:
		return "trial_quota_exhausted", nil
	}
}

// Reset resets the trial quota (for testing or admin purposes)
func (m *TrialQuotaManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tokensUsed = 0
	m.conversationsUsed = 0
	m.exhausted = false
	m.exhaustedReason = ""
	m.lastUpdated = time.Now()
}

// IsTrialProvider checks if a provider ID is the trial provider
func IsTrialProvider(providerID string) bool {
	return providerID == TrialProviderID
}
