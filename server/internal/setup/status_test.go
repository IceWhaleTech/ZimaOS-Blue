package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFirstRunManager(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewFirstRunManager(tmpDir)

	if manager == nil {
		t.Fatal("expected non-nil manager")
	}

	if manager.storagePath != tmpDir {
		t.Errorf("expected storage path %s, got %s", tmpDir, manager.storagePath)
	}
}

func TestFirstRunManager_IsFirstRun(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewFirstRunManager(tmpDir)

	// Should be first run initially
	if !manager.IsFirstRun() {
		t.Error("expected IsFirstRun to return true initially")
	}

	// Mark complete
	if err := manager.MarkComplete(); err != nil {
		t.Fatalf("MarkComplete failed: %v", err)
	}

	// Should no longer be first run
	if manager.IsFirstRun() {
		t.Error("expected IsFirstRun to return false after MarkComplete")
	}
}

func TestFirstRunManager_GetStatus(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewFirstRunManager(tmpDir)

	status := manager.GetStatus()
	if status == nil {
		t.Fatal("expected non-nil status")
	}

	if status.FirstRunCompleted {
		t.Error("expected FirstRunCompleted to be false initially")
	}
}

func TestFirstRunManager_SetCLIStatus(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewFirstRunManager(tmpDir)

	err := manager.SetCLIStatus(true, "1.0.0", "/usr/local/bin/claude")
	if err != nil {
		t.Fatalf("SetCLIStatus failed: %v", err)
	}

	status := manager.GetStatus()
	if !status.CLIInstalled {
		t.Error("expected CLIInstalled to be true")
	}
	if status.CLIVersion != "1.0.0" {
		t.Errorf("expected CLIVersion '1.0.0', got '%s'", status.CLIVersion)
	}
	if status.CLIPath != "/usr/local/bin/claude" {
		t.Errorf("expected CLIPath '/usr/local/bin/claude', got '%s'", status.CLIPath)
	}
}

func TestFirstRunManager_SetCLISkipped(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewFirstRunManager(tmpDir)

	err := manager.SetCLISkipped(true)
	if err != nil {
		t.Fatalf("SetCLISkipped failed: %v", err)
	}

	status := manager.GetStatus()
	if !status.CLISkipped {
		t.Error("expected CLISkipped to be true")
	}
}

func TestFirstRunManager_SetOllamaStatus(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewFirstRunManager(tmpDir)

	models := []string{"llama3.2", "mistral"}
	err := manager.SetOllamaStatus(true, "http://localhost:11434", models)
	if err != nil {
		t.Fatalf("SetOllamaStatus failed: %v", err)
	}

	status := manager.GetStatus()
	if !status.OllamaDetected {
		t.Error("expected OllamaDetected to be true")
	}
	if status.OllamaEndpoint != "http://localhost:11434" {
		t.Errorf("expected OllamaEndpoint 'http://localhost:11434', got '%s'", status.OllamaEndpoint)
	}
	if len(status.OllamaModels) != 2 {
		t.Errorf("expected 2 models, got %d", len(status.OllamaModels))
	}
}

func TestFirstRunManager_SetProvidersConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewFirstRunManager(tmpDir)

	providers := []string{"anthropic", "ollama"}
	err := manager.SetProvidersConfigured(providers, "anthropic")
	if err != nil {
		t.Fatalf("SetProvidersConfigured failed: %v", err)
	}

	status := manager.GetStatus()
	if len(status.ProvidersConfigured) != 2 {
		t.Errorf("expected 2 providers, got %d", len(status.ProvidersConfigured))
	}
	if status.DefaultProvider != "anthropic" {
		t.Errorf("expected DefaultProvider 'anthropic', got '%s'", status.DefaultProvider)
	}
}

func TestFirstRunManager_SetStatisticsOptIn(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewFirstRunManager(tmpDir)

	err := manager.SetStatisticsOptIn(true)
	if err != nil {
		t.Fatalf("SetStatisticsOptIn failed: %v", err)
	}

	status := manager.GetStatus()
	if !status.StatisticsOptIn {
		t.Error("expected StatisticsOptIn to be true")
	}
}

func TestFirstRunManager_SetEnvironmentStatus(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewFirstRunManager(tmpDir)

	err := manager.SetEnvironmentStatus(true, false, true)
	if err != nil {
		t.Fatalf("SetEnvironmentStatus failed: %v", err)
	}

	status := manager.GetStatus()
	if !status.HasAnthropicKey {
		t.Error("expected HasAnthropicKey to be true")
	}
	if status.HasOpenAIKey {
		t.Error("expected HasOpenAIKey to be false")
	}
	if !status.CCSwitchActive {
		t.Error("expected CCSwitchActive to be true")
	}
}

func TestFirstRunManager_Reset(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewFirstRunManager(tmpDir)

	// Set some values
	manager.MarkComplete()
	manager.SetCLIStatus(true, "1.0.0", "/usr/local/bin/claude")

	// Reset
	err := manager.Reset()
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	status := manager.GetStatus()
	if status.FirstRunCompleted {
		t.Error("expected FirstRunCompleted to be false after reset")
	}
	if status.CLIInstalled {
		t.Error("expected CLIInstalled to be false after reset")
	}
}

func TestFirstRunManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manager and set values
	manager1 := NewFirstRunManager(tmpDir)
	manager1.MarkComplete()
	manager1.SetCLIStatus(true, "1.0.0", "/usr/local/bin/claude")
	manager1.SetStatisticsOptIn(true)

	// Create new manager and verify persistence
	manager2 := NewFirstRunManager(tmpDir)
	status := manager2.GetStatus()

	if !status.FirstRunCompleted {
		t.Error("expected FirstRunCompleted to persist")
	}
	if !status.CLIInstalled {
		t.Error("expected CLIInstalled to persist")
	}
	if status.CLIVersion != "1.0.0" {
		t.Errorf("expected CLIVersion '1.0.0' to persist, got '%s'", status.CLIVersion)
	}
	if !status.StatisticsOptIn {
		t.Error("expected StatisticsOptIn to persist")
	}
}

func TestFirstRunManager_StatusFile(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewFirstRunManager(tmpDir)

	manager.MarkComplete()

	// Check file exists
	statusFile := filepath.Join(tmpDir, "first_run_status.json")
	if _, err := os.Stat(statusFile); os.IsNotExist(err) {
		t.Error("expected status file to exist")
	}
}

func TestFirstRunManager_GetStoragePath(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewFirstRunManager(tmpDir)

	if manager.GetStoragePath() != tmpDir {
		t.Errorf("expected storage path %s, got %s", tmpDir, manager.GetStoragePath())
	}
}
