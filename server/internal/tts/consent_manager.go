package tts

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// ConsentManager handles user privacy consent for online TTS services
type ConsentManager struct {
	db *sql.DB
	mu sync.RWMutex
}

// Consent represents a user's consent for a service
type Consent struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Service         string    `json:"service"` // 'edge-tts', 'sherpa', 'espeak'
	ConsentGiven    bool      `json:"consent_given"`
	ConsentDate     *time.Time `json:"consent_date,omitempty"`
	ConsentVersion  string    `json:"consent_version"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// NewConsentManager creates a new consent manager
func NewConsentManager(db *sql.DB) *ConsentManager {
	return &ConsentManager{
		db: db,
	}
}

// GetConsent retrieves user consent for a service
func (cm *ConsentManager) GetConsent(ctx context.Context, userID, service string) (*Consent, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var consent Consent
	err := cm.db.QueryRowContext(ctx,
		`SELECT id, user_id, service, consent_given, consent_date, consent_version, created_at, updated_at
		 FROM user_privacy_consent
		 WHERE user_id = ? AND service = ?`,
		userID, service).Scan(
		&consent.ID, &consent.UserID, &consent.Service, &consent.ConsentGiven,
		&consent.ConsentDate, &consent.ConsentVersion, &consent.CreatedAt, &consent.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil // No consent record yet
	}
	if err != nil {
		return nil, err
	}

	return &consent, nil
}

// SetConsent saves or updates user consent
func (cm *ConsentManager) SetConsent(ctx context.Context, userID, service string, given bool, version string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	var consentDate *time.Time
	if given {
		consentDate = &now
	}

	_, err := cm.db.ExecContext(ctx,
		`INSERT INTO user_privacy_consent (id, user_id, service, consent_given, consent_date, consent_version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, service) DO UPDATE SET
		 consent_given = ?, consent_date = ?, consent_version = ?, updated_at = ?`,
		generateID(), userID, service, given, consentDate, version, now, now,
		given, consentDate, version, now)

	return err
}

// HasConsent checks if user has given consent
func (cm *ConsentManager) HasConsent(ctx context.Context, userID, service string) (bool, error) {
	consent, err := cm.GetConsent(ctx, userID, service)
	if err != nil {
		return false, err
	}
	if consent == nil {
		return false, nil
	}
	return consent.ConsentGiven, nil
}

// generateID generates a unique ID
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
