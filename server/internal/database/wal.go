package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/rs/zerolog"
)

// WALManager manages SQLite WAL mode configuration and checkpointing.
type WALManager struct {
	db     *sql.DB
	config WALConfig
	logger zerolog.Logger

	// Checkpoint management
	stopCh   chan struct{}
	wg       sync.WaitGroup
	running  bool
	runningMu sync.Mutex

	// Statistics
	stats   *WALStats
	statsMu sync.RWMutex
}

// WALStats holds WAL statistics.
type WALStats struct {
	TotalCheckpoints   int64         `json:"total_checkpoints"`
	SuccessCheckpoints int64         `json:"success_checkpoints"`
	FailedCheckpoints  int64         `json:"failed_checkpoints"`
	LastCheckpoint     time.Time     `json:"last_checkpoint"`
	AvgCheckpointTime  time.Duration `json:"avg_checkpoint_time"`
	WALSize            int64         `json:"wal_size"`
	totalDuration      time.Duration
}

// WALInfo holds current WAL state information.
type WALInfo struct {
	JournalMode     string `json:"journal_mode"`
	WALCheckpoint   int    `json:"wal_checkpoint"`
	PageSize        int    `json:"page_size"`
	CacheSize       int    `json:"cache_size"`
	SynchronousMode string `json:"synchronous_mode"`
	BusyTimeout     int    `json:"busy_timeout"`
}

// NewWALManager creates a new WAL manager.
func NewWALManager(db *sql.DB, config WALConfig, logger zerolog.Logger) *WALManager {
	return &WALManager{
		db:     db,
		config: config,
		logger: logger.With().Str("component", "wal-manager").Logger(),
		stopCh: make(chan struct{}),
		stats:  &WALStats{},
	}
}

// Configure applies WAL configuration to the database.
func (w *WALManager) Configure(ctx context.Context) error {
	if !w.config.Enabled {
		w.logger.Info().Msg("WAL mode disabled")
		return nil
	}

	// Enable WAL mode
	if _, err := w.db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Set page size (must be done before any data is written)
	if w.config.PageSize > 0 {
		if _, err := w.db.ExecContext(ctx, fmt.Sprintf("PRAGMA page_size=%d", w.config.PageSize)); err != nil {
			w.logger.Warn().Err(err).Msg("Failed to set page size (may already have data)")
		}
	}

	// Set cache size
	if w.config.CacheSize != 0 {
		if _, err := w.db.ExecContext(ctx, fmt.Sprintf("PRAGMA cache_size=%d", w.config.CacheSize)); err != nil {
			return fmt.Errorf("failed to set cache size: %w", err)
		}
	}

	// Set synchronous mode
	if w.config.SynchronousMode != "" {
		if _, err := w.db.ExecContext(ctx, fmt.Sprintf("PRAGMA synchronous=%s", w.config.SynchronousMode)); err != nil {
			return fmt.Errorf("failed to set synchronous mode: %w", err)
		}
	}

	// Set busy timeout
	if w.config.BusyTimeout > 0 {
		if _, err := w.db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d", w.config.BusyTimeout)); err != nil {
			return fmt.Errorf("failed to set busy timeout: %w", err)
		}
	}

	// Set WAL autocheckpoint threshold
	if w.config.CheckpointThreshold > 0 {
		if _, err := w.db.ExecContext(ctx, fmt.Sprintf("PRAGMA wal_autocheckpoint=%d", w.config.CheckpointThreshold)); err != nil {
			return fmt.Errorf("failed to set wal_autocheckpoint: %w", err)
		}
	}

	w.logger.Info().
		Int("page_size", w.config.PageSize).
		Int("cache_size", w.config.CacheSize).
		Str("synchronous", w.config.SynchronousMode).
		Int("busy_timeout", w.config.BusyTimeout).
		Int("checkpoint_threshold", w.config.CheckpointThreshold).
		Msg("WAL mode configured")

	return nil
}

// StartAutoCheckpoint starts automatic checkpointing.
func (w *WALManager) StartAutoCheckpoint() {
	w.runningMu.Lock()
	defer w.runningMu.Unlock()

	if w.running {
		return
	}

	w.running = true
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()

		ticker := time.NewTicker(w.config.CheckpointInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := w.Checkpoint(ctx, CheckpointPassive); err != nil {
					w.logger.Warn().Err(err).Msg("Auto checkpoint failed")
				}
				cancel()
			case <-w.stopCh:
				return
			}
		}
	}()

	w.logger.Info().
		Dur("interval", w.config.CheckpointInterval).
		Msg("Auto checkpoint started")
}

// StopAutoCheckpoint stops automatic checkpointing.
func (w *WALManager) StopAutoCheckpoint() {
	w.runningMu.Lock()
	defer w.runningMu.Unlock()

	if !w.running {
		return
	}

	close(w.stopCh)
	w.wg.Wait()
	w.running = false
	w.stopCh = make(chan struct{})

	w.logger.Info().Msg("Auto checkpoint stopped")
}

// CheckpointMode represents the checkpoint mode.
type CheckpointMode string

