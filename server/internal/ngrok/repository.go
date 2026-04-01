package ngrok

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// RemoteAccessConfig represents the remote access configuration.
type RemoteAccessConfig struct {
	ID                    string    `json:"id"`
	Enabled               bool      `json:"enabled"`
	TunnelSubdomain       string    `json:"tunnel_subdomain,omitempty"`
	NgrokAuthtoken        string    `json:"ngrok_authtoken,omitempty"`
	NgrokDomain           string    `json:"ngrok_domain,omitempty"`
	CloudflareToken       string    `json:"cloudflare_token,omitempty"`
	DefaultProvider       string    `json:"default_provider,omitempty"`
	NotificationEmail     string    `json:"notification_email,omitempty"`
	NotifyOnURLChange     bool      `json:"notify_on_url_change"`
	NotifyOnExpiryWarning bool      `json:"notify_on_expiry_warning"`
	NotifyOnError         bool      `json:"notify_on_error"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// RemoteAccessSession represents a remote access session.
type RemoteAccessSession struct {
	ID           string    `json:"id"`
	TunnelURL    string    `json:"tunnel_url"`
	StartedAt    time.Time `json:"started_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	EndedAt      time.Time `json:"ended_at,omitempty"`
	RenewedCount int       `json:"renewed_count"`
	Status       string    `json:"status"` // active, expired, error
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// RemoteAccessLog represents a log entry.
type RemoteAccessLog struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id,omitempty"`
	EventType string                 `json:"event_type"`
	Message   string                 `json:"message"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// ConfigProvider defines the interface for tunnel configuration storage.
// Both Repository (SQLite) and ConfigStore (JSON) implement this interface.
type ConfigProvider interface {
	GetConfig(ctx context.Context) (*RemoteAccessConfig, error)
	SaveConfig(ctx context.Context, config *RemoteAccessConfig) error
	EnsureTunnelSubdomain(ctx context.Context) (string, error)
	AddLog(ctx context.Context, sessionID, eventType, message string, metadata map[string]interface{}) error
	GetLogs(ctx context.Context, limit, offset int) ([]*RemoteAccessLog, error)
	GetErrorLogs(ctx context.Context, limit, offset int) ([]*RemoteAccessLog, error)
	Close() error
}

// Repository handles remote access data persistence.
type Repository struct {
	db     *sql.DB
	readDB *sql.DB
}

// NewRepository creates a new repository.
func NewRepository(dbPath string) (*Repository, error) {
	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(dbPath, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(2)
		db.SetMaxIdleConns(1)
		if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA synchronous=FULL`); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA wal_autocheckpoint=1000`); err != nil {
			return err
		}
		repo := &Repository{db: db, readDB: db}
		return repo.migrate()
	})
	if err != nil {
		return nil, err
	}

	readDB, readErr := openNgrokReaderDB(dbPath)
	if readErr != nil || readDB == nil {
		readDB = db
	}

	return &Repository{db: db, readDB: readDB}, nil
}

// Close closes the database connection.
func (r *Repository) Close() error {
	if r == nil {
		return nil
	}
	if r.readDB != nil && r.readDB != r.db {
		_ = r.readDB.Close()
	}
	return r.db.Close()
}

func (r *Repository) reader() *sql.DB {
	if r != nil && r.readDB != nil {
		return r.readDB
	}
	if r == nil {
		return nil
	}
	return r.db
}

func (r *Repository) table(ctx context.Context, name string) *z.ZormTable {
	return z.TableContext(ctx, r.db, name)
}

func (r *Repository) readTable(ctx context.Context, name string) *z.ZormTable {
	return z.TableContext(ctx, r.reader(), name)
}

type remoteAccessConfigRow struct {
	ID                    string  `json:"id" zorm:"id"`
	Enabled               bool    `json:"enabled" zorm:"enabled"`
	TunnelSubdomain       *string `json:"tunnel_subdomain" zorm:"tunnel_subdomain"`
	NgrokAuthtoken        *string `json:"ngrok_authtoken" zorm:"ngrok_authtoken"`
	NgrokDomain           *string `json:"ngrok_domain" zorm:"ngrok_domain"`
	CloudflareToken       *string `json:"cloudflare_token" zorm:"cloudflare_token"`
	DefaultProvider       *string `json:"default_provider" zorm:"default_provider"`
	NotificationEmail     *string `json:"notification_email" zorm:"notification_email"`
	NotifyOnURLChange     bool    `json:"notify_on_url_change" zorm:"notify_on_url_change"`
	NotifyOnExpiryWarning bool    `json:"notify_on_expiry_warning" zorm:"notify_on_expiry_warning"`
	NotifyOnError         bool    `json:"notify_on_error" zorm:"notify_on_error"`
	CreatedAt             string  `json:"created_at" zorm:"created_at"`
	UpdatedAt             string  `json:"updated_at" zorm:"updated_at"`
}

type remoteAccessSessionRow struct {
	ID           string  `json:"id" zorm:"id"`
	TunnelURL    string  `json:"tunnel_url" zorm:"tunnel_url"`
	StartedAt    string  `json:"started_at" zorm:"started_at"`
	ExpiresAt    string  `json:"expires_at" zorm:"expires_at"`
	EndedAt      *string `json:"ended_at" zorm:"ended_at"`
	RenewedCount int     `json:"renewed_count" zorm:"renewed_count"`
	Status       string  `json:"status" zorm:"status"`
	ErrorMessage *string `json:"error_message" zorm:"error_message"`
	CreatedAt    string  `json:"created_at" zorm:"created_at"`
}

type remoteAccessLogRow struct {
	ID        string  `json:"id" zorm:"id"`
	SessionID *string `json:"session_id" zorm:"session_id"`
	EventType string  `json:"event_type" zorm:"event_type"`
	Message   string  `json:"message" zorm:"message"`
	Metadata  *string `json:"metadata" zorm:"metadata"`
	CreatedAt string  `json:"created_at" zorm:"created_at"`
}

func parseNgrokTime(raw string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func nullableNgrokTime(ts time.Time) interface{} {
	if ts.IsZero() {
		return nil
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func strValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func strOrDefault(v *string, def string) string {
	if v == nil || *v == "" {
		return def
	}
	return *v
}

func rowToConfig(row remoteAccessConfigRow) *RemoteAccessConfig {
	config := &RemoteAccessConfig{
		ID:                    row.ID,
		Enabled:               row.Enabled,
		TunnelSubdomain:       strValue(row.TunnelSubdomain),
		NgrokAuthtoken:        strValue(row.NgrokAuthtoken),
		NgrokDomain:           strValue(row.NgrokDomain),
		CloudflareToken:       strValue(row.CloudflareToken),
		DefaultProvider:       strOrDefault(row.DefaultProvider, "auto"),
		NotificationEmail:     strValue(row.NotificationEmail),
		NotifyOnURLChange:     row.NotifyOnURLChange,
		NotifyOnExpiryWarning: row.NotifyOnExpiryWarning,
		NotifyOnError:         row.NotifyOnError,
		CreatedAt:             parseNgrokTime(row.CreatedAt),
		UpdatedAt:             parseNgrokTime(row.UpdatedAt),
	}
	if config.ID == "" {
		config.ID = "default"
	}
	return config
}

func rowToSession(row remoteAccessSessionRow) *RemoteAccessSession {
	session := &RemoteAccessSession{
		ID:           row.ID,
		TunnelURL:    row.TunnelURL,
		StartedAt:    parseNgrokTime(row.StartedAt),
		ExpiresAt:    parseNgrokTime(row.ExpiresAt),
		RenewedCount: row.RenewedCount,
		Status:       row.Status,
		CreatedAt:    parseNgrokTime(row.CreatedAt),
	}
	if row.EndedAt != nil {
		session.EndedAt = parseNgrokTime(*row.EndedAt)
	}
	if row.ErrorMessage != nil {
		session.ErrorMessage = *row.ErrorMessage
	}
	return session
}

func rowToLog(row remoteAccessLogRow) *RemoteAccessLog {
	log := &RemoteAccessLog{
		ID:        row.ID,
		EventType: row.EventType,
		Message:   row.Message,
		CreatedAt: parseNgrokTime(row.CreatedAt),
	}
	if row.SessionID != nil {
		log.SessionID = *row.SessionID
	}
	if row.Metadata != nil && *row.Metadata != "" {
		_ = json.Unmarshal([]byte(*row.Metadata), &log.Metadata)
	}
	return log
}

func rowsToLogs(rows []remoteAccessLogRow) []*RemoteAccessLog {
	logs := make([]*RemoteAccessLog, 0, len(rows))
	for i := range rows {
		logs = append(logs, rowToLog(rows[i]))
	}
	return logs
}

func openNgrokReaderDB(dbPath string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := dbutil.OpenSQLiteWithRecovery(dsn, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(2)
		if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// migrate creates the necessary tables.
func (r *Repository) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS remote_access_config (
			id TEXT PRIMARY KEY DEFAULT 'default',
			enabled BOOLEAN DEFAULT FALSE,
			ngrok_authtoken TEXT,
			cloudflare_token TEXT,
			default_provider TEXT,
			notification_email TEXT,
			notify_on_url_change BOOLEAN DEFAULT TRUE,
			notify_on_expiry_warning BOOLEAN DEFAULT FALSE,
			notify_on_error BOOLEAN DEFAULT TRUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS remote_access_sessions (
			id TEXT PRIMARY KEY,
			tunnel_url TEXT NOT NULL,
			started_at DATETIME NOT NULL,
			expires_at DATETIME NOT NULL,
			ended_at DATETIME,
			renewed_count INTEGER DEFAULT 0,
			status TEXT DEFAULT 'active',
			error_message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS remote_access_logs (
			id TEXT PRIMARY KEY,
			session_id TEXT,
			event_type TEXT NOT NULL,
			message TEXT,
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_status ON remote_access_sessions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_created_at ON remote_access_logs(created_at DESC)`,
	}

	for _, query := range queries {
		if _, err := r.db.Exec(query); err != nil {
			return err
		}
	}

	// Add new columns if they don't exist (for existing databases)
	alterQueries := []string{
		`ALTER TABLE remote_access_config ADD COLUMN cloudflare_token TEXT`,
		`ALTER TABLE remote_access_config ADD COLUMN default_provider TEXT`,
		`ALTER TABLE remote_access_config ADD COLUMN tunnel_subdomain TEXT`,
		`ALTER TABLE remote_access_config ADD COLUMN ngrok_domain TEXT`,
	}

	for _, query := range alterQueries {
		// Ignore errors for columns that already exist
		r.db.Exec(query)
	}

	// Initialize tunnel_subdomain if not set
	if err := r.initializeTunnelSubdomain(context.Background()); err != nil {
		return err
	}

	return nil
}

