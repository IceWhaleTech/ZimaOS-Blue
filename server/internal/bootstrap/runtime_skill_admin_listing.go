package bootstrap

import (
	"sort"

	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func listRuntimeAdminSkills(registry *skillpkg.Registry, workspaceDir string) ([]tools.AdminSkillInfo, error) {
	result, err := listRuntimeAdminSkillsWithRoots(registry, skillmanifest.ResolveRoots(workspaceDir))
	if err != nil {
		return nil, err
	}
	return overlayRuntimeAdminSkillExposure(result, workspaceDir)
}

func listRuntimeAdminSkillsWithRoots(registry *skillpkg.Registry, roots []string) ([]tools.AdminSkillInfo, error) {
	items := make(map[string]tools.AdminSkillInfo)
	if err := appendRegistryAdminSkills(items, registry, roots); err != nil {
		return nil, err
	}
	if err := appendInstalledRootAdminSkills(items, registry, roots); err != nil {
		return nil, err
	}

	result := make([]tools.AdminSkillInfo, 0, len(items))
	for _, info := range items {
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Category != result[j].Category {
			return result[i].Category < result[j].Category
		}
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}
