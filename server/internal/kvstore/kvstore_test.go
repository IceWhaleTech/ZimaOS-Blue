package kvstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// Test Store interface
func TestStoreInterface(t *testing.T) {
	var _ Store = (*MemoryStore)(nil)
	var _ Store = (*SQLiteStore)(nil)
}

// Test MemoryStore creation
func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	if store == nil {
		t.Fatal("expected store, got nil")
	}
}

// Test MemoryStore Set and Get
func TestMemoryStoreSetGet(t *testing.T) {
	store := NewMemoryStore()

	err := store.Set(context.Background(), "key1", "value1", 0)
	if err != nil {
		t.Fatalf("failed to set: %v", err)
	}

	value, err := store.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}

	if value != "value1" {
		t.Errorf("expected 'value1', got '%v'", value)
	}
}

// Test MemoryStore Get nonexistent
func TestMemoryStoreGetNonexistent(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

// Test MemoryStore Delete
func TestMemoryStoreDelete(t *testing.T) {
	store := NewMemoryStore()

	store.Set(context.Background(), "key1", "value1", 0)
	err := store.Delete(context.Background(), "key1")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	_, err = store.Get(context.Background(), "key1")
	if err != ErrKeyNotFound {
		t.Error("expected key to be deleted")
	}
}

// Test MemoryStore Exists
func TestMemoryStoreExists(t *testing.T) {
	store := NewMemoryStore()

	exists, _ := store.Exists(context.Background(), "key1")
	if exists {
		t.Error("expected key to not exist")
	}

	store.Set(context.Background(), "key1", "value1", 0)

	exists, _ = store.Exists(context.Background(), "key1")
	if !exists {
		t.Error("expected key to exist")
	}
}

// Test MemoryStore TTL
func TestMemoryStoreTTL(t *testing.T) {
	store := NewMemoryStore()

	// Set with 100ms TTL
	err := store.Set(context.Background(), "key1", "value1", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to set: %v", err)
	}

	// Should exist immediately
	value, err := store.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	if value != "value1" {
		t.Errorf("expected 'value1', got '%v'", value)
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, err = store.Get(context.Background(), "key1")
	if err != ErrKeyNotFound {
		t.Error("expected key to be expired")
	}
}

// Test MemoryStore Keys
func TestMemoryStoreKeys(t *testing.T) {
	store := NewMemoryStore()

	store.Set(context.Background(), "key1", "value1", 0)
	store.Set(context.Background(), "key2", "value2", 0)
	store.Set(context.Background(), "other", "value3", 0)

	keys, err := store.Keys(context.Background(), "key*")
	if err != nil {
		t.Fatalf("failed to get keys: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

// Test MemoryStore Clear
func TestMemoryStoreClear(t *testing.T) {
	store := NewMemoryStore()

	store.Set(context.Background(), "key1", "value1", 0)
	store.Set(context.Background(), "key2", "value2", 0)

	err := store.Clear(context.Background())
	if err != nil {
		t.Fatalf("failed to clear: %v", err)
	}

	keys, _ := store.Keys(context.Background(), "*")
	if len(keys) != 0 {
		t.Errorf("expected 0 keys after clear, got %d", len(keys))
	}
}

// Test MemoryStore SetJSON and GetJSON
func TestMemoryStoreJSON(t *testing.T) {
	store := NewMemoryStore()

	data := map[string]interface{}{
		"name": "test",
		"age":  30,
	}

	err := store.SetJSON(context.Background(), "json_key", data, 0)
	if err != nil {
		t.Fatalf("failed to set JSON: %v", err)
	}

	var result map[string]interface{}
	err = store.GetJSON(context.Background(), "json_key", &result)
	if err != nil {
		t.Fatalf("failed to get JSON: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("expected name 'test', got '%v'", result["name"])
	}
}

// Test SQLiteStore creation
func TestNewSQLiteStore(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("expected store, got nil")
	}
}

func TestNewSQLiteStoreUsesReaderPoolForFileDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kvstore.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create file-backed store: %v", err)
	}
	defer store.Close()

	if store.readDB == nil {
		t.Fatal("expected read db to be initialized")
	}
	if store.readDB == store.db {
		t.Fatal("expected file-backed store to use a separate read db")
	}

	if err := store.Set(context.Background(), "key1", "value1", 0); err != nil {
		t.Fatalf("failed to set key: %v", err)
	}

	if err := store.db.Close(); err != nil {
		t.Fatalf("failed to close writer db: %v", err)
	}

	value, err := store.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("failed to read through reader db: %v", err)
	}
	if value != "value1" {
		t.Fatalf("expected value1, got %v", value)
	}

	keys, err := store.Keys(context.Background(), "key%")
	if err != nil {
		t.Fatalf("failed to list keys through reader db: %v", err)
	}
	if len(keys) != 1 || keys[0] != "key1" {
		t.Fatalf("unexpected keys through reader db: %+v", keys)
	}
}

// Test SQLiteStore Set and Get
func TestSQLiteStoreSetGet(t *testing.T) {
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	err := store.Set(context.Background(), "key1", "value1", 0)
	if err != nil {
		t.Fatalf("failed to set: %v", err)
	}

	value, err := store.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}

	if value != "value1" {
		t.Errorf("expected 'value1', got '%v'", value)
	}
}

// Test SQLiteStore Get nonexistent
func TestSQLiteStoreGetNonexistent(t *testing.T) {
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	_, err := store.Get(context.Background(), "nonexistent")
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

// Test SQLiteStore Delete
func TestSQLiteStoreDelete(t *testing.T) {
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	store.Set(context.Background(), "key1", "value1", 0)
	err := store.Delete(context.Background(), "key1")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	_, err = store.Get(context.Background(), "key1")
	if err != ErrKeyNotFound {
		t.Error("expected key to be deleted")
	}
}

// Test SQLiteStore TTL
func TestSQLiteStoreTTL(t *testing.T) {
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	err := store.Set(context.Background(), "key1", "value1", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to set: %v", err)
	}

	// Should exist immediately
	_, err = store.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, err = store.Get(context.Background(), "key1")
	if err != ErrKeyNotFound {
		t.Error("expected key to be expired")
	}
}

// Test SQLiteStore Keys
func TestSQLiteStoreKeys(t *testing.T) {
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	store.Set(context.Background(), "key1", "value1", 0)
	store.Set(context.Background(), "key2", "value2", 0)
	store.Set(context.Background(), "other", "value3", 0)

	keys, err := store.Keys(context.Background(), "key%")
	if err != nil {
		t.Fatalf("failed to get keys: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

// Test SQLiteStore Exists
func TestSQLiteStoreExists(t *testing.T) {
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	exists, _ := store.Exists(context.Background(), "key1")
	if exists {
		t.Error("expected key to not exist")
	}

	store.Set(context.Background(), "key1", "value1", 0)

	exists, _ = store.Exists(context.Background(), "key1")
	if !exists {
		t.Error("expected key to exist")
	}
}

// Test SQLiteStore Clear
func TestSQLiteStoreClear(t *testing.T) {
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	store.Set(context.Background(), "key1", "value1", 0)
	store.Set(context.Background(), "key2", "value2", 0)

	err := store.Clear(context.Background())
	if err != nil {
		t.Fatalf("failed to clear: %v", err)
	}

	keys, _ := store.Keys(context.Background(), "%")
	if len(keys) != 0 {
		t.Errorf("expected 0 keys after clear, got %d", len(keys))
	}
}

// Test SQLiteStore SetJSON and GetJSON
func TestSQLiteStoreJSON(t *testing.T) {
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	data := map[string]interface{}{
		"name": "test",
		"age":  30,
	}

	err := store.SetJSON(context.Background(), "json_key", data, 0)
	if err != nil {
		t.Fatalf("failed to set JSON: %v", err)
	}

	var result map[string]interface{}
	err = store.GetJSON(context.Background(), "json_key", &result)
	if err != nil {
		t.Fatalf("failed to get JSON: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("expected name 'test', got '%v'", result["name"])
	}
}

// Test SQLiteStore Update (upsert)
func TestSQLiteStoreUpdate(t *testing.T) {
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	store.Set(context.Background(), "key1", "value1", 0)
	store.Set(context.Background(), "key1", "value2", 0) // Update

	value, _ := store.Get(context.Background(), "key1")
	if value != "value2" {
		t.Errorf("expected 'value2', got '%v'", value)
	}
}

// Test concurrent access
func TestSQLiteStoreConcurrentAccess(t *testing.T) {
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			key := "key"
			store.Set(context.Background(), key, i, 0)
			store.Get(context.Background(), key)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
