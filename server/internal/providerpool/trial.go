package providerpool

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// TrialProviderID is the ID of the trial provider
	TrialProviderID = "zimaos-blue-trial"

	// TrialTokenLimit is the maximum number of tokens allowed for trial
	TrialTokenLimit int64 = 10000

	// trialExhaustedMarkerFile is the filename for the trial exhausted marker
	// This file is stored in a location that is NOT backed up to prevent
	// users from restoring trial quota via backup restore
	trialExhaustedMarkerFile = ".trial_exhausted"

	// trialQuotaFile is the filename for persistent quota storage
	trialQuotaFile = ".trial_quota"
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
}

// TrialQuotaManager manages trial quota tracking
type TrialQuotaManager struct {
	storage  Storage
	registry *Registry

	// markerDir is the directory where the exhausted marker is stored
	// This should be a system directory that is NOT backed up
	markerDir string

	mu              sync.RWMutex
	tokensUsed      int64
	exhausted       bool
	exhaustedReason string
	lastUpdated     time.Time
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
	return filepath.Join(os.TempDir(), "zimaos-blue", trialExhaustedMarkerFile)
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

// trialQuotaData is the structure for persistent quota storage
type trialQuotaData struct {
	TokensUsed      int64     `json:"tokens_used"`
	Exhausted       bool      `json:"exhausted"`
	ExhaustedReason string    `json:"exhausted_reason,omitempty"`
	LastUpdated     time.Time `json:"last_updated"`
	MachineID       string    `json:"machine_id"`
}

// getEncryptionKey derives a 32-byte AES key from machine ID
func getEncryptionKey() []byte {
	machineID := getMachineID()
	// Add a salt to make the key more unique
	salt := "zimaos-blue-trial-quota-v1"
	h := sha256.New()
	h.Write([]byte(machineID))
	h.Write([]byte(salt))
	return h.Sum(nil)
}

// encrypt encrypts data using AES-GCM
func encrypt(plaintext []byte) (string, error) {
	key := getEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts data using AES-GCM
func decrypt(encoded string) ([]byte, error) {
	key := getEncryptionKey()
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// getQuotaFilePath returns the path to the quota storage file
func (m *TrialQuotaManager) getQuotaFilePath() string {
	if m.markerDir != "" {
		return filepath.Join(m.markerDir, trialQuotaFile)
	}
	return filepath.Join(os.TempDir(), "zimaos-blue", trialQuotaFile)
}

// saveQuotaToFile saves the current quota to an encrypted file
func (m *TrialQuotaManager) saveQuotaToFile() error {
	data := trialQuotaData{
		TokensUsed:      m.tokensUsed,
		Exhausted:       m.exhausted,
		ExhaustedReason: m.exhaustedReason,
		LastUpdated:     m.lastUpdated,
		MachineID:       getMachineID(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	encrypted, err := encrypt(jsonData)
	if err != nil {
		return err
	}

	quotaPath := m.getQuotaFilePath()
	if err := os.MkdirAll(filepath.Dir(quotaPath), 0700); err != nil {
		return err
	}

	return os.WriteFile(quotaPath, []byte(encrypted), 0600)
}

// loadQuotaFromFile loads quota from the encrypted file
func (m *TrialQuotaManager) loadQuotaFromFile() (*trialQuotaData, error) {
	quotaPath := m.getQuotaFilePath()
	encrypted, err := os.ReadFile(quotaPath)
	if err != nil {
		return nil, err
	}

	decrypted, err := decrypt(string(encrypted))
	if err != nil {
		return nil, err
	}

	var data trialQuotaData
	if err := json.Unmarshal(decrypted, &data); err != nil {
		return nil, err
	}

	// Verify machine ID matches
	if data.MachineID != getMachineID() {
		return nil, errors.New("machine ID mismatch")
	}

	return &data, nil
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
	if m.checkExhaustedMarker() {
		m.mu.Lock()
		m.exhausted = true
		m.exhaustedReason = "permanently_exhausted"
		m.lastUpdated = time.Now()
		m.mu.Unlock()
		return
	}

	// Try to load from encrypted quota file
	data, err := m.loadQuotaFromFile()
	if err != nil {
		// If file doesn't exist, this is a fresh install - start with zero usage
		if os.IsNotExist(err) {
			m.mu.Lock()
			m.tokensUsed = 0
			m.exhausted = false
			m.lastUpdated = time.Now()
			m.mu.Unlock()
			// Save initial state
			m.saveQuotaToFile()
			fmt.Printf("[TrialQuotaManager] Fresh install, starting with zero usage\n")
			return
		}
		// File exists but failed to load (corrupted/tampered) - treat as exhausted
		m.mu.Lock()
		m.exhausted = true
		m.exhaustedReason = "quota_file_invalid"
		m.lastUpdated = time.Now()
		m.mu.Unlock()
		m.writeExhaustedMarker()
		fmt.Printf("[TrialQuotaManager] Failed to load quota file, treating as exhausted: %v\n", err)
		return
	}

	// Successfully loaded from encrypted file
	m.mu.Lock()
	m.tokensUsed = data.TokensUsed
	m.exhausted = data.Exhausted
	m.exhaustedReason = data.ExhaustedReason
	m.lastUpdated = data.LastUpdated
	m.mu.Unlock()
	fmt.Printf("[TrialQuotaManager] Loaded from encrypted file: tokens=%d, exhausted=%v\n",
		data.TokensUsed, data.Exhausted)
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

	// Update tokens
	previousUsed := m.tokensUsed
	m.tokensUsed += inputTokens + outputTokens
	m.lastUpdated = time.Now()

	fmt.Printf("[TrialQuotaManager] RecordUsage: input=%d, output=%d, previous=%d, new=%d, limit=%d\n",
		inputTokens, outputTokens, previousUsed, m.tokensUsed, TrialTokenLimit)

	// Check if exhausted
	if m.tokensUsed >= TrialTokenLimit {
		m.exhausted = true
		m.exhaustedReason = "token_limit"
		m.writeExhaustedMarker()
		fmt.Printf("[TrialQuotaManager] Trial quota EXHAUSTED!\n")
		m.mu.Unlock()
		m.saveQuotaToFile()
		return true, nil
	}

	m.mu.Unlock()
	// Save to encrypted file after each usage update
	m.saveQuotaToFile()
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

	return &TrialQuotaStatus{
		TokensUsed:      m.tokensUsed,
		TokensRemaining: tokensRemaining,
		TokenLimit:      TrialTokenLimit,
		IsExhausted:     m.exhausted,
		ExhaustedReason: m.exhaustedReason,
		LastUpdated:     m.lastUpdated,
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
	// Support both old and new trial provider IDs for backwards compatibility
	return providerID == TrialProviderID || providerID == "zimaos-blue-trial"
}
