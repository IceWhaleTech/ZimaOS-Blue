package audit

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
)

// RetentionConfig holds configuration for audit log retention.
type RetentionConfig struct {
	// RetentionDays is the number of days to keep audit logs.
	RetentionDays int
	// CleanupInterval is how often to run the cleanup job.
	CleanupInterval time.Duration
	// BatchSize is the number of records to delete per batch.
	BatchSize int
}

// DefaultRetentionConfig returns the default retention configuration.
func DefaultRetentionConfig() *RetentionConfig {
	return &RetentionConfig{
		RetentionDays:   90,
		CleanupInterval: 24 * time.Hour,
		BatchSize:       1000,
	}
}

// RetentionManager manages audit log retention.
type RetentionManager struct {
	db      *sql.DB
	readDB  *sql.DB
	config  *RetentionConfig
	done    chan struct{}
	stopped bool
}

// NewRetentionManager creates a new retention manager.
func NewRetentionManager(db *sql.DB, config *RetentionConfig) *RetentionManager {
	return NewRetentionManagerWithReadDB(db, db, config)
}

// NewRetentionManagerWithReadDB creates a new retention manager with separate
// write and read database handles.
func NewRetentionManagerWithReadDB(writeDB, readDB *sql.DB, config *RetentionConfig) *RetentionManager {
	if config == nil {
		config = DefaultRetentionConfig()
	}
	if readDB == nil {
		readDB = writeDB
	}

	return &RetentionManager{
		db:     writeDB,
		readDB: readDB,
		config: config,
		done:   make(chan struct{}),
	}
}

func (m *RetentionManager) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, m.db, "audit_logs")
}

func (m *RetentionManager) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, m.reader(), "audit_logs")
}

func (m *RetentionManager) reader() *sql.DB {
	if m != nil && m.readDB != nil {
		return m.readDB
	}
	if m == nil {
		return nil
	}
	return m.db
}

// Start starts the retention manager background job.
func (m *RetentionManager) Start() {
	go m.run()
}

// Stop stops the retention manager.
func (m *RetentionManager) Stop() {
	if m.stopped {
		return
	}
	m.stopped = true
	close(m.done)
}

// run is the main loop for the retention manager.
func (m *RetentionManager) run() {
	// Run immediately on start
	m.cleanup()

	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.done:
			return
		}
	}
}

// cleanup removes old audit logs.
func (m *RetentionManager) cleanup() {
	ctx := context.Background()
	cutoff := timeutil.NowTime().AddDate(0, 0, -m.config.RetentionDays)

	for {
		deleted, err := m.deleteBatch(ctx, cutoff)
		if err != nil {
			log.Printf("[WARN] audit retention: failed to delete batch: %v", err)
			return
		}

		if deleted == 0 {
			break
		}

		// Small delay between batches to avoid overwhelming the database
		time.Sleep(100 * time.Millisecond)
	}
}

// deleteBatch deletes a batch of old audit logs.
func (m *RetentionManager) deleteBatch(ctx context.Context, cutoff time.Time) (int64, error) {
	n, err := m.table(ctx).Delete(
		z.Where(z.Expr(
			"id IN (SELECT id FROM audit_logs WHERE timestamp < ? LIMIT ?)",
			cutoff,
			m.config.BatchSize,
		)),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete audit logs: %w", err)
	}

	return int64(n), nil
}

// CleanupNow runs the cleanup immediately.
func (m *RetentionManager) CleanupNow() error {
	m.cleanup()
	return nil
}

// GetRetentionStats returns statistics about audit log retention.
func (m *RetentionManager) GetRetentionStats(ctx context.Context) (*RetentionStats, error) {
	stats := &RetentionStats{}

	// Get total count
	if _, err := m.readTable(ctx).Select(&stats.TotalCount, z.Fields("count(1)")); err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	var boundRows []z.V
	if _, err := m.readTable(ctx).Select(&boundRows, z.Fields("min(timestamp)", "max(timestamp)"), z.Limit(1)); err != nil {
		return nil, fmt.Errorf("failed to get retention bounds: %w", err)
	}
	if len(boundRows) > 0 {
		stats.OldestEntry = parseRetentionValue(boundRows[0], "min(timestamp)")
		stats.NewestEntry = parseRetentionValue(boundRows[0], "max(timestamp)")
	}

	// Get count of entries to be deleted
	cutoff := timeutil.NowTime().AddDate(0, 0, -m.config.RetentionDays)
	if _, err := m.readTable(ctx).Select(
		&stats.ExpiredCount,
		z.Fields("count(1)"),
		z.Where(z.Expr("timestamp < ?", cutoff)),
	); err != nil {
		return nil, fmt.Errorf("failed to get expired count: %w", err)
	}

	stats.RetentionDays = m.config.RetentionDays
	stats.CutoffDate = cutoff

	return stats, nil
}

// RetentionStats holds statistics about audit log retention.
type RetentionStats struct {
	TotalCount    int64      `json:"total_count"`
	ExpiredCount  int64      `json:"expired_count"`
	OldestEntry   *time.Time `json:"oldest_entry,omitempty"`
	NewestEntry   *time.Time `json:"newest_entry,omitempty"`
	RetentionDays int        `json:"retention_days"`
	CutoffDate    time.Time  `json:"cutoff_date"`
}

// ArchiveConfig holds configuration for audit log archiving.
type ArchiveConfig struct {
	// ArchivePath is the path to store archived logs.
	ArchivePath string
	// ArchiveAfterDays is the number of days after which to archive logs.
	ArchiveAfterDays int
	// CompressArchives enables compression of archived logs.
	CompressArchives bool
}

// DefaultArchiveConfig returns the default archive configuration.
func DefaultArchiveConfig() *ArchiveConfig {
	return &ArchiveConfig{
		ArchivePath:      "./data/audit_archives",
		ArchiveAfterDays: 30,
		CompressArchives: true,
	}
}

func parseRetentionValue(values z.V, key string) *time.Time {
	raw, ok := values[key]
	if !ok || raw == nil {
		return nil
	}
	text, ok := raw.(string)
	if !ok || text == "" {
		return nil
	}
	return parseRetentionTimePtr(&text)
}

func parseRetentionTimePtr(raw *string) *time.Time {
	if raw == nil || *raw == "" {
		return nil
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, *raw); err == nil {
			return &t
		}
	}
	return nil
}
