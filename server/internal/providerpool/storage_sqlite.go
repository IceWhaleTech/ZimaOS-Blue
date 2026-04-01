package providerpool

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	z "github.com/IceWhaleTech/zorm"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// SQLiteStorage implements Storage using SQLite tables.
type SQLiteStorage struct {
	db        *sql.DB
	readDB    *sql.DB
	encryptor SecretEncryptor
}

// NewSQLiteStorage creates a new SQLite-backed storage and runs migrations.
func NewSQLiteStorage(db *sql.DB, opts ...StorageOption) (*SQLiteStorage, error) {
	return NewSQLiteStorageWithReadDB(db, db, opts...)
}

// NewSQLiteStorageWithReadDB creates a new SQLite-backed storage with separate
// write and read database handles and runs migrations.
func NewSQLiteStorageWithReadDB(writeDB, readDB *sql.DB, opts ...StorageOption) (*SQLiteStorage, error) {
	if writeDB == nil {
		return nil, fmt.Errorf("provider pool db is required")
	}
	if readDB == nil {
		readDB = writeDB
	}
	storageOpts := applyStorageOptions(opts...)
	s := &SQLiteStorage{db: writeDB, readDB: readDB, encryptor: storageOpts.encryptor}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate provider pool tables: %w", err)
	}
	return s, nil
}

func (s *SQLiteStorage) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS pp_providers (
		id TEXT PRIMARY KEY,
		data TEXT NOT NULL,
		api_keys TEXT,
		oauth_secrets TEXT,
		updated_at TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS pp_models (
		provider_id TEXT PRIMARY KEY,
		data TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS pp_usage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_id TEXT NOT NULL,
		data TEXT NOT NULL,
		timestamp TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_pp_usage_provider_ts ON pp_usage(provider_id, timestamp);
	CREATE TABLE IF NOT EXISTS pp_pricing (
		id TEXT PRIMARY KEY DEFAULT 'default',
		data TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS pp_config (
		id TEXT PRIMARY KEY DEFAULT 'default',
		data TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

// --- helpers ---

func (s *SQLiteStorage) providers(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "pp_providers")
}
func (s *SQLiteStorage) providersRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "pp_providers")
}
func (s *SQLiteStorage) models(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "pp_models")
}
func (s *SQLiteStorage) modelsRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "pp_models")
}
func (s *SQLiteStorage) usage(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "pp_usage")
}
func (s *SQLiteStorage) usageRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "pp_usage")
}
func (s *SQLiteStorage) pricing(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "pp_pricing")
}
func (s *SQLiteStorage) pricingRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "pp_pricing")
}
func (s *SQLiteStorage) poolConfig(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "pp_config")
}
func (s *SQLiteStorage) poolConfigRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "pp_config")
}

func (s *SQLiteStorage) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

type ppProviderRow struct {
	ID           string  `json:"id" zorm:"id"`
	Data         string  `json:"data" zorm:"data"`
	APIKeys      *string `json:"api_keys" zorm:"api_keys"`
	OAuthSecrets *string `json:"oauth_secrets" zorm:"oauth_secrets"`
	UpdatedAt    string  `json:"updated_at" zorm:"updated_at"`
}

type ppModelsRow struct {
	ProviderID string `json:"provider_id" zorm:"provider_id"`
	Data       string `json:"data" zorm:"data"`
	UpdatedAt  string `json:"updated_at" zorm:"updated_at"`
}

type ppUsageRow struct {
	ID         int64  `json:"id" zorm:"id"`
	ProviderID string `json:"provider_id" zorm:"provider_id"`
	Data       string `json:"data" zorm:"data"`
	Timestamp  string `json:"timestamp" zorm:"timestamp"`
}

type ppSingleRow struct {
	ID        string `json:"id" zorm:"id"`
	Data      string `json:"data" zorm:"data"`
	UpdatedAt string `json:"updated_at" zorm:"updated_at"`
}

func (s *SQLiteStorage) retryAfterWALCheckpoint(ctx context.Context, opName string, op func() error) error {
	err := op()
	if err == nil || !dbutil.IsSQLiteCorruptionError(err) {
		return err
	}

	slog.Warn("[providerpool] sqlite corruption detected, retrying after WAL checkpoint", "op", opName, "error", err)
	if _, checkpointErr := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); checkpointErr != nil {
		return fmt.Errorf("%s: sqlite corruption detected (%v) and WAL checkpoint failed: %w", opName, err, checkpointErr)
	}

	if retryErr := op(); retryErr != nil {
		return fmt.Errorf("%s: retry after WAL checkpoint failed: %w", opName, retryErr)
	}
	return nil
}

