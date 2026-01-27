package plugin

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestResourceMonitor_CheckLimits(t *testing.T) {
	limits := &ResourceLimits{
		MaxConcurrentOps: 2,
	}
	monitor := NewResourceMonitor("test-plugin", limits)

	// First two operations should succeed
	ctx := context.Background()
	if err := monitor.AcquireOperation(ctx); err != nil {
		t.Errorf("first acquire should succeed: %v", err)
	}
	if err := monitor.AcquireOperation(ctx); err != nil {
		t.Errorf("second acquire should succeed: %v", err)
	}

	// Third should fail
	if err := monitor.AcquireOperation(ctx); err == nil {
		t.Error("third acquire should fail due to limit")
	}

	// Release one and try again
	monitor.ReleaseOperation()
	if err := monitor.AcquireOperation(ctx); err != nil {
		t.Errorf("acquire after release should succeed: %v", err)
	}
}

func TestResourceMonitor_GetStats(t *testing.T) {
	monitor := NewResourceMonitor("test-plugin", nil)
	ctx := context.Background()

	monitor.AcquireOperation(ctx)
	monitor.AcquireOperation(ctx)
	monitor.RecordFailure()
	monitor.ReleaseOperation()

	stats := monitor.GetStats()

	if stats.PluginID != "test-plugin" {
		t.Errorf("expected plugin ID 'test-plugin', got %s", stats.PluginID)
	}
	if stats.CurrentOps != 1 {
		t.Errorf("expected 1 current op, got %d", stats.CurrentOps)
	}
	if stats.TotalOps != 2 {
		t.Errorf("expected 2 total ops, got %d", stats.TotalOps)
	}
	if stats.FailedOps != 1 {
		t.Errorf("expected 1 failed op, got %d", stats.FailedOps)
	}
}

func TestPluginScheduler_Acquire(t *testing.T) {
	scheduler := NewPluginScheduler("test-plugin", 2)
	ctx := context.Background()

	// First two should succeed immediately
	if err := scheduler.Acquire(ctx); err != nil {
		t.Errorf("first acquire should succeed: %v", err)
	}
	if err := scheduler.Acquire(ctx); err != nil {
		t.Errorf("second acquire should succeed: %v", err)
	}

	// Third should block, test with timeout
	ctx2, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := scheduler.Acquire(ctx2); err == nil {
		t.Error("third acquire should fail due to timeout")
	}

	// Release and try again
	scheduler.Release()
	if err := scheduler.Acquire(context.Background()); err != nil {
		t.Errorf("acquire after release should succeed: %v", err)
	}
}

func TestPluginScheduler_Schedule(t *testing.T) {
	scheduler := NewPluginScheduler("test-plugin", 2)
	ctx := context.Background()

	executed := false
	err := scheduler.Schedule(ctx, func(ctx context.Context) error {
		executed = true
		return nil
	})

	if err != nil {
		t.Errorf("schedule should succeed: %v", err)
	}
	if !executed {
		t.Error("function should have been executed")
	}
}

func TestPluginScheduler_ConcurrentSchedule(t *testing.T) {
	scheduler := NewPluginScheduler("test-plugin", 2)
	ctx := context.Background()

	var running int32
	var maxRunning int32
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scheduler.Schedule(ctx, func(ctx context.Context) error {
				current := atomic.AddInt32(&running, 1)
				for {
					old := atomic.LoadInt32(&maxRunning)
					if current <= old || atomic.CompareAndSwapInt32(&maxRunning, old, current) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				atomic.AddInt32(&running, -1)
				return nil
			})
		}()
	}

	wg.Wait()

	if maxRunning > 2 {
		t.Errorf("max concurrent should be 2, got %d", maxRunning)
	}
}

func TestPluginSandbox_Go(t *testing.T) {
	sandbox := NewPluginSandbox("test-plugin", &ResourceLimits{
		MaxGoroutines: 2,
	})

	done := make(chan struct{}, 3)

	// First two should succeed
	if err := sandbox.Go(func() {
		time.Sleep(100 * time.Millisecond)
		done <- struct{}{}
	}); err != nil {
		t.Errorf("first goroutine should succeed: %v", err)
	}

	if err := sandbox.Go(func() {
		time.Sleep(100 * time.Millisecond)
		done <- struct{}{}
	}); err != nil {
		t.Errorf("second goroutine should succeed: %v", err)
	}

	// Third should fail
	if err := sandbox.Go(func() {
		done <- struct{}{}
	}); err == nil {
		t.Error("third goroutine should fail due to limit")
	}

	// Wait for goroutines to complete
	<-done
	<-done

	// Now should be able to spawn again
	time.Sleep(10 * time.Millisecond) // Give time for cleanup
	if err := sandbox.Go(func() {
		done <- struct{}{}
	}); err != nil {
		t.Errorf("goroutine after completion should succeed: %v", err)
	}
	<-done
}

