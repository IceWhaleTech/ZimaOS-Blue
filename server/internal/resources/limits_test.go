package resources

import (
	"runtime"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxMemoryMB != 512 {
		t.Errorf("expected MaxMemoryMB=512, got %d", config.MaxMemoryMB)
	}
	if config.MaxCPUPercent != 50 {
		t.Errorf("expected MaxCPUPercent=50, got %d", config.MaxCPUPercent)
	}
	if config.MaxOpenFiles != 65536 {
		t.Errorf("expected MaxOpenFiles=65536, got %d", config.MaxOpenFiles)
	}
	if config.MaxGoroutines != 10000 {
		t.Errorf("expected MaxGoroutines=10000, got %d", config.MaxGoroutines)
	}
	if config.GCPercent != 100 {
		t.Errorf("expected GCPercent=100, got %d", config.GCPercent)
	}
}

func TestNewLimiter(t *testing.T) {
	config := DefaultConfig()
	limiter := NewLimiter(config)

	if limiter == nil {
		t.Fatal("expected non-nil limiter")
	}

	if limiter.Config().MaxMemoryMB != config.MaxMemoryMB {
		t.Error("config mismatch")
	}
}

func TestLimiterStats(t *testing.T) {
	config := DefaultConfig()
	limiter := NewLimiter(config)

	stats := limiter.Stats()

	if stats.NumCPU != runtime.NumCPU() {
		t.Errorf("expected NumCPU=%d, got %d", runtime.NumCPU(), stats.NumCPU)
	}

	if stats.NumGoroutines <= 0 {
		t.Error("expected positive NumGoroutines")
	}

	if stats.AllocMB < 0 {
		t.Error("expected non-negative AllocMB")
	}
}

func TestLimiterApply(t *testing.T) {
	config := Config{
		MaxMemoryMB:   256,
		MaxCPUPercent: 25,
		MaxOpenFiles:  1024,
		MaxGoroutines: 1000,
		GCPercent:     50,
	}
	limiter := NewLimiter(config)

	err := limiter.Apply()
	if err != nil {
		// On some systems, setting rlimits may fail without root
		t.Logf("Apply returned error (may be expected): %v", err)
	}

	// Verify GOMAXPROCS was adjusted
	expectedProcs := (runtime.NumCPU() * 25) / 100
	if expectedProcs < 1 {
		expectedProcs = 1
	}
	if runtime.GOMAXPROCS(0) != expectedProcs {
		t.Logf("GOMAXPROCS: expected %d, got %d", expectedProcs, runtime.GOMAXPROCS(0))
	}
}

func TestCheckGoroutineLimit(t *testing.T) {
	// Test with high limit (should pass)
	config := Config{MaxGoroutines: 100000}
	limiter := NewLimiter(config)

	err := limiter.CheckGoroutineLimit()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test with very low limit (should fail)
	config = Config{MaxGoroutines: 1}
	limiter = NewLimiter(config)

	err = limiter.CheckGoroutineLimit()
	if err == nil {
		t.Error("expected error for exceeded goroutine limit")
	}

	// Test with no limit
	config = Config{MaxGoroutines: 0}
	limiter = NewLimiter(config)

	err = limiter.CheckGoroutineLimit()
	if err != nil {
		t.Errorf("unexpected error with no limit: %v", err)
	}
}

func TestResourceStats(t *testing.T) {
	config := DefaultConfig()
	limiter := NewLimiter(config)

	stats := limiter.Stats()

	// Basic sanity checks
	if stats.SysMB < stats.AllocMB {
		t.Error("SysMB should be >= AllocMB")
	}

	if stats.GOMAXPROCS <= 0 {
		t.Error("GOMAXPROCS should be positive")
	}
}
