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

func TestGetSmartSkillSelection_DefaultTrue(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if !h.GetSmartSkillSelection() {
		t.Fatalf("GetSmartSkillSelection() = false, want true")
	}
}

func TestGetSkillSelectorMode_DefaultHybrid(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetSkillSelectorMode(); got != "hybrid" {
		t.Fatalf("GetSkillSelectorMode() = %q, want %q", got, "hybrid")
	}
}

func TestGetSkillRerankEnabled_DefaultTrue(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if !h.GetSkillRerankEnabled() {
		t.Fatalf("GetSkillRerankEnabled() = false, want true")
	}
}

func TestGetSkillRerankONNXEnabled_DefaultFalse(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if h.GetSkillRerankONNXEnabled() {
		t.Fatalf("GetSkillRerankONNXEnabled() = true, want false")
	}
}

func TestGetSkillRerankONNXAutoDownload_DefaultFalse(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if h.GetSkillRerankONNXAutoDownload() {
		t.Fatalf("GetSkillRerankONNXAutoDownload() = true, want false")
	}
}

func TestGetSkillSelectorConfidenceThreshold_Default(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetSkillSelectorConfidenceThreshold(); got != 0.78 {
		t.Fatalf("GetSkillSelectorConfidenceThreshold() = %v, want 0.78", got)
	}
}

func TestGetAgentAskTimeoutSeconds_Default(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetAgentAskTimeoutSeconds(); got != 120 {
		t.Fatalf("GetAgentAskTimeoutSeconds() = %d, want 120", got)
	}
}

func TestGetAgentAskTimeoutSeconds_Clamped(t *testing.T) {
	store := kvstore.NewMemoryStore()
	v := 3
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		AgentAskTimeoutSeconds: &v,
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	h := NewSettingsHandler(store)
	if got := h.GetAgentAskTimeoutSeconds(); got != 15 {
		t.Fatalf("GetAgentAskTimeoutSeconds() = %d, want 15", got)
	}
}

func TestGetAgentAskTimeoutAction_Default(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetAgentAskTimeoutAction(); got != "default" {
		t.Fatalf("GetAgentAskTimeoutAction() = %q, want %q", got, "default")
	}
}

func TestGetAgentAskTimeoutAction_StoredError(t *testing.T) {
	store := kvstore.NewMemoryStore()
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		AgentAskTimeoutAction: "error",
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	h := NewSettingsHandler(store)
	if got := h.GetAgentAskTimeoutAction(); got != "error" {
		t.Fatalf("GetAgentAskTimeoutAction() = %q, want %q", got, "error")
	}
}
