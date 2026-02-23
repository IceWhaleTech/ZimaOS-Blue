package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// BatchExecutor provides batch database operations.
type BatchExecutor struct {
	db     *sql.DB
	config BatchConfig
	logger zerolog.Logger

	// Statistics
	stats   *BatchStats
	statsMu sync.RWMutex
}

// BatchStats holds batch operation statistics.
type BatchStats struct {
	TotalBatches     int64         `json:"total_batches"`
	TotalRows        int64         `json:"total_rows"`
	FailedBatches    int64         `json:"failed_batches"`
	RetriedBatches   int64         `json:"retried_batches"`
	AvgBatchDuration time.Duration `json:"avg_batch_duration"`
	totalDuration    time.Duration
}

// BatchResult represents the result of a batch operation.
type BatchResult struct {
	TotalRows    int64         `json:"total_rows"`
	AffectedRows int64         `json:"affected_rows"`
	FailedRows   int64         `json:"failed_rows"`
	Duration     time.Duration `json:"duration"`
	Batches      int           `json:"batches"`
	Errors       []error       `json:"errors,omitempty"`
}

// NewBatchExecutor creates a new batch executor.
func NewBatchExecutor(db *sql.DB, config BatchConfig, logger zerolog.Logger) *BatchExecutor {
	return &BatchExecutor{
		db:     db,
		config: config,
		logger: logger.With().Str("component", "batch-executor").Logger(),
		stats:  &BatchStats{},
	}
}

// BatchInsert performs a batch insert operation.
func (b *BatchExecutor) BatchInsert(ctx context.Context, table string, columns []string, values [][]interface{}) (*BatchResult, error) {
	if len(values) == 0 {
		return &BatchResult{}, nil
	}

	start := timeutil.NowTime()
	result := &BatchResult{
		TotalRows: int64(len(values)),
	}

	// Process in batches
	for i := 0; i < len(values); i += b.config.BatchSize {
		end := i + b.config.BatchSize
		if end > len(values) {
			end = len(values)
		}

		batch := values[i:end]
		affected, err := b.executeBatchInsert(ctx, table, columns, batch)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("batch %d: %w", result.Batches, err))
			result.FailedRows += int64(len(batch))
			b.recordFailedBatch()
		} else {
			result.AffectedRows += affected
		}
		result.Batches++
		b.recordBatch(timeutil.SinceTime(start))
	}

	result.Duration = timeutil.SinceTime(start)
	return result, nil
}

// executeBatchInsert executes a single batch insert with retry.
func (b *BatchExecutor) executeBatchInsert(ctx context.Context, table string, columns []string, values [][]interface{}) (int64, error) {
	var lastErr error

	for attempt := 0; attempt <= b.config.MaxRetries; attempt++ {
		if attempt > 0 {
			b.recordRetry()
			time.Sleep(b.config.RetryDelay * time.Duration(attempt))
		}

		affected, err := b.doInsert(ctx, table, columns, values)
		if err == nil {
			return affected, nil
		}
		lastErr = err

		b.logger.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Int("max_retries", b.config.MaxRetries).
			Msg("Batch insert failed, retrying")
	}

	return 0, lastErr
}

// doInsert performs the actual insert operation.
func (b *BatchExecutor) doInsert(ctx context.Context, table string, columns []string, values [][]interface{}) (int64, error) {
	if len(values) == 0 {
		return 0, nil
	}

	// Build query
	placeholders := make([]string, len(values))
	singlePlaceholder := "(" + strings.Repeat("?,", len(columns)-1) + "?)"
	for i := range placeholders {
		placeholders[i] = singlePlaceholder
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES %s",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	// Flatten values
	args := make([]interface{}, 0, len(values)*len(columns))
	for _, row := range values {
		args = append(args, row...)
	}

	// Execute in transaction
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute insert: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	affected, _ := result.RowsAffected()
	return affected, nil
}

// BatchUpdate performs a batch update operation.
func (b *BatchExecutor) BatchUpdate(ctx context.Context, table string, updates []BatchUpdate) (*BatchResult, error) {
	if len(updates) == 0 {
		return &BatchResult{}, nil
	}

	start := timeutil.NowTime()
	result := &BatchResult{
		TotalRows: int64(len(updates)),
	}

	// Process in batches
	for i := 0; i < len(updates); i += b.config.BatchSize {
		end := i + b.config.BatchSize
		if end > len(updates) {
			end = len(updates)
		}

		batch := updates[i:end]
		affected, err := b.executeBatchUpdate(ctx, table, batch)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("batch %d: %w", result.Batches, err))
			result.FailedRows += int64(len(batch))
			b.recordFailedBatch()
		} else {
			result.AffectedRows += affected
		}
		result.Batches++
		b.recordBatch(timeutil.SinceTime(start))
	}

	result.Duration = timeutil.SinceTime(start)
	return result, nil
}

