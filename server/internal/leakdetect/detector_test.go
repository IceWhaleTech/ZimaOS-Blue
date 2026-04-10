package leakdetect

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

func setConnectionCreatedAt(tracker *ConnectionTracker, id string, created time.Time) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	tracker.connections[id] = created
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !condition() {
		t.Fatal(message)
	}
}

func TestNewDetector(t *testing.T) {
	thresholds := DefaultThresholds()
	d := NewDetector(thresholds, 100*time.Millisecond)

	if d == nil {
		t.Fatal("Expected detector, got nil")
	}

	if d.thresholds.MaxGoroutines != thresholds.MaxGoroutines {
		t.Errorf("Expected max goroutines %d, got %d", thresholds.MaxGoroutines, d.thresholds.MaxGoroutines)
	}
}

func TestDetector_StartStop(t *testing.T) {
	thresholds := DefaultThresholds()
	d := NewDetector(thresholds, 100*time.Millisecond)

	ctx := context.Background()

	// Start
	err := d.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !d.IsRunning() {
		t.Error("Expected detector to be running")
	}

	// Should fail if already running
	err = d.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting already running detector")
	}

	// Stop
	d.Stop()

	if d.IsRunning() {
		t.Error("Expected detector to be stopped")
	}
}

func TestDetector_GetSnapshot(t *testing.T) {
	thresholds := DefaultThresholds()
	d := NewDetector(thresholds, 100*time.Millisecond)

	snapshot := d.GetSnapshot()

	if snapshot == nil {
		t.Fatal("Expected snapshot, got nil")
	}

	if snapshot.Goroutines <= 0 {
		t.Error("Expected positive goroutine count")
	}

	if snapshot.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestDetector_GetBaseline(t *testing.T) {
	thresholds := DefaultThresholds()
	d := NewDetector(thresholds, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d.Start(ctx)
	defer d.Stop()

	baseline := d.GetBaseline()
	if baseline == nil {
		t.Fatal("Expected baseline snapshot")
	}
}

func TestDetector_AlertHandler(t *testing.T) {
	thresholds := Thresholds{
		MaxGoroutines:  1, // Very low threshold to trigger alert
		MaxOpenFiles:   1000,
		MaxConnections: 500,
		AlertCooldown:  0, // No cooldown for testing
	}

	d := NewDetector(thresholds, 50*time.Millisecond)

	alertReceived := make(chan Alert, 1)
	d.SetAlertHandler(func(a Alert) {
		select {
		case alertReceived <- a:
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d.Start(ctx)
	defer d.Stop()

	// Wait for alert
	select {
	case alert := <-alertReceived:
		if alert.Type != AlertTypeGoroutine {
			t.Errorf("Expected goroutine alert, got %s", alert.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Expected alert to be raised")
	}
}

func TestDetector_GetAlerts(t *testing.T) {
	thresholds := Thresholds{
		MaxGoroutines:  1,
		MaxOpenFiles:   1000,
		MaxConnections: 500,
		AlertCooldown:  0,
	}

	d := NewDetector(thresholds, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d.Start(ctx)
	waitForCondition(t, 500*time.Millisecond, func() bool {
		return len(d.GetAlerts()) > 0
	}, "Expected alerts to be recorded")
	d.Stop()

	alerts := d.GetAlerts()
	if len(alerts) == 0 {
		t.Error("Expected alerts to be recorded")
	}
}

func TestDetector_ClearAlerts(t *testing.T) {
	thresholds := Thresholds{
		MaxGoroutines:  1,
		MaxOpenFiles:   1000,
		MaxConnections: 500,
		AlertCooldown:  0,
	}

	d := NewDetector(thresholds, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d.Start(ctx)
	waitForCondition(t, 500*time.Millisecond, func() bool {
		return len(d.GetAlerts()) > 0
	}, "Expected alerts before clearing")
	d.Stop()

	d.ClearAlerts()

	alerts := d.GetAlerts()
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts after clear, got %d", len(alerts))
	}
}

func TestDetector_GetLeakCounts(t *testing.T) {
	thresholds := Thresholds{
		MaxGoroutines:  1,
		MaxOpenFiles:   1000,
		MaxConnections: 500,
		AlertCooldown:  0,
	}

	d := NewDetector(thresholds, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d.Start(ctx)
	waitForCondition(t, 500*time.Millisecond, func() bool {
		goroutines, _, _ := d.GetLeakCounts()
		return goroutines > 0
	}, "Expected goroutine leak count > 0")
	d.Stop()

	goroutines, fds, connections := d.GetLeakCounts()
	if goroutines == 0 {
		t.Error("Expected goroutine leak count > 0")
	}
	_ = fds
	_ = connections
}

func TestDefaultThresholds(t *testing.T) {
	thresholds := DefaultThresholds()

	if thresholds.MaxGoroutines != 10000 {
		t.Errorf("Expected max goroutines 10000, got %d", thresholds.MaxGoroutines)
	}

	if thresholds.MaxOpenFiles != 1000 {
		t.Errorf("Expected max open files 1000, got %d", thresholds.MaxOpenFiles)
	}

	if thresholds.AlertCooldown != 5*time.Minute {
		t.Errorf("Expected alert cooldown 5m, got %v", thresholds.AlertCooldown)
	}
}

func TestConnectionTracker_AddRemove(t *testing.T) {
	tracker := NewConnectionTracker(time.Minute)

	tracker.Add("conn1")
	tracker.Add("conn2")

	if tracker.Count() != 2 {
		t.Errorf("Expected 2 connections, got %d", tracker.Count())
	}

	tracker.Remove("conn1")

	if tracker.Count() != 1 {
		t.Errorf("Expected 1 connection, got %d", tracker.Count())
	}
}

func TestConnectionTracker_GetStale(t *testing.T) {
	tracker := NewConnectionTracker(50 * time.Millisecond)
	now := timeutil.NowTime()

	setConnectionCreatedAt(tracker, "conn1", now.Add(-2*tracker.maxAge))
	setConnectionCreatedAt(tracker, "conn2", now)

	stale := tracker.GetStale()
	if len(stale) != 1 {
		t.Errorf("Expected 1 stale connection, got %d", len(stale))
	}

	if stale[0] != "conn1" {
		t.Errorf("Expected conn1 to be stale, got %s", stale[0])
	}
}

func TestConnectionTracker_Cleanup(t *testing.T) {
	tracker := NewConnectionTracker(50 * time.Millisecond)
	now := timeutil.NowTime()

	setConnectionCreatedAt(tracker, "conn1", now.Add(-3*tracker.maxAge))
	setConnectionCreatedAt(tracker, "conn2", now.Add(-2*tracker.maxAge))
	setConnectionCreatedAt(tracker, "conn3", now)

	cleaned := tracker.Cleanup()
	if cleaned != 2 {
		t.Errorf("Expected 2 connections cleaned, got %d", cleaned)
	}

	if tracker.Count() != 1 {
		t.Errorf("Expected 1 connection remaining, got %d", tracker.Count())
	}
}

func TestConnectionTracker_Concurrent(t *testing.T) {
	tracker := NewConnectionTracker(time.Minute)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			connID := string(rune('a' + id%26))
			tracker.Add(connID)
			time.Sleep(10 * time.Millisecond)
			tracker.Remove(connID)
		}(i)
	}

	wg.Wait()

	// All connections should be removed
	if tracker.Count() != 0 {
		t.Errorf("Expected 0 connections after concurrent test, got %d", tracker.Count())
	}
}

func TestDetector_ContextCancellation(t *testing.T) {
	thresholds := DefaultThresholds()
	d := NewDetector(thresholds, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	d.Start(ctx)

	// Cancel context
	cancel()

	// Give some time for goroutine to exit
	time.Sleep(200 * time.Millisecond)

	// Detector should still report as running (Stop() not called)
	// but internal goroutine should have exited
}

func TestDetector_StopWithoutStart(t *testing.T) {
	thresholds := DefaultThresholds()
	d := NewDetector(thresholds, 100*time.Millisecond)

	// Should not panic
	d.Stop()
}

func TestSnapshot_Fields(t *testing.T) {
	thresholds := DefaultThresholds()
	d := NewDetector(thresholds, 100*time.Millisecond)

	snapshot := d.GetSnapshot()

	// Verify all fields are populated
	if snapshot.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	if snapshot.Goroutines <= 0 {
		t.Error("Goroutines should be positive")
	}

	// HeapAlloc should be non-zero in any running Go program
	if snapshot.HeapAlloc == 0 {
		t.Error("HeapAlloc should be non-zero")
	}
}
