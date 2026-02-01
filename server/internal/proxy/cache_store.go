package proxy

import (
	"database/sql"
	"encoding/json"
	"os"
	"time"
)

// SQLiteStatsStore implements StatsStore using SQLite
type SQLiteStatsStore struct {
	db *sql.DB
}

// NewSQLiteStatsStore creates a new SQLite-based stats store
func NewSQLiteStatsStore(db *sql.DB) (*SQLiteStatsStore, error) {
	store := &SQLiteStatsStore{db: db}
	if err := store.initSchema(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStatsStore) initSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS cache_stats (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			hits INTEGER DEFAULT 0,
			misses INTEGER DEFAULT 0,
			evictions INTEGER DEFAULT 0,
			bypasses INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// SaveCacheStats saves cache statistics to the database
func (s *SQLiteStatsStore) SaveCacheStats(stats *CacheStats) error {
	_, err := s.db.Exec(`
		INSERT INTO cache_stats (id, hits, misses, evictions, bypasses, updated_at)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			hits = excluded.hits,
			misses = excluded.misses,
			evictions = excluded.evictions,
			bypasses = excluded.bypasses,
			updated_at = excluded.updated_at
	`, stats.Hits, stats.Misses, stats.Evictions, stats.Bypasses, stats.UpdatedAt)
	return err
}

// LoadCacheStats loads cache statistics from the database
func (s *SQLiteStatsStore) LoadCacheStats() (*CacheStats, error) {
	var stats CacheStats
	var updatedAt string
	err := s.db.QueryRow(`
		SELECT hits, misses, evictions, bypasses, updated_at
		FROM cache_stats WHERE id = 1
	`).Scan(&stats.Hits, &stats.Misses, &stats.Evictions, &stats.Bypasses, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	stats.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &stats, nil
}

// JSONFileStatsStore implements StatsStore using a JSON file (fallback)
type JSONFileStatsStore struct {
	filePath string
}

// NewJSONFileStatsStore creates a new JSON file-based stats store
func NewJSONFileStatsStore(filePath string) *JSONFileStatsStore {
	return &JSONFileStatsStore{filePath: filePath}
}

// SaveCacheStats saves cache statistics to a JSON file
func (s *JSONFileStatsStore) SaveCacheStats(stats *CacheStats) error {
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(s.filePath, data)
}

// LoadCacheStats loads cache statistics from a JSON file
func (s *JSONFileStatsStore) LoadCacheStats() (*CacheStats, error) {
	data, err := readFile(s.filePath)
	if err != nil {
		return nil, nil // File doesn't exist, return nil
	}
	var stats CacheStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// Helper functions for file operations
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
