package cgroup

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Enabled {
		t.Error("expected Enabled=false by default")
	}
	if config.CgroupRoot != DefaultCgroupRoot {
		t.Errorf("expected CgroupRoot=%s, got %s", DefaultCgroupRoot, config.CgroupRoot)
	}
	if config.CgroupName != DefaultCgroupName {
		t.Errorf("expected CgroupName=%s, got %s", DefaultCgroupName, config.CgroupName)
	}
	if config.IO.Enabled {
		t.Error("expected IO.Enabled=false by default")
	}
	if config.Memory.Enabled {
		t.Error("expected Memory.Enabled=false by default")
	}
	if config.CPU.Enabled {
		t.Error("expected CPU.Enabled=false by default")
	}
	if config.CPU.Weight != 100 {
		t.Errorf("expected CPU.Weight=100, got %d", config.CPU.Weight)
	}
}

func TestNewManagerDisabled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if manager == nil {
		t.Fatal("expected non-nil manager")
	}
	if manager.IsEnabled() {
		t.Error("expected manager to be disabled")
	}
}

func TestNewManagerEnabled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true

	manager, err := NewManager(config)

	// On non-Linux systems, this should return ErrCgroupNotSupported
	if !IsCgroupV2Available(config.CgroupRoot) {
		if err != ErrCgroupNotSupported {
			t.Errorf("expected ErrCgroupNotSupported on non-Linux, got: %v", err)
		}
		return
	}

	// On Linux with cgroup v2
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if manager == nil {
		t.Fatal("expected non-nil manager")
	}
	if !manager.IsEnabled() {
		t.Error("expected manager to be enabled")
	}
}

func TestManagerApplyDisabled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Apply should be a no-op when disabled
	err = manager.Apply()
	if err != nil {
		t.Errorf("unexpected error on disabled manager: %v", err)
	}
}

func TestManagerStatsDisabled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stats should return error when disabled
	_, err = manager.Stats()
	if err != ErrCgroupNotEnabled {
		t.Errorf("expected ErrCgroupNotEnabled, got: %v", err)
	}
}

func TestManagerCleanupDisabled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Cleanup should be a no-op when disabled
	err = manager.Cleanup()
	if err != nil {
		t.Errorf("unexpected error on cleanup: %v", err)
	}
}

func TestManagerConfig(t *testing.T) {
	config := DefaultConfig()
	config.CgroupName = "test-cgroup"

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	returnedConfig := manager.Config()
	if returnedConfig.CgroupName != "test-cgroup" {
		t.Errorf("expected CgroupName=test-cgroup, got %s", returnedConfig.CgroupName)
	}
}

func TestIOConfig(t *testing.T) {
	ioConfig := IOConfig{
		Enabled:   true,
		ReadBPS:   10 * 1024 * 1024,  // 10 MB/s
		WriteBPS:  5 * 1024 * 1024,   // 5 MB/s
		ReadIOPS:  1000,
		WriteIOPS: 500,
		Devices:   []string{"8:0"},
	}

	if !ioConfig.Enabled {
		t.Error("expected Enabled=true")
	}
	if ioConfig.ReadBPS != 10*1024*1024 {
		t.Errorf("expected ReadBPS=10MB/s, got %d", ioConfig.ReadBPS)
	}
	if ioConfig.WriteBPS != 5*1024*1024 {
		t.Errorf("expected WriteBPS=5MB/s, got %d", ioConfig.WriteBPS)
	}
	if ioConfig.ReadIOPS != 1000 {
		t.Errorf("expected ReadIOPS=1000, got %d", ioConfig.ReadIOPS)
	}
	if ioConfig.WriteIOPS != 500 {
		t.Errorf("expected WriteIOPS=500, got %d", ioConfig.WriteIOPS)
	}
	if len(ioConfig.Devices) != 1 || ioConfig.Devices[0] != "8:0" {
		t.Errorf("expected Devices=[8:0], got %v", ioConfig.Devices)
	}
}

