package tools

import (
	"sync"
	"testing"
)

func TestValidateCommandSafety_InitializesDangerousPatternsOnDemand(t *testing.T) {
	originalPatterns := dangerousCommandPatterns
	dangerousCommandPatterns = nil
	dangerousCommandPatternsOnce = sync.Once{}
	t.Cleanup(func() {
		dangerousCommandPatterns = originalPatterns
		dangerousCommandPatternsOnce = sync.Once{}
	})

	if dangerousCommandPatterns != nil {
		t.Fatal("expected dangerous exec patterns to start nil")
	}

	if err := ValidateCommandSafety("echo safe"); err != nil {
		t.Fatalf("ValidateCommandSafety returned error: %v", err)
	}
	if len(dangerousCommandPatterns) == 0 {
		t.Fatal("expected dangerous exec patterns to initialize on first validation")
	}
}
