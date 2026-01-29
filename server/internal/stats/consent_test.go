package stats

import (
	"testing"
)

func TestConsentManager_SetConsent(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, false)
	manager := NewConsentManager(tmpDir, collector)

	// Initially not consented
	if manager.IsConsented() {
		t.Error("IsConsented() should be false initially")
	}

	// Set consent
	err := manager.SetConsent(true)
	if err != nil {
		t.Fatalf("SetConsent(true) error = %v", err)
	}

	if !manager.IsConsented() {
		t.Error("IsConsented() should be true after SetConsent(true)")
	}

	// Collector should be enabled
	if !collector.IsEnabled() {
		t.Error("Collector should be enabled after consent")
	}

	// Revoke consent
	err = manager.SetConsent(false)
	if err != nil {
		t.Fatalf("SetConsent(false) error = %v", err)
	}

	if manager.IsConsented() {
		t.Error("IsConsented() should be false after SetConsent(false)")
	}

	// Collector should be disabled
	if collector.IsEnabled() {
		t.Error("Collector should be disabled after revoking consent")
	}
}

func TestConsentManager_GetStatus(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewConsentManager(tmpDir, nil)

	status := manager.GetStatus()
	if status == nil {
		t.Fatal("GetStatus() returned nil")
	}

	if status.Consented {
		t.Error("Consented should be false initially")
	}

	if status.Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestConsentManager_RevokeConsent(t *testing.T) {
	tmpDir := t.TempDir()
	collector := NewStatisticsCollector(tmpDir, true)
	manager := NewConsentManager(tmpDir, collector)

	// Set consent and record some data
	manager.SetConsent(true)
	collector.Record(&APICallEvent{
		Provider:     "anthropic",
		Model:        "claude-3-sonnet",
		InputTokens:  100,
		OutputTokens: 200,
		Success:      true,
	})

	if collector.GetEventCount() != 1 {
		t.Errorf("GetEventCount() = %d, want 1", collector.GetEventCount())
	}

	// Revoke consent and clear data
	err := manager.RevokeConsent(true)
	if err != nil {
		t.Fatalf("RevokeConsent(true) error = %v", err)
	}

	if manager.IsConsented() {
		t.Error("IsConsented() should be false after RevokeConsent")
	}

	if collector.GetEventCount() != 0 {
		t.Errorf("GetEventCount() = %d, want 0 after clearing", collector.GetEventCount())
	}
}

func TestConsentManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manager and set consent
	manager1 := NewConsentManager(tmpDir, nil)
	manager1.SetConsent(true)

	// Create new manager with same path
	manager2 := NewConsentManager(tmpDir, nil)

	// Should load persisted consent
	if !manager2.IsConsented() {
		t.Error("Consent should be persisted and loaded")
	}
}

func TestGetConsentInfo(t *testing.T) {
	info := GetConsentInfo()

	if info == nil {
		t.Fatal("GetConsentInfo() returned nil")
	}

	if info.Title == "" {
		t.Error("Title should not be empty")
	}

	if info.Description == "" {
		t.Error("Description should not be empty")
	}

	if len(info.DataTypes) == 0 {
		t.Error("DataTypes should not be empty")
	}

	if info.Purpose == "" {
		t.Error("Purpose should not be empty")
	}

	if info.Version == "" {
		t.Error("Version should not be empty")
	}
}
