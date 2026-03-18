package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
)

func TestLoadOrImport_FirstRunGeneratesJWTSecretAndPersists(t *testing.T) {
	kv := kvstore.NewMemoryStore()
	store := NewConfigStore(kv)
	cfg := defaults()
	cfg.Security.JWT.Secret = defaultJWTSecretPlaceholder

	got, err := store.LoadOrImport(&cfg)
	if err != nil {
		t.Fatalf("LoadOrImport() error = %v", err)
	}
	if got.Security.JWT.Secret == "" {
		t.Fatal("expected generated JWT secret, got empty string")
	}
	if got.Security.JWT.Secret == defaultJWTSecretPlaceholder {
		t.Fatal("expected JWT secret to be replaced on first import")
	}
	if len(got.Security.JWT.Secret) != 64 {
		t.Fatalf("expected 64-char hex JWT secret, got len=%d", len(got.Security.JWT.Secret))
	}

	var sec SecurityConfig
	if err := kv.GetJSON(context.Background(), configKeyPrefix+"security", &sec); err != nil {
		t.Fatalf("GetJSON(security) error = %v", err)
	}
	if sec.JWT.Secret != got.Security.JWT.Secret {
		t.Fatalf("persisted JWT secret mismatch: got=%q persisted=%q", got.Security.JWT.Secret, sec.JWT.Secret)
	}
}

func TestLoadOrImport_SecondLoadKeepsSameJWTSecret(t *testing.T) {
	kv := kvstore.NewMemoryStore()
	store := NewConfigStore(kv)
	cfg := defaults()
	cfg.Security.JWT.Secret = defaultJWTSecretPlaceholder

	first, err := store.LoadOrImport(&cfg)
	if err != nil {
		t.Fatalf("first LoadOrImport() error = %v", err)
	}
	firstSecret := first.Security.JWT.Secret

	another := defaults()
	another.Security.JWT.Secret = defaultJWTSecretPlaceholder
	second, err := store.LoadOrImport(&another)
	if err != nil {
		t.Fatalf("second LoadOrImport() error = %v", err)
	}
	if second.Security.JWT.Secret != firstSecret {
		t.Fatalf("expected JWT secret to stay stable across reload, first=%q second=%q", firstSecret, second.Security.JWT.Secret)
	}
}

func TestLoadOrImport_CustomJWTSecretIsPreserved(t *testing.T) {
	kv := kvstore.NewMemoryStore()
	store := NewConfigStore(kv)
	cfg := defaults()
	cfg.Security.JWT.Secret = "custom-jwt-secret-abcdefghijklmnopqrstuvwxyz123456"

	got, err := store.LoadOrImport(&cfg)
	if err != nil {
		t.Fatalf("LoadOrImport() error = %v", err)
	}
	if got.Security.JWT.Secret != cfg.Security.JWT.Secret {
		t.Fatalf("expected custom JWT secret preserved, got=%q want=%q", got.Security.JWT.Secret, cfg.Security.JWT.Secret)
	}
}

func TestLoadOrImport_MigratesLegacyDefaultJWTSecretFromDB(t *testing.T) {
	kv := kvstore.NewMemoryStore()
	seedStore := NewConfigStore(kv)
	legacy := defaults()
	legacy.Security.JWT.Secret = defaultJWTSecretPlaceholder
	if err := seedStore.Import(&legacy); err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	store := NewConfigStore(kv)
	got, err := store.LoadOrImport(&Config{})
	if err != nil {
		t.Fatalf("LoadOrImport() error = %v", err)
	}
	if got.Security.JWT.Secret == "" {
		t.Fatal("expected migrated JWT secret, got empty string")
	}
	if got.Security.JWT.Secret == defaultJWTSecretPlaceholder {
		t.Fatal("expected legacy placeholder secret to be migrated")
	}
	firstSecret := got.Security.JWT.Secret

	var sec SecurityConfig
	if err := kv.GetJSON(context.Background(), configKeyPrefix+"security", &sec); err != nil {
		t.Fatalf("GetJSON(security) error = %v", err)
	}
	if sec.JWT.Secret != firstSecret {
		t.Fatalf("persisted migrated JWT secret mismatch: got=%q persisted=%q", firstSecret, sec.JWT.Secret)
	}

	got2, err := store.LoadOrImport(&Config{})
	if err != nil {
		t.Fatalf("second LoadOrImport() error = %v", err)
	}
	if got2.Security.JWT.Secret != firstSecret {
		t.Fatalf("expected migrated secret to remain stable, first=%q second=%q", firstSecret, got2.Security.JWT.Secret)
	}
}

