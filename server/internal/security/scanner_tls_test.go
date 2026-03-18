package security

import (
	"sync"
	"testing"
)

func TestSecurityScanner_TLSDisabledInProductionIsWarning(t *testing.T) {
	handler := NewHandler(nil)
	scanner := NewSecurityScanner(handler, &ScannerConfig{
		Environment: "production",
	})

	previousManager := globalTLSManager
	globalTLSManager = NewTLSManager(&TLSManagerConfig{})
	globalTLSManagerOnce = sync.Once{}
	globalTLSManagerOnce.Do(func() {})
	t.Cleanup(func() {
		globalTLSManager = previousManager
		globalTLSManagerOnce = sync.Once{}
		if previousManager != nil {
			globalTLSManagerOnce.Do(func() {})
		}
	})

	items := scanner.checkNetworkSecurity()

	for _, item := range items {
		if item.ID != "network_tls" {
			continue
		}
		if item.Status != "warning" {
			t.Fatalf("network_tls status = %q, want %q", item.Status, "warning")
		}
		if item.Details != "TLS is disabled in production. All traffic is unencrypted." {
			t.Fatalf("network_tls details = %q", item.Details)
		}
		return
	}

	t.Fatal("network_tls item not found")
}
