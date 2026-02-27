package server

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
)

func TestGetSmartToolSelection_DefaultTrue(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if !h.GetSmartToolSelection() {
		t.Fatalf("GetSmartToolSelection() = false, want true")
	}
}

func TestGetSmartToolSelection_StoredFalse(t *testing.T) {
	store := kvstore.NewMemoryStore()
	disabled := false
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		SmartToolSelection: &disabled,
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	h := NewSettingsHandler(store)
	if h.GetSmartToolSelection() {
		t.Fatalf("GetSmartToolSelection() = true, want false")
	}
}

func TestGetMemoryRecallMode_DefaultBalanced(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetMemoryRecallMode(); got != "balanced" {
		t.Fatalf("GetMemoryRecallMode() = %q, want %q", got, "balanced")
	}
}

func TestGetMemoryRecallMode_StoredValue(t *testing.T) {
	store := kvstore.NewMemoryStore()
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		MemoryRecallMode: "aggressive",
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	h := NewSettingsHandler(store)
	if got := h.GetMemoryRecallMode(); got != "aggressive" {
		t.Fatalf("GetMemoryRecallMode() = %q, want %q", got, "aggressive")
	}
}

func TestGetMemoryRecallMode_InvalidStoredValueFallback(t *testing.T) {
	store := kvstore.NewMemoryStore()
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		MemoryRecallMode: "unexpected-mode",
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	h := NewSettingsHandler(store)
	if got := h.GetMemoryRecallMode(); got != "balanced" {
		t.Fatalf("GetMemoryRecallMode() = %q, want %q", got, "balanced")
	}
}
