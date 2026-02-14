package plugin

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
)

// ResourceLimits defines resource limits for a plugin
type ResourceLimits struct {
	// MaxMemoryMB is the maximum memory usage in megabytes (0 = unlimited)
	MaxMemoryMB int64 `json:"max_memory_mb"`
	// MaxCPUPercent is the maximum CPU usage percentage (0 = unlimited)
	MaxCPUPercent int `json:"max_cpu_percent"`
	// MaxGoroutines is the maximum number of goroutines (0 = unlimited)
	MaxGoroutines int `json:"max_goroutines"`
	// MaxExecutionTime is the maximum execution time for a single operation
	MaxExecutionTime time.Duration `json:"max_execution_time"`
	// MaxConcurrentOps is the maximum number of concurrent operations
	MaxConcurrentOps int `json:"max_concurrent_ops"`
}

// DefaultResourceLimits returns default resource limits
func DefaultResourceLimits() *ResourceLimits {
	return &ResourceLimits{
		MaxMemoryMB:      64,
		MaxCPUPercent:    50,
		MaxGoroutines:    100,
		MaxExecutionTime: 60 * time.Second,
		MaxConcurrentOps: 10,
	}
}

// ResourceMonitor monitors resource usage for plugins
type ResourceMonitor struct {
	pluginID string
	limits   *ResourceLimits

	// Current usage tracking
	currentOps     int64
	totalOps       int64
	failedOps      int64
	goroutineCount int64

	// Violation tracking
	violations     int64
	lastViolation  time.Time
	violationMu    sync.RWMutex

	// Scheduler for controlled execution
	scheduler *PluginScheduler
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(pluginID string, limits *ResourceLimits) *ResourceMonitor {
	if limits == nil {
		limits = DefaultResourceLimits()
	}
	return &ResourceMonitor{
		pluginID:  pluginID,
		limits:    limits,
		scheduler: NewPluginScheduler(pluginID, limits.MaxConcurrentOps),
	}
}

// CheckLimits checks if the plugin is within resource limits
func (m *ResourceMonitor) CheckLimits() error {
	// Check concurrent operations
	if m.limits.MaxConcurrentOps > 0 {
		current := atomic.LoadInt64(&m.currentOps)
		if int(current) >= m.limits.MaxConcurrentOps {
			m.recordViolation("concurrent operations limit exceeded")
			return fmt.Errorf("plugin %s: concurrent operations limit exceeded (%d/%d)",
				m.pluginID, current, m.limits.MaxConcurrentOps)
		}
	}

	return nil
}

// AcquireOperation acquires a slot for an operation
func (m *ResourceMonitor) AcquireOperation(ctx context.Context) error {
	if err := m.CheckLimits(); err != nil {
		return err
	}

	// Use scheduler for controlled execution
	if err := m.scheduler.Acquire(ctx); err != nil {
		return err
	}

	atomic.AddInt64(&m.currentOps, 1)
	atomic.AddInt64(&m.totalOps, 1)
	return nil
}

// ReleaseOperation releases an operation slot
func (m *ResourceMonitor) ReleaseOperation() {
	atomic.AddInt64(&m.currentOps, -1)
	m.scheduler.Release()
}

// RecordFailure records a failed operation
func (m *ResourceMonitor) RecordFailure() {
	atomic.AddInt64(&m.failedOps, 1)
}

// GetStats returns current resource usage statistics
func (m *ResourceMonitor) GetStats() *ResourceStats {
	return &ResourceStats{
		PluginID:       m.pluginID,
		CurrentOps:     atomic.LoadInt64(&m.currentOps),
		TotalOps:       atomic.LoadInt64(&m.totalOps),
		FailedOps:      atomic.LoadInt64(&m.failedOps),
		GoroutineCount: atomic.LoadInt64(&m.goroutineCount),
		Violations:     atomic.LoadInt64(&m.violations),
	}
}

func (m *ResourceMonitor) recordViolation(reason string) {
	atomic.AddInt64(&m.violations, 1)
	m.violationMu.Lock()
	m.lastViolation = time.Now()
	m.violationMu.Unlock()

	logger.Warn().
		Str("plugin_id", m.pluginID).
		Str("reason", reason).
		Msg("Plugin resource violation")
}

// ResourceStats contains resource usage statistics
type ResourceStats struct {
	PluginID       string `json:"plugin_id"`
	CurrentOps     int64  `json:"current_ops"`
	TotalOps       int64  `json:"total_ops"`
	FailedOps      int64  `json:"failed_ops"`
	GoroutineCount int64  `json:"goroutine_count"`
	Violations     int64  `json:"violations"`
}

// PluginScheduler controls plugin execution
type PluginScheduler struct {
	pluginID   string
	maxConcurrent int
	semaphore  chan struct{}
	mu         sync.Mutex
}

// NewPluginScheduler creates a new plugin scheduler
func NewPluginScheduler(pluginID string, maxConcurrent int) *PluginScheduler {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	return &PluginScheduler{
		pluginID:      pluginID,
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}
}

// Acquire acquires a slot in the scheduler
func (s *PluginScheduler) Acquire(ctx context.Context) error {
	select {
	case s.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("plugin %s: scheduler acquisition cancelled: %w", s.pluginID, ctx.Err())
	}
}

// Release releases a slot in the scheduler
func (s *PluginScheduler) Release() {
	select {
	case <-s.semaphore:
	default:
		// Semaphore was already empty, this shouldn't happen
		logger.Warn().Str("plugin_id", s.pluginID).Msg("Scheduler release called without acquire")
	}
}

// Schedule schedules a function for execution
func (s *PluginScheduler) Schedule(ctx context.Context, fn func(context.Context) error) error {
	if err := s.Acquire(ctx); err != nil {
		return err
	}
	defer s.Release()

	return fn(ctx)
}

// ScheduleWithResult schedules a function for execution and returns a result
func ScheduleWithResult[T any](s *PluginScheduler, ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	if err := s.Acquire(ctx); err != nil {
		var zero T
		return zero, err
	}
	defer s.Release()

	return fn(ctx)
}

// RestrictedPluginAPI wraps PluginAPI with resource restrictions
type RestrictedPluginAPI struct {
	inner   PluginAPI
	monitor *ResourceMonitor
	limits  *ResourceLimits
}

// NewRestrictedPluginAPI creates a new restricted plugin API
func NewRestrictedPluginAPI(inner PluginAPI, limits *ResourceLimits) *RestrictedPluginAPI {
	if limits == nil {
		limits = DefaultResourceLimits()
	}
	return &RestrictedPluginAPI{
		inner:   inner,
		monitor: NewResourceMonitor(inner.PluginID(), limits),
		limits:  limits,
	}
}

func (a *RestrictedPluginAPI) PluginID() string {
	return a.inner.PluginID()
}

func (a *RestrictedPluginAPI) PluginConfig() map[string]interface{} {
	return a.inner.PluginConfig()
}

func (a *RestrictedPluginAPI) RegisterTool(tool Tool) error {
	// Wrap the tool handler with resource monitoring
	if tool.Handler != nil {
		originalHandler := tool.Handler
		tool.Handler = func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			if err := a.monitor.AcquireOperation(ctx); err != nil {
				return nil, err
			}
			defer a.monitor.ReleaseOperation()

			result, err := originalHandler(ctx, params)
			if err != nil {
				a.monitor.RecordFailure()
			}
			return result, err
		}
	}
	return a.inner.RegisterTool(tool)
}

