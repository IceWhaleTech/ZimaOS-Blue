package llm

import (
	"errors"
	"testing"
)

func TestDefaultErrorClassifier_InitializesPatternsOnDemand(t *testing.T) {
	classifier := NewDefaultErrorClassifier()
	if classifier.retryablePatterns != nil {
		t.Fatal("expected retryable patterns to start nil")
	}
	if classifier.nonRetryablePatterns != nil {
		t.Fatal("expected non-retryable patterns to start nil")
	}

	if got := classifier.Classify(errors.New("service temporarily unavailable")); got != ErrorTypeRetryable {
		t.Fatalf("Classify() = %v, want %v", got, ErrorTypeRetryable)
	}
	if len(classifier.retryablePatterns) == 0 {
		t.Fatal("expected retryable patterns to initialize on first classify")
	}
	if len(classifier.nonRetryablePatterns) == 0 {
		t.Fatal("expected non-retryable patterns to initialize on first classify")
	}
}
