// Package cgroup provides cgroup v2 resource management for Linux systems.
package cgroup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	// DefaultCgroupRoot is the default cgroup v2 mount point.
	DefaultCgroupRoot = "/sys/fs/cgroup"

	// DefaultCgroupName is the default cgroup name for this application.
	DefaultCgroupName = "zimaos-blue"
)

var (
	// ErrCgroupNotSupported indicates cgroup v2 is not available.
	ErrCgroupNotSupported = errors.New("cgroup v2 is not supported on this system")

	// ErrCgroupNotEnabled indicates cgroup management is disabled.
	ErrCgroupNotEnabled = errors.New("cgroup management is not enabled")

	// ErrInvalidConfig indicates invalid configuration.
	ErrInvalidConfig = errors.New("invalid cgroup configuration")
)

// Config holds cgroup configuration.
type Config struct {
	// Enabled indicates whether cgroup management is enabled.
	Enabled bool `yaml:"enabled" yaml:"enabled"`

	// CgroupRoot is the cgroup v2 mount point.
	CgroupRoot string `yaml:"cgroup_root" yaml:"cgroup_root"`

	// CgroupName is the name of the cgroup to use.
	CgroupName string `yaml:"cgroup_name" yaml:"cgroup_name"`

	// IO contains IO bandwidth limit configuration.
	IO IOConfig `yaml:"io" yaml:"io"`

	// Memory contains memory limit configuration (optional, can use systemd).
	Memory MemoryConfig `yaml:"memory" yaml:"memory"`

	// CPU contains CPU limit configuration (optional, can use systemd).
	CPU CPUConfig `yaml:"cpu" yaml:"cpu"`
}

// IOConfig holds IO bandwidth limit configuration.
type IOConfig struct {
	// Enabled indicates whether IO limits are enabled.
	Enabled bool `yaml:"enabled" yaml:"enabled"`

	// ReadBPS is the maximum read bytes per second (0 = unlimited).
	ReadBPS uint64 `yaml:"read_bps" yaml:"read_bps"`

	// WriteBPS is the maximum write bytes per second (0 = unlimited).
	WriteBPS uint64 `yaml:"write_bps" yaml:"write_bps"`

	// ReadIOPS is the maximum read IO operations per second (0 = unlimited).
	ReadIOPS uint64 `yaml:"read_iops" yaml:"read_iops"`

	// WriteIOPS is the maximum write IO operations per second (0 = unlimited).
	WriteIOPS uint64 `yaml:"write_iops" yaml:"write_iops"`

	// Devices is a list of device major:minor numbers to apply limits to.
	// If empty, limits are applied to all devices.
	Devices []string `yaml:"devices" yaml:"devices"`
}

// MemoryConfig holds memory limit configuration.
type MemoryConfig struct {
	// Enabled indicates whether memory limits via cgroup are enabled.
	Enabled bool `yaml:"enabled" yaml:"enabled"`

	// MaxBytes is the maximum memory in bytes (0 = unlimited).
	MaxBytes uint64 `yaml:"max_bytes" yaml:"max_bytes"`

	// HighBytes is the memory high threshold in bytes (0 = disabled).
	HighBytes uint64 `yaml:"high_bytes" yaml:"high_bytes"`

	// SwapMaxBytes is the maximum swap in bytes (0 = unlimited).
	SwapMaxBytes uint64 `yaml:"swap_max_bytes" yaml:"swap_max_bytes"`
}

// CPUConfig holds CPU limit configuration.
type CPUConfig struct {
	// Enabled indicates whether CPU limits via cgroup are enabled.
	Enabled bool `yaml:"enabled" yaml:"enabled"`

	// MaxPercent is the maximum CPU percentage (0 = unlimited).
	// This is converted to cpu.max format: quota period.
	MaxPercent int `yaml:"max_percent" yaml:"max_percent"`

	// Weight is the CPU weight (1-10000, default 100).
	Weight int `yaml:"weight" yaml:"weight"`
}

