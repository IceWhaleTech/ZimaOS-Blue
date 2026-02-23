package leakdetect

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Detector monitors for resource leaks.
type Detector struct {
	mu              sync.RWMutex
	enabled         bool
	checkInterval   time.Duration
	thresholds      Thresholds
	baseline        *Snapshot
	snapshots       []*Snapshot
	alerts          []Alert
	onAlert         func(Alert)
	stopCh          chan struct{}
	goroutineLeaks  int64
	fdLeaks         int64
	connectionLeaks int64
}

// Thresholds defines leak detection thresholds.
type Thresholds struct {
	MaxGoroutines        int           `json:"max_goroutines"`
	MaxOpenFiles         int           `json:"max_open_files"`
	MaxConnections       int           `json:"max_connections"`
	GoroutineGrowthRate  float64       `json:"goroutine_growth_rate"`  // per minute
	FDGrowthRate         float64       `json:"fd_growth_rate"`         // per minute
	ConnectionGrowthRate float64       `json:"connection_growth_rate"` // per minute
	AlertCooldown        time.Duration `json:"alert_cooldown"`
}

// Snapshot captures resource usage at a point in time.
type Snapshot struct {
	Timestamp   time.Time `json:"timestamp"`
	Goroutines  int       `json:"goroutines"`
	OpenFiles   int       `json:"open_files"`
	Connections int       `json:"connections"`
	HeapAlloc   uint64    `json:"heap_alloc"`
	HeapObjects uint64    `json:"heap_objects"`
	StackInUse  uint64    `json:"stack_in_use"`
	NumGC       uint32    `json:"num_gc"`
}

