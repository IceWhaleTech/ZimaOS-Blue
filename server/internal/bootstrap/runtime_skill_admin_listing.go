package bootstrap

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func listRuntimeAdminSkills(registry *skillpkg.Registry, workspaceDir string) ([]tools.AdminSkillInfo, error) {
	return listRuntimeAdminSkillsWithRoots(registry, skillmanifest.ResolveRoots(workspaceDir))
}

func listRuntimeAdminSkillsWithRoots(registry *skillpkg.Registry, roots []string) ([]tools.AdminSkillInfo, error) {
	items := make(map[string]tools.AdminSkillInfo)

	add := func(info tools.AdminSkillInfo) {
		id := strings.TrimSpace(info.ID)
		if id == "" {
			return
		}
		key := strings.ToLower(id)
		if strings.TrimSpace(info.Name) == "" {
			info.Name = id
		}
		items[key] = info
	}

	if registry != nil {
		for _, info := range registry.List() {
			if info == nil || info.Manifest == nil {
				continue
			}
			item := tools.AdminSkillInfo{
				ID:       strings.TrimSpace(info.Manifest.ID),
				Name:     strings.TrimSpace(info.Manifest.Name),
				Category: strings.TrimSpace(info.Manifest.Category),
				Enabled:  info.Enabled,
				Builtin:  info.Builtin,
			}
			resolved, ok, err := skillmanifest.FindAnyByCandidatesStrict(skillmanifest.CandidateIDs(item.ID), roots, "", skillmanifest.Options{})
			if err != nil {
				return nil, err
			}
			if ok {
				item = adminSkillInfoFromDocument(resolved.Document, item.Enabled, item.Builtin && resolved.Embedded)
				if regInfo := registry.GetInfo(item.ID); regInfo != nil {
					item.Enabled = regInfo.Enabled
				}
			}
			add(item)
		}
	}

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			bundle, err := skillmanifest.ValidateInstalledDir(filepath.Join(root, entry.Name()), "", skillmanifest.Options{})
			if err != nil {
				continue
			}
			item := adminSkillInfoFromDocument(bundle.Document, bundle.Document.Enabled, false)
			if registry != nil {
				if regInfo := registry.GetInfo(item.ID); regInfo != nil {
					item.Enabled = regInfo.Enabled
					item.Builtin = regInfo.Builtin
				}
			}
			if _, exists := items[strings.ToLower(item.ID)]; exists {
				continue
			}
			add(item)
		}
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