func TestPluginSandbox_GoroutineCount(t *testing.T) {
	sandbox := NewPluginSandbox("test-plugin", nil)

	if sandbox.GoroutineCount() != 0 {
		t.Errorf("initial count should be 0, got %d", sandbox.GoroutineCount())
	}

	done := make(chan struct{})
	sandbox.Go(func() {
		<-done
	})

	time.Sleep(10 * time.Millisecond)
	if sandbox.GoroutineCount() != 1 {
		t.Errorf("count should be 1, got %d", sandbox.GoroutineCount())
	}

	close(done)
	time.Sleep(10 * time.Millisecond)
	if sandbox.GoroutineCount() != 0 {
		t.Errorf("count should be 0 after completion, got %d", sandbox.GoroutineCount())
	}
}

func TestPluginSandbox_PanicRecovery(t *testing.T) {
	sandbox := NewPluginSandbox("test-plugin", nil)

	done := make(chan struct{})
	sandbox.Go(func() {
		defer close(done)
		panic("test panic")
	})

	// Should not crash, wait for completion
	select {
	case <-done:
		// Success - panic was recovered
	case <-time.After(time.Second):
		t.Error("goroutine should have completed")
	}
}

func TestPluginRestrictions_CheckNetworkAccess(t *testing.T) {
	tests := []struct {
		name        string
		restrictions *PluginRestrictions
		host        string
		shouldAllow bool
	}{
		{
			name:        "network disabled",
			restrictions: &PluginRestrictions{AllowNetworkAccess: false},
			host:        "example.com",
			shouldAllow: false,
		},
		{
			name:        "network enabled no restrictions",
			restrictions: &PluginRestrictions{AllowNetworkAccess: true},
			host:        "example.com",
			shouldAllow: true,
		},
		{
			name: "allowed host",
			restrictions: &PluginRestrictions{
				AllowNetworkAccess: true,
				AllowedHosts:       []string{"example.com"},
			},
			host:        "example.com",
			shouldAllow: true,
		},
		{
			name: "disallowed host",
			restrictions: &PluginRestrictions{
				AllowNetworkAccess: true,
				AllowedHosts:       []string{"example.com"},
			},
			host:        "other.com",
			shouldAllow: false,
		},
		{
			name: "wildcard host",
			restrictions: &PluginRestrictions{
				AllowNetworkAccess: true,
				AllowedHosts:       []string{"*"},
			},
			host:        "any.com",
			shouldAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.restrictions.CheckNetworkAccess(tt.host)
			if tt.shouldAllow && err != nil {
				t.Errorf("expected access to be allowed, got error: %v", err)
			}
			if !tt.shouldAllow && err == nil {
				t.Error("expected access to be denied")
			}
		})
	}
}

func TestPluginRestrictions_CheckFileAccess(t *testing.T) {
	tests := []struct {
		name        string
		restrictions *PluginRestrictions
		path        string
		shouldAllow bool
	}{
		{
			name:        "file access disabled",
			restrictions: &PluginRestrictions{AllowFileAccess: false},
			path:        "/tmp/test",
			shouldAllow: false,
		},
		{
			name:        "file access enabled no restrictions",
			restrictions: &PluginRestrictions{AllowFileAccess: true},
			path:        "/tmp/test",
			shouldAllow: true,
		},
		{
			name: "allowed path exact",
			restrictions: &PluginRestrictions{
				AllowFileAccess: true,
				AllowedPaths:    []string{"/tmp/test"},
			},
			path:        "/tmp/test",
			shouldAllow: true,
		},
		{
			name: "allowed path directory",
			restrictions: &PluginRestrictions{
				AllowFileAccess: true,
				AllowedPaths:    []string{"/tmp/"},
			},
			path:        "/tmp/test",
			shouldAllow: true,
		},
		{
			name: "disallowed path",
			restrictions: &PluginRestrictions{
				AllowFileAccess: true,
				AllowedPaths:    []string{"/tmp/"},
			},
			path:        "/etc/passwd",
			shouldAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.restrictions.CheckFileAccess(tt.path)
			if tt.shouldAllow && err != nil {
				t.Errorf("expected access to be allowed, got error: %v", err)
			}
			if !tt.shouldAllow && err == nil {
				t.Error("expected access to be denied")
			}
		})
	}
}

