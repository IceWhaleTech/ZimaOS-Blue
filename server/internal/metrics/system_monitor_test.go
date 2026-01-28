package metrics

import (
	"os"
	"runtime"
	"testing"
)

func TestNewSystemMonitor(t *testing.T) {
	monitor := NewSystemMonitor(100, "")

	if monitor == nil {
		t.Fatal("Expected monitor to be created")
	}

	if monitor.maxHistory != 100 {
		t.Errorf("Expected maxHistory 100, got %d", monitor.maxHistory)
	}

	// Check default disk path
	expectedPath := "/"
	if runtime.GOOS == "windows" {
		expectedPath = "C:\\"
	}
	if monitor.diskPath != expectedPath {
		t.Errorf("Expected diskPath '%s', got '%s'", expectedPath, monitor.diskPath)
	}
}

func TestSystemMonitor_Collect(t *testing.T) {
	monitor := NewSystemMonitor(100, "")

	err := monitor.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	metrics := monitor.GetSystemMetrics()

	// CPU count should be > 0
	if metrics.CPUCount <= 0 {
		t.Errorf("Expected CPUCount > 0, got %d", metrics.CPUCount)
	}

	// Memory total should be > 0
	if metrics.MemoryTotal <= 0 {
		t.Errorf("Expected MemoryTotal > 0, got %d", metrics.MemoryTotal)
	}

	// Memory percent should be between 0 and 100
	if metrics.MemoryPercent < 0 || metrics.MemoryPercent > 100 {
		t.Errorf("Expected MemoryPercent between 0-100, got %.2f", metrics.MemoryPercent)
	}

	// Disk total should be > 0
	if metrics.DiskTotal <= 0 {
		t.Errorf("Expected DiskTotal > 0, got %d", metrics.DiskTotal)
	}
}

func TestSystemMonitor_GetResourceHistory(t *testing.T) {
	monitor := NewSystemMonitor(10, "")

	// Collect multiple times
	for i := 0; i < 5; i++ {
		err := monitor.Collect()
		if err != nil {
			t.Fatalf("Collect failed: %v", err)
		}
	}

	history := monitor.GetResourceHistory()
	if len(history) != 5 {
		t.Errorf("Expected 5 history entries, got %d", len(history))
	}
}

func TestSystemMonitor_HistoryTrimming(t *testing.T) {
	monitor := NewSystemMonitor(3, "")

	// Collect more than maxHistory times
	for i := 0; i < 5; i++ {
		err := monitor.Collect()
		if err != nil {
			t.Fatalf("Collect failed: %v", err)
		}
	}

	history := monitor.GetResourceHistory()
	if len(history) != 3 {
		t.Errorf("Expected 3 history entries (trimmed), got %d", len(history))
	}
}

func TestSystemMonitor_GetCurrentProcessMetrics(t *testing.T) {
	monitor := NewSystemMonitor(100, "")

	metrics, err := monitor.GetCurrentProcessMetrics()
	if err != nil {
		t.Fatalf("GetCurrentProcessMetrics failed: %v", err)
	}

	// PID should match current process
	if metrics.PID != os.Getpid() {
		t.Errorf("Expected PID %d, got %d", os.Getpid(), metrics.PID)
	}

	// Command should not be empty
	if metrics.Command == "" {
		t.Error("Expected Command to be set")
	}

	// Memory should be > 0
	if metrics.MemoryRSS <= 0 {
		t.Errorf("Expected MemoryRSS > 0, got %d", metrics.MemoryRSS)
	}
}

func TestSystemMonitor_CollectProcessMetrics(t *testing.T) {
	monitor := NewSystemMonitor(100, "")

	pid := int32(os.Getpid())
	metrics, err := monitor.CollectProcessMetrics(pid)
	if err != nil {
		t.Fatalf("CollectProcessMetrics failed: %v", err)
	}

	if metrics.PID != int(pid) {
		t.Errorf("Expected PID %d, got %d", pid, metrics.PID)
	}

	// Check that metrics are stored
	storedMetrics := monitor.GetProcessMetrics(pid)
	if storedMetrics == nil {
		t.Error("Expected stored metrics, got nil")
	}
}

func TestSystemMonitor_GetAllProcessMetrics(t *testing.T) {
	monitor := NewSystemMonitor(100, "")

	// Collect current process metrics
	_, err := monitor.GetCurrentProcessMetrics()
	if err != nil {
		t.Fatalf("GetCurrentProcessMetrics failed: %v", err)
	}

	allMetrics := monitor.GetAllProcessMetrics()
	if len(allMetrics) != 1 {
		t.Errorf("Expected 1 process, got %d", len(allMetrics))
	}
}

func TestSystemMonitor_Reset(t *testing.T) {
	monitor := NewSystemMonitor(100, "")

	// Collect some data
	monitor.Collect()
	monitor.GetCurrentProcessMetrics()

	history := monitor.GetResourceHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 history entry before reset, got %d", len(history))
	}

	monitor.Reset()

	history = monitor.GetResourceHistory()
	if len(history) != 0 {
		t.Errorf("Expected 0 history entries after reset, got %d", len(history))
	}

	allMetrics := monitor.GetAllProcessMetrics()
	if len(allMetrics) != 0 {
		t.Errorf("Expected 0 processes after reset, got %d", len(allMetrics))
	}
}

func TestSystemMonitor_NonExistentProcess(t *testing.T) {
	monitor := NewSystemMonitor(100, "")

	// Try to get metrics for a non-existent process
	metrics := monitor.GetProcessMetrics(999999)
	if metrics != nil {
		t.Error("Expected nil for non-existent process")
	}
}
