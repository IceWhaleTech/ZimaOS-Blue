package tools

import (
	"testing"
	"time"
)

func TestNewGuardedMediaHTTPClientUsesLongerDefaultTimeout(t *testing.T) {
	client := newGuardedMediaHTTPClient(0)
	if client == nil {
		t.Fatal("expected http client")
	}
	if client.Timeout != 5*time.Minute {
		t.Fatalf("timeout = %v, want %v", client.Timeout, 5*time.Minute)
	}
}
