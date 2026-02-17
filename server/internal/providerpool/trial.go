package providerpool

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	// TrialProviderID is the ID of the trial provider
	TrialProviderID = "zimaos-blue-trial"
)

var (
	// ErrTrialQuotaExhausted is returned when trial quota is exhausted
	ErrTrialQuotaExhausted = errors.New("trial quota exhausted")
)

// TrialQuotaStatus represents the current trial quota status
type TrialQuotaStatus struct {
	TokensUsed      int64     `json:"tokens_used"`
	TokensRemaining int64     `json:"tokens_remaining"`
	TokenLimit      int64     `json:"token_limit"`
	IsExhausted     bool      `json:"is_exhausted"`
	ExhaustedReason string    `json:"exhausted_reason,omitempty"`
	LastUpdated     time.Time `json:"last_updated"`
	ExpiresAt       int64     `json:"expires_at,omitempty"` // license expiry unix timestamp
	IsExpired       bool      `json:"is_expired,omitempty"`
}

// TrialQuotaManager manages trial quota tracking with Ed25519 license verification
// and HMAC-protected state persistence.
type TrialQuotaManager struct {
	registry *Registry
	dataDir  string

	// License fields
	claims     *LicenseClaims
	licenseSig []byte // used as HMAC key for state file

	mu              sync.RWMutex
	tokensUsed      int64
	tokenLimit      int64
	exhausted       bool
	exhaustedReason string
	lastUpdated     time.Time
}

// NewTrialQuotaManager creates a new trial quota manager.
// licenseStr is the Ed25519-signed license injected at build time.
// dataDir is a persistent directory for state storage (not tmpdir).
func NewTrialQuotaManager(registry *Registry, dataDir string, licenseStr string) *TrialQuotaManager {
	m := &TrialQuotaManager{
		registry: registry,
		dataDir:  dataDir,
	}

	if licenseStr == "" {
		m.exhausted = true
		m.exhaustedReason = "no_license"
		return m
	}

	claims, sig, err := VerifyLicense(licenseStr)
	if err != nil && !errors.Is(err, ErrLicenseExpired) {
		fmt.Printf("[TrialQuotaManager] License verification failed: %v\n", err)
		m.exhausted = true
		m.exhaustedReason = "invalid_license"
		return m
	}

	m.claims = claims
	m.licenseSig = sig
	m.tokenLimit = claims.Limit

	// Check time-based expiry
	if errors.Is(err, ErrLicenseExpired) {
		m.exhausted = true
		m.exhaustedReason = "license_expired"
		fmt.Printf("[TrialQuotaManager] License expired (exp=%d)\n", claims.ExpiresAt)
		return m
	}

	m.loadFromStorage()
	return m
}

// Claims returns the verified license claims, or nil if no valid license.
func (m *TrialQuotaManager) Claims() *LicenseClaims {
	return m.claims
}

// loadFromStorage loads trial quota from HMAC-protected state file.
func (m *TrialQuotaManager) loadFromStorage() {
	// Check version change via IAT marker (works across different HMAC keys)
	if m.claims != nil {
		storedIAT := LoadLicenseIAT(m.dataDir)
		if storedIAT > 0 && storedIAT != m.claims.IssuedAt {
			// New license version → delete old state and reset
			DeleteTrialState(m.dataDir)
			m.mu.Lock()
			m.tokensUsed = 0
			m.exhausted = false
			m.exhaustedReason = ""
			m.lastUpdated = time.Now()
			m.mu.Unlock()
			m.saveState()
			fmt.Printf("[TrialQuotaManager] New license version detected (old_iat=%d, new_iat=%d), resetting quota\n",
				storedIAT, m.claims.IssuedAt)
			return
		}
	}

	state, err := LoadTrialState(m.dataDir, m.licenseSig)
	if err != nil {
		if errors.Is(err, ErrStateTampered) {
			m.mu.Lock()
			m.exhausted = true
			m.exhaustedReason = "state_tampered"
			m.lastUpdated = time.Now()
			m.mu.Unlock()
			m.saveState()
			fmt.Printf("[TrialQuotaManager] State file tampered, treating as exhausted\n")
			return
		}
		fmt.Printf("[TrialQuotaManager] Failed to load state: %v, starting fresh\n", err)
		m.mu.Lock()
		m.tokensUsed = 0
		m.exhausted = false
		m.lastUpdated = time.Now()
		m.mu.Unlock()
		m.saveState()
		return
	}

	if state == nil {
		m.mu.Lock()
		m.tokensUsed = 0
		m.exhausted = false
		m.lastUpdated = time.Now()
		m.mu.Unlock()
		m.saveState()
		fmt.Printf("[TrialQuotaManager] Fresh install, starting with zero usage\n")
		return
	}

	now := time.Now().Unix()

	// Clock-rollback detection: 5-minute tolerance for NTP drift
	if state.HighWaterMark > 0 && now < state.HighWaterMark-300 {
		m.mu.Lock()
		m.exhausted = true
		m.exhaustedReason = "clock_rollback"
		m.lastUpdated = time.Now()
		m.mu.Unlock()
		m.saveState()
		fmt.Printf("[TrialQuotaManager] Clock rollback detected (now=%d, hwm=%d), exhausting trial\n", now, state.HighWaterMark)
		return
	}

	m.mu.Lock()
	m.tokensUsed = state.TokensUsed
	m.exhausted = state.Exhausted
	m.exhaustedReason = state.ExhaustedReason
	m.lastUpdated = state.LastUpdated
	m.mu.Unlock()
	fmt.Printf("[TrialQuotaManager] Loaded state: tokens=%d, exhausted=%v\n",
		state.TokensUsed, state.Exhausted)
}

