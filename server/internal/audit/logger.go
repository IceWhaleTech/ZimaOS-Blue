package audit

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Logger provides audit logging functionality.
type Logger interface {
	// Log records an audit entry.
	Log(ctx context.Context, entry *Entry) error
	// Query retrieves audit entries based on parameters.
	Query(ctx context.Context, params *QueryParams) (*QueryResult, error)
	// GetByID retrieves a single audit entry.
	GetByID(ctx context.Context, id uuid.UUID) (*Entry, error)
	// Close closes the logger and flushes any pending entries.
	Close() error
}

// SQLiteLogger implements Logger using SQLite.
type SQLiteLogger struct {
	db       *sql.DB
	buffer   chan *Entry
	done     chan struct{}
	batchSize int
	flushInterval time.Duration
}

// LoggerConfig holds configuration for the logger.
type LoggerConfig struct {
	// BatchSize is the number of entries to batch before writing.
	BatchSize int
	// FlushInterval is how often to flush the buffer.
	FlushInterval time.Duration
	// BufferSize is the size of the entry buffer.
	BufferSize int
}

// DefaultLoggerConfig returns the default logger configuration.
func DefaultLoggerConfig() *LoggerConfig {
	return &LoggerConfig{
		BatchSize:     100,
		FlushInterval: 5 * time.Second,
		BufferSize:    1000,
	}
}

// NewSQLiteLogger creates a new SQLite-based audit logger.
func NewSQLiteLogger(db *sql.DB, config *LoggerConfig) (*SQLiteLogger, error) {
	if config == nil {
		config = DefaultLoggerConfig()
	}

	logger := &SQLiteLogger{
		db:            db,
		buffer:        make(chan *Entry, config.BufferSize),
		done:          make(chan struct{}),
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
	}

	if err := logger.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	// Start background worker
	go logger.worker()

	return logger, nil
}

