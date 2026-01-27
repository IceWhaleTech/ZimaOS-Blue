//go:build linux

package sysinfo

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func getUptime() int64 {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0
	}
	return info.Uptime
}

func getOSVersion() string {
	// Try /etc/os-release first (most modern distros)
	if version := readOSRelease(); version != "" {
		return version
	}
	// Fallback to /etc/lsb-release
	if version := readLSBRelease(); version != "" {
		return version
	}
	// Fallback to uname
	var uname unix.Utsname
	if err := unix.Uname(&uname); err == nil {
		return bytesToString(uname.Version[:])
	}
	return "Linux"
}

func getKernelVersion() string {
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return ""
	}
	return bytesToString(uname.Release[:])
}

func readOSRelease() string {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer file.Close()

	var prettyName, name, version string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			prettyName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		} else if strings.HasPrefix(line, "NAME=") {
			name = strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
		} else if strings.HasPrefix(line, "VERSION=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION="), "\"")
		}
	}

	if prettyName != "" {
		return prettyName
	}
	if name != "" && version != "" {
		return name + " " + version
	}
	return name
}

func readLSBRelease() string {
	file, err := os.Open("/etc/lsb-release")
	if err != nil {
		return ""
	}
	defer file.Close()

	var distrib string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "DISTRIB_DESCRIPTION=") {
			distrib = strings.Trim(strings.TrimPrefix(line, "DISTRIB_DESCRIPTION="), "\"")
			break
		}
	}
	return distrib
}

func collectCPUInfo() CPUInfo {
	info := CPUInfo{
		Threads: 0,
	}

	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return info
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "model name":
			if info.Model == "" {
				info.Model = value
			}
		case "vendor_id":
			if info.VendorID == "" {
				info.VendorID = value
			}
		case "cpu MHz":
			if info.Frequency == 0 {
				if freq, err := strconv.ParseFloat(value, 64); err == nil {
					info.Frequency = freq
				}
			}
		case "cache size":
			if info.CacheSize == 0 {
				// Parse "6144 KB" format
				parts := strings.Fields(value)
				if len(parts) > 0 {
					if size, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
						info.CacheSize = size
					}
				}
			}
		case "processor":
			info.Threads++
		case "cpu cores":
			if info.Cores == 0 {
				if cores, err := strconv.Atoi(value); err == nil {
					info.Cores = cores
				}
			}
		}
	}

	// Calculate CPU usage
	info.Usage = calculateCPUUsage()

	return info
}

func calculateCPUUsage() float64 {
	// Read /proc/stat for CPU usage
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				user, _ := strconv.ParseUint(fields[1], 10, 64)
				nice, _ := strconv.ParseUint(fields[2], 10, 64)
				system, _ := strconv.ParseUint(fields[3], 10, 64)
				idle, _ := strconv.ParseUint(fields[4], 10, 64)

				total := user + nice + system + idle
				if total > 0 {
					return float64(user+nice+system) / float64(total) * 100
				}
			}
		}
	}
	return 0
}

func collectMemoryInfo() MemoryInfo {
	info := MemoryInfo{}

	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return info
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valueStr := strings.TrimSpace(parts[1])
		// Remove "kB" suffix and parse
		valueStr = strings.TrimSuffix(valueStr, " kB")
		value, err := strconv.ParseUint(strings.TrimSpace(valueStr), 10, 64)
		if err != nil {
			continue
		}
		// Convert from KB to bytes
		value *= 1024

		switch key {
		case "MemTotal":
			info.Total = value
		case "MemAvailable":
			info.Available = value
		case "SwapTotal":
			info.SwapTotal = value
		case "SwapFree":
			info.SwapUsed = info.SwapTotal - value
		}
	}

	info.Used = info.Total - info.Available
	if info.Total > 0 {
		info.UsedPercent = float64(info.Used) / float64(info.Total) * 100
	}

	return info
}

