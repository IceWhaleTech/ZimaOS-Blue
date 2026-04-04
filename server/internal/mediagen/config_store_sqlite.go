package mediagen

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	z "github.com/IceWhaleTech/zorm"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// SQLiteConfigStore persists media provider configurations in SQLite.
type SQLiteConfigStore struct {
	db     *sql.DB
	readDB *sql.DB
}

type mediagenProviderRow struct {
	ID        string  `json:"id" zorm:"id"`
	Enabled   int     `json:"enabled" zorm:"enabled"`
	BaseURL   *string `json:"base_url" zorm:"base_url"`
	APIKey    *string `json:"api_key" zorm:"api_key"`
	Priority  int     `json:"priority" zorm:"priority"`
	Data      string  `json:"data" zorm:"data"` // full JSON blob for all fields
	UpdatedAt string  `json:"updated_at" zorm:"updated_at"`
}

// NewSQLiteConfigStore creates a new SQLite-backed media config store.
func NewSQLiteConfigStore(db *sql.DB) (*SQLiteConfigStore, error) {
	return NewSQLiteConfigStoreWithReadDB(db, db)
}

// NewSQLiteConfigStoreWithReadDB creates a new SQLite-backed media config store
// with separate write and read database handles.
func NewSQLiteConfigStoreWithReadDB(writeDB, readDB *sql.DB) (*SQLiteConfigStore, error) {
	s := &SQLiteConfigStore{db: writeDB, readDB: readDB}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate mediagen tables: %w", err)
	}
	return s, nil
}

func (s *SQLiteConfigStore) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "mediagen_providers")
}

func (s *SQLiteConfigStore) readTable(ctx context.Context) *z.ZormTable {
	if s != nil && s.readDB != nil {
		return z.TableContext(ctx, s.readDB, "mediagen_providers")
	}
	return z.TableContext(ctx, s.db, "mediagen_providers")
}

func (s *SQLiteConfigStore) migrate() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS mediagen_providers (
		id TEXT PRIMARY KEY,
		enabled INTEGER NOT NULL DEFAULT 0,
		base_url TEXT,
		api_key TEXT,
		priority INTEGER NOT NULL DEFAULT 0,
		data TEXT NOT NULL DEFAULT '{}',
		updated_at TEXT NOT NULL DEFAULT ''
	)`)
	return err
}

// Load reads all saved configs from DB. Returns nil map if empty.
func (s *SQLiteConfigStore) Load() (map[string]*configOnDisk, error) {
	ctx := context.Background()
	var rows []mediagenProviderRow
	_, err := s.readTable(ctx).Select(&rows)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	result := make(map[string]*configOnDisk, len(rows))
	for _, row := range rows {
		priority := row.Priority
		c := &configOnDisk{
			ID:       row.ID,
			Enabled:  row.Enabled == 1,
			Priority: &priority,
		}
		if row.BaseURL != nil {
			c.BaseURL = *row.BaseURL
		}
		if row.APIKey != nil {
			c.APIKey = *row.APIKey
		}
		result[row.ID] = c
	}
	return result, nil
}

// Save writes configs to DB.
func (s *SQLiteConfigStore) Save(configs map[string]*MediaProviderConfig) error {
	ctx := context.Background()
	now := timeutil.NowTime().Format("2006-01-02T15:04:05Z")

	for _, c := range configs {
		enabled := 0
		if c.Enabled {
			enabled = 1
		}
		data, _ := json.Marshal(c)

		var baseURL, apiKey *string
		if c.BaseURL != "" {
			baseURL = &c.BaseURL
		}
		if c.APIKey != "" {
			apiKey = &c.APIKey
		}

		_, err := s.table(ctx).Insert(map[string]interface{}{
			"id":         c.ID,
			"enabled":    enabled,
			"base_url":   baseURL,
			"api_key":    apiKey,
			"priority":   c.Priority,
			"data":       string(data),
			"updated_at": now,
		}, z.OnConflictDoUpdateSet(
			[]string{"id"},
			[]string{"enabled", "base_url", "api_key", "priority", "data", "updated_at"},
		))
		if err != nil {
			return err
		}
	}
	return nil
}

// Dir returns empty string (no directory for SQLite store).
func (s *SQLiteConfigStore) Dir() string {
	return ""
}

// MigrateFromJSON imports configs from a legacy JSON file into this store.
// After successful migration, the old file is renamed to *.bak.
func (s *SQLiteConfigStore) MigrateFromJSON(dir string) error {
	// Skip if DB already has data
	ctx := context.Background()
	var rows []mediagenProviderRow
	s.readTable(ctx).Select(&rows, z.Limit(1))
	if len(rows) > 0 {
		return nil // Already migrated
	}

	jsonPath := filepath.Join(dir, configStoreFile)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var items []configOnDisk
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	now := timeutil.NowTime().Format("2006-01-02T15:04:05Z")
	for _, item := range items {
		enabled := 0
		if item.Enabled {
			enabled = 1
		}
		priority := 0
		if item.Priority != nil {
			priority = *item.Priority
		} else {
			for _, builtin := range BuiltinMediaProviders("") {
				if builtin != nil && builtin.ID == item.ID {
					priority = builtin.Priority
					break
				}
			}
		}
		var baseURL, apiKey *string
		if item.BaseURL != "" {
			baseURL = &item.BaseURL
		}
		if item.APIKey != "" {
			apiKey = &item.APIKey
		}

		s.table(ctx).Insert(map[string]interface{}{
			"id":         item.ID,
			"enabled":    enabled,
			"base_url":   baseURL,
			"api_key":    apiKey,
			"priority":   priority,
			"data":       "{}",
			"updated_at": now,
		}, z.OnConflictDoUpdateSet(
			[]string{"id"},
			[]string{"enabled", "base_url", "api_key", "priority", "updated_at"},
		))
	}

	// Rename old file
	bakPath := jsonPath + ".bak"
	os.Rename(jsonPath, bakPath)

	return nil
}
