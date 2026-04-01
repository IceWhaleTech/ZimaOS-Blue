package tts

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	z "github.com/IceWhaleTech/zorm"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ConsentManager handles user privacy consent for online TTS services
type ConsentManager struct {
	db         *sql.DB
	readDB     *sql.DB
	mu         sync.RWMutex
	schemaOnce sync.Once
	schemaErr  error
}

// Consent represents a user's consent for a service
type Consent struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	Service        string     `json:"service"` // 'edge-tts', 'espeak'
	ConsentGiven   bool       `json:"consent_given"`
	ConsentDate    *time.Time `json:"consent_date,omitempty"`
	ConsentVersion string     `json:"consent_version"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// NewConsentManager creates a new consent manager
func NewConsentManager(db *sql.DB) *ConsentManager {
	return NewConsentManagerWithReadDB(db, db)
}

// NewConsentManagerWithReadDB creates a new consent manager with separate write
// and read database handles.
func NewConsentManagerWithReadDB(writeDB, readDB *sql.DB) *ConsentManager {
	if readDB == nil {
		readDB = writeDB
	}
	return &ConsentManager{
		db:     writeDB,
		readDB: readDB,
	}
}

// GetConsent retrieves user consent for a service
func (cm *ConsentManager) GetConsent(ctx context.Context, userID, service string) (*Consent, error) {
	if err := cm.ensureSchema(); err != nil {
		return nil, err
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var rows []consentRow
	_, err := cm.readTable(ctx).Select(&rows,
		z.Where(
			z.Eq("user_id", userID),
			z.Eq("service", service),
		),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil // No consent record yet
	}

	return rowToConsent(rows[0]), nil
}

// SetConsent saves or updates user consent
func (cm *ConsentManager) SetConsent(ctx context.Context, userID, service string, given bool, version string) error {
	if err := cm.ensureSchema(); err != nil {
		return err
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := timeutil.NowTime()
	nowStr := now.UTC().Format(time.RFC3339Nano)
	var consentDate interface{}
	if given {
		consentDate = nowStr
	}

	consentGiven := 0
	if given {
		consentGiven = 1
	}

	_, err := cm.table(ctx).Insert(
		map[string]interface{}{
			"id":              generateID(),
			"user_id":         userID,
			"service":         service,
			"consent_given":   consentGiven,
			"consent_date":    consentDate,
			"consent_version": version,
			"created_at":      nowStr,
			"updated_at":      nowStr,
		},
		z.OnConflictDoUpdateSet(
			[]string{"user_id", "service"},
			[]string{"consent_given", "consent_date", "consent_version", "updated_at"},
		),
	)
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
	return timeutil.NowTime().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0180789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[timeutil.NowNano()%int64(len(charset))]
	}
	return string(b)
}

type consentRow struct {
	ID             string  `json:"id" zorm:"id"`
	UserID         string  `json:"user_id" zorm:"user_id"`
	Service        string  `json:"service" zorm:"service"`
	ConsentGiven   int     `json:"consent_given" zorm:"consent_given"`
	ConsentDate    *string `json:"consent_date" zorm:"consent_date"`
	ConsentVersion string  `json:"consent_version" zorm:"consent_version"`
	CreatedAt      string  `json:"created_at" zorm:"created_at"`
	UpdatedAt      string  `json:"updated_at" zorm:"updated_at"`
}

func (cm *ConsentManager) ensureSchema() error {
	cm.schemaOnce.Do(func() {
		cm.schemaErr = cm.initSchema()
	})
	return cm.schemaErr
}

func (cm *ConsentManager) initSchema() error {
	if cm == nil || cm.db == nil {
		return fmt.Errorf("tts consent db is required")
	}
	_, err := cm.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_privacy_consent (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			service TEXT NOT NULL,
			consent_given INTEGER NOT NULL DEFAULT 0,
			consent_date DATETIME,
			consent_version TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			UNIQUE(user_id, service)
		);
		CREATE INDEX IF NOT EXISTS idx_user_privacy_consent_user_service
			ON user_privacy_consent(user_id, service);
	`)
	return err
}

func (cm *ConsentManager) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, cm.db, "user_privacy_consent")
}

func (cm *ConsentManager) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, cm.reader(), "user_privacy_consent")
}

func (cm *ConsentManager) reader() *sql.DB {
	if cm != nil && cm.readDB != nil {
		return cm.readDB
	}
	if cm == nil {
		return nil
	}
	return cm.db
}

func rowToConsent(row consentRow) *Consent {
	return &Consent{
		ID:             row.ID,
		UserID:         row.UserID,
		Service:        row.Service,
		ConsentGiven:   row.ConsentGiven == 1,
		ConsentDate:    parseConsentTimePtr(row.ConsentDate),
		ConsentVersion: row.ConsentVersion,
		CreatedAt:      parseConsentTime(row.CreatedAt),
		UpdatedAt:      parseConsentTime(row.UpdatedAt),
	}
}

func parseConsentTimePtr(raw *string) *time.Time {
	if raw == nil {
		return nil
	}
	parsed := parseConsentTime(*raw)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}

func parseConsentTime(raw string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
