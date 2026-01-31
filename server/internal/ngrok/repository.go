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
	NgrokAuthtoken        string    `json:"ngrok_authtoken,omitempty"`
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

	return nil
}

// GetConfig returns the remote access configuration.
func (r *Repository) GetConfig(ctx context.Context) (*RemoteAccessConfig, error) {
	config := &RemoteAccessConfig{
		ID:                    "default",
		Enabled:               false,
		NotifyOnURLChange:     true,
		NotifyOnExpiryWarning: false,
		NotifyOnError:         true,
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, enabled, ngrok_authtoken, notification_email,
		       notify_on_url_change, notify_on_expiry_warning, notify_on_error,
		       created_at, updated_at
		FROM remote_access_config
		WHERE id = 'default'
	`)

	var ngrokAuthtoken, notificationEmail sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := row.Scan(
		&config.ID,
		&config.Enabled,
		&ngrokAuthtoken,
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

	if ngrokAuthtoken.Valid {
		config.NgrokAuthtoken = ngrokAuthtoken.String
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
			id, enabled, ngrok_authtoken, notification_email,
			notify_on_url_change, notify_on_expiry_warning, notify_on_error,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			enabled = excluded.enabled,
			ngrok_authtoken = excluded.ngrok_authtoken,
			notification_email = excluded.notification_email,
			notify_on_url_change = excluded.notify_on_url_change,
			notify_on_expiry_warning = excluded.notify_on_expiry_warning,
			notify_on_error = excluded.notify_on_error,
			updated_at = CURRENT_TIMESTAMP
	`,
		"default",
		config.Enabled,
		config.NgrokAuthtoken,
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
