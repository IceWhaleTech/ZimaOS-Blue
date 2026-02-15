package memory

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewMemoryEntry(t *testing.T) {
	e := NewMemoryEntry("test-ns", "hello world")

	if e.Namespace != "test-ns" {
		t.Errorf("Namespace = %q, want %q", e.Namespace, "test-ns")
	}
	if e.Content != "hello world" {
		t.Errorf("Content = %q, want %q", e.Content, "hello world")
	}
	if e.Version != 1 {
		t.Errorf("Version = %d, want 1", e.Version)
	}
	if e.Status != EntryStatusActive {
		t.Errorf("Status = %q, want %q", e.Status, EntryStatusActive)
	}
	if e.ID == "" {
		t.Error("ID should not be empty")
	}
	if e.ExpiresAt != nil {
		t.Error("ExpiresAt should be nil for permanent entry")
	}
}

func TestComputeExpiresAt(t *testing.T) {
	e := NewMemoryEntry("ns", "content")

	// No TTL → no expiration
	e.ComputeExpiresAt()
	if e.ExpiresAt != nil {
		t.Error("ExpiresAt should be nil when TTL is 0")
	}

	// With TTL
	e.TTL = Duration(24 * time.Hour)
	e.ComputeExpiresAt()
	if e.ExpiresAt == nil {
		t.Fatal("ExpiresAt should not be nil when TTL is set")
	}
	expected := e.CreatedAt.Add(24 * time.Hour)
	if !e.ExpiresAt.Equal(expected) {
		t.Errorf("ExpiresAt = %v, want %v", e.ExpiresAt, expected)
	}
}

func TestIsExpired(t *testing.T) {
	e := NewMemoryEntry("ns", "content")

	// No TTL → never expires
	if e.IsExpired() {
		t.Error("entry without TTL should not be expired")
	}

	// Future expiration
	future := time.Now().Add(1 * time.Hour)
	e.ExpiresAt = &future
	if e.IsExpired() {
		t.Error("entry with future ExpiresAt should not be expired")
	}

	// Past expiration
	past := time.Now().Add(-1 * time.Hour)
	e.ExpiresAt = &past
	if !e.IsExpired() {
		t.Error("entry with past ExpiresAt should be expired")
	}
}

func TestNewVersion(t *testing.T) {
	e := NewMemoryEntry("ns", "original content")
	e.Category = "fact"
	e.Tags = []string{"test"}
	e.Source = "agent:chat"

	v2 := e.NewVersion("updated content")

	if v2.Version != 2 {
		t.Errorf("Version = %d, want 2", v2.Version)
	}
	if v2.ParentID != e.ID {
		t.Errorf("ParentID = %q, want %q", v2.ParentID, e.ID)
	}
	if v2.Content != "updated content" {
		t.Errorf("Content = %q, want %q", v2.Content, "updated content")
	}
	if v2.Category != "fact" {
		t.Errorf("Category not inherited: %q", v2.Category)
	}
	if v2.Namespace != "ns" {
		t.Errorf("Namespace not inherited: %q", v2.Namespace)
	}
	if v2.ID == e.ID {
		t.Error("new version should have a different ID")
	}
}

func TestSoftDelete(t *testing.T) {
	e := NewMemoryEntry("ns", "content")
	e.SoftDelete()

	if e.Status != EntryStatusDeleted {
		t.Errorf("Status = %q, want %q", e.Status, EntryStatusDeleted)
	}
	if e.DeletedAt == nil {
		t.Error("DeletedAt should be set after soft delete")
	}
}

func TestDurationJSON(t *testing.T) {
	d := Duration(24 * time.Hour)
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"24h0m0s"` {
		t.Errorf("JSON = %s, want %q", b, "24h0m0s")
	}

	var d2 Duration
	if err := json.Unmarshal([]byte(`"30m"`), &d2); err != nil {
		t.Fatal(err)
	}
	if time.Duration(d2) != 30*time.Minute {
		t.Errorf("Duration = %v, want 30m", time.Duration(d2))
	}
}

func TestNewEntryID(t *testing.T) {
	id1 := NewEntryID()
	id2 := NewEntryID()
	if id1 == id2 {
		t.Error("IDs should be unique")
	}
	if len(id1) < 4 || id1[:4] != "mem_" {
		t.Errorf("ID should start with mem_, got %q", id1)
	}
}
