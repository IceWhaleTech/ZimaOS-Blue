package tunnel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBlacklist_AddAndCheck(t *testing.T) {
	tmpDir := t.TempDir()

	bl, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	err = bl.Add(ProviderCloudflare, "test failure")
	if err != nil {
		t.Fatalf("Failed to add provider to blacklist: %v", err)
	}

	if !bl.IsBlacklisted(ProviderCloudflare) {
		t.Error("Provider should be blacklisted")
	}

	if bl.IsBlacklisted(ProviderNgrok) {
		t.Error("Provider should not be blacklisted")
	}
}

func TestBlacklist_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	bl1, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	err = bl1.Add(ProviderCloudflare, "test failure")
	if err != nil {
		t.Fatalf("Failed to add provider: %v", err)
	}

	bl2, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create second blacklist: %v", err)
	}

	if !bl2.IsBlacklisted(ProviderCloudflare) {
		t.Error("Provider should still be blacklisted after reload")
	}
}

func TestBlacklist_Expiration(t *testing.T) {
	tmpDir := t.TempDir()

	bl, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	bl.mu.Lock()
	bl.entries[ProviderCloudflare] = BlacklistEntry{
		Provider:      ProviderCloudflare,
		BlacklistedAt: time.Now().Add(-25 * time.Hour),
		Reason:        "test",
	}
	bl.mu.Unlock()

	if bl.IsBlacklisted(ProviderCloudflare) {
		t.Error("Expired provider should not be blacklisted")
	}
}

func TestBlacklist_Remove(t *testing.T) {
	tmpDir := t.TempDir()

	bl, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	err = bl.Add(ProviderCloudflare, "test failure")
	if err != nil {
		t.Fatalf("Failed to add provider: %v", err)
	}

	if !bl.IsBlacklisted(ProviderCloudflare) {
		t.Error("Provider should be blacklisted")
	}

	err = bl.Remove(ProviderCloudflare)
	if err != nil {
		t.Fatalf("Failed to remove provider: %v", err)
	}

	if bl.IsBlacklisted(ProviderCloudflare) {
		t.Error("Provider should not be blacklisted after removal")
	}
}

func TestBlacklist_GetAll(t *testing.T) {
	tmpDir := t.TempDir()

	bl, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	providers := []Provider{ProviderCloudflare, ProviderNgrok}
	for _, p := range providers {
		err = bl.Add(p, "test failure")
		if err != nil {
			t.Fatalf("Failed to add provider %s: %v", p, err)
		}
	}

	entries := bl.GetAll()
	if len(entries) != len(providers) {
		t.Errorf("Expected %d entries, got %d", len(providers), len(entries))
	}

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

	err = bl.Add(ProviderCloudflare, "current failure")
	if err != nil {
		t.Fatalf("Failed to add current provider: %v", err)
	}

	bl.mu.Lock()
	bl.entries[ProviderNgrok] = BlacklistEntry{
		Provider:      ProviderNgrok,
		BlacklistedAt: time.Now().Add(-25 * time.Hour),
		Reason:        "expired",
	}
	bl.mu.Unlock()

	bl.cleanExpired()

	if !bl.IsBlacklisted(ProviderCloudflare) {
		t.Error("Current entry should still be blacklisted")
	}

	if bl.IsBlacklisted(ProviderNgrok) {
		t.Error("Expired entry should be removed")
	}
}

func TestBlacklist_FileFormat(t *testing.T) {
	tmpDir := t.TempDir()

	bl, err := NewBlacklist(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create blacklist: %v", err)
	}

	err = bl.Add(ProviderCloudflare, "test failure")
	if err != nil {
		t.Fatalf("Failed to add provider: %v", err)
	}

	filePath := filepath.Join(tmpDir, "tunnel_blacklist.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Blacklist file should exist")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read blacklist file: %v", err)
	}

	if len(data) == 0 {
		t.Error("Blacklist file should not be empty")
	}
}