func (a *RestrictedPluginAPI) RegisterHook(event string, handler HookHandler) error {
	// Wrap the hook handler with resource monitoring
	wrappedHandler := func(ctx context.Context, data interface{}) error {
		if err := a.monitor.AcquireOperation(ctx); err != nil {
			return err
		}
		defer a.monitor.ReleaseOperation()

		err := handler(ctx, data)
		if err != nil {
			a.monitor.RecordFailure()
		}
		return err
	}
	return a.inner.RegisterHook(event, wrappedHandler)
}

func (a *RestrictedPluginAPI) RegisterService(service Service) error {
	// Wrap the service with resource monitoring
	wrappedService := &restrictedService{
		service: service,
		monitor: a.monitor,
	}
	return a.inner.RegisterService(wrappedService)
}

func (a *RestrictedPluginAPI) RegisterCommand(command Command) error {
	// Wrap the command handler with resource monitoring
	if command.Handler != nil {
		originalHandler := command.Handler
		command.Handler = func(ctx context.Context, args []string) (string, error) {
			if err := a.monitor.AcquireOperation(ctx); err != nil {
				return "", err
			}
			defer a.monitor.ReleaseOperation()

			result, err := originalHandler(ctx, args)
			if err != nil {
				a.monitor.RecordFailure()
			}
			return result, err
		}
	}
	return a.inner.RegisterCommand(command)
}