// DefaultConfig returns the default cgroup configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:    false,
		CgroupRoot: DefaultCgroupRoot,
		CgroupName: DefaultCgroupName,
		IO: IOConfig{
			Enabled: false,
		},
		Memory: MemoryConfig{
			Enabled: false,
		},
		CPU: CPUConfig{
			Enabled: false,
			Weight:  100,
		},
	}
}

// Manager manages cgroup v2 resources.
type Manager struct {
	config    Config
	cgroupDir string
	mu        sync.RWMutex
	applied   bool
}

// NewManager creates a new cgroup manager.
func NewManager(config Config) (*Manager, error) {
	if !config.Enabled {
		return &Manager{config: config}, nil
	}

	// Validate cgroup root
	cgroupRoot := config.CgroupRoot
	if cgroupRoot == "" {
		cgroupRoot = DefaultCgroupRoot
	}

	// Check if cgroup v2 is available
	if !IsCgroupV2Available(cgroupRoot) {
		return nil, ErrCgroupNotSupported
	}

	cgroupName := config.CgroupName
	if cgroupName == "" {
		cgroupName = DefaultCgroupName
	}

	return &Manager{
		config:    config,
		cgroupDir: filepath.Join(cgroupRoot, cgroupName),
	}, nil
}

// Apply applies the cgroup configuration.
func (m *Manager) Apply() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil
	}

	// Create cgroup directory if it doesn't exist
	// Security: Use 0700 to restrict access to cgroup controls
	if err := os.MkdirAll(m.cgroupDir, 0700); err != nil {
		return fmt.Errorf("failed to create cgroup directory: %w", err)
	}

	// Apply IO limits
	if m.config.IO.Enabled {
		if err := m.applyIOLimits(); err != nil {
			return fmt.Errorf("failed to apply IO limits: %w", err)
		}
	}

	// Apply memory limits
	if m.config.Memory.Enabled {
		if err := m.applyMemoryLimits(); err != nil {
			return fmt.Errorf("failed to apply memory limits: %w", err)
		}
	}

	// Apply CPU limits
	if m.config.CPU.Enabled {
		if err := m.applyCPULimits(); err != nil {
			return fmt.Errorf("failed to apply CPU limits: %w", err)
		}
	}

	// Add current process to cgroup
	if err := m.addCurrentProcess(); err != nil {
		return fmt.Errorf("failed to add process to cgroup: %w", err)
	}

	m.applied = true
	return nil
}

// applyIOLimits applies IO bandwidth limits.
func (m *Manager) applyIOLimits() error {
	ioConfig := m.config.IO

	// Get devices to apply limits to
	devices := ioConfig.Devices
	if len(devices) == 0 {
		// Try to detect root device
		rootDev, err := getRootDevice()
		if err != nil {
			return fmt.Errorf("failed to detect root device: %w", err)
		}
		if rootDev != "" {
			devices = []string{rootDev}
		}
	}

	if len(devices) == 0 {
		return nil // No devices to limit
	}

	// Build io.max content
	var lines []string
	for _, dev := range devices {
		var parts []string

		if ioConfig.ReadBPS > 0 {
			parts = append(parts, fmt.Sprintf("rbps=%d", ioConfig.ReadBPS))
		}
		if ioConfig.WriteBPS > 0 {
			parts = append(parts, fmt.Sprintf("wbps=%d", ioConfig.WriteBPS))
		}
		if ioConfig.ReadIOPS > 0 {
			parts = append(parts, fmt.Sprintf("riops=%d", ioConfig.ReadIOPS))
		}
		if ioConfig.WriteIOPS > 0 {
			parts = append(parts, fmt.Sprintf("wiops=%d", ioConfig.WriteIOPS))
		}

		if len(parts) > 0 {
			line := fmt.Sprintf("%s %s", dev, strings.Join(parts, " "))
			lines = append(lines, line)
		}
	}

	if len(lines) > 0 {
		content := strings.Join(lines, "\n")
		ioMaxPath := filepath.Join(m.cgroupDir, "io.max")
		if err := os.WriteFile(ioMaxPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write io.max: %w", err)
		}
	}

	return nil
}

