package ngrok

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestRepository_UsesReaderDBForReads(t *testing.T) {
	repo, err := NewRepository(filepath.Join(t.TempDir(), "ngrok.db"))
	if err != nil {
		t.Fatalf("NewRepository() error: %v", err)
	}
	defer repo.Close()

	if repo.readDB == nil {
		t.Fatal("expected readDB to be initialized")
	}
	if repo.readDB == repo.db {
		t.Fatal("expected separate reader db")
	}

	ctx := context.Background()

	if err := repo.SaveConfig(ctx, &RemoteAccessConfig{
		Enabled:               true,
		NgrokDomain:           "example.ngrok.app",
		DefaultProvider:       "ngrok",
		NotificationEmail:     "reader@example.com",
		NotifyOnURLChange:     true,
		NotifyOnExpiryWarning: true,
		NotifyOnError:         false,
	}); err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}

	session := &RemoteAccessSession{
		TunnelURL:    "https://reader.ngrok.app",
		StartedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(8 * time.Hour),
		RenewedCount: 1,
		Status:       "active",
	}
	sessionID, err := repo.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	if err := repo.AddLog(ctx, sessionID, "error", "reader error", map[string]interface{}{"code": "E1"}); err != nil {
		t.Fatalf("AddLog(error) error: %v", err)
	}
	if err := repo.AddLog(ctx, sessionID, "started", "reader started", nil); err != nil {
		t.Fatalf("AddLog(started) error: %v", err)
	}

	subdomain, err := repo.EnsureTunnelSubdomain(ctx)
	if err != nil {
		t.Fatalf("EnsureTunnelSubdomain() error: %v", err)
	}
	if subdomain == "" {
		t.Fatal("expected non-empty tunnel subdomain")
	}

	if err := repo.db.Close(); err != nil {
		t.Fatalf("close write db: %v", err)
	}

	cfg, err := repo.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig() error: %v", err)
	}
	if cfg.NgrokDomain != "example.ngrok.app" {
		t.Fatalf("NgrokDomain = %q, want %q", cfg.NgrokDomain, "example.ngrok.app")
	}

	active, err := repo.GetActiveSession(ctx)
	if err != nil {
		t.Fatalf("GetActiveSession() error: %v", err)
	}
	if active == nil || active.ID != sessionID {
		t.Fatalf("unexpected active session: %+v", active)
	}

	readerSubdomain, err := repo.EnsureTunnelSubdomain(ctx)
	if err != nil {
		t.Fatalf("EnsureTunnelSubdomain() via reader error: %v", err)
	}
	if readerSubdomain != subdomain {
		t.Fatalf("reader subdomain = %q, want %q", readerSubdomain, subdomain)
	}

	logs, err := repo.GetLogs(ctx, 10, 0)
	if err != nil {
		t.Fatalf("GetLogs() error: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("len(GetLogs()) = %d, want 2", len(logs))
	}

	errorLogs, err := repo.GetErrorLogs(ctx, 10, 0)
	if err != nil {
		t.Fatalf("GetErrorLogs() error: %v", err)
	}
	if len(errorLogs) != 1 || errorLogs[0].EventType != "error" {
		t.Fatalf("unexpected error logs: %+v", errorLogs)
	}
}

func TestRepository_SaveConfigPreservesTunnelSubdomain(t *testing.T) {
	repo, err := NewRepository(filepath.Join(t.TempDir(), "ngrok.db"))
	if err != nil {
		t.Fatalf("NewRepository() error: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	subdomain, err := repo.EnsureTunnelSubdomain(ctx)
	if err != nil {
		t.Fatalf("EnsureTunnelSubdomain() error: %v", err)
	}
	if subdomain == "" {
		t.Fatal("expected tunnel subdomain")
	}

	if err := repo.SaveConfig(ctx, &RemoteAccessConfig{
		Enabled:               true,
		DefaultProvider:       "ngrok",
		NotificationEmail:     "preserve@example.com",
		NotifyOnURLChange:     true,
		NotifyOnExpiryWarning: false,
		NotifyOnError:         true,
	}); err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}

	cfg, err := repo.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig() error: %v", err)
	}
	if cfg.TunnelSubdomain != subdomain {
		t.Fatalf("TunnelSubdomain = %q, want %q", cfg.TunnelSubdomain, subdomain)
	}
}