// --- Provider operations ---

func (s *SQLiteStorage) SaveProvider(provider *Provider) error {
	ctx := context.Background()
	normalizeProviderAPIKeys(provider)

	data, err := json.Marshal(provider)
	if err != nil {
		return err
	}

	var apiKeysJSON *string
	if keys, err := prepareStoredAPIKeys(provider, s.encryptor); err != nil {
		return err
	} else if len(keys) > 0 {
		b, _ := json.Marshal(keys)
		s := string(b)
		apiKeysJSON = &s
	}

	var oauthJSON *string
	if secrets, err := prepareStoredOAuthSecrets(provider, s.encryptor); err != nil {
		return err
	} else if secrets != nil {
		b, _ := json.Marshal(secrets)
		s := string(b)
		oauthJSON = &s
	}

	now := timeutil.NowTime().Format(time.RFC3339)
	err = s.retryAfterWALCheckpoint(ctx, "save provider", func() error {
		_, err := s.providers(ctx).Insert(map[string]interface{}{
			"id":            provider.ID,
			"data":          string(data),
			"api_keys":      apiKeysJSON,
			"oauth_secrets": oauthJSON,
			"updated_at":    now,
		}, z.OnConflictDoUpdateSet(
			[]string{"id"},
			[]string{"data", "api_keys", "oauth_secrets", "updated_at"},
		))
		return err
	})
	return err
}

func (s *SQLiteStorage) LoadProvider(id string) (*Provider, error) {
	ctx := context.Background()
	var rows []ppProviderRow
	_, err := s.providersRead(ctx).Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	)
	if err != nil || len(rows) == 0 {
		return nil, ErrProviderNotFound
	}
	return s.rowToProvider(&rows[0])
}

func (s *SQLiteStorage) LoadAllProviders() ([]*Provider, error) {
	ctx := context.Background()
	var rows []ppProviderRow
	err := s.retryAfterWALCheckpoint(ctx, "load providers", func() error {
		rows = nil
		_, err := s.providersRead(ctx).Select(&rows)
		return err
	})
	if err != nil {
		return nil, err
	}

	providers := make([]*Provider, 0, len(rows))
	needsSave := false
	for i := range rows {
		if rows[i].APIKeys != nil && *rows[i].APIKeys != "" {
			var keys []string
			if err := json.Unmarshal([]byte(*rows[i].APIKeys), &keys); err == nil && storedAPIKeysNeedEncryption(keys, s.encryptor) {
				needsSave = true
			}
		}
		if rows[i].OAuthSecrets != nil && *rows[i].OAuthSecrets != "" {
			var secrets oauthSecrets
			if err := json.Unmarshal([]byte(*rows[i].OAuthSecrets), &secrets); err == nil && storedOAuthSecretsNeedEncryption(&secrets, s.encryptor) {
				needsSave = true
			}
		}
		p, err := s.rowToProvider(&rows[i])
		if err != nil {
			slog.Warn("[providerpool] skip invalid provider row", "provider_id", rows[i].ID, "error", err)
			continue
		}
		// Backfill missing key IDs
		for j := range p.APIKeys {
			if p.APIKeys[j].ID == "" {
				p.APIKeys[j].ID = GenerateID("key")
				needsSave = true
			}
		}
		providers = append(providers, p)
	}

	if needsSave {
		for _, p := range providers {
			_ = s.SaveProvider(p)
		}
	}

	return providers, nil
}

func (s *SQLiteStorage) DeleteProvider(id string) error {
	ctx := context.Background()
	return s.retryAfterWALCheckpoint(ctx, "delete provider", func() error {
		if _, err := s.providers(ctx).Delete(z.Where(z.Eq("id", id))); err != nil {
			return err
		}
		_, err := s.models(ctx).Delete(z.Where(z.Eq("provider_id", id)))
		return err
	})
}

