//go:build darwin

package sysinfo

import (
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func getUptime() int64 {
	// Use sysctl to get boot time
	cmd := exec.Command("sysctl", "-n", "kern.boottime")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	// Parse "{ sec = 1807890, usec = 0 }" format
	outputStr := string(output)
	if idx := strings.Index(outputStr, "sec = "); idx != -1 {
		start := idx + 6
		end := strings.Index(outputStr[start:], ",")
		if end != -1 {
			bootTime, _ := strconv.ParseInt(outputStr[start:start+end], 10, 64)
			var tv unix.Timeval
			unix.Gettimeofday(&tv)
			return tv.Sec - bootTime
		}
	}
	return 0
}

func getOSVersion() string {
	// Get macOS version using sw_vers
	cmd := exec.Command("sw_vers", "-productVersion")
	output, err := cmd.Output()
	if err != nil {
		return "macOS"
	}
	version := strings.TrimSpace(string(output))

	// Get product name
	cmd = exec.Command("sw_vers", "-productName")
	nameOutput, err := cmd.Output()
	if err != nil {
		return "macOS " + version
	}
	name := strings.TrimSpace(string(nameOutput))

	return name + " " + version
}

func getKernelVersion() string {
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return ""
	}
	return bytesToString(uname.Release[:])
}

func collectCPUInfo() CPUInfo {
	info := CPUInfo{}

	// Get CPU brand string
	if brand, err := unix.Sysctl("machdep.cpu.brand_string"); err == nil {
		info.Model = brand
	}

	// Get vendor
	if vendor, err := unix.Sysctl("machdep.cpu.vendor"); err == nil {
		info.VendorID = vendor
	}

	// Get core count
	if cores, err := unix.SysctlUint32("hw.physicalcpu"); err == nil {
		info.Cores = int(cores)
	}

	// Get thread count
	if threads, err := unix.SysctlUint32("hw.logicalcpu"); err == nil {
		info.Threads = int(threads)
	}

	// Get CPU frequency (in Hz, convert to MHz)
	if freq, err := unix.SysctlUint64("hw.cpufrequency"); err == nil {
		info.Frequency = float64(freq) / 1000000
	}

	// Get cache size
	if cache, err := unix.SysctlUint64("hw.l2cachesize"); err == nil {
		info.CacheSize = int64(cache / 1024) // Convert to KB
	}

	// Get CPU usage
	info.Usage = getCPUUsage()

	return info
}

func getCPUUsage() float64 {
	// Use top command to get CPU usage
	cmd := exec.Command("top", "-l", "1", "-n", "0", "-stats", "cpu")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "CPU usage:") {
			// Parse "CPU usage: 5.0% user, 10.0% sys, 85.0% idle"
			parts := strings.Split(line, ",")
			var user, sys float64
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if strings.Contains(part, "user") {
					fields := strings.Fields(part)
					if len(fields) > 0 {
						user, _ = strconv.ParseFloat(strings.TrimSuffix(fields[0], "%"), 64)
					}
				} else if strings.Contains(part, "sys") {
					fields := strings.Fields(part)
					if len(fields) > 0 {
						sys, _ = strconv.ParseFloat(strings.TrimSuffix(fields[0], "%"), 64)
					}
				}
			}
			return user + sys
		}
	}
	return 0
}

func collectMemoryInfo() MemoryInfo {
	info := MemoryInfo{}

	// Get total memory
	if total, err := unix.SysctlUint64("hw.memsize"); err == nil {
		info.Total = total
	}

	// Get page size
	pageSize := uint64(4096)
	if ps, err := unix.SysctlUint32("hw.pagesize"); err == nil {
		pageSize = uint64(ps)
	}
	_ = pageSize // Used below

	// Use vm_stat to get memory details
	cmd := exec.Command("vm_stat")
	output, err := cmd.Output()
	if err != nil {
		return info
	}

	var freePages, activePages, inactivePages, wiredPages, compressedPages uint64
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(strings.TrimSuffix(parts[1], "."))
		pages, _ := strconv.ParseUint(value, 10, 64)

		switch key {
		case "Pages free":
			freePages = pages
		case "Pages active":
			activePages = pages
		case "Pages inactive":
			inactivePages = pages
		case "Pages wired down":
			wiredPages = pages
		case "Pages occupied by compressor":
			compressedPages = pages
		}
	}

	info.Available = (freePages + inactivePages) * pageSize
	info.Used = (activePages + wiredPages + compressedPages) * pageSize
	if info.Total > 0 {
		info.UsedPercent = float64(info.Used) / float64(info.Total) * 100
	}

	// Get swap info
	cmd = exec.Command("sysctl", "-n", "vm.swapusage")
	output, err = cmd.Output()
	if err == nil {
		// Parse "total = 2048.00M  used = 100.00M  free = 1948.00M"
		outputStr := string(output)
		if idx := strings.Index(outputStr, "total = "); idx != -1 {
			info.SwapTotal = parseMemoryValue(outputStr[idx+8:])
		}
		if idx := strings.Index(outputStr, "used = "); idx != -1 {
			info.SwapUsed = parseMemoryValue(outputStr[idx+7:])
		}
	}

	return info
}

