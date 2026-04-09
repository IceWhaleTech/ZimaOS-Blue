//go:build windows

package sysinfo

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	ntdll                    = windows.NewLazySystemDLL("ntdll.dll")
	procGetTickCount64       = kernel32.NewProc("GetTickCount64")
	procRtlGetVersion        = ntdll.NewProc("RtlGetVersion")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetSystemInfo        = kernel32.NewProc("GetSystemInfo")
)

func hiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

// RTL_OSVERSIONINFOW structure
type rtlOSVersionInfoW struct {
	OSVersionInfoSize uint32
	MajorVersion      uint32
	MinorVersion      uint32
	BuildNumber       uint32
	PlatformId        uint32
	CSDVersion        [128]uint16
}

// MEMORYSTATUSEX structure
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// SYSTEM_INFO structure
type systemInfo struct {
	ProcessorArchitecture     uint16
	Reserved                  uint16
	PageSize                  uint32
	MinimumApplicationAddress uintptr
	MaximumApplicationAddress uintptr
	ActiveProcessorMask       uintptr
	NumberOfProcessors        uint32
	ProcessorType             uint32
	AllocationGranularity     uint32
	ProcessorLevel            uint16
	ProcessorRevision         uint16
}

func getUptime() int64 {
	ret, _, _ := procGetTickCount64.Call()
	return int64(ret) / 1000 // Convert milliseconds to seconds
}

func getOSVersion() string {
	var osvi rtlOSVersionInfoW
	osvi.OSVersionInfoSize = uint32(unsafe.Sizeof(osvi))
	procRtlGetVersion.Call(uintptr(unsafe.Pointer(&osvi)))

	major := osvi.MajorVersion
	minor := osvi.MinorVersion
	build := osvi.BuildNumber

	// Map to Windows version names
	var versionName string
	switch {
	case major == 10 && build >= 22000:
		versionName = "Windows 11"
	case major == 10:
		versionName = "Windows 10"
	case major == 6 && minor == 3:
		versionName = "Windows 8.1"
	case major == 6 && minor == 2:
		versionName = "Windows 8"
	case major == 6 && minor == 1:
		versionName = "Windows 7"
	default:
		versionName = "Windows"
	}

	return versionName + " (Build " + strconv.FormatUint(uint64(build), 10) + ")"
}

func getKernelVersion() string {
	var osvi rtlOSVersionInfoW
	osvi.OSVersionInfoSize = uint32(unsafe.Sizeof(osvi))
	procRtlGetVersion.Call(uintptr(unsafe.Pointer(&osvi)))

	return strconv.FormatUint(uint64(osvi.MajorVersion), 10) + "." +
		strconv.FormatUint(uint64(osvi.MinorVersion), 10) + "." +
		strconv.FormatUint(uint64(osvi.BuildNumber), 10)
}

func collectCPUInfo() CPUInfo {
	info := CPUInfo{}

	// Get CPU info from WMI via PowerShell
	cmd := hiddenCommand("powershell", "-NoProfile", "-Command",
		"Get-CimInstance -ClassName Win32_Processor | Select-Object Name,Manufacturer,NumberOfCores,NumberOfLogicalProcessors,MaxClockSpeed,L2CacheSize,L3CacheSize | ConvertTo-Json")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to basic info
		var sysInfo systemInfo
		procGetSystemInfo.Call(uintptr(unsafe.Pointer(&sysInfo)))
		info.Threads = int(sysInfo.NumberOfProcessors)
		return info
	}

	// Parse JSON output
	outputStr := string(output)
	info.Model = extractJSONString(outputStr, "Name")
	info.VendorID = extractJSONString(outputStr, "Manufacturer")
	info.Cores = extractJSONInt(outputStr, "NumberOfCores")
	info.Threads = extractJSONInt(outputStr, "NumberOfLogicalProcessors")
	info.Frequency = float64(extractJSONInt(outputStr, "MaxClockSpeed"))

	l2 := extractJSONInt(outputStr, "L2CacheSize")
	l3 := extractJSONInt(outputStr, "L3CacheSize")
	info.CacheSize = int64(l2 + l3)

	// Get CPU usage
	info.Usage = getCPUUsage()

	return info
}

func getCPUUsage() float64 {
	cmd := hiddenCommand("powershell", "-NoProfile", "-Command",
		"(Get-CimInstance -ClassName Win32_Processor).LoadPercentage")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	usage, _ := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	return usage
}

func collectMemoryInfo() MemoryInfo {
	var memStatus memoryStatusEx
	memStatus.Length = uint32(unsafe.Sizeof(memStatus))

	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	if ret == 0 {
		return MemoryInfo{}
	}

	return MemoryInfo{
		Total:       memStatus.TotalPhys,
		Available:   memStatus.AvailPhys,
		Used:        memStatus.TotalPhys - memStatus.AvailPhys,
		UsedPercent: float64(memStatus.MemoryLoad),
		SwapTotal:   memStatus.TotalPageFile - memStatus.TotalPhys,
		SwapUsed:    (memStatus.TotalPageFile - memStatus.TotalPhys) - (memStatus.AvailPageFile - memStatus.AvailPhys),
	}
}