// BatchUpdate represents a single update operation.
type BatchUpdate struct {
	// Set contains column-value pairs to update.
	Set map[string]interface{}
	// Where contains the WHERE clause conditions.
	Where map[string]interface{}
}

// executeBatchUpdate executes a single batch update with retry.
func (b *BatchExecutor) executeBatchUpdate(ctx context.Context, table string, updates []BatchUpdate) (int64, error) {
	var lastErr error

	for attempt := 0; attempt <= b.config.MaxRetries; attempt++ {
		if attempt > 0 {
			b.recordRetry()
			time.Sleep(b.config.RetryDelay * time.Duration(attempt))
		}

		affected, err := b.doUpdate(ctx, table, updates)
		if err == nil {
			return affected, nil
		}
		lastErr = err

		b.logger.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Int("max_retries", b.config.MaxRetries).
			Msg("Batch update failed, retrying")
	}

	return 0, lastErr
}

// doUpdate performs the actual update operation.
func (b *BatchExecutor) doUpdate(ctx context.Context, table string, updates []BatchUpdate) (int64, error) {
	if len(updates) == 0 {
		return 0, nil
	}

	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var totalAffected int64
	for _, update := range updates {
		// Build SET clause
		setClauses := make([]string, 0, len(update.Set))
		setArgs := make([]interface{}, 0, len(update.Set))
		for col, val := range update.Set {
			setClauses = append(setClauses, col+" = ?")
			setArgs = append(setArgs, val)
		}

		// Build WHERE clause
		whereClauses := make([]string, 0, len(update.Where))
		whereArgs := make([]interface{}, 0, len(update.Where))
		for col, val := range update.Where {
			whereClauses = append(whereClauses, col+" = ?")
			whereArgs = append(whereArgs, val)
		}

		query := fmt.Sprintf(
			"UPDATE %s SET %s WHERE %s",
			table,
			strings.Join(setClauses, ", "),
			strings.Join(whereClauses, " AND "),
		)

		args := append(setArgs, whereArgs...)
		result, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return totalAffected, fmt.Errorf("failed to execute update: %w", err)
		}

		affected, _ := result.RowsAffected()
		totalAffected += affected
	}

	if err := tx.Commit(); err != nil {
		return totalAffected, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return totalAffected, nil
}

// BatchDelete performs a batch delete operation with chunking.
func (b *BatchExecutor) BatchDelete(ctx context.Context, table string, whereColumn string, values []interface{}) (*BatchResult, error) {
	if len(values) == 0 {
		return &BatchResult{}, nil
	}

	start := timeutil.NowTime()
	result := &BatchResult{
		TotalRows: int64(len(values)),
	}

	// Process in batches
	for i := 0; i < len(values); i += b.config.BatchSize {
		end := i + b.config.BatchSize
		if end > len(values) {
			end = len(values)
		}

		batch := values[i:end]
		affected, err := b.executeBatchDelete(ctx, table, whereColumn, batch)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("batch %d: %w", result.Batches, err))
			result.FailedRows += int64(len(batch))
			b.recordFailedBatch()
		} else {
			result.AffectedRows += affected
		}
		result.Batches++
		b.recordBatch(timeutil.SinceTime(start))
	}

	result.Duration = timeutil.SinceTime(start)
	return result, nil
}

