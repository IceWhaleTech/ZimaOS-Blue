package tools

import (
	"sync"
	"testing"
)

func TestToolLoopRegexes_InitializeOnDemand(t *testing.T) {
	originalWhitespace := toolLoopWhitespaceRE
	originalDigits := toolLoopDigitsRE
	originalOnce := toolLoopRegexesOnce

	toolLoopWhitespaceRE = nil
	toolLoopDigitsRE = nil
	toolLoopRegexesOnce = sync.Once{}
	t.Cleanup(func() {
		toolLoopWhitespaceRE = originalWhitespace
		toolLoopDigitsRE = originalDigits
		toolLoopRegexesOnce = originalOnce
	})

	if toolLoopWhitespaceRE != nil || toolLoopDigitsRE != nil {
		t.Fatal("expected tool loop regexes to start nil")
	}

	got := normalizeToolLoopText("Step 123   still running")
	if got != "step # still running" {
		t.Fatalf("normalizeToolLoopText() = %q, want %q", got, "step # still running")
	}

	if toolLoopWhitespaceRE == nil || toolLoopDigitsRE == nil {
		t.Fatal("expected tool loop regexes to initialize on first normalization")
	}
}