func TestMemoryConfig(t *testing.T) {
	memConfig := MemoryConfig{
		Enabled:      true,
		MaxBytes:     512 * 1024 * 1024, // 512 MB
		HighBytes:    400 * 1024 * 1024, // 400 MB
		SwapMaxBytes: 0,
	}

	if !memConfig.Enabled {
		t.Error("expected Enabled=true")
	}
	if memConfig.MaxBytes != 512*1024*1024 {
		t.Errorf("expected MaxBytes=512MB, got %d", memConfig.MaxBytes)
	}
	if memConfig.HighBytes != 400*1024*1024 {
		t.Errorf("expected HighBytes=400MB, got %d", memConfig.HighBytes)
	}
}

func TestCPUConfig(t *testing.T) {
	cpuConfig := CPUConfig{
		Enabled:    true,
		MaxPercent: 50,
		Weight:     200,
	}

	if !cpuConfig.Enabled {
		t.Error("expected Enabled=true")
	}
	if cpuConfig.MaxPercent != 50 {
		t.Errorf("expected MaxPercent=50, got %d", cpuConfig.MaxPercent)
	}
	if cpuConfig.Weight != 200 {
		t.Errorf("expected Weight=200, got %d", cpuConfig.Weight)
	}
}

func TestIsCgroupV2Available(t *testing.T) {
	// This test just verifies the function doesn't panic
	available := IsCgroupV2Available("")
	t.Logf("cgroup v2 available: %v", available)

	// Test with custom root
	available = IsCgroupV2Available("/nonexistent/path")
	if available {
		t.Error("expected false for nonexistent path")
	}
}

func TestStatsStructures(t *testing.T) {
	// Test IOStats
	ioStats := IOStats{
		TotalReadBytes:  1024,
		TotalWriteBytes: 2048,
		TotalReadIOs:    10,
		TotalWriteIOs:   20,
		Devices: map[string]DeviceIOStats{
			"8:0": {
				ReadBytes:  1024,
				WriteBytes: 2048,
				ReadIOs:    10,
				WriteIOs:   20,
			},
		},
	}

	if ioStats.TotalReadBytes != 1024 {
		t.Errorf("expected TotalReadBytes=1024, got %d", ioStats.TotalReadBytes)
	}
	if len(ioStats.Devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(ioStats.Devices))
	}

	// Test MemoryStats
	memStats := MemoryStats{
		Current: 256 * 1024 * 1024,
		Max:     512 * 1024 * 1024,
		High:    400 * 1024 * 1024,
	}

	if memStats.Current != 256*1024*1024 {
		t.Errorf("expected Current=256MB, got %d", memStats.Current)
	}

	// Test CPUStats
	cpuStats := CPUStats{
		UsageUsec:     1000000,
		UserUsec:      800000,
		SystemUsec:    200000,
		NrPeriods:     100,
		NrThrottled:   5,
		ThrottledUsec: 50000,
	}

	if cpuStats.UsageUsec != 1000000 {
		t.Errorf("expected UsageUsec=1000000, got %d", cpuStats.UsageUsec)
	}
	if cpuStats.NrThrottled != 5 {
		t.Errorf("expected NrThrottled=5, got %d", cpuStats.NrThrottled)
	}

	// Test Stats aggregate
	stats := Stats{
		IO:     &ioStats,
		Memory: &memStats,
		CPU:    &cpuStats,
	}

	if stats.IO == nil || stats.Memory == nil || stats.CPU == nil {
		t.Error("expected all stats to be non-nil")
	}
}

func TestErrors(t *testing.T) {
	// Test error messages
	if ErrCgroupNotSupported.Error() != "cgroup v2 is not supported on this system" {
		t.Error("unexpected error message for ErrCgroupNotSupported")
	}
	if ErrCgroupNotEnabled.Error() != "cgroup management is not enabled" {
		t.Error("unexpected error message for ErrCgroupNotEnabled")
	}
	if ErrInvalidConfig.Error() != "invalid cgroup configuration" {
		t.Error("unexpected error message for ErrInvalidConfig")
	}
}