// executeBatchDelete executes a single batch delete with retry.
func (b *BatchExecutor) executeBatchDelete(ctx context.Context, table string, whereColumn string, values []interface{}) (int64, error) {
	var lastErr error

	for attempt := 0; attempt <= b.config.MaxRetries; attempt++ {
		if attempt > 0 {
			b.recordRetry()
			time.Sleep(b.config.RetryDelay * time.Duration(attempt))
		}

		affected, err := b.doDelete(ctx, table, whereColumn, values)
		if err == nil {
			return affected, nil
		}
		lastErr = err

		b.logger.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Int("max_retries", b.config.MaxRetries).
			Msg("Batch delete failed, retrying")
	}

	return 0, lastErr
}

// doDelete performs the actual delete operation.
func (b *BatchExecutor) doDelete(ctx context.Context, table string, whereColumn string, values []interface{}) (int64, error) {
	if len(values) == 0 {
		return 0, nil
	}

	// Build query with IN clause
	placeholders := strings.Repeat("?,", len(values)-1) + "?"
	query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)", table, whereColumn, placeholders)

	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, query, values...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute delete: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	affected, _ := result.RowsAffected()
	return affected, nil
}

// BatchDeleteWithCondition performs a batch delete with custom conditions.
func (b *BatchExecutor) BatchDeleteWithCondition(ctx context.Context, table string, conditions []map[string]interface{}) (*BatchResult, error) {
	if len(conditions) == 0 {
		return &BatchResult{}, nil
	}

	start := timeutil.NowTime()
	result := &BatchResult{
		TotalRows: int64(len(conditions)),
	}

	// Process in batches
	for i := 0; i < len(conditions); i += b.config.BatchSize {
		end := i + b.config.BatchSize
		if end > len(conditions) {
			end = len(conditions)
		}

		batch := conditions[i:end]
		affected, err := b.doDeleteWithConditions(ctx, table, batch)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("batch %d: %w", result.Batches, err))
			result.FailedRows += int64(len(batch))
			b.recordFailedBatch()
		} else {
			result.AffectedRows += affected
		}
		result.Batches++
		b.recordBatch(timeutil.SinceTime(start))
	}

	result.Duration = timeutil.SinceTime(start)
	return result, nil
}

// doDeleteWithConditions performs delete with multiple conditions.
func (b *BatchExecutor) doDeleteWithConditions(ctx context.Context, table string, conditions []map[string]interface{}) (int64, error) {
	if len(conditions) == 0 {
		return 0, nil
	}

	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var totalAffected int64
	for _, cond := range conditions {
		whereClauses := make([]string, 0, len(cond))
		args := make([]interface{}, 0, len(cond))
		for col, val := range cond {
			whereClauses = append(whereClauses, col+" = ?")
			args = append(args, val)
		}

		query := fmt.Sprintf("DELETE FROM %s WHERE %s", table, strings.Join(whereClauses, " AND "))
		result, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return totalAffected, fmt.Errorf("failed to execute delete: %w", err)
		}

		affected, _ := result.RowsAffected()
		totalAffected += affected
	}

	if err := tx.Commit(); err != nil {
		return totalAffected, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return totalAffected, nil
}

// GetStats returns batch executor statistics.
func (b *BatchExecutor) GetStats() BatchStats {
	b.statsMu.RLock()
	defer b.statsMu.RUnlock()

	stats := *b.stats
	if stats.TotalBatches > 0 {
		stats.AvgBatchDuration = stats.totalDuration / time.Duration(stats.TotalBatches)
	}
	return stats
}

// recordBatch records a batch execution.
func (b *BatchExecutor) recordBatch(duration time.Duration) {
	b.statsMu.Lock()
	defer b.statsMu.Unlock()
	b.stats.TotalBatches++
	b.stats.totalDuration += duration
}

// recordFailedBatch records a failed batch.
func (b *BatchExecutor) recordFailedBatch() {
	b.statsMu.Lock()
	defer b.statsMu.Unlock()
	b.stats.FailedBatches++
}

// recordRetry records a retry attempt.
func (b *BatchExecutor) recordRetry() {
	b.statsMu.Lock()
	defer b.statsMu.Unlock()
	b.stats.RetriedBatches++
}