func (s *SQLiteStorage) rowToProvider(row *ppProviderRow) (*Provider, error) {
	var provider Provider
	raw := []byte(row.Data)
	if err := json.Unmarshal(raw, &provider); err != nil {
		normalized := normalizeProviderJSONTimestamps(raw)
		if len(normalized) == 0 || string(normalized) == string(raw) {
			return nil, err
		}
		if err2 := json.Unmarshal(normalized, &provider); err2 != nil {
			return nil, err
		}
	}

	// Restore API keys
	if row.APIKeys != nil && *row.APIKeys != "" {
		var keys []string
		if err := json.Unmarshal([]byte(*row.APIKeys), &keys); err == nil {
			if err := restoreStoredAPIKeys(&provider, keys, s.encryptor); err != nil {
				return nil, err
			}
		}
	}

	// Restore OAuth secrets
	if row.OAuthSecrets != nil && *row.OAuthSecrets != "" {
		var secrets oauthSecrets
		if err := json.Unmarshal([]byte(*row.OAuthSecrets), &secrets); err == nil {
			if err := restoreStoredOAuthSecrets(&provider, &secrets, s.encryptor); err != nil {
				return nil, err
			}
		}
	}

	return &provider, nil
}

var providerTimestampFields = []string{
	"created_at",
	"updated_at",
	"detected_at",
	"last_health_check",
	"last_error_time",
}

func normalizeProviderJSONTimestamps(raw []byte) []byte {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return raw
	}

	changed := false
	for _, field := range providerTimestampFields {
		value, ok := obj[field]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(value, &s); err != nil {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339Nano, s); err == nil {
			continue
		}
		if ts, ok := parseProviderTimestamp(s); ok {
			encoded, _ := json.Marshal(ts.Format(time.RFC3339Nano))
			obj[field] = encoded
			changed = true
			continue
		}
		// Drop unparseable timestamps to keep provider loadable.
		delete(obj, field)
		changed = true
	}

	if !changed {
		return raw
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return out
}

