// Package sysinfo provides comprehensive system information collection
package sysinfo

import (
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Info contains comprehensive system information
type Info struct {
	OS       OSInfo       `json:"os"`
	Hardware HardwareInfo `json:"hardware"`
	Network  NetworkInfo  `json:"network"`
	Runtime  RuntimeInfo  `json:"runtime"`
}

// OSInfo contains operating system information
type OSInfo struct {
	Name         string `json:"name"`          // e.g., "linux", "windows", "darwin"
	Version      string `json:"version"`       // OS version string
	Kernel       string `json:"kernel"`        // Kernel version
	Architecture string `json:"architecture"`  // e.g., "amd64", "arm64"
	Hostname     string `json:"hostname"`      // Machine hostname
	Uptime       int64  `json:"uptime"`        // System uptime in seconds
	UptimeHuman  string `json:"uptime_human"`  // Human-readable uptime
	BootTime     int64  `json:"boot_time"`     // Unix timestamp of boot time
}

// HardwareInfo contains hardware information
type HardwareInfo struct {
	CPU    CPUInfo    `json:"cpu"`
	Memory MemoryInfo `json:"memory"`
	Disk   []DiskInfo `json:"disk"`
	GPU    []GPUInfo  `json:"gpu,omitempty"`
}

// CPUInfo contains CPU information
type CPUInfo struct {
	Model      string  `json:"model"`       // CPU model name
	Cores      int     `json:"cores"`       // Physical cores
	Threads    int     `json:"threads"`     // Logical processors
	Frequency  float64 `json:"frequency"`   // Base frequency in MHz
	Usage      float64 `json:"usage"`       // Current CPU usage percentage
	VendorID   string  `json:"vendor_id"`   // CPU vendor
	CacheSize  int64   `json:"cache_size"`  // L2/L3 cache size in KB
}

// MemoryInfo contains memory information
type MemoryInfo struct {
	Total       uint64  `json:"total"`        // Total physical memory in bytes
	Available   uint64  `json:"available"`    // Available memory in bytes
	Used        uint64  `json:"used"`         // Used memory in bytes
	UsedPercent float64 `json:"used_percent"` // Memory usage percentage
	SwapTotal   uint64  `json:"swap_total"`   // Total swap in bytes
	SwapUsed    uint64  `json:"swap_used"`    // Used swap in bytes
}

// DiskInfo contains disk information
type DiskInfo struct {
	Device      string  `json:"device"`       // Device name
	MountPoint  string  `json:"mount_point"`  // Mount point
	FSType      string  `json:"fs_type"`      // Filesystem type
	Total       uint64  `json:"total"`        // Total space in bytes
	Used        uint64  `json:"used"`         // Used space in bytes
	Available   uint64  `json:"available"`    // Available space in bytes
	UsedPercent float64 `json:"used_percent"` // Usage percentage
}

// GPUInfo contains GPU information
type GPUInfo struct {
	Name        string `json:"name"`         // GPU name
	Vendor      string `json:"vendor"`       // GPU vendor
	Driver      string `json:"driver"`       // Driver version
	MemoryTotal uint64 `json:"memory_total"` // Total VRAM in bytes
	MemoryUsed  uint64 `json:"memory_used"`  // Used VRAM in bytes
}

// NetworkInfo contains network information
type NetworkInfo struct {
	Interfaces []InterfaceInfo `json:"interfaces"`
	PublicIP   string          `json:"public_ip,omitempty"` // External IP (if available)
}

// InterfaceInfo contains network interface information
type InterfaceInfo struct {
	Name       string   `json:"name"`        // Interface name
	MAC        string   `json:"mac"`         // MAC address
	IPv4       []string `json:"ipv4"`        // IPv4 addresses
	IPv6       []string `json:"ipv6"`        // IPv6 addresses
	MTU        int      `json:"mtu"`         // Maximum transmission unit
	Flags      []string `json:"flags"`       // Interface flags
	IsUp       bool     `json:"is_up"`       // Whether interface is up
	IsLoopback bool     `json:"is_loopback"` // Whether it's a loopback interface
}

// RuntimeInfo contains Go runtime information
type RuntimeInfo struct {
	GoVersion    string `json:"go_version"`
	NumCPU       int    `json:"num_cpu"`
	NumGoroutine int    `json:"num_goroutine"`
	GOMAXPROCS   int    `json:"gomaxprocs"`
	AllocMB      uint64 `json:"alloc_mb"`
	TotalAllocMB uint64 `json:"total_alloc_mb"`
	SysMB        uint64 `json:"sys_mb"`
	NumGC        uint32 `json:"num_gc"`
}

// Collect gathers all system information
func Collect() *Info {
	return &Info{
		OS:       collectOSInfo(),
		Hardware: collectHardwareInfo(),
		Network:  collectNetworkInfo(),
		Runtime:  collectRuntimeInfo(),
	}
}

// CollectOS gathers only OS information
func CollectOS() OSInfo {
	return collectOSInfo()
}

// CollectHardware gathers only hardware information
func CollectHardware() HardwareInfo {
	return collectHardwareInfo()
}

// CollectNetwork gathers only network information
func CollectNetwork() NetworkInfo {
	return collectNetworkInfo()
}

// CollectRuntime gathers only runtime information
func CollectRuntime() RuntimeInfo {
	return collectRuntimeInfo()
}

func collectOSInfo() OSInfo {
	hostname, _ := os.Hostname()
	uptime := getUptime()
	bootTime := timeutil.Now() - uptime

	return OSInfo{
		Name:         runtime.GOOS,
		Version:      getOSVersion(),
		Kernel:       getKernelVersion(),
		Architecture: runtime.GOARCH,
		Hostname:     hostname,
		Uptime:       uptime,
		UptimeHuman:  formatUptime(uptime),
		BootTime:     bootTime,
	}
}

func collectHardwareInfo() HardwareInfo {
	return HardwareInfo{
		CPU:    collectCPUInfo(),
		Memory: collectMemoryInfo(),
		Disk:   collectDiskInfo(),
		GPU:    collectGPUInfo(),
	}
}

func collectNetworkInfo() NetworkInfo {
	return NetworkInfo{
		Interfaces: collectInterfaceInfo(),
	}
}

func collectRuntimeInfo() RuntimeInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return RuntimeInfo{
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		GOMAXPROCS:   runtime.GOMAXPROCS(0),
		AllocMB:      m.Alloc / 1024 / 1024,
		TotalAllocMB: m.TotalAlloc / 1024 / 1024,
		SysMB:        m.Sys / 1024 / 1024,
		NumGC:        m.NumGC,
	}
}

