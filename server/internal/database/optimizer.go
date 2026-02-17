// Package database provides database optimization utilities.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/rs/zerolog"
)

// Optimizer provides query optimization and analysis capabilities.
type Optimizer struct {
	db     *sql.DB
	config OptimizerConfig
	logger zerolog.Logger

	// Query cache
	cache   map[string]*cacheEntry
	cacheMu sync.RWMutex

	// Statistics
	stats   *OptimizerStats
	statsMu sync.RWMutex
}

// cacheEntry represents a cached query result.
type cacheEntry struct {
	result    interface{}
	expiresAt int64 // unix nanos
	hits      int64
}

// OptimizerStats holds optimizer statistics.
type OptimizerStats struct {
	TotalQueries     int64         `json:"total_queries"`
	SlowQueries      int64         `json:"slow_queries"`
	CacheHits        int64         `json:"cache_hits"`
	CacheMisses      int64         `json:"cache_misses"`
	AvgQueryDuration time.Duration `json:"avg_query_duration"`
	totalDuration    time.Duration
}

// NewOptimizer creates a new query optimizer.
func NewOptimizer(db *sql.DB, config OptimizerConfig, logger zerolog.Logger) *Optimizer {
	return &Optimizer{
		db:     db,
		config: config,
		logger: logger.With().Str("component", "db-optimizer").Logger(),
		cache:  make(map[string]*cacheEntry),
		stats:  &OptimizerStats{},
	}
}

// AnalyzeQuery analyzes a query using EXPLAIN QUERY PLAN.
func (o *Optimizer) AnalyzeQuery(ctx context.Context, query string, args ...interface{}) ([]QueryPlan, error) {
	explainQuery := "EXPLAIN QUERY PLAN " + query
	rows, err := o.db.QueryContext(ctx, explainQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to explain query: %w", err)
	}
	defer rows.Close()

	var plans []QueryPlan
	for rows.Next() {
		var plan QueryPlan
		if err := rows.Scan(&plan.ID, &plan.Parent, &plan.NotUsed, &plan.Detail); err != nil {
			return nil, fmt.Errorf("failed to scan query plan: %w", err)
		}
		plans = append(plans, plan)
	}

	return plans, rows.Err()
}

// IsFullTableScan checks if a query results in a full table scan.
func (o *Optimizer) IsFullTableScan(ctx context.Context, query string, args ...interface{}) (bool, error) {
	plans, err := o.AnalyzeQuery(ctx, query, args...)
	if err != nil {
		return false, err
	}

	for _, plan := range plans {
		detail := strings.ToUpper(plan.Detail)
		// SQLite outputs "SCAN tablename" for full table scans
		// and "SEARCH tablename USING INDEX" for indexed lookups
		if strings.HasPrefix(detail, "SCAN ") && !strings.Contains(detail, "USING INDEX") {
			return true, nil
		}
	}

	return false, nil
}

// SuggestIndexes analyzes queries and suggests indexes.
func (o *Optimizer) SuggestIndexes(ctx context.Context, queries []string) ([]string, error) {
	var suggestions []string
	seen := make(map[string]bool)

	for _, query := range queries {
		// Count placeholders and provide dummy values
		placeholderCount := strings.Count(query, "?")
		args := make([]interface{}, placeholderCount)
		for i := range args {
			args[i] = "dummy" // Placeholder value for EXPLAIN QUERY PLAN
		}

		plans, err := o.AnalyzeQuery(ctx, query, args...)
		if err != nil {
			continue
		}

		for _, plan := range plans {
			detail := strings.ToUpper(plan.Detail)
			// SQLite outputs "SCAN tablename" for full table scans
			if strings.HasPrefix(detail, "SCAN ") && !strings.Contains(detail, "USING INDEX") {
				// Extract table name - it's the word after "SCAN"
				parts := strings.Fields(plan.Detail)
				if len(parts) >= 2 {
					tableName := parts[1]
					suggestion := fmt.Sprintf("Consider adding index on table '%s' for query: %s", tableName, truncateQuery(query))
					if !seen[suggestion] {
						suggestions = append(suggestions, suggestion)
						seen[suggestion] = true
					}
				}
			}
		}
	}

	return suggestions, nil
}

