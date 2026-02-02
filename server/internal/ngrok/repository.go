package ngrok

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
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
	db *sql.DB
}

// NewRepository creates a new repository.
func NewRepository(dbPath string) (*Repository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	repo := &Repository{db: db}
	if err := repo.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return repo, nil
}

// Close closes the database connection.
func (r *Repository) Close() error {
	return r.db.Close()
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
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// initializeTunnelSubdomain ensures a tunnel subdomain exists.
func (r *Repository) initializeTunnelSubdomain(ctx context.Context) error {
	// Check if subdomain already exists
	var subdomain sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT tunnel_subdomain FROM remote_access_config WHERE id = 'default'
	`).Scan(&subdomain)

	if err == sql.ErrNoRows {
		// No config exists, create one with subdomain
		_, err = r.db.ExecContext(ctx, `
			INSERT INTO remote_access_config (id, tunnel_subdomain) VALUES ('default', ?)
		`, generateTunnelSubdomain())
		return err
	}

	if err != nil {
		return err
	}

	// If subdomain is empty, generate one
	if !subdomain.Valid || subdomain.String == "" {
		_, err = r.db.ExecContext(ctx, `
			UPDATE remote_access_config SET tunnel_subdomain = ? WHERE id = 'default'
		`, generateTunnelSubdomain())
		return err
	}

	return nil
}

// EnsureTunnelSubdomain returns the tunnel subdomain, generating and persisting one if empty.
// Call this when starting auto (Serveo) so we always pass echo-xxx.
func (r *Repository) EnsureTunnelSubdomain(ctx context.Context) (string, error) {
	var subdomain sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT tunnel_subdomain FROM remote_access_config WHERE id = 'default'
	`).Scan(&subdomain)

	if err == sql.ErrNoRows {
		sub := generateTunnelSubdomain()
		_, err = r.db.ExecContext(ctx, `
			INSERT INTO remote_access_config (id, tunnel_subdomain) VALUES ('default', ?)
		`, sub)
		if err != nil {
			return "", err
		}
		return sub, nil
	}
	if err != nil {
		return "", err
	}
	if subdomain.Valid && subdomain.String != "" {
		return subdomain.String, nil
	}
	sub := generateTunnelSubdomain()
	_, err = r.db.ExecContext(ctx, `
		UPDATE remote_access_config SET tunnel_subdomain = ? WHERE id = 'default'
	`, sub)
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

	row := r.db.QueryRowContext(ctx, `
		SELECT id, enabled, tunnel_subdomain, ngrok_authtoken, ngrok_domain, cloudflare_token, default_provider,
		       notification_email, notify_on_url_change, notify_on_expiry_warning,
		       notify_on_error, created_at, updated_at
		FROM remote_access_config
		WHERE id = 'default'
	`)

	var tunnelSubdomain, ngrokAuthtoken, ngrokDomain, cloudflareToken, defaultProvider, notificationEmail sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := row.Scan(
		&config.ID,
		&config.Enabled,
		&tunnelSubdomain,
		&ngrokAuthtoken,
		&ngrokDomain,
		&cloudflareToken,
		&defaultProvider,
		&notificationEmail,
		&config.NotifyOnURLChange,
		&config.NotifyOnExpiryWarning,
		&config.NotifyOnError,
		&createdAt,
		&updatedAt,
	)

	if err == sql.ErrNoRows {
		// Return default config
		return config, nil
	}

	if err != nil {
		return nil, err
	}

	if tunnelSubdomain.Valid {
		config.TunnelSubdomain = tunnelSubdomain.String
	}
	if ngrokAuthtoken.Valid {
		config.NgrokAuthtoken = ngrokAuthtoken.String
	}
	if ngrokDomain.Valid {
		config.NgrokDomain = ngrokDomain.String
	}
	if cloudflareToken.Valid {
		config.CloudflareToken = cloudflareToken.String
	}
	if defaultProvider.Valid && defaultProvider.String != "" {
		config.DefaultProvider = defaultProvider.String
	}
	if notificationEmail.Valid {
		config.NotificationEmail = notificationEmail.String
	}
	if createdAt.Valid {
		config.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		config.UpdatedAt = updatedAt.Time
	}

	return config, nil
}

