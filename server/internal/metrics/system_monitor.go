package metrics

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

func safeNetIOCounters(pernic bool) (_ []net.IOCountersStat, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while collecting network IO counters: %v", r)
		}
	}()
	return net.IOCounters(pernic)
}

// SystemMonitor monitors system resources.
type SystemMonitor struct {
	mu sync.RWMutex

	// Current metrics
	current *SystemResourceMetrics

	// Ring buffer for history
	history     []ResourceHistory
	historyHead int // Next write position
	historyLen  int // Current number of items
	maxHistory  int

	// Process tracking
	processes map[int32]*ProcessMetrics

	// Configuration
	diskPath string
}

// NewSystemMonitor creates a new SystemMonitor.
func NewSystemMonitor(maxHistory int, diskPath string) *SystemMonitor {
	if diskPath == "" {
		if runtime.GOOS == "windows" {
			diskPath = "C:\\"
		} else {
			diskPath = "/"
		}
	}

	return &SystemMonitor{
		current:     &SystemResourceMetrics{},
		history:     make([]ResourceHistory, maxHistory), // Pre-allocate ring buffer
		historyHead: 0,
		historyLen:  0,
		maxHistory:  maxHistory,
		processes:   make(map[int32]*ProcessMetrics),
		diskPath:    diskPath,
	}
}

// Collect collects current system metrics.
func (m *SystemMonitor) Collect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics := &SystemResourceMetrics{}

	// CPU metrics
	if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
		metrics.CPUUsagePercent = cpuPercent[0]
	}
	metrics.CPUCount = runtime.NumCPU()

	// Load average (not available on Windows)
	if runtime.GOOS != "windows" {
		if loadAvg, err := cpu.Times(false); err == nil && len(loadAvg) > 0 {
			// Note: gopsutil doesn't directly provide load average
			// We'll use a placeholder for now
		}
	}

	// Memory metrics
	if memInfo, err := mem.VirtualMemory(); err == nil {
		metrics.MemoryTotal = int64(memInfo.Total)
		metrics.MemoryUsed = int64(memInfo.Used)
		metrics.MemoryFree = int64(memInfo.Free)
		metrics.MemoryPercent = memInfo.UsedPercent
	}

	// Disk metrics
	if diskInfo, err := disk.Usage(m.diskPath); err == nil {
		metrics.DiskTotal = int64(diskInfo.Total)
		metrics.DiskUsed = int64(diskInfo.Used)
		metrics.DiskFree = int64(diskInfo.Free)
		metrics.DiskPercent = diskInfo.UsedPercent
	}

	// Network metrics
	if netIO, err := safeNetIOCounters(false); err == nil && len(netIO) > 0 {
		metrics.NetworkBytesSent = int64(netIO[0].BytesSent)
		metrics.NetworkBytesRecv = int64(netIO[0].BytesRecv)
	}

	m.current = metrics

	// Add to ring buffer history
	m.history[m.historyHead] = ResourceHistory{
		Timestamp: timeutil.NowTime(),
		CPU:       metrics.CPUUsagePercent,
		Memory:    metrics.MemoryPercent,
		Disk:      metrics.DiskPercent,
	}
	m.historyHead = (m.historyHead + 1) % m.maxHistory
	if m.historyLen < m.maxHistory {
		m.historyLen++
	}

	return nil
}

// GetSystemMetrics returns current system metrics.
func (m *SystemMonitor) GetSystemMetrics() *SystemResourceMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &SystemResourceMetrics{
		CPUCount:         m.current.CPUCount,
		CPUUsagePercent:  m.current.CPUUsagePercent,
		LoadAvg1:         m.current.LoadAvg1,
		LoadAvg5:         m.current.LoadAvg5,
		LoadAvg15:        m.current.LoadAvg15,
		MemoryTotal:      m.current.MemoryTotal,
		MemoryUsed:       m.current.MemoryUsed,
		MemoryFree:       m.current.MemoryFree,
		MemoryPercent:    m.current.MemoryPercent,
		DiskTotal:        m.current.DiskTotal,
		DiskUsed:         m.current.DiskUsed,
		DiskFree:         m.current.DiskFree,
		DiskPercent:      m.current.DiskPercent,
		NetworkBytesSent: m.current.NetworkBytesSent,
		NetworkBytesRecv: m.current.NetworkBytesRecv,
	}
}