// Alert represents a leak detection alert.
type Alert struct {
	Type       AlertType `json:"type"`
	Severity   Severity  `json:"severity"`
	Message    string    `json:"message"`
	Current    int       `json:"current"`
	Threshold  int       `json:"threshold"`
	GrowthRate float64   `json:"growth_rate,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// AlertType represents the type of leak alert.
type AlertType string

const (
	AlertTypeGoroutine  AlertType = "goroutine_leak"
	AlertTypeFD         AlertType = "fd_leak"
	AlertTypeConnection AlertType = "connection_leak"
	AlertTypeMemory     AlertType = "memory_leak"
)

// Severity represents alert severity.
type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// DefaultThresholds returns default leak detection thresholds.
func DefaultThresholds() Thresholds {
	return Thresholds{
		MaxGoroutines:        10000,
		MaxOpenFiles:         1000,
		MaxConnections:       500,
		GoroutineGrowthRate:  100, // 100 new goroutines per minute
		FDGrowthRate:         50,  // 50 new FDs per minute
		ConnectionGrowthRate: 50,  // 50 new connections per minute
		AlertCooldown:        5 * time.Minute,
	}
}

// NewDetector creates a new leak detector.
func NewDetector(thresholds Thresholds, checkInterval time.Duration) *Detector {
	if checkInterval == 0 {
		checkInterval = 30 * time.Second
	}

	return &Detector{
		thresholds:    thresholds,
		checkInterval: checkInterval,
		snapshots:     make([]*Snapshot, 0, 100),
		alerts:        make([]Alert, 0),
		stopCh:        make(chan struct{}),
	}
}

// SetAlertHandler sets the alert callback function.
func (d *Detector) SetAlertHandler(handler func(Alert)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onAlert = handler
}

// Start starts the leak detector.
func (d *Detector) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.enabled {
		d.mu.Unlock()
		return fmt.Errorf("detector already running")
	}
	d.enabled = true
	d.stopCh = make(chan struct{})
	d.mu.Unlock()

	// Take baseline snapshot
	d.baseline = d.takeSnapshot()

	go d.run(ctx)
	return nil
}

// Stop stops the leak detector.
func (d *Detector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.enabled {
		return
	}

	close(d.stopCh)
	d.enabled = false
}

// IsRunning returns whether the detector is running.
func (d *Detector) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.enabled
}

// GetSnapshot returns the current resource snapshot.
func (d *Detector) GetSnapshot() *Snapshot {
	return d.takeSnapshot()
}

// GetBaseline returns the baseline snapshot.
func (d *Detector) GetBaseline() *Snapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.baseline
}

// GetAlerts returns all alerts.
func (d *Detector) GetAlerts() []Alert {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return append([]Alert{}, d.alerts...)
}

// GetLeakCounts returns the number of detected leaks.
func (d *Detector) GetLeakCounts() (goroutines, fds, connections int64) {
	return atomic.LoadInt64(&d.goroutineLeaks),
		atomic.LoadInt64(&d.fdLeaks),
		atomic.LoadInt64(&d.connectionLeaks)
}

// ClearAlerts clears all alerts.
func (d *Detector) ClearAlerts() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.alerts = d.alerts[:0]
}

func (d *Detector) run(ctx context.Context) {
	ticker := time.NewTicker(d.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.check()
		}
	}
}

func (d *Detector) check() {
	snapshot := d.takeSnapshot()

	d.mu.Lock()
	d.snapshots = append(d.snapshots, snapshot)
	// Keep only last 100 snapshots
	if len(d.snapshots) > 100 {
		d.snapshots = d.snapshots[1:]
	}
	d.mu.Unlock()

	// Check thresholds
	d.checkThresholds(snapshot)

	// Check growth rates
	d.checkGrowthRates()
}

func (d *Detector) takeSnapshot() *Snapshot {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return &Snapshot{
		Timestamp:   timeutil.NowTime(),
		Goroutines:  runtime.NumGoroutine(),
		OpenFiles:   getOpenFileCount(),
		Connections: getConnectionCount(),
		HeapAlloc:   memStats.HeapAlloc,
		HeapObjects: memStats.HeapObjects,
		StackInUse:  memStats.StackInuse,
		NumGC:       memStats.NumGC,
	}
}

func (d *Detector) checkThresholds(snapshot *Snapshot) {
	// Check goroutine threshold
	if snapshot.Goroutines > d.thresholds.MaxGoroutines {
		d.raiseAlert(Alert{
			Type:      AlertTypeGoroutine,
			Severity:  SeverityCritical,
			Message:   fmt.Sprintf("Goroutine count (%d) exceeds threshold (%d)", snapshot.Goroutines, d.thresholds.MaxGoroutines),
			Current:   snapshot.Goroutines,
			Threshold: d.thresholds.MaxGoroutines,
			Timestamp: timeutil.NowTime(),
		})
		atomic.AddInt64(&d.goroutineLeaks, 1)
	}

	// Check open files threshold
	if snapshot.OpenFiles > d.thresholds.MaxOpenFiles {
		d.raiseAlert(Alert{
			Type:      AlertTypeFD,
			Severity:  SeverityCritical,
			Message:   fmt.Sprintf("Open file count (%d) exceeds threshold (%d)", snapshot.OpenFiles, d.thresholds.MaxOpenFiles),
			Current:   snapshot.OpenFiles,
			Threshold: d.thresholds.MaxOpenFiles,
			Timestamp: timeutil.NowTime(),
		})
		atomic.AddInt64(&d.fdLeaks, 1)
	}

	// Check connection threshold
	if snapshot.Connections > d.thresholds.MaxConnections {
		d.raiseAlert(Alert{
			Type:      AlertTypeConnection,
			Severity:  SeverityCritical,
			Message:   fmt.Sprintf("Connection count (%d) exceeds threshold (%d)", snapshot.Connections, d.thresholds.MaxConnections),
			Current:   snapshot.Connections,
			Threshold: d.thresholds.MaxConnections,
			Timestamp: timeutil.NowTime(),
		})
		atomic.AddInt64(&d.connectionLeaks, 1)
	}
}

func (d *Detector) checkGrowthRates() {
	d.mu.RLock()
	if len(d.snapshots) < 2 {
		d.mu.RUnlock()
		return
	}

	// Get snapshots from last minute
	now := timeutil.NowTime()
	oneMinuteAgo := now.Add(-time.Minute)

	var oldSnapshot, newSnapshot *Snapshot
	for i := len(d.snapshots) - 1; i >= 0; i-- {
		s := d.snapshots[i]
		if newSnapshot == nil {
			newSnapshot = s
		}
		if s.Timestamp.Before(oneMinuteAgo) {
			oldSnapshot = s
			break
		}
	}
	d.mu.RUnlock()

	if oldSnapshot == nil || newSnapshot == nil {
		return
	}

	duration := newSnapshot.Timestamp.Sub(oldSnapshot.Timestamp).Minutes()
	if duration < 0.5 {
		return
	}

	// Check goroutine growth rate
	goroutineGrowth := float64(newSnapshot.Goroutines-oldSnapshot.Goroutines) / duration
	if goroutineGrowth > d.thresholds.GoroutineGrowthRate {
		d.raiseAlert(Alert{
			Type:       AlertTypeGoroutine,
			Severity:   SeverityWarning,
			Message:    fmt.Sprintf("Goroutine growth rate (%.1f/min) exceeds threshold (%.1f/min)", goroutineGrowth, d.thresholds.GoroutineGrowthRate),
			Current:    newSnapshot.Goroutines,
			GrowthRate: goroutineGrowth,
			Timestamp:  timeutil.NowTime(),
		})
	}

	// Check FD growth rate
	fdGrowth := float64(newSnapshot.OpenFiles-oldSnapshot.OpenFiles) / duration
	if fdGrowth > d.thresholds.FDGrowthRate {
		d.raiseAlert(Alert{
			Type:       AlertTypeFD,
			Severity:   SeverityWarning,
			Message:    fmt.Sprintf("File descriptor growth rate (%.1f/min) exceeds threshold (%.1f/min)", fdGrowth, d.thresholds.FDGrowthRate),
			Current:    newSnapshot.OpenFiles,
			GrowthRate: fdGrowth,
			Timestamp:  timeutil.NowTime(),
		})
	}

	// Check connection growth rate
	connGrowth := float64(newSnapshot.Connections-oldSnapshot.Connections) / duration
	if connGrowth > d.thresholds.ConnectionGrowthRate {
		d.raiseAlert(Alert{
			Type:       AlertTypeConnection,
			Severity:   SeverityWarning,
			Message:    fmt.Sprintf("Connection growth rate (%.1f/min) exceeds threshold (%.1f/min)", connGrowth, d.thresholds.ConnectionGrowthRate),
			Current:    newSnapshot.Connections,
			GrowthRate: connGrowth,
			Timestamp:  timeutil.NowTime(),
		})
	}
}

func (d *Detector) raiseAlert(alert Alert) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check cooldown
	for _, existing := range d.alerts {
		if existing.Type == alert.Type &&
			timeutil.SinceTime(existing.Timestamp) < d.thresholds.AlertCooldown {
			return // Still in cooldown
		}
	}

	d.alerts = append(d.alerts, alert)

	// Call alert handler
	if d.onAlert != nil {
		go d.onAlert(alert)
	}
}

// getOpenFileCount returns the number of open file descriptors.
func getOpenFileCount() int {
	// This is a simplified cross-platform implementation
	// On Linux, you would read /proc/self/fd
	// On Windows, this is more complex

	if runtime.GOOS == "linux" {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			return 0
		}
		return len(entries)
	}

	// Fallback: return 0 for unsupported platforms
	return 0
}

// getConnectionCount returns the number of active network connections.
func getConnectionCount() int {
	// This is a simplified implementation
	// In production, you would track connections in your application

	// Try to count TCP connections
	count := 0

	// Check common ports
	ports := []int{80, 443, 80, 3000, 5432, 3306, 6379}
	for _, port := range ports {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 10*time.Millisecond)
		if err == nil {
			conn.Close()
			count++
		}
	}

	return count
}

// ConnectionTracker tracks active connections.
type ConnectionTracker struct {
	mu          sync.RWMutex
	connections map[string]time.Time
	maxAge      time.Duration
}

// NewConnectionTracker creates a new connection tracker.
func NewConnectionTracker(maxAge time.Duration) *ConnectionTracker {
	if maxAge == 0 {
		maxAge = 5 * time.Minute
	}

	return &ConnectionTracker{
		connections: make(map[string]time.Time),
		maxAge:      maxAge,
	}
}

// Add adds a connection.
func (t *ConnectionTracker) Add(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connections[id] = timeutil.NowTime()
}

// Remove removes a connection.
func (t *ConnectionTracker) Remove(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.connections, id)
}

// Count returns the number of active connections.
func (t *ConnectionTracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.connections)
}

// GetStale returns connections older than maxAge.
func (t *ConnectionTracker) GetStale() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cutoff := timeutil.NowTime().Add(-t.maxAge)
	var stale []string

	for id, created := range t.connections {
		if created.Before(cutoff) {
			stale = append(stale, id)
		}
	}

	return stale
}

// Cleanup removes stale connections.
func (t *ConnectionTracker) Cleanup() int {
	stale := t.GetStale()

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, id := range stale {
		delete(t.connections, id)
	}

	return len(stale)
}
