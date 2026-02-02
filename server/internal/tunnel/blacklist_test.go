package tunnel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBlacklist_AddAndCheck(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	bl, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	// Test adding a provider to blacklist
	err = bl.Add(ProviderBore, "test failure")
	if err != nil {
		t.Fatalf("Failed to add provider to blacklist: %v", err)
	}

	// Check if provider is blacklisted
	if !bl.IsBlacklisted(ProviderBore) {
		t.Error("Provider should be blacklisted")
	}

	// Check if other provider is not blacklisted
	if bl.IsBlacklisted(ProviderServeo) {
		t.Error("Provider should not be blacklisted")
	}
}

func TestBlacklist_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create blacklist and add provider
	bl1, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	err = bl1.Add(ProviderBore, "test failure")
	if err != nil {
		t.Fatalf("Failed to add provider: %v", err)
	}

	// Create new blacklist instance (simulating restart)
	bl2, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create second blacklist: %v", err)
	}

	// Check if provider is still blacklisted
	if !bl2.IsBlacklisted(ProviderBore) {
		t.Error("Provider should still be blacklisted after reload")
	}
}

func TestBlacklist_Expiration(t *testing.T) {
	tmpDir := t.TempDir()

	bl, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	// Manually add an expired entry
	bl.mu.Lock()
	bl.entries[ProviderBore] = BlacklistEntry{
		Provider:      ProviderBore,
		BlacklistedAt: time.Now().Add(-25 * time.Hour), // 25 hours ago
		Reason:        "test",
	}
	bl.mu.Unlock()

	// Check if expired entry is not considered blacklisted
	if bl.IsBlacklisted(ProviderBore) {
		t.Error("Expired provider should not be blacklisted")
	}
}

func TestBlacklist_Remove(t *testing.T) {
	tmpDir := t.TempDir()

	bl, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	// Add provider
	err = bl.Add(ProviderBore, "test failure")
	if err != nil {
		t.Fatalf("Failed to add provider: %v", err)
	}

	// Verify it's blacklisted
	if !bl.IsBlacklisted(ProviderBore) {
		t.Error("Provider should be blacklisted")
	}

	// Remove provider
	err = bl.Remove(ProviderBore)
	if err != nil {
		t.Fatalf("Failed to remove provider: %v", err)
	}

	// Verify it's no longer blacklisted
	if bl.IsBlacklisted(ProviderBore) {
		t.Error("Provider should not be blacklisted after removal")
	}
}

func TestBlacklist_GetAll(t *testing.T) {
	tmpDir := t.TempDir()

	bl, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	// Add multiple providers
	providers := []Provider{ProviderBore, ProviderServeo, ProviderLocalTunnel}
	for _, p := range providers {
		err = bl.Add(p, "test failure")
		if err != nil {
			t.Fatalf("Failed to add provider %s: %v", p, err)
		}
	}

	// Get all entries
	entries := bl.GetAll()
	if len(entries) != len(providers) {
		t.Errorf("Expected %d entries, got %d", len(providers), len(entries))
	}

	// Verify all providers are in the list
	found := make(map[Provider]bool)
	for _, entry := range entries {
		found[entry.Provider] = true
	}

	for _, p := range providers {
		if !found[p] {
			t.Errorf("Provider %s not found in GetAll() results", p)
		}
	}
}

func TestBlacklist_CleanExpired(t *testing.T) {
	tmpDir := t.TempDir()

	bl, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	// Add current entry
	err = bl.Add(ProviderBore, "current failure")
	if err != nil {
		t.Fatalf("Failed to add current provider: %v", err)
	}

	// Add expired entry manually
	bl.mu.Lock()
	bl.entries[ProviderServeo] = BlacklistEntry{
		Provider:      ProviderServeo,
		BlacklistedAt: time.Now().Add(-25 * time.Hour),
		Reason:        "expired",
	}
	bl.mu.Unlock()

	// Clean expired entries
	bl.cleanExpired()

	// Verify current entry still exists
	if !bl.IsBlacklisted(ProviderBore) {
		t.Error("Current entry should still be blacklisted")
	}

	// Verify expired entry is removed
	if bl.IsBlacklisted(ProviderServeo) {
		t.Error("Expired entry should be removed")
	}
}

func TestBlacklist_FileFormat(t *testing.T) {
	tmpDir := t.TempDir()

	bl, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	// Add provider
	err = bl.Add(ProviderBore, "test failure")
	if err != nil {
		t.Fatalf("Failed to add provider: %v", err)
	}

	// Check if file exists
	filePath := filepath.Join(tmpDir, "tunnel_blacklist.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Blacklist file should exist")
	}

	// Read file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read blacklist file: %v", err)
	}

	// Verify it's valid JSON (basic check)
	if len(data) == 0 {
		t.Error("Blacklist file should not be empty")
	}
}
