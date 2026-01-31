package builtin

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/sysinfo"
)

// SystemInfo is a built-in system information skill
type SystemInfo struct {
	manifest *skill.Manifest
}

// NewSystemInfo creates a new system info skill
func NewSystemInfo() *SystemInfo {
	return &SystemInfo{
		manifest: &skill.Manifest{
			ID:          "system-info",
			Name:        "System Info",
			Version:     "1.0.0",
			Description: "Returns comprehensive system information including OS, hardware, network, and GPU details",
			Category:    "system",
			Icon:        "systeminfo",
			Tags:        []string{"system", "info", "utility", "hardware", "network"},
			Inputs: []skill.Parameter{
				{
					Name:        "type",
					Type:        "string",
					Description: "Type of info to retrieve: 'all', 'os', 'memory', 'cpu', 'disk', 'network', 'gpu', 'runtime'",
					Required:    false,
					Default:     "all",
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "info",
					Type:        "object",
					Description: "System information",
				},
			},
			Permissions: []string{"system.read"},
		},
	}
}

// Manifest returns the skill manifest
func (s *SystemInfo) Manifest() *skill.Manifest {
	return s.manifest
}

// Validate validates the input parameters
func (s *SystemInfo) Validate(input map[string]any) error {
	if infoType, ok := input["type"]; ok {
		typeStr, ok := infoType.(string)
		if !ok {
			return fmt.Errorf("type must be a string")
		}
		validTypes := map[string]bool{
			"all":     true,
			"os":      true,
			"memory":  true,
			"cpu":     true,
			"disk":    true,
			"network": true,
			"gpu":     true,
			"runtime": true,
		}
		if !validTypes[typeStr] {
			return fmt.Errorf("invalid type: %s", typeStr)
		}
	}
	return nil
}

// Execute executes the system info skill
func (s *SystemInfo) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	infoType := "all"
	if t, ok := input["type"].(string); ok {
		infoType = t
	}

	info := make(map[string]any)

	switch infoType {
	case "os":
		info["os"] = sysinfo.CollectOS()
	case "memory":
		hw := sysinfo.CollectHardware()
		info["memory"] = hw.Memory
	case "cpu":
		hw := sysinfo.CollectHardware()
		info["cpu"] = hw.CPU
	case "disk":
		hw := sysinfo.CollectHardware()
		info["disk"] = hw.Disk
	case "network":
		info["network"] = sysinfo.CollectNetwork()
	case "gpu":
		hw := sysinfo.CollectHardware()
		info["gpu"] = hw.GPU
	case "runtime":
		info["runtime"] = sysinfo.CollectRuntime()
	case "all":
		fullInfo := sysinfo.Collect()
		info["os"] = fullInfo.OS
		info["hardware"] = fullInfo.Hardware
		info["network"] = fullInfo.Network
		info["runtime"] = fullInfo.Runtime
	}

	return skill.NewResult(info), nil
}
