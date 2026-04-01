package audit

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	z "github.com/IceWhaleTech/zorm"
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
	db            *sql.DB
	readDB        *sql.DB
	buffer        chan *Entry
	done          chan struct{}
	batchSize     int
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
	return NewSQLiteLoggerWithReadDB(db, db, config)
}

// NewSQLiteLoggerWithReadDB creates a new SQLite-based audit logger with
// separate write and read database handles.
func NewSQLiteLoggerWithReadDB(writeDB, readDB *sql.DB, config *LoggerConfig) (*SQLiteLogger, error) {
	if config == nil {
		config = DefaultLoggerConfig()
	}
	if writeDB == nil {
		return nil, fmt.Errorf("audit db is required")
	}
	if readDB == nil {
		readDB = writeDB
	}

	logger := &SQLiteLogger{
		db:            writeDB,
		readDB:        readDB,
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

type auditLogRow struct {
	ID           string    `json:"id" zorm:"id"`
	Timestamp    time.Time `json:"timestamp" zorm:"timestamp"`
	UserID       *string   `json:"user_id" zorm:"user_id"`
	Username     *string   `json:"username" zorm:"username"`
	Action       string    `json:"action" zorm:"action"`
	ResourceType *string   `json:"resource_type" zorm:"resource_type"`
	ResourceID   *string   `json:"resource_id" zorm:"resource_id"`
	IPAddress    *string   `json:"ip_address" zorm:"ip_address"`
	UserAgent    *string   `json:"user_agent" zorm:"user_agent"`
	RequestID    *string   `json:"request_id" zorm:"request_id"`
	Status       string    `json:"status" zorm:"status"`
	Details      *string   `json:"details" zorm:"details"`
	OldValue     *string   `json:"old_value" zorm:"old_value"`
	NewValue     *string   `json:"new_value" zorm:"new_value"`
}

func (l *SQLiteLogger) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, l.db, "audit_logs")
}

func (l *SQLiteLogger) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, l.reader(), "audit_logs")
}

func (l *SQLiteLogger) reader() *sql.DB {
	if l != nil && l.readDB != nil {
		return l.readDB
	}
	if l == nil {
		return nil
	}
	return l.db
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
			log.Printf("[WARN] audit: failed to write batch: %v", err)
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
	table := z.Table(tx, "audit_logs")

	for _, entry := range entries {
		if _, err := table.Insert(auditEntryValues(entry)); err != nil {
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
	if params == nil {
		params = &QueryParams{}
	}
	// Set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}

	conditions := buildAuditQueryConditions(params)

	var total int64
	countOpts := []z.ZormItem{z.Fields("count(1)")}
	if len(conditions) > 0 {
		countOpts = append(countOpts, z.Where(conditions...))
	}
	if _, err := l.readTable(ctx).Select(&total, countOpts...); err != nil {
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
	selectOpts := make([]z.ZormItem, 0, 3)
	if len(conditions) > 0 {
		selectOpts = append(selectOpts, z.Where(conditions...))
	}
	selectOpts = append(selectOpts,
		z.OrderBy(fmt.Sprintf("%s %s", orderBy, orderDir)),
		z.Limit(params.PageSize, offset),
	)

	var rows []auditLogRow
	if _, err := l.readTable(ctx).Select(&rows, selectOpts...); err != nil {
		return nil, fmt.Errorf("failed to query entries: %w", err)
	}

	entries := make([]*Entry, 0, len(rows))
	for i := range rows {
		entries = append(entries, rowToEntry(rows[i]))
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
	var rows []auditLogRow
	_, err := l.readTable(ctx).Select(&rows,
		z.Where(z.Eq("id", id.String())),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query entry: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("entry not found")
	}
	return rowToEntry(rows[0]), nil
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

func auditEntryValues(entry *Entry) z.V {
	var userID interface{}
	if entry != nil && entry.UserID != nil {
		userID = entry.UserID.String()
	}
	return z.V{
		"id":            entry.ID.String(),
		"timestamp":     entry.Timestamp,
		"user_id":       userID,
		"username":      entry.Username,
		"action":        entry.Action,
		"resource_type": entry.ResourceType,
		"resource_id":   entry.ResourceID,
		"ip_address":    entry.IPAddress,
		"user_agent":    entry.UserAgent,
		"request_id":    entry.RequestID,
		"status":        entry.Status,
		"details":       nullString(entry.Details),
		"old_value":     nullString(entry.OldValue),
		"new_value":     nullString(entry.NewValue),
	}
}

func buildAuditQueryConditions(params *QueryParams) []interface{} {
	conditions := make([]interface{}, 0, 8)
	if params.UserID != nil {
		conditions = append(conditions, z.Eq("user_id", params.UserID.String()))
	}
	if params.Action != nil {
		conditions = append(conditions, z.Eq("action", string(*params.Action)))
	}
	if params.ResourceType != "" {
		conditions = append(conditions, z.Eq("resource_type", params.ResourceType))
	}
	if params.ResourceID != "" {
		conditions = append(conditions, z.Eq("resource_id", params.ResourceID))
	}
	if params.Status != nil {
		conditions = append(conditions, z.Eq("status", string(*params.Status)))
	}
	if params.StartTime != nil {
		conditions = append(conditions, z.Expr("timestamp >= ?", *params.StartTime))
	}
	if params.EndTime != nil {
		conditions = append(conditions, z.Expr("timestamp <= ?", *params.EndTime))
	}
	if params.IPAddress != "" {
		conditions = append(conditions, z.Eq("ip_address", params.IPAddress))
	}
	return conditions
}

func rowToEntry(row auditLogRow) *Entry {
	entry := &Entry{
		Timestamp: row.Timestamp,
		Action:    Action(row.Action),
		Status:    Status(row.Status),
	}
	entry.ID, _ = uuid.Parse(row.ID)
	if row.UserID != nil {
		if uid, err := uuid.Parse(*row.UserID); err == nil {
			entry.UserID = &uid
		}
	}
	if row.Username != nil {
		entry.Username = *row.Username
	}
	if row.ResourceType != nil {
		entry.ResourceType = *row.ResourceType
	}
	if row.ResourceID != nil {
		entry.ResourceID = *row.ResourceID
	}
	if row.IPAddress != nil {
		entry.IPAddress = *row.IPAddress
	}
	if row.UserAgent != nil {
		entry.UserAgent = *row.UserAgent
	}
	if row.RequestID != nil {
		entry.RequestID = *row.RequestID
	}
	if row.Details != nil {
		entry.Details = []byte(*row.Details)
	}
	if row.OldValue != nil {
		entry.OldValue = []byte(*row.OldValue)
	}
	if row.NewValue != nil {
		entry.NewValue = []byte(*row.NewValue)
	}
	return entry
}
