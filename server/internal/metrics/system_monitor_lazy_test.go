package metrics

import "testing"

func TestNewSystemMonitor_LazilyAllocatesHistoryAndProcesses(t *testing.T) {
	monitor := NewSystemMonitor(100, "")
	if monitor == nil {
		t.Fatal("expected monitor to be created")
	}
	if monitor.history != nil {
		t.Fatal("expected system monitor history buffer to start nil")
	}
	if monitor.processes != nil {
		t.Fatal("expected process metrics map to start nil")
	}

	if err := monitor.Collect(); err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if len(monitor.history) != monitor.maxHistory {
		t.Fatalf("expected history buffer len %d after first collect, got %d", monitor.maxHistory, len(monitor.history))
	}
	if monitor.historyLen != 1 {
		t.Fatalf("expected historyLen 1 after first collect, got %d", monitor.historyLen)
	}

	if _, err := monitor.GetCurrentProcessMetrics(); err != nil {
		t.Fatalf("GetCurrentProcessMetrics failed: %v", err)
	}
	if monitor.processes == nil {
		t.Fatal("expected process metrics map to initialize on first process collection")
	}
}
