package config

import (
	"testing"
	"time"
)

func TestDefaultHarnessConfig_InitializesRuntimeReflectionDefaults(t *testing.T) {
	cfg := DefaultHarnessConfig()
	if cfg == nil {
		t.Fatal("expected default harness config")
	}
	if !cfg.RuntimeReflection.Enabled {
		t.Fatal("expected runtime reflection to be enabled by default")
	}
	if got := cfg.RuntimeReflection.IntervalToolFinishes; got != 10 {
		t.Fatalf("interval_tool_finishes = %d, want 10", got)
	}
	if got := cfg.RuntimeReflection.MinReviewGap; got != 30*time.Second {
		t.Fatalf("min_review_gap = %v, want 30s", got)
	}
	if got := cfg.RuntimeReflection.RepeatWindow; got != 8 {
		t.Fatalf("repeat_window = %d, want 8", got)
	}
	if got := cfg.RuntimeReflection.MaxEvidenceEvents; got != 8 {
		t.Fatalf("max_evidence_events = %d, want 8", got)
	}
}
