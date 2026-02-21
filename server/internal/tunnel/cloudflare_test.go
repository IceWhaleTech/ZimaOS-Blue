package tunnel

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// TestTolerantRegisterer_IgnoresDuplicates verifies that registering the same
// collector twice does not return an error (the core fix for the tunnel restart panic).
func TestTolerantRegisterer_IgnoresDuplicates(t *testing.T) {
	reg := prometheus.NewRegistry()
	tolerant := tolerantRegisterer{reg}

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_tolerant_dup",
		Help: "test",
	})

	// First registration should succeed
	if err := tolerant.Register(counter); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	// Second registration should be silently ignored
	if err := tolerant.Register(counter); err != nil {
		t.Fatalf("duplicate Register should be ignored, got: %v", err)
	}
}

// TestTolerantRegisterer_MustRegisterNoPanic verifies MustRegister does not
// panic on duplicate collectors (this is the exact code path that crashed).
func TestTolerantRegisterer_MustRegisterNoPanic(t *testing.T) {
	reg := prometheus.NewRegistry()
	tolerant := tolerantRegisterer{reg}

	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_tolerant_must",
		Help: "test",
	})

	// Should not panic even when called twice
	tolerant.MustRegister(gauge)
	tolerant.MustRegister(gauge)
}

// TestTolerantRegisterer_PropagatesRealErrors ensures non-duplicate errors
// are still returned (e.g. nil collector).
func TestTolerantRegisterer_PropagatesRealErrors(t *testing.T) {
	reg := prometheus.NewRegistry()
	tolerant := tolerantRegisterer{reg}

	// Register two different collectors with the same name → not AlreadyRegisteredError
	// but an inconsistent descriptor error
	c1 := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_conflict", Help: "a"})
	c2 := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_conflict", Help: "b"})

	if err := tolerant.Register(c1); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err := tolerant.Register(c2)
	if err == nil {
		t.Fatal("expected error for conflicting collector, got nil")
	}
	// Should NOT be silenced — it's not AlreadyRegisteredError
	if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
		t.Fatal("conflicting collector should not produce AlreadyRegisteredError")
	}
}

// TestInstallTolerantRegisterer_Idempotent verifies sync.Once prevents double-wrapping.
func TestInstallTolerantRegisterer_Idempotent(t *testing.T) {
	// Save and restore global state
	origReg := prometheus.DefaultRegisterer
	origOnce := tolerantOnce
	defer func() {
		prometheus.DefaultRegisterer = origReg
		tolerantOnce = origOnce
	}()

	// Reset for test
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	tolerantOnce = sync.Once{}

	installTolerantRegisterer()
	first := prometheus.DefaultRegisterer

	installTolerantRegisterer()
	second := prometheus.DefaultRegisterer

	if first != second {
		t.Fatal("installTolerantRegisterer should be idempotent (sync.Once)")
	}

	// Verify it's actually a tolerantRegisterer
	if _, ok := first.(tolerantRegisterer); !ok {
		t.Fatalf("expected tolerantRegisterer, got %T", first)
	}
}