func parseMemoryValue(s string) uint64 {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && (s[end] >= '0' && s[end] <= '9' || s[end] == '.') {
		end++
	}
	if end == 0 {
		return 0
	}
	value, _ := strconv.ParseFloat(s[:end], 64)

	// Check unit
	if end < len(s) {
		switch s[end] {
		case 'G':
			value *= 1024 * 1024 * 1024
		case 'M':
			value *= 1024 * 1024
		case 'K':
			value *= 1024
		}
	}
	return uint64(value)
}

func collectDiskInfo() []DiskInfo {
	var disks []DiskInfo

	// Use df command
	cmd := exec.Command("df", "-k")
	output, err := cmd.Output()
	if err != nil {
		return disks
	}

	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if i == 0 { // Skip header
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		device := fields[0]
		// Only include physical disks
		if !strings.HasPrefix(device, "/dev/disk") {
			continue
		}

		total, _ := strconv.ParseUint(fields[1], 10, 64)
		used, _ := strconv.ParseUint(fields[2], 10, 64)
		available, _ := strconv.ParseUint(fields[3], 10, 64)
		mountPoint := fields[len(fields)-1]

		// Convert from KB to bytes
		total *= 1024
		used *= 1024
		available *= 1024

		var usedPercent float64
		if total > 0 {
			usedPercent = float64(used) / float64(total) * 100
		}

		// Get filesystem type
		fsType := "apfs" // Default for modern macOS
		cmd := exec.Command("diskutil", "info", device)
		if infoOutput, err := cmd.Output(); err == nil {
			infoStr := string(infoOutput)
			if idx := strings.Index(infoStr, "Type (Bundle):"); idx != -1 {
				line := infoStr[idx:]
				if endIdx := strings.Index(line, "\n"); endIdx != -1 {
					parts := strings.SplitN(line[:endIdx], ":", 2)
					if len(parts) == 2 {
						fsType = strings.TrimSpace(parts[1])
					}
				}
			}
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

	// Use system_profiler to get GPU info
	cmd := exec.Command("system_profiler", "SPDisplaysDataType", "-json")
	output, err := cmd.Output()
	if err != nil {
		return gpus
	}

	// Simple parsing of JSON output
	outputStr := string(output)

	// Find GPU entries
	idx := 0
	for {
		nameIdx := strings.Index(outputStr[idx:], "\"_name\"")
		if nameIdx == -1 {
			break
		}
		idx += nameIdx

		gpu := GPUInfo{}

		// Extract name
		if start := strings.Index(outputStr[idx:], ":"); start != -1 {
			start += idx + 1
			// Skip whitespace and quote
			for start < len(outputStr) && (outputStr[start] == ' ' || outputStr[start] == '"') {
				start++
			}
			if end := strings.Index(outputStr[start:], "\""); end != -1 {
				gpu.Name = outputStr[start : start+end]
			}
		}

		// Extract vendor
		if vendorIdx := strings.Index(outputStr[idx:], "\"sppci_vendor\""); vendorIdx != -1 {
			vendorStart := idx + vendorIdx
			if start := strings.Index(outputStr[vendorStart:], ":"); start != -1 {
				start += vendorStart + 1
				for start < len(outputStr) && (outputStr[start] == ' ' || outputStr[start] == '"') {
					start++
				}
				if end := strings.Index(outputStr[start:], "\""); end != -1 {
					gpu.Vendor = outputStr[start : start+end]
				}
			}
		}

		// Extract VRAM
		if vramIdx := strings.Index(outputStr[idx:], "\"spdisplays_vram\""); vramIdx != -1 {
			vramStart := idx + vramIdx
			if start := strings.Index(outputStr[vramStart:], ":"); start != -1 {
				start += vramStart + 1
				for start < len(outputStr) && (outputStr[start] == ' ' || outputStr[start] == '"') {
					start++
				}
				if end := strings.Index(outputStr[start:], "\""); end != -1 {
					vramStr := outputStr[start : start+end]
					gpu.MemoryTotal = parseMemoryValue(vramStr)
				}
			}
		}

		if gpu.Name != "" {
			gpus = append(gpus, gpu)
		}

		idx++
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