// applyMemoryLimits applies memory limits.
func (m *Manager) applyMemoryLimits() error {
	memConfig := m.config.Memory

	// Set memory.max
	if memConfig.MaxBytes > 0 {
		memMaxPath := filepath.Join(m.cgroupDir, "memory.max")
		content := strconv.FormatUint(memConfig.MaxBytes, 10)
		if err := os.WriteFile(memMaxPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write memory.max: %w", err)
		}
	}

	// Set memory.high
	if memConfig.HighBytes > 0 {
		memHighPath := filepath.Join(m.cgroupDir, "memory.high")
		content := strconv.FormatUint(memConfig.HighBytes, 10)
		if err := os.WriteFile(memHighPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write memory.high: %w", err)
		}
	}

	// Set memory.swap.max
	if memConfig.SwapMaxBytes > 0 {
		swapMaxPath := filepath.Join(m.cgroupDir, "memory.swap.max")
		content := strconv.FormatUint(memConfig.SwapMaxBytes, 10)
		if err := os.WriteFile(swapMaxPath, []byte(content), 0644); err != nil {
			// Swap might not be enabled, log but don't fail
			return nil
		}
	}

	return nil
}

// applyCPULimits applies CPU limits.
func (m *Manager) applyCPULimits() error {
	cpuConfig := m.config.CPU

	// Set cpu.weight
	if cpuConfig.Weight > 0 {
		cpuWeightPath := filepath.Join(m.cgroupDir, "cpu.weight")
		content := strconv.Itoa(cpuConfig.Weight)
		if err := os.WriteFile(cpuWeightPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write cpu.weight: %w", err)
		}
	}

	// Set cpu.max (quota period format)
	if cpuConfig.MaxPercent > 0 && cpuConfig.MaxPercent < 100 {
		// cpu.max format: $MAX $PERIOD (in microseconds)
		// Default period is 100000 (100ms)
		period := 100000
		quota := (period * cpuConfig.MaxPercent) / 100

		cpuMaxPath := filepath.Join(m.cgroupDir, "cpu.max")
		content := fmt.Sprintf("%d %d", quota, period)
		if err := os.WriteFile(cpuMaxPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write cpu.max: %w", err)
		}
	}

	return nil
}

// addCurrentProcess adds the current process to the cgroup.
func (m *Manager) addCurrentProcess() error {
	procsPath := filepath.Join(m.cgroupDir, "cgroup.procs")
	pid := os.Getpid()
	content := strconv.Itoa(pid)

	if err := os.WriteFile(procsPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to add process %d to cgroup: %w", pid, err)
	}

	return nil
}

// Stats returns current cgroup statistics.
func (m *Manager) Stats() (*Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.Enabled || !m.applied {
		return nil, ErrCgroupNotEnabled
	}

	stats := &Stats{}

	// Read IO stats
	ioStats, err := m.readIOStats()
	if err == nil {
		stats.IO = ioStats
	}

	// Read memory stats
	memStats, err := m.readMemoryStats()
	if err == nil {
		stats.Memory = memStats
	}

	// Read CPU stats
	cpuStats, err := m.readCPUStats()
	if err == nil {
		stats.CPU = cpuStats
	}

	return stats, nil
}

// readIOStats reads IO statistics from cgroup.
func (m *Manager) readIOStats() (*IOStats, error) {
	ioStatPath := filepath.Join(m.cgroupDir, "io.stat")
	data, err := os.ReadFile(ioStatPath)
	if err != nil {
		return nil, err
	}

	stats := &IOStats{
		Devices: make(map[string]DeviceIOStats),
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		devStats := DeviceIOStats{}
		device := parts[0]

		for _, part := range parts[1:] {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}

			value, _ := strconv.ParseUint(kv[1], 10, 64)
			switch kv[0] {
			case "rbytes":
				devStats.ReadBytes = value
				stats.TotalReadBytes += value
			case "wbytes":
				devStats.WriteBytes = value
				stats.TotalWriteBytes += value
			case "rios":
				devStats.ReadIOs = value
				stats.TotalReadIOs += value
			case "wios":
				devStats.WriteIOs = value
				stats.TotalWriteIOs += value
			}
		}

		stats.Devices[device] = devStats
	}

	return stats, nil
}