// SaveConfig saves the remote access configuration.
func (r *Repository) SaveConfig(ctx context.Context, config *RemoteAccessConfig) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO remote_access_config (
			id, enabled, ngrok_authtoken, ngrok_domain, cloudflare_token, default_provider,
			notification_email, notify_on_url_change, notify_on_expiry_warning,
			notify_on_error, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			enabled = excluded.enabled,
			ngrok_authtoken = excluded.ngrok_authtoken,
			ngrok_domain = excluded.ngrok_domain,
			cloudflare_token = excluded.cloudflare_token,
			default_provider = excluded.default_provider,
			notification_email = excluded.notification_email,
			notify_on_url_change = excluded.notify_on_url_change,
			notify_on_expiry_warning = excluded.notify_on_expiry_warning,
			notify_on_error = excluded.notify_on_error,
			updated_at = CURRENT_TIMESTAMP
	`,
		"default",
		config.Enabled,
		config.NgrokAuthtoken,
		config.NgrokDomain,
		config.CloudflareToken,
		config.DefaultProvider,
		config.NotificationEmail,
		config.NotifyOnURLChange,
		config.NotifyOnExpiryWarning,
		config.NotifyOnError,
	)

	return err
}

// CreateSession creates a new session record.
func (r *Repository) CreateSession(ctx context.Context, session *RemoteAccessSession) (string, error) {
	id := uuid.New().String()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO remote_access_sessions (
			id, tunnel_url, started_at, expires_at, renewed_count, status
		) VALUES (?, ?, ?, ?, ?, ?)
	`,
		id,
		session.TunnelURL,
		session.StartedAt,
		session.ExpiresAt,
		session.RenewedCount,
		session.Status,
	)

	if err != nil {
		return "", err
	}

	return id, nil
}

// UpdateSession updates a session record.
func (r *Repository) UpdateSession(ctx context.Context, session *RemoteAccessSession) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE remote_access_sessions
		SET tunnel_url = ?, expires_at = ?, ended_at = ?,
		    renewed_count = ?, status = ?, error_message = ?
		WHERE id = ?
	`,
		session.TunnelURL,
		session.ExpiresAt,
		session.EndedAt,
		session.RenewedCount,
		session.Status,
		session.ErrorMessage,
		session.ID,
	)

	return err
}

// GetActiveSession returns the currently active session.
func (r *Repository) GetActiveSession(ctx context.Context) (*RemoteAccessSession, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tunnel_url, started_at, expires_at, ended_at,
		       renewed_count, status, error_message, created_at
		FROM remote_access_sessions
		WHERE status = 'active'
		ORDER BY created_at DESC
		LIMIT 1
	`)

	session := &RemoteAccessSession{}
	var endedAt sql.NullTime
	var errorMessage sql.NullString

	err := row.Scan(
		&session.ID,
		&session.TunnelURL,
		&session.StartedAt,
		&session.ExpiresAt,
		&endedAt,
		&session.RenewedCount,
		&session.Status,
		&errorMessage,
		&session.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if endedAt.Valid {
		session.EndedAt = endedAt.Time
	}
	if errorMessage.Valid {
		session.ErrorMessage = errorMessage.String
	}

	return session, nil
}

// EndSession marks a session as ended.
func (r *Repository) EndSession(ctx context.Context, sessionID string, status string, errorMsg string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE remote_access_sessions
		SET status = ?, error_message = ?, ended_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, errorMsg, sessionID)

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

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO remote_access_logs (id, session_id, event_type, message, metadata)
		VALUES (?, ?, ?, ?, ?)
	`, id, sessionID, eventType, message, string(metadataJSON))

	return err
}

// GetLogs returns log entries with pagination.
func (r *Repository) GetLogs(ctx context.Context, limit, offset int) ([]*RemoteAccessLog, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, session_id, event_type, message, metadata, created_at
		FROM remote_access_logs
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*RemoteAccessLog
	for rows.Next() {
		log := &RemoteAccessLog{}
		var sessionID, metadataStr sql.NullString

		err := rows.Scan(
			&log.ID,
			&sessionID,
			&log.EventType,
			&log.Message,
			&metadataStr,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if sessionID.Valid {
			log.SessionID = sessionID.String
		}
		if metadataStr.Valid && metadataStr.String != "" {
			json.Unmarshal([]byte(metadataStr.String), &log.Metadata)
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// GetErrorLogs returns only error log entries with pagination.
func (r *Repository) GetErrorLogs(ctx context.Context, limit, offset int) ([]*RemoteAccessLog, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, session_id, event_type, message, metadata, created_at
		FROM remote_access_logs
		WHERE event_type = 'error'
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*RemoteAccessLog
	for rows.Next() {
		log := &RemoteAccessLog{}
		var sessionID, metadataStr sql.NullString

		err := rows.Scan(
			&log.ID,
			&sessionID,
			&log.EventType,
			&log.Message,
			&metadataStr,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if sessionID.Valid {
			log.SessionID = sessionID.String
		}
		if metadataStr.Valid && metadataStr.String != "" {
			json.Unmarshal([]byte(metadataStr.String), &log.Metadata)
		}

		logs = append(logs, log)
	}

	return logs, nil
}
