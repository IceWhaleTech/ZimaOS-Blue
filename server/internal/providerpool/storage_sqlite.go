package providerpool

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	z "github.com/IceWhaleTech/zorm"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// SQLiteStorage implements Storage using SQLite tables.
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage creates a new SQLite-backed storage and runs migrations.
func NewSQLiteStorage(db *sql.DB) (*SQLiteStorage, error) {
	s := &SQLiteStorage{db: db}
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
	CREATE TABLE IF NOT EXISTS pp_health (
		provider_id TEXT PRIMARY KEY,
		data TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
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
func (s *SQLiteStorage) models(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "pp_models")
}
func (s *SQLiteStorage) usage(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "pp_usage")
}
func (s *SQLiteStorage) health(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "pp_health")
}
func (s *SQLiteStorage) pricing(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "pp_pricing")
}
func (s *SQLiteStorage) poolConfig(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "pp_config")
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

type ppHealthRow struct {
	ProviderID string `json:"provider_id" zorm:"provider_id"`
	Data       string `json:"data" zorm:"data"`
	UpdatedAt  string `json:"updated_at" zorm:"updated_at"`
}

type ppSingleRow struct {
	ID        string `json:"id" zorm:"id"`
	Data      string `json:"data" zorm:"data"`
	UpdatedAt string `json:"updated_at" zorm:"updated_at"`
}

// --- Provider operations ---

func (s *SQLiteStorage) SaveProvider(provider *Provider) error {
	ctx := context.Background()

	data, err := json.Marshal(provider)
	if err != nil {
		return err
	}

	// Extract API keys (json:"-" tagged)
	var apiKeysJSON *string
	if provider.Type != ProviderTypeTrial && len(provider.APIKeys) > 0 {
		keys := make([]string, len(provider.APIKeys))
		for i, k := range provider.APIKeys {
			keys[i] = k.Key
		}
		b, _ := json.Marshal(keys)
		s := string(b)
		apiKeysJSON = &s
	}

	// Extract OAuth secrets
	var oauthJSON *string
	if secrets := extractOAuthSecrets(provider); secrets != nil {
		b, _ := json.Marshal(secrets)
		s := string(b)
		oauthJSON = &s
	}

	now := timeutil.NowTime().Format(time.RFC3339)
	_, err = s.providers(ctx).Insert(map[string]interface{}{
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
}

func (s *SQLiteStorage) LoadProvider(id string) (*Provider, error) {
	ctx := context.Background()
	var rows []ppProviderRow
	_, err := s.providers(ctx).Select(&rows,
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
	_, err := s.providers(ctx).Select(&rows)
	if err != nil {
		return nil, err
	}

	providers := make([]*Provider, 0, len(rows))
	needsSave := false
	for i := range rows {
		p, err := s.rowToProvider(&rows[i])
		if err != nil {
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
	_, err := s.providers(ctx).Delete(z.Where(z.Eq("id", id)))
	if err != nil {
		return err
	}
	// Also delete models
	s.models(ctx).Delete(z.Where(z.Eq("provider_id", id)))
	return nil
}

func (s *SQLiteStorage) rowToProvider(row *ppProviderRow) (*Provider, error) {
	var provider Provider
	if err := json.Unmarshal([]byte(row.Data), &provider); err != nil {
		return nil, err
	}

	// Restore API keys
	if row.APIKeys != nil && *row.APIKeys != "" {
		var keys []string
		if err := json.Unmarshal([]byte(*row.APIKeys), &keys); err == nil {
			for i := range provider.APIKeys {
				if i < len(keys) {
					provider.APIKeys[i].Key = keys[i]
				}
			}
		}
	}

	// Restore OAuth secrets
	if row.OAuthSecrets != nil && *row.OAuthSecrets != "" {
		var secrets oauthSecrets
		if err := json.Unmarshal([]byte(*row.OAuthSecrets), &secrets); err == nil {
			restoreOAuthSecrets(&provider, &secrets)
		}
	}

	return &provider, nil
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
	_, err := s.models(ctx).Select(&rows,
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

	_, err := s.usage(ctx).Select(&rows, conds...)
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

// --- Health operations ---

func (s *SQLiteStorage) SaveHealthStatus(results map[string]*HealthCheckResult) error {
	ctx := context.Background()
	now := timeutil.NowTime().Format(time.RFC3339)
	for pid, result := range results {
		data, err := json.Marshal(result)
		if err != nil {
			continue
		}
		s.health(ctx).Insert(map[string]interface{}{
			"provider_id": pid,
			"data":        string(data),
			"updated_at":  now,
		}, z.OnConflictDoUpdateSet(
			[]string{"provider_id"},
			[]string{"data", "updated_at"},
		))
	}
	return nil
}

func (s *SQLiteStorage) LoadHealthStatus() (map[string]*HealthCheckResult, error) {
	ctx := context.Background()
	var rows []ppHealthRow
	_, err := s.health(ctx).Select(&rows)
	if err != nil {
		return make(map[string]*HealthCheckResult), nil
	}
	results := make(map[string]*HealthCheckResult, len(rows))
	for _, row := range rows {
		var result HealthCheckResult
		if err := json.Unmarshal([]byte(row.Data), &result); err != nil {
			continue
		}
		results[row.ProviderID] = &result
	}
	return results, nil
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
	_, err := s.pricing(ctx).Select(&rows, z.Where(z.Eq("id", "default")), z.Limit(1))
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
	_, err := s.poolConfig(ctx).Select(&rows, z.Where(z.Eq("id", "default")), z.Limit(1))
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
	s.providers(ctx).Select(&rows, z.Limit(1))
	if len(rows) > 0 {
		return nil // Already migrated
	}

	// Check if legacy directory exists
	providersFile := filepath.Join(basePath, "providers.json")
	if _, err := os.Stat(providersFile); os.IsNotExist(err) {
		return nil // No legacy data
	}

	// Load from FileStorage
	fs, err := NewFileStorage(basePath)
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

	// Migrate health
	health, err := fs.LoadHealthStatus()
	if err == nil && len(health) > 0 {
		s.SaveHealthStatus(health)
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
