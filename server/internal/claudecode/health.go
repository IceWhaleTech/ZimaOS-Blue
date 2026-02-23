package claudecode

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// CheckResult represents the result of a single health check.
type CheckResult struct {
	// Name is the name of the check.
	Name string `json:"name"`
	// Healthy indicates whether the check passed.
	Healthy bool `json:"healthy"`
	// Message contains additional information about the check.
	Message string `json:"message,omitempty"`
	// Duration is how long the check took.
	Duration time.Duration `json:"duration_ms"`
	// LastCheck is when the check was last performed.
	LastCheck time.Time `json:"last_check"`
}

// HealthStatus represents the overall health status.
type HealthStatus struct {
	// Healthy indicates whether all checks passed.
	Healthy bool `json:"healthy"`
	// Message contains a summary message.
	Message string `json:"message"`
	// Checks contains individual check results.
	Checks map[string]CheckResult `json:"checks"`
	// LastCheck is when the health check was last performed.
	LastCheck time.Time `json:"last_check"`
}

// HealthChecker performs health checks on the CLI.
type HealthChecker struct {
	config        HealthCheckConfig
	binaryManager *BinaryManager
	status        *HealthStatus
	stopCh        chan struct{}
	mu            sync.RWMutex
	running       bool
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(config HealthCheckConfig, binaryManager *BinaryManager) *HealthChecker {
	return &HealthChecker{
		config:        config,
		binaryManager: binaryManager,
		status: &HealthStatus{
			Healthy: true,
			Checks:  make(map[string]CheckResult),
		},
		stopCh: make(chan struct{}),
	}
}

// Start begins periodic health checks.
func (h *HealthChecker) Start() {
	if !h.config.Enabled {
		return
	}

	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}
	h.running = true
	h.mu.Unlock()

	// Perform initial check
	h.Check(context.Background())

	// Start periodic checks
	go h.runPeriodic()
}

// Stop stops periodic health checks.
func (h *HealthChecker) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	close(h.stopCh)
	h.running = false
}

// runPeriodic runs health checks periodically.
func (h *HealthChecker) runPeriodic() {
	ticker := time.NewTicker(h.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), h.config.Timeout)
			h.Check(ctx)
			cancel()
		case <-h.stopCh:
			return
		}
	}
}

// Check performs all health checks.
func (h *HealthChecker) Check(ctx context.Context) HealthStatus {
	checks := make(map[string]CheckResult)

	// Binary check
	checks["binary"] = h.checkBinary(ctx)

	// Version check
	checks["version"] = h.checkVersion(ctx)

	// Determine overall health
	healthy := true
	var messages []string
	for name, check := range checks {
		if !check.Healthy {
			healthy = false
			messages = append(messages, name+": "+check.Message)
		}
	}

	message := "All checks passed"
	if !healthy {
		message = strings.Join(messages, "; ")
	}

	status := HealthStatus{
		Healthy:   healthy,
		Message:   message,
		Checks:    checks,
		LastCheck: timeutil.NowTime(),
	}

	// Update cached status
	h.mu.Lock()
	h.status = &status
	h.mu.Unlock()

	return status
}

// checkBinary checks if the CLI binary is available.
func (h *HealthChecker) checkBinary(ctx context.Context) CheckResult {
	start := timeutil.NowTime()
	result := CheckResult{
		Name:      "binary",
		LastCheck: start,
	}

	// Try to get binary path
	binaryPath, err := h.binaryManager.GetBinaryPath()
	if err != nil {
		result.Healthy = false
		result.Message = "Binary not found: " + err.Error()
		result.Duration = timeutil.SinceTime(start)
		return result
	}

	// Check if binary is executable
	cmd := exec.CommandContext(ctx, binaryPath, "--version")
	if err := cmd.Run(); err != nil {
		result.Healthy = false
		result.Message = "Binary not executable: " + err.Error()
		result.Duration = timeutil.SinceTime(start)
		return result
	}

	result.Healthy = true
	result.Message = "Binary available at " + binaryPath
	result.Duration = timeutil.SinceTime(start)
	return result
}

// checkVersion checks if the CLI version is compatible.
func (h *HealthChecker) checkVersion(ctx context.Context) CheckResult {
	start := timeutil.NowTime()
	result := CheckResult{
		Name:      "version",
		LastCheck: start,
	}

	// Get binary path
	binaryPath, err := h.binaryManager.GetBinaryPath()
	if err != nil {
		result.Healthy = false
		result.Message = "Cannot check version: binary not found"
		result.Duration = timeutil.SinceTime(start)
		return result
	}

	// Get version
	cmd := exec.CommandContext(ctx, binaryPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		result.Healthy = false
		result.Message = "Failed to get version: " + err.Error()
		result.Duration = timeutil.SinceTime(start)
		return result
	}

	version := strings.TrimSpace(string(output))
	result.Healthy = true
	result.Message = "Version: " + version
	result.Duration = timeutil.SinceTime(start)
	return result
}

// Status returns the current health status.
func (h *HealthChecker) Status() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.status == nil {
		return HealthStatus{
			Healthy: true,
			Message: "No health check performed yet",
			Checks:  make(map[string]CheckResult),
		}
	}

	return *h.status
}

// IsHealthy returns true if the CLI is healthy.
func (h *HealthChecker) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.status == nil {
		return true
	}

	return h.status.Healthy
}