func (a *RestrictedPluginAPI) RegisterHTTPHandler(path string, handler HTTPHandler) error {
	// Wrap the HTTP handler with resource monitoring
	wrappedHandler := func(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error) {
		if err := a.monitor.AcquireOperation(ctx); err != nil {
			return nil, err
		}
		defer a.monitor.ReleaseOperation()

		result, err := handler(ctx, req)
		if err != nil {
			a.monitor.RecordFailure()
		}
		return result, err
	}
	return a.inner.RegisterHTTPHandler(path, wrappedHandler)
}

func (a *RestrictedPluginAPI) Logger() Logger {
	return a.inner.Logger()
}

// GetResourceStats returns the resource usage statistics
func (a *RestrictedPluginAPI) GetResourceStats() *ResourceStats {
	return a.monitor.GetStats()
}

// restrictedService wraps a service with resource monitoring
type restrictedService struct {
	service Service
	monitor *ResourceMonitor
}

func (s *restrictedService) Name() string {
	return s.service.Name()
}

func (s *restrictedService) Start(ctx context.Context) error {
	if err := s.monitor.AcquireOperation(ctx); err != nil {
		return err
	}
	defer s.monitor.ReleaseOperation()

	return s.service.Start(ctx)
}

func (s *restrictedService) Stop(ctx context.Context) error {
	if err := s.monitor.AcquireOperation(ctx); err != nil {
		return err
	}
	defer s.monitor.ReleaseOperation()

	return s.service.Stop(ctx)
}

// PluginSandbox provides a sandboxed execution environment for plugins
type PluginSandbox struct {
	pluginID string
	limits   *ResourceLimits
	monitor  *ResourceMonitor

	// Goroutine tracking
	goroutines     map[int64]struct{}
	goroutinesMu   sync.Mutex
	nextGoroutineID int64
}

// NewPluginSandbox creates a new plugin sandbox
func NewPluginSandbox(pluginID string, limits *ResourceLimits) *PluginSandbox {
	if limits == nil {
		limits = DefaultResourceLimits()
	}
	return &PluginSandbox{
		pluginID:   pluginID,
		limits:     limits,
		monitor:    NewResourceMonitor(pluginID, limits),
		goroutines: make(map[int64]struct{}),
	}
}

// Go spawns a controlled goroutine within the sandbox
func (s *PluginSandbox) Go(fn func()) error {
	s.goroutinesMu.Lock()

	// Check goroutine limit
	if s.limits.MaxGoroutines > 0 && len(s.goroutines) >= s.limits.MaxGoroutines {
		s.goroutinesMu.Unlock()
		return fmt.Errorf("plugin %s: goroutine limit exceeded (%d/%d)",
			s.pluginID, len(s.goroutines), s.limits.MaxGoroutines)
	}

	id := s.nextGoroutineID
	s.nextGoroutineID++
	s.goroutines[id] = struct{}{}
	s.goroutinesMu.Unlock()

	go func() {
		defer func() {
			s.goroutinesMu.Lock()
			delete(s.goroutines, id)
			s.goroutinesMu.Unlock()

			if r := recover(); r != nil {
				logger.Error().
					Str("plugin_id", s.pluginID).
					Interface("panic", r).
					Msg("Plugin goroutine panicked")
			}
		}()

		fn()
	}()

	return nil
}