func TestLoadOrImport_PreservesMissingNestedSecurityDefaultsFromDB(t *testing.T) {
	kv := kvstore.NewMemoryStore()
	store := NewConfigStore(kv)
	ctx := context.Background()

	if err := kv.SetJSON(ctx, configKeyPrefix+"server", ServerConfig{Host: "0.0.0.0"}, 0); err != nil {
		t.Fatalf("SetJSON(server) error = %v", err)
	}
	if err := kv.SetJSON(
		ctx,
		configKeyPrefix+"security",
		map[string]any{
			"jwt": map[string]any{
				"secret": "legacy-jwt-secret",
			},
		},
		0,
	); err != nil {
		t.Fatalf("SetJSON(security) error = %v", err)
	}

	cfg := defaults()
	got, err := store.LoadOrImport(&cfg)
	if err != nil {
		t.Fatalf("LoadOrImport() error = %v", err)
	}

	if got.Security.JWT.Secret != "legacy-jwt-secret" {
		t.Fatalf("Security.JWT.Secret = %q, want %q", got.Security.JWT.Secret, "legacy-jwt-secret")
	}
	if got.Security.JWT.Expiration != 24*time.Hour {
		t.Fatalf("Security.JWT.Expiration = %v, want %v", got.Security.JWT.Expiration, 24*time.Hour)
	}
	if got.Security.JWT.RefreshExpiration != 720*time.Hour {
		t.Fatalf(
			"Security.JWT.RefreshExpiration = %v, want %v",
			got.Security.JWT.RefreshExpiration,
			720*time.Hour,
		)
	}
	if got.Security.Sandbox.DefaultTimeout != 30*time.Second {
		t.Fatalf("Security.Sandbox.DefaultTimeout = %v, want %v", got.Security.Sandbox.DefaultTimeout, 30*time.Second)
	}
}

func TestLoadOrImport_PreservesBrowserSection(t *testing.T) {
	kv := kvstore.NewMemoryStore()
	store := NewConfigStore(kv)
	cfg := defaults()
	cfg.Browser.Headless = false
	cfg.Browser.PoolSize = 1

	got, err := store.LoadOrImport(&cfg)
	if err != nil {
		t.Fatalf("LoadOrImport() error = %v", err)
	}
	if got.Browser.Headless {
		t.Fatalf("Browser.Headless = %v, want false", got.Browser.Headless)
	}
	if got.Browser.PoolSize != 1 {
		t.Fatalf("Browser.PoolSize = %d, want 1", got.Browser.PoolSize)
	}

	reloaded, err := store.Reload()
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if reloaded.Browser.Headless {
		t.Fatalf("reloaded Browser.Headless = %v, want false", reloaded.Browser.Headless)
	}
	if reloaded.Browser.PoolSize != 1 {
		t.Fatalf("reloaded Browser.PoolSize = %d, want 1", reloaded.Browser.PoolSize)
	}
}

func TestConfigStore_RejectsInvalidSectionNames(t *testing.T) {
	kv := kvstore.NewMemoryStore()
	store := NewConfigStore(kv)

	if _, err := store.GetSection("../security"); !errors.Is(err, ErrInvalidSection) {
		t.Fatalf("GetSection() error = %v, want ErrInvalidSection", err)
	}

	if err := store.SetSection("../security", []byte(`{}`)); !errors.Is(err, ErrInvalidSection) {
		t.Fatalf("SetSection() error = %v, want ErrInvalidSection", err)
	}
}