// Base58 alphabet (excludes 0, O, I, l to avoid confusion).
const base58Alphabet = "180789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// initializeTunnelSubdomain ensures a tunnel subdomain exists.
func (r *Repository) initializeTunnelSubdomain(ctx context.Context) error {
	var rows []remoteAccessConfigRow
	_, err := r.readTable(ctx, "remote_access_config").Select(
		&rows,
		z.Fields("id", "tunnel_subdomain"),
		z.Where(z.Eq("id", "default")),
		z.Limit(1),
	)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		// No config exists, create one with subdomain
		_, err = r.table(ctx, "remote_access_config").Insert(z.V{
			"id":               "default",
			"tunnel_subdomain": generateTunnelSubdomain(),
		})
		return err
	}

	// If subdomain is empty, generate one
	if rows[0].TunnelSubdomain == nil || *rows[0].TunnelSubdomain == "" {
		_, err = r.table(ctx, "remote_access_config").Update(
			z.V{"tunnel_subdomain": generateTunnelSubdomain()},
			z.Fields("tunnel_subdomain"),
			z.Where(z.Eq("id", "default")),
		)
		return err
	}

	return nil
}

// EnsureTunnelSubdomain returns the tunnel subdomain, generating and persisting one if empty.
// Call this when starting auto (Serveo) so we always pass echo-xxx.
func (r *Repository) EnsureTunnelSubdomain(ctx context.Context) (string, error) {
	var rows []remoteAccessConfigRow
	_, err := r.readTable(ctx, "remote_access_config").Select(
		&rows,
		z.Fields("id", "tunnel_subdomain"),
		z.Where(z.Eq("id", "default")),
		z.Limit(1),
	)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		sub := generateTunnelSubdomain()
		_, err = r.table(ctx, "remote_access_config").Insert(z.V{
			"id":               "default",
			"tunnel_subdomain": sub,
		})
		if err != nil {
			return "", err
		}
		return sub, nil
	}
	if rows[0].TunnelSubdomain != nil && *rows[0].TunnelSubdomain != "" {
		return *rows[0].TunnelSubdomain, nil
	}
	sub := generateTunnelSubdomain()
	_, err = r.table(ctx, "remote_access_config").Update(
		z.V{"tunnel_subdomain": sub},
		z.Fields("tunnel_subdomain"),
		z.Where(z.Eq("id", "default")),
	)
	if err != nil {
		return "", err
	}
	return sub, nil
}

