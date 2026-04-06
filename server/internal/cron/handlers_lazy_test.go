package cron

import (
	"sync"
	"testing"
)

func TestValidateCommand_InitializesDangerousPatternsOnDemand(t *testing.T) {
	originalPatterns := dangerousPatterns
	dangerousPatterns = nil
	dangerousPatternsOnce = sync.Once{}
	t.Cleanup(func() {
		dangerousPatterns = originalPatterns
		dangerousPatternsOnce = sync.Once{}
	})

	if dangerousPatterns != nil {
		t.Fatal("expected dangerous patterns to start nil")
	}

	if err := validateCommand("date"); err != nil {
		t.Fatalf("validateCommand(date) returned error: %v", err)
	}
	if len(dangerousPatterns) == 0 {
		t.Fatal("expected dangerous patterns to initialize on first validation")
	}
}

func TestValidateCommand_BlocksDangerousPatternAfterLazyInit(t *testing.T) {
	if err := validateCommand("echo hello; rm -rf /tmp/test"); err == nil {
		t.Fatal("expected dangerous command to be rejected")
	}
}