// GetTableStats retrieves statistics for all tables.
func (o *Optimizer) GetTableStats(ctx context.Context) ([]TableStats, error) {
	rows, err := o.db.QueryContext(ctx, `
		SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var stats []TableStats
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}

		stat, err := o.getTableStat(ctx, tableName)
		if err != nil {
			o.logger.Warn().Err(err).Str("table", tableName).Msg("Failed to get table stats")
			continue
		}
		stats = append(stats, *stat)
	}

	return stats, rows.Err()
}

// getTableStat retrieves statistics for a single table.
func (o *Optimizer) getTableStat(ctx context.Context, tableName string) (*TableStats, error) {
	stat := &TableStats{Name: tableName}

	// Get row count
	var count int64
	err := o.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to count rows: %w", err)
	}
	stat.RowCount = count

	// Get index count
	rows, err := o.db.QueryContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND tbl_name=?
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to count indexes: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		var indexCount int
		if err := rows.Scan(&indexCount); err != nil {
			return nil, fmt.Errorf("failed to scan index count: %w", err)
		}
		stat.IndexCount = indexCount
	}

	return stat, nil
}

// GetIndexes retrieves all indexes for a table.
func (o *Optimizer) GetIndexes(ctx context.Context, tableName string) ([]IndexInfo, error) {
	rows, err := o.db.QueryContext(ctx, `
		SELECT name, sql FROM sqlite_master WHERE type='index' AND tbl_name=? AND sql IS NOT NULL
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}
	defer rows.Close()

	var indexes []IndexInfo
	for rows.Next() {
		var name, sqlStr string
		if err := rows.Scan(&name, &sqlStr); err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}

		info := IndexInfo{
			Name:      name,
			TableName: tableName,
			Unique:    strings.Contains(strings.ToUpper(sqlStr), "UNIQUE"),
		}

		// Parse columns from SQL
		info.Columns = parseIndexColumns(sqlStr)
		indexes = append(indexes, info)
	}

	return indexes, rows.Err()
}

// ExecuteWithStats executes a query and returns statistics.
func (o *Optimizer) ExecuteWithStats(ctx context.Context, query string, args ...interface{}) (*QueryStats, error) {
	start := time.Now()

	// Check cache first
	if o.config.EnableQueryCache {
		cacheKey := o.getCacheKey(query, args)
		if cached := o.getFromCache(cacheKey); cached != nil {
			o.recordCacheHit()
			return &QueryStats{
				Query:    truncateQuery(query),
				Duration: time.Since(start),
			}, nil
		}
		o.recordCacheMiss()
	}

	// Execute query
	rows, err := o.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Count rows
	var rowCount int64
	for rows.Next() {
		rowCount++
	}

	duration := time.Since(start)

	// Record statistics
	o.recordQuery(duration)

	// Check for slow query
	if duration > o.config.SlowQueryThreshold {
		o.recordSlowQuery()
		if o.config.LogSlowQueries {
			o.logger.Warn().
				Str("query", truncateQuery(query)).
				Dur("duration", duration).
				Int64("rows", rowCount).
				Msg("Slow query detected")
		}
	}

	// Check if full table scan
	fullScan, _ := o.IsFullTableScan(ctx, query, args...)

	stats := &QueryStats{
		Query:        truncateQuery(query),
		Duration:     duration,
		RowsReturned: rowCount,
		FullScan:     fullScan,
	}

	return stats, rows.Err()
}