// GetConfig returns the remote access configuration.
func (r *Repository) GetConfig(ctx context.Context) (*RemoteAccessConfig, error) {
	config := &RemoteAccessConfig{
		ID:                    "default",
		Enabled:               false,
		DefaultProvider:       "auto",
		NotifyOnURLChange:     true,
		NotifyOnExpiryWarning: false,
		NotifyOnError:         true,
	}
	var rows []remoteAccessConfigRow
	_, err := r.readTable(ctx, "remote_access_config").Select(
		&rows,
		z.Where(z.Eq("id", "default")),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		// Return default config
		return config, nil
	}
	return rowToConfig(rows[0]), nil
}

// SaveConfig saves the remote access configuration.
func (r *Repository) SaveConfig(ctx context.Context, config *RemoteAccessConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}
	_, err := r.table(ctx, "remote_access_config").Insert(
		z.V{
			"id":                       "default",
			"enabled":                  config.Enabled,
			"ngrok_authtoken":          config.NgrokAuthtoken,
			"ngrok_domain":             config.NgrokDomain,
			"cloudflare_token":         config.CloudflareToken,
			"default_provider":         config.DefaultProvider,
			"notification_email":       config.NotificationEmail,
			"notify_on_url_change":     config.NotifyOnURLChange,
			"notify_on_expiry_warning": config.NotifyOnExpiryWarning,
			"notify_on_error":          config.NotifyOnError,
			"updated_at":               timeutil.NowTime().UTC().Format(time.RFC3339Nano),
		},
		z.OnConflictDoUpdateSet(
			[]string{"id"},
			[]string{
				"enabled",
				"ngrok_authtoken",
				"ngrok_domain",
				"cloudflare_token",
				"default_provider",
				"notification_email",
				"notify_on_url_change",
				"notify_on_expiry_warning",
				"notify_on_error",
				"updated_at",
			},
		),
	)

	return err
}

