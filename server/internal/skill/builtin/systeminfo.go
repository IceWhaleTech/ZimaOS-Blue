package builtin

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
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
			Description: "Returns system information",
			Category:    "system",
			Tags:        []string{"system", "info", "utility"},
			Inputs: []skill.Parameter{
				{
					Name:        "type",
					Type:        "string",
					Description: "Type of info to retrieve: 'all', 'os', 'memory', 'cpu'",
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
		validTypes := map[string]bool{"all": true, "os": true, "memory": true, "cpu": true}
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
		info["os"] = s.getOSInfo()
	case "memory":
		info["memory"] = s.getMemoryInfo()
	case "cpu":
		info["cpu"] = s.getCPUInfo()
	case "all":
		info["os"] = s.getOSInfo()
		info["memory"] = s.getMemoryInfo()
		info["cpu"] = s.getCPUInfo()
	}

	return skill.NewResult(info), nil
}

func (s *SystemInfo) getOSInfo() map[string]any {
	hostname, _ := os.Hostname()
	return map[string]any{
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
		"hostname": hostname,
		"go_version": runtime.Version(),
	}
}

func (s *SystemInfo) getMemoryInfo() map[string]any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]any{
		"alloc_mb":       m.Alloc / 1024 / 1024,
		"total_alloc_mb": m.TotalAlloc / 1024 / 1024,
		"sys_mb":         m.Sys / 1024 / 1024,
		"num_gc":         m.NumGC,
	}
}

func (s *SystemInfo) getCPUInfo() map[string]any {
	return map[string]any{
		"num_cpu":       runtime.NumCPU(),
		"num_goroutine": runtime.NumGoroutine(),
		"gomaxprocs":    runtime.GOMAXPROCS(0),
	}
}