func parseProviderTimestamp(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	// Common legacy format found in migrated rows.
	if ts, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return ts, true
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

// --- Model operations ---

func (s *SQLiteStorage) SaveModels(providerID string, models []*Model) error {
	ctx := context.Background()
	data, err := json.Marshal(models)
	if err != nil {
		return err
	}
	now := timeutil.NowTime().Format(time.RFC3339)
	_, err = s.models(ctx).Insert(map[string]interface{}{
		"provider_id": providerID,
		"data":        string(data),
		"updated_at":  now,
	}, z.OnConflictDoUpdateSet(
		[]string{"provider_id"},
		[]string{"data", "updated_at"},
	))
	return err
}

func (s *SQLiteStorage) LoadModels(providerID string) ([]*Model, error) {
	ctx := context.Background()
	var rows []ppModelsRow
	_, err := s.modelsRead(ctx).Select(&rows,
		z.Where(z.Eq("provider_id", providerID)),
		z.Limit(1),
	)
	if err != nil || len(rows) == 0 {
		return []*Model{}, nil
	}
	var models []*Model
	if err := json.Unmarshal([]byte(rows[0].Data), &models); err != nil {
		return nil, err
	}
	return models, nil
}

// --- Usage operations ---

func (s *SQLiteStorage) AppendUsage(record *UsageRecord) error {
	ctx := context.Background()
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = s.usage(ctx).Insert(map[string]interface{}{
		"provider_id": record.ProviderID,
		"data":        string(data),
		"timestamp":   record.Timestamp.Format(time.RFC3339),
	})
	return err
}

func (s *SQLiteStorage) LoadUsage(providerID string, start, end time.Time) ([]*UsageRecord, error) {
	ctx := context.Background()
	startStr := start.Format(time.RFC3339)
	endStr := end.Format(time.RFC3339)

	var rows []ppUsageRow
	conds := []z.ZormItem{
		z.Where(
			z.Gt("timestamp", startStr),
			z.Lt("timestamp", endStr),
		),
		z.OrderBy("timestamp ASC"),
	}
	if providerID != "" {
		conds = []z.ZormItem{
			z.Where(
				z.Eq("provider_id", providerID),
				z.Gt("timestamp", startStr),
				z.Lt("timestamp", endStr),
			),
			z.OrderBy("timestamp ASC"),
		}
	}

	_, err := s.usageRead(ctx).Select(&rows, conds...)
	if err != nil {
		return nil, err
	}

	records := make([]*UsageRecord, 0, len(rows))
	for _, row := range rows {
		var rec UsageRecord
		if err := json.Unmarshal([]byte(row.Data), &rec); err != nil {
			continue
		}
		records = append(records, &rec)
	}
	return records, nil
}

// --- Pricing operations ---

func (s *SQLiteStorage) SavePricingConfig(config *PricingConfig) error {
	ctx := context.Background()
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	now := timeutil.NowTime().Format(time.RFC3339)
	_, err = s.pricing(ctx).Insert(map[string]interface{}{
		"id":         "default",
		"data":       string(data),
		"updated_at": now,
	}, z.OnConflictDoUpdateSet(
		[]string{"id"},
		[]string{"data", "updated_at"},
	))
	return err
}

func (s *SQLiteStorage) LoadPricingConfig() (*PricingConfig, error) {
	ctx := context.Background()
	var rows []ppSingleRow
	_, err := s.pricingRead(ctx).Select(&rows, z.Where(z.Eq("id", "default")), z.Limit(1))
	if err != nil || len(rows) == 0 {
		return nil, os.ErrNotExist
	}
	var config PricingConfig
	if err := json.Unmarshal([]byte(rows[0].Data), &config); err != nil {
		return nil, err
	}
	if config.CustomPricing == nil {
		config.CustomPricing = make(map[string]*ModelPricing)
	}
	return &config, nil
}

// --- Pool config operations ---

func (s *SQLiteStorage) SaveConfig(config *PoolConfig) error {
	ctx := context.Background()
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	now := timeutil.NowTime().Format(time.RFC3339)
	_, err = s.poolConfig(ctx).Insert(map[string]interface{}{
		"id":         "default",
		"data":       string(data),
		"updated_at": now,
	}, z.OnConflictDoUpdateSet(
		[]string{"id"},
		[]string{"data", "updated_at"},
	))
	return err
}

func (s *SQLiteStorage) LoadConfig() (*PoolConfig, error) {
	ctx := context.Background()
	var rows []ppSingleRow
	_, err := s.poolConfigRead(ctx).Select(&rows, z.Where(z.Eq("id", "default")), z.Limit(1))
	if err != nil || len(rows) == 0 {
		return nil, os.ErrNotExist
	}
	var config PoolConfig
	if err := json.Unmarshal([]byte(rows[0].Data), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// --- Migration from JSON files ---

// MigrateFromFiles imports data from the legacy FileStorage directory into SQLite.
// After successful migration, the old directory is renamed to basePath+".bak".
// This is idempotent — if the DB already has providers, migration is skipped.
func (s *SQLiteStorage) MigrateFromFiles(basePath string) error {
	// Skip if DB already has providers
	ctx := context.Background()
	var rows []ppProviderRow
	s.providersRead(ctx).Select(&rows, z.Limit(1))
	if len(rows) > 0 {
		return nil // Already migrated
	}

	// Check if legacy directory exists
	providersFile := filepath.Join(basePath, "providers.json")
	if _, err := os.Stat(providersFile); os.IsNotExist(err) {
		return nil // No legacy data
	}

	// Load from FileStorage
	fs, err := NewFileStorage(basePath, WithStorageEncryptor(s.encryptor))
	if err != nil {
		return fmt.Errorf("open legacy file storage: %w", err)
	}

	// Migrate providers
	providers, err := fs.LoadAllProviders()
	if err != nil {
		return fmt.Errorf("load legacy providers: %w", err)
	}
	for _, p := range providers {
		if err := s.SaveProvider(p); err != nil {
			return fmt.Errorf("migrate provider %s: %w", p.ID, err)
		}
		// Migrate models
		models, err := fs.LoadModels(p.ID)
		if err == nil && len(models) > 0 {
			s.SaveModels(p.ID, models)
		}
	}

	// Migrate pricing
	pricing, err := fs.LoadPricingConfig()
	if err == nil {
		s.SavePricingConfig(pricing)
	}

	// Migrate pool config
	poolCfg, err := fs.LoadConfig()
	if err == nil {
		s.SaveConfig(poolCfg)
	}

	// Usage migration: read all JSONL files from usage/ directory
	usageDir := filepath.Join(basePath, "usage")
	if entries, err := os.ReadDir(usageDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(usageDir, entry.Name()))
			if err != nil {
				continue
			}
			for _, line := range splitLines(data) {
				if len(line) == 0 {
					continue
				}
				var rec UsageRecord
				if json.Unmarshal(line, &rec) == nil {
					s.AppendUsage(&rec)
				}
			}
		}
	}

	// Rename old directory
	bakPath := basePath + ".bak"
	os.RemoveAll(bakPath) // Remove old backup if exists
	if err := os.Rename(basePath, bakPath); err != nil {
		// Non-fatal — data is already in DB
		return nil
	}

	return nil
}