// CreateSession creates a new session record.
func (r *Repository) CreateSession(ctx context.Context, session *RemoteAccessSession) (string, error) {
	id := uuid.New().String()

	_, err := r.table(ctx, "remote_access_sessions").Insert(z.V{
		"id":            id,
		"tunnel_url":    session.TunnelURL,
		"started_at":    session.StartedAt.UTC().Format(time.RFC3339Nano),
		"expires_at":    session.ExpiresAt.UTC().Format(time.RFC3339Nano),
		"renewed_count": session.RenewedCount,
		"status":        session.Status,
	})

	if err != nil {
		return "", err
	}

	session.ID = id
	return id, nil
}

// UpdateSession updates a session record.
func (r *Repository) UpdateSession(ctx context.Context, session *RemoteAccessSession) error {
	_, err := r.table(ctx, "remote_access_sessions").Update(
		z.V{
			"tunnel_url":    session.TunnelURL,
			"expires_at":    session.ExpiresAt.UTC().Format(time.RFC3339Nano),
			"ended_at":      nullableNgrokTime(session.EndedAt),
			"renewed_count": session.RenewedCount,
			"status":        session.Status,
			"error_message": session.ErrorMessage,
		},
		z.Fields("tunnel_url", "expires_at", "ended_at", "renewed_count", "status", "error_message"),
		z.Where(z.Eq("id", session.ID)),
	)

	return err
}

// GetActiveSession returns the currently active session.
func (r *Repository) GetActiveSession(ctx context.Context) (*RemoteAccessSession, error) {
	var rows []remoteAccessSessionRow
	_, err := r.readTable(ctx, "remote_access_sessions").Select(
		&rows,
		z.Where(z.Eq("status", "active")),
		z.OrderBy("created_at DESC"),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rowToSession(rows[0]), nil
}

// EndSession marks a session as ended.
func (r *Repository) EndSession(ctx context.Context, sessionID string, status string, errorMsg string) error {
	_, err := r.table(ctx, "remote_access_sessions").Update(
		z.V{
			"status":        status,
			"error_message": errorMsg,
			"ended_at":      timeutil.NowTime().UTC().Format(time.RFC3339Nano),
		},
		z.Fields("status", "error_message", "ended_at"),
		z.Where(z.Eq("id", sessionID)),
	)

	return err
}

// AddLog adds a log entry.
func (r *Repository) AddLog(ctx context.Context, sessionID, eventType, message string, metadata map[string]interface{}) error {
	id := uuid.New().String()

	var metadataJSON []byte
	if metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return err
		}
	}

	_, err := r.table(ctx, "remote_access_logs").Insert(z.V{
		"id":         id,
		"session_id": sessionID,
		"event_type": eventType,
		"message":    message,
		"metadata":   string(metadataJSON),
	})

	return err
}

// GetLogs returns log entries with pagination.
func (r *Repository) GetLogs(ctx context.Context, limit, offset int) ([]*RemoteAccessLog, error) {
	var rows []remoteAccessLogRow
	_, err := r.readTable(ctx, "remote_access_logs").Select(
		&rows,
		z.OrderBy("created_at DESC"),
		z.Limit(limit, offset),
	)
	if err != nil {
		return nil, err
	}
	return rowsToLogs(rows), nil
}

// GetErrorLogs returns only error log entries with pagination.
func (r *Repository) GetErrorLogs(ctx context.Context, limit, offset int) ([]*RemoteAccessLog, error) {
	var rows []remoteAccessLogRow
	_, err := r.readTable(ctx, "remote_access_logs").Select(
		&rows,
		z.Where(z.Eq("event_type", "error")),
		z.OrderBy("created_at DESC"),
		z.Limit(limit, offset),
	)
	if err != nil {
		return nil, err
	}
	return rowsToLogs(rows), nil
}