// GetStats returns optimizer statistics.
func (o *Optimizer) GetStats() OptimizerStats {
	o.statsMu.RLock()
	defer o.statsMu.RUnlock()

	stats := *o.stats
	if stats.TotalQueries > 0 {
		stats.AvgQueryDuration = stats.totalDuration / time.Duration(stats.TotalQueries)
	}
	return stats
}

// ClearCache clears the query cache.
func (o *Optimizer) ClearCache() {
	o.cacheMu.Lock()
	defer o.cacheMu.Unlock()
	o.cache = make(map[string]*cacheEntry)
}

// getCacheKey generates a cache key for a query.
func (o *Optimizer) getCacheKey(query string, args []interface{}) string {
	return fmt.Sprintf("%s:%v", query, args)
}

// getFromCache retrieves a cached result.
func (o *Optimizer) getFromCache(key string) interface{} {
	o.cacheMu.RLock()
	defer o.cacheMu.RUnlock()

	entry, ok := o.cache[key]
	if !ok || timeutil.NowNano() > entry.expiresAt {
		return nil
	}

	entry.hits++
	return entry.result
}

// setCache stores a result in the cache.
func (o *Optimizer) setCache(key string, result interface{}) {
	o.cacheMu.Lock()
	defer o.cacheMu.Unlock()

	// Evict if cache is full
	if len(o.cache) >= o.config.MaxCachedQueries {
		o.evictOldest()
	}

	o.cache[key] = &cacheEntry{
		result:    result,
		expiresAt: timeutil.NowNano() + int64(o.config.QueryCacheTTL),
	}
}

// evictOldest removes the oldest cache entry.
func (o *Optimizer) evictOldest() {
	var oldestKey string
	var oldestTime int64

	for key, entry := range o.cache {
		if oldestKey == "" || entry.expiresAt < oldestTime {
			oldestKey = key
			oldestTime = entry.expiresAt
		}
	}

	if oldestKey != "" {
		delete(o.cache, oldestKey)
	}
}

// recordQuery records a query execution.
func (o *Optimizer) recordQuery(duration time.Duration) {
	o.statsMu.Lock()
	defer o.statsMu.Unlock()
	o.stats.TotalQueries++
	o.stats.totalDuration += duration
}

// recordSlowQuery records a slow query.
func (o *Optimizer) recordSlowQuery() {
	o.statsMu.Lock()
	defer o.statsMu.Unlock()
	o.stats.SlowQueries++
}

// recordCacheHit records a cache hit.
func (o *Optimizer) recordCacheHit() {
	o.statsMu.Lock()
	defer o.statsMu.Unlock()
	o.stats.CacheHits++
}

// recordCacheMiss records a cache miss.
func (o *Optimizer) recordCacheMiss() {
	o.statsMu.Lock()
	defer o.statsMu.Unlock()
	o.stats.CacheMisses++
}

// parseIndexColumns extracts column names from an index SQL statement.
func parseIndexColumns(sql string) []string {
	// Find content between parentheses
	start := strings.Index(sql, "(")
	end := strings.LastIndex(sql, ")")
	if start == -1 || end == -1 || start >= end {
		return nil
	}

	columnsStr := sql[start+1 : end]
	parts := strings.Split(columnsStr, ",")

	var columns []string
	for _, part := range parts {
		col := strings.TrimSpace(part)
		// Remove ASC/DESC
		col = strings.TrimSuffix(col, " ASC")
		col = strings.TrimSuffix(col, " DESC")
		col = strings.TrimSuffix(col, " asc")
		col = strings.TrimSuffix(col, " desc")
		if col != "" {
			columns = append(columns, col)
		}
	}

	return columns
}

// truncateQuery truncates a query for logging.
func truncateQuery(query string) string {
	query = strings.TrimSpace(query)
	query = strings.ReplaceAll(query, "\n", " ")
	query = strings.ReplaceAll(query, "\t", " ")

	if len(query) > 200 {
		return query[:200] + "..."
	}
	return query
}