const (
	// CheckpointPassive checkpoints as many frames as possible without waiting.
	CheckpointPassive CheckpointMode = "PASSIVE"
	// CheckpointFull checkpoints all frames, waiting for readers.
	CheckpointFull CheckpointMode = "FULL"
	// CheckpointRestart like FULL but also truncates the WAL file.
	CheckpointRestart CheckpointMode = "RESTART"
	// CheckpointTruncate like RESTART but also truncates the WAL file to zero bytes.
	CheckpointTruncate CheckpointMode = "TRUNCATE"
)

// Checkpoint performs a WAL checkpoint.
func (w *WALManager) Checkpoint(ctx context.Context, mode CheckpointMode) error {
	start := timeutil.NowTime()

	query := fmt.Sprintf("PRAGMA wal_checkpoint(%s)", mode)
	rows, err := w.db.QueryContext(ctx, query)
	if err != nil {
		w.recordFailedCheckpoint()
		return fmt.Errorf("checkpoint failed: %w", err)
	}
	defer rows.Close()

	var busy, log, checkpointed int
	if rows.Next() {
		if err := rows.Scan(&busy, &log, &checkpointed); err != nil {
			w.recordFailedCheckpoint()
			return fmt.Errorf("failed to scan checkpoint result: %w", err)
		}
	}

	duration := timeutil.SinceTime(start)
	w.recordSuccessCheckpoint(duration)

	w.logger.Debug().
		Str("mode", string(mode)).
		Int("busy", busy).
		Int("log", log).
		Int("checkpointed", checkpointed).
		Dur("duration", duration).
		Msg("Checkpoint completed")

	return nil
}

// GetInfo returns current WAL configuration info.
func (w *WALManager) GetInfo(ctx context.Context) (*WALInfo, error) {
	info := &WALInfo{}

	// Get journal mode
	if err := w.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&info.JournalMode); err != nil {
		return nil, fmt.Errorf("failed to get journal_mode: %w", err)
	}

	// Get page size
	if err := w.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&info.PageSize); err != nil {
		return nil, fmt.Errorf("failed to get page_size: %w", err)
	}

	// Get cache size
	if err := w.db.QueryRowContext(ctx, "PRAGMA cache_size").Scan(&info.CacheSize); err != nil {
		return nil, fmt.Errorf("failed to get cache_size: %w", err)
	}

	// Get synchronous mode
	var syncMode int
	if err := w.db.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&syncMode); err != nil {
		return nil, fmt.Errorf("failed to get synchronous: %w", err)
	}
	switch syncMode {
	case 0:
		info.SynchronousMode = "OFF"
	case 1:
		info.SynchronousMode = "NORMAL"
	case 2:
		info.SynchronousMode = "FULL"
	case 3:
		info.SynchronousMode = "EXTRA"
	}

	// Get busy timeout
	if err := w.db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&info.BusyTimeout); err != nil {
		return nil, fmt.Errorf("failed to get busy_timeout: %w", err)
	}

	// Get wal_autocheckpoint
	if err := w.db.QueryRowContext(ctx, "PRAGMA wal_autocheckpoint").Scan(&info.WALCheckpoint); err != nil {
		return nil, fmt.Errorf("failed to get wal_autocheckpoint: %w", err)
	}

	return info, nil
}

// GetStats returns WAL statistics.
func (w *WALManager) GetStats() WALStats {
	w.statsMu.RLock()
	defer w.statsMu.RUnlock()

	stats := *w.stats
	if stats.SuccessCheckpoints > 0 {
		stats.AvgCheckpointTime = stats.totalDuration / time.Duration(stats.SuccessCheckpoints)
	}
	return stats
}

// recordSuccessCheckpoint records a successful checkpoint.
func (w *WALManager) recordSuccessCheckpoint(duration time.Duration) {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	w.stats.TotalCheckpoints++
	w.stats.SuccessCheckpoints++
	w.stats.LastCheckpoint = timeutil.NowTime()
	w.stats.totalDuration += duration
}

// recordFailedCheckpoint records a failed checkpoint.
func (w *WALManager) recordFailedCheckpoint() {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	w.stats.TotalCheckpoints++
	w.stats.FailedCheckpoints++
}

// Optimize runs VACUUM and ANALYZE to optimize the database.
func (w *WALManager) Optimize(ctx context.Context) error {
	w.logger.Info().Msg("Starting database optimization")

	// Run ANALYZE to update statistics
	if _, err := w.db.ExecContext(ctx, "ANALYZE"); err != nil {
		return fmt.Errorf("ANALYZE failed: %w", err)
	}

	// Run VACUUM to reclaim space (this may take a while)
	if _, err := w.db.ExecContext(ctx, "VACUUM"); err != nil {
		return fmt.Errorf("VACUUM failed: %w", err)
	}

	w.logger.Info().Msg("Database optimization completed")
	return nil
}

// IntegrityCheck runs an integrity check on the database.
func (w *WALManager) IntegrityCheck(ctx context.Context) ([]string, error) {
	rows, err := w.db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return nil, fmt.Errorf("integrity check failed: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, result)
	}

	return results, rows.Err()
}