// GetResourceHistory returns resource history in chronological order.
func (m *SystemMonitor) GetResourceHistory() []ResourceHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.historyLen == 0 {
		return nil
	}

	result := make([]ResourceHistory, m.historyLen)

	// Calculate start position (oldest entry)
	start := 0
	if m.historyLen == m.maxHistory {
		start = m.historyHead // When full, head points to oldest
	}

	// Copy in chronological order
	for i := 0; i < m.historyLen; i++ {
		idx := (start + i) % m.maxHistory
		result[i] = m.history[idx]
	}

	return result
}

// CollectProcessMetrics collects metrics for a specific process.
func (m *SystemMonitor) CollectProcessMetrics(pid int32) (*ProcessMetrics, error) {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return nil, err
	}

	metrics := &ProcessMetrics{
		PID: int(pid),
	}

	// Command
	if name, err := proc.Name(); err == nil {
		metrics.Command = name
	}

	// Status
	if status, err := proc.Status(); err == nil && len(status) > 0 {
		metrics.State = status[0]
	}

	// CPU
	if cpuPercent, err := proc.CPUPercent(); err == nil {
		metrics.CPUPercent = cpuPercent
	}

	if cpuTimes, err := proc.Times(); err == nil {
		metrics.CPUTime = cpuTimes.User + cpuTimes.System
	}

	// Memory
	if memInfo, err := proc.MemoryInfo(); err == nil {
		metrics.MemoryRSS = int64(memInfo.RSS)
		metrics.MemoryVMS = int64(memInfo.VMS)
	}

	if memPercent, err := proc.MemoryPercent(); err == nil {
		metrics.MemoryPercent = float64(memPercent)
	}

	// I/O
	if ioCounters, err := proc.IOCounters(); err == nil {
		metrics.IOReadBytes = int64(ioCounters.ReadBytes)
		metrics.IOWriteBytes = int64(ioCounters.WriteBytes)
	}

	// Threads
	if numThreads, err := proc.NumThreads(); err == nil {
		metrics.NumThreads = int(numThreads)
	}

	// Start time and uptime
	if createTime, err := proc.CreateTime(); err == nil {
		metrics.StartTime = time.UnixMilli(createTime)
		metrics.Uptime = timeutil.SinceTime(metrics.StartTime)
	}

	m.mu.Lock()
	m.processes[pid] = metrics
	m.mu.Unlock()

	return metrics, nil
}

// GetProcessMetrics returns metrics for a specific process.
func (m *SystemMonitor) GetProcessMetrics(pid int32) *ProcessMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if metrics, ok := m.processes[pid]; ok {
		return metrics
	}
	return nil
}

// GetCurrentProcessMetrics returns metrics for the current process.
func (m *SystemMonitor) GetCurrentProcessMetrics() (*ProcessMetrics, error) {
	return m.CollectProcessMetrics(int32(os.Getpid()))
}

// GetAllProcessMetrics returns metrics for all tracked processes.
func (m *SystemMonitor) GetAllProcessMetrics() []ProcessMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ProcessMetrics, 0, len(m.processes))
	for _, metrics := range m.processes {
		result = append(result, *metrics)
	}
	return result
}

// Reset clears all collected metrics.
func (m *SystemMonitor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.current = &SystemResourceMetrics{}
	// Reset ring buffer without reallocating
	m.historyHead = 0
	m.historyLen = 0
	m.processes = make(map[int32]*ProcessMetrics)
}
