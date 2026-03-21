package server

import (
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestConversationCacheCloseStopsCleanupLoop(t *testing.T) {
	cache := NewConversationCache(20*time.Millisecond, 4)
	cache.Set("conv-1", []memory.Message{{Role: "user", Content: "hello"}})

	cache.Close()

	select {
	case <-cache.doneCh:
	case <-time.After(time.Second):
		t.Fatal("cleanup loop did not stop after Close()")
	}

	if _, ok := cache.Get("conv-1"); ok {
		t.Fatal("expected closed cache to stop serving entries")
	}

	cache.Set("conv-2", []memory.Message{{Role: "user", Content: "after-close"}})
	if _, ok := cache.Get("conv-2"); ok {
		t.Fatal("expected closed cache to ignore later writes")
	}
}

func TestChatHandlerCloseClearsTransientCaches(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())

	handler.warmupMu.Lock()
	handler.warmupCache["conv-1"] = &warmupResult{}
	handler.warmupMu.Unlock()

	handler.warmupTokenMu.Lock()
	handler.warmupTokens["conv-1"] = "token"
	handler.warmupTokenMu.Unlock()

	handler.providerWarmupsMu.Lock()
	handler.providerWarmups["provider-1"] = &providerWarmupState{}
	handler.providerWarmupsMu.Unlock()

	handler.summaryCache.Put("conv-1", &ConversationSummary{Text: "cached summary"})
	handler.conversationCache.Set("conv-1", []memory.Message{{Role: "assistant", Content: "cached"}})

	handler.Close()

	if len(handler.warmupCache) != 0 {
		t.Fatalf("warmupCache size = %d, want 0", len(handler.warmupCache))
	}
	if len(handler.warmupTokens) != 0 {
		t.Fatalf("warmupTokens size = %d, want 0", len(handler.warmupTokens))
	}
	if len(handler.providerWarmups) != 0 {
		t.Fatalf("providerWarmups size = %d, want 0", len(handler.providerWarmups))
	}
	if _, ok := handler.summaryCache.Get("conv-1"); ok {
		t.Fatal("expected summaryCache to be cleared")
	}
	if _, ok := handler.conversationCache.Get("conv-1"); ok {
		t.Fatal("expected conversationCache to be closed and cleared")
	}
}
