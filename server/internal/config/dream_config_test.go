package config

import "testing"

func TestDefaultsIncludeDreamMemoryConfig(t *testing.T) {
	cfg := defaults()

	if !cfg.Memory.Dream.Enabled {
		t.Fatal("defaults().Memory.Dream.Enabled = false, want true")
	}
	if cfg.Memory.Dream.Schedule != "0 30 3 * * *" {
		t.Fatalf("defaults().Memory.Dream.Schedule = %q, want %q", cfg.Memory.Dream.Schedule, "0 30 3 * * *")
	}
	if cfg.Memory.Dream.SessionMinMessages <= 0 {
		t.Fatalf("defaults().Memory.Dream.SessionMinMessages = %d, want > 0", cfg.Memory.Dream.SessionMinMessages)
	}
	if cfg.Memory.Dream.MaxPromotionsPerRun <= 0 {
		t.Fatalf("defaults().Memory.Dream.MaxPromotionsPerRun = %d, want > 0", cfg.Memory.Dream.MaxPromotionsPerRun)
	}
}