func (m *TrialQuotaManager) saveState() {
	if m.licenseSig == nil {
		return
	}
	now := time.Now().Unix()
	var licenseIAT int64
	if m.claims != nil {
		licenseIAT = m.claims.IssuedAt
	}
	state := &trialState{
		TokensUsed:      m.tokensUsed,
		Exhausted:       m.exhausted,
		ExhaustedReason: m.exhaustedReason,
		LastUpdated:     m.lastUpdated,
		HighWaterMark:   now,
		LicenseIAT:      licenseIAT,
	}
	if err := SaveTrialState(m.dataDir, m.licenseSig, state); err != nil {
		fmt.Printf("[TrialQuotaManager] Failed to save state: %v\n", err)
	}
	if licenseIAT > 0 {
		SaveLicenseIAT(m.dataDir, licenseIAT)
	}
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

	previousUsed := m.tokensUsed
	m.tokensUsed += inputTokens + outputTokens
	m.lastUpdated = time.Now()

	fmt.Printf("[TrialQuotaManager] RecordUsage: input=%d, output=%d, previous=%d, new=%d, limit=%d\n",
		inputTokens, outputTokens, previousUsed, m.tokensUsed, m.tokenLimit)

	if m.tokensUsed >= m.tokenLimit {
		m.exhausted = true
		m.exhaustedReason = "token_limit"
		fmt.Printf("[TrialQuotaManager] Trial quota EXHAUSTED!\n")
		m.mu.Unlock()
		m.saveState()
		return true, nil
	}

	m.mu.Unlock()
	m.saveState()
	return false, nil
}

// GetStatus returns the current trial quota status
func (m *TrialQuotaManager) GetStatus() *TrialQuotaStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tokensRemaining := m.tokenLimit - m.tokensUsed
	if tokensRemaining < 0 {
		tokensRemaining = 0
	}

	status := &TrialQuotaStatus{
		TokensUsed:      m.tokensUsed,
		TokensRemaining: tokensRemaining,
		TokenLimit:      m.tokenLimit,
		IsExhausted:     m.exhausted,
		ExhaustedReason: m.exhaustedReason,
		LastUpdated:     m.lastUpdated,
	}
	if m.claims != nil && m.claims.ExpiresAt > 0 {
		status.ExpiresAt = m.claims.ExpiresAt
		status.IsExpired = time.Now().Unix() > m.claims.ExpiresAt
	}
	return status
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
	err := m.registry.Unregister(TrialProviderID)
	if err != nil && err != ErrProviderNotFound {
		return err
	}
	return nil
}

// HandleQuotaExhausted handles the quota exhausted event
func (m *TrialQuotaManager) HandleQuotaExhausted() (message string, err error) {
	status := m.GetStatus()
	if err := m.DeleteTrialProvider(); err != nil {
		return "", err
	}
	switch status.ExhaustedReason {
	case "token_limit":
		return "trial_quota_exhausted_tokens", nil
	case "license_expired":
		return "trial_expired_update_available", nil
	case "clock_rollback":
		return "trial_clock_tampered", nil
	default:
		return "trial_quota_exhausted", nil
	}
}

// Reset resets the trial quota (for testing or admin purposes)
func (m *TrialQuotaManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tokensUsed = 0
	m.exhausted = false
	m.exhaustedReason = ""
	m.lastUpdated = time.Now()
}

// IsTrialProvider checks if a provider ID is the trial provider
func IsTrialProvider(providerID string) bool {
	return providerID == TrialProviderID
}
