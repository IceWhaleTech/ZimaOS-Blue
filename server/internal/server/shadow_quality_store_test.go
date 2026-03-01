package server

import (
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
)

func TestShadowQualityStoreAppendCapAndReload(t *testing.T) {
	kv := kvstore.NewMemoryStore()
	store := NewShadowQualityStore(kv, 2)

	if err := store.Append(ShadowQualitySample{Scene: "short_qa_shadow", Delta: 0.1, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("append #1: %v", err)
	}
	if err := store.Append(ShadowQualitySample{Scene: "short_qa_shadow", Delta: 0.2, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("append #2: %v", err)
	}
	if err := store.Append(ShadowQualitySample{Scene: "tool_dispatch_shadow", Delta: 1, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("append #3: %v", err)
	}

	snap := store.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("snapshot len = %d, want 2", len(snap))
	}
	if snap[0].Delta != 0.2 || snap[1].Delta != 1 {
		t.Fatalf("unexpected retained samples: %+v", snap)
	}

	reloaded := NewShadowQualityStore(kv, 2)
	snap2 := reloaded.Snapshot()
	if len(snap2) != 2 {
		t.Fatalf("reloaded snapshot len = %d, want 2", len(snap2))
	}
	if snap2[0].Delta != 0.2 || snap2[1].Delta != 1 {
		t.Fatalf("unexpected reloaded samples: %+v", snap2)
	}
}

func TestShadowQualityStoreReset(t *testing.T) {
	kv := kvstore.NewMemoryStore()
	store := NewShadowQualityStore(kv, 4)
	if err := store.Append(ShadowQualitySample{Scene: "short_qa_shadow", Delta: 0.3}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := store.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if got := len(store.Snapshot()); got != 0 {
		t.Fatalf("snapshot len after reset = %d, want 0", got)
	}

	reloaded := NewShadowQualityStore(kv, 4)
	if got := len(reloaded.Snapshot()); got != 0 {
		t.Fatalf("reloaded snapshot len = %d, want 0", got)
	}
}
