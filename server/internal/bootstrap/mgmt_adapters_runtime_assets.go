package bootstrap

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sysinfo"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type mgmtSkillAdapter struct {
	registry     *skill.Registry
	workspaceDir string
}

func (a *mgmtSkillAdapter) ListSkills(_ context.Context) ([]tools.AdminSkillInfo, error) {
	return listRuntimeAdminSkills(a.registry, a.workspaceDir)
}

func (a *mgmtSkillAdapter) EnableSkill(_ context.Context, id string) error {
	canonicalID, err := ensureRuntimeManagedSkillRegistered(a.registry, a.workspaceDir, id)
	if err != nil {
		return err
	}
	return a.registry.Enable(canonicalID)
}

func (a *mgmtSkillAdapter) DisableSkill(_ context.Context, id string) error {
	canonicalID, err := ensureRuntimeManagedSkillRegistered(a.registry, a.workspaceDir, id)
	if err != nil {
		return err
	}
	return a.registry.Disable(canonicalID)
}

type mgmtToolAdapter struct {
	registry *tools.Registry
}

func (a *mgmtToolAdapter) ListTools(_ context.Context) ([]tools.AdminToolInfo, error) {
	names := a.registry.List()
	result := make([]tools.AdminToolInfo, 0, len(names))
	for _, name := range names {
		t := a.registry.Get(name)
		if t == nil {
			continue
		}
		def := t.Definition()
		result = append(result, tools.AdminToolInfo{
			Name:                def.Name,
			Description:         def.Description,
			RiskLevel:           def.RiskLevel,
			VisibilityAllowlist: append([]string(nil), def.VisibilityAllowlist...),
		})
	}
	for _, name := range a.registry.ListDisabled() {
		t := a.registry.Get(name)
		if t == nil {
			continue
		}
		def := t.Definition()
		result = append(result, tools.AdminToolInfo{
			Name:                def.Name,
			Description:         def.Description,
			Disabled:            true,
			RiskLevel:           def.RiskLevel,
			VisibilityAllowlist: append([]string(nil), def.VisibilityAllowlist...),
		})
	}
	return result, nil
}

func (a *mgmtToolAdapter) EnableTool(_ context.Context, name string) error {
	if !a.registry.Enable(name) {
		return fmt.Errorf("tool %q not found or already enabled", name)
	}
	return nil
}

func (a *mgmtToolAdapter) DisableTool(_ context.Context, name string) error {
	if !a.registry.Disable(name) {
		return fmt.Errorf("tool %q not found or already disabled", name)
	}
	return nil
}

type mgmtSystemAdapter struct {
	version   string
	startTime time.Time
}

func (a *mgmtSystemAdapter) Health(_ context.Context) (*tools.AdminSystemInfo, error) {
	procMem := sysinfo.GetProcessMemInfo()
	startTime := a.startTime
	if startTime.IsZero() {
		startTime = time.Now()
	}
	uptime := time.Since(startTime)
	return &tools.AdminSystemInfo{
		Version:       a.version,
		Uptime:        formatDuration(uptime),
		UptimeSeconds: uptime.Seconds(),
		GoVersion:     runtime.Version(),
		NumCPU:        runtime.NumCPU(),
		Goroutines:    runtime.NumGoroutine(),
		MemAllocMB:    float64(procMem.GoAlloc) / (1024 * 1024),
		MemRSSMB:      float64(procMem.RSSB) / (1024 * 1024),
	}, nil
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