func collectDiskInfo() []DiskInfo {
	var disks []DiskInfo

	// Read /proc/mounts
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return disks
	}
	defer file.Close()

	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		device := fields[0]
		mountPoint := fields[1]
		fsType := fields[2]

		// Skip virtual filesystems
		if !strings.HasPrefix(device, "/dev/") {
			continue
		}
		// Skip duplicates
		if seen[mountPoint] {
			continue
		}
		seen[mountPoint] = true

		var stat unix.Statfs_t
		if err := unix.Statfs(mountPoint, &stat); err != nil {
			continue
		}

		total := stat.Blocks * uint64(stat.Bsize)
		available := stat.Bavail * uint64(stat.Bsize)
		used := total - stat.Bfree*uint64(stat.Bsize)

		var usedPercent float64
		if total > 0 {
			usedPercent = float64(used) / float64(total) * 100
		}

		disks = append(disks, DiskInfo{
			Device:      device,
			MountPoint:  mountPoint,
			FSType:      fsType,
			Total:       total,
			Used:        used,
			Available:   available,
			UsedPercent: usedPercent,
		})
	}

	return disks
}

func collectGPUInfo() []GPUInfo {
	var gpus []GPUInfo

	// Try nvidia-smi for NVIDIA GPUs
	if nvidiaGPUs := collectNvidiaGPU(); len(nvidiaGPUs) > 0 {
		gpus = append(gpus, nvidiaGPUs...)
	}

	// Try to read from /sys/class/drm for other GPUs
	if drmGPUs := collectDRMGPU(); len(drmGPUs) > 0 {
		gpus = append(gpus, drmGPUs...)
	}

	return gpus
}

func collectNvidiaGPU() []GPUInfo {
	var gpus []GPUInfo

	// Check if nvidia-smi exists
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,driver_version,memory.total,memory.used", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return gpus
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		fields := strings.Split(line, ", ")
		if len(fields) < 4 {
			continue
		}

		memTotal, _ := strconv.ParseUint(strings.TrimSpace(fields[2]), 10, 64)
		memUsed, _ := strconv.ParseUint(strings.TrimSpace(fields[3]), 10, 64)

		gpus = append(gpus, GPUInfo{
			Name:        strings.TrimSpace(fields[0]),
			Vendor:      "NVIDIA",
			Driver:      strings.TrimSpace(fields[1]),
			MemoryTotal: memTotal * 1024 * 1024, // Convert MiB to bytes
			MemoryUsed:  memUsed * 1024 * 1024,
		})
	}

	return gpus
}

func collectDRMGPU() []GPUInfo {
	var gpus []GPUInfo

	// Read from /sys/class/drm
	drmPath := "/sys/class/drm"
	entries, err := os.ReadDir(drmPath)
	if err != nil {
		return gpus
	}

	seen := make(map[string]bool)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "card") || strings.Contains(entry.Name(), "-") {
			continue
		}

		cardPath := filepath.Join(drmPath, entry.Name(), "device")

		// Read vendor
		vendorData, err := os.ReadFile(filepath.Join(cardPath, "vendor"))
		if err != nil {
			continue
		}
		vendor := strings.TrimSpace(string(vendorData))

		// Skip if already seen (NVIDIA handled separately)
		if vendor == "0x10de" { // NVIDIA vendor ID
			continue
		}
		if seen[vendor] {
			continue
		}
		seen[vendor] = true

		gpu := GPUInfo{}

		// Map vendor ID to name
		switch vendor {
		case "0x1002":
			gpu.Vendor = "AMD"
		case "0x8086":
			gpu.Vendor = "Intel"
		default:
			gpu.Vendor = vendor
		}

		// Try to read device name
		if nameData, err := os.ReadFile(filepath.Join(cardPath, "label")); err == nil {
			gpu.Name = strings.TrimSpace(string(nameData))
		}

		// Try to read driver
		if driverLink, err := os.Readlink(filepath.Join(cardPath, "driver")); err == nil {
			gpu.Driver = filepath.Base(driverLink)
		}

		gpus = append(gpus, gpu)
	}

	return gpus
}

func bytesToString(b []byte) string {
	n := 0
	for i, v := range b {
		if v == 0 {
			n = i
			break
		}
	}
	return string(b[:n])
}