func collectInterfaceInfo() []InterfaceInfo {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var result []InterfaceInfo
	for _, iface := range interfaces {
		info := InterfaceInfo{
			Name:       iface.Name,
			MAC:        iface.HardwareAddr.String(),
			MTU:        iface.MTU,
			IsUp:       iface.Flags&net.FlagUp != 0,
			IsLoopback: iface.Flags&net.FlagLoopback != 0,
		}

		// Parse flags
		if iface.Flags&net.FlagUp != 0 {
			info.Flags = append(info.Flags, "up")
		}
		if iface.Flags&net.FlagBroadcast != 0 {
			info.Flags = append(info.Flags, "broadcast")
		}
		if iface.Flags&net.FlagLoopback != 0 {
			info.Flags = append(info.Flags, "loopback")
		}
		if iface.Flags&net.FlagPointToPoint != 0 {
			info.Flags = append(info.Flags, "pointtopoint")
		}
		if iface.Flags&net.FlagMulticast != 0 {
			info.Flags = append(info.Flags, "multicast")
		}

		// Get IP addresses
		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				ip := addr.String()
				if strings.Contains(ip, ":") {
					info.IPv6 = append(info.IPv6, ip)
				} else {
					info.IPv4 = append(info.IPv4, ip)
				}
			}
		}

		result = append(result, info)
	}

	return result
}

func formatUptime(seconds int64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	if days > 0 {
		return strings.TrimSpace(strings.Join([]string{
			formatUnit(days, "day"),
			formatUnit(hours, "hour"),
			formatUnit(minutes, "minute"),
		}, " "))
	}
	if hours > 0 {
		return strings.TrimSpace(strings.Join([]string{
			formatUnit(hours, "hour"),
			formatUnit(minutes, "minute"),
		}, " "))
	}
	return formatUnit(minutes, "minute")
}

func formatUnit(value int64, unit string) string {
	if value == 0 {
		return ""
	}
	if value == 1 {
		return "1 " + unit
	}
	return strconv.FormatInt(value, 10) + " " + unit + "s"
}