// migrate creates the necessary tables.
func (l *SQLiteLogger) migrate() error {
	query := `CREATE TABLE IF NOT EXISTS audit_logs (
		id TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
		user_id TEXT,
		username TEXT,
		action TEXT NOT NULL,
		resource_type TEXT,
		resource_id TEXT,
		ip_address TEXT,
		user_agent TEXT,
		request_id TEXT,
		status TEXT NOT NULL,
		details TEXT,
		old_value TEXT,
		new_value TEXT
	)`

	if _, err := l.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create indexes
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_user_id ON audit_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs(resource_type, resource_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_status ON audit_logs(status)`,
	}

	for _, idx := range indexes {
		if _, err := l.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// worker processes entries from the buffer.
func (l *SQLiteLogger) worker() {
	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	var batch []*Entry

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := l.writeBatch(batch); err != nil {
			// Log error but don't fail
			fmt.Printf("audit: failed to write batch: %v\n", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case entry := <-l.buffer:
			batch = append(batch, entry)
			if len(batch) >= l.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-l.done:
			// Drain remaining entries
			for {
				select {
				case entry := <-l.buffer:
					batch = append(batch, entry)
				default:
					flush()
					return
				}
			}
		}
	}
}

// writeBatch writes a batch of entries to the database.
func (l *SQLiteLogger) writeBatch(entries []*Entry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := l.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO audit_logs (
		id, timestamp, user_id, username, action, resource_type, resource_id,
		ip_address, user_agent, request_id, status, details, old_value, new_value
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, entry := range entries {
		var userID interface{}
		if entry.UserID != nil {
			userID = entry.UserID.String()
		}

		_, err := stmt.Exec(
			entry.ID.String(),
			entry.Timestamp,
			userID,
			entry.Username,
			entry.Action,
			entry.ResourceType,
			entry.ResourceID,
			entry.IPAddress,
			entry.UserAgent,
			entry.RequestID,
			entry.Status,
			nullString(entry.Details),
			nullString(entry.OldValue),
			nullString(entry.NewValue),
		)
		if err != nil {
			return fmt.Errorf("failed to insert entry: %w", err)
		}
	}

	return tx.Commit()
}

// Log records an audit entry.
func (l *SQLiteLogger) Log(ctx context.Context, entry *Entry) error {
	select {
	case l.buffer <- entry:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Buffer full, write directly
		return l.writeBatch([]*Entry{entry})
	}
}

// Query retrieves audit entries based on parameters.
func (l *SQLiteLogger) Query(ctx context.Context, params *QueryParams) (*QueryResult, error) {
	// Set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}

	// Build query
	var conditions []string
	var args []interface{}

	if params.UserID != nil {
		conditions = append(conditions, "user_id = ?")
		args = append(args, params.UserID.String())
	}

	if params.Action != nil {
		conditions = append(conditions, "action = ?")
		args = append(args, *params.Action)
	}

	if params.ResourceType != "" {
		conditions = append(conditions, "resource_type = ?")
		args = append(args, params.ResourceType)
	}

	if params.ResourceID != "" {
		conditions = append(conditions, "resource_id = ?")
		args = append(args, params.ResourceID)
	}

	if params.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, *params.Status)
	}

	if params.StartTime != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, *params.StartTime)
	}

	if params.EndTime != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, *params.EndTime)
	}

	if params.IPAddress != "" {
		conditions = append(conditions, "ip_address = ?")
		args = append(args, params.IPAddress)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_logs %s", whereClause)
	var total int64
	if err := l.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count entries: %w", err)
	}

	// Build order clause
	orderBy := "timestamp"
	if params.SortBy != "" {
		allowedSorts := map[string]bool{"timestamp": true, "action": true, "status": true, "user_id": true}
		if allowedSorts[params.SortBy] {
			orderBy = params.SortBy
		}
	}
	orderDir := "DESC"
	if params.SortDir == "asc" {
		orderDir = "ASC"
	}

	// Fetch entries
	offset := (params.Page - 1) * params.PageSize
	selectQuery := fmt.Sprintf(`SELECT id, timestamp, user_id, username, action, resource_type, resource_id,
		ip_address, user_agent, request_id, status, details, old_value, new_value
		FROM audit_logs %s ORDER BY %s %s LIMIT ? OFFSET ?`,
		whereClause, orderBy, orderDir)

	args = append(args, params.PageSize, offset)
	rows, err := l.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries: %w", err)
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		entry, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate entries: %w", err)
	}

	totalPages := int(total) / params.PageSize
	if int(total)%params.PageSize > 0 {
		totalPages++
	}

	return &QueryResult{
		Entries:    entries,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetByID retrieves a single audit entry.
func (l *SQLiteLogger) GetByID(ctx context.Context, id uuid.UUID) (*Entry, error) {
	query := `SELECT id, timestamp, user_id, username, action, resource_type, resource_id,
		ip_address, user_agent, request_id, status, details, old_value, new_value
		FROM audit_logs WHERE id = ?`

	row := l.db.QueryRowContext(ctx, query, id.String())
	return scanEntryRow(row)
}

// Close closes the logger and flushes any pending entries.
func (l *SQLiteLogger) Close() error {
	close(l.done)
	return nil
}

// Helper functions

func scanEntry(rows *sql.Rows) (*Entry, error) {
	var entry Entry
	var id string
	var userID sql.NullString
	var details, oldValue, newValue sql.NullString
	var action, status string

	err := rows.Scan(
		&id,
		&entry.Timestamp,
		&userID,
		&entry.Username,
		&action,
		&entry.ResourceType,
		&entry.ResourceID,
		&entry.IPAddress,
		&entry.UserAgent,
		&entry.RequestID,
		&status,
		&details,
		&oldValue,
		&newValue,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan entry: %w", err)
	}

	entry.ID, _ = uuid.Parse(id)
	entry.Action = Action(action)
	entry.Status = Status(status)

	if userID.Valid {
		uid, _ := uuid.Parse(userID.String)
		entry.UserID = &uid
	}

	if details.Valid {
		entry.Details = []byte(details.String)
	}
	if oldValue.Valid {
		entry.OldValue = []byte(oldValue.String)
	}
	if newValue.Valid {
		entry.NewValue = []byte(newValue.String)
	}

	return &entry, nil
}

func scanEntryRow(row *sql.Row) (*Entry, error) {
	var entry Entry
	var id string
	var userID sql.NullString
	var details, oldValue, newValue sql.NullString
	var action, status string

	err := row.Scan(
		&id,
		&entry.Timestamp,
		&userID,
		&entry.Username,
		&action,
		&entry.ResourceType,
		&entry.ResourceID,
		&entry.IPAddress,
		&entry.UserAgent,
		&entry.RequestID,
		&status,
		&details,
		&oldValue,
		&newValue,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("entry not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan entry: %w", err)
	}

	entry.ID, _ = uuid.Parse(id)
	entry.Action = Action(action)
	entry.Status = Status(status)

	if userID.Valid {
		uid, _ := uuid.Parse(userID.String)
		entry.UserID = &uid
	}

	if details.Valid {
		entry.Details = []byte(details.String)
	}
	if oldValue.Valid {
		entry.OldValue = []byte(oldValue.String)
	}
	if newValue.Valid {
		entry.NewValue = []byte(newValue.String)
	}

	return &entry, nil
}

func nullString(data []byte) interface{} {
	if data == nil {
		return nil
	}
	return string(data)
}
