package audit

import (
	"context"
	"database/sql"
	"fmt"
	"time"
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
	db     *sql.DB
	config *RetentionConfig
	done   chan struct{}
}

// NewRetentionManager creates a new retention manager.
func NewRetentionManager(db *sql.DB, config *RetentionConfig) *RetentionManager {
	if config == nil {
		config = DefaultRetentionConfig()
	}

	return &RetentionManager{
		db:     db,
		config: config,
		done:   make(chan struct{}),
	}
}

// Start starts the retention manager background job.
func (m *RetentionManager) Start() {
	go m.run()
}

// Stop stops the retention manager.
func (m *RetentionManager) Stop() {
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
	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)

	for {
		deleted, err := m.deleteBatch(ctx, cutoff)
		if err != nil {
			fmt.Printf("audit retention: failed to delete batch: %v\n", err)
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
	query := `DELETE FROM audit_logs WHERE id IN (
		SELECT id FROM audit_logs WHERE timestamp < ? LIMIT ?
	)`

	result, err := m.db.ExecContext(ctx, query, cutoff, m.config.BatchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to delete audit logs: %w", err)
	}

	return result.RowsAffected()
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
	err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs").Scan(&stats.TotalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	// Get oldest entry
	var oldest sql.NullTime
	err = m.db.QueryRowContext(ctx, "SELECT MIN(timestamp) FROM audit_logs").Scan(&oldest)
	if err != nil {
		return nil, fmt.Errorf("failed to get oldest entry: %w", err)
	}
	if oldest.Valid {
		stats.OldestEntry = &oldest.Time
	}

	// Get newest entry
	var newest sql.NullTime
	err = m.db.QueryRowContext(ctx, "SELECT MAX(timestamp) FROM audit_logs").Scan(&newest)
	if err != nil {
		return nil, fmt.Errorf("failed to get newest entry: %w", err)
	}
	if newest.Valid {
		stats.NewestEntry = &newest.Time
	}

	// Get count of entries to be deleted
	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)
	err = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs WHERE timestamp < ?", cutoff).Scan(&stats.ExpiredCount)
	if err != nil {
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
