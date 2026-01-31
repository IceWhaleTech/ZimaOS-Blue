package ngrok

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRepository_NewRepository(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "ngrok-repo-test")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	repo, err := NewRepository(filepath.Join(tempDir, "ngrok.db"))
	if err != nil {
		t.Fatalf("NewRepository() error: %v", err)
	}
	defer repo.Close()

	if repo == nil {
		t.Fatal("NewRepository() returned nil")
	}
}

func TestRepository_GetConfig_Default(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "ngrok-repo-test-config")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	repo, err := NewRepository(filepath.Join(tempDir, "ngrok.db"))
	if err != nil {
		t.Fatalf("NewRepository() error: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	config, err := repo.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig() error: %v", err)
	}

	// Default config should have enabled = false
	if config.Enabled {
		t.Error("Default config should have Enabled = false")
	}

	// Default notification settings
	if !config.NotifyOnURLChange {
		t.Error("Default NotifyOnURLChange should be true")
	}
}

func TestRepository_SaveConfig(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "ngrok-repo-test-save")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	repo, err := NewRepository(filepath.Join(tempDir, "ngrok.db"))
	if err != nil {
		t.Fatalf("NewRepository() error: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Save config
	config := &RemoteAccessConfig{
		Enabled:              true,
		NotificationEmail:    "test@example.com",
		NotifyOnURLChange:    true,
		NotifyOnExpiryWarning: true,
		NotifyOnError:        false,
	}

	err = repo.SaveConfig(ctx, config)
	if err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}

	// Read back
	savedConfig, err := repo.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig() error: %v", err)
	}

	if savedConfig.Enabled != config.Enabled {
		t.Errorf("Enabled = %v, want %v", savedConfig.Enabled, config.Enabled)
	}

	if savedConfig.NotificationEmail != config.NotificationEmail {
		t.Errorf("NotificationEmail = %s, want %s", savedConfig.NotificationEmail, config.NotificationEmail)
	}
}

func TestRepository_CreateSession(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "ngrok-repo-test-session")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	repo, err := NewRepository(filepath.Join(tempDir, "ngrok.db"))
	if err != nil {
		t.Fatalf("NewRepository() error: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	session := &RemoteAccessSession{
		TunnelURL:    "https://abc123.ngrok-free.app",
		StartedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(8 * time.Hour),
		RenewedCount: 0,
		Status:       "active",
	}

	id, err := repo.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	if id == "" {
		t.Error("CreateSession() returned empty ID")
	}
}

func TestRepository_GetActiveSession(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "ngrok-repo-test-active")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	repo, err := NewRepository(filepath.Join(tempDir, "ngrok.db"))
	if err != nil {
		t.Fatalf("NewRepository() error: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Initially no active session
	session, err := repo.GetActiveSession(ctx)
	if err != nil {
		t.Fatalf("GetActiveSession() error: %v", err)
	}
	if session != nil {
		t.Error("Should have no active session initially")
	}

	// Create a session
	newSession := &RemoteAccessSession{
		TunnelURL:    "https://abc123.ngrok-free.app",
		StartedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(8 * time.Hour),
		RenewedCount: 0,
		Status:       "active",
	}

	_, err = repo.CreateSession(ctx, newSession)
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	// Now should have active session
	session, err = repo.GetActiveSession(ctx)
	if err != nil {
		t.Fatalf("GetActiveSession() error: %v", err)
	}
	if session == nil {
		t.Error("Should have active session after creation")
	}
}

func TestRepository_AddLog(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "ngrok-repo-test-log")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	repo, err := NewRepository(filepath.Join(tempDir, "ngrok.db"))
	if err != nil {
		t.Fatalf("NewRepository() error: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	err = repo.AddLog(ctx, "", "started", "Tunnel started", nil)
	if err != nil {
		t.Fatalf("AddLog() error: %v", err)
	}
}

func TestRepository_GetLogs(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "ngrok-repo-test-logs")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	repo, err := NewRepository(filepath.Join(tempDir, "ngrok.db"))
	if err != nil {
		t.Fatalf("NewRepository() error: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Add some logs
	repo.AddLog(ctx, "", "started", "Tunnel started", nil)
	repo.AddLog(ctx, "", "renewed", "Tunnel renewed", nil)
	repo.AddLog(ctx, "", "stopped", "Tunnel stopped", nil)

	// Get logs
	logs, err := repo.GetLogs(ctx, 10, 0)
	if err != nil {
		t.Fatalf("GetLogs() error: %v", err)
	}

	if len(logs) != 3 {
		t.Errorf("GetLogs() returned %d logs, want 3", len(logs))
	}
}