func TestPluginRestrictions_CheckExec(t *testing.T) {
	tests := []struct {
		name        string
		restrictions *PluginRestrictions
		command     string
		shouldAllow bool
	}{
		{
			name:        "exec disabled",
			restrictions: &PluginRestrictions{AllowExec: false},
			command:     "ls",
			shouldAllow: false,
		},
		{
			name:        "exec enabled no restrictions",
			restrictions: &PluginRestrictions{AllowExec: true},
			command:     "ls",
			shouldAllow: true,
		},
		{
			name: "allowed command",
			restrictions: &PluginRestrictions{
				AllowExec:       true,
				AllowedCommands: []string{"ls", "cat"},
			},
			command:     "ls",
			shouldAllow: true,
		},
		{
			name: "disallowed command",
			restrictions: &PluginRestrictions{
				AllowExec:       true,
				AllowedCommands: []string{"ls", "cat"},
			},
			command:     "rm",
			shouldAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.restrictions.CheckExec(tt.command)
			if tt.shouldAllow && err != nil {
				t.Errorf("expected exec to be allowed, got error: %v", err)
			}
			if !tt.shouldAllow && err == nil {
				t.Error("expected exec to be denied")
			}
		})
	}
}

func TestDefaultResourceLimits(t *testing.T) {
	limits := DefaultResourceLimits()

	if limits.MaxMemoryMB != 64 {
		t.Errorf("expected MaxMemoryMB 64, got %d", limits.MaxMemoryMB)
	}
	if limits.MaxCPUPercent != 50 {
		t.Errorf("expected MaxCPUPercent 50, got %d", limits.MaxCPUPercent)
	}
	if limits.MaxGoroutines != 100 {
		t.Errorf("expected MaxGoroutines 100, got %d", limits.MaxGoroutines)
	}
	if limits.MaxExecutionTime != 60*time.Second {
		t.Errorf("expected MaxExecutionTime 60s, got %v", limits.MaxExecutionTime)
	}
	if limits.MaxConcurrentOps != 10 {
		t.Errorf("expected MaxConcurrentOps 10, got %d", limits.MaxConcurrentOps)
	}
}

func TestDefaultPluginRestrictions(t *testing.T) {
	restrictions := DefaultPluginRestrictions()

	if restrictions.AllowNetworkAccess {
		t.Error("expected AllowNetworkAccess to be false")
	}
	if restrictions.AllowFileAccess {
		t.Error("expected AllowFileAccess to be false")
	}
	if restrictions.AllowExec {
		t.Error("expected AllowExec to be false")
	}
}

func TestRestrictedPluginAPI(t *testing.T) {
	// Create a mock inner API
	inner := &mockPluginAPI{
		pluginID: "test-plugin",
		config:   map[string]interface{}{"key": "value"},
	}

	limits := &ResourceLimits{
		MaxConcurrentOps: 2,
	}

	restricted := NewRestrictedPluginAPI(inner, limits)

	if restricted.PluginID() != "test-plugin" {
		t.Errorf("expected plugin ID 'test-plugin', got %s", restricted.PluginID())
	}

	config := restricted.PluginConfig()
	if config["key"] != "value" {
		t.Error("expected config to be passed through")
	}
}

// mockPluginAPI is a mock implementation of PluginAPI for testing
type mockPluginAPI struct {
	pluginID string
	config   map[string]interface{}
	tools    []Tool
	hooks    map[string][]HookHandler
	services []Service
	commands []Command
	handlers map[string]HTTPHandler
}

func (m *mockPluginAPI) PluginID() string {
	return m.pluginID
}

func (m *mockPluginAPI) PluginConfig() map[string]interface{} {
	return m.config
}

func (m *mockPluginAPI) RegisterTool(tool Tool) error {
	m.tools = append(m.tools, tool)
	return nil
}

func (m *mockPluginAPI) RegisterHook(event string, handler HookHandler) error {
	if m.hooks == nil {
		m.hooks = make(map[string][]HookHandler)
	}
	m.hooks[event] = append(m.hooks[event], handler)
	return nil
}

func (m *mockPluginAPI) RegisterService(service Service) error {
	m.services = append(m.services, service)
	return nil
}

func (m *mockPluginAPI) RegisterCommand(command Command) error {
	m.commands = append(m.commands, command)
	return nil
}

func (m *mockPluginAPI) RegisterHTTPHandler(path string, handler HTTPHandler) error {
	if m.handlers == nil {
		m.handlers = make(map[string]HTTPHandler)
	}
	m.handlers[path] = handler
	return nil
}

func (m *mockPluginAPI) Logger() Logger {
	return &pluginLogger{pluginID: m.pluginID}
}