func collectDiskInfo() []DiskInfo {
	var disks []DiskInfo

	// Get logical drives
	drives, err := windows.GetLogicalDrives()
	if err != nil {
		return disks
	}

	for i := 0; i < 26; i++ {
		if drives&(1<<uint(i)) == 0 {
			continue
		}

		driveLetter := string(rune('A'+i)) + ":\\"
		driveType := windows.GetDriveType(syscall.StringToUTF16Ptr(driveLetter))

		// Only include fixed drives
		if driveType != windows.DRIVE_FIXED {
			continue
		}

		var freeBytesAvailable, totalBytes, totalFreeBytes uint64
		err := windows.GetDiskFreeSpaceEx(
			syscall.StringToUTF16Ptr(driveLetter),
			&freeBytesAvailable,
			&totalBytes,
			&totalFreeBytes,
		)
		if err != nil {
			continue
		}

		used := totalBytes - totalFreeBytes
		var usedPercent float64
		if totalBytes > 0 {
			usedPercent = float64(used) / float64(totalBytes) * 100
		}

		// Get filesystem type
		var volumeName [256]uint16
		var fsName [256]uint16
		windows.GetVolumeInformation(
			syscall.StringToUTF16Ptr(driveLetter),
			&volumeName[0], uint32(len(volumeName)),
			nil, nil, nil,
			&fsName[0], uint32(len(fsName)),
		)

		disks = append(disks, DiskInfo{
			Device:      driveLetter[:2],
			MountPoint:  driveLetter,
			FSType:      syscall.UTF16ToString(fsName[:]),
			Total:       totalBytes,
			Used:        used,
			Available:   freeBytesAvailable,
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
		return gpus
	}

	// Fallback to WMI
	cmd := hiddenCommand("powershell", "-NoProfile", "-Command",
		"Get-CimInstance -ClassName Win32_VideoController | Select-Object Name,AdapterCompatibility,DriverVersion,AdapterRAM | ConvertTo-Json")
	output, err := cmd.Output()
	if err != nil {
		return gpus
	}

	outputStr := string(output)

	// Handle both single object and array
	if strings.HasPrefix(strings.TrimSpace(outputStr), "[") {
		// Multiple GPUs - parse as array
		entries := strings.Split(outputStr, "},{")
		for _, entry := range entries {
			gpu := GPUInfo{
				Name:        extractJSONString(entry, "Name"),
				Vendor:      extractJSONString(entry, "AdapterCompatibility"),
				Driver:      extractJSONString(entry, "DriverVersion"),
				MemoryTotal: uint64(extractJSONInt(entry, "AdapterRAM")),
			}
			if gpu.Name != "" {
				gpus = append(gpus, gpu)
			}
		}
	} else {
		// Single GPU
		gpu := GPUInfo{
			Name:        extractJSONString(outputStr, "Name"),
			Vendor:      extractJSONString(outputStr, "AdapterCompatibility"),
			Driver:      extractJSONString(outputStr, "DriverVersion"),
			MemoryTotal: uint64(extractJSONInt(outputStr, "AdapterRAM")),
		}
		if gpu.Name != "" {
			gpus = append(gpus, gpu)
		}
	}

	return gpus
}

func collectNvidiaGPU() []GPUInfo {
	var gpus []GPUInfo

	cmd := hiddenCommand("nvidia-smi", "--query-gpu=name,driver_version,memory.total,memory.used", "--format=csv,noheader,nounits")
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
			MemoryTotal: memTotal * 1024 * 1024,
			MemoryUsed:  memUsed * 1024 * 1024,
		})
	}

	return gpus
}

// Helper functions for simple JSON parsing
func extractJSONString(json, key string) string {
	search := "\"" + key + "\":"
	idx := strings.Index(json, search)
	if idx == -1 {
		return ""
	}
	start := idx + len(search)
	// Skip whitespace
	for start < len(json) && (json[start] == ' ' || json[start] == '\t') {
		start++
	}
	if start >= len(json) {
		return ""
	}
	if json[start] == '"' {
		start++
		end := strings.Index(json[start:], "\"")
		if end == -1 {
			return ""
		}
		return json[start : start+end]
	}
	return ""
}

func extractJSONInt(json, key string) int {
	search := "\"" + key + "\":"
	idx := strings.Index(json, search)
	if idx == -1 {
		return 0
	}
	start := idx + len(search)
	// Skip whitespace
	for start < len(json) && (json[start] == ' ' || json[start] == '\t') {
		start++
	}
	if start >= len(json) {
		return 0
	}
	// Find end of number
	end := start
	for end < len(json) && (json[end] >= '0' && json[end] <= '9') {
		end++
	}
	if end == start {
		return 0
	}
	val, _ := strconv.Atoi(json[start:end])
	return val
}
