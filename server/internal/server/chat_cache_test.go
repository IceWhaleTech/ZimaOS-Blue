package server

import (
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
)

func TestConversationCacheEnforceBudgets_PrefersOlderSequenceWhenTimestampsTie(t *testing.T) {
	cache := NewConversationCache(time.Minute, 4)
	now := time.Now()
	cache.maxBytes = 120
	cache.entries["conv-a"] = &cacheEntry{
		messages:  []memory.Message{{Role: "user", Content: "first"}},
		timestamp: now,
		sizeBytes: 60,
		sequence:  1,
	}
	cache.entries["conv-b"] = &cacheEntry{
		messages:  []memory.Message{{Role: "user", Content: "second"}},
		timestamp: now,
		sizeBytes: 80,
		sequence:  2,
	}
	cache.totalBytes = 140

	cache.mu.Lock()
	cache.enforceBudgetsLocked()
	_, hasA := cache.entries["conv-a"]
	_, hasB := cache.entries["conv-b"]
	totalBytes := cache.totalBytes
	cache.mu.Unlock()

	if hasA {
		t.Fatal("expected older sequence entry to be evicted first when timestamps tie")
	}
	if !hasB {
		t.Fatal("expected newer sequence entry to remain after eviction")
	}
	if totalBytes != 80 {
		t.Fatalf("totalBytes = %d, want 80", totalBytes)
	}
}