// GoWithContext spawns a controlled goroutine with context
func (s *PluginSandbox) GoWithContext(ctx context.Context, fn func(context.Context)) error {
	return s.Go(func() {
		fn(ctx)
	})
}

// GoroutineCount returns the current number of goroutines
func (s *PluginSandbox) GoroutineCount() int {
	s.goroutinesMu.Lock()
	defer s.goroutinesMu.Unlock()
	return len(s.goroutines)
}

// GetStats returns sandbox statistics
func (s *PluginSandbox) GetStats() *SandboxStats {
	s.goroutinesMu.Lock()
	goroutineCount := len(s.goroutines)
	s.goroutinesMu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return &SandboxStats{
		PluginID:       s.pluginID,
		GoroutineCount: goroutineCount,
		ResourceStats:  s.monitor.GetStats(),
	}
}

// SandboxStats contains sandbox statistics
type SandboxStats struct {
	PluginID       string         `json:"plugin_id"`
	GoroutineCount int            `json:"goroutine_count"`
	ResourceStats  *ResourceStats `json:"resource_stats"`
}

// PluginRestrictions defines what a plugin is allowed to do
type PluginRestrictions struct {
	// AllowNetworkAccess allows the plugin to make network requests
	AllowNetworkAccess bool `json:"allow_network_access"`
	// AllowFileAccess allows the plugin to access the filesystem
	AllowFileAccess bool `json:"allow_file_access"`
	// AllowedPaths restricts file access to specific paths
	AllowedPaths []string `json:"allowed_paths,omitempty"`
	// AllowedHosts restricts network access to specific hosts
	AllowedHosts []string `json:"allowed_hosts,omitempty"`
	// AllowExec allows the plugin to execute external commands
	AllowExec bool `json:"allow_exec"`
	// AllowedCommands restricts exec to specific commands
	AllowedCommands []string `json:"allowed_commands,omitempty"`
}

// DefaultPluginRestrictions returns default plugin restrictions
func DefaultPluginRestrictions() *PluginRestrictions {
	return &PluginRestrictions{
		AllowNetworkAccess: false,
		AllowFileAccess:    false,
		AllowExec:          false,
	}
}

// CheckNetworkAccess checks if network access to a host is allowed
func (r *PluginRestrictions) CheckNetworkAccess(host string) error {
	if !r.AllowNetworkAccess {
		return fmt.Errorf("network access not allowed")
	}

	if len(r.AllowedHosts) > 0 {
		for _, allowed := range r.AllowedHosts {
			if allowed == "*" || allowed == host {
				return nil
			}
		}
		return fmt.Errorf("network access to host %s not allowed", host)
	}

	return nil
}

// CheckFileAccess checks if file access to a path is allowed
func (r *PluginRestrictions) CheckFileAccess(path string) error {
	if !r.AllowFileAccess {
		return fmt.Errorf("file access not allowed")
	}

	if len(r.AllowedPaths) > 0 {
		for _, allowed := range r.AllowedPaths {
			if allowed == "*" || pathMatches(path, allowed) {
				return nil
			}
		}
		return fmt.Errorf("file access to path %s not allowed", path)
	}

	return nil
}

// CheckExec checks if executing a command is allowed
func (r *PluginRestrictions) CheckExec(command string) error {
	if !r.AllowExec {
		return fmt.Errorf("command execution not allowed")
	}

	if len(r.AllowedCommands) > 0 {
		for _, allowed := range r.AllowedCommands {
			if allowed == "*" || allowed == command {
				return nil
			}
		}
		return fmt.Errorf("command %s not allowed", command)
	}

	return nil
}

// pathMatches checks if a path matches a pattern (simple prefix matching)
func pathMatches(path, pattern string) bool {
	if len(pattern) == 0 {
		return false
	}
	if pattern[len(pattern)-1] == '/' {
		// Directory pattern - check prefix
		return len(path) >= len(pattern) && path[:len(pattern)] == pattern
	}
	return path == pattern
}