// readMemoryStats reads memory statistics from cgroup.
func (m *Manager) readMemoryStats() (*MemoryStats, error) {
	stats := &MemoryStats{}

	// Read memory.current
	currentPath := filepath.Join(m.cgroupDir, "memory.current")
	if data, err := os.ReadFile(currentPath); err == nil {
		stats.Current, _ = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	}

	// Read memory.max
	maxPath := filepath.Join(m.cgroupDir, "memory.max")
	if data, err := os.ReadFile(maxPath); err == nil {
		content := strings.TrimSpace(string(data))
		if content != "max" {
			stats.Max, _ = strconv.ParseUint(content, 10, 64)
		}
	}

	// Read memory.high
	highPath := filepath.Join(m.cgroupDir, "memory.high")
	if data, err := os.ReadFile(highPath); err == nil {
		content := strings.TrimSpace(string(data))
		if content != "max" {
			stats.High, _ = strconv.ParseUint(content, 10, 64)
		}
	}

	return stats, nil
}

// readCPUStats reads CPU statistics from cgroup.
func (m *Manager) readCPUStats() (*CPUStats, error) {
	stats := &CPUStats{}

	// Read cpu.stat
	statPath := filepath.Join(m.cgroupDir, "cpu.stat")
	if data, err := os.ReadFile(statPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) != 2 {
				continue
			}

			value, _ := strconv.ParseUint(parts[1], 10, 64)
			switch parts[0] {
			case "usage_usec":
				stats.UsageUsec = value
			case "user_usec":
				stats.UserUsec = value
			case "system_usec":
				stats.SystemUsec = value
			case "nr_periods":
				stats.NrPeriods = value
			case "nr_throttled":
				stats.NrThrottled = value
			case "throttled_usec":
				stats.ThrottledUsec = value
			}
		}
	}

	return stats, nil
}

// Cleanup removes the cgroup.
func (m *Manager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled || !m.applied {
		return nil
	}

	// Note: Cannot remove cgroup while processes are in it.
	// The cgroup will be cleaned up when all processes exit.
	m.applied = false
	return nil
}

// IsEnabled returns whether cgroup management is enabled.
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// Config returns the current configuration.
func (m *Manager) Config() Config {
	return m.config
}

// Stats holds cgroup statistics.
type Stats struct {
	IO     *IOStats     `json:"io,omitempty"`
	Memory *MemoryStats `json:"memory,omitempty"`
	CPU    *CPUStats    `json:"cpu,omitempty"`
}

// IOStats holds IO statistics.
type IOStats struct {
	TotalReadBytes  uint64                   `json:"total_read_bytes"`
	TotalWriteBytes uint64                   `json:"total_write_bytes"`
	TotalReadIOs    uint64                   `json:"total_read_ios"`
	TotalWriteIOs   uint64                   `json:"total_write_ios"`
	Devices         map[string]DeviceIOStats `json:"devices,omitempty"`
}

// DeviceIOStats holds per-device IO statistics.
type DeviceIOStats struct {
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
	ReadIOs    uint64 `json:"read_ios"`
	WriteIOs   uint64 `json:"write_ios"`
}

// MemoryStats holds memory statistics.
type MemoryStats struct {
	Current uint64 `json:"current"`
	Max     uint64 `json:"max,omitempty"`
	High    uint64 `json:"high,omitempty"`
}

// CPUStats holds CPU statistics.
type CPUStats struct {
	UsageUsec    uint64 `json:"usage_usec"`
	UserUsec     uint64 `json:"user_usec"`
	SystemUsec   uint64 `json:"system_usec"`
	NrPeriods    uint64 `json:"nr_periods"`
	NrThrottled  uint64 `json:"nr_throttled"`
	ThrottledUsec uint64 `json:"throttled_usec"`
}
